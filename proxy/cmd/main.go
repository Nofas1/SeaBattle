package main

import (
	"bytes"
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"log/slog"
	"net/http"
	"net/url"
	"os"
	"sea_battle/my_types"
	"sea_battle/proxy/config"
	"time"
)

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
			Name   string  `json:"name"`
			Field  [][]int `json:"field"`
			Action string  `json:"action"`
			Result my_types.ShotResult `json:"result,omitempty"`
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
		resp.Body.Close()

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

		type Request struct{
			Field [][]int `json:"field"`
		}
		var body_req Request
		body_req.Field = req.Field

		body, err := json.Marshal(body_req)
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		p.logger.Error(string(body), "error", err)

		ctx, cancel = context.WithTimeout(context.Background(), time.Millisecond*1500)
		defer cancel()

		joinURL, err = url.JoinPath(baseURL, req.Action)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
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
		w.WriteHeader(http.StatusOK)
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
	http.HandleFunc("/", proxy.ProxyHandler())
	err = http.ListenAndServe(fmt.Sprintf("%s:%d", cfg.Proxy.Address, cfg.Proxy.Port), nil)
	fmt.Println(err)
}
