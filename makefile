.PHONY: help
help:
	@echo Usage:
	@echo run/api: Run the API server
	@echo run/api/origins: Run the API server with CORS origins
	@echo db/psql            -  connect to the db using psql
	@echo build/api          -  build the cmd/api application

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