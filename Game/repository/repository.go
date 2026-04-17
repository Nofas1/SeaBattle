package repository

import (
	"context"

	"github.com/jackc/pgx/v5/pgxpool"
)

type Repo struct {
	pool *pgxpool.Pool
}

type PlayerStats struct {
	Name   string
	Wins   int
	Losses int
}

func (rep *Repo) AddWin(ctx context.Context, name string) error {
	query := `UPDATE SB_History SET wins = wins + 1 WHERE name = $1`
	_, err := rep.pool.Exec(ctx, query, name)
	return err
}

func (rep *Repo) AddLoss(ctx context.Context, name string) error {
	query := `UPDATE SB_History SET losses = losses + 1 WHERE name = $1`
	_, err := rep.pool.Exec(ctx, query, name)
	return err
}

func (rep *Repo) GetStats(ctx context.Context, name string) (PlayerStats, error) {
	query := `SELECT name, wins, losses FROM SB_History WHERE name = $1`
	var users_stats PlayerStats
	err := rep.pool.QueryRow(ctx, query, name).Scan(&users_stats.Name, &users_stats.Wins, &users_stats.Losses)
	if err != nil {
		return PlayerStats{}, err
	}
	return users_stats, nil
}

func (rep *Repo) RegisterUser(ctx context.Context, name string) error {
	query := `INSERT INTO SB_History (name) VALUES ($1) ON CONFLICT (name) DO NOTHING`
	_, err := rep.pool.Exec(ctx, query, name)
	if err != nil {
		return err
	}
	return nil
}

func (rep *Repo) UserExists(ctx context.Context, name string) (bool, error) {
	query := `SELECT EXISTS(SELECT name FROM SB_History WHERE name = $1)`
	var exists bool
	err := rep.pool.QueryRow(ctx, query, name).Scan(&exists)
	if err != nil {
		return false, err
	}
	return exists, nil
}

func (rep *Repo) SetMatchDetails(ctx context.Context, name string, win bool, shots, hits int) error {
	query := `INSERT INTO matches (user_id, user_win, shots, hit_rate) VALUES((SELECT id FROM SB_History WHERE name = $1), $2, $3,
	CASE WHEN $3 > 0 THEN $4::float / $3 ELSE 0 END)`
	_, err := rep.pool.Exec(ctx, query, name, win, shots, hits)
	return err
}

func (rep *Repo) GetLeaderboard(ctx context.Context, limit int) ([]PlayerStats, error) {
	query := `SELECT name, wins, losses FROM SB_History ORDER BY wins DESC LIMIT $1`
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
