package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"sea_battle/my_types"
	"sea_battle/smart_bot/config"
	"sea_battle/smart_bot/internal"
	// "sea_battle/smart_bot/db"
)

type Bot interface {
	Place() (int, int, my_types.Pair)      // returns ship placement coordinates and direction
	Shoot(string) (my_types.Pair, error)   // returns target coordinates for the given user session
	SetResult(string, my_types.ShotResult) // updates bot state based on shot result
	StartGame(string) error // initialize fresh bot state
	GameOver(string) error
}

type Handler struct {
	bot    Bot
	logger *slog.Logger
}

func NewHandler(bot Bot, logger *slog.Logger) *Handler {
	return &Handler{bot: bot, logger: logger}
}

func (h *Handler) StartGameHandler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		defer r.Body.Close()
		var req struct {
			UserKey string `json:"user_key"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			h.logger.Error(
				"failed to decode start game request",
				"error", err,
			)
			http.Error(w, err.Error(), http.StatusBadRequest)
			return 
		}

		if err := h.bot.StartGame(req.UserKey); err != nil {
			h.logger.Error(
				"failed to start game",
				"user_key", req.UserKey,
				"error", err,
			)
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		h.logger.Info(
			"game started",
			"user_key", req.UserKey,
		)
		w.WriteHeader(http.StatusOK)
	}
}

// POST /shoot requests
// decodes user_key from body, calls bot.Shoot and returns target coordinates
func (h *Handler) ShootHandler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		defer r.Body.Close()
		var req struct {
			UserKey string `json:"user_key"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			h.logger.Error(
				"failed to decode UserKey",
				"source", "smart_bot",
				"error", err,
			)
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		h.logger.Info(
			"shoot request received",
			"user_key", req.UserKey,
		)

		shot, err := h.bot.Shoot(req.UserKey)
		if err != nil {
			h.logger.Error(
				"shoot failed",
				"user_key", req.UserKey,
				"error", err,
			)
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(shot)
	}
}

// POST /set_result requests
// decodes user_key and shot result, updates bot internal state
func (h *Handler) SetResultHandler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		defer r.Body.Close()
		var req struct {
			UserKey  string              `json:"user_key"`
			Result   my_types.ShotResult `json:"result"`
			LastShot my_types.Pair       `json:"last_shot"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			h.logger.Error(
				"failed to decode shot result",
				"source", "smart_bot",
				"error", err,
			)
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		h.bot.SetResult(req.UserKey, req.Result)
		w.WriteHeader(http.StatusOK)
	}
}

// POST /place requests
// returns random ship placement coordinates and direction
func (h *Handler) PlaceHandler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		defer r.Body.Close()
		var req struct {
			Field [][]int `json:"field"`
		}
		json.NewDecoder(r.Body).Decode(&req)
		type PlaceResponse struct {
			X   int           `json:"x"`
			Y   int           `json:"y"`
			Dir my_types.Pair `json:"dir"`
		}

		x, y, dir := h.bot.Place()
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(PlaceResponse{X: x, Y: y, Dir: dir})
		w.WriteHeader(http.StatusOK)
	}
}

func (h *Handler) GameOverHandler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		defer r.Body.Close()
		var req struct {
			UserKey string `json:"user_key"`
		}
		json.NewDecoder(r.Body).Decode(&req)

		err := h.bot.GameOver(req.UserKey)
		if err != nil {
			h.logger.Error(
				"failed to clear state",
				"error", err,
			)
		}
		w.WriteHeader(http.StatusOK)
	}
}


func main() {
	// config parsing, log level parsing
	var cfg_path string
	var logLevel string
	flag.StringVar(&cfg_path, "config", "config.yaml", "config path")
	flag.StringVar(&logLevel, "log", "info", "log level: debug, info, warn, error")
	flag.Parse()

	cfg, err := config.LoadConfig(cfg_path)
	if err != nil {
		panic(err)
	}

	// log level initialization
	var level slog.Level
	switch logLevel {
	case "debug":
		level = slog.LevelDebug
	case "warn":
		level = slog.LevelWarn
	case "error":
		level = slog.LevelError
	default:
		level = slog.LevelInfo
	}

	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{
		Level: level,
	}))

	var bot Bot = internal.NewSmartBot(logger)
	h := NewHandler(bot, logger)

	http.HandleFunc("/start", h.StartGameHandler())
	http.HandleFunc("/shoot", h.ShootHandler())
	http.HandleFunc("/set_result", h.SetResultHandler())
	http.HandleFunc("/place", h.PlaceHandler())
	http.HandleFunc("/game_over", h.GameOverHandler())

	logger.Info("Listening on", "Adr", cfg.BotCfg.Address, "Port", cfg.BotCfg.Port)
	err = http.ListenAndServe(fmt.Sprintf("%s:%d", cfg.BotCfg.Address, cfg.BotCfg.Port), nil)
	fmt.Println(err)
}
