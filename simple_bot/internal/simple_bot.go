package internal

import (
	"sea_battle/my_types"
)

type SimpleBot struct {}

func NewSimpleBot() *SimpleBot {
	return &SimpleBot{}
}

func (sb *SimpleBot) Shoot(field *my_types.Field) my_types.Pair {
	for {
		target := my_types.Pair{X: my_types.GlobalRand.Intn(10), Y: my_types.GlobalRand.Intn(10)}
		row, col := target.X, target.Y
		if field.Matrix[row][col] == my_types.EMPTY || field.Matrix[row][col] == my_types.SHIP {
			return target
		}
	}
}

func (sb *SimpleBot) SetResult(my_types.ShotResult) {}

func (sb *SimpleBot) Place() (int, int, my_types.Pair) {
    x := my_types.GlobalRand.Intn(my_types.Size)
    y := my_types.GlobalRand.Intn(my_types.Size)
    ind := my_types.GlobalRand.Intn(4)
    dir := my_types.Directions[ind]
    return x, y, my_types.Pair{X: dir[0], Y: dir[1]}
}