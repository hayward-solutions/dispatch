# Getting Started

## Prerequisites

- Go 1.25+
- PostgreSQL 16+
- [Tailwind CSS CLI](https://tailwindcss.com/blog/standalone-cli) (standalone binary)
- A [GitHub OAuth App](https://docs.github.com/en/apps/oauth-apps/building-oauth-apps/creating-an-oauth-app)

## Quick Start

```bash
# Clone the repository
git clone https://github.com/hayward-solutions/dispatch.git
cd dispatch

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

## Development Commands

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
