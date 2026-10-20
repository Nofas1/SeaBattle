import { ApiError, getGameState, shoot } from "./api/client";
import type { Pair, ShotRecord } from "./api/types";
import { applyServerState, state } from "./state";
import { renderBoard, type CellState } from "./render";

let stateStale = false;

export function renderBattle(): void {
  const game = state.game;
  const playerBoard = document.querySelector<HTMLDivElement>("#battle-player-board");
  const enemyBoard = document.querySelector<HTMLDivElement>("#battle-opponent-board");
  const status = document.querySelector<HTMLParagraphElement>("#battle-status");
  if (game === null || playerBoard === null || enemyBoard === null || status === null) {
    return;
  }

  const sunkPlayerCells = getSunkCells(game.enemyShots);
  const sunkEnemyCells = getSunkCells(game.playerShots);
  const aroundPlayer = getAroundCells(sunkPlayerCells);
  const aroundEnemy = getAroundCells(sunkEnemyCells);

  renderBoard(playerBoard, {
    label: "Your fleet and incoming shots",
    getCellState: (x, y) => ownCellState(game.playerFieldShips, game.enemyShots, aroundPlayer, x, y),
    isInteractive: () => false,
  });
  applySunkPerimeter(playerBoard);
  playerBoard.classList.toggle("board-loading", game.loading);

  renderBoard(enemyBoard, {
    label: "Enemy fleet and your shots",
    getCellState: (x, y) => enemyCellState(game.playerShots, aroundEnemy, x, y),
    isInteractive: (x, y) => game.yourTurn && !game.gameOver && !game.loading && !stateStale &&
      !hasShot(game.playerShots, x, y),
    onCellClick: (x, y) => {
      void requestPlayerShot(x, y);
    },
  });
  applySunkPerimeter(enemyBoard);
  enemyBoard.classList.toggle("board-loading", game.loading);
  enemyBoard.classList.toggle("board-not-turn", !game.yourTurn || game.loading || game.gameOver);

  status.textContent = stateStale
    ? "Connection lost. Retry to sync the game."
    : game.gameOver
    ? "Game over"
    : game.yourTurn
      ? "Your turn"
      : "Bot's turn";
}

export async function refreshGameState(): Promise<void> {
  const game = state.game;
  if (game === null || game.sessionId === null || game.loading || game.gameOver) {
    return;
  }
  const sessionId = game.sessionId;
  game.loading = true;
  renderBattle();
  try {
    let response = await getGameState(sessionId);
    applyServerState(response);
    renderBattle();
    let attempts = 0;
    while (!response.yourTurn && !response.gameOver && attempts < 100) {
      response = await getGameState(sessionId);
      applyServerState(response);
      renderBattle();
      attempts += 1;
    }
    if (!response.yourTurn && !response.gameOver) {
      throw new Error("The bot did not finish its turn. Please retry.");
    }
    stateStale = false;
    setRetryVisible(false);
    if (response.gameOver && response.winner !== null) {
      showEndModal(response.winner);
    }
  } catch (error) {
    handleGameRequestError(error);
  } finally {
    game.loading = false;
    renderBattle();
  }
}

export async function requestPlayerShot(x: number, y: number): Promise<void> {
  const game = state.game;
  if (game === null || game.sessionId === null || !game.yourTurn || game.loading ||
      game.gameOver || hasShot(game.playerShots, x, y)) {
    return;
  }
  const sessionId = game.sessionId;
  game.loading = true;
  renderBattle();
  try {
    let response = await shoot(sessionId, x, y);
    applyServerState(response);
    renderBattle();
    let attempts = 0;
    while (!response.yourTurn && !response.gameOver && attempts < 100) {
      response = await getGameState(sessionId);
      applyServerState(response);
      renderBattle();
      attempts += 1;
    }
    if (!response.yourTurn && !response.gameOver) {
      throw new Error("The bot did not finish its turn. Please retry.");
    }
    stateStale = false;
    setRetryVisible(false);
    if (response.gameOver && response.winner !== null) {
      showEndModal(response.winner);
    }
  } catch (error) {
    handleGameRequestError(error);
  } finally {
    game.loading = false;
    renderBattle();
  }
}

export function showEndModal(winner: "player" | "bot"): void {
  const modal = document.querySelector<HTMLElement>("#modal-end");
  const title = document.querySelector<HTMLHeadingElement>("#end-title");
  const icon = document.querySelector<HTMLParagraphElement>("#end-icon");
  if (modal === null || title === null || icon === null) {
    return;
  }
  title.textContent = winner === "player" ? "Victory!" : "Defeat";
  icon.textContent = winner === "player" ? "🏆" : "☠";
  modal.classList.toggle("defeat", winner === "bot");
  modal.classList.remove("hidden");
}

export function hideEndModal(): void {
  document.querySelector<HTMLElement>("#modal-end")?.classList.add("hidden");
}

function handleGameError(error: unknown): void {
  if (error instanceof ApiError && error.status === 401) {
    window.dispatchEvent(new Event("auth:unauthorized"));
    return;
  }
  window.dispatchEvent(new CustomEvent("app:toast", {
    detail: error instanceof Error ? error.message : String(error),
  }));
}

function handleGameRequestError(error: unknown): void {
  if (!(error instanceof ApiError && error.status === 400)) {
    stateStale = true;
    setRetryVisible(true);
  }
  handleGameError(error);
}

function setRetryVisible(visible: boolean): void {
  document.querySelector<HTMLButtonElement>("#retry-state-button")
    ?.classList.toggle("hidden", !visible);
}

function ownCellState(
  ships: readonly { cells: readonly Pair[] }[],
  shots: readonly ShotRecord[],
  around: ReadonlySet<string>,
  x: number,
  y: number,
): CellState {
  const result = shotAt(shots, x, y);
  if (result === "Hit" || result === "Sink") {
    return "hit";
  }
  if (around.has(pairKey(x, y))) {
    return "around-sunk";
  }
  if (result === "Miss") {
    return "miss";
  }
  return ships.some((ship) => ship.cells.some((cell) => cell.x === x && cell.y === y))
    ? "ship"
    : "water";
}

function enemyCellState(
  shots: readonly ShotRecord[],
  around: ReadonlySet<string>,
  x: number,
  y: number,
): CellState {
  const result = shotAt(shots, x, y);
  if (result === "Sink") {
    return "sunk";
  }
  if (around.has(pairKey(x, y))) {
    return "around-sunk";
  }
  if (result === "Hit") {
    return "hit";
  }
  if (result === "Miss") {
    return "miss";
  }
  return "water";
}

function shotAt(shots: readonly ShotRecord[], x: number, y: number): ShotRecord["result"] | undefined {
  return shots.find((shot) => shot.x === x && shot.y === y)?.result;
}

function hasShot(shots: readonly ShotRecord[], x: number, y: number): boolean {
  return shotAt(shots, x, y) !== undefined;
}

function getSunkCells(shots: readonly ShotRecord[]): Pair[] {
  return shots.filter((shot) => shot.result === "Sink").map(({ x, y }) => ({ x, y }));
}

function getAroundCells(sunkCells: readonly Pair[]): Set<string> {
  const shipCells = new Set(sunkCells.map(({ x, y }) => pairKey(x, y)));
  const around = new Set<string>();
  for (const cell of sunkCells) {
    for (let x = cell.x - 1; x <= cell.x + 1; x += 1) {
      for (let y = cell.y - 1; y <= cell.y + 1; y += 1) {
        const key = pairKey(x, y);
        if (x >= 0 && x < 10 && y >= 0 && y < 10 && !shipCells.has(key)) {
          around.add(key);
        }
      }
    }
  }
  return around;
}

function pairKey(x: number, y: number): string {
  return `${x},${y}`;
}

function applySunkPerimeter(board: HTMLElement): void {
  const cells = board.querySelectorAll<HTMLElement>('.cell[data-state="sunk"]');
  for (const cell of cells) {
    const x = Number(cell.dataset.x);
    const y = Number(cell.dataset.y);
    if (board.querySelector(`.cell[data-x="${x}"][data-y="${y - 1}"][data-state="sunk"]`) === null) {
      cell.classList.add("sunk-edge-top");
    }
    if (board.querySelector(`.cell[data-x="${x + 1}"][data-y="${y}"][data-state="sunk"]`) === null) {
      cell.classList.add("sunk-edge-right");
    }
    if (board.querySelector(`.cell[data-x="${x}"][data-y="${y + 1}"][data-state="sunk"]`) === null) {
      cell.classList.add("sunk-edge-bottom");
    }
    if (board.querySelector(`.cell[data-x="${x - 1}"][data-y="${y}"][data-state="sunk"]`) === null) {
      cell.classList.add("sunk-edge-left");
    }
  }
}
