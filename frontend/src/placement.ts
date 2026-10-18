import { startGame } from "./api/client";
import { applyServerState, SHIP_SIZES, state } from "./state";
import { renderBoard, showPlacementPreview } from "./render";

const SHIP_NAMES: Record<number, string> = {
  4: "Battleship",
  3: "Cruiser",
  2: "Destroyer",
  1: "Patrol boat",
};

export function initPlacement(
  onStarted: () => void,
  onError: (error: unknown) => void,
): void {
  document.querySelector<HTMLButtonElement>("#placement-back")?.addEventListener("click", () => {
    window.dispatchEvent(new CustomEvent("screenchange", { detail: "bots" }));
  });
  document.querySelector<HTMLButtonElement>("#rotate-button")?.addEventListener("click", rotate);
  document.addEventListener("keydown", (event) => {
    if (event.key.toLowerCase() === "r" && !event.repeat && state.screen === "placement") {
      rotate();
    }
  });
  document.querySelector<HTMLButtonElement>("#start-battle-button")?.addEventListener("click", async () => {
    const game = state.game;
    if (game === null || state.shipIndex !== SHIP_SIZES.length) {
      return;
    }
    const button = document.querySelector<HTMLButtonElement>("#start-battle-button");
    if (button !== null) {
      button.disabled = true;
      button.textContent = "Starting...";
    }
    try {
      applyServerState(await startGame(game.botId, game.playerField));
      onStarted();
    } catch (error) {
      onError(error);
    } finally {
      if (button !== null) {
        button.textContent = "Start battle";
      }
      renderPlacement();
    }
  });
}

export function renderPlacement(): void {
  const game = state.game;
  const board = document.querySelector<HTMLDivElement>("#placement-board");
  const opponent = document.querySelector<HTMLDivElement>("#placement-opponent-board");
  const shipList = document.querySelector<HTMLOListElement>("#ship-list");
  const startButton = document.querySelector<HTMLButtonElement>("#start-battle-button");
  const message = document.querySelector<HTMLParagraphElement>("#placement-message");
  if (game === null || board === null || opponent === null ||
      shipList === null || startButton === null || message === null) {
    return;
  }

  renderBoard(board, {
    label: "Your fleet placement board",
    getCellState: (x, y) => game.playerField[x]?.[y] === 1 ? "ship" : "water",
    isInteractive: () => state.shipIndex < SHIP_SIZES.length,
    onCellClick: tryPlace,
    onCellHover: (x, y) => {
      const size = SHIP_SIZES[state.shipIndex];
      if (size !== undefined) {
        showPlacementPreview(board, game.playerField, x, y, size, state.direction);
      }
    },
  });
  renderBoard(opponent, {
    label: "Empty opponent board",
    getCellState: () => "disabled",
  });
  shipList.replaceChildren();
  for (let index = 0; index < SHIP_SIZES.length; index += 1) {
    const size = SHIP_SIZES[index];
    const item = document.createElement("li");
    item.textContent = `${SHIP_NAMES[size]} — ${size} cell${size === 1 ? "" : "s"}`;
    if (index < state.shipIndex) {
      item.classList.add("ship-placed");
      item.textContent += " — placed";
    } else if (index === state.shipIndex) {
      item.classList.add("current-ship");
    }
    shipList.append(item);
  }

  const remaining = SHIP_SIZES[state.shipIndex];
  message.textContent = remaining === undefined
    ? "Fleet ready. You can start the battle."
    : `Place: ${SHIP_NAMES[remaining]} (${remaining} cell${remaining === 1 ? "" : "s"}). Orientation: ${state.direction.x === 0 ? "vertical" : "horizontal"}.`;
  startButton.disabled = state.shipIndex !== SHIP_SIZES.length;
}

function tryPlace(x: number, y: number): void {
  const game = state.game;
  const size = SHIP_SIZES[state.shipIndex];
  if (game === null || size === undefined) {
    return;
  }
  if (!canPlaceShip(game.playerField, x, y, size)) {
    const cell = document.querySelector<HTMLElement>(`#placement-board [data-x="${x}"][data-y="${y}"]`);
    cell?.classList.add("placement-preview-invalid");
    return;
  }
  for (let offset = 0; offset < size; offset += 1) {
    const cellX = x + state.direction.x * offset;
    const cellY = y + state.direction.y * offset;
    game.playerField[cellX][cellY] = 1;
  }
  state.shipIndex += 1;
  renderPlacement();
}

function canPlaceShip(field: number[][], x: number, y: number, size: number): boolean {
  for (let offset = 0; offset < size; offset += 1) {
    const cellX = x + state.direction.x * offset;
    const cellY = y + state.direction.y * offset;
    if (cellX < 0 || cellX >= 10 || cellY < 0 || cellY >= 10) {
      return false;
    }
    for (let neighborX = cellX - 1; neighborX <= cellX + 1; neighborX += 1) {
      for (let neighborY = cellY - 1; neighborY <= cellY + 1; neighborY += 1) {
        if (neighborX >= 0 && neighborX < 10 && neighborY >= 0 && neighborY < 10 &&
            field[neighborX][neighborY] === 1) {
          return false;
        }
      }
    }
  }
  return true;
}

function rotate(): void {
  state.direction = state.direction.x === 0 ? { x: 1, y: 0 } : { x: 0, y: 1 };
  renderPlacement();
}
