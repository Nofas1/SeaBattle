package game

import (
	"fmt"
	"sea_battle/Game/internal/domain"
	"sea_battle/my_types"
)

type ShotError int

const (
    OutOfBounds ShotError = iota
)

func Shoot(field *domain.Field, row, col int) my_types.ShotResult {
	if field.Matrix[row][col] == my_types.SHOOTED || field.Matrix[row][col] == my_types.MISSED || field.Matrix[row][col] == my_types.FILL {
		return my_types.Already
	}
	if field.Matrix[row][col] == my_types.SHIP {
		field.Matrix[row][col] = my_types.SHOOTED
		if field.IsSunk(row, col) {
			field.FillSunkArea(row, col)
			return my_types.Sink
		}
		return my_types.Hit
	}
	field.Matrix[row][col] = my_types.MISSED
	return my_types.Miss
}

func UserShot(field *domain.Field, row, col int) my_types.ShotResult {
	return Shoot(field, row, col)
}

func BotShot(bot Bot, field *domain.Field) (my_types.ShotResult, error) {
	for {
        shot, err := bot.Shoot()
        if err != nil {
            return my_types.Miss, fmt.Errorf("bot unavailable: %w", err)
        }
		if shot.X < 0 || shot.X >= my_types.Size || shot.Y < 0 || shot.Y >= my_types.Size {
            if err := bot.SetResult(my_types.Already); err != nil {
                return my_types.Miss, fmt.Errorf("bot unavailable: %w", err)
            }
            continue
        }
        shotRes := Shoot(field, shot.X, shot.Y)
        if shotRes == my_types.Already {
            bot.SetResult(shotRes)
            continue
        }
        if err := bot.SetResult(shotRes); err != nil {
            return shotRes, fmt.Errorf("bot unavailable: %w", err)
        }
        return shotRes, nil
    }
}
