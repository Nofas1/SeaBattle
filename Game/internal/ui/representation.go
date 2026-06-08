package ui

import (
	// "bytes"
	// "encoding/json"
	"log/slog"
	// "net/http"
	"os"
	"sea_battle/Game/internal/domain"
	"sea_battle/Game/internal/game"
	"sea_battle/my_types"
	// "strings"

	rl "github.com/gen2brain/raylib-go/raylib"
)

const (
	CELL = 50
	ROWS = HEIGHT / CELL
	COLS = WIDTH / CELL

	PADDING = 40
	GRID    = CELL * 10
	WIDTH   = GRID + PADDING*2
	HEIGHT  = GRID*2 + PADDING*3
)

const (
	userOffsetX = int32(PADDING)
	botOffsetX  = int32(PADDING*2 + GRID)
	offsetY     = int32(PADDING)
)

type AuthRequest struct {
	Name     string `json:"name"`
	Password string `json:"password"`
}

type AuthResponse struct {
	Token string `json:"token"`
}

type AuthBox int

const (
	BoxNone AuthBox = iota
	BoxUsername
	BoxPassword
)

// func DrawAuthWindow(music rl.Music, logger *slog.Logger) string {
// 	username := ""
// 	password := ""
// 	activeBox := BoxNone
// 	loginError := ""

// 	for !rl.WindowShouldClose() {
// 		rl.UpdateMusicStream(music)

// 		key := rl.GetCharPressed()
// 		for key > 0 {
// 			if activeBox == BoxUsername {
// 				username += string(rune(key))
// 			}
// 			if activeBox == BoxPassword {
// 				password += string(rune(key))
// 			}
// 			key = rl.GetCharPressed()
// 		}

// 		if rl.IsKeyPressed(rl.KeyBackspace) {
// 			if activeBox == BoxUsername && len(username) > 0 {
// 				username = username[:len(username)-1]
// 			}
// 			if activeBox == BoxPassword && len(password) > 0 {
// 				password = password[:len(password)-1]
// 			}
// 		}

// 		mouse := rl.GetMousePosition()

// 		userRect := rl.Rectangle{X: 250, Y: 180, Width: 300, Height: 40}
// 		passRect := rl.Rectangle{X: 250, Y: 260, Width: 300, Height: 40}
// 		loginBtn := rl.Rectangle{X: 300, Y: 340, Width: 200, Height: 50}
// 		registerBtn := rl.Rectangle{X: 300, Y: 410, Width: 200, Height: 50}

// 		if rl.IsMouseButtonPressed(rl.MouseButtonLeft) {
// 			switch {
// 			case rl.CheckCollisionPointRec(mouse, userRect):
// 				activeBox = BoxUsername
// 			case rl.CheckCollisionPointRec(mouse, passRect):
// 				activeBox = BoxPassword
// 			case rl.CheckCollisionPointRec(mouse, loginBtn):
// 				token, err := authRequest("http://localhost:28080/login", username, password)
// 				if err != nil {
// 					logger.Error("login failed", "error", err)
// 					loginError = err.Error()
// 				} else {
// 					return token
// 				}
// 			case rl.CheckCollisionPointRec(mouse, registerBtn):
// 				_, err := authRequest("http://localhost:28080/register", username, password)
// 				if err != nil {
// 					logger.Error("register failed", "error", err)
// 					loginError = err.Error()
// 				} else {
// 					loginError = "registered, please login"
// 				}
// 			}
// 		}

// 		rl.BeginDrawing()
// 		rl.ClearBackground(rl.RayWhite)

// 		rl.DrawText("LOGIN", 320, 100, 40, rl.DarkBlue)

// 		rl.DrawRectangleRec(userRect, rl.LightGray)
// 		rl.DrawRectangleLinesEx(userRect, 2, rl.DarkBlue)
// 		rl.DrawText(username, int32(userRect.X)+10, int32(userRect.Y)+10, 20, rl.Black)

// 		rl.DrawRectangleRec(passRect, rl.LightGray)
// 		rl.DrawRectangleLinesEx(passRect, 2, rl.DarkBlue)
// 		stars := strings.Repeat("*", len(password))
// 		rl.DrawText(stars, int32(passRect.X)+10, int32(passRect.Y)+10, 20, rl.Black)

// 		rl.DrawRectangleRec(loginBtn, rl.SkyBlue)
// 		rl.DrawText("LOGIN", int32(loginBtn.X)+55, int32(loginBtn.Y)+15, 20, rl.White)

// 		rl.DrawRectangleRec(registerBtn, rl.Green)
// 		rl.DrawText("REGISTER", int32(registerBtn.X)+35, int32(registerBtn.Y)+15, 20, rl.White)

// 		if loginError != "" {
// 			rl.DrawText(loginError, 250, 480, 20, rl.Red)
// 		}

// 		rl.EndDrawing()
// 	}

// 	return ""
// }

// func authRequest(url, username, password string) (string, error) {
// 	body, err := json.Marshal(map[string]string{
// 		"name":     username,
// 		"password": password,
// 	})
// 	if err != nil {
// 		return "", err
// 	}
// 	resp, err := http.Post(url, "application/json", bytes.NewReader(body))
// 	if err != nil {
// 		return "", err
// 	}
// 	defer resp.Body.Close()

// 	var authResp AuthResponse
// 	if err := json.NewDecoder(resp.Body).Decode(&authResp); err != nil {
// 		return "", err
// 	}
// 	return authResp.Token, nil
// }

func SelectBot(userField *domain.Field, music rl.Music, logger *slog.Logger, token string) game.Bot {
	buttons := []struct {
		label string
		rect  rl.Rectangle
	}{
		{"Simple Bot", rl.Rectangle{X: float32(WIDTH/2 - 100), Y: 200, Width: 300, Height: 60}},
		{"Smart Bot", rl.Rectangle{X: float32(WIDTH/2 - 100), Y: 300, Width: 300, Height: 60}},
		{"AI Bot", rl.Rectangle{X: float32(WIDTH/2 - 100), Y: 400, Width: 300, Height: 60}},
	}

	for !rl.WindowShouldClose() {
		rl.UpdateMusicStream(music)
		rl.BeginDrawing()
		rl.ClearBackground(rl.RayWhite)

		rl.DrawText("SEA BATTLE", int32(WIDTH/2-100), 80, 40, rl.DarkBlue)
		rl.DrawText("Choose your opponent:", int32(WIDTH/2-100), 160, 20, rl.Gray)

		mp := rl.GetMousePosition()
		clicked := rl.IsMouseButtonPressed(rl.MouseButtonLeft)

		for _, btn := range buttons {
			hovered := rl.CheckCollisionPointRec(mp, btn.rect)

			color := rl.LightGray
			if hovered {
				color = rl.SkyBlue
			}

			rl.DrawRectangleRec(btn.rect, color)
			rl.DrawRectangleLinesEx(btn.rect, 2, rl.DarkBlue)

			textW := rl.MeasureText(btn.label, 24)
			rl.DrawText(
				btn.label,
				int32(btn.rect.X)+int32(btn.rect.Width)/2-textW/2,
				int32(btn.rect.Y)+int32(btn.rect.Height)/2-12,
				24, rl.DarkBlue,
			)

			if hovered && clicked {
				rl.EndDrawing()
				switch btn.label {
				case "Simple Bot":
					logger.Info(
						"bot selected",
						"bot", "simple_bot",
					)
					return game.NewBotProxy(userField, "http://localhost:28080", "simple_bot", logger, token)
				case "Smart Bot":
					logger.Info(
						"bot selected",
						"bot", "smart_bot",
					)
					return game.NewBotProxy(userField, "http://localhost:28080", "smart_bot", logger, token)
				case "AI Bot":
					logger.Info(
						"bot selected",
						"bot", "ai_bot",
					)
					return game.NewBotProxy(userField, "http://localhost:28080", "ai_bot", logger, token)
				}
			}
		}

		rl.EndDrawing()
	}
	return game.NewBotProxy(userField, "http://localhost:28080", "simple_bot", logger, token)
}

func DrawGrid(offsetX, offsetY int32, matrix [][]int, hideShips bool) {
	for i := int32(0); i < 10; i++ {
		for j := int32(0); j < 10; j++ {
			x := offsetX + j*CELL
			y := offsetY + i*CELL

			rect := rl.Rectangle{
				X:      float32(x),
				Y:      float32(y),
				Width:  CELL,
				Height: CELL,
			}

			color := rl.RayWhite
			switch matrix[i][j] {
			case my_types.SHIP:
				if hideShips {
					color = rl.RayWhite
				} else {
					color = rl.Gray
				}
			case my_types.SHOOTED:
				color = rl.Fade(rl.Red, 0.8)
			case my_types.MISSED:
				color = rl.Fade(rl.Blue, 0.5)
			case my_types.FILL:
				color = rl.Fade(rl.Blue, 0.5)
			}

			rl.DrawRectangleRec(rect, color)
			rl.DrawRectangleLinesEx(rect, 1, rl.Black)
		}
	}

	if !hideShips {
		mp := rl.GetMousePosition()
		col := (int32(mp.X) - offsetX) / CELL
		row := (int32(mp.Y) - offsetY) / CELL
		if col >= 0 && col < my_types.Size && row >= 0 && row < my_types.Size {
			rl.DrawRectangle(offsetX+col*CELL, offsetY+row*CELL, CELL, CELL, rl.Fade(rl.Red, 0.5))
		}
	}
}

func Placer(userField *domain.Field, cancel <-chan struct{}, music rl.Music) {
	ship_index := 0
	dir := my_types.Up
	input := make(chan domain.PlaceRequest)
	placed := make(chan bool)

	go userField.BuildField(domain.UserPlacer(input), cancel)

	for !rl.WindowShouldClose() && ship_index < len(my_types.ShipSizes) {
		rl.UpdateMusicStream(music)
		rl.BeginDrawing()
		rl.ClearBackground(rl.RayWhite)

		DrawGrid(userOffsetX, offsetY, userField.Matrix, false)

		select {
		case ok := <-placed:
			if ok {
				ship_index++
			}
		default:
		}

		if rl.IsMouseButtonPressed(rl.MouseButtonRight) {
			dir = (dir + 1) % 2
		}

		mp := rl.GetMousePosition()
		col := (int32(mp.X) - userOffsetX) / CELL
		row := (int32(mp.Y) - offsetY) / CELL

		if ship_index < len(my_types.ShipSizes) {
			pc, pr := col, row
			for i := 0; i < my_types.ShipSizes[ship_index]; i++ {
				rl.DrawRectangle(userOffsetX+pc*CELL, offsetY+pr*CELL, CELL, CELL, rl.Fade(rl.Blue, 0.4))
				pr += int32(my_types.Directions[dir][0])
				pc += int32(my_types.Directions[dir][1])
			}
		}

		if rl.IsMouseButtonPressed(rl.MouseButtonLeft) {
			feedback := make(chan bool)
			req := domain.PlaceRequest{
				ShipSize: my_types.ShipSizes[ship_index],
				Dir:      dir,
				Point:    domain.Pair{X: int(row), Y: int(col)},
				Feedback: feedback,
			}
			go func() {
				input <- req
				placed <- <-feedback
			}()
		}

		rl.EndDrawing()
	}
}

func Battle(userField, botField *domain.Field, bot game.Bot, music rl.Music, logger *slog.Logger) {
	user_sunk := 0
	bot_sunk := 0
	turn := true
	hit_sound := rl.LoadSound("sounds/hit.wav")
	defer rl.UnloadSound(hit_sound)
	rl.SetSoundVolume(hit_sound, 0.1)
	for !rl.WindowShouldClose() {
		rl.UpdateMusicStream(music)

		rl.BeginDrawing()
		rl.ClearBackground(rl.RayWhite)

		DrawGrid(userOffsetX, offsetY, userField.Matrix, false)
		DrawGrid(botOffsetX, offsetY, botField.Matrix, true)

		if user_sunk == 10 {
			break
		}
		if bot_sunk == 10 {
			break
		}

		if rl.IsMouseButtonPressed(rl.MouseButtonLeft) && turn == true {
			mp := rl.GetMousePosition()
			col := (int32(mp.X) - botOffsetX) / CELL
			row := (int32(mp.Y) - offsetY) / CELL
			if col >= 0 && col < my_types.Size && row >= 0 && row < my_types.Size {
				shotRes := game.UserShot(botField, int(row), int(col))
				if shotRes != my_types.Already {
					rl.PlaySound(hit_sound)
					if shotRes == my_types.Sink {
						turn = true
						bot_sunk++
					} else if shotRes == my_types.Hit {
						turn = true
					} else {
						turn = false
					}
				}
			}
		} else if turn == false {
			// timeout
			shotRes, err := game.BotShot(bot, userField, logger)
			if err != nil {
				logger.Error("bot error", "error", err)
				rl.DrawText("Bot unavailable!", 400, 300, 30, rl.Red)
				continue
			}

			if shotRes != my_types.Already {
				rl.PlaySound(hit_sound)
				if shotRes == my_types.Sink {
					turn = false
					user_sunk++
				} else if shotRes == my_types.Hit {
					turn = false
				} else {
					turn = true
				}
			}
		}

		rl.EndDrawing()
	}
}

func Run(userField, botField *domain.Field) {
	rl.InitWindow(HEIGHT, WIDTH, "Sea Battle")
	defer rl.CloseWindow()

	cancel := make(chan struct{})
	defer close(cancel)
	rl.InitAudioDevice()
	defer rl.CloseAudioDevice()
	music := rl.LoadMusicStream("sounds/theme.mp3")
	defer rl.UnloadMusicStream(music)
	rl.SetMusicVolume(music, 0.1)
	rl.PlayMusicStream(music)

	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))

	// token := DrawAuthWindow(music, logger)
	// if token == "" {
	// 	return
	// }
	token := "s10f9"
	bot := SelectBot(userField, music, logger, token)
	_ = bot.StartGame()
	Placer(userField, cancel, music)
	Battle(userField, botField, bot, music, logger)
}
