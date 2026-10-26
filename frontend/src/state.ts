import type { GameStateResponse, Pair, ShotRecord } from "./api/types";

export type ScreenName = "auth" | "bots" | "placement" | "battle";

export interface GameState {
  botId: string;
  playerField: number[][];
  sessionId: string | null;
  playerFieldShips: GameStateResponse["playerField"]["ships"];
  enemyShots: ShotRecord[];
  playerShots: ShotRecord[];
  yourTurn: boolean;
  gameOver: boolean;
  winner: "player" | "bot" | null;
  loading: boolean;
}

export interface AppState {
  screen: ScreenName;
  token: string | null;
  authMode: "login" | "register";
  game: GameState | null;
  shipIndex: number;
  direction: Pair;
}

export const SHIP_SIZES = [4, 3, 3, 2, 2, 2, 1, 1, 1, 1] as const;

export const state: AppState = {
  screen: "auth",
  token: null,
  authMode: "login",
  game: null,
  shipIndex: 0,
  direction: { x: 0, y: 1 },
};

export function emptyField(): number[][] {
  return Array.from({ length: 10 }, () => Array<number>(10).fill(0));
}

export function resetGame(): void {
  state.game = null;
  state.shipIndex = 0;
  state.direction = { x: 0, y: 1 };
}

export function setGameBot(botId: string): void {
  state.game = {
    botId,
    playerField: emptyField(),
    sessionId: null,
    playerFieldShips: [],
    enemyShots: [],
    playerShots: [],
    yourTurn: false,
    gameOver: false,
    winner: null,
    loading: false,
  };
  state.shipIndex = 0;
  state.direction = { x: 0, y: 1 };
}

export function applyServerState(response: GameStateResponse): void {
  const game = state.game;
  if (game === null) {
    return;
  }
  game.sessionId = response.sessionId;
  game.playerFieldShips = response.playerField.ships;
  game.enemyShots = response.enemyShotsOnPlayerField;
  game.playerShots = response.playerShotsOnEnemyField;
  game.yourTurn = response.yourTurn;
  game.gameOver = response.gameOver;
  game.winner = response.winner;
}
