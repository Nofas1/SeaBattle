package main

import (
	"context"
	"crypto/subtle"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/url"
	"strings"
	"time"

	"sea_battle/proxy/internal/oauth"
	mw "sea_battle/proxy/middleware"
)

func corsHandler(next http.Handler, frontendOrigin string) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		origin := r.Header.Get("Origin")
		if origin != "" {
			w.Header().Add("Vary", "Origin")
		}
		if origin == frontendOrigin {
			w.Header().Set("Access-Control-Allow-Origin", frontendOrigin)
			if r.Method == http.MethodOptions {
				w.Header().Set("Access-Control-Allow-Methods", "GET, POST, OPTIONS")
				w.Header().Set("Access-Control-Allow-Headers", "Authorization, Content-Type")
				w.WriteHeader(http.StatusNoContent)
				return
			}
		}
		next.ServeHTTP(w, r)
	})
}

type googleUserRepository interface {
	FindOrCreateGoogleUser(context.Context, string, string) (int64, string, error)
}

func GoogleAuthHandler(flow *oauth.Flow, rep googleUserRepository, auth *mw.AAA, logger *slog.Logger) http.HandlerFunc {
	clearState := func(w http.ResponseWriter) {
		http.SetCookie(w, &http.Cookie{
			Name:     "oauth_state",
			Value:    "",
			Path:     "/",
			MaxAge:   -1,
			HttpOnly: true,
			Secure:   flow.SecureCookie(),
			SameSite: http.SameSiteLaxMode,
		})
	}

	return func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/auth/google" {
			state, err := oauth.NewState()
			if err != nil {
				logger.Error("failed to generate Google OAuth state")
				http.Error(w, "unable to start Google sign-in", http.StatusInternalServerError)
				return
			}
			http.SetCookie(w, &http.Cookie{
				Name:     "oauth_state",
				Value:    state,
				Path:     "/",
				MaxAge:   300,
				HttpOnly: true,
				Secure:   flow.SecureCookie(),
				SameSite: http.SameSiteLaxMode,
			})
			http.Redirect(w, r, flow.Config.AuthCodeURL(state), http.StatusTemporaryRedirect)
			return
		}

		defer clearState(w)
		cookie, err := r.Cookie("oauth_state")
		if err != nil {
			http.Error(w, "Google sign-in state is missing; retry using the same hostname for the frontend and GOOGLE_REDIRECT_URL (127.0.0.1)", http.StatusBadRequest)
			return
		}
		state := r.URL.Query().Get("state")
		if state == "" || subtle.ConstantTimeCompare([]byte(state), []byte(cookie.Value)) != 1 {
			http.Error(w, "Google sign-in state is invalid", http.StatusBadRequest)
			return
		}

		code := r.URL.Query().Get("code")
		if code == "" {
			http.Error(w, "Google authorization code is missing", http.StatusBadRequest)
			return
		}
		ctx, cancel := context.WithTimeout(r.Context(), 15*time.Second)
		defer cancel()
		token, err := flow.Config.Exchange(ctx, code)
		if err != nil {
			logger.Error("Google OAuth code exchange failed")
			http.Error(w, "Google sign-in failed", http.StatusBadGateway)
			return
		}

		userInfoRequest, err := http.NewRequestWithContext(ctx, http.MethodGet, flow.UserInfoURL, nil)
		if err != nil {
			logger.Error("failed to create Google userinfo request")
			http.Error(w, "Google sign-in failed", http.StatusInternalServerError)
			return
		}
		userInfoResponse, err := flow.Config.Client(ctx, token).Do(userInfoRequest)
		if err != nil {
			logger.Error("Google userinfo request failed")
			http.Error(w, "Google sign-in failed", http.StatusBadGateway)
			return
		}
		defer userInfoResponse.Body.Close()
		if userInfoResponse.StatusCode != http.StatusOK {
			logger.Error("Google userinfo returned a non-200 status", "status", userInfoResponse.StatusCode)
			http.Error(w, "Google sign-in failed", http.StatusBadGateway)
			return
		}

		var profile struct {
			Subject       string `json:"sub"`
			Email         string `json:"email"`
			EmailVerified bool   `json:"email_verified"`
			Picture       string `json:"picture"`
			Name          string `json:"name"`
		}
		if err := json.NewDecoder(io.LimitReader(userInfoResponse.Body, 1<<20)).Decode(&profile); err != nil {
			logger.Error("failed to decode Google userinfo")
			http.Error(w, "Google sign-in failed", http.StatusBadGateway)
			return
		}
		if profile.Subject == "" || profile.Email == "" {
			http.Error(w, "Google account information is incomplete", http.StatusBadGateway)
			return
		}
		if !profile.EmailVerified {
			http.Error(w, "Google email is not verified", http.StatusUnauthorized)
			return
		}

		_, userName, err := rep.FindOrCreateGoogleUser(r.Context(), profile.Subject, profile.Email)
		if err != nil {
			logger.Error("failed to find or create Google user")
			http.Error(w, "Google sign-in failed", http.StatusInternalServerError)
			return
		}
		jwt, err := auth.IssueToken(userName)
		if err != nil {
			logger.Error("failed to issue Google sign-in token")
			http.Error(w, "Google sign-in failed", http.StatusInternalServerError)
			return
		}

		redirectURL := strings.TrimRight(flow.FrontendURL, "/") + "/#auth?token=" + url.QueryEscape(jwt)
		w.Header().Set("Cache-Control", "no-store")
		http.Redirect(w, r, redirectURL, http.StatusFound)
	}
}
