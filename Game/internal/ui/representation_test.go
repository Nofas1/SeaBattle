package ui

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestAuthRequestReturnsToken(test *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		if request.Method != http.MethodPost || request.Header.Get("Content-Type") != "application/json" {
			test.Errorf("request method/content type = %s/%q", request.Method, request.Header.Get("Content-Type"))
		}
		var body AuthRequest
		if err := json.NewDecoder(request.Body).Decode(&body); err != nil {
			test.Errorf("decode request: %v", err)
		}
		if body.Name != "captain" || body.Password != "secret" {
			test.Errorf("request body = %+v", body)
		}
		_, _ = writer.Write([]byte(`{"token":"signed-token"}`))
	}))
	defer server.Close()

	token, err := authRequest(server.URL, "captain", "secret")
	if err != nil || token != "signed-token" {
		test.Fatalf("authRequest() = %q, %v; want signed-token", token, err)
	}
}

func TestAuthRequestRejectsInvalidResponses(test *testing.T) {
	tests := []struct {
		name      string
		status    int
		body      string
		wantError string
	}{
		{name: "server error", status: http.StatusUnauthorized, body: "invalid credentials", wantError: "invalid credentials"},
		{name: "malformed JSON", status: http.StatusOK, body: "{", wantError: "failed to decode response"},
		{name: "empty token", status: http.StatusCreated, body: `{"token":""}`, wantError: "empty token"},
	}
	for _, testCase := range tests {
		test.Run(testCase.name, func(test *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, _ *http.Request) {
				writer.WriteHeader(testCase.status)
				_, _ = writer.Write([]byte(testCase.body))
			}))
			defer server.Close()
			_, err := authRequest(server.URL, "captain", "secret")
			if err == nil || !strings.Contains(err.Error(), testCase.wantError) {
				test.Errorf("authRequest() error = %v; want containing %q", err, testCase.wantError)
			}
		})
	}
}

func TestAuthRequestReturnsTransportErrors(test *testing.T) {
	server := httptest.NewServer(http.NotFoundHandler())
	url := server.URL
	server.Close()
	if _, err := authRequest(url, "captain", "secret"); err == nil {
		test.Error("authRequest() error = nil; expected transport error")
	}
}
