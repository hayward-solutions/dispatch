.PHONY: run build dev db db-stop clean css css-watch

# Run the application
run: css
	go run ./cmd/dispatch

# Build the binary
build: css
	go build -o bin/dispatch ./cmd/dispatch

# Build CSS
css:
	./tailwindcss -i web/static/css/input.css -o web/static/css/app.css --minify

# Watch CSS for changes
css-watch:
	./tailwindcss -i web/static/css/input.css -o web/static/css/app.css --watch

# Start database
db:
	docker compose up -d postgres

# Stop database
db-stop:
	docker compose down

# Run with auto-reload (requires air: go install github.com/air-verse/air@latest)
dev: db
	air

# Download dependencies
deps:
	go mod tidy

# Clean build artifacts
clean:
	rm -rf bin/ tmp/

# Generate encryption and session keys
keygen:
	@echo "ENCRYPTION_KEY=$$(openssl rand -base64 32)"
	@echo "SESSION_SECRET=$$(openssl rand -base64 32)"
