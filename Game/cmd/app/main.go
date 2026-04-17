package main

import (
  // "sea_battle/my_types"
  "sea_battle/Game/internal/domain"
  "sea_battle/Game/internal/ui"
)

func main() {
  cancel := make(chan struct{})
  defer close(cancel)

  botField := domain.Constructor()
  userField := domain.Constructor()

  go botField.BuildField(domain.RandomPlacer, cancel)

  ui.Run(userField, botField)
}
