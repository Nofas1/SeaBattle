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
	protected := (&MyProxy{logger: logger}).AuthHandler(auth, http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		if request.Context().Value(userContextKey{}) != "captain" {
			test.Errorf("authenticated username was not attached to request context")
		}
		writer.WriteHeader(http.StatusNoContent)
	}))
	protected.ServeHTTP(protectedRecorder, protectedRequest)
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
}

func TestValidatePlayerFleet(test *testing.T) {
	field := validFleet()
	ships, err := validatePlayerField(field)
	if err != nil || len(ships) != 10 {
		test.Fatalf("valid fleet rejected: ships=%d error=%v", len(ships), err)
	}
	field[1][4] = 1
	if _, err := validatePlayerField(field); err == nil || !strings.Contains(err.Error(), "touch") {
		test.Fatalf("diagonally touching ships accepted: %v", err)
	}
	if isStraightShip([]point{{X: 1, Y: 1}, {X: 1, Y: 2}, {X: 2, Y: 2}}) {
		test.Fatal("bent ship accepted")
	}
}

func TestGameSessionValidatesShotsAndDoesNotExposeEnemyField(test *testing.T) {
	botServer := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		switch request.URL.Path {
		case "/health", "/start", "/game_over":
			writer.WriteHeader(http.StatusOK)
		case "/shoot":
			_, _ = io.WriteString(writer, `{"x":0,"y":0}`)
		case "/set_result":
			writer.WriteHeader(http.StatusOK)
		default:
			http.NotFound(writer, request)
		}
	}))
	defer botServer.Close()

func (rep *proxyAuthRepoStub) FindOrCreateGoogleUser(context.Context, string, string) (int64, string, error) {
	return 1, "google@example.com", nil
}

func (rep *proxyAuthRepoStub) SetResult(_ context.Context, _ string, userWin bool) error {
	rep.matchCalls++
	rep.lastUserWin = userWin
	return rep.matchError
}

	game := proxy.sessions[state.SessionID]
	target := point{}
	found := false
	for _, ship := range game.enemyShips {
		if len(ship.Cells) == 1 {
			target = ship.Cells[0]
			found = true
			break
		}
	}
	if !found {
		test.Fatal("generated fleet has no patrol boats")
	}
	shotRequest := botRequest{Action: "shoot", SessionID: state.SessionID, X: intPointer(target.X), Y: intPointer(target.Y)}
	first := serveAsUser(proxy, "captain", shotRequest)
	if first.Code != http.StatusOK {
		test.Fatalf("first shot status=%d body=%q", first.Code, first.Body.String())
	}
	var shotResponse gameResponse
	if err := json.Unmarshal(first.Body.Bytes(), &shotResponse); err != nil {
		test.Fatal(err)
	}
	if shotResponse.PlayerShot == nil || shotResponse.PlayerShot.Result != "Sink" || len(shotResponse.PlayerShot.SunkShip) != 1 {
		test.Fatalf("expected a sunk single-cell ship in response: %+v", shotResponse.PlayerShot)
	}
	if !shotResponse.YourTurn {
		test.Fatal("a player hit should keep the player's turn")
	}
	for _, around := range shotResponse.PlayerShot.AroundSunk {
		if around.X < 0 || around.X >= 10 || around.Y < 0 || around.Y >= 10 {
			test.Fatalf("out-of-bounds around-sunk cell: %+v", around)
		}
	}

	before := len(game.playerShots)
	duplicate := serveAsUser(proxy, "captain", shotRequest)
	if duplicate.Code != http.StatusBadRequest || !strings.Contains(duplicate.Body.String(), "already") {
		test.Fatalf("duplicate shot status=%d body=%q", duplicate.Code, duplicate.Body.String())
	}
	if len(game.playerShots) != before {
		test.Fatal("invalid duplicate shot changed session state")
	}

	outOfRange := serveAsUser(proxy, "captain", botRequest{
		Action: "shoot", SessionID: state.SessionID, X: intPointer(10), Y: intPointer(0),
	})
	if outOfRange.Code != http.StatusBadRequest || len(game.playerShots) != before {
		test.Fatalf("out-of-range shot status=%d and shots=%d", outOfRange.Code, len(game.playerShots))
	}
	missingCoordinates := serveAsUser(proxy, "captain", botRequest{
		Action: "shoot", SessionID: state.SessionID,
	})
	if missingCoordinates.Code != http.StatusBadRequest || len(game.playerShots) != before {
		test.Fatalf("missing coordinates status=%d and shots=%d", missingCoordinates.Code, len(game.playerShots))
	}
}

func TestProxyHandlerValidatesRequestAndBotName(test *testing.T) {
	server := httptest.NewServer(http.NotFoundHandler())
	defer server.Close()
	handler := newTestProxy(server).legacyProxyHandler()

func TestOutOfTurnAndForeignSessionAreRejectedWithoutMutation(test *testing.T) {
	proxy := &MyProxy{logger: slog.New(slog.NewTextHandler(io.Discard, nil)), sessions: make(map[string]*gameSession)}
	field := validFleet()
	ships, err := validatePlayerField(field)
	if err != nil {
		test.Fatal(err)
	}
	enemyField, enemyShips, err := generateEnemyFleet()
	if err != nil {
		test.Fatal(err)
	}
	session := &gameSession{
		id: "session-1", user: "captain", botID: "simple_bot",
		playerField: field, playerShips: ships, enemyField: enemyField, enemyShips: enemyShips,
		playerShots: make(map[string]string), enemyShots: make(map[string]string),
		yourTurn: false, expiresAt: time.Now().Add(time.Hour),
	}
	proxy.sessions[session.id] = session
	request := botRequest{Action: "shoot", SessionID: session.id, X: intPointer(0), Y: intPointer(0)}
	response := serveAsUser(proxy, "captain", request)
	if response.Code != http.StatusBadRequest || len(session.playerShots) != 0 {
		test.Fatalf("out-of-turn shot status=%d body=%q", response.Code, response.Body.String())
	}
	foreign := serveAsUser(proxy, "intruder", request)
	if foreign.Code != http.StatusNotFound || len(session.playerShots) != 0 {
		test.Fatalf("foreign session status=%d body=%q", foreign.Code, foreign.Body.String())
	}
}

func TestMissRunsOneBotShotAndStateAdvancesRemainingBotTurn(test *testing.T) {
	var botShotIndex int
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		switch request.URL.Path {
		case "/health", "/start", "/set_result", "/game_over":
			writer.WriteHeader(http.StatusOK)
		case "/shoot":
			targets := []point{{X: 0, Y: 0}, {X: 0, Y: 1}}
			target := targets[botShotIndex]
			botShotIndex++
			_ = json.NewEncoder(writer).Encode(target)
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

func TestBotSinkCommitsOnlyAfterResultIsRecorded(test *testing.T) {
	var gotResult my_types.ShotResult
	var session *gameSession
	var resultObservedBeforeCommit bool
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		switch request.URL.Path {
		case "/shoot":
			_ = json.NewEncoder(writer).Encode(point{X: 0, Y: 1})
		case "/set_result":
			var body struct {
				Result my_types.ShotResult `json:"result"`
			}
			if err := json.NewDecoder(request.Body).Decode(&body); err != nil {
				test.Errorf("decode bot result: %v", err)
				writer.WriteHeader(http.StatusBadRequest)
				return
			}
			gotResult = body.Result
			resultObservedBeforeCommit = len(session.enemyShots) == 0 &&
				session.playerField[0][1] == my_types.SHIP
		default:
			http.NotFound(writer, request)
		}
	}))
	defer server.Close()
	handler := newTestProxy(server).legacyProxyHandler()
	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, request)
	if recorder.Code != http.StatusNoContent ||
		recorder.Header().Get("Access-Control-Allow-Origin") != "http://localhost:8080" ||
		!strings.Contains(recorder.Header().Get("Access-Control-Allow-Headers"), "Authorization") {
		test.Fatalf("unexpected CORS preflight: status=%d headers=%v", recorder.Code, recorder.Header())
	}

	rejected := httptest.NewRequest(http.MethodOptions, "/bot", nil)
	rejected.Header.Set("Origin", "https://evil.example")
	rejectedRecorder := httptest.NewRecorder()
	handler.ServeHTTP(rejectedRecorder, rejected)
	if rejectedRecorder.Code != http.StatusForbidden {
		test.Fatalf("untrusted origin status=%d; want 403", rejectedRecorder.Code)
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
	proxy.ProxyHandler().ServeHTTP(recorder, request)
	return recorder
}

func mustJSON(test *testing.T, value any) io.Reader {
	body, err := json.Marshal(value)
	if err != nil {
		if test != nil {
			test.Fatal(err)
		}
		panic(err)
	}
	return strings.NewReader(string(body))
}

func intPointer(value int) *int {
	return &value
}

func validFleet() [][]int {
	field := emptyField()
	place := func(x, y, size, dx, dy int) {
		for offset := 0; offset < size; offset++ {
			field[x+offset*dx][y+offset*dy] = 1
		}
	}
	place(0, 0, 4, 0, 1)
	place(2, 0, 3, 0, 1)
	place(4, 0, 3, 0, 1)
	place(6, 0, 2, 0, 1)
	place(8, 0, 2, 0, 1)
	place(0, 5, 2, 0, 1)
	place(2, 5, 1, 0, 1)
	place(4, 5, 1, 0, 1)
	place(6, 5, 1, 0, 1)
	place(8, 5, 1, 0, 1)
	return field
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

func TestSaveGameResultRetriesRepositoryFailures(test *testing.T) {
	repository := &proxyAuthRepoStub{matchError: errors.New("database unavailable")}
	proxy := &MyProxy{
		rep:    repository,
		logger: slog.New(slog.NewTextHandler(io.Discard, nil)),
	}
	winner := "player"
	session := &gameSession{id: "match-1", user: "captain", winner: &winner, gameOver: true}
	proxy.saveGameResult(session)
	if session.resultSaved || repository.matchCalls != 1 {
		test.Fatalf("failed persistence should remain retryable: saved=%v calls=%d", session.resultSaved, repository.matchCalls)
	}
}

func TestSaveGameResultRecordsWinnerOnce(test *testing.T) {
	repository := &proxyAuthRepoStub{}
	proxy := &MyProxy{
		rep:    repository,
		logger: slog.New(slog.NewTextHandler(io.Discard, nil)),
	}
	winner := "player"
	session := &gameSession{id: "match-2", user: "captain", winner: &winner, gameOver: true}
	proxy.saveGameResult(session)
	proxy.saveGameResult(session)
	if !session.resultSaved || repository.matchCalls != 1 || !repository.lastUserWin {
		test.Fatalf("game result saved=%v calls=%d user_win=%v",
			session.resultSaved, repository.matchCalls, repository.lastUserWin)
	}
}
