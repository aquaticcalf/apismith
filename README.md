# API Test Console

Internal console for exercising Nimbus backend APIs from the same OpenAPI
contract that `oapi-codegen` uses to generate the server.

This is **Phase 1 (API Explorer)** — not a Postman replacement. The OpenAPI
spec is the single source of truth: new endpoints appear here automatically.

```
openapi.yaml  ──►  oapi-codegen  ──►  backend server
       │
       └──►  API Test Console  ──►  DEV / STAGING API
                    │
                    └── Cognito JWT (shared with jwt-cli)
```

## What you can do

1. Start the console
2. Select **DEV** or **STAGING**
3. Enter or reuse a Cognito Client ID
4. Generate a JWT (same SRP flow as [jwt-token-printer](https://github.com/aegion-dynamic/jwt-token-printer))
5. Search and select any operation from the OpenAPI spec
6. Fill path/query params and a form or JSON body
7. Send the request through the local proxy
8. Inspect status, timing, headers, and body

The browser talks only to this process. The process attaches
`Authorization: Bearer <access token>` and forwards the call, which avoids
CORS and keeps Cognito secrets off the frontend.

## Run

```sh
cd api-console
cp .env.example .env          # fill Cognito + credentials
make run                      # http://localhost:8090
```

The console listens on **:8090** so it does not collide with the backend on
`:8080`. Override with `CONSOLE_LISTEN`.

### OpenAPI spec

By default the console loads the sibling backend spec:

```
../framework-backend/nimbus_openapi_spec.yaml
```

Override with `CONSOLE_OPENAPI_SPEC`, or place a copy at `openapi/openapi.yaml`.

Do not maintain a second hand-written API list.

### Environments

Edit `config/environments.yaml`. DEV and STAGING are enabled; PRODUCTION, if
you add it with `production: true`, requires an explicit confirmation before
a request is sent.

Secrets do **not** belong in that file. Put them in:

- environment variables (`COGNITO_*`, `CONSOLE_USERNAME`, `CONSOLE_PASSWORD`)
- `config/credentials.yaml` (gitignored; see `config/credentials.yaml.example`)

The credentials file is compatible with jwt-token-printer's `config.json`.

## JWT CLI

The printer is no longer a one-off script. Both the console and CLI call
`auth/cognito`:

```sh
make jwt ARGS=config/credentials.yaml
# or, drop-in for the old tool:
go run ./cmd/jwt-cli path/to/config.json
```

The backend verifies Cognito **access** tokens (`token_use=access`), so the
console attaches the access token, not the ID token.

## Layout

```
auth/cognito/     reusable Cognito token generation (SRP + password)
auth/             in-memory auth manager (JWT never written to disk)
openapi/          kin-openapi parser → endpoint catalog
request/          URL builder, proxy executor, masked history
environments/     DEV / STAGING / PRODUCTION
server/           local HTTP API + request proxy
ui/               explorer, request builder, response viewer
cmd/console/      web process
cmd/jwt-cli/      JWT printer
```

## Security

- JWTs live in memory (browser + process). They are not logged.
- `Authorization` is stored in history as `Bearer ********`.
- Client secrets stay on the server; the UI only sees client IDs.
- Production requests require `confirm_production`.

## Later phases (not in this MVP)

- Saved requests
- Repeatable test cases, variables, cleanup
- `api-test run --environment staging --suite regression`
- CI/CD gate on failed tests
