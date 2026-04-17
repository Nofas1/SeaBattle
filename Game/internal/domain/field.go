package domain

import (
	"sea_battle/my_types"	
)

func Constructor() *Field {
	m := make([][]int, my_types.Size)
	for i := range m {
		m[i] = make([]int, my_types.Size)
	}
	return &Field{Matrix: m}
}

type PlacerFunc func() <-chan PlaceRequest

func RandomPlacer() <-chan PlaceRequest {
	ch := make(chan PlaceRequest)
	go func() {
		defer close(ch)
		for i := 0; i < len(my_types.ShipSizes); i++ {
			feedback := make(chan bool)
			for {
				ch <- PlaceRequest{
					ShipSize: my_types.ShipSizes[i],
					Dir:      my_types.GlobalRand.Intn(4),
					Point:    Pair{X: my_types.GlobalRand.Intn(10), Y: my_types.GlobalRand.Intn(10)},
					Feedback: feedback,
				}
				if check := <-feedback; check {
					break
				}
			}
			close(feedback)
		}
	}()
	return ch
}

func UserPlacer(input <-chan PlaceRequest) PlacerFunc {
	return func() <-chan PlaceRequest {
		return input
	}
}

func (f *Field) Validation(point Pair) bool {
	if point.X < 0 || point.X >= my_types.Size || point.Y < 0 || point.Y >= my_types.Size {
		return false
	}
	for i := point.X - 1; i < point.X+2; i++ {
		for j := point.Y - 1; j < point.Y+2; j++ {
			if i < 0 || i >= my_types.Size || j < 0 || j >= my_types.Size {
				continue
			}
			if f.Matrix[i][j] == my_types.SHIP {
				return false
			}
		}
	}
	return true
}

func (f *Field) PlaceShip(ship, dir int, point Pair) bool {
	var cells []Pair
	myDir := my_types.Directions[dir]

	for i := 0; i < ship; i++ {
		p := Pair{X: point.X + myDir[0]*i, Y: point.Y + myDir[1]*i}
		if !f.Validation(p) {
			return false
		}
		cells = append(cells, p)
	}

	if cells == nil {
		return false
	}
	for _, cell := range cells {
		f.Matrix[cell.X][cell.Y] = my_types.SHIP
	}
	return true
}

func (f *Field) BuildField(placer PlacerFunc, cancel <-chan struct{}) error {
	requests := placer()
	for cnt := 0; cnt < len(my_types.ShipSizes); {
		select {
		case req, ok := <-requests:
			if !ok {
				return nil
			}
			if f.PlaceShip(req.ShipSize, req.Dir, req.Point) {
				req.Feedback <- true
				cnt++
			} else {
				req.Feedback <- false
			}
		case <-cancel:
			return nil
		}
	}
	return nil
}

func (f *Field) FillSunkArea(row, col int) {
	shipCells := []Pair{{X: row, Y: col}}
	for _, d := range my_types.Directions {
		newRow, newCol := row+d[0], col+d[1]
		for newRow >= 0 && newRow < my_types.Size && newCol >= 0 && newCol < my_types.Size {
			if f.Matrix[newRow][newCol] != my_types.SHOOTED {
				break
			}
			shipCells = append(shipCells, Pair{X: newRow, Y: newCol})
			newRow += d[0]
			newCol += d[1]
		}
	}

	for _, cell := range shipCells {
		for i := cell.X - 1; i <= cell.X+1; i++ {
			for j := cell.Y - 1; j <= cell.Y+1; j++ {
				if i >= 0 && i < my_types.Size && j >= 0 && j < my_types.Size {
					if f.Matrix[i][j] == my_types.EMPTY {
						f.Matrix[i][j] = my_types.FILL
					}
				}
			}
		}
	}
}

func (f *Field) IsSunk(row, col int) bool {
	for _, d := range my_types.Directions {
		newRow, newCol := row+d[0], col+d[1]
		for newCol < my_types.Size && newCol >= 0 && newRow < my_types.Size && newRow >= 0 {
			if f.Matrix[newRow][newCol] == my_types.SHIP {
				return false
			}
			if f.Matrix[newRow][newCol] == my_types.EMPTY || f.Matrix[newRow][newCol] == my_types.MISSED {
				break
			}
			newRow += d[0]
			newCol += d[1]
		}
	}

	return true
}
