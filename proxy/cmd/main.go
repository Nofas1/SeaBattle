package main

import (
	"bytes"
	"context"
	"encoding/json"
	"flag"
	"fmt"
	// "io"
	"log/slog"
	"net/http"
	"net/url"
	"os"
	"sea_battle/my_types"
	"sea_battle/proxy/config"
	// mw "sea_battle/proxy/middleware"
	// "strings"
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
	bots   map[string]config.BotConfig
	logger *slog.Logger
}

func NewProxy(client *http.Client, cfg *config.Config, logger *slog.Logger) *Proxy {
	return &Proxy{
		client: client,
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
			UserKey string              `json:"user_key"`
		}

		type ProxyResponse struct {
			X int `json:"x"`
			Y int `json:"y"`
		}

		defer r.Body.Close()
		var req ProxyRequest
		w.Header().Set("Content-Type", "application/json")

		err := json.NewDecoder(r.Body).Decode(&req)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		botCfg, exist := p.bots[req.Name]
		baseURL := botCfg.URL
		if !exist {
			http.Error(w, "does not exist", http.StatusBadRequest)
			return
		}
		ctx, cancel := context.WithTimeout(context.Background(), time.Millisecond*1500)
		defer cancel()

		joinURL, err := url.JoinPath(baseURL, "health")
		new_req, err := http.NewRequestWithContext(ctx, "GET", joinURL, nil)
		resp, err := p.client.Do(new_req)
		if err != nil {
			http.Error(w, err.Error(), http.StatusServiceUnavailable)
			return
		}
		defer resp.Body.Close()

		if req.Action == "set_result" {
			type SetResultRequest struct {
				Result my_types.ShotResult `json:"result"`
			}
			body, _ := json.Marshal(SetResultRequest{Result: req.Result})
			joinURL, _ := url.JoinPath(baseURL, "set_result")
			new_req, _ := http.NewRequestWithContext(ctx, "POST", joinURL, bytes.NewReader(body))
			p.client.Do(new_req)
			w.WriteHeader(http.StatusOK)
			return
		}

		type Request struct {
			Field [][]int `json:"field"`
		}
		var body_req Request
		body_req.Field = req.Field

		body, err := json.Marshal(body_req)
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}

		ctx, cancel = context.WithTimeout(context.Background(), time.Millisecond*1500)
		defer cancel()

		joinURL, err = url.JoinPath(baseURL, req.Action)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		new_req, err = http.NewRequestWithContext(ctx, "POST", joinURL, bytes.NewReader(body))
		resp, err = p.client.Do(new_req)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		var botResp ProxyResponse

		err = json.NewDecoder(resp.Body).Decode(&botResp)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		err = json.NewEncoder(w).Encode(botResp)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
	}
}

func Matches() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {}
}

func Result() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {}
}

// func LoginHandler(auth mw.AAA) http.HandlerFunc {
// 	return func(w http.ResponseWriter, r *http.Request) {
// 		var req AuthRequest

// 		err := json.NewDecoder(r.Body).Decode(&req)
// 		if err != nil {
// 			http.Error(w, err.Error(), http.StatusBadRequest)
// 			return
// 		}

// 		token, err := auth.Login(req.Name, req.Password)
// 		if err != nil {
// 			http.Error(w, err.Error(), http.StatusUnauthorized)
// 			return
// 		}

// 		json.NewEncoder(w).Encode(AuthResponse{
// 			Token: token,
// 		})
// 	}
// }

// func RegisterHandler(auth mw.AAA) http.HandlerFunc {
// 	return func(w http.ResponseWriter, r *http.Request) {
// 		var req AuthRequest
// 		body, _ := io.ReadAll(r.Body)
// 		fmt.Printf("%s", string(body))
// 		err := json.Unmarshal(body, &req)
// 		// err := json.NewDecoder(body).Decode(&req)

// 		if err != nil {
// 			http.Error(w, err.Error(), http.StatusBadRequest)
// 			return
// 		}

// 		err = auth.Register(req.Name, req.Password)
// 		if err != nil {
// 			http.Error(w, err.Error(), http.StatusBadRequest)
// 			return
// 		}

// 		w.WriteHeader(http.StatusCreated)
// 	}
// }

// func (p *Proxy) AuthHandler(auth mw.AAA, next http.Handler) http.HandlerFunc {
// 	return func(w http.ResponseWriter, r *http.Request) {
// 		authHeader := r.Header.Get("Authorization")
// 		if authHeader == "" {
// 			http.Error(w, "Authorization header required", http.StatusUnauthorized)
// 			return
// 		}

// 		bearer_token := strings.Split(authHeader, " ")
// 		if strings.ToLower(bearer_token[0]) != "bearer" || len(bearer_token) != 2 {
// 			http.Error(w, "Invalid authorization format", http.StatusUnauthorized)
// 			return
// 		}

// 		tokenString := bearer_token[1]

// 		if _, err := auth.Verify(tokenString); err != nil {
// 			p.logger.Error(
// 				"Token verification failed",
// 				"error", err,
// 			)
// 			http.Error(w, "Invalid or expired token", http.StatusUnauthorized)
// 			return
// 		}

// 		next.ServeHTTP(w, r)
// 	}
// }

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
	http.HandleFunc("/bot", proxy.ProxyHandler())

	// authService, err := mw.New(time.Hour, logger)
	// if err != nil {
	// 	panic(err)
	// }

	// http.HandleFunc("/login", LoginHandler(authService))
	// http.HandleFunc("/register", RegisterHandler(authService))
	// http.Handle("/bot", proxy.AuthHandler(authService, http.HandlerFunc(proxy.ProxyHandler())))

	logger.Info("Listening on", "Adr", cfg.Proxy.Address, "Port", cfg.Proxy.Port)
	err = http.ListenAndServe(fmt.Sprintf("%s:%d", cfg.Proxy.Address, cfg.Proxy.Port), nil)
	fmt.Println(err)
}
