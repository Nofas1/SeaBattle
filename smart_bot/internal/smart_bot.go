package internal

import (
    "sea_battle/my_types"
)

type SmartBot struct {
	targets     []my_types.Pair
	last_shot   my_types.Pair
	dir         []int
	first_shot  my_types.Pair
	try_to_sink bool
}

func NewSmartBot() *SmartBot {
	return &SmartBot{
		targets:   make([]my_types.Pair, 0),
		last_shot: my_types.Pair{},
	}
}

func (sb *SmartBot) reset() {
	sb.targets = make([]my_types.Pair, 0)
	sb.dir = nil
	sb.first_shot = my_types.Pair{}
	sb.try_to_sink = false
}

func (sb *SmartBot) Shoot(field *my_types.Field) my_types.Pair {
	if len(sb.targets) == 0 && sb.try_to_sink && sb.dir != nil {
		dx, dy := sb.dir[0], sb.dir[1]
		sb.addIfValid(my_types.Pair{
			X: sb.first_shot.X - dx,
			Y: sb.first_shot.Y - dy,
		})
		sb.dir = nil
	}
	if len(sb.targets) > 0 {
		sb.last_shot = sb.targets[0]
		sb.targets = sb.targets[1:]
		return sb.last_shot
	} else {
		for {
			target := my_types.Pair{X: my_types.GlobalRand.Intn(10), Y: my_types.GlobalRand.Intn(10)}
			row, col := target.X, target.Y
			if field.Matrix[row][col] == my_types.EMPTY || field.Matrix[row][col] == my_types.SHIP {
				sb.last_shot = target
				return target
			}
		}
	}
}

func (sb *SmartBot) Place() (int, int, my_types.Pair) {
    x := my_types.GlobalRand.Intn(my_types.Size)
    y := my_types.GlobalRand.Intn(my_types.Size)
    ind := my_types.GlobalRand.Intn(4)
    dir := my_types.Directions[ind]
    return x, y, my_types.Pair{X: dir[0], Y: dir[1]}
}


func (sb *SmartBot) addIfValid(p my_types.Pair) {
    row, col := p.X, p.Y
    if row < 0 || row >= my_types.Size || col < 0 || col >= my_types.Size {
        return
    }
    // if field.Matrix[row][col] == my_types.EMPTY || field.Matrix[row][col] == my_types.SHIP {
    //     sb.targets = append(sb.targets, p)
    // }
}

func (sb *SmartBot) SetResult(shotRes my_types.ShotResult) {
    if shotRes == my_types.Already {
        return
    }
    if shotRes == my_types.Sink {
        sb.reset()
        return
    }
    if shotRes == my_types.Hit {
        if !sb.try_to_sink {
            sb.try_to_sink = true
            sb.first_shot = sb.last_shot
            for _, d := range my_types.Directions {
                sb.addIfValid(my_types.Pair{
                    X: sb.last_shot.X + d[0],
                    Y: sb.last_shot.Y + d[1],
                })
            }
        } else {
            dx := sb.last_shot.X - sb.first_shot.X
            if dx != 0 {
                sb.dir = []int{1, 0}
            } else {
                sb.dir = []int{0, 1}
            }
            sb.addIfValid(my_types.Pair{
                X: sb.last_shot.X + sb.dir[0],
                Y: sb.last_shot.Y + sb.dir[1],
            })
            filtered := sb.targets[:0]
            for _, t := range sb.targets {
                tdx := t.X - sb.first_shot.X
                tdy := t.Y - sb.first_shot.Y
                if (sb.dir[0] != 0 && tdx != 0) || (sb.dir[1] != 0 && tdy != 0) {
                    filtered = append(filtered, t)
                }
            }
            sb.targets = filtered
        }
        return
    }
    if shotRes == my_types.Miss {
        if sb.try_to_sink && sb.dir != nil && len(sb.targets) == 0 {
            sb.addIfValid(my_types.Pair{
                X: sb.first_shot.X - sb.dir[0],
                Y: sb.first_shot.Y - sb.dir[1],
            })
        }
    }
}
