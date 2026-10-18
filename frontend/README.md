# SeaBattle frontend

The frontend is a TypeScript SPA without a UI framework. Build it with Node.js and the existing `esbuild` development dependency:

```powershell
cd frontend
npm install
node build.mjs
```

The bot cards are loaded at runtime from the authenticated `POST /bot` action `list_bots`; the frontend bundle contains no bot IDs.

## Run the game locally

1. Configure `DATABASE_URL`, `JWT_SECRET` (at least 32 bytes), and `AI_KEY` as required by the services. Start PostgreSQL separately; `compose.yaml` does not define a database container.
2. Start the proxy and bot services:

   ```powershell
   docker compose -f compose.yaml up --build -d
   ```

   For a local Go build, build services with `go build ./...` and run the proxy plus `simple_bot`, `smart_bot`, and `ai_bot` with their own working directories so each reads its config file.
3. From the repository root, serve the static frontend:

   ```powershell
   python -m http.server 8080 --directory frontend
   ```

4. Open <http://localhost:8080>, register or sign in, place all ten ships, then select an opponent and play. The proxy accepts browser requests from `http://localhost:8080` and `http://localhost:3000`.

The proxy listens on port `28080` by default. To serve through the same-origin Go frontend server instead, build the bundle and run `go run ./frontend`; it defaults to `http://127.0.0.1:3000`.

## Game protocol

The proxy owns each game session in memory, tied to the authenticated user and an opaque `sessionId`. Sessions expire after two hours and are periodically collected. It validates the submitted fleet and every player shot, stores both fields and shot histories, detects sunk ships and the winner, and only returns the player's fleet, shot results, and game status. The opponent's field is never serialized to the browser.

`POST /bot` actions are `list_bots`, `start_game`, `shoot`, and `state`. A `shoot` request applies one player shot and, after a miss, one bot shot. If the bot hits and retains its turn, the frontend requests `state` again to advance the next bot shot until the turn returns to the player.
