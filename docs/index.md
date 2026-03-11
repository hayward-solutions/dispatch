# Dispatch

[![CI](https://github.com/hayward-solutions/dispatch/actions/workflows/ci.yml/badge.svg)](https://github.com/hayward-solutions/dispatch/actions/workflows/ci.yml)
[![License](https://img.shields.io/badge/License-Apache_2.0-blue.svg)](https://github.com/hayward-solutions/dispatch/blob/main/LICENSE)
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

## Architecture

- **Go stdlib** `net/http` router — no framework dependencies
- **htmx** — server-rendered HTML with dynamic updates
- **Tailwind CSS v4** — utility-first styling
- **PostgreSQL** — user sessions, tracked repositories
- **go-github** — GitHub API client for repos, environments, workflows, and deployments
- **Embedded assets** — templates and static files compiled into the binary via `go:embed`

## License

This project is licensed under the Apache License 2.0 — see the [LICENSE](https://github.com/hayward-solutions/dispatch/blob/main/LICENSE) file for details.
