package main

import (
	"encoding/json"
	"net/http"
	"log/slog"
	"os"
	"flag"
	"fmt"
	"sea_battle/my_types"
	"sea_battle/smart_bot/internal"
	"sea_battle/smart_bot/config"
)

type Bot interface{
	Place() (int, int, my_types.Pair)
	Shoot(*my_types.Field) my_types.Pair
	SetResult(my_types.ShotResult)
}

type Handler struct {
	bot Bot
	logger *slog.Logger
}

func NewHandler(bot Bot, logger *slog.Logger) *Handler {
	return &Handler{bot: bot, logger: logger}
}

func (h *Handler) ShootHandler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		defer r.Body.Close()
		var field my_types.Field
		err := json.NewDecoder(r.Body).Decode(&field)
		if err != nil {
			h.logger.Error(
				"failed to decode field",
				"source", "smart_bot",
				"error", err,
			)
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		shot := h.bot.Shoot(&field)
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(shot)
		w.WriteHeader(http.StatusOK)
	}
}

func (h *Handler) SetResultHandler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		defer r.Body.Close()
		var res my_types.ShotResult
		err := json.NewDecoder(r.Body).Decode(&res)
		if err != nil {
			h.logger.Error(
				"failed to decode shot result",
				"source", "smart_bot",
				"error", err,
			)
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		h.bot.SetResult(res)
		w.WriteHeader(http.StatusOK)
	}
}

func (h *Handler) PlaceHandler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
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

func main() {
	var cfg_path string
	flag.StringVar(&cfg_path, "config", "config.yaml", "config path")
	flag.Parse()
	cfg, err := config.LoadConfig(cfg_path)
	if err != nil {
		panic(err)
	}

	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))
	var bot Bot = internal.NewSmartBot()
	h := NewHandler(bot, logger)

	http.HandleFunc("/shoot", h.ShootHandler())
	http.HandleFunc("/set_result", h.SetResultHandler())
	http.HandleFunc("/place", h.PlaceHandler())

	err = http.ListenAndServe(fmt.Sprintf("%s:%d", cfg.Address, cfg.Port), nil)
	fmt.Println(err)
}
