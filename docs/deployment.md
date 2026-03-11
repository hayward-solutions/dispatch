# Deployment

## Prerequisites

- Docker and Docker Compose (recommended), or Go 1.25+ and PostgreSQL 16+
- A public domain or IP address for OAuth callbacks
- HTTPS (strongly recommended for production)

## 1. Create a GitHub OAuth App

Dispatch uses GitHub OAuth for authentication. You need to create an OAuth App in your GitHub account or organization.

1. Go to **GitHub Settings > Developer settings > OAuth Apps > New OAuth App**
    - For a personal account: [github.com/settings/developers](https://github.com/settings/developers)
    - For an organization: `https://github.com/organizations/YOUR_ORG/settings/applications`

2. Fill in the application details:
    - **Application name**: `Dispatch` (or any name you prefer)
    - **Homepage URL**: Your deployment URL (e.g., `https://dispatch.example.com`)
    - **Authorization callback URL**: `https://dispatch.example.com/auth/github/callback`

3. Click **Register application**

4. Copy the **Client ID**

5. Click **Generate a new client secret** and copy the secret immediately

6. Save both values for the configuration step

### Required Scopes

Dispatch requests the following scopes during OAuth login:

- `repo` — access to repositories, environments, and deployments
- `read:org` — read organization membership (for org repository access)

## 2. Deploy

### Docker Compose (Recommended)

```bash
cp .env.example .env
# Edit .env with your configuration

docker compose up -d
```

This starts both PostgreSQL and the Dispatch server. The database schema is applied automatically on first startup.

To update:

```bash
docker compose pull
docker compose up -d
```

### Docker (Standalone)

If you have an existing PostgreSQL instance:

```bash
make css
docker build -t dispatch .

docker run -d \
  --name dispatch \
  -p 8080:8080 \
  --env-file .env \
  dispatch
```

### Binary

```bash
make build
./bin/dispatch
```

The binary embeds all templates and static assets, so no additional files are needed at runtime.

## 3. Reverse Proxy

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

## 4. Health Check

```
GET /healthz
```

Verifies the database connection is alive. Use it for load balancer health checks, Docker health checks, or monitoring.

## Troubleshooting

### OAuth callback error

Ensure `BASE_URL` in your `.env` matches the **Authorization callback URL** in your GitHub OAuth App settings. The callback URL must be `{BASE_URL}/auth/github/callback`.

### Sessions lost on restart

Set `ENCRYPTION_KEY` and `SESSION_SECRET` explicitly. Without these, keys are auto-generated at startup and all existing sessions become invalid.

### Cannot access organization repositories

The GitHub user must have appropriate access to the organization's repositories. Dispatch uses the authenticated user's permissions.

### Database connection refused

Verify `DATABASE_URL` is correct and PostgreSQL is reachable. If using Docker Compose, the database may need a few seconds to start.
