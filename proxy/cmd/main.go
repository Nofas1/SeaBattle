package main

import (
	"bytes"
	"context"
	"encoding/json"
	"flag"
	"fmt"

	"io"
	"log/slog"
	"net/http"
	"net/url"
	"os"
	"sea_battle/my_types"
	"sea_battle/proxy/config"
	"sea_battle/proxy/repository"

	mw "sea_battle/proxy/middleware"
	"strings"
	"time"
)

type AuthRequest struct {
	Name     string `json:"name"`
	Password string `json:"password"`
}

type AuthResponse struct {
	Token string `json:"token"`
}

type Proxy struct {
	client *http.Client
	rep *repository.Repo
	bots   map[string]config.BotConfig
	logger *slog.Logger
}

func NewProxy(client *http.Client, cfg *config.Config, logger *slog.Logger) *Proxy {
	rep, err := repository.NewRepository(logger)
    if err != nil {
        logger.Error(
			"proxy failed to initialize repository",
			"error", err,
		)
        panic("failed to connect to database")
    }
    logger.Info("proxy initialized successfully")
	return &Proxy{
		client: client,
		rep: rep,
		bots:   cfg.Bots,
		logger: logger,
	}
}

func (p *Proxy) ProxyHandler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		type ProxyRequest struct {
			Name    string              `json:"name"`
			Field   [][]int             `json:"field"`
			Action  string              `json:"action"`
			Result  my_types.ShotResult `json:"result,omitempty"`
			UserWin bool                `json:"user_win,omitempty"`
			UserKey string              `json:"user_key"`
		}

		type ProxyResponse struct {
			X int `json:"x"`
			Y int `json:"y"`
		}
		defer r.Body.Close()

		var req ProxyRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			p.logger.Error(
				"failed to decode request",
				"error", err,
			)
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}

		botCfg, exists := p.bots[req.Name]
		if !exists {
			p.logger.Error(
				"bot not found",
				"name", req.Name,
			)
			http.Error(w, "bot not found", http.StatusBadRequest)
			return
		}

		healthURL, err := url.JoinPath(botCfg.URL, "health")
		if err != nil {
			p.logger.Error(
				"failed to build health URL",
				"bot", req.Name,
				"error", err,
			)
			http.Error(w, "invalid bot URL", http.StatusInternalServerError)
			return
		}

		healthCtx, healthCancel := context.WithTimeout(r.Context(), 1500*time.Millisecond)
		defer healthCancel()

		healthReq, err := http.NewRequestWithContext(healthCtx, http.MethodGet, healthURL, nil)
		if err != nil {
			p.logger.Error(
				"failed to create health request",
				"error", err,
			)
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		healthResp, err := p.client.Do(healthReq)
		if err != nil {
			p.logger.Error(
				"bot health check failed",
				"bot", req.Name,
				"error", err,
			)
			http.Error(w, err.Error(), http.StatusServiceUnavailable)
			return
		}
		healthResp.Body.Close()

		if req.Action == "start_game" {
			type StartGameRequest struct {
				UserKey string `json:"user_key"`
			}
			sgBody, err := json.Marshal(StartGameRequest{UserKey: req.UserKey})
			if err != nil {
				p.logger.Error(
					"failed to marshal start_game",
					"error", err,
				)
				http.Error(w, err.Error(), http.StatusInternalServerError)
				return
			}

			sgURL, err := url.JoinPath(botCfg.URL, "start")
			if err != nil {
				p.logger.Error(
					"failed to build start_game URL",
					"error", err,
				)
				http.Error(w, err.Error(), http.StatusInternalServerError)
				return
			}

			sgCtx, sgCancel := context.WithTimeout(r.Context(), 1500*time.Millisecond)
			defer sgCancel()

			sgReq, err := http.NewRequestWithContext(sgCtx, http.MethodPost, sgURL, bytes.NewReader(sgBody))
			if err != nil {
				p.logger.Error(
					"failed to create start_game request",
					"error", err,
				)
				http.Error(w, err.Error(), http.StatusInternalServerError)
				return
			}
			sgReq.Header.Set("Content-Type", "application/json")

			sgResp, err := p.client.Do(sgReq)
			if err != nil {
				p.logger.Error(
					"start_game call failed",
					"bot", req.Name,
					"error", err,
				)
			} else {
				sgResp.Body.Close()
			}

			w.WriteHeader(http.StatusCreated)
			return
		}

		if req.Action == "set_result" {
			type SetResultRequest struct {
				Result  my_types.ShotResult `json:"result"`
				UserKey string              `json:"user_key"`
			}

			srBody, err := json.Marshal(SetResultRequest{Result: req.Result, UserKey: req.UserKey})
			if err != nil {
				p.logger.Error(
					"failed to marshal set_result",
					"error", err,
				)
				http.Error(w, err.Error(), http.StatusInternalServerError)
				return
			}

			srURL, err := url.JoinPath(botCfg.URL, "set_result")
			if err != nil {
				p.logger.Error(
					"failed to build set_result URL",
					"error", err,
				)
				http.Error(w, err.Error(), http.StatusInternalServerError)
				return
			}

			srCtx, srCancel := context.WithTimeout(r.Context(), 1500*time.Millisecond)
			defer srCancel()

			srReq, err := http.NewRequestWithContext(srCtx, http.MethodPost, srURL, bytes.NewReader(srBody))
			if err != nil {
				p.logger.Error(
					"failed to create set_result request",
					"error", err,
				)
				http.Error(w, err.Error(), http.StatusInternalServerError)
				return
			}
			srReq.Header.Set("Content-Type", "application/json")

			srResp, err := p.client.Do(srReq)
			if err != nil {
				p.logger.Error(
					"set_result call failed",
					"bot", req.Name,
					"error", err,
				)
			} else {
				srResp.Body.Close()
			}

			w.WriteHeader(http.StatusOK)
			return
		}

		if req.Action == "game_over" {
			type GameOverRequest struct {
				UserKey string `json:"user_key"`
			}

			goBody, err := json.Marshal(GameOverRequest{UserKey: req.UserKey})
			if err != nil {
				p.logger.Error(
					"failed to marshal game_over",
					"error", err,
				)
				http.Error(w, err.Error(), http.StatusInternalServerError)
				return
			}

			goURL, err := url.JoinPath(botCfg.URL, "game_over")
			if err != nil {
				p.logger.Error(
					"failed to build game_over URL",
					"error", err,
				)
				http.Error(w, err.Error(), http.StatusInternalServerError)
				return
			}

			goCtx, goCancel := context.WithTimeout(r.Context(), 1500*time.Millisecond)
			defer goCancel()

			goReq, err := http.NewRequestWithContext(goCtx, http.MethodPost, goURL, bytes.NewReader(goBody))
			if err != nil {
				p.logger.Error("failed to create game_over request", "error", err)
				http.Error(w, err.Error(), http.StatusInternalServerError)
				return
			}

			goReq.Header.Set("Content-Type", "application/json")

			goResp, err := p.client.Do(goReq)
			if err != nil {
				p.logger.Error(
					"game_over call failed",
					"bot", req.Name, 
					"error", err,
				)
			} else {
				goResp.Body.Close()
			}

			if err := p.rep.SetResult(context.Background(), req.UserKey, req.UserWin); err != nil {
				p.logger.Error(
					"failed to save match result",
					"user_key", req.UserKey,
					"user_win", req.UserWin,
					"error", err,
				)
			} else {
				p.logger.Info(
					"match result saved",
					"user_key", req.UserKey,
					"user_win", req.UserWin,
				)
			}

			w.WriteHeader(http.StatusOK)
			return
		}

		type BotRequest struct {
			Field   [][]int `json:"field"`
			UserKey string  `json:"user_key"`
		}

		botBody, err := json.Marshal(BotRequest{Field: req.Field, UserKey: req.UserKey})
		if err != nil {
			p.logger.Error("failed to marshal bot request", "error", err)
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		botURL, err := url.JoinPath(botCfg.URL, req.Action)
		if err != nil {
			p.logger.Error("failed to build action URL", "action", req.Action, "error", err)
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		botCtx, botCancel := context.WithTimeout(r.Context(), 1500*time.Millisecond)
		defer botCancel()

		botReq, err := http.NewRequestWithContext(botCtx, http.MethodPost, botURL, bytes.NewReader(botBody))
		if err != nil {
			p.logger.Error("failed to create bot request", "error", err)
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		botReq.Header.Set("Content-Type", "application/json")

		botResp, err := p.client.Do(botReq)
		if err != nil {
			p.logger.Error("bot request failed", "bot", req.Name, "action", req.Action, "error", err)
			http.Error(w, err.Error(), http.StatusServiceUnavailable)
			return
		}
		defer botResp.Body.Close()

		var result ProxyResponse
		if err := json.NewDecoder(botResp.Body).Decode(&result); err != nil {
			p.logger.Error("failed to decode bot response",
				"bot", req.Name,
				"action", req.Action,
				"error", err,
			)
			http.Error(w, err.Error(), http.StatusBadGateway)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		if err := json.NewEncoder(w).Encode(result); err != nil {
			p.logger.Error("failed to encode response", "error", err)
		}
	}
}

func LoginHandler(auth mw.AAA) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req AuthRequest

		err := json.NewDecoder(r.Body).Decode(&req)
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}

		token, err := auth.Login(req.Name, req.Password)
		if err != nil {
			http.Error(w, err.Error(), http.StatusUnauthorized)
			return
		}

		json.NewEncoder(w).Encode(AuthResponse{
			Token: token,
		})
	}
}

func RegisterHandler(auth mw.AAA) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req AuthRequest
		body, _ := io.ReadAll(r.Body)
		fmt.Printf("%s", string(body))
		err := json.Unmarshal(body, &req)
		// err := json.NewDecoder(body).Decode(&req)

		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}

		err = auth.Register(req.Name, req.Password)
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}

		w.WriteHeader(http.StatusCreated)
	}
}

func (p *Proxy) AuthHandler(auth mw.AAA, next http.Handler) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		authHeader := r.Header.Get("Authorization")
		if authHeader == "" {
			http.Error(w, "Authorization header required", http.StatusUnauthorized)
			return
		}

		bearer_token := strings.Split(authHeader, " ")
		if strings.ToLower(bearer_token[0]) != "bearer" || len(bearer_token) != 2 {
			http.Error(w, "Invalid authorization format", http.StatusUnauthorized)
			return
		}

		tokenString := bearer_token[1]

		if _, err := auth.Verify(tokenString); err != nil {
			p.logger.Error(
				"Token verification failed",
				"error", err,
			)
			http.Error(w, "Invalid or expired token", http.StatusUnauthorized)
			return
		}

		next.ServeHTTP(w, r)
	}
}

func main() {
	var cfg_path string
	flag.StringVar(&cfg_path, "config", "config.yaml", "config path")
	flag.Parse()

	client := &http.Client{}
	cfg, err := config.LoadConfig(cfg_path)
	if err != nil {
		panic(err)
	}
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))
	proxy := NewProxy(client, cfg, logger)
	// http.HandleFunc("/bot", proxy.ProxyHandler())

	authService, err := mw.New(time.Hour, logger, proxy.rep)
	if err != nil {
		panic(err)
	}

	http.HandleFunc("/login", LoginHandler(authService))
	http.HandleFunc("/register", RegisterHandler(authService))
	http.Handle("/bot", proxy.AuthHandler(authService, http.HandlerFunc(proxy.ProxyHandler())))

	logger.Info("Listening on", "Adr", cfg.Proxy.Address, "Port", cfg.Proxy.Port)
	err = http.ListenAndServe(fmt.Sprintf("%s:%d", cfg.Proxy.Address, cfg.Proxy.Port), nil)
	fmt.Println(err)
}
