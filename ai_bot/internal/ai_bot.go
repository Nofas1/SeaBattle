package internal

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"sea_battle/my_types"
	"strconv"
	"strings"

	"github.com/openai/openai-go/v3"
	"github.com/openai/openai-go/v3/option"
	"github.com/openai/openai-go/v3/responses"
)

type AIBot struct {
	field        [][]int
	OpenAIClient openai.Client
	api_key      string
	logger       *slog.Logger
}

func NewAIBot(logger *slog.Logger) *AIBot {
	key, _ := os.LookupEnv("AI_KEY")
	field := make([][]int, my_types.Size)
	for i := 0; i < my_types.Size; i++ {
		field[i] = make([]int, my_types.Size)
	}
	return &AIBot{
		field:        field,
		OpenAIClient: openai.NewClient(option.WithAPIKey(key)),
		api_key:      key,
		logger:       logger,
	}
}

func (ab *AIBot) translateMatrix(matrix [][]int) string {
	var res strings.Builder
	for i := 0; i < my_types.Size; i++ {
		for j := 0; j < my_types.Size; j++ {
			res.WriteString(strconv.Itoa(matrix[i][j]) + " ")
		}
		res.WriteString("\n")
	}
	return res.String()
}

func parseShotResponse(response string) (my_types.Pair, error) {
	coordinates := strings.Fields(response)
	if len(coordinates) != 2 {
		return my_types.Pair{}, fmt.Errorf("expected two coordinates, got %d", len(coordinates))
	}
	x, err := strconv.Atoi(coordinates[0])
	if err != nil {
		return my_types.Pair{}, fmt.Errorf("invalid x coordinate: %w", err)
	}
	y, err := strconv.Atoi(coordinates[1])
	if err != nil {
		return my_types.Pair{}, fmt.Errorf("invalid y coordinate: %w", err)
	}
	if x < 0 || x >= my_types.Size || y < 0 || y >= my_types.Size {
		return my_types.Pair{}, fmt.Errorf("coordinates out of bounds: (%d, %d)", x, y)
	}
	return my_types.Pair{X: x, Y: y}, nil
}

func fallbackShot(field *my_types.Field) my_types.Pair {
	if field == nil || len(field.Matrix) != my_types.Size {
		return my_types.Pair{X: -1, Y: -1}
	}
	for rowIndex, row := range field.Matrix {
		if len(row) != my_types.Size {
			return my_types.Pair{X: -1, Y: -1}
		}
		for columnIndex, cell := range row {
			if cell == my_types.EMPTY || cell == my_types.SHIP {
				return my_types.Pair{X: rowIndex, Y: columnIndex}
			}
		}
	}
	return my_types.Pair{X: -1, Y: -1}
}

func (ab *AIBot) Shoot(field *my_types.Field) my_types.Pair {
	fallback := fallbackShot(field)
	if fallback.X < 0 {
		return fallback
	}
	curr_field := ab.translateMatrix(field.Matrix)
	for attempt := 0; attempt < 3; attempt++ {
		req := fmt.Sprintf("You recieve a current field positon: \n%s\n, choose where you want to shoot, Reply with two integers only (e.g. \"1 2\")", curr_field)

		resp, err := ab.OpenAIClient.Responses.New(context.Background(), responses.ResponseNewParams{
			Input: responses.ResponseNewParamsInputUnion{OfString: openai.String(req)},
			Model: openai.ChatModelChatgpt4oLatest,
		})

		if err != nil {
			return fallback
		}

		shot, err := parseShotResponse(resp.OutputText())
		if err == nil {
			return shot
		}
	}
	return fallback
}

func (ab *AIBot) Place() (int, int, my_types.Pair) {
	curr_field := ab.translateMatrix(ab.field)

	for {
		req := fmt.Sprintf(
			"You are playing a game of Russian sea battle. You are placing one quardo-decked, two tripple-decked, three double-decked, four single-decked ships on a 10x10 field one by one:\n%s\n0=empty, 1=ship. Reply with four integers only (x y dx dy) for one ship, where dx dy is direction: up=(0,-1) right=(1,0) down=(0,1) left=(-1,0), e.g. \"3 5 1 0\"",
			curr_field,
		)
		resp, err := ab.OpenAIClient.Responses.New(context.Background(), responses.ResponseNewParams{
			Input: responses.ResponseNewParamsInputUnion{OfString: openai.String(req)},
			Model: openai.ChatModelChatgpt4oLatest,
		})
		if err != nil {
			x := my_types.GlobalRand.Intn(my_types.Size)
			y := my_types.GlobalRand.Intn(my_types.Size)
			ind := my_types.GlobalRand.Intn(4)
			dir := my_types.Directions[ind]
			target := my_types.Pair{
				X: x,
				Y: y,
			}
			ab.logger.Info(
				"random shot",
				"target", target,
			)
			return x, y, my_types.Pair{X: dir[0], Y: dir[1]}
		}

		p := strings.Split(strings.TrimSpace(resp.OutputText()), " ")
		if len(p) < 4 {
			continue
		}
		x, err1 := strconv.Atoi(p[0])
		y, err2 := strconv.Atoi(p[1])
		dx, err3 := strconv.Atoi(p[2])
		dy, err4 := strconv.Atoi(p[3])
		if err1 != nil || err2 != nil || err3 != nil || err4 != nil {
			continue
		}
		if x >= 0 && x < my_types.Size && y >= 0 && y < my_types.Size {
			target := my_types.Pair{
				X: x,
				Y: y,
			}
			ab.logger.Info(
				"ordinary ai shot",
				"target", target,
			)
			return x, y, my_types.Pair{X: dx, Y: dy}
		}
	}
}

func (ab *AIBot) SetResult(shotRes my_types.ShotResult) {}
