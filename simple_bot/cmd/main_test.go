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

type simpleHandlerBotStub struct {
	shootCalls  int
	resultCalls int
}

func (bot *simpleHandlerBotStub) Place() (int, int, my_types.Pair) {
	return 1, 2, my_types.Pair{X: 0, Y: 1}
}

func (bot *simpleHandlerBotStub) Shoot(field *my_types.Field) my_types.Pair {
	bot.shootCalls++
	if len(field.Matrix) == 0 {
		return my_types.Pair{}
	}
	return my_types.Pair{X: 3, Y: 4}
}

func (bot *simpleHandlerBotStub) SetResult(my_types.ShotResult) {
	bot.resultCalls++
}

func newSimpleTestHandler(bot *simpleHandlerBotStub) *Handler {
	return NewHandler(bot, slog.New(slog.NewTextHandler(io.Discard, nil)))
}

func TestShootHandler(t *testing.T) {
	bot := &simpleHandlerBotStub{}
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPost, "/shoot", strings.NewReader(`{"field":[[0]]}`))
	newSimpleTestHandler(bot).ShootHandler().ServeHTTP(recorder, request)
	if recorder.Code != http.StatusOK || bot.shootCalls != 1 || !strings.Contains(recorder.Body.String(), `"X":3`) {
		t.Fatalf("status=%d calls=%d body=%q", recorder.Code, bot.shootCalls, recorder.Body.String())
	}

	badRecorder := httptest.NewRecorder()
	badRequest := httptest.NewRequest(http.MethodPost, "/shoot", strings.NewReader("{"))
	newSimpleTestHandler(&simpleHandlerBotStub{}).ShootHandler().ServeHTTP(badRecorder, badRequest)
	if badRecorder.Code != http.StatusBadRequest {
		t.Errorf("invalid JSON status = %d; want %d", badRecorder.Code, http.StatusBadRequest)
	}
}

func TestSetResultHandler(t *testing.T) {
	bot := &simpleHandlerBotStub{}
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPost, "/set_result", strings.NewReader("2"))
	newSimpleTestHandler(bot).SetResultHandler().ServeHTTP(recorder, request)
	if recorder.Code != http.StatusOK || bot.resultCalls != 1 {
		t.Fatalf("status=%d result calls=%d", recorder.Code, bot.resultCalls)
	}

	badRecorder := httptest.NewRecorder()
	badRequest := httptest.NewRequest(http.MethodPost, "/set_result", strings.NewReader("invalid"))
	newSimpleTestHandler(&simpleHandlerBotStub{}).SetResultHandler().ServeHTTP(badRecorder, badRequest)
	if badRecorder.Code != http.StatusBadRequest {
		t.Errorf("invalid JSON status = %d; want %d", badRecorder.Code, http.StatusBadRequest)
	}
}

func TestPlaceHandler(t *testing.T) {
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPost, "/place", nil)
	newSimpleTestHandler(&simpleHandlerBotStub{}).PlaceHandler().ServeHTTP(recorder, request)
	if recorder.Code != http.StatusOK || !strings.Contains(recorder.Body.String(), `"x":1`) {
		t.Fatalf("status=%d body=%q", recorder.Code, recorder.Body.String())
	}
}
