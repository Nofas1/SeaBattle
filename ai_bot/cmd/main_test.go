package main

import (
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"sea_battle/my_types"
)

type aiHandlerBotStub struct {
	shootCalls  int
	resultCalls int
}

func (bot *aiHandlerBotStub) Place() (int, int, my_types.Pair) {
	return 4, 5, my_types.Pair{X: -1, Y: 0}
}

func (bot *aiHandlerBotStub) Shoot(*my_types.Field) my_types.Pair {
	bot.shootCalls++
	return my_types.Pair{X: 2, Y: 3}
}

func (bot *aiHandlerBotStub) SetResult(my_types.ShotResult) {
	bot.resultCalls++
}

func newAITestHandler(bot *aiHandlerBotStub) *Handler {
	return NewHandler(bot, slog.New(slog.NewTextHandler(io.Discard, nil)))
}

func TestShootHandler(t *testing.T) {
	bot := &aiHandlerBotStub{}
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPost, "/shoot", strings.NewReader(`{"field":[[0]]}`))
	newAITestHandler(bot).ShootHandler().ServeHTTP(recorder, request)
	if recorder.Code != http.StatusOK || bot.shootCalls != 1 || !strings.Contains(recorder.Body.String(), `"X":2`) {
		t.Fatalf("status=%d calls=%d body=%q", recorder.Code, bot.shootCalls, recorder.Body.String())
	}

	badRecorder := httptest.NewRecorder()
	badRequest := httptest.NewRequest(http.MethodPost, "/shoot", strings.NewReader("{"))
	newAITestHandler(&aiHandlerBotStub{}).ShootHandler().ServeHTTP(badRecorder, badRequest)
	if badRecorder.Code != http.StatusBadRequest {
		t.Errorf("invalid JSON status = %d; want %d", badRecorder.Code, http.StatusBadRequest)
	}
}

func TestSetResultHandler(t *testing.T) {
	bot := &aiHandlerBotStub{}
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPost, "/set_result", strings.NewReader("2"))
	newAITestHandler(bot).SetResultHandler().ServeHTTP(recorder, request)
	if recorder.Code != http.StatusOK || bot.resultCalls != 1 {
		t.Fatalf("status=%d result calls=%d", recorder.Code, bot.resultCalls)
	}

	badRecorder := httptest.NewRecorder()
	badRequest := httptest.NewRequest(http.MethodPost, "/set_result", strings.NewReader("invalid"))
	newAITestHandler(&aiHandlerBotStub{}).SetResultHandler().ServeHTTP(badRecorder, badRequest)
	if badRecorder.Code != http.StatusBadRequest {
		t.Errorf("invalid JSON status = %d; want %d", badRecorder.Code, http.StatusBadRequest)
	}
}

func TestPlaceHandler(t *testing.T) {
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPost, "/place", nil)
	newAITestHandler(&aiHandlerBotStub{}).PlaceHandler().ServeHTTP(recorder, request)
	if recorder.Code != http.StatusOK || !strings.Contains(recorder.Body.String(), `"x":4`) {
		t.Fatalf("status=%d body=%q", recorder.Code, recorder.Body.String())
	}
}
