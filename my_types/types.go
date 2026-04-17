package my_types

import (
	"math/rand"
	"time"
)

const (
	Size = 10
)

type Cell int

const (
	EMPTY = iota
	SHIP
	SHOOTED
	MISSED
	FILL
)

type ShotResult int

const (
	Miss ShotResult = iota
	Hit
	Sink
	Already
)

const (
	Up = iota
	Right
	Down
	Left
)

type Pair struct {
	X int
	Y int
}

type Field struct {
	Matrix [][]int `json:"field"`
}

var ShipSizes = []int{4, 3, 3, 2, 2, 2, 1, 1, 1, 1}

var GlobalRand = rand.New(rand.NewSource(time.Now().UnixNano()))

var Directions = [][]int{
	{0, -1},
	{1, 0},
	{0, 1},
	{-1, 0},
}
