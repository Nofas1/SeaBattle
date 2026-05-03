package internal

import (
	"log/slog"
	"sea_battle/my_types"
)

type State int

const (
    StateRandom State = iota
    StateSink
)

type SmartBot struct {
    state  State
    memory []my_types.Pair
    dir    *my_types.Pair
    lastShot my_types.Pair
    lastHit my_types.Pair
    logger *slog.Logger
}

func NewSmartBot(logger *slog.Logger) *SmartBot {
	return &SmartBot{
        state: StateRandom,
		memory:   make([]my_types.Pair, 0),
        logger: logger,
	}
}

func (sb *SmartBot) reset() {
    sb.state = StateRandom
	sb.memory = make([]my_types.Pair, 0)
	sb.dir = nil
}

func (sb *SmartBot) Place() (int, int, my_types.Pair) {
    x := my_types.GlobalRand.Intn(my_types.Size)
    y := my_types.GlobalRand.Intn(my_types.Size)
    ind := my_types.GlobalRand.Intn(4)
    dir := my_types.Directions[ind]
    return x, y, my_types.Pair{X: dir[0], Y: dir[1]}
}

func (sb *SmartBot) Shoot(field *my_types.Field) my_types.Pair {
    sb.logger.Debug(
        "shoot called",
        "state", sb.state,
        "memory", sb.memory,
    )
    if sb.state == StateSink {
        for len(sb.memory) > 0 {
            next := sb.memory[0]
            sb.memory = sb.memory[1:]
            if next.X < 0 || next.X >= my_types.Size || next.Y < 0 || next.Y >= my_types.Size {
                sb.logger.Debug(
                    "skip out of bounds",
                    "target", next,
                )
                continue
            }
            if field.Matrix[next.X][next.Y] == my_types.FILL || 
                field.Matrix[next.X][next.Y] == my_types.SHOOTED || 
                field.Matrix[next.X][next.Y] == my_types.MISSED {
                    sb.logger.Debug(
                        "skip already shot",
                        "target", next,
                    )
                    continue
            }
            sb.logger.Info(
                "sink shot",
                "target", next,
                "memory_left", sb.memory,
            )
            sb.lastShot = next
            return next
        }
        sb.reset()
    } else {
        for {
            target := my_types.Pair{
                X: my_types.GlobalRand.Intn(my_types.Size),
                Y: my_types.GlobalRand.Intn(my_types.Size),
            }
            if field.Matrix[target.X][target.Y] == my_types.EMPTY || field.Matrix[target.X][target.Y] == my_types.SHIP {
                sb.logger.Info(
                    "random shot",
                    "target", target,
                )
                sb.lastShot = target
                return target
            }
        }
    }

    return my_types.Pair{}
}

func (sb *SmartBot) SetResult(shotRes my_types.ShotResult) {
    sb.logger.Info(
        "shot result",
        "result", shotRes,
        "last_shot", sb.lastShot,
        "state", sb.state,
    )
    if shotRes == my_types.Already {
        return
    }
    if shotRes == my_types.Sink {
        sb.reset()
        sb.logger.Debug(
        "reset after sink",
        "memory", sb.memory,
        )
    }
    if shotRes == my_types.Hit {
        sb.lastHit = sb.lastShot
        if sb.state == StateRandom {
            sb.state = StateSink
            sb.memory = []my_types.Pair{sb.lastShot}
            for _, d := range my_types.Directions {
                sb.memory = append(sb.memory, my_types.Pair{
                    X: sb.lastShot.X + d[0],
                    Y: sb.lastShot.Y + d[1],
                })
            }
            sb.logger.Debug(
                "first hit",
                "first_shot", sb.lastShot,
                "memory", sb.memory,
            )
        } else {
            var dir my_types.Pair
            if len(sb.memory) == 0 {
                dx := sb.lastShot.X - sb.lastHit.X
                if dx != 0 {
                    dir = my_types.Pair{X: 0, Y: 1}
                } else {
                    dir = my_types.Pair{X: 1, Y: 0}
                }
                sb.dir = &dir
                sb.logger.Info("", "dir", sb.dir)
                
            } else {
                first := sb.memory[0]
                dx := sb.lastShot.X - first.X
                if dx != 0 {
                    dir = my_types.Pair{X: 0, Y: 1}
                } else {
                    dir = my_types.Pair{X: 1, Y: 0}
                }
                sb.dir = &dir
            }


            sb.memory = append(sb.memory, my_types.Pair{
                X: sb.lastShot.X + dir.X,
                Y: sb.lastShot.Y + dir.Y,
            })

            sb.memory = append(sb.memory, my_types.Pair{
                X: sb.lastShot.X - dir.X,
                Y: sb.lastShot.Y - dir.Y,
            })

            first := sb.memory[0]
            sb.logger.Debug(
                "direction determined",
                "dir", dir,
                "first", first,
                "last", sb.lastShot,
            )

            filtered_memory := []my_types.Pair{first}
            for _, t := range sb.memory[1:] {
                tdx := t.X - first.X
                tdy := t.Y - first.Y
                if (dir.X != 0 && tdx != 0) || (dir.Y != 0 && tdy != 0) {
                    filtered_memory = append(filtered_memory, t)
                }
            }
            sb.memory = filtered_memory
            sb.logger.Debug(
                "filtered memory",
                "memory", sb.memory,
            )
        }
    }
    if shotRes == my_types.Miss {
        sb.logger.Debug(
            "miss",
            "memory", sb.memory,
            "dir", sb.dir,
        )
    }
}
