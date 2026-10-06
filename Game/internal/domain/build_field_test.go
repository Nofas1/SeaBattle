package domain

import (
	"testing"
	"time"

	"sea_battle/my_types"
)

func TestRandomPlacerProducesEveryShipAndCloses(test *testing.T) {
	requests := RandomPlacer()
	for index, expectedSize := range my_types.ShipSizes {
		select {
		case request, ok := <-requests:
			if !ok {
				test.Fatalf("placer closed after %d ships", index)
			}
			if request.ShipSize != expectedSize {
				test.Errorf("ship %d size = %d; want %d", index, request.ShipSize, expectedSize)
			}
			if request.Point.X < 0 || request.Point.X >= my_types.Size || request.Point.Y < 0 || request.Point.Y >= my_types.Size {
				test.Errorf("ship %d point = %+v; out of bounds", index, request.Point)
			}
			if request.Dir < 0 || request.Dir >= len(my_types.Directions) {
				test.Errorf("ship %d direction = %d; out of bounds", index, request.Dir)
			}
			request.Feedback <- true
		case <-time.After(time.Second):
			test.Fatalf("placer did not produce ship %d", index)
		}
	}
	select {
	case _, ok := <-requests:
		if ok {
			test.Error("placer produced more ships than configured")
		}
	case <-time.After(time.Second):
		test.Error("placer did not close its request channel")
	}
}

func TestBuildFieldRejectsPrematurePlacerClose(test *testing.T) {
	requests := make(chan PlaceRequest)
	close(requests)

	err := Constructor().BuildField(UserPlacer(requests), make(chan struct{}))
	if err == nil {
		test.Fatal("BuildField() error = nil; expected an incomplete-placement error")
	}
}

func TestBuildFieldStopsWhenCanceled(test *testing.T) {
	cancel := make(chan struct{})
	close(cancel)
	requests := make(chan PlaceRequest)

	if err := Constructor().BuildField(UserPlacer(requests), cancel); err != nil {
		test.Errorf("BuildField() error = %v; want nil after cancellation", err)
	}
}
