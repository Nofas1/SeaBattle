package internal

import (
	"fmt"
	"context"
	"os"
	"strconv"
	"strings"
	"sea_battle/my_types"

	"github.com/openai/openai-go/v3"
	"github.com/openai/openai-go/v3/option"
	"github.com/openai/openai-go/v3/responses"
)

type AIBot struct {
	field [][]int
	OpenAIClient openai.Client
	api_key    string
}

func NewAIBot() *AIBot {
	key, _ := os.LookupEnv("AI_KEY")
	field := make([][]int, my_types.Size)
	for i := 0; i < my_types.Size; i++ {
		field[i] = make([]int, my_types.Size)
	}
	return &AIBot{
		field: field,
		OpenAIClient: openai.NewClient(option.WithAPIKey(key)),
		api_key: key,
	}
}

// func (ab *AIBot) translate_field(field *my_types.Field) string {
// 	var res strings.Builder

// 	for i := 0; i < my_types.Size; i++ {
// 		for j := 0; j < my_types.Size; j++ {
// 			res.WriteString(strconv.Itoa(field.Matrix[i][j]) + " ")
// 		}
// 		res.WriteString("\n")
// 	}

// 	return res.String()
// }

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

func (ab *AIBot) Shoot(field *my_types.Field) my_types.Pair {
	curr_field := ab.translateMatrix(field.Matrix)
	var f, s int

	for {
		req := fmt.Sprintf("You recieve a current field positon: \n%s\n, choose where you want to shoot, Reply with two integers only (e.g. \"1 2\")", curr_field)

		resp, err := ab.OpenAIClient.Responses.New(context.Background(), responses.ResponseNewParams{
			Input: responses.ResponseNewParamsInputUnion{OfString: openai.String(req)},
			Model: openai.ChatModelChatgpt4oLatest,
		}, )

		if err != nil {
			return my_types.Pair{X: 11, Y: 11}
		}

		p := strings.Split(strings.TrimSpace(resp.OutputText()), " ")
		f, _ = strconv.Atoi(p[0])
		s, _ = strconv.Atoi(p[1])
		if f >= 0 && f < my_types.Size && s >= 0 && s < my_types.Size {
			break
		}
	}
	
	return my_types.Pair{X: f, Y: s}
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
            return x, y, my_types.Pair{X: dx, Y: dy}
        }
    }
}

func (ab *AIBot) SetResult(shotRes my_types.ShotResult) {}

