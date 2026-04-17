package game

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/url"
	"sea_battle/Game/internal/domain"
	"sea_battle/my_types"
)

type Bot interface {
	Place() (int, int, domain.Pair)
	Shoot(*domain.Field) domain.Pair
	SetResult(my_types.ShotResult)
}

type BotProxy struct {
	field   *domain.Field
	baseURL string
	client  *http.Client
}

func NewBotProxy(field *domain.Field, url string) *BotProxy {
	return &BotProxy{field: field, baseURL: url, client: &http.Client{}}
}

func (bp *BotProxy) Shoot(field *domain.Field) domain.Pair {
	body, _ := json.Marshal(field.Matrix)
    
    url, _ := url.JoinPath(bp.baseURL, "shoot")
    req, err := http.NewRequest("POST", url, bytes.NewReader(body))
    if err != nil {
        return domain.Pair{X: my_types.GlobalRand.Intn(10), Y: my_types.GlobalRand.Intn(10)}
    }
    req.Header.Set("Content-Type", "application/json")
    
    resp, err := bp.client.Do(req)
    if err != nil {
        return domain.Pair{X: my_types.GlobalRand.Intn(10), Y: my_types.GlobalRand.Intn(10)}
    }
    defer resp.Body.Close()
    
    var pair domain.Pair
    json.NewDecoder(resp.Body).Decode(&pair)
    return pair
}

func (bp *BotProxy) Place() (int, int, domain.Pair) {
	url, _ := url.JoinPath(bp.baseURL, "place")
    req, err := http.NewRequest("POST", url, nil)
    if err != nil {
        return my_types.GlobalRand.Intn(4), my_types.GlobalRand.Intn(10),
            domain.Pair{X: my_types.GlobalRand.Intn(10), Y: my_types.GlobalRand.Intn(10)}
    }
    req.Header.Set("Content-Type", "application/json")

    resp, err := bp.client.Do(req)
    if err != nil {
        return my_types.GlobalRand.Intn(4), my_types.GlobalRand.Intn(10),
            domain.Pair{X: my_types.GlobalRand.Intn(10), Y: my_types.GlobalRand.Intn(10)}
    }
    defer resp.Body.Close()

    var res struct {
        X   int          `json:"x"`
        Y   int          `json:"y"`
        Dir domain.Pair `json:"dir"`
    }
    json.NewDecoder(resp.Body).Decode(&res)
    return res.X, res.Y, res.Dir
}

func (bp *BotProxy) SetResult(result my_types.ShotResult) {
	body, _ := json.Marshal(result)
    url, _ := url.JoinPath(bp.baseURL, "set_result")
    req, _ := http.NewRequest("POST", url, bytes.NewReader(body))
    req.Header.Set("Content-Type", "application/json")
    bp.client.Do(req)
}
