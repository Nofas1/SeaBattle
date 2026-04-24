package game

import (
	"bytes"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"net/url"
	"sea_battle/Game/internal/domain"
	"sea_battle/my_types"
)

type Bot interface {
	Place() (int, int, domain.Pair, error)
	Shoot(*domain.Field) (domain.Pair, error)
	SetResult(my_types.ShotResult) error
}

type BotProxy struct {
	field   *domain.Field
	baseURL string
	botName string
	logger  *slog.Logger
	client  *http.Client
}

type ProxyRequest struct {
	Name   string              `json:"name"`
	Field  [][]int             `json:"field"`
	Action string              `json:"action"`
	Result my_types.ShotResult `json:"result,omitempty"`
}

func NewBotProxy(field *domain.Field, url string, botName string, logger *slog.Logger) *BotProxy {
	return &BotProxy{field: field, baseURL: url, botName: botName, client: &http.Client{}, logger: logger}
}

func (bp *BotProxy) Shoot(field *domain.Field) (domain.Pair, error) {
    body, _ := json.Marshal(ProxyRequest{
        Name:   bp.botName,
        Field:  field.Matrix,
        Action: "shoot",
    })
    joinURL, _ := url.JoinPath(bp.baseURL, "/")
    req, err := http.NewRequest("POST", joinURL, bytes.NewReader(body))
    if err != nil {
        return domain.Pair{}, fmt.Errorf("shoot: failed to create request: %w", err)
    }
    req.Header.Set("Content-Type", "application/json")
    resp, err := bp.client.Do(req)
    if err != nil {
        return domain.Pair{}, fmt.Errorf("shoot: bot unavailable: %w", err)
    }
    defer resp.Body.Close()
    var pair domain.Pair
    if err := json.NewDecoder(resp.Body).Decode(&pair); err != nil {
        return domain.Pair{}, fmt.Errorf("shoot: failed to decode: %w", err)
    }
    return pair, nil
}

func (bp *BotProxy) Place() (int, int, domain.Pair, error) {
    body, _ := json.Marshal(ProxyRequest{
        Name:   bp.botName,
        Field:  bp.field.Matrix,
        Action: "place",
    })
    joinURL, _ := url.JoinPath(bp.baseURL, "/")
    req, err := http.NewRequest("POST", joinURL, bytes.NewReader(body))
    if err != nil {
        return 0, 0, domain.Pair{}, fmt.Errorf("place: failed to create request: %w", err)
    }
    req.Header.Set("Content-Type", "application/json")
    resp, err := bp.client.Do(req)
    if err != nil {
        return 0, 0, domain.Pair{}, fmt.Errorf("place: bot unavailable: %w", err)
    }
    defer resp.Body.Close()
    var res struct {
        X   int         `json:"x"`
        Y   int         `json:"y"`
        Dir domain.Pair `json:"dir"`
    }
    if err := json.NewDecoder(resp.Body).Decode(&res); err != nil {
        return 0, 0, domain.Pair{}, fmt.Errorf("place: failed to decode: %w", err)
    }
    return res.X, res.Y, res.Dir, nil
}

func (bp *BotProxy) SetResult(result my_types.ShotResult) error {
    type SetResultRequest struct {
        Name   string             `json:"name"`
        Result my_types.ShotResult `json:"result"`
        Action string             `json:"action"`
    }
    body, _ := json.Marshal(SetResultRequest{
        Name:   bp.botName,
        Result: result,
        Action: "set_result",
    })
    joinURL, _ := url.JoinPath(bp.baseURL, "/")
    req, err := http.NewRequest("POST", joinURL, bytes.NewReader(body))
    if err != nil {
        return fmt.Errorf("set_result: failed to create request: %w", err)
    }
    req.Header.Set("Content-Type", "application/json")
    resp, err := bp.client.Do(req)
    if err != nil {
        return fmt.Errorf("set_result: bot unavailable: %w", err)
    }
    defer resp.Body.Close()
    return nil
}
