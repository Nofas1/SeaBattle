# Google OAuth sign-in

## Google Cloud setup

1. In Google Cloud Console, create or select a project and configure the OAuth consent screen.
2. Create an OAuth 2.0 Client ID of type **Web application**.
3. Add the following authorized redirect URI exactly:
   - Local: `http://127.0.0.1:28080/auth/google/callback`
   - Production: `https://YOUR_PROXY_HOST/auth/google/callback`
4. Download the client JSON as `credentials.json`. Keep it out of source control.

The OAuth state cookie uses `Path=/`, so the browser sends it to both
`/auth/google` and `/auth/google/callback`. It is HttpOnly, SameSite=Lax, and
expires after five minutes. A user who leaves Google sign-in open for longer
than five minutes must restart the sign-in flow. `Secure` is enabled except
when `IS_PROD=false` and the configured callback URL uses HTTP on a loopback IP.

## Run locally

1. Copy `.env.example` to `.env`. Set `POSTGRES_PASSWORD` and make its password
   match the password in `DATABASE_URL`. Set `JWT_SECRET` to at least 32 bytes.
2. Put the downloaded Google client JSON at the repository root as
   `credentials.json`. Compose mounts that file read-only at
   `/run/secrets/google-oauth.json`; it is never copied into the proxy image.
   Alternatively, when running the proxy directly, set
   `GOOGLE_CLIENT_ID` and `GOOGLE_CLIENT_SECRET` instead of
   `GOOGLE_CREDENTIALS_JSON`.
3. Set `GOOGLE_REDIRECT_URL=http://127.0.0.1:28080/auth/google/callback`,
   `FRONTEND_URL=http://127.0.0.1:8080`, and `IS_PROD=false`.
4. Start the frontend at `http://127.0.0.1:8080` and start PostgreSQL and the
   services with `docker compose -f compose.yaml up --build`.
5. Open the frontend and select **Continue with Google**.

Register the exact local callback URL
`http://127.0.0.1:28080/auth/google/callback` in Google Cloud Console. The
Google error `redirect_uri_mismatch` means that exact URL is not registered
for the client ID used by the running proxy. Use `127.0.0.1` consistently in
the browser URL and `GOOGLE_REDIRECT_URL`; the browser scopes the HttpOnly
state cookie to the hostname that started sign-in, so it will not send it to a
callback on a different hostname.

The smart bot uses its own `smart_bot_schema_migrations` tracking table. This
keeps its migration versions independent from the proxy's migrations even
though both services use the same PostgreSQL database.

For production, configure HTTPS URLs, set `IS_PROD=true`, keep the client
credentials in a read-only secret mount or secret manager, and register the
exact production callback URI in Google Cloud Console. The proxy permits
cross-origin API requests only from the configured `FRONTEND_URL` origin.
OAuth itself uses browser redirects rather than cross-origin fetches.

The callback links a local account whose `name` matches the verified Google
email (case-insensitively), or creates a new Google-backed user. Access and
refresh tokens are not persisted. The migration rollback preserves accounts
but removes their Google provider link; OAuth-only accounts receive an empty
password value so the legacy password column can be restored as NOT NULL.
