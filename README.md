# Dispatch

[![CI](https://github.com/hayward-solutions/dispatch.v2/actions/workflows/ci.yml/badge.svg)](https://github.com/hayward-solutions/dispatch.v2/actions/workflows/ci.yml)
[![License](https://img.shields.io/badge/License-Apache_2.0-blue.svg)](LICENSE)
[![Go](https://img.shields.io/badge/Go-1.25+-00ADD8?logo=go&logoColor=white)](https://go.dev)

A self-hosted dashboard for managing GitHub Actions workflows and environments. Authenticate with GitHub OAuth, track repositories, manage environment variables and secrets, dispatch workflows, and view deployment history — all from a single UI.

## Features

- **GitHub OAuth login** — sign in with your GitHub account
- **Repository tracking** — browse and track your GitHub repositories
- **Environment management** — create, configure, and delete GitHub environments
- **Variables & secrets** — CRUD operations on environment variables and secrets
- **Workflow dispatch** — trigger `workflow_dispatch` workflows with custom inputs
- **Deployment history** — view deployment status and history per environment
- **Command palette** — quick navigation with `Cmd+K`

## Prerequisites

- Go 1.25+
- PostgreSQL 16+
- [Tailwind CSS CLI](https://tailwindcss.com/blog/standalone-cli) (standalone binary)
- A [GitHub OAuth App](https://docs.github.com/en/apps/oauth-apps/building-oauth-apps/creating-an-oauth-app)

## Getting Started

```bash
# Clone the repository
git clone https://github.com/hayward-solutions/dispatch.v2.git
cd dispatch.v2

# Copy environment template
cp .env.example .env

# Generate encryption and session keys
make keygen
# Copy the output values into .env

# Configure your GitHub OAuth app credentials in .env
# Set the callback URL to: http://localhost:8080/auth/github/callback

# Start PostgreSQL
make db

# Run in development mode (requires air: go install github.com/air-verse/air@latest)
make dev
```

## Configuration

| Variable | Required | Default | Description |
|----------|----------|---------|-------------|
| `DATABASE_URL` | Yes | — | PostgreSQL connection string |
| `GITHUB_CLIENT_ID` | Yes | — | GitHub OAuth app client ID |
| `GITHUB_CLIENT_SECRET` | Yes | — | GitHub OAuth app client secret |
| `ENCRYPTION_KEY` | No | Auto-generated | Base64-encoded 32-byte key for AES-256 token encryption |
| `SESSION_SECRET` | No | Auto-generated | Base64-encoded 32-byte key for session cookies |
| `PORT` | No | `8080` | HTTP server port |
| `BASE_URL` | No | `http://localhost:PORT` | Public URL (used for OAuth callback) |
| `LOG_LEVEL` | No | `info` | Log level: `debug`, `info`, `warn`, `error` |
| `SESSION_MAX_AGE` | No | `2592000` | Session lifetime in seconds (default: 30 days) |

> **Note:** If `ENCRYPTION_KEY` or `SESSION_SECRET` are not set, they are auto-generated at startup. Sessions will not survive restarts without persistent keys.

## Development

```bash
make dev        # Start with hot-reload (air) + database
make run        # Build CSS and run
make build      # Build binary to bin/dispatch
make css        # Build CSS once
make css-watch  # Watch CSS for changes
make db         # Start PostgreSQL
make db-stop    # Stop PostgreSQL
make keygen     # Generate encryption and session keys
make clean      # Remove build artifacts
```

## Deployment

### Docker

```bash
# Build CSS first (it gets embedded in the binary)
make css

# Build the container
docker build -t dispatch .

# Run
docker run -p 8080:8080 --env-file .env dispatch
```

### Docker Compose

```bash
cp .env.example .env
# Edit .env with your configuration

docker compose up -d
```

The compose file starts both PostgreSQL and the Dispatch server. The `DATABASE_URL` is automatically configured to use the internal Docker network.

## Architecture

- **Go stdlib** `net/http` router — no framework dependencies
- **htmx** — server-rendered HTML with dynamic updates
- **Tailwind CSS v4** — utility-first styling
- **PostgreSQL** — user sessions, tracked repositories
- **go-github** — GitHub API client for repos, environments, workflows, and deployments
- **Embedded assets** — templates and static files compiled into the binary via `go:embed`

## Contributing

Contributions are welcome! Please see [CONTRIBUTING.md](CONTRIBUTING.md) for guidelines.

## Security

To report a security vulnerability, please see [SECURITY.md](SECURITY.md).

## License

This project is licensed under the Apache License 2.0 — see the [LICENSE](LICENSE) file for details.
