# Configuration

## Environment Variables

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

!!! note
    If `ENCRYPTION_KEY` or `SESSION_SECRET` are not set, they are auto-generated at startup. Sessions will not survive restarts without persistent keys.

## Generating Security Keys

```bash
make keygen
```

This outputs an `ENCRYPTION_KEY` and `SESSION_SECRET`. Both are base64-encoded 32-byte keys used for AES-256 token encryption and session cookie signing respectively.

!!! warning
    Set these explicitly in production. Without persistent keys, all sessions and encrypted tokens are invalidated on restart.
