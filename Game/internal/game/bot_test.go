package game

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"sea_battle/Game/internal/domain"
)

func TestBotProxyStartGameUsesAuthenticatedProxyRoute(test *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		if request.Method != http.MethodPost || request.URL.Path != "/bot" {
			test.Errorf("request = %s %s; expected POST /bot", request.Method, request.URL.Path)
		}
		if request.Header.Get("Authorization") != "Bearer test-token" {
			test.Errorf("Authorization = %q; expected Bearer test-token", request.Header.Get("Authorization"))
		}
		if request.Header.Get("Content-Type") != "application/json" {
			test.Errorf("Content-Type = %q; expected application/json", request.Header.Get("Content-Type"))
		}

		var body ProxyRequest
		if err := json.NewDecoder(request.Body).Decode(&body); err != nil {
			test.Errorf("decode request: %v", err)
			return
		}
		if body.Action != "start_game" || body.Name != "smart_bot" || body.UserKey == "" {
			test.Errorf("request body = %+v; expected start_game for smart_bot with a user key", body)
		}
		writer.WriteHeader(http.StatusCreated)
	}))
	defer server.Close()

	bot := NewBotProxy(domain.Constructor(), server.URL, "smart_bot", nil, "test-token")
	bot.client = server.Client()
	if err := bot.StartGame(); err != nil {
		test.Fatalf("StartGame() error = %v", err)
	}
}

func TestBotProxyGameOverUsesAuthenticatedProxyRoute(test *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		if request.Method != http.MethodPost || request.URL.Path != "/bot" {
			test.Errorf("request = %s %s; expected POST /bot", request.Method, request.URL.Path)
		}
		if request.Header.Get("Authorization") != "Bearer test-token" {
			test.Errorf("Authorization = %q; expected Bearer test-token", request.Header.Get("Authorization"))
		}

		var body ProxyRequest
		if err := json.NewDecoder(request.Body).Decode(&body); err != nil {
			test.Errorf("decode request: %v", err)
			return
		}
		if body.Action != "game_over" || body.Name != "smart_bot" || !body.UserWin || body.UserKey == "" {
			test.Errorf("request body = %+v; expected winning game_over for smart_bot with a user key", body)
		}
		writer.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	bot := NewBotProxy(domain.Constructor(), server.URL, "smart_bot", nil, "test-token")
	bot.client = server.Client()
	if err := bot.GameOver(true); err != nil {
		test.Fatalf("GameOver(true) error = %v", err)
	}
}

func TestBotProxyLifecycleMethodsRejectHTTPErrors(test *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, _ *http.Request) {
		http.Error(writer, "unauthorized", http.StatusUnauthorized)
	}))
	defer server.Close()

	bot := NewBotProxy(domain.Constructor(), server.URL, "smart_bot", nil, "test-token")
	bot.client = server.Client()
	if err := bot.StartGame(); err == nil {
		test.Fatal("StartGame() error = nil; expected an HTTP status error")
	}
	if err := bot.GameOver(false); err == nil {
		test.Fatal("GameOver(false) error = nil; expected an HTTP status error")
	}
}

func TestBotProxyActionMethodsRejectHTTPErrors(test *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, _ *http.Request) {
		writer.WriteHeader(http.StatusBadGateway)
		_, _ = writer.Write([]byte(`{"x":1,"y":2,"dir":{"X":1,"Y":0}}`))
	}))
	defer server.Close()

	bot := NewBotProxy(domain.Constructor(), server.URL, "smart_bot", nil, "test-token")
	bot.client = server.Client()
	if _, err := bot.Shoot(); err == nil {
		test.Error("Shoot() error = nil; expected an HTTP status error")
	}
	if _, _, _, err := bot.Place(); err == nil {
		test.Error("Place() error = nil; expected an HTTP status error")
	}
	if err := bot.SetResult(1); err == nil {
		test.Error("SetResult() error = nil; expected an HTTP status error")
	}
}

func TestBotProxyActionMethodsDecodeSuccessfulResponses(test *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		if request.URL.Path != "/bot" || request.Method != http.MethodPost {
			test.Errorf("request = %s %s; want POST /bot", request.Method, request.URL.Path)
		}
		if request.Header.Get("Authorization") != "Bearer test-token" {
			test.Errorf("Authorization = %q; want Bearer test-token", request.Header.Get("Authorization"))
		}
		var body ProxyRequest
		if err := json.NewDecoder(request.Body).Decode(&body); err != nil {
			test.Errorf("decode request: %v", err)
			return
		}
		switch body.Action {
		case "shoot":
			_, _ = writer.Write([]byte(`{"X":2,"Y":3}`))
		case "place":
			_, _ = writer.Write([]byte(`{"x":4,"y":5,"dir":{"X":1,"Y":0}}`))
		case "set_result":
			if body.UserKey == "" {
				test.Error("set_result request has an empty user key")
			}
		default:
			test.Errorf("unexpected action %q", body.Action)
		}
	}))
	defer server.Close()

	bot := NewBotProxy(domain.Constructor(), server.URL, "smart_bot", nil, "test-token")
	bot.client = server.Client()
	shot, err := bot.Shoot()
	if err != nil || shot != (domain.Pair{X: 2, Y: 3}) {
		test.Errorf("Shoot() = %+v, %v; want (2, 3), nil", shot, err)
	}
	x, y, direction, err := bot.Place()
	if err != nil || x != 4 || y != 5 || direction != (domain.Pair{X: 1, Y: 0}) {
		test.Errorf("Place() = %d, %d, %+v, %v; want 4, 5, (1, 0), nil", x, y, direction, err)
	}
	if err := bot.SetResult(2); err != nil {
		test.Errorf("SetResult() error = %v", err)
	}
}
