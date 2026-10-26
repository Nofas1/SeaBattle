package main

import (
	"context"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

	"sea_battle/proxy/internal/oauth"
	mw "sea_battle/proxy/middleware"

	"golang.org/x/oauth2"
)

type googleRepoStub struct {
	userID  int64
	name    string
	err     error
	calls   int
	subject string
	email   string
}

func (rep *googleRepoStub) FindOrCreateGoogleUser(_ context.Context, subject, email string) (int64, string, error) {
	rep.calls++
	rep.subject = subject
	rep.email = email
	return rep.userID, rep.name, rep.err
}

func TestGoogleAuthStartCreatesSecurelyScopedStateCookie(t *testing.T) {
	handler, _, _ := newGoogleTestHandler(t, http.StatusOK, `{"sub":"google-sub","email":"user@example.com","email_verified":true}`)

	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/auth/google", nil))

	if recorder.Code != http.StatusTemporaryRedirect {
		t.Fatalf("status = %d; want 307", recorder.Code)
	}
	cookies := recorder.Result().Cookies()
	if len(cookies) != 1 {
		t.Fatalf("got %d cookies; want one", len(cookies))
	}
	cookie := cookies[0]
	if cookie.Name != "oauth_state" || cookie.Path != "/" || cookie.MaxAge != 300 || !cookie.HttpOnly || cookie.Secure || cookie.SameSite != http.SameSiteLaxMode {
		t.Fatalf("unexpected state cookie: %+v", cookie)
	}
	if len(cookie.Value) != 44 {
		t.Fatalf("state length = %d; want base64 encoding of 32 bytes", len(cookie.Value))
	}
}

func TestCORSMiddlewareAllowsConfiguredFrontend(t *testing.T) {
	next := http.HandlerFunc(func(writer http.ResponseWriter, _ *http.Request) {
		writer.WriteHeader(http.StatusNoContent)
	})
	handler := corsHandler(next, "http://127.0.0.1:8080")
	request := httptest.NewRequest(http.MethodOptions, "/verify", nil)
	request.Header.Set("Origin", "http://127.0.0.1:8080")
	request.Header.Set("Access-Control-Request-Method", "GET")
	request.Header.Set("Access-Control-Request-Headers", "authorization")
	recorder := httptest.NewRecorder()

	handler.ServeHTTP(recorder, request)

	if recorder.Code != http.StatusNoContent {
		t.Fatalf("preflight status = %d; want 204", recorder.Code)
	}
	if recorder.Header().Get("Access-Control-Allow-Origin") != "http://127.0.0.1:8080" {
		t.Fatal("configured frontend origin was not allowed")
	}
	if recorder.Header().Get("Access-Control-Allow-Headers") != "Authorization, Content-Type" {
		t.Fatal("authentication headers were not allowed")
	}
}

func TestGoogleAuthCallbackRejectsMissingOrMismatchedState(t *testing.T) {
	handler, _, repo := newGoogleTestHandler(t, http.StatusOK, `{"sub":"google-sub","email":"user@example.com","email_verified":true}`)
	for _, testCase := range []struct {
		name       string
		cookie     *http.Cookie
		queryState string
	}{
		{name: "missing cookie", queryState: "state"},
		{name: "mismatched state", cookie: &http.Cookie{Name: "oauth_state", Value: "expected", Path: "/"}, queryState: "actual"},
	} {
		t.Run(testCase.name, func(t *testing.T) {
			request := httptest.NewRequest(http.MethodGet, "/auth/google/callback?state="+testCase.queryState+"&code=test-code", nil)
			if testCase.cookie != nil {
				request.AddCookie(testCase.cookie)
			}
			recorder := httptest.NewRecorder()
			handler.ServeHTTP(recorder, request)
			if recorder.Code != http.StatusBadRequest {
				t.Fatalf("status = %d; want 400", recorder.Code)
			}
			if testCase.name == "missing cookie" &&
				!strings.Contains(recorder.Body.String(), "same hostname") {
				t.Fatalf("missing-cookie response does not explain the hostname requirement: %q", recorder.Body.String())
			}
			clearedCookie := recorder.Header().Get("Set-Cookie")
			if !strings.Contains(clearedCookie, "oauth_state=") || !strings.Contains(clearedCookie, "Max-Age=0") {
				t.Fatal("callback did not clear oauth_state")
			}
		})
	}
	if repo.calls != 0 {
		t.Fatalf("repository calls = %d; want 0", repo.calls)
	}
}

func TestGoogleAuthCallbackRejectsUnverifiedEmail(t *testing.T) {
	handler, _, repo := newGoogleTestHandler(t, http.StatusOK, `{"sub":"google-sub","email":"user@example.com","email_verified":false}`)
	recorder := googleCallback(handler, "valid-state")
	if recorder.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d; want 401", recorder.Code)
	}
	if repo.calls != 0 {
		t.Fatalf("repository calls = %d; want 0", repo.calls)
	}
}

func TestGoogleAuthCallbackRejectsUserInfoFailure(t *testing.T) {
	handler, _, repo := newGoogleTestHandler(t, http.StatusServiceUnavailable, `{"error":"unavailable"}`)
	recorder := googleCallback(handler, "valid-state")
	if recorder.Code != http.StatusBadGateway {
		t.Fatalf("status = %d; want 502", recorder.Code)
	}
	if repo.calls != 0 {
		t.Fatalf("repository calls = %d; want 0", repo.calls)
	}
}

func TestGoogleAuthCallbackIssuesTheExistingJWTAndRedirects(t *testing.T) {
	handler, _, repo := newGoogleTestHandler(t, http.StatusOK, `{"sub":"google-sub","email":"user@example.com","email_verified":true}`)
	recorder := googleCallback(handler, "valid-state")
	if recorder.Code != http.StatusFound {
		t.Fatalf("status = %d; want 302: %s", recorder.Code, recorder.Body.String())
	}
	location, err := url.Parse(recorder.Header().Get("Location"))
	if err != nil {
		t.Fatalf("parse redirect: %v", err)
	}
	if location.Host != "frontend.example" || location.Fragment == "" {
		t.Fatalf("unexpected redirect: %s", location)
	}
	fragment, err := url.ParseQuery(strings.TrimPrefix(location.Fragment, "auth?"))
	if err != nil {
		t.Fatalf("parse fragment query: %v", err)
	}
	token := fragment.Get("token")
	if token == "" {
		t.Fatal("redirect is missing token")
	}
	if repo.calls != 1 || repo.subject != "google-sub" || repo.email != "user@example.com" {
		t.Fatalf("repository interaction = calls:%d subject:%q email:%q", repo.calls, repo.subject, repo.email)
	}

	t.Setenv("JWT_SECRET", "01234567890123456789012345678901")
	auth, err := mw.New(time.Minute, slog.New(slog.NewTextHandler(io.Discard, nil)), &proxyAuthRepoStub{})
	if err != nil {
		t.Fatalf("create auth service: %v", err)
	}
	name, err := auth.Verify(token)
	if err != nil || name != repo.name {
		t.Fatalf("verified token subject = %q, error = %v; want %q", name, err, repo.name)
	}
}

func newGoogleTestHandler(t *testing.T, userInfoStatus int, userInfoBody string) (http.Handler, *httptest.Server, *googleRepoStub) {
	t.Helper()
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		switch request.URL.Path {
		case "/token":
			writer.Header().Set("Content-Type", "application/json")
			io.WriteString(writer, `{"access_token":"test-access-token","token_type":"Bearer"}`)
		case "/userinfo":
			writer.WriteHeader(userInfoStatus)
			io.WriteString(writer, userInfoBody)
		default:
			http.NotFound(writer, request)
		}
	}))
	t.Cleanup(server.Close)

	flow := &oauth.Flow{
		Config: &oauth2.Config{
			ClientID:     "test-client",
			ClientSecret: "test-secret",
			RedirectURL:  "http://127.0.0.1:28080/auth/google/callback",
			Endpoint:     oauth2.Endpoint{AuthURL: server.URL + "/auth", TokenURL: server.URL + "/token"},
		},
		FrontendURL: "http://frontend.example",
		UserInfoURL: server.URL + "/userinfo",
	}
	repo := &googleRepoStub{userID: 17, name: "user@example.com"}
	t.Setenv("JWT_SECRET", "01234567890123456789012345678901")
	auth, err := mw.New(time.Hour, slog.New(slog.NewTextHandler(io.Discard, nil)), &proxyAuthRepoStub{})
	if err != nil {
		t.Fatalf("create auth service: %v", err)
	}
	return GoogleAuthHandler(flow, repo, &auth, slog.New(slog.NewTextHandler(io.Discard, nil))), server, repo
}

func googleCallback(handler http.Handler, state string) *httptest.ResponseRecorder {
	request := httptest.NewRequest(http.MethodGet, "/auth/google/callback?state="+url.QueryEscape(state)+"&code=test-code", nil)
	request.AddCookie(&http.Cookie{Name: "oauth_state", Value: state, Path: "/"})
	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, request)
	return recorder
}
