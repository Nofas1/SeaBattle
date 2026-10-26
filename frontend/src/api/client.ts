import type {
  AuthRequest,
  AuthResponse,
  BotListResponse,
  BotShootRequest,
  BotShootResponse,
  GameStateRequest,
  PlayerField,
  StartGameRequest,
  StartGameResponse,
  VerifyResponse,
} from "./types";

export let baseURL = "";
let jwtToken: string | null = null;

export class ApiError extends Error {
  constructor(message: string, public readonly status: number) {
    super(message);
    this.name = "ApiError";
  }
}

export function setBaseURL(url: string): void {
  baseURL = url.replace(/\/+$/, "");
}

export function setToken(token: string): void {
  jwtToken = token;
}

export function clearToken(): void {
  jwtToken = null;
}

async function request<T>(
  path: string,
  method: "GET" | "POST",
  body?: unknown,
  authenticated = false,
): Promise<T> {
  const headers = new Headers();
  if (body !== undefined) {
    headers.set("Content-Type", "application/json");
  }
  if (authenticated && jwtToken !== null) {
    headers.set("Authorization", `Bearer ${jwtToken}`);
  }

  const controller = new AbortController();
  const timeout = window.setTimeout(() => controller.abort(), 35000);
  let response: Response;
  try {
    response = await fetch(`${baseURL}${path}`, {
      method,
      headers,
      body: body === undefined ? undefined : JSON.stringify(body),
      signal: controller.signal,
    });
  } catch (error) {
    if (error instanceof DOMException && error.name === "AbortError") {
      throw new ApiError("Cannot reach server: request timed out.", 0);
    }
    throw new ApiError("Cannot reach server.", 0);
  } finally {
    window.clearTimeout(timeout);
  }

  const raw = await response.text();
  if (!response.ok) {
    throw new ApiError(raw.trim() || `Request failed with HTTP ${response.status}.`, response.status);
  }
  if (raw.length === 0) {
    return undefined as T;
  }
  try {
    return JSON.parse(raw) as T;
  } catch {
    throw new ApiError("Server returned invalid JSON.", response.status);
  }
}

export function login(name: string, password: string): Promise<AuthResponse> {
  const body: AuthRequest = { name, password };
  return request<AuthResponse>("/login", "POST", body);
}

export function register(name: string, password: string): Promise<void> {
  const body: AuthRequest = { name, password };
  return request<void>("/register", "POST", body);
}

export function verify(): Promise<VerifyResponse> {
  return request<VerifyResponse>("/verify", "GET", undefined, true);
}

export function listBots(): Promise<BotListResponse> {
  return request<BotListResponse>("/bot", "POST", { action: "list_bots" }, true);
}

export function startGame(botId: string, playerField: PlayerField): Promise<StartGameResponse> {
  const body: StartGameRequest = { action: "start_game", botId, playerField };
  return request<StartGameResponse>("/bot", "POST", body, true);
}

export function shoot(sessionId: string, x: number, y: number): Promise<BotShootResponse> {
  const body: BotShootRequest = { action: "shoot", sessionId, x, y };
  return request<BotShootResponse>("/bot", "POST", body, true);
}

export function getGameState(sessionId: string): Promise<BotShootResponse> {
  const body: GameStateRequest = { action: "state", sessionId };
  return request<BotShootResponse>("/bot", "POST", body, true);
}
