# Skyrouter
Route Planning for anything that flies

## Requirements

- [Docker](https://docs.docker.com/get-docker/) with the Compose plugin

No local Go installation is required — all build and test commands run inside Docker.

## First-time setup

```bash
cp .env.example .env   # copy the template and fill in your values
make tidy              # generate go.sum (runs go mod tidy inside Docker)
make run                # start postgres + app with hot reload
```

The server will be available at `http://localhost:8080`.

## Environment variables

| Variable | Required | Default | Description |
|---|---|---|---|
| `DB_HOST` | yes | — | Postgres hostname |
| `DB_PORT` | no | `5432` | Postgres port |
| `DB_USER` | yes | — | Postgres user |
| `DB_PASSWORD` | yes | — | Postgres password |
| `DB_NAME` | yes | — | Postgres database name |
| `DB_SSLMODE` | no | `require` | `disable` for local dev |
| `PORT` | no | `8080` | HTTP server port |

## Makefile targets

| Command | Description |
|---|---|
| `make run` | Start postgres + app with air hot reload |
| `make down` | Stop all services and remove volumes |
| `make build` | Compile `./bin/server` inside Docker (dev binary) |
| `make test` | Run test suite against postgres inside Docker |
| `make generate` | Run SQLBoiler to regenerate ORM models from live schema |
| `make tidy` | Run `go mod tidy` inside Docker (updates `go.sum`) |
| `make logs` | Tail logs for all running services |
| `make clean` | Remove containers, volumes, images, and `./bin/` |

### Environment flag

Pass `ENV=<environment>` to target a different deployment template:

```bash
make build ENV=prod    # build production Docker image tagged skyrouter:latest
make test  ENV=ci      # CI test suite (no exposed ports, CI-friendly defaults)
```

Compose templates live in [deploy/](deploy/). `ENV=prod` uses `docker build` directly — no compose file.

## Project structure

```
cmd/server/        entry point
internal/config/   environment-based configuration
internal/db/       database connection pool
internal/models/   SQLBoiler-generated ORM (gitignored, regenerate with make generate)
deploy/            docker-compose templates (local / ci)
```

## ORM models

`internal/models/` is gitignored. After applying your database migrations, regenerate the models with:

```bash
make generate
```

This runs SQLBoiler against the live postgres schema and writes Go types into `internal/models/`.
