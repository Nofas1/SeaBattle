package domain

type Pair struct {
	X int
	Y int
}

type Field struct {
	Matrix [][]int
}

type PlaceRequest struct {
	ShipSize int
	Dir      int
	Point    Pair
	Feedback chan bool
}
