package internal

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"testing"

	"sea_battle/my_types"
	"sea_battle/smart_bot/internal/repository"

	"github.com/jackc/pgx/v4"
)

type memoryRepository struct {
	state       repository.BotState
	stateExists bool
	getError    error
	clearError  error
	initError   error
	setError    error
	clearCalls  int
	initCalls   int
	setCalls    int
}

func (rep *memoryRepository) ClearState(context.Context, string) error {
	rep.clearCalls++
	if rep.clearError != nil {
		return rep.clearError
	}
	rep.stateExists = false
	return nil
}

func (rep *memoryRepository) InitState(context.Context, string) error {
	rep.initCalls++
	if rep.initError != nil {
		return rep.initError
	}
	rep.state = repository.BotState{State: repository.StateRandom}
	rep.stateExists = true
	return nil
}

func (rep *memoryRepository) GetState(context.Context, string) (repository.BotState, error) {
	if rep.getError != nil {
		return repository.BotState{}, rep.getError
	}
	if !rep.stateExists {
		return repository.BotState{}, pgx.ErrNoRows
	}
	return cloneBotState(rep.state), nil
}

func (rep *memoryRepository) SetState(_ context.Context, _ string, state repository.BotState) error {
	rep.setCalls++
	if rep.setError != nil {
		return rep.setError
	}
	rep.state = cloneBotState(state)
	rep.stateExists = true
	return nil
}

func cloneBotState(state repository.BotState) repository.BotState {
	state.Memory = append([]my_types.Pair(nil), state.Memory...)
	if state.Dir != nil {
		direction := *state.Dir
		state.Dir = &direction
	}
	return state
}

func testSmartBot(rep *memoryRepository) *SmartBot {
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	return &SmartBot{rep: rep, logger: logger}
}

func TestStartGameClearsAndInitializesState(test *testing.T) {
	rep := &memoryRepository{}
	bot := testSmartBot(rep)
	if err := bot.StartGame("session"); err != nil {
		test.Fatalf("StartGame() error = %v", err)
	}
	if rep.clearCalls != 1 || rep.initCalls != 1 || !rep.stateExists {
		test.Fatalf("repository calls: clear=%d init=%d stateExists=%v", rep.clearCalls, rep.initCalls, rep.stateExists)
	}
}

func TestStartGameReturnsRepositoryErrors(test *testing.T) {
	clearError := errors.New("clear failed")
	if err := testSmartBot(&memoryRepository{clearError: clearError}).StartGame("session"); !errors.Is(err, clearError) {
		test.Errorf("StartGame() error = %v; want %v", err, clearError)
	}

	initError := errors.New("init failed")
	rep := &memoryRepository{initError: initError}
	if err := testSmartBot(rep).StartGame("session"); !errors.Is(err, initError) {
		test.Errorf("StartGame() error = %v; want %v", err, initError)
	}
}

func TestShootUsesRandomStateForNewSession(test *testing.T) {
	rep := &memoryRepository{}
	shot, err := testSmartBot(rep).Shoot("session")
	if err != nil {
		test.Fatalf("Shoot() error = %v", err)
	}
	if shot.X < 0 || shot.X >= my_types.Size || shot.Y < 0 || shot.Y >= my_types.Size {
		test.Fatalf("shot = %+v; out of bounds", shot)
	}
	if rep.setCalls != 1 || rep.state.State != repository.StateRandom || rep.state.LastShot != shot {
		test.Errorf("saved state = %+v; expected random state with last shot %+v", rep.state, shot)
	}
}

func TestShootConsumesNextSinkTarget(test *testing.T) {
	state := repository.BotState{
		State:    repository.StateSink,
		Memory:   []my_types.Pair{{X: 2, Y: 3}, {X: 2, Y: 4}},
		LastShot: my_types.Pair{X: 2, Y: 2},
	}
	rep := &memoryRepository{state: state, stateExists: true}

	shot, err := testSmartBot(rep).Shoot("session")
	if err != nil {
		test.Fatalf("Shoot() error = %v", err)
	}
	if shot != state.Memory[0] {
		test.Errorf("shot = %+v; want first queued target %+v", shot, state.Memory[0])
	}
	if len(rep.state.Memory) != 1 || rep.state.Memory[0] != state.Memory[1] {
		test.Errorf("saved target queue = %v; want remaining target %v", rep.state.Memory, state.Memory[1:])
	}
}

func TestShootResetsAfterInvalidSinkTargets(test *testing.T) {
	state := repository.BotState{
		State:  repository.StateSink,
		Memory: []my_types.Pair{{X: -1, Y: 3}, {X: 2, Y: my_types.Size}},
	}
	rep := &memoryRepository{state: state, stateExists: true}

	shot, err := testSmartBot(rep).Shoot("session")
	if err != nil {
		test.Fatalf("Shoot() error = %v", err)
	}
	if shot.X < 0 || shot.X >= my_types.Size || shot.Y < 0 || shot.Y >= my_types.Size {
		test.Fatalf("shot = %+v; out of bounds", shot)
	}
	if rep.clearCalls != 1 || rep.state.State != repository.StateRandom || len(rep.state.Memory) != 0 {
		test.Errorf("state after exhausting invalid targets = %+v; want a cleared random state", rep.state)
	}
}

func TestShootReturnsRepositoryErrors(test *testing.T) {
	wantError := errors.New("read failed")
	if _, err := testSmartBot(&memoryRepository{getError: wantError, stateExists: true}).Shoot("session"); !errors.Is(err, wantError) {
		test.Errorf("Shoot() error = %v; want %v", err, wantError)
	}

	setError := errors.New("write failed")
	if _, err := testSmartBot(&memoryRepository{setError: setError}).Shoot("session"); !errors.Is(err, setError) {
		test.Errorf("Shoot() error = %v; want %v", err, setError)
	}
}

func TestSetResultHitStartsSinkSearch(test *testing.T) {
	rep := &memoryRepository{
		state:       repository.BotState{State: repository.StateRandom, LastShot: my_types.Pair{X: 4, Y: 5}},
		stateExists: true,
	}
	testSmartBot(rep).SetResult("session", my_types.Hit)

	if rep.state.State != repository.StateSink || rep.state.LastHit != rep.state.LastShot || len(rep.state.Memory) != 4 {
		test.Fatalf("state after first hit = %+v; want sink mode and four adjacent targets", rep.state)
	}
}

func TestSetResultHitContinuesAlongShip(test *testing.T) {
	rep := &memoryRepository{
		state: repository.BotState{
			State:    repository.StateSink,
			Memory:   []my_types.Pair{{X: 3, Y: 4}, {X: 4, Y: 5}, {X: 5, Y: 4}},
			LastShot: my_types.Pair{X: 4, Y: 5},
			LastHit:  my_types.Pair{X: 4, Y: 4},
		},
		stateExists: true,
	}
	testSmartBot(rep).SetResult("session", my_types.Hit)

	if rep.state.Dir == nil || *rep.state.Dir != (my_types.Pair{X: 0, Y: 1}) {
		test.Fatalf("direction = %v; want (0, 1)", rep.state.Dir)
	}
	for _, target := range rep.state.Memory {
		if target.X != 4 {
			test.Errorf("off-axis target retained after direction found: %+v", target)
		}
	}
}

func TestSetResultAlreadyPreservesRemainingTargets(test *testing.T) {
	state := repository.BotState{
		State:  repository.StateSink,
		Memory: []my_types.Pair{{X: 3, Y: 4}, {X: 4, Y: 3}},
	}
	rep := &memoryRepository{state: state, stateExists: true}
	bot := testSmartBot(rep)
	if _, err := bot.Shoot("session"); err != nil {
		test.Fatalf("Shoot() error = %v", err)
	}
	remainingTargets := append([]my_types.Pair(nil), rep.state.Memory...)
	bot.SetResult("session", my_types.Already)

	if len(rep.state.Memory) != len(remainingTargets) || rep.state.Memory[0] != remainingTargets[0] {
		test.Errorf("remaining targets = %v; want unchanged %v", rep.state.Memory, remainingTargets)
	}
	if rep.setCalls != 1 {
		test.Errorf("SetState calls = %d after Already; want no additional state write", rep.setCalls)
	}
}

func TestSetResultSinkAndGameOverClearState(test *testing.T) {
	rep := &memoryRepository{stateExists: true}
	bot := testSmartBot(rep)
	bot.SetResult("session", my_types.Sink)
	if rep.clearCalls != 1 {
		test.Errorf("ClearState calls after Sink = %d; want 1", rep.clearCalls)
	}

	if err := bot.GameOver("session"); err != nil {
		test.Fatalf("GameOver() error = %v", err)
	}
	if rep.clearCalls != 2 {
		test.Errorf("ClearState calls after GameOver = %d; want 2", rep.clearCalls)
	}
}

func TestGameOverReturnsRepositoryError(test *testing.T) {
	wantError := errors.New("clear failed")
	if err := testSmartBot(&memoryRepository{clearError: wantError}).GameOver("session"); !errors.Is(err, wantError) {
		test.Errorf("GameOver() error = %v; want %v", err, wantError)
	}
}
