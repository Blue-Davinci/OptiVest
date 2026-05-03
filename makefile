.PHONY: help
help:
	@echo Usage:
	@echo "  run/api             - Run the API server"
	@echo "  run/api/origins     - Run the API server with CORS origins"
	@echo "  db/psql             - Connect to the db using psql"
	@echo "  build/api           - Build the cmd/api application"
	@echo "  audit               - Run vet, staticcheck, govulncheck, and the test suite (matches CI)"
	@echo "  tidy                - Format code and tidy go.mod/go.sum"
	@echo "  test                - Run the full test suite with -race"

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

## audit: full local equivalent of CI (vet + staticcheck + govulncheck + test)
.PHONY: audit
audit:
	@echo 'Verifying go.mod is tidy...'
	go mod tidy
	git diff --exit-code go.mod go.sum
	@echo 'Running go vet...'
	go vet ./...
	@echo 'Running staticcheck (install: go install honnef.co/go/tools/cmd/staticcheck@latest)...'
	staticcheck ./...
	@echo 'Running govulncheck (install: go install golang.org/x/vuln/cmd/govulncheck@latest)...'
	govulncheck ./...
	@echo 'Running tests with race detector...'
	go test -race -count=1 -timeout=120s ./...