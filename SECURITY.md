# Security Policy

## Reporting a Vulnerability

If you discover a security vulnerability in Dispatch, please report it responsibly.

**Do not open a public GitHub issue for security vulnerabilities.**

Instead, please email security concerns to the project maintainers. Include:

- A description of the vulnerability
- Steps to reproduce the issue
- Any potential impact

We will acknowledge receipt within 48 hours and provide a timeline for a fix.

## Supported Versions

Security updates are applied to the latest release only.

## Security Considerations

Dispatch handles sensitive data including GitHub OAuth tokens and environment secrets. The following measures are in place:

- **Token encryption**: GitHub OAuth tokens are encrypted at rest using AES-256 (configurable via `ENCRYPTION_KEY`)
- **Session security**: Sessions use signed cookies via `gorilla/securecookie` (configurable via `SESSION_SECRET`)
- **Minimal attack surface**: The production binary runs on a distroless container image as a non-root user
- **No client-side secrets**: All GitHub API calls are made server-side

### Deployment Recommendations

- Always set `ENCRYPTION_KEY` and `SESSION_SECRET` explicitly in production (use `make keygen`)
- Use HTTPS in production (set `BASE_URL` to your HTTPS URL)
- Restrict network access to the PostgreSQL instance
- Regularly rotate OAuth tokens and session secrets
