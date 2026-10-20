package main

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"sea_battle/proxy/config"
	"sea_battle/proxy/repository"
	"strings"
	"sync"
	"time"

	mw "sea_battle/proxy/middleware"
)

type AuthRequest struct {
	Name     string `json:"name"`
	Password string `json:"password"`
}

type AuthResponse struct {
	Token string `json:"token"`
}

type MyProxy struct {
	client *http.Client
	rep    proxyRepository
	bots   map[string]config.BotConfig
	logger *slog.Logger

	sessionsMu sync.Mutex
	sessions   map[string]*gameSession
	storeOnce  sync.Once
}

type matchRepository interface {
	SetResult(context.Context, string, bool) error
}

type proxyRepository interface {
	matchRepository
	RegisterUser(context.Context, string, string) error
	GetPasswordHash(context.Context, string) (string, error)
}

func NewProxy(client *http.Client, cfg *config.Config, logger *slog.Logger) *MyProxy {
	rep, err := repository.NewRepository(logger)
	if err != nil {
		logger.Error("proxy failed to initialize repository", "error", err)
		panic("failed to connect to database")
	}
	logger.Info("proxy initialized successfully")
	return &MyProxy{
		client:   client,
		rep:      rep,
		bots:     cfg.Bots,
		logger:   logger,
		sessions: make(map[string]*gameSession),
	}
}

func LoginHandler(auth mw.AAA) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req AuthRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}

		token, err := auth.Login(req.Name, req.Password)
		if err != nil {
			http.Error(w, err.Error(), http.StatusUnauthorized)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		if err := json.NewEncoder(w).Encode(AuthResponse{Token: token}); err != nil {
			http.Error(w, "failed to encode login response", http.StatusInternalServerError)
		}
	}
}

func RegisterHandler(auth mw.AAA) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req AuthRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		if err := auth.Register(req.Name, req.Password); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		w.WriteHeader(http.StatusCreated)
	}
}

func VerifyHandler(auth mw.AAA) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		tokenString, err := bearerToken(r.Header.Get("Authorization"))
		if err != nil {
			http.Error(w, err.Error(), http.StatusUnauthorized)
			return
		}
		name, err := auth.Verify(tokenString)
		if err != nil {
			http.Error(w, err.Error(), http.StatusUnauthorized)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		if err := json.NewEncoder(w).Encode(map[string]string{"user": name}); err != nil {
			http.Error(w, "failed to encode verify response", http.StatusInternalServerError)
		}
	}
}

func bearerToken(authHeader string) (string, error) {
	parts := strings.Fields(authHeader)
	if len(parts) == 0 {
		return "", errors.New("authorization header required")
	}
	if len(parts) != 2 || !strings.EqualFold(parts[0], "Bearer") {
		return "", errors.New("invalid authorization format")
	}
	return parts[1], nil
}

type userContextKey struct{}

func (p *MyProxy) AuthHandler(auth mw.AAA, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		tokenString, err := bearerToken(r.Header.Get("Authorization"))
		if err != nil {
			http.Error(w, err.Error(), http.StatusUnauthorized)
			return
		}
		username, err := auth.Verify(tokenString)
		if err != nil {
			p.logger.Error("token verification failed", "error", err)
			http.Error(w, "Invalid or expired token", http.StatusUnauthorized)
			return
		}
		next.ServeHTTP(w, r.WithContext(context.WithValue(r.Context(), userContextKey{}, username)))
	})
}

func cors(next http.Handler) http.Handler {
	allowedOrigins := map[string]struct{}{
		"http://localhost:8080": {},
		"http://localhost:3000": {},
		"http://127.0.0.1:8080": {},
		"http://127.0.0.1:3000": {},
	}
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		origin := r.Header.Get("Origin")
		if _, allowed := allowedOrigins[origin]; allowed {
			w.Header().Set("Access-Control-Allow-Origin", origin)
			w.Header().Add("Vary", "Origin")
			w.Header().Set("Access-Control-Allow-Methods", "GET, POST, OPTIONS")
			w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")
		}
		if r.Method == http.MethodOptions {
			if origin == "" {
				http.Error(w, "Origin header required", http.StatusForbidden)
				return
			}
			if _, allowed := allowedOrigins[origin]; !allowed {
				http.Error(w, "Origin not allowed", http.StatusForbidden)
				return
			}
			w.WriteHeader(http.StatusNoContent)
			return
		}
		next.ServeHTTP(w, r)
	})
}

func main() {
	var cfgPath string
	flag.StringVar(&cfgPath, "config", "config.yaml", "config path")
	flag.Parse()

	client := &http.Client{Timeout: 30 * time.Second}
	cfg, err := config.LoadConfig(cfgPath)
	if err != nil {
		panic(err)
	}
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))
	proxy := NewProxy(client, cfg, logger)

	authService, err := mw.New(time.Hour, logger, proxy.rep)
	if err != nil {
		panic(err)
	}

	mux := http.NewServeMux()
	mux.HandleFunc("POST /login", LoginHandler(authService))
	mux.HandleFunc("POST /register", RegisterHandler(authService))
	mux.HandleFunc("GET /verify", VerifyHandler(authService))
	mux.Handle("POST /bot", proxy.AuthHandler(authService, http.HandlerFunc(proxy.ProxyHandler())))

	address := fmt.Sprintf("%s:%d", cfg.Proxy.Address, cfg.Proxy.Port)
	logger.Info("Listening on", "address", address)
	if err := http.ListenAndServe(address, cors(mux)); err != nil {
		logger.Error("proxy server stopped", "error", err)
	}
}
