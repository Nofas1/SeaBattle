package internal

import (
	"io"
	"log/slog"
	"testing"

	"sea_battle/my_types"
)

func TestShootChoosesAnAvailableCell(test *testing.T) {
	field := &my_types.Field{Matrix: make([][]int, my_types.Size)}
	for rowIndex := range field.Matrix {
		field.Matrix[rowIndex] = make([]int, my_types.Size)
		for columnIndex := range field.Matrix[rowIndex] {
			field.Matrix[rowIndex][columnIndex] = my_types.MISSED
		}
	}
	field.Matrix[my_types.Size-1][my_types.Size-1] = my_types.SHIP

	bot := NewSimpleBot(slog.New(slog.NewTextHandler(io.Discard, nil)))
	shot := bot.Shoot(field)
	if shot.X != my_types.Size-1 || shot.Y != my_types.Size-1 {
		test.Errorf("Shoot() = %+v; want the only unshot ship cell", shot)
	}
}

func TestPlaceReturnsInBoundsPointAndCardinalDirection(test *testing.T) {
	bot := NewSimpleBot(slog.New(slog.NewTextHandler(io.Discard, nil)))
	validDirections := map[my_types.Pair]bool{
		{X: 0, Y: -1}: true,
		{X: 1, Y: 0}:  true,
		{X: 0, Y: 1}:  true,
		{X: -1, Y: 0}: true,
	}

	for attempt := 0; attempt < 100; attempt++ {
		x, y, direction := bot.Place()
		if x < 0 || x >= my_types.Size || y < 0 || y >= my_types.Size {
			test.Fatalf("Place() point = (%d, %d); out of bounds", x, y)
		}
		if !validDirections[direction] {
			test.Fatalf("Place() direction = %+v; not cardinal", direction)
		}
	}
}

func TestSetResultDoesNotPanic(test *testing.T) {
	NewSimpleBot(slog.New(slog.NewTextHandler(io.Discard, nil))).SetResult(my_types.Hit)
}
