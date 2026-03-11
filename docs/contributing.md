# Contributing

Thank you for your interest in contributing to Dispatch!

## Getting Started

1. Fork the repository
2. Clone your fork:
   ```bash
   git clone https://github.com/YOUR_USERNAME/dispatch.git
   cd dispatch
   ```
3. Set up your development environment (see [Getting Started](getting-started.md))

## Development Setup

### Prerequisites

- Go 1.25+
- PostgreSQL 16+ (or Docker)
- [Tailwind CSS CLI](https://tailwindcss.com/blog/standalone-cli) (standalone binary)
- A [GitHub OAuth App](https://docs.github.com/en/apps/oauth-apps/building-oauth-apps/creating-an-oauth-app) for testing

### Running Locally

```bash
cp .env.example .env
make keygen          # Generate keys, copy output to .env
# Configure GitHub OAuth credentials in .env
make dev             # Starts PostgreSQL + hot-reload server
```

## Making Changes

### Branch Naming

Use descriptive branch names:

- `feat/add-workflow-filters` for new features
- `fix/oauth-token-refresh` for bug fixes
- `docs/update-api-reference` for documentation

### Code Style

- Follow standard Go conventions (`gofmt`, `go vet`)
- Use Go stdlib patterns — this project intentionally avoids heavy frameworks
- Keep templates in `web/templates/` organized by `layouts/`, `pages/`, and `partials/`
- Use htmx attributes for dynamic behavior rather than custom JavaScript

### Testing

Run the test suite before submitting:

```bash
go test ./...
```

Add tests for new functionality, especially for:

- Model layer changes (`internal/models/`)
- GitHub API integration changes (`internal/github/`)

### CSS

Tailwind CSS is compiled from `web/static/css/input.css`. After making style changes:

```bash
make css              # One-time build
make css-watch        # Watch mode during development
```

The compiled CSS (`web/static/css/app.css`) is committed to the repository so that `go:embed` can include it in the binary.

## Submitting Changes

1. Ensure all tests pass: `go test ./...`
2. Ensure code passes vet: `go vet ./...`
3. Commit your changes with a clear message
4. Push to your fork and open a pull request against `main`

### Pull Request Guidelines

- Provide a clear description of the change and motivation
- Reference any related issues
- Keep PRs focused — one feature or fix per PR
- Include screenshots for UI changes

## Reporting Issues

- Use GitHub Issues to report bugs or request features
- Include steps to reproduce for bug reports
- Check existing issues before opening a new one

## License

By contributing, you agree that your contributions will be licensed under the [Apache License 2.0](https://github.com/hayward-solutions/dispatch/blob/main/LICENSE).
