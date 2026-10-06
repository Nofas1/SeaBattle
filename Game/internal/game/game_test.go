package game

import (
	"errors"
	"io"
	"log/slog"
	"testing"

	"sea_battle/Game/internal/domain"
	"sea_battle/my_types"
)

type stubBot struct {
	shots       []domain.Pair
	shotError   error
	resultError error
	results     []my_types.ShotResult
	shotIndex   int
}

func (bot *stubBot) StartGame() error { return nil }

func (bot *stubBot) Place() (int, int, domain.Pair, error) {
	return 0, 0, domain.Pair{}, nil
}

func (bot *stubBot) Shoot() (domain.Pair, error) {
	if bot.shotError != nil {
		return domain.Pair{}, bot.shotError
	}
	if bot.shotIndex >= len(bot.shots) {
		return domain.Pair{}, errors.New("no stub shots remaining")
	}
	shot := bot.shots[bot.shotIndex]
	bot.shotIndex++
	return shot, nil
}

func (bot *stubBot) SetResult(result my_types.ShotResult) error {
	bot.results = append(bot.results, result)
	return bot.resultError
}

func (bot *stubBot) GameOver(bool) error { return nil }

func TestShootResultsAndFieldMutations(test *testing.T) {
	tests := []struct {
		name       string
		prepare    func(*domain.Field)
		row        int
		col        int
		wantResult my_types.ShotResult
		wantCell   int
	}{
		{
			name:       "miss",
			row:        2,
			col:        3,
			wantResult: my_types.Miss,
			wantCell:   my_types.MISSED,
		},
		{
			name: "hit",
			prepare: func(field *domain.Field) {
				field.Matrix[4][4] = my_types.SHIP
				field.Matrix[4][5] = my_types.SHIP
			},
			row:        4,
			col:        4,
			wantResult: my_types.Hit,
			wantCell:   my_types.SHOOTED,
		},
		{
			name: "sink and mark neighboring cells",
			prepare: func(field *domain.Field) {
				field.Matrix[0][0] = my_types.SHIP
			},
			row:        0,
			col:        0,
			wantResult: my_types.Sink,
			wantCell:   my_types.SHOOTED,
		},
		{
			name: "already shot",
			prepare: func(field *domain.Field) {
				field.Matrix[2][3] = my_types.SHOOTED
			},
			row:        2,
			col:        3,
			wantResult: my_types.Already,
			wantCell:   my_types.SHOOTED,
		},
		{
			name: "filled cell",
			prepare: func(field *domain.Field) {
				field.Matrix[2][3] = my_types.FILL
			},
			row:        2,
			col:        3,
			wantResult: my_types.Already,
			wantCell:   my_types.FILL,
		},
	}

	for _, testCase := range tests {
		test.Run(testCase.name, func(test *testing.T) {
			field := domain.Constructor()
			if testCase.prepare != nil {
				testCase.prepare(field)
			}
			if result := Shoot(field, testCase.row, testCase.col); result != testCase.wantResult {
				test.Fatalf("Shoot() = %v; want %v", result, testCase.wantResult)
			}
			if cell := field.Matrix[testCase.row][testCase.col]; cell != testCase.wantCell {
				test.Errorf("cell = %v; want %v", cell, testCase.wantCell)
			}
		})
	}
}

func TestShootSecondHitSinksShip(test *testing.T) {
	field := domain.Constructor()
	field.Matrix[5][5] = my_types.SHIP
	field.Matrix[5][6] = my_types.SHIP

	if result := Shoot(field, 5, 5); result != my_types.Hit {
		test.Fatalf("first Shoot() = %v; want Hit", result)
	}
	if result := Shoot(field, 5, 6); result != my_types.Sink {
		test.Fatalf("second Shoot() = %v; want Sink", result)
	}
	if field.Matrix[4][4] != my_types.FILL || field.Matrix[6][7] != my_types.FILL {
		test.Errorf("cells surrounding the sunk ship were not filled")
	}
}

func TestUserShotDelegatesToShoot(test *testing.T) {
	field := domain.Constructor()
	if result := UserShot(field, 1, 1); result != my_types.Miss {
		test.Fatalf("UserShot() = %v; want Miss", result)
	}
}

func TestBotShotReturnsShotAndNotifiesBot(test *testing.T) {
	field := domain.Constructor()
	field.Matrix[3][4] = my_types.SHIP
	bot := &stubBot{shots: []domain.Pair{{X: 3, Y: 4}}}

	result, err := BotShot(bot, field, slog.New(slog.NewTextHandler(io.Discard, nil)))
	if err != nil {
		test.Fatalf("BotShot() error = %v", err)
	}
	if result != my_types.Sink {
		test.Errorf("BotShot() result = %v; want Sink", result)
	}
	if len(bot.results) != 1 || bot.results[0] != my_types.Sink {
		test.Errorf("SetResult calls = %v; want [Sink]", bot.results)
	}
}

func TestBotShotRetriesInvalidCoordinates(test *testing.T) {
	field := domain.Constructor()
	bot := &stubBot{shots: []domain.Pair{{X: -1, Y: 0}, {X: 1, Y: 2}}}
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))

	result, err := BotShot(bot, field, logger)
	if err != nil {
		test.Fatalf("BotShot() error = %v", err)
	}
	if result != my_types.Miss {
		test.Errorf("BotShot() result = %v; want Miss", result)
	}
	if len(bot.results) != 2 || bot.results[0] != my_types.Already || bot.results[1] != my_types.Miss {
		test.Errorf("SetResult calls = %v; want [Already Miss]", bot.results)
	}
}

func TestBotShotReturnsBotErrors(test *testing.T) {
	wantErr := errors.New("bot offline")
	bot := &stubBot{shotError: wantErr}
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))

	if _, err := BotShot(bot, domain.Constructor(), logger); !errors.Is(err, wantErr) {
		test.Errorf("BotShot() error = %v; want wrapped %v", err, wantErr)
	}
}

func TestBotShotReturnsSetResultErrors(test *testing.T) {
	wantErr := errors.New("result callback failed")
	bot := &stubBot{
		shots:       []domain.Pair{{X: 1, Y: 1}},
		resultError: wantErr,
	}
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))

	if _, err := BotShot(bot, domain.Constructor(), logger); !errors.Is(err, wantErr) {
		test.Errorf("BotShot() error = %v; want wrapped %v", err, wantErr)
	}
}

func TestBotShotReturnsSetResultErrorWhenRetryingAlreadyShot(test *testing.T) {
	wantErr := errors.New("already callback failed")
	field := domain.Constructor()
	field.Matrix[1][1] = my_types.MISSED
	bot := &stubBot{
		shots:       []domain.Pair{{X: 1, Y: 1}},
		resultError: wantErr,
	}
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))

	if _, err := BotShot(bot, field, logger); !errors.Is(err, wantErr) {
		test.Errorf("BotShot() error = %v; want wrapped %v", err, wantErr)
	}
}
