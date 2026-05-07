.PHONY: help
help:
	@echo Usage:
	@echo "  run/api             - Run the API server"
	@echo "  run/api/origins     - Run the API server with CORS origins"
	@echo "  db/psql             - Connect to the db using psql"
	@echo "  build/api           - Build the cmd/api application"
	@echo "  audit               - Run vet, golangci-lint, govulncheck, and the test suite (matches CI)"
	@echo "  lint                - Run golangci-lint (matches the CI lint job)"
	@echo "  tidy                - Format code and tidy go.mod/go.sum"
	@echo "  test                - Run the full test suite with -race"
	@echo "  docker/up           - Build images and bring up the local stack (postgres, redis, migrations, api)"
	@echo "  docker/down         - Stop and remove the local stack (volumes preserved)"
	@echo "  docker/down/clean   - Stop the stack AND drop the postgres data volume"
	@echo "  docker/logs         - Tail the api service logs"
	@echo "  docker/migrate      - Re-run the goose up migrations against the running postgres"
	@echo "  docker/build        - Build the api image without starting anything"
	@echo "  docker/ps           - Show the running compose services"

.PHONY: run/api
run/api:
	@echo Running API server..
	go run ./cmd/api

.PHONY: run/api/origins
run/api/origins:
	@echo Running API server with CORS origins..
	go run ./cmd/api -cors-trusted-origins="http://localhost:5173"

# db/psql: connect to the db using psql
.PHONY: db/psql
db/psql:
	psql ${OPTIVEST_DB_DSN}

## build/api: build the cmd/api application
.PHONY: build/api
build/api:
	@echo 'Building cmd/api...'
	go build -ldflags '-s' -o ./bin/api.exe ./cmd/api
## For linux: GOOS=linux GOARCH=amd64 go build -ldflags='-s' -o bin/linux_amd64_api ./cmd/api

## test: run the full test suite with the race detector
.PHONY: test
test:
	@echo 'Running tests...'
	go test -race -count=1 -timeout=120s ./...

## tidy: format code and tidy module files
.PHONY: tidy
tidy:
	@echo 'Formatting and tidying...'
	gofmt -s -w .
	go mod tidy

## lint: run golangci-lint with the repo's .golangci.yml ruleset
.PHONY: lint
lint:
	@echo 'Running golangci-lint (install: https://golangci-lint.run/usage/install/)...'
	golangci-lint run --timeout=5m ./...

## audit: full local equivalent of CI (tidy + vet + lint + govulncheck + test)
.PHONY: audit
audit:
	@echo 'Verifying go.mod is tidy...'
	go mod tidy
	git diff --exit-code go.mod go.sum
	@echo 'Running go vet...'
	go vet ./...
	@echo 'Running golangci-lint (install: https://golangci-lint.run/usage/install/)...'
	golangci-lint run --timeout=5m ./...
	@echo 'Running govulncheck (install: go install golang.org/x/vuln/cmd/govulncheck@latest)...'
	govulncheck ./...
	@echo 'Running tests with race detector...'
	go test -race -count=1 -timeout=120s ./...

## docker/up: build images and bring up the full local stack in detached mode
.PHONY: docker/up
docker/up:
	@echo 'Bringing up postgres, redis, migrate, api...'
	docker compose up --build -d

## docker/down: stop the stack but preserve the postgres data volume
.PHONY: docker/down
docker/down:
	@echo 'Stopping stack (volumes preserved)...'
	docker compose down

## docker/down/clean: stop the stack AND wipe the postgres data volume
.PHONY: docker/down/clean
docker/down/clean:
	@echo 'Stopping stack and dropping the postgres volume...'
	docker compose down -v

## docker/logs: tail the api service logs (Ctrl-C to stop)
.PHONY: docker/logs
docker/logs:
	docker compose logs -f api

## docker/migrate: re-run goose up against the running postgres
.PHONY: docker/migrate
docker/migrate:
	@echo 'Re-running migrations...'
	docker compose run --rm migrate up

## docker/build: build the api image without bringing the stack up
.PHONY: docker/build
docker/build:
	docker compose build api

## docker/ps: list the running compose services
.PHONY: docker/ps
docker/ps:
	docker compose ps