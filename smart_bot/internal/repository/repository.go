package repository

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"sea_battle/my_types"
	"sea_battle/smart_bot/db"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/jmoiron/sqlx"
	"github.com/joho/godotenv"
)

type Repo struct {
	pool *pgxpool.Pool
}

// current shooting mode of the bot
type State int

const (
	StateRandom State = iota // bot is shooting randomly, looking for a ship
	StateSink                // bot has found a ship and is trying to sink it
)

type BotState struct {
	State    State           // current shooting mode
	Memory   []my_types.Pair // queue of coordinates to try next
	Dir      *my_types.Pair  // direction of the ship once identified
	LastShot my_types.Pair   // coordinates of the most recent shot
	LastHit  my_types.Pair   // coordinates of the first hit on current ship
}

// loads database config from .env and creates a connection pool
func NewRepository(logger *slog.Logger) (*Repo, error) {
	if err := godotenv.Load(); err != nil {
		return nil, fmt.Errorf("failed to load .env: %w", err)
	}

	connString := os.Getenv("DATABASE_URL")
	if connString == "" {
		return nil, fmt.Errorf("DATABASE_URL not set")
	}
    conn, err := sqlx.ConnectContext(context.Background(), "pgx", connString)
    if err != nil {
        return nil, fmt.Errorf("faile to sqlx connect: %w", err)
    }
    defer conn.Close()

	migrator := db.NewMigrator(logger, conn)
	if err := migrator.Migrate(); err != nil {
		return nil, fmt.Errorf("failed to run migrations: %w", err)
	}

	// fmt.Println("DATABASE_URL =", connString)

	pool, err := pgxpool.New(context.Background(), connString)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to db: %w", err)
	}

	return &Repo{pool: pool}, nil
}

// deletes all state and memory for a given user session
// called when a ship is sunk or the game resets
func (rep *Repo) ClearState(ctx context.Context, user_key string) error {
	query := `DELETE FROM states WHERE user_key = $1`
	_, err := rep.pool.Exec(ctx, query, user_key)
	if err != nil {
		return fmt.Errorf("failed to delete state: %w", err)
	}
	return nil
}

func (rep *Repo) ClearMemory(ctx context.Context, user_key string) error {
	query := `DELETE FROM memory WHERE states_id = (SELECT states_id FROM states WHERE user_key = $1)`
	_, err := rep.pool.Exec(ctx, query, user_key)
	if err != nil {
		return fmt.Errorf("failed to delete memory: %w", err)
	}
	return nil
}

func (rep *Repo) InitState(ctx context.Context, user_key string) error {
	query := `INSERT INTO states (user_key, bot_state, direction_x, direction_y,
        last_shot_x, last_shot_y, last_hit_x, last_hit_y)
        VALUES ($1, false, 0, 0, -1, -1, -1, -1)`

	if _, err := rep.pool.Exec(ctx, query, user_key); err != nil {
		return fmt.Errorf("failed to initialize start state: %w", err)
	}
	return nil
}

// saves the complete bot state for a user session
func (rep *Repo) SetState(ctx context.Context, user_key string, bot_state BotState) error {
	rep.ClearMemory(ctx, user_key)
	rep.InitState(ctx, user_key)

	var dirX, dirY int
	if bot_state.Dir != nil {
		dirX = bot_state.Dir.X
		dirY = bot_state.Dir.Y
	}

	query :=
		`UPDATE states SET bot_state=$2, direction_x=$3, direction_y=$4,
        last_shot_x=$5, last_shot_y=$6, last_hit_x=$7, last_hit_y=$8
        WHERE user_key=$1
        RETURNING states_id`

	var statesID int

	err := rep.pool.QueryRow(ctx, query,
		user_key,
		bot_state.State == StateSink,
		dirX, dirY,
		bot_state.LastShot.X, bot_state.LastShot.Y,
		bot_state.LastHit.X, bot_state.LastHit.Y,
	).Scan(&statesID)
	if err != nil {
		return fmt.Errorf("failed to insert state: %w", err)
	}

	memory_query := `INSERT INTO memory (states_id, coord_x, coord_y) VALUES ($1, $2, $3)`

	for _, pair := range bot_state.Memory {
		_, err := rep.pool.Exec(ctx, memory_query, statesID, pair.X, pair.Y)
		if err != nil {
			return fmt.Errorf("failed to insert memory: %w", err)
		}
	}
	return nil
}

// retrieves the bot state for a user session
func (rep *Repo) GetState(ctx context.Context, user_key string) (BotState, error) {
	query := `
        SELECT states_id, bot_state, direction_x, direction_y,
            last_shot_x, last_shot_y, last_hit_x, last_hit_y
        FROM states WHERE user_key = $1`

	var statesID int
	var isSink bool
	var dirX, dirY, lsX, lsY, lhX, lhY int

	err := rep.pool.QueryRow(ctx, query, user_key).Scan(
		&statesID, &isSink,
		&dirX, &dirY,
		&lsX, &lsY,
		&lhX, &lhY,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return BotState{State: StateRandom}, nil // fresh session
		}
		return BotState{}, fmt.Errorf("failed to get state: %w", err)
	}

	state := StateRandom
	if isSink {
		state = StateSink
	}

	var dir *my_types.Pair
	if dirX != 0 || dirY != 0 {
		dir = &my_types.Pair{X: dirX, Y: dirY}
	}

	memory_query := `SELECT coord_x, coord_y FROM memory WHERE states_id = $1 ORDER BY memory_id`
	rows, err := rep.pool.Query(ctx, memory_query, statesID)
	if err != nil {
		return BotState{}, fmt.Errorf("failed to get memory: %w", err)
	}
	defer rows.Close()

	var memory []my_types.Pair
	for rows.Next() {
		var p my_types.Pair
		if err := rows.Scan(&p.X, &p.Y); err != nil {
			return BotState{}, fmt.Errorf("failed to scan memory: %w", err)
		}
		memory = append(memory, p)
	}

	return BotState{
		State:    state,
		Memory:   memory,
		Dir:      dir,
		LastShot: my_types.Pair{X: lsX, Y: lsY},
		LastHit:  my_types.Pair{X: lhX, Y: lhY},
	}, nil
}
