package internal

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"sea_battle/my_types"
	"sea_battle/smart_bot/internal/repository"

	"github.com/jackc/pgx/v4"
)

type SmartBot struct {
    rep *repository.Repo
    logger *slog.Logger
}

func NewSmartBot(logger *slog.Logger) *SmartBot {
    rep, err := repository.NewRepository()
    if err != nil {
        logger.Error("failed to initialize repository", "error", err)
        panic("failed to connect to database")
    }
    logger.Info("smart bot initialized successfully")
	return &SmartBot{
        rep: rep,
        logger: logger,
	}
}

func (sb *SmartBot) StartGame(user_key string) error {
    ctx := context.Background()

    if err := sb.rep.ClearState(ctx, user_key); err != nil {
        return fmt.Errorf("failed to clear state at start game: %w", err)
    }
    if err := sb.rep.InitState(ctx, user_key); err != nil {
        return fmt.Errorf("failed to initialize start state at start game: %w", err)
    }

    sb.logger.Info(
        "game started, state cleared, start state initialized",
        "user_key", user_key,
    )
    return nil
}

func (sb *SmartBot) Place() (int, int, my_types.Pair) {
    x := my_types.GlobalRand.Intn(my_types.Size)
    y := my_types.GlobalRand.Intn(my_types.Size)
    ind := my_types.GlobalRand.Intn(4)
    dir := my_types.Directions[ind]
    return x, y, my_types.Pair{X: dir[0], Y: dir[1]}
}

func (sb *SmartBot) Shoot(userKey string) (my_types.Pair, error) {
    ctx := context.Background()
    state, err := sb.rep.GetState(ctx, userKey)
    if err != nil {
        if errors.Is(err, pgx.ErrNoRows) {
            state = repository.BotState{State: repository.StateRandom}
        } else {
            return my_types.Pair{}, fmt.Errorf("failed to get state: %w", err)
        }
    }

    sb.logger.Info(
        "shoot state loaded",
        "state", state.State,
        "memory_len", len(state.Memory),
        "last_shot", state.LastShot,
    )

    sb.logger.Debug(
        "shoot called",
        "state", state.State,
        "memory", state.Memory,
    )

    if state.State == repository.StateSink {
        if len(state.Memory) > 0 {
            next := state.Memory[0]
            state.Memory = state.Memory[1:]
            state.LastShot = next

            if err := sb.rep.SetState(ctx, userKey, state); err != nil {
                return my_types.Pair{}, fmt.Errorf("failed to save state: %w", err)
            }
            sb.logger.Info("sink shot", "target", next, "memory_left", state.Memory)
            return next, nil
        }
        
        if err := sb.rep.ClearState(ctx, userKey); err != nil {
            return my_types.Pair{}, fmt.Errorf("failed to reset: %w", err)
        }
    }

    target := my_types.Pair{
        X: my_types.GlobalRand.Intn(my_types.Size),
        Y: my_types.GlobalRand.Intn(my_types.Size),
    }
    state.LastShot = target
    if err := sb.rep.SetState(ctx, userKey, state); err != nil {
        return my_types.Pair{}, fmt.Errorf("failed to save state: %w", err)
    }
    sb.logger.Info("random shot", "target", target)
    return target, nil
}

func (sb *SmartBot) SetResult(userKey string, shotRes my_types.ShotResult) {
    ctx := context.Background()

    if shotRes == my_types.Already {
        state, _ := sb.rep.GetState(ctx, userKey)

        if len(state.Memory) > 0 {
            state.Memory = state.Memory[1:]
            sb.rep.SetState(ctx, userKey, state)
        }

        return
    }

    if shotRes == my_types.Sink {
        sb.logger.Debug("reset after sink")
        sb.rep.ClearState(ctx, userKey)
        return
    }

    state, err := sb.rep.GetState(ctx, userKey)
    if err != nil {
        state = repository.BotState{}
    }

    sb.logger.Info(
        "set_result called",
        "result", shotRes,
        "current_state", state.State,
        "last_shot", state.LastShot,
    )

    if shotRes == my_types.Hit {
        if state.State == repository.StateRandom {
            state.State = repository.StateSink
            state.Memory = []my_types.Pair{}
            for _, d := range my_types.Directions {
                state.Memory = append(state.Memory, my_types.Pair{
                    X: state.LastShot.X + d[0],
                    Y: state.LastShot.Y + d[1],
                })
            }
            state.LastHit = state.LastShot

            sb.logger.Debug(
                "first hit",
                "first_shot", state.LastShot,
                "memory", state.Memory,
            )

        } else {
            dx := state.LastShot.X - state.LastHit.X
            var dir my_types.Pair
            if dx != 0 {
                dir = my_types.Pair{X: 1, Y: 0}
            } else {
                dir = my_types.Pair{X: 0, Y: 1}
            }
            state.Dir = &dir

            state.Memory = append(state.Memory, my_types.Pair{
                X: state.LastShot.X + dir.X,
                Y: state.LastShot.Y + dir.Y,
            })
            state.Memory = append(state.Memory, my_types.Pair{
                X: state.LastShot.X - dir.X,
                Y: state.LastShot.Y - dir.Y,
            })

            filtered := []my_types.Pair{}
            for _, t := range state.Memory {
                tdx := t.X - state.LastHit.X
                tdy := t.Y - state.LastHit.Y
                if (dir.X != 0 && tdx != 0) || (dir.Y != 0 && tdy != 0) {
                    filtered = append(filtered, t)
                }
            }
            state.Memory = filtered
            state.LastHit = state.LastShot

            sb.logger.Debug(
                "filtered memory",
                "memory", state.Memory,
            )
        }
    }

    sb.logger.Info("saving state after hit",
        "new_state", state.State,
        "memory", state.Memory,
    )

    sb.rep.SetState(ctx, userKey, state)
}
