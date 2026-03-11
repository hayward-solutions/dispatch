# Deployment Guide

This guide covers everything needed to deploy Dispatch in production.

## Prerequisites

- Docker and Docker Compose (recommended), or Go 1.25+ and PostgreSQL 16+
- A public domain or IP address for OAuth callbacks
- HTTPS (strongly recommended for production)

## 1. Create a GitHub OAuth App

Dispatch uses GitHub OAuth for authentication. You need to create an OAuth App in your GitHub account or organization.

### Steps

1. Go to **GitHub Settings > Developer settings > OAuth Apps > New OAuth App**
   - For a personal account: https://github.com/settings/developers
   - For an organization: `https://github.com/organizations/YOUR_ORG/settings/applications`

2. Fill in the application details:
   - **Application name**: `Dispatch` (or any name you prefer)
   - **Homepage URL**: Your deployment URL (e.g., `https://dispatch.example.com`)
   - **Authorization callback URL**: `https://dispatch.example.com/auth/github/callback`

3. Click **Register application**

4. On the next page, copy the **Client ID**

5. Click **Generate a new client secret** and copy the secret immediately — it won't be shown again

6. Save both values for the configuration step below

### Required Scopes

Dispatch requests the following scopes during OAuth login:

- `repo` — access to repositories, environments, and deployments
- `read:org` — read organization membership (for org repository access)

## 2. Configuration

### Generate Security Keys

```bash
make keygen
```

This outputs an `ENCRYPTION_KEY` and `SESSION_SECRET`. Both are base64-encoded 32-byte keys used for AES-256 token encryption and session cookie signing respectively.

> **Important:** Set these explicitly in production. Without persistent keys, all sessions and encrypted tokens are invalidated on restart.

### Environment Variables

Create a `.env` file from the template:

```bash
cp .env.example .env
```

Configure the required variables:

```env
# Database
DATABASE_URL=postgres://dispatch:YOUR_DB_PASSWORD@localhost:5432/dispatch?sslmode=require

# GitHub OAuth (from step 1)
GITHUB_CLIENT_ID=your_client_id
GITHUB_CLIENT_SECRET=your_client_secret

# Security keys (from make keygen)
ENCRYPTION_KEY=your_generated_encryption_key
SESSION_SECRET=your_generated_session_secret

# Production settings
PORT=8080
BASE_URL=https://dispatch.example.com
LOG_LEVEL=info
```

| Variable | Required | Default | Description |
|----------|----------|---------|-------------|
| `DATABASE_URL` | Yes | — | PostgreSQL connection string |
| `GITHUB_CLIENT_ID` | Yes | — | GitHub OAuth app client ID |
| `GITHUB_CLIENT_SECRET` | Yes | — | GitHub OAuth app client secret |
| `ENCRYPTION_KEY` | No | Auto-generated | Base64-encoded 32-byte key for AES-256 token encryption |
| `SESSION_SECRET` | No | Auto-generated | Base64-encoded 32-byte key for session cookies |
| `PORT` | No | `8080` | HTTP server port |
| `BASE_URL` | No | `http://localhost:PORT` | Public URL (must match OAuth callback domain) |
| `LOG_LEVEL` | No | `info` | Log level: `debug`, `info`, `warn`, `error` |
| `SESSION_MAX_AGE` | No | `2592000` | Session lifetime in seconds (default: 30 days) |

## 3. Deploy

### Docker Compose (Recommended)

The simplest way to run Dispatch with PostgreSQL:

```bash
cp .env.example .env
# Edit .env with your configuration (see step 2)

docker compose up -d
```

This starts both PostgreSQL and the Dispatch server. The `DATABASE_URL` is automatically configured for the internal Docker network. The database schema is applied automatically on first startup.

To update to a newer version:

```bash
docker compose pull
docker compose up -d
```

### Docker (Standalone)

If you have an existing PostgreSQL instance:

```bash
# Build CSS first (it gets embedded in the binary)
make css

# Build the container
docker build -t dispatch .

# Run
docker run -d \
  --name dispatch \
  -p 8080:8080 \
  --env-file .env \
  dispatch
```

### Binary

Build and run directly:

```bash
make build
./bin/dispatch
```

The binary embeds all templates and static assets, so no additional files are needed at runtime. Ensure PostgreSQL is accessible via `DATABASE_URL`.

## 4. Reverse Proxy

Dispatch listens on HTTP. Use a reverse proxy for HTTPS in production.

### Nginx

```nginx
server {
    listen 443 ssl;
    server_name dispatch.example.com;

    ssl_certificate     /path/to/cert.pem;
    ssl_certificate_key /path/to/key.pem;

    location / {
        proxy_pass http://127.0.0.1:8080;
        proxy_set_header Host $host;
        proxy_set_header X-Real-IP $remote_addr;
        proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;
        proxy_set_header X-Forwarded-Proto $scheme;
    }
}
```

### Caddy

```
dispatch.example.com {
    reverse_proxy localhost:8080
}
```

Caddy handles TLS certificates automatically.

## 5. Health Check

Dispatch exposes a health check endpoint:

```
GET /healthz
```

This verifies the database connection is alive. Use it for load balancer health checks, Docker health checks, or monitoring.

## Troubleshooting

### OAuth callback error

Ensure `BASE_URL` in your `.env` matches the **Authorization callback URL** in your GitHub OAuth App settings. The callback URL must be `{BASE_URL}/auth/github/callback`.

### Sessions lost on restart

Set `ENCRYPTION_KEY` and `SESSION_SECRET` explicitly. Without these, keys are auto-generated at startup and all existing sessions become invalid after a restart.

### Cannot access organization repositories

The GitHub user must have appropriate access to the organization's repositories. Dispatch uses the authenticated user's permissions — it cannot access repositories the user doesn't have access to.

### Database connection refused

Verify `DATABASE_URL` is correct and PostgreSQL is reachable. If using Docker Compose, the database may need a few seconds to start — the health check handles this automatically.
