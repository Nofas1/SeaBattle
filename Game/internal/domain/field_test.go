package domain

import (
	"testing"

	"sea_battle/my_types"
)

// arrange, act, assert

func TestValidationDefault(test *testing.T) {
	newField := Constructor()

	// arrange
	samples := []struct {
		point    Pair
		expected bool
	}{
		{Pair{X: -1, Y: 0}, false},
		{Pair{X: 0, Y: -1}, false},
		{Pair{X: 10, Y: 0}, false},
		{Pair{X: 0, Y: 10}, false},
		{Pair{X: 5, Y: 5}, true},
		{Pair{X: 9, Y: 8}, true},
	}

	// act

	for _, sample := range samples {
		result := newField.Validation(sample.point)

		// assert
		if result != sample.expected {
			test.Errorf("Validation(%v) = %v; expected %v", sample.point, result, sample.expected)
		}
	}
}

func TestValidationRejectsAdjacentShips(test *testing.T) {
	field := Constructor()
	field.Matrix[4][4] = my_types.SHIP

	for _, point := range []Pair{{X: 3, Y: 3}, {X: 4, Y: 5}, {X: 5, Y: 5}} {
		if field.Validation(point) {
			test.Errorf("Validation(%v) = true; expected false near a ship", point)
		}
	}
	if !field.Validation(Pair{X: 6, Y: 6}) {
		test.Error("Validation rejected a point outside the ship's exclusion zone")
	}
}

func TestConstructorCreatesEmptySquareField(test *testing.T) {
	field := Constructor()
	if len(field.Matrix) != 10 {
		test.Fatalf("row count = %d; want 10", len(field.Matrix))
	}
	for rowIndex, row := range field.Matrix {
		if len(row) != 10 {
			test.Errorf("row %d length = %d; want 10", rowIndex, len(row))
		}
		for columnIndex, cell := range row {
			if cell != 0 {
				test.Errorf("cell [%d][%d] = %d; want empty", rowIndex, columnIndex, cell)
			}
		}
	}
}

func TestPlaceShipDefault(test *testing.T) {
	samples := []struct {
		shipSize int
		dir      int
		point    Pair
		expected bool
	}{
		{shipSize: 3, dir: 0, point: Pair{X: 0, Y: 0}, expected: false},
		{shipSize: 3, dir: 1, point: Pair{X: 0, Y: 0}, expected: true},
		{shipSize: 3, dir: 1, point: Pair{X: 8, Y: 0}, expected: false},
		{shipSize: 4, dir: 2, point: Pair{X: 0, Y: 9}, expected: false},
		{shipSize: 2, dir: 3, point: Pair{X: 0, Y: 8}, expected: false},
	}

	for _, sample := range samples {
		newField := Constructor()
		result := newField.PlaceShip(sample.shipSize, sample.dir, sample.point)

		if result != sample.expected {
			test.Errorf("PlaceShip(%d, %d, %v) = %v; expected %v", sample.shipSize, sample.dir, sample.point, result, sample.expected)
		}
	}
}

func TestPlaceShipDoesNotPartiallyMutateField(test *testing.T) {
	field := Constructor()
	field.Matrix[4][6] = my_types.SHIP

	if field.PlaceShip(3, 2, Pair{X: 4, Y: 4}) {
		test.Fatal("PlaceShip() = true; expected false because the ship touches an existing ship")
	}
	if field.Matrix[4][4] != my_types.EMPTY || field.Matrix[4][5] != my_types.EMPTY || field.Matrix[4][6] != my_types.SHIP {
		test.Errorf("field changed partially after rejected placement: %v", field.Matrix[4])
	}
}

func TestIsSunkChecksRemainingShipCells(test *testing.T) {
	field := Constructor()
	field.Matrix[5][5] = my_types.SHOOTED
	if !field.IsSunk(5, 5) {
		test.Error("IsSunk() = false for an isolated shot ship cell")
	}

	field.Matrix[5][6] = my_types.SHIP
	if field.IsSunk(5, 5) {
		test.Error("IsSunk() = true while an adjacent ship cell remains")
	}
}

func TestFillSunkAreaMarksNeighborsWithoutOverwritingCells(test *testing.T) {
	field := Constructor()
	field.Matrix[4][4] = my_types.SHOOTED
	field.Matrix[4][5] = my_types.SHOOTED
	field.Matrix[3][4] = my_types.MISSED
	field.Matrix[3][5] = my_types.SHIP

	field.FillSunkArea(4, 4)

	if field.Matrix[3][3] != my_types.FILL || field.Matrix[5][6] != my_types.FILL {
		test.Error("empty cells around the sunk ship were not marked as filled")
	}
	if field.Matrix[3][4] != my_types.MISSED || field.Matrix[3][5] != my_types.SHIP {
		test.Error("FillSunkArea overwrote a missed or ship cell")
	}
}

func TestBuildFieldPlacesTenShipsAndReportsFeedback(test *testing.T) {
	requests := make(chan PlaceRequest, 10)
	feedback := make([]chan bool, 10)
	for index := 0; index < 10; index++ {
		feedback[index] = make(chan bool, 1)
		requests <- PlaceRequest{
			ShipSize: 1,
			Dir:      0,
			Point:    Pair{X: (index / 5) * 2, Y: (index % 5) * 2},
			Feedback: feedback[index],
		}
	}
	close(requests)

	field := Constructor()
	if err := field.BuildField(UserPlacer(requests), make(chan struct{})); err != nil {
		test.Fatalf("BuildField() error = %v", err)
	}
	for index, channel := range feedback {
		if accepted := <-channel; !accepted {
			test.Errorf("placement %d was rejected", index)
		}
	}
}

func TestPlaceShipRejectsInvalidDirectionAndSize(test *testing.T) {
	samples := []struct {
		shipSize int
		dir      int
	}{
		{shipSize: 1, dir: -1},
		{shipSize: 1, dir: 4},
		{shipSize: 0, dir: 0},
		{shipSize: -1, dir: 0},
		{shipSize: 11, dir: 0},
	}

	for _, sample := range samples {
		if Constructor().PlaceShip(sample.shipSize, sample.dir, Pair{X: 4, Y: 4}) {
			test.Errorf("PlaceShip(%d, %d) = true; expected false", sample.shipSize, sample.dir)
		}
	}
}

func TestPlaceShip(test *testing.T) {
	newField := Constructor()
	newField.PlaceShip(3, 1, Pair{X: 0, Y: 0})
	newField.PlaceShip(4, 2, Pair{X: 4, Y: 5})
	newField.PlaceShip(2, 0, Pair{X: 2, Y: 8})
	newField.PlaceShip(1, 3, Pair{X: 5, Y: 5})
	newField.PlaceShip(1, 3, Pair{X: 5, Y: 7})

	samples := []struct {
		shipSize int
		dir      int
		point    Pair
		expected bool
	}{
		{shipSize: 3, dir: 0, point: Pair{X: 0, Y: 0}, expected: false},
		{shipSize: 3, dir: 1, point: Pair{X: 0, Y: 0}, expected: false},
		{shipSize: 2, dir: 2, point: Pair{X: 5, Y: 5}, expected: false},
		{shipSize: 2, dir: 3, point: Pair{X: 9, Y: 9}, expected: true},
		{shipSize: 1, dir: 3, point: Pair{X: 7, Y: 7}, expected: true},
	}

	for _, sample := range samples {
		result := newField.PlaceShip(sample.shipSize, sample.dir, sample.point)
		if result != sample.expected {
			test.Errorf("PlaceShip(%d, %d, %v) = %v; expected %v", sample.shipSize, sample.dir, sample.point, result, sample.expected)
		}
	}
}

// func FuzzSumming(test *testing.F) {
// 	test.Fuzz(func(t *testing.T, a, b int) {
// 		result := Summing(a, b)
// 		expected := a + b
// 		if result != expected {
// 			t.Errorf("Summing(%d, %d) = %d; expected %d", a, b, result, expected)
// 		}
// 	})
// }
