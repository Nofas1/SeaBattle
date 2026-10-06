package internal

import (
	"testing"

	"sea_battle/my_types"
)

func TestParseShotResponse(test *testing.T) {
	tests := []struct {
		name      string
		response  string
		want      my_types.Pair
		wantError bool
	}{
		{name: "valid coordinates", response: " 3\t7\n", want: my_types.Pair{X: 3, Y: 7}},
		{name: "missing coordinate", response: "3", wantError: true},
		{name: "extra text", response: "3 7 shoot", wantError: true},
		{name: "invalid coordinate", response: "x 7", wantError: true},
		{name: "negative coordinate", response: "-1 7", wantError: true},
		{name: "out of bounds", response: "3 10", wantError: true},
	}
	for _, testCase := range tests {
		test.Run(testCase.name, func(test *testing.T) {
			got, err := parseShotResponse(testCase.response)
			if (err != nil) != testCase.wantError {
				test.Fatalf("parseShotResponse(%q) error = %v; wantError=%v", testCase.response, err, testCase.wantError)
			}
			if err == nil && got != testCase.want {
				test.Errorf("parseShotResponse(%q) = %+v; want %+v", testCase.response, got, testCase.want)
			}
		})
	}
}

func TestFallbackShotFindsAvailableCell(test *testing.T) {
	field := &my_types.Field{Matrix: make([][]int, my_types.Size)}
	for rowIndex := range field.Matrix {
		field.Matrix[rowIndex] = make([]int, my_types.Size)
		for columnIndex := range field.Matrix[rowIndex] {
			field.Matrix[rowIndex][columnIndex] = my_types.MISSED
		}
	}
	field.Matrix[6][8] = my_types.SHIP

	if shot := fallbackShot(field); shot != (my_types.Pair{X: 6, Y: 8}) {
		test.Errorf("fallbackShot() = %+v; want (6, 8)", shot)
	}
}

func TestFallbackShotRejectsInvalidOrExhaustedField(test *testing.T) {
	if shot := fallbackShot(nil); shot != (my_types.Pair{X: -1, Y: -1}) {
		test.Errorf("fallbackShot(nil) = %+v; want invalid coordinates", shot)
	}
	if shot := fallbackShot(&my_types.Field{Matrix: [][]int{{my_types.EMPTY}}}); shot != (my_types.Pair{X: -1, Y: -1}) {
		test.Errorf("fallbackShot(malformed field) = %+v; want invalid coordinates", shot)
	}

	field := &my_types.Field{Matrix: make([][]int, my_types.Size)}
	for rowIndex := range field.Matrix {
		field.Matrix[rowIndex] = make([]int, my_types.Size)
		for columnIndex := range field.Matrix[rowIndex] {
			field.Matrix[rowIndex][columnIndex] = my_types.MISSED
		}
	}
	if shot := fallbackShot(field); shot != (my_types.Pair{X: -1, Y: -1}) {
		test.Errorf("fallbackShot(exhausted field) = %+v; want invalid coordinates", shot)
	}
}
