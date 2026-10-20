package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math/rand"
	"net/http"
	"net/url"
	"sort"
	"strings"
	"sync"
	"time"

	"sea_battle/my_types"

	"github.com/google/uuid"
)

const (
	gameSessionTTL  = 2 * time.Hour
	sessionGCPeriod = 10 * time.Minute
)

var fleetSizes = []int{4, 3, 3, 2, 2, 2, 1, 1, 1, 1}

type point struct {
	X int `json:"x"`
	Y int `json:"y"`
}

type shipState struct {
	Cells []point `json:"cells"`
	Sunk  bool    `json:"sunk"`
}

type fieldView struct {
	Ships []shipState `json:"ships"`
}

type shotRecord struct {
	X      int    `json:"x"`
	Y      int    `json:"y"`
	Result string `json:"result"`
}

type shotView struct {
	X          int     `json:"x"`
	Y          int     `json:"y"`
	Result     string  `json:"result"`
	SunkShip   []point `json:"sunkShip,omitempty"`
	AroundSunk []point `json:"aroundSunk,omitempty"`
}

type gameResponse struct {
	SessionID          string       `json:"sessionId"`
	PlayerShot         *shotView    `json:"playerShot,omitempty"`
	BotShot            *shotView    `json:"botShot"`
	YourTurn           bool         `json:"yourTurn"`
	GameOver           bool         `json:"gameOver"`
	Winner             *string      `json:"winner"`
	PlayerField        fieldView    `json:"playerField"`
	EnemyShotsOnPlayer []shotRecord `json:"enemyShotsOnPlayerField"`
	PlayerShotsOnEnemy []shotRecord `json:"playerShotsOnEnemyField"`
}

type botListResponse struct {
	Bots []string `json:"bots"`
}

type gameSession struct {
	mu sync.Mutex

	// The server owns both fields; only playerField is included in responses.
	id          string
	user        string
	botID       string
	playerField [][]int
	playerShips []shipState
	enemyField  [][]int
	enemyShips  []shipState
	playerShots map[string]string
	enemyShots  map[string]string
	yourTurn    bool
	gameOver    bool
	winner      *string
	expiresAt   time.Time
	resultSaved bool
	botCleaned  bool
}

type botRequest struct {
	Action      string  `json:"action"`
	BotID       string  `json:"botId"`
	SessionID   string  `json:"sessionId"`
	PlayerField [][]int `json:"playerField"`
	X           *int    `json:"x"`
	Y           *int    `json:"y"`
}

func (p *MyProxy) ensureSessionStore() {
	p.storeOnce.Do(func() {
		if p.sessions == nil {
			p.sessions = make(map[string]*gameSession)
		}
		go p.collectExpiredSessions()
	})
}

func (p *MyProxy) collectExpiredSessions() {
	ticker := time.NewTicker(sessionGCPeriod)
	defer ticker.Stop()
	for range ticker.C {
		now := time.Now()
		var expiredSessions []*gameSession
		p.sessionsMu.Lock()
		for id, session := range p.sessions {
			session.mu.Lock()
			isExpired := now.After(session.expiresAt)
			session.mu.Unlock()
			if isExpired {
				delete(p.sessions, id)
				expiredSessions = append(expiredSessions, session)
			}
		}
		p.sessionsMu.Unlock()
		for _, session := range expiredSessions {
			if bot, exists := p.bots[session.botID]; exists {
				if err := p.callBot(context.Background(), bot.URL, "game_over",
					map[string]string{"user_key": session.id}, nil, true); err != nil {
					p.logger.Warn("expired bot session cleanup failed",
						"bot", session.botID, "session_id", session.id, "error", err)
				}
			}
		}
	}
}

func (p *MyProxy) ProxyHandler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		user, ok := r.Context().Value(userContextKey{}).(string)
		if !ok || user == "" {
			http.Error(w, "authenticated user is missing", http.StatusUnauthorized)
			return
		}
		var req botRequest
		decoder := json.NewDecoder(io.LimitReader(r.Body, 1<<20))
		if err := decoder.Decode(&req); err != nil {
			http.Error(w, "invalid request body: "+err.Error(), http.StatusBadRequest)
			return
		}
		switch req.Action {
		case "list_bots":
			bots := make([]string, 0, len(p.bots))
			for id := range p.bots {
				bots = append(bots, id)
			}
			sort.Strings(bots)
			writeJSON(w, http.StatusOK, botListResponse{Bots: bots})
		case "start_game":
			p.startGame(w, r, user, req)
		case "shoot":
			if req.SessionID == "" {
				http.Error(w, "sessionId is required", http.StatusBadRequest)
				return
			}
			if req.X == nil || req.Y == nil {
				http.Error(w, "x and y coordinates are required", http.StatusBadRequest)
				return
			}
			p.shoot(w, r, user, req, *req.X, *req.Y)
		case "state":
			if req.SessionID == "" {
				http.Error(w, "sessionId is required", http.StatusBadRequest)
				return
			}
			p.getState(w, r, user, req)
		default:
			http.Error(w, "unsupported action", http.StatusBadRequest)
		}
	}
}

func (p *MyProxy) startGame(w http.ResponseWriter, r *http.Request, user string, req botRequest) {
	if req.BotID == "" {
		http.Error(w, "botId is required", http.StatusBadRequest)
		return
	}
	botConfig, exists := p.bots[req.BotID]
	if !exists {
		http.Error(w, "bot not found", http.StatusBadRequest)
		return
	}
	playerShips, err := validatePlayerField(req.PlayerField)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	// The opaque ID is the bot user_key too, isolating each stateful bot session.
	sessionID := uuid.NewString()
	enemyField, enemyShips, err := generateEnemyFleet()
	if err != nil {
		p.logger.Error("failed to generate enemy fleet", "error", err)
		http.Error(w, "failed to initialize game", http.StatusInternalServerError)
		return
	}
	if err := p.checkBot(r.Context(), botConfig.URL); err != nil {
		p.logger.Error("bot health check failed", "bot", req.BotID, "error", err)
		http.Error(w, "bot unavailable", http.StatusServiceUnavailable)
		return
	}
	if err := p.callBot(r.Context(), botConfig.URL, "start", map[string]string{"user_key": sessionID}, nil, true); err != nil {
		p.logger.Error("bot start failed", "bot", req.BotID, "error", err)
		http.Error(w, "failed to start bot game", http.StatusBadGateway)
		return
	}

	session := &gameSession{
		id:          sessionID,
		user:        user,
		botID:       req.BotID,
		playerField: cloneField(req.PlayerField),
		playerShips: playerShips,
		enemyField:  enemyField,
		enemyShips:  enemyShips,
		playerShots: make(map[string]string),
		enemyShots:  make(map[string]string),
		yourTurn:    true,
		expiresAt:   time.Now().Add(gameSessionTTL),
	}
	p.ensureSessionStore()
	p.sessionsMu.Lock()
	p.sessions[sessionID] = session
	p.sessionsMu.Unlock()
	writeJSON(w, http.StatusCreated, p.response(session, nil, nil))
}

func (p *MyProxy) shoot(w http.ResponseWriter, r *http.Request, user string, req botRequest, x, y int) {
	session, ok := p.findSession(req.SessionID, user)
	if !ok {
		http.Error(w, "game session not found or expired", http.StatusNotFound)
		return
	}
	session.mu.Lock()
	defer session.mu.Unlock()
	if err := validateShot(session, x, y); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	session.expiresAt = time.Now().Add(gameSessionTTL)

	playerShot := p.applyPlayerShot(session, x, y)
	var botShot *shotView
	if session.gameOver {
		p.saveGameResult(session)
	} else if playerShot.Result == "Miss" {
		var err error
		botShot, err = p.applyBotShot(r.Context(), session)
		if err != nil {
			p.logger.Error("bot move failed", "session_id", session.id, "error", err)
			http.Error(w, "bot move failed: "+err.Error(), http.StatusBadGateway)
			return
		}
		if session.gameOver {
			p.saveGameResult(session)
		}
	}
	writeJSON(w, http.StatusOK, p.response(session, playerShot, botShot))
}

func (p *MyProxy) getState(w http.ResponseWriter, r *http.Request, user string, req botRequest) {
	session, ok := p.findSession(req.SessionID, user)
	if !ok {
		http.Error(w, "game session not found or expired", http.StatusNotFound)
		return
	}
	session.mu.Lock()
	defer session.mu.Unlock()
	session.expiresAt = time.Now().Add(gameSessionTTL)
	if session.gameOver {
		p.saveGameResult(session)
	}
	var botShot *shotView
	if !session.yourTurn && !session.gameOver {
		var err error
		botShot, err = p.applyBotShot(r.Context(), session)
		if err != nil {
			p.logger.Error("bot move failed", "session_id", session.id, "error", err)
			http.Error(w, "bot move failed: "+err.Error(), http.StatusBadGateway)
			return
		}
		if session.gameOver {
			p.saveGameResult(session)
		}
	}
	writeJSON(w, http.StatusOK, p.response(session, nil, botShot))
}

func (p *MyProxy) findSession(id, user string) (*gameSession, bool) {
	p.ensureSessionStore()
	p.sessionsMu.Lock()
	session, ok := p.sessions[id]
	p.sessionsMu.Unlock()
	if !ok {
		return nil, false
	}
	session.mu.Lock()
	defer session.mu.Unlock()
	if session.user != user || time.Now().After(session.expiresAt) {
		return nil, false
	}
	return session, true
}

func (p *MyProxy) applyPlayerShot(session *gameSession, x, y int) *shotView {
	cell := session.enemyField[x][y]
	result := "Miss"
	if cell == my_types.SHIP {
		session.enemyField[x][y] = my_types.SHOOTED
		result = "Hit"
	} else {
		session.enemyField[x][y] = my_types.MISSED
	}
	session.playerShots[cellKey(x, y)] = result
	shot := &shotView{X: x, Y: y, Result: result}
	if result == "Hit" {
		shipIndex := shipIndexAt(session.enemyShips, x, y)
		if shipIndex >= 0 && shipIsSunk(session.enemyField, session.enemyShips[shipIndex]) {
			ship := &session.enemyShips[shipIndex]
			ship.Sunk = true
			shot.Result = "Sink"
			shot.SunkShip = ship.Cells
			shot.AroundSunk = aroundShip(ship.Cells)
			markSunkAndAround(session.enemyField, session.playerShots, *ship)
		}
	}
	if allShipsSunk(session.enemyShips) {
		session.gameOver = true
		session.yourTurn = false
		winner := "player"
		session.winner = &winner
	} else {
		session.yourTurn = shot.Result != "Miss"
	}
	return shot
}

func (p *MyProxy) applyBotShot(ctx context.Context, session *gameSession) (*shotView, error) {
	cfg := p.bots[session.botID]
	for attempt := 0; attempt < my_types.Size*my_types.Size; attempt++ {
		var target point
		payload := map[string]any{
			"field":    session.playerField,
			"user_key": session.id,
		}
		if err := p.callBot(ctx, cfg.URL, "shoot", payload, &target, false); err != nil {
			return nil, err
		}
		if target.X < 0 || target.X >= my_types.Size || target.Y < 0 || target.Y >= my_types.Size {
			return nil, fmt.Errorf("bot returned out-of-range coordinates (%d,%d)", target.X, target.Y)
		}
		key := cellKey(target.X, target.Y)
		if _, alreadyShot := session.enemyShots[key]; alreadyShot {
			if err := p.sendBotResult(ctx, cfg.URL, session.id, my_types.Already); err != nil {
				return nil, err
			}
			continue
		}

		result := "Miss"
		botResult := my_types.Miss
		if session.playerField[target.X][target.Y] == my_types.SHIP {
			result = "Hit"
			botResult = my_types.Hit
		}
		shipIndex := -1
		if result == "Hit" {
			shipIndex = shipIndexAt(session.playerShips, target.X, target.Y)
			if shipIndex >= 0 && shipWillBeSunk(session.playerField, session.playerShips[shipIndex], target) {
				botResult = my_types.Sink
			}
		}
		if err := p.sendBotResult(ctx, cfg.URL, session.id, botResult); err != nil {
			return nil, err
		}

		if result == "Hit" {
			session.playerField[target.X][target.Y] = my_types.SHOOTED
		} else {
			session.playerField[target.X][target.Y] = my_types.MISSED
		}
		session.enemyShots[key] = result
		shot := &shotView{X: target.X, Y: target.Y, Result: result}
		if botResult == my_types.Sink {
			ship := &session.playerShips[shipIndex]
			ship.Sunk = true
			shot.Result = "Sink"
			shot.SunkShip = ship.Cells
			shot.AroundSunk = aroundShip(ship.Cells)
			markSunkAndAround(session.playerField, session.enemyShots, *ship)
		}
		if allShipsSunk(session.playerShips) {
			session.gameOver = true
			session.yourTurn = false
			winner := "bot"
			session.winner = &winner
		} else {
			session.yourTurn = shot.Result == "Miss"
		}
		return shot, nil
	}
	return nil, errors.New("bot did not choose an unshot cell")
}

func (p *MyProxy) saveGameResult(session *gameSession) {
	if !session.resultSaved {
		if p.rep == nil {
			session.resultSaved = true
		} else {
			userWon := session.winner != nil && *session.winner == "player"
			if err := p.rep.SetResult(context.Background(), session.user, userWon); err != nil {
				p.logger.Error("failed to save match result", "user", session.user, "session_id", session.id, "error", err)
			} else {
				session.resultSaved = true
			}
		}
	}
	if !session.botCleaned {
		cfg, ok := p.bots[session.botID]
		if !ok {
			session.botCleaned = true
			return
		}
		if err := p.callBot(context.Background(), cfg.URL, "game_over", map[string]string{"user_key": session.id}, nil, true); err != nil {
			p.logger.Warn("bot game-over cleanup failed", "bot", session.botID, "session_id", session.id, "error", err)
		} else {
			session.botCleaned = true
		}
	}
}

func (p *MyProxy) response(session *gameSession, playerShot, botShot *shotView) gameResponse {
	return gameResponse{
		SessionID:          session.id,
		PlayerShot:         playerShot,
		BotShot:            botShot,
		YourTurn:           session.yourTurn,
		GameOver:           session.gameOver,
		Winner:             session.winner,
		PlayerField:        fieldView{Ships: cloneShips(session.playerShips)},
		EnemyShotsOnPlayer: records(session.enemyShots),
		PlayerShotsOnEnemy: records(session.playerShots),
	}
}

func records(shots map[string]string) []shotRecord {
	result := make([]shotRecord, 0, len(shots))
	for key, shotResult := range shots {
		var x, y int
		if _, err := fmt.Sscanf(key, "%d,%d", &x, &y); err == nil {
			result = append(result, shotRecord{X: x, Y: y, Result: shotResult})
		}
	}
	sort.Slice(result, func(i, j int) bool {
		if result[i].X == result[j].X {
			return result[i].Y < result[j].Y
		}
		return result[i].X < result[j].X
	})
	return result
}

func validateShot(session *gameSession, x, y int) error {
	if session.gameOver {
		return errors.New("game is over")
	}
	if !session.yourTurn {
		return errors.New("it is not your turn")
	}
	if x < 0 || x >= my_types.Size || y < 0 || y >= my_types.Size {
		return errors.New("shot coordinates must be between 0 and 9")
	}
	if _, exists := session.playerShots[cellKey(x, y)]; exists {
		return errors.New("cell has already been shot")
	}
	return nil
}

func validatePlayerField(field [][]int) ([]shipState, error) {
	if len(field) != my_types.Size {
		return nil, errors.New("playerField must have 10 columns")
	}
	for x := range field {
		if len(field[x]) != my_types.Size {
			return nil, errors.New("playerField must have 10 rows per column")
		}
		for y, cell := range field[x] {
			if cell != my_types.EMPTY && cell != my_types.SHIP {
				return nil, fmt.Errorf("playerField[%d][%d] must be 0 or 1", x, y)
			}
		}
	}

	visited := make([][]bool, my_types.Size)
	for x := range visited {
		visited[x] = make([]bool, my_types.Size)
	}
	var ships []shipState
	for x := 0; x < my_types.Size; x++ {
		for y := 0; y < my_types.Size; y++ {
			if field[x][y] != my_types.SHIP || visited[x][y] {
				continue
			}
			cells := floodShip(field, visited, x, y)
			ships = append(ships, shipState{Cells: cells})
		}
	}
	for _, ship := range ships {
		if !isStraightShip(ship.Cells) {
			return nil, errors.New("ships must be straight")
		}
	}
	for x := 0; x < my_types.Size; x++ {
		for y := 0; y < my_types.Size; y++ {
			if field[x][y] != my_types.SHIP {
				continue
			}
			for dx := -1; dx <= 1; dx++ {
				for dy := -1; dy <= 1; dy++ {
					nx, ny := x+dx, y+dy
					if (dx != 0 || dy != 0) && inBounds(nx, ny) && field[nx][ny] == my_types.SHIP &&
						shipIndexAt(ships, x, y) != shipIndexAt(ships, nx, ny) {
						return nil, errors.New("ships may not touch, including diagonally")
					}
				}
			}
		}
	}
	actual := make([]int, len(ships))
	for i, ship := range ships {
		actual[i] = len(ship.Cells)
	}
	sort.Ints(actual)
	expected := append([]int(nil), fleetSizes...)
	sort.Ints(expected)
	if len(actual) != len(expected) {
		return nil, errors.New("player fleet must contain exactly 10 ships")
	}
	for i := range expected {
		if actual[i] != expected[i] {
			return nil, errors.New("player fleet must contain one 4-cell, two 3-cell, three 2-cell, and four 1-cell ships")
		}
	}
	return ships, nil
}

func floodShip(field [][]int, visited [][]bool, startX, startY int) []point {
	queue := []point{{X: startX, Y: startY}}
	visited[startX][startY] = true
	var cells []point
	directions := [][2]int{{1, 0}, {-1, 0}, {0, 1}, {0, -1}}
	for len(queue) > 0 {
		current := queue[0]
		queue = queue[1:]
		cells = append(cells, current)
		for _, direction := range directions {
			x, y := current.X+direction[0], current.Y+direction[1]
			if inBounds(x, y) && field[x][y] == my_types.SHIP && !visited[x][y] {
				visited[x][y] = true
				queue = append(queue, point{X: x, Y: y})
			}
		}
	}
	return cells
}

func isStraightShip(cells []point) bool {
	if len(cells) < 2 {
		return true
	}
	sameX, sameY := true, true
	for _, cell := range cells[1:] {
		sameX = sameX && cell.X == cells[0].X
		sameY = sameY && cell.Y == cells[0].Y
	}
	return sameX || sameY
}

func generateEnemyFleet() ([][]int, []shipState, error) {
	for attempt := 0; attempt < 100; attempt++ {
		field := emptyField()
		ships := make([]shipState, 0, len(fleetSizes))
		placed := true
		for _, size := range fleetSizes {
			var chosen []point
			for try := 0; try < 4000; try++ {
				start := point{X: rand.Intn(my_types.Size), Y: rand.Intn(my_types.Size)}
				dx, dy := 0, 1
				if rand.Intn(2) == 0 {
					dx, dy = 1, 0
				}
				cells := make([]point, size)
				valid := true
				for i := range cells {
					x, y := start.X+i*dx, start.Y+i*dy
					if !inBounds(x, y) {
						valid = false
						break
					}
					cells[i] = point{X: x, Y: y}
					for nx := x - 1; nx <= x+1; nx++ {
						for ny := y - 1; ny <= y+1; ny++ {
							if inBounds(nx, ny) && field[nx][ny] == my_types.SHIP {
								valid = false
							}
						}
					}
				}
				if valid {
					chosen = cells
					break
				}
			}
			if len(chosen) == 0 {
				placed = false
				break
			}
			for _, cell := range chosen {
				field[cell.X][cell.Y] = my_types.SHIP
			}
			ships = append(ships, shipState{Cells: chosen})
		}
		if placed {
			return field, ships, nil
		}
	}
	return nil, nil, errors.New("unable to generate a valid fleet after 100 attempts")
}

func (p *MyProxy) checkBot(ctx context.Context, baseURL string) error {
	target, err := url.JoinPath(baseURL, "health")
	if err != nil {
		return fmt.Errorf("invalid bot health URL: %w", err)
	}
	ctx, cancel := context.WithTimeout(ctx, 2*time.Second)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, target, nil)
	if err != nil {
		return fmt.Errorf("create bot health request: %w", err)
	}
	resp, err := p.client.Do(req)
	if err != nil {
		return fmt.Errorf("request bot health: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		return fmt.Errorf("bot health returned %s", resp.Status)
	}
	return nil
}

func (p *MyProxy) callBot(ctx context.Context, baseURL, path string, requestBody, responseBody any, ignoreNotFound bool) error {
	target, err := url.JoinPath(baseURL, path)
	if err != nil {
		return fmt.Errorf("invalid bot URL: %w", err)
	}
	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	var body io.Reader
	if requestBody != nil {
		encoded, err := json.Marshal(requestBody)
		if err != nil {
			return fmt.Errorf("encode bot request: %w", err)
		}
		body = bytes.NewReader(encoded)
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, target, body)
	if err != nil {
		return fmt.Errorf("create bot request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := p.client.Do(req)
	if err != nil {
		return fmt.Errorf("request bot %s: %w", path, err)
	}
	defer resp.Body.Close()
	if ignoreNotFound && resp.StatusCode == http.StatusNotFound {
		return nil
	}
	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		message, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
		return fmt.Errorf("bot %s returned %s: %s", path, resp.Status, strings.TrimSpace(string(message)))
	}
	if responseBody == nil {
		return nil
	}
	if err := json.NewDecoder(io.LimitReader(resp.Body, 1<<20)).Decode(responseBody); err != nil {
		return fmt.Errorf("decode bot %s response: %w", path, err)
	}
	return nil
}

func (p *MyProxy) sendBotResult(ctx context.Context, baseURL, sessionID string, result my_types.ShotResult) error {
	payload := map[string]any{"user_key": sessionID, "result": result}
	return p.callBot(ctx, baseURL, "set_result", payload, nil, false)
}

func writeJSON(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(value); err != nil {
		return
	}
}

func emptyField() [][]int {
	field := make([][]int, my_types.Size)
	for x := range field {
		field[x] = make([]int, my_types.Size)
	}
	return field
}

func cloneField(field [][]int) [][]int {
	copyOf := make([][]int, len(field))
	for i := range field {
		copyOf[i] = append([]int(nil), field[i]...)
	}
	return copyOf
}

func cloneShips(ships []shipState) []shipState {
	copyOf := make([]shipState, len(ships))
	for i := range ships {
		copyOf[i] = shipState{Cells: append([]point(nil), ships[i].Cells...), Sunk: ships[i].Sunk}
	}
	return copyOf
}

func cellKey(x, y int) string { return fmt.Sprintf("%d,%d", x, y) }

func inBounds(x, y int) bool {
	return x >= 0 && x < my_types.Size && y >= 0 && y < my_types.Size
}

func shipIndexAt(ships []shipState, x, y int) int {
	for i, ship := range ships {
		for _, cell := range ship.Cells {
			if cell.X == x && cell.Y == y {
				return i
			}
		}
	}
	return -1
}

func shipIsSunk(field [][]int, ship shipState) bool {
	for _, cell := range ship.Cells {
		if field[cell.X][cell.Y] != my_types.SHOOTED {
			return false
		}
	}
	return true
}

func shipWillBeSunk(field [][]int, ship shipState, target point) bool {
	for _, cell := range ship.Cells {
		if cell != target && field[cell.X][cell.Y] != my_types.SHOOTED {
			return false
		}
	}
	return true
}

func allShipsSunk(ships []shipState) bool {
	for _, ship := range ships {
		if !ship.Sunk {
			return false
		}
	}
	return true
}

func aroundShip(cells []point) []point {
	occupied := make(map[string]struct{}, len(cells))
	for _, cell := range cells {
		occupied[cellKey(cell.X, cell.Y)] = struct{}{}
	}
	around := make(map[string]point)
	for _, cell := range cells {
		for x := cell.X - 1; x <= cell.X+1; x++ {
			for y := cell.Y - 1; y <= cell.Y+1; y++ {
				key := cellKey(x, y)
				if inBounds(x, y) {
					if _, isShip := occupied[key]; !isShip {
						around[key] = point{X: x, Y: y}
					}
				}
			}
		}
	}
	result := make([]point, 0, len(around))
	for _, cell := range around {
		result = append(result, cell)
	}
	sort.Slice(result, func(i, j int) bool {
		if result[i].X == result[j].X {
			return result[i].Y < result[j].Y
		}
		return result[i].X < result[j].X
	})
	return result
}

func markSunkAndAround(field [][]int, shots map[string]string, ship shipState) {
	for _, cell := range ship.Cells {
		field[cell.X][cell.Y] = my_types.SHOOTED
		shots[cellKey(cell.X, cell.Y)] = "Sink"
	}
	for _, cell := range aroundShip(ship.Cells) {
		shots[cellKey(cell.X, cell.Y)] = "Miss"
		if field[cell.X][cell.Y] == my_types.EMPTY {
			field[cell.X][cell.Y] = my_types.FILL
		}
	}
}
