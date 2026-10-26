/** Board coordinate sent by the proxy. @see ../../../proxy/cmd/game.go:24-27 */
export interface Pair {
  /** @see ../../../proxy/cmd/game.go:25 */
  x: number;
  /** @see ../../../proxy/cmd/game.go:26 */
  y: number;
}

/** Shot result used by the server-owned game state. @see ../../../proxy/cmd/game.go:43-47 */
export type ShotResult = "Miss" | "Hit" | "Sink" | "Already";

/** A ship is disclosed only to its owner or after it has sunk. @see ../../../proxy/cmd/game.go:29-32 */
export interface Ship {
  /** @see ../../../proxy/cmd/game.go:30 */
  cells: Pair[];
  /** @see ../../../proxy/cmd/game.go:31 */
  sunk: boolean;
}

/** User credentials sent to the existing auth API. @see ../../../Game/internal/api/model.go:3-6 */
export interface AuthRequest {
  /** @see ../../../Game/internal/api/model.go:4 */
  name: string;
  /** @see ../../../Game/internal/api/model.go:5 */
  password: string;
}

/** Token returned by login. @see ../../../Game/internal/api/model.go:8-10 */
export interface AuthResponse {
  /** @see ../../../Game/internal/api/model.go:9 */
  token: string;
}

/** Verify endpoint response. @see ../../../proxy/cmd/main.go:104-114 */
export interface VerifyResponse {
  /** @see ../../../proxy/cmd/main.go:113 */
  user: string;
}

/** Board is indexed as [x][y], matching the Go Matrix. @see ../../../proxy/cmd/game.go:78-81 */
export type PlayerField = number[][];

/** Start a server-owned game with the player's validated fleet. @see ../../../proxy/cmd/game.go:78-81 */
export interface StartGameRequest {
  /** @see ../../../proxy/cmd/game.go:123 */
  action: "start_game";
  /** @see ../../../proxy/cmd/game.go:124 */
  botId: string;
  /** @see ../../../proxy/cmd/game.go:125 */
  playerField: PlayerField;
}

/** Authenticated request for a player's shot. @see ../../../proxy/cmd/game.go:122-129 */
export interface BotShootRequest {
  /** @see ../../../proxy/cmd/game.go:122 */
  action: "shoot";
  /** @see ../../../proxy/cmd/game.go:124 */
  sessionId: string;
  /** @see ../../../proxy/cmd/game.go:126-127 */
  x: number;
  /** @see ../../../proxy/cmd/game.go:127 */
  y: number;
}

/** Fetch current state and advance one bot shot while it owns the turn. @see ../../../proxy/cmd/game.go:122 */
export interface GameStateRequest {
  /** @see ../../../proxy/cmd/game.go:122 */
  action: "state";
  /** @see ../../../proxy/cmd/game.go:124 */
  sessionId: string;
}

/** Get bot IDs from server configuration. @see ../../../proxy/cmd/game.go:122 */
export interface BotListRequest {
  /** @see ../../../proxy/cmd/game.go:122 */
  action: "list_bots";
}

/** A shot returned to the client; sunk cells and adjacent cells appear only on Sink. @see ../../../proxy/cmd/game.go:35-41 */
export interface Shot {
  /** @see ../../../proxy/cmd/game.go:36 */
  x: number;
  /** @see ../../../proxy/cmd/game.go:37 */
  y: number;
  /** @see ../../../proxy/cmd/game.go:38 */
  result: Exclude<ShotResult, "Already">;
  /** @see ../../../proxy/cmd/game.go:39 */
  sunkShip?: Pair[];
  /** @see ../../../proxy/cmd/game.go:40 */
  aroundSunk?: Pair[];
}

/** Shot history returned by the authoritative server. @see ../../../proxy/cmd/game.go:43-47 */
export interface ShotRecord extends Pair {
  /** @see ../../../proxy/cmd/game.go:46 */
  result: Exclude<ShotResult, "Already">;
}

/** Shared authoritative state returned by start_game, shoot, and state. @see ../../../proxy/cmd/game.go:57-68 */
export interface GameStateResponse {
  /** @see ../../../proxy/cmd/game.go:58 */
  sessionId: string;
  /** @see ../../../proxy/cmd/game.go:59 */
  playerShot?: Shot;
  /** @see ../../../proxy/cmd/game.go:60 */
  botShot: Shot | null;
  /** @see ../../../proxy/cmd/game.go:61 */
  yourTurn: boolean;
  /** @see ../../../proxy/cmd/game.go:62 */
  gameOver: boolean;
  /** @see ../../../proxy/cmd/game.go:63 */
  winner: "player" | "bot" | null;
  /** @see ../../../proxy/cmd/game.go:64 */
  playerField: { ships: Ship[] };
  /** @see ../../../proxy/cmd/game.go:65 */
  enemyShotsOnPlayerField: ShotRecord[];
  /** @see ../../../proxy/cmd/game.go:66 */
  playerShotsOnEnemyField: ShotRecord[];
}

/** start_game returns the initial authoritative state. */
export type StartGameResponse = GameStateResponse;

/** shoot and state return updated authoritative state. */
export type BotShootResponse = GameStateResponse;

/** Configured bot identifiers. @see ../../../proxy/cmd/game.go:70-72 */
export interface BotListResponse {
  /** @see ../../../proxy/cmd/game.go:71 */
  bots: string[];
}

export type BotRequest =
  | StartGameRequest
  | BotShootRequest
  | GameStateRequest
  | BotListRequest;

export type BotResponse = GameStateResponse | BotListResponse;
