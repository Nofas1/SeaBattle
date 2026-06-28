package repository

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"sea_battle/smart_bot/db"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/jmoiron/sqlx"
	"github.com/joho/godotenv"
)

type Repo struct {
	pool *pgxpool.Pool
}

type Stats struct {
	Wins int
	Loses int
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

	pool, err := pgxpool.New(context.Background(), connString)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to db: %w", err)
	}

	return &Repo{pool: pool}, nil
}

func (rep *Repo) InitState(ctx context.Context, name string) error {
	query := `INSERT INTO sb_history (user_id, wins, loses) VALUES ((SELECT user_id FROM users WHERE name=$1), $2, $3)`

	if _, err := rep.pool.Exec(ctx, query, name, 0, 0); err != nil {
		return fmt.Errorf("failed to initialize start sb_history: %w", err)
	}
	return nil
}

// saves the complete match result for a user
func (rep *Repo) SetResult(ctx context.Context, name string, user_win bool) error {
	rep.InitState(ctx, name)

	match_date := time.Now()
	query := `INSERT INTO matches (user_id, user_win, match_date) VALUES ((SELECT user_id FROM users WHERE name=$1), $2, $3)`

	_, err := rep.pool.Exec(ctx, query, name, user_win, match_date)
	if err != nil {
		return fmt.Errorf("failed to insert match result: %w", err)
	}

	var sb_history_query string
	if user_win {
		sb_history_query = `UPDATE sb_history SET wins = wins + 1 
							WHERE user_id = (SELECT user_id FROM users WHERE name=$1)`
	} else {
		sb_history_query = `UPDATE sb_history SET loses = loses + 1 
							WHERE user_id = (SELECT user_id FROM users WHERE name=$1)`
	}

	_, err = rep.pool.Exec(ctx, sb_history_query, name)
	if err != nil {
		return fmt.Errorf("failed to update sb_history: %w", err)
	}

	return nil
}

// retrieves the whole result for a user
func (rep *Repo) GetResult(ctx context.Context, name string) (Stats, error) {
	query := `
        SELECT wins, loses
        FROM sb_history WHERE user_id = (SELECT user_id FROM users WHERE name=$1)`

	var wins, loses int

	err := rep.pool.QueryRow(ctx, query, name).Scan(
		&wins, &loses,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return Stats{}, nil // fresh session
		}
		return Stats{}, fmt.Errorf("failed to get stats: %w", err)
	}

	return Stats{
		Wins: wins,
		Loses: loses,
	}, nil
}
