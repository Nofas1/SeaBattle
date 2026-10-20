package main

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
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

func (rep *proxyAuthRepoStub) RegisterUser(_ context.Context, name, passwordHash string) error {
	rep.registeredName = name
	rep.registeredHash = passwordHash
	return nil
}

func (rep *proxyAuthRepoStub) GetPasswordHash(context.Context, string) (string, error) {
	return rep.passwordHash, nil
}

func (rep *proxyAuthRepoStub) SetResult(_ context.Context, _ string, userWin bool) error {
	rep.matchCalls++
	rep.lastUserWin = userWin
	return rep.matchError
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

	proxy := newTestProxy(botServer)
	proxy.rep = &proxyAuthRepoStub{}
	start := httptest.NewRequest(http.MethodPost, "/bot", mustJSON(test, botRequest{
		Action:      "start_game",
		BotID:       "simple_bot",
		PlayerField: validFleet(),
	}))
	start = start.WithContext(context.WithValue(start.Context(), userContextKey{}, "captain"))
	recorder := httptest.NewRecorder()
	proxy.ProxyHandler().ServeHTTP(recorder, start)
	if recorder.Code != http.StatusCreated {
		test.Fatalf("start status=%d body=%q", recorder.Code, recorder.Body.String())
	}
	if strings.Contains(recorder.Body.String(), "enemyField") || strings.Contains(recorder.Body.String(), "botField") {
		test.Fatalf("start response exposes the hidden enemy field: %s", recorder.Body.String())
	}
	var state gameResponse
	if err := json.Unmarshal(recorder.Body.Bytes(), &state); err != nil {
		test.Fatal(err)
	}
	if state.SessionID == "" || !state.YourTurn || state.GameOver || state.Winner != nil || len(state.PlayerField.Ships) != 10 {
		test.Fatalf("unexpected initial game state: %+v", state)
	}
	if len(state.PlayerShotsOnEnemy) != 0 || len(state.EnemyShotsOnPlayer) != 0 {
		test.Fatal("initial state should have empty shot histories")
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

func TestLastEnemyShipSunkEndsGameAndSavesWinner(test *testing.T) {
	field := validFleet()
	ships, err := validatePlayerField(field)
	if err != nil {
		test.Fatal(err)
	}
	targetShip := -1
	for index, ship := range ships {
		if len(ship.Cells) == 1 {
			targetShip = index
			break
		}
	}
	if targetShip < 0 {
		test.Fatal("valid fleet has no single-cell ship")
	}
	for index := range ships {
		if index == targetShip {
			continue
		}
		ships[index].Sunk = true
		for _, cell := range ships[index].Cells {
			field[cell.X][cell.Y] = 2
		}
	}
	target := ships[targetShip].Cells[0]
	field[target.X][target.Y] = 1
	session := &gameSession{
		id: "final-shot", user: "captain", botID: "simple_bot",
		enemyField: field, enemyShips: ships,
		playerField: emptyField(), playerShips: nil,
		playerShots: make(map[string]string), enemyShots: make(map[string]string),
		yourTurn: true, expiresAt: time.Now().Add(time.Hour),
	}
	proxy := &MyProxy{
		rep: &proxyAuthRepoStub{}, logger: slog.New(slog.NewTextHandler(io.Discard, nil)),
		bots: make(map[string]config.BotConfig), sessions: map[string]*gameSession{session.id: session},
	}
	recorder := serveAsUser(proxy, "captain", botRequest{
		Action: "shoot", SessionID: session.id,
		X: intPointer(target.X), Y: intPointer(target.Y),
	})
	if recorder.Code != http.StatusOK {
		test.Fatalf("final shot status=%d body=%q", recorder.Code, recorder.Body.String())
	}
	var response gameResponse
	if err := json.Unmarshal(recorder.Body.Bytes(), &response); err != nil {
		test.Fatal(err)
	}
	repository := proxy.rep.(*proxyAuthRepoStub)
	if !response.GameOver || response.Winner == nil || *response.Winner != "player" ||
		response.PlayerShot == nil || response.PlayerShot.Result != "Sink" ||
		repository.matchCalls != 1 || !repository.lastUserWin {
		test.Fatalf("unexpected final response=%+v match_calls=%d win=%v",
			response, repository.matchCalls, repository.lastUserWin)
	}
}

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

	proxy := newTestProxy(server)
	start := serveAsUser(proxy, "captain", botRequest{
		Action: "start_game", BotID: "simple_bot", PlayerField: validFleet(),
	})
	if start.Code != http.StatusCreated {
		test.Fatalf("start status=%d body=%q", start.Code, start.Body.String())
	}
	var initial gameResponse
	if err := json.Unmarshal(start.Body.Bytes(), &initial); err != nil {
		test.Fatal(err)
	}
	session := proxy.sessions[initial.SessionID]

	miss := point{}
	foundMiss := false
	for x := 0; x < my_types.Size && !foundMiss; x++ {
		for y := 0; y < my_types.Size; y++ {
			if session.enemyField[x][y] == my_types.EMPTY {
				miss = point{X: x, Y: y}
				foundMiss = true
				break
			}
		}
	}
	if !foundMiss {
		test.Fatal("generated enemy fleet has no empty cells")
	}
	playerTurn := serveAsUser(proxy, "captain", botRequest{
		Action: "shoot", SessionID: initial.SessionID, X: intPointer(miss.X), Y: intPointer(miss.Y),
	})
	if playerTurn.Code != http.StatusOK {
		test.Fatalf("player shot status=%d body=%q", playerTurn.Code, playerTurn.Body.String())
	}
	var firstResponse gameResponse
	if err := json.Unmarshal(playerTurn.Body.Bytes(), &firstResponse); err != nil {
		test.Fatal(err)
	}
	if firstResponse.PlayerShot == nil || firstResponse.PlayerShot.Result != "Miss" ||
		firstResponse.BotShot == nil || firstResponse.BotShot.Result != "Hit" ||
		firstResponse.YourTurn {
		test.Fatalf("unexpected state after one bot response: %+v", firstResponse)
	}
	if botShotIndex != 1 {
		test.Fatalf("one player miss caused %d bot shots; want exactly one", botShotIndex)
	}

	continued := serveAsUser(proxy, "captain", botRequest{Action: "state", SessionID: initial.SessionID})
	if continued.Code != http.StatusOK {
		test.Fatalf("state status=%d body=%q", continued.Code, continued.Body.String())
	}
	var continuedResponse gameResponse
	if err := json.Unmarshal(continued.Body.Bytes(), &continuedResponse); err != nil {
		test.Fatal(err)
	}
	if continuedResponse.BotShot == nil || continuedResponse.BotShot.X != 0 ||
		continuedResponse.BotShot.Y != 1 || continuedResponse.YourTurn {
		test.Fatalf("state action did not advance the bot turn: %+v", continuedResponse)
	}
	if len(continuedResponse.EnemyShotsOnPlayer) != 2 || botShotIndex != 2 {
		test.Fatalf("bot history length=%d bot shots=%d; want 2 each",
			len(continuedResponse.EnemyShotsOnPlayer), botShotIndex)
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

	field := emptyField()
	field[0][0] = my_types.SHOOTED
	field[0][1] = my_types.SHIP
	session = &gameSession{
		id:          "game-1",
		botID:       "simple_bot",
		playerField: field,
		playerShips: []shipState{{Cells: []point{{X: 0, Y: 0}, {X: 0, Y: 1}}}},
		enemyShots:  make(map[string]string),
	}
	proxy := newTestProxy(server)
	shot, err := proxy.applyBotShot(context.Background(), session)
	if err != nil {
		test.Fatalf("applyBotShot() error = %v", err)
	}
	if !resultObservedBeforeCommit || gotResult != my_types.Sink || shot == nil || shot.Result != "Sink" ||
		len(shot.SunkShip) != 2 || session.winner == nil || *session.winner != "bot" {
		test.Fatalf("unexpected bot sink result: sent=%v shot=%+v winner=%v",
			gotResult, shot, session.winner)
	}
}

func TestCORSPreflightAllowsConfiguredFrontend(test *testing.T) {
	handler := cors(http.NotFoundHandler())
	request := httptest.NewRequest(http.MethodOptions, "/bot", nil)
	request.Header.Set("Origin", "http://localhost:8080")
	request.Header.Set("Access-Control-Request-Method", "POST")
	request.Header.Set("Access-Control-Request-Headers", "authorization,content-type")
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

func newTestProxy(server *httptest.Server) *MyProxy {
	return &MyProxy{
		client:   server.Client(),
		bots:     map[string]config.BotConfig{"simple_bot": {URL: server.URL}},
		logger:   slog.New(slog.NewTextHandler(io.Discard, nil)),
		sessions: make(map[string]*gameSession),
	}
}

func serveAsUser(proxy *MyProxy, username string, body botRequest) *httptest.ResponseRecorder {
	request := httptest.NewRequest(http.MethodPost, "/bot", mustJSON(nil, body))
	request = request.WithContext(context.WithValue(request.Context(), userContextKey{}, username))
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
