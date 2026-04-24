package game

import (
	"fmt"
	"sea_battle/Game/internal/domain"
	"sea_battle/my_types"
)

// type Bot interface{
// 	Shoot(*domain.Field) domain.Pair
// 	SetResult(my_types.ShotResult)
// }

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
	shot, err := bot.Shoot(field)
    if err != nil {
        return my_types.Miss, fmt.Errorf("bot unavailable: %w\n", err)
    }
    shotRes := Shoot(field, shot.X, shot.Y)
    if err := bot.SetResult(shotRes); err != nil {
        return shotRes, fmt.Errorf("bot unavailable: %w\n", err)
    }
    return shotRes, nil
}
