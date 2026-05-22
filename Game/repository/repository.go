package repository

import (
	"context"
	"fmt"
	"os"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/joho/godotenv"
	"golang.org/x/crypto/bcrypt"
)

type Repo struct {
	pool *pgxpool.Pool
}

type PlayerStats struct {
	Name   string
	Wins   int
	Losses int
}

func NewRepository() (*Repo, error) {
	if err := godotenv.Load(); err != nil {
        return nil, fmt.Errorf("failed to load .env: %w", err)
    }

    connString := os.Getenv("DATABASE_URL")
    if connString == "" {
        return nil, fmt.Errorf("DATABASE_URL not set")
    }

    pool, err := pgxpool.New(context.Background(), connString)
    if err != nil {
        return nil, fmt.Errorf("failed to connect to db: %w", err)
    }

    return &Repo{pool: pool}, nil
}

func (rep *Repo) AddWin(ctx context.Context, name string) error {
	query := `UPDATE sb_history SET wins = wins + 1 JOIN users USING(user_id) WHERE users.name = $1`
	_, err := rep.pool.Exec(ctx, query, name)
	return err
}

func (rep *Repo) AddLoss(ctx context.Context, name string) error {
	query := `UPDATE sb_history SET losses = losses + 1 JOIN users USING(user_id) WHERE users.name = $1`
	_, err := rep.pool.Exec(ctx, query, name)
	return err
}

func (rep *Repo) GetStats(ctx context.Context, name string) (PlayerStats, error) {
	query := `SELECT sb_history.wins, sb_history.losses FROM sb_history JOIN users USING(user_id) WHERE users.name = $1`
	var users_stats PlayerStats
	users_stats.Name = name
	err := rep.pool.QueryRow(ctx, query, name).Scan(&users_stats.Wins, &users_stats.Losses)
	if err != nil {
		return PlayerStats{}, err
	}
	return users_stats, nil
}

func (rep *Repo) RegisterUser(ctx context.Context, name, password string) error {
	hashed_password, err := bcrypt.GenerateFromPassword([]byte(password), 10)
	if err != nil {
		return err
	}
	if err = bcrypt.CompareHashAndPassword(hashed_password, []byte(password)); err != nil {
		return err
	}
	query := `INSERT INTO users (name, hashed_password) VALUES ($1, $2) ON CONFLICT (name) DO NOTHING`
	_, err = rep.pool.Exec(ctx, query, name, hashed_password)
	if err != nil {
		return err
	}
	return nil
}

func (rep *Repo) LoginUser(ctx context.Context, name, password string) error {
	query := `SELECT password FROM users WHERE name = $1`
	var hashed_password []byte
	err := rep.pool.QueryRow(ctx, query, password).Scan(&hashed_password)
	if err != nil {
		return err
	}
	if err = bcrypt.CompareHashAndPassword(hashed_password, []byte(password)); err != nil {
		return err
	}
	return nil
}

func (rep *Repo) UserExists(ctx context.Context, name string) (bool, error) {
	query := `SELECT EXISTS(SELECT name FROM users WHERE name = $1)`
	var exists bool
	err := rep.pool.QueryRow(ctx, query, name).Scan(&exists)
	if err != nil {
		return false, err
	}
	return exists, nil
}

func (rep *Repo) SetMatchDetails(ctx context.Context, name string, win bool) error {
	query := `INSERT INTO matches (user_id, user_win) VALUES((SELECT user_id FROM users WHERE name = $1), $2)`
	_, err := rep.pool.Exec(ctx, query, name, win)
	return err
}

func (rep *Repo) GetLeaderboard(ctx context.Context, limit int) ([]PlayerStats, error) {
	query := `SELECT users.name, sb_history.wins, sb_history.losses FROM sb_history JOIN users USING(user_id) ORDER BY wins DESC LIMIT $1`
	var leaders []PlayerStats
	stats, err := rep.pool.Query(ctx, query, limit)
	if err != nil {
		return []PlayerStats{}, err
	}
	defer stats.Close()

	for stats.Next() {
		var player PlayerStats
		err := stats.Scan(&player.Name, &player.Wins, &player.Losses)
		if err != nil {
			return []PlayerStats{}, err
		}
		leaders = append(leaders, player)
	}

	return leaders, nil
}
