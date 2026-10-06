package main

import (
	"errors"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"sea_battle/my_types"
)

type handlerBotStub struct {
	startError    error
	shootError    error
	gameOverError error
	startCalls    int
	shootCalls    int
	resultCalls   int
	gameOverCalls int
	lastUserKey   string
	lastResult    my_types.ShotResult
}

func (bot *handlerBotStub) Place() (int, int, my_types.Pair) {
	return 1, 2, my_types.Pair{X: 0, Y: 1}
}

func (bot *handlerBotStub) Shoot(userKey string) (my_types.Pair, error) {
	bot.shootCalls++
	bot.lastUserKey = userKey
	return my_types.Pair{X: 3, Y: 4}, bot.shootError
}

func (bot *handlerBotStub) SetResult(userKey string, result my_types.ShotResult) {
	bot.resultCalls++
	bot.lastUserKey = userKey
	bot.lastResult = result
}

func (bot *handlerBotStub) StartGame(userKey string) error {
	bot.startCalls++
	bot.lastUserKey = userKey
	return bot.startError
}

func (bot *handlerBotStub) GameOver(userKey string) error {
	bot.gameOverCalls++
	bot.lastUserKey = userKey
	return bot.gameOverError
}

func newTestHandler(bot *handlerBotStub) *Handler {
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	return NewHandler(bot, logger)
}

func TestStartGameHandler(t *testing.T) {
	tests := []struct {
		name       string
		body       string
		botError   error
		wantStatus int
		wantCalls  int
	}{
		{name: "invalid JSON", body: "{", wantStatus: http.StatusBadRequest},
		{name: "bot failure", body: `{"user_key":"game-1"}`, botError: errors.New("start failed"), wantStatus: http.StatusInternalServerError, wantCalls: 1},
		{name: "success", body: `{"user_key":"game-1"}`, wantStatus: http.StatusOK, wantCalls: 1},
	}
	for _, testCase := range tests {
		t.Run(testCase.name, func(t *testing.T) {
			bot := &handlerBotStub{startError: testCase.botError}
			recorder := httptest.NewRecorder()
			request := httptest.NewRequest(http.MethodPost, "/start", strings.NewReader(testCase.body))
			newTestHandler(bot).StartGameHandler().ServeHTTP(recorder, request)
			if recorder.Code != testCase.wantStatus || bot.startCalls != testCase.wantCalls {
				t.Fatalf("status=%d calls=%d; want status=%d calls=%d", recorder.Code, bot.startCalls, testCase.wantStatus, testCase.wantCalls)
			}
			if testCase.wantCalls > 0 && bot.lastUserKey != "game-1" {
				t.Errorf("user key = %q; want game-1", bot.lastUserKey)
			}
		})
	}
}

func TestShootHandler(t *testing.T) {
	tests := []struct {
		name       string
		body       string
		botError   error
		wantStatus int
		wantCalls  int
	}{
		{name: "invalid JSON", body: "{", wantStatus: http.StatusBadRequest},
		{name: "bot failure", body: `{"user_key":"game-1"}`, botError: errors.New("shoot failed"), wantStatus: http.StatusInternalServerError, wantCalls: 1},
		{name: "success", body: `{"user_key":"game-1"}`, wantStatus: http.StatusOK, wantCalls: 1},
	}
	for _, testCase := range tests {
		t.Run(testCase.name, func(t *testing.T) {
			bot := &handlerBotStub{shootError: testCase.botError}
			recorder := httptest.NewRecorder()
			request := httptest.NewRequest(http.MethodPost, "/shoot", strings.NewReader(testCase.body))
			newTestHandler(bot).ShootHandler().ServeHTTP(recorder, request)
			if recorder.Code != testCase.wantStatus || bot.shootCalls != testCase.wantCalls {
				t.Fatalf("status=%d calls=%d; want status=%d calls=%d", recorder.Code, bot.shootCalls, testCase.wantStatus, testCase.wantCalls)
			}
			if testCase.wantStatus == http.StatusOK && !strings.Contains(recorder.Body.String(), `"X":3`) {
				t.Errorf("response body = %q; want shot coordinates", recorder.Body.String())
			}
		})
	}
}

func TestSetResultHandler(t *testing.T) {
	bot := &handlerBotStub{}
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPost, "/set_result", strings.NewReader(`{"user_key":"game-2","result":2}`))
	newTestHandler(bot).SetResultHandler().ServeHTTP(recorder, request)
	if recorder.Code != http.StatusOK || bot.resultCalls != 1 || bot.lastResult != my_types.Sink || bot.lastUserKey != "game-2" {
		t.Fatalf("status=%d result calls=%d result=%v key=%q", recorder.Code, bot.resultCalls, bot.lastResult, bot.lastUserKey)
	}

	badRecorder := httptest.NewRecorder()
	badRequest := httptest.NewRequest(http.MethodPost, "/set_result", strings.NewReader("{"))
	newTestHandler(&handlerBotStub{}).SetResultHandler().ServeHTTP(badRecorder, badRequest)
	if badRecorder.Code != http.StatusBadRequest {
		t.Errorf("invalid JSON status = %d; want %d", badRecorder.Code, http.StatusBadRequest)
	}
}

func TestPlaceHandler(t *testing.T) {
	bot := &handlerBotStub{}
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPost, "/place", strings.NewReader(`{"field":[]}`))
	newTestHandler(bot).PlaceHandler().ServeHTTP(recorder, request)
	if recorder.Code != http.StatusOK || !strings.Contains(recorder.Body.String(), `"x":1`) {
		t.Fatalf("status=%d body=%q", recorder.Code, recorder.Body.String())
	}

	badRecorder := httptest.NewRecorder()
	badRequest := httptest.NewRequest(http.MethodPost, "/place", strings.NewReader("{"))
	newTestHandler(&handlerBotStub{}).PlaceHandler().ServeHTTP(badRecorder, badRequest)
	if badRecorder.Code != http.StatusBadRequest {
		t.Errorf("invalid JSON status = %d; want %d", badRecorder.Code, http.StatusBadRequest)
	}
}

func TestGameOverHandler(t *testing.T) {
	bot := &handlerBotStub{}
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPost, "/game_over", strings.NewReader(`{"user_key":"game-3"}`))
	newTestHandler(bot).GameOverHandler().ServeHTTP(recorder, request)
	if recorder.Code != http.StatusOK || bot.gameOverCalls != 1 || bot.lastUserKey != "game-3" {
		t.Fatalf("status=%d calls=%d key=%q", recorder.Code, bot.gameOverCalls, bot.lastUserKey)
	}

	failedBot := &handlerBotStub{gameOverError: errors.New("cleanup failed")}
	failedRecorder := httptest.NewRecorder()
	failedRequest := httptest.NewRequest(http.MethodPost, "/game_over", strings.NewReader(`{"user_key":"game-3"}`))
	newTestHandler(failedBot).GameOverHandler().ServeHTTP(failedRecorder, failedRequest)
	if failedRecorder.Code != http.StatusInternalServerError {
		t.Errorf("bot failure status = %d; want %d", failedRecorder.Code, http.StatusInternalServerError)
	}

	badRecorder := httptest.NewRecorder()
	badRequest := httptest.NewRequest(http.MethodPost, "/game_over", strings.NewReader("{"))
	newTestHandler(&handlerBotStub{}).GameOverHandler().ServeHTTP(badRecorder, badRequest)
	if badRecorder.Code != http.StatusBadRequest {
		t.Errorf("invalid JSON status = %d; want %d", badRecorder.Code, http.StatusBadRequest)
	}
}
