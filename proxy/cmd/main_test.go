package main

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"testing"
	"time"

	"sea_battle/my_types"
	"sea_battle/proxy/config"
	mw "sea_battle/proxy/middleware"

	"golang.org/x/crypto/bcrypt"
)

type proxyAuthRepoStub struct {
	passwordHash   string
	registeredName string
	registeredHash string
	matchError     error
	matchCalls     int
	lastUserWin    bool
}

func TestProxyHandlerStoresGameResultAndReportsPersistenceErrors(test *testing.T) {
	tests := []struct {
		name            string
		repositoryError error
		wantStatus      int
	}{
		{name: "saved", wantStatus: http.StatusOK},
		{name: "persistence failure", repositoryError: errors.New("database unavailable"), wantStatus: http.StatusInternalServerError},
	}
	for _, testCase := range tests {
		test.Run(testCase.name, func(test *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
				if request.URL.Path == "/health" {
					writer.WriteHeader(http.StatusOK)
					return
				}
				writer.WriteHeader(http.StatusOK)
			}))
			defer server.Close()

			rep := &proxyAuthRepoStub{matchError: testCase.repositoryError}
			proxy := newTestProxy(server)
			proxy.rep = rep
			recorder := httptest.NewRecorder()
			request := httptest.NewRequest(http.MethodPost, "/bot", strings.NewReader(`{"name":"simple_bot","action":"game_over","user_key":"game-2","user_win":true}`))
			proxy.legacyProxyHandler().ServeHTTP(recorder, request)
			if recorder.Code != testCase.wantStatus || rep.matchCalls != 1 || !rep.lastUserWin {
				test.Errorf("status=%d match calls=%d userWin=%v; want status=%d, one saved win", recorder.Code, rep.matchCalls, rep.lastUserWin, testCase.wantStatus)
			}
		})
	}
}

func TestProxyHandlerPropagatesLifecycleActionErrors(test *testing.T) {
	tests := []struct {
		name       string
		action     string
		upstream   int
		wantStatus int
	}{
		{name: "start success", action: "start_game", upstream: http.StatusOK, wantStatus: http.StatusCreated},
		{name: "start failure", action: "start_game", upstream: http.StatusInternalServerError, wantStatus: http.StatusBadGateway},
		{name: "set result success", action: "set_result", upstream: http.StatusOK, wantStatus: http.StatusOK},
		{name: "set result failure", action: "set_result", upstream: http.StatusInternalServerError, wantStatus: http.StatusBadGateway},
	}
	for _, testCase := range tests {
		test.Run(testCase.name, func(test *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
				if request.URL.Path == "/health" {
					writer.WriteHeader(http.StatusOK)
					return
				}
				writer.WriteHeader(testCase.upstream)
			}))
			defer server.Close()
			proxy := newTestProxy(server)
			proxy.rep = &proxyAuthRepoStub{}
			recorder := httptest.NewRecorder()
			body := `{"name":"simple_bot","action":"` + testCase.action + `","user_key":"game-1"}`
			request := httptest.NewRequest(http.MethodPost, "/bot", strings.NewReader(body))
			proxy.legacyProxyHandler().ServeHTTP(recorder, request)
			if recorder.Code != testCase.wantStatus {
				test.Errorf("status = %d; want %d", recorder.Code, testCase.wantStatus)
			}
		})
	}
}

func TestProxyHandlerListsConfiguredBotsWithoutBotName(test *testing.T) {
	proxy := &Proxy{
		bots: map[string]config.BotConfig{
			"smart_bot":  {URL: "http://smart_bot:28080"},
			"simple_bot": {URL: "http://simple_bot:28080"},
			"ai_bot":     {URL: "http://ai_bot:28080"},
		},
		logger: slog.New(slog.NewTextHandler(io.Discard, nil)),
	}
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPost, "/bot", strings.NewReader(`{"action":"list_bots"}`))

	proxy.ProxyHandler().ServeHTTP(recorder, request.WithContext(context.WithValue(request.Context(), userContextKey{}, "test-user")))

	if recorder.Code != http.StatusOK {
		test.Fatalf("status = %d; want 200: %s", recorder.Code, recorder.Body.String())
	}
	var response struct {
		Bots []string `json:"bots"`
	}
	if err := json.Unmarshal(recorder.Body.Bytes(), &response); err != nil {
		test.Fatalf("decode bot list: %v", err)
	}
	want := []string{"ai_bot", "simple_bot", "smart_bot"}
	if !reflect.DeepEqual(response.Bots, want) {
		test.Fatalf("bots = %v; want %v", response.Bots, want)
	}
}

func TestProxyHandlerStartsGameWithConfiguredBot(test *testing.T) {
	botStarted := false
	botServer := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		switch request.URL.Path {
		case "/health":
			writer.WriteHeader(http.StatusOK)
		case "/start":
			botStarted = true
			writer.WriteHeader(http.StatusOK)
		default:
			http.NotFound(writer, request)
		}
	}))
	defer botServer.Close()

	proxy := newTestProxy(botServer)
	proxy.rep = &proxyAuthRepoStub{}
	requestBody, err := json.Marshal(botRequest{
		Action:      "start_game",
		BotID:       "simple_bot",
		PlayerField: testFleet(),
	})
	if err != nil {
		test.Fatal(err)
	}
	request := httptest.NewRequest(http.MethodPost, "/bot", strings.NewReader(string(requestBody)))
	request = request.WithContext(context.WithValue(request.Context(), userContextKey{}, "captain"))
	recorder := httptest.NewRecorder()

	proxy.ProxyHandler().ServeHTTP(recorder, request)

	if recorder.Code != http.StatusCreated || !botStarted {
		test.Fatalf("status=%d botStarted=%v body=%q", recorder.Code, botStarted, recorder.Body.String())
	}
	var response gameResponse
	if err := json.Unmarshal(recorder.Body.Bytes(), &response); err != nil {
		test.Fatalf("decode game response: %v", err)
	}
	if response.SessionID == "" || !response.YourTurn || len(response.PlayerField.Ships) != len(fleetSizes) {
		test.Fatalf("unexpected initial game response: %+v", response)
	}
	if strings.Contains(recorder.Body.String(), "enemyField") {
		test.Fatal("game response exposed the opponent's field")
	}
}

func TestSendBotResultUsesConfiguredBotRequestShape(test *testing.T) {
	tests := []struct {
		name          string
		botID         string
		wantObject    bool
		expectedValue int
	}{
		{name: "simple bot uses numeric result", botID: "simple_bot", expectedValue: int(my_types.Hit)},
		{name: "AI bot uses numeric result", botID: "ai_bot", expectedValue: int(my_types.Hit)},
		{name: "smart bot includes session key", botID: "smart_bot", wantObject: true, expectedValue: int(my_types.Hit)},
	}
	for _, testCase := range tests {
		test.Run(testCase.name, func(test *testing.T) {
			var requestBody []byte
			server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
				if request.URL.Path != "/set_result" {
					test.Errorf("request path = %q; want /set_result", request.URL.Path)
				}
				requestBody, _ = io.ReadAll(request.Body)
				writer.WriteHeader(http.StatusOK)
			}))
			defer server.Close()

			proxy := &Proxy{client: server.Client()}
			if err := proxy.sendBotResult(context.Background(), testCase.botID, server.URL, "session-1", my_types.Hit); err != nil {
				test.Fatalf("sendBotResult() error = %v", err)
			}
			if testCase.wantObject {
				var payload struct {
					UserKey string `json:"user_key"`
					Result  int    `json:"result"`
				}
				if err := json.Unmarshal(requestBody, &payload); err != nil {
					test.Fatalf("decode smart bot payload: %v", err)
				}
				if payload.UserKey != "session-1" || payload.Result != testCase.expectedValue {
					test.Fatalf("payload = %+v; want session-1 and result %d", payload, testCase.expectedValue)
				}
				return
			}

			var payload int
			if err := json.Unmarshal(requestBody, &payload); err != nil {
				test.Fatalf("decode numeric bot payload %q: %v", requestBody, err)
			}
			if payload != testCase.expectedValue {
				test.Fatalf("payload = %d; want %d", payload, testCase.expectedValue)
			}
		})
	}
}

func testFleet() [][]int {
	field := emptyField()
	place := func(x, y, size int) {
		for offset := 0; offset < size; offset++ {
			field[x][y+offset] = my_types.SHIP
		}
	}
	place(0, 0, 4)
	place(2, 0, 3)
	place(4, 0, 3)
	place(6, 0, 2)
	place(8, 0, 2)
	place(0, 5, 2)
	place(2, 5, 1)
	place(4, 5, 1)
	place(6, 5, 1)
	place(8, 5, 1)
	return field
}

func TestAuthenticationHandlers(test *testing.T) {
	test.Setenv("JWT_SECRET", "01234567890123456789012345678901")
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	rep := &proxyAuthRepoStub{}
	auth, err := mw.New(time.Minute, logger, rep)
	if err != nil {
		test.Fatalf("New() error = %v", err)
	}

	registerRecorder := httptest.NewRecorder()
	registerRequest := httptest.NewRequest(http.MethodPost, "/register", strings.NewReader(`{"name":"captain","password":"secret"}`))
	RegisterHandler(auth).ServeHTTP(registerRecorder, registerRequest)
	if registerRecorder.Code != http.StatusCreated || rep.registeredName != "captain" {
		test.Fatalf("register status=%d name=%q", registerRecorder.Code, rep.registeredName)
	}
	if err := bcrypt.CompareHashAndPassword([]byte(rep.registeredHash), []byte("secret")); err != nil {
		test.Fatalf("registered password is not hashed: %v", err)
	}
	rep.passwordHash = rep.registeredHash

	loginRecorder := httptest.NewRecorder()
	loginRequest := httptest.NewRequest(http.MethodPost, "/login", strings.NewReader(`{"name":"captain","password":"secret"}`))
	LoginHandler(auth).ServeHTTP(loginRecorder, loginRequest)
	if loginRecorder.Code != http.StatusOK {
		test.Fatalf("login status = %d; want 200", loginRecorder.Code)
	}
	var response AuthResponse
	if err := json.Unmarshal(loginRecorder.Body.Bytes(), &response); err != nil || response.Token == "" {
		test.Fatalf("login response = %q, error = %v", loginRecorder.Body.String(), err)
	}

	verifyRecorder := httptest.NewRecorder()
	verifyRequest := httptest.NewRequest(http.MethodGet, "/verify", nil)
	verifyRequest.Header.Set("Authorization", "Bearer "+response.Token)
	VerifyHandler(auth).ServeHTTP(verifyRecorder, verifyRequest)
	if verifyRecorder.Code != http.StatusOK || !strings.Contains(verifyRecorder.Body.String(), `"user":"captain"`) {
		test.Fatalf("verify status=%d body=%q", verifyRecorder.Code, verifyRecorder.Body.String())
	}

	protectedRecorder := httptest.NewRecorder()
	protectedRequest := httptest.NewRequest(http.MethodGet, "/protected", nil)
	protectedRequest.Header.Set("Authorization", "Bearer "+response.Token)
	protected := &Proxy{logger: logger}
	protected.AuthHandler(auth, http.HandlerFunc(func(writer http.ResponseWriter, _ *http.Request) {
		writer.WriteHeader(http.StatusNoContent)
	})).ServeHTTP(protectedRecorder, protectedRequest)
	if protectedRecorder.Code != http.StatusNoContent {
		test.Errorf("protected status = %d; want 204", protectedRecorder.Code)
	}
}

func TestAuthenticationHandlersRejectBadRequests(test *testing.T) {
	test.Setenv("JWT_SECRET", "01234567890123456789012345678901")
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	auth, err := mw.New(time.Minute, logger, &proxyAuthRepoStub{})
	if err != nil {
		test.Fatalf("New() error = %v", err)
	}

	registerRecorder := httptest.NewRecorder()
	RegisterHandler(auth).ServeHTTP(registerRecorder, httptest.NewRequest(http.MethodPost, "/register", strings.NewReader("{")))
	if registerRecorder.Code != http.StatusBadRequest {
		test.Errorf("invalid register status = %d; want 400", registerRecorder.Code)
	}
	loginRecorder := httptest.NewRecorder()
	LoginHandler(auth).ServeHTTP(loginRecorder, httptest.NewRequest(http.MethodPost, "/login", strings.NewReader("{")))
	if loginRecorder.Code != http.StatusBadRequest {
		test.Errorf("invalid login status = %d; want 400", loginRecorder.Code)
	}
	verifyRecorder := httptest.NewRecorder()
	VerifyHandler(auth).ServeHTTP(verifyRecorder, httptest.NewRequest(http.MethodGet, "/verify", nil))
	if verifyRecorder.Code != http.StatusUnauthorized {
		test.Errorf("missing token status = %d; want 401", verifyRecorder.Code)
	}
	loginDeniedRecorder := httptest.NewRecorder()
	LoginHandler(auth).ServeHTTP(loginDeniedRecorder, httptest.NewRequest(http.MethodPost, "/login", strings.NewReader(`{"name":"unknown","password":"secret"}`)))
	if loginDeniedRecorder.Code != http.StatusUnauthorized {
		test.Errorf("invalid credentials status = %d; want 401", loginDeniedRecorder.Code)
	}
}

func (rep *proxyAuthRepoStub) RegisterUser(_ context.Context, name, passwordHash string) error {
	rep.registeredName = name
	rep.registeredHash = passwordHash
	return nil
}

func (rep *proxyAuthRepoStub) GetPasswordHash(context.Context, string) (string, error) {
	return rep.passwordHash, nil
}

func (rep *proxyAuthRepoStub) FindOrCreateGoogleUser(context.Context, string, string) (int64, string, error) {
	return 1, "google@example.com", nil
}

func (rep *proxyAuthRepoStub) SetResult(_ context.Context, _ string, userWin bool) error {
	rep.matchCalls++
	rep.lastUserWin = userWin
	return rep.matchError
}

func newTestProxy(server *httptest.Server) *Proxy {
	return &Proxy{
		client: server.Client(),
		bots:   map[string]config.BotConfig{"simple_bot": {URL: server.URL}},
		logger: slog.New(slog.NewTextHandler(io.Discard, nil)),
	}
}

func TestProxyHandlerValidatesRequestAndBotName(test *testing.T) {
	server := httptest.NewServer(http.NotFoundHandler())
	defer server.Close()
	handler := newTestProxy(server).legacyProxyHandler()

	tests := []struct {
		name       string
		body       string
		wantStatus int
	}{
		{name: "invalid JSON", body: "{", wantStatus: http.StatusBadRequest},
		{name: "unknown bot", body: `{"name":"unknown","action":"shoot"}`, wantStatus: http.StatusBadRequest},
	}
	for _, testCase := range tests {
		test.Run(testCase.name, func(test *testing.T) {
			recorder := httptest.NewRecorder()
			request := httptest.NewRequest(http.MethodPost, "/bot", strings.NewReader(testCase.body))
			handler.ServeHTTP(recorder, request)
			if recorder.Code != testCase.wantStatus {
				test.Errorf("status = %d; want %d", recorder.Code, testCase.wantStatus)
			}
		})
	}
}

func TestProxyHandlerForwardsSuccessfulBotAction(test *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		switch request.URL.Path {
		case "/health":
			writer.WriteHeader(http.StatusOK)
		case "/shoot":
			_, _ = writer.Write([]byte(`{"x":2,"y":5}`))
		default:
			http.NotFound(writer, request)
		}
	}))
	defer server.Close()
	handler := newTestProxy(server).legacyProxyHandler()
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPost, "/bot", strings.NewReader(`{"name":"simple_bot","action":"shoot","user_key":"game-1","field":[[0]]}`))
	handler.ServeHTTP(recorder, request)
	if recorder.Code != http.StatusOK || !strings.Contains(recorder.Body.String(), `"x":2`) || !strings.Contains(recorder.Body.String(), `"y":5`) {
		test.Fatalf("status=%d body=%q", recorder.Code, recorder.Body.String())
	}
}

func TestProxyHandlerRejectsUnhealthyBot(test *testing.T) {
	shootCalled := false
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		if request.URL.Path == "/health" {
			writer.WriteHeader(http.StatusServiceUnavailable)
			return
		}
		shootCalled = true
	}))
	defer server.Close()
	handler := newTestProxy(server).legacyProxyHandler()
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPost, "/bot", strings.NewReader(`{"name":"simple_bot","action":"shoot"}`))
	handler.ServeHTTP(recorder, request)
	if recorder.Code != http.StatusServiceUnavailable || shootCalled {
		test.Errorf("status=%d actionCalled=%v; want 503 and no action", recorder.Code, shootCalled)
	}
}

func TestProxyHandlerRejectsBotActionErrors(test *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		if request.URL.Path == "/health" {
			return
		}
		writer.WriteHeader(http.StatusInternalServerError)
		_, _ = writer.Write([]byte(`{"x":2,"y":5}`))
	}))
	defer server.Close()
	handler := newTestProxy(server).legacyProxyHandler()
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPost, "/bot", strings.NewReader(`{"name":"simple_bot","action":"shoot"}`))
	handler.ServeHTTP(recorder, request)
	if recorder.Code != http.StatusBadGateway {
		test.Errorf("status = %d; want %d", recorder.Code, http.StatusBadGateway)
	}
}

func TestBearerToken(test *testing.T) {
	tests := []struct {
		name      string
		header    string
		wantToken string
		wantError bool
	}{
		{name: "missing", wantError: true},
		{name: "missing scheme", header: "token", wantError: true},
		{name: "wrong scheme", header: "Basic token", wantError: true},
		{name: "empty token", header: "Bearer ", wantError: true},
		{name: "extra fields", header: "Bearer token extra", wantError: true},
		{name: "valid case insensitive scheme", header: "bearer token", wantToken: "token"},
	}
	for _, testCase := range tests {
		test.Run(testCase.name, func(test *testing.T) {
			token, err := bearerToken(testCase.header)
			if (err != nil) != testCase.wantError || token != testCase.wantToken {
				test.Errorf("bearerToken(%q) = %q, %v; want %q, error=%v", testCase.header, token, err, testCase.wantToken, testCase.wantError)
			}
		})
	}
}
