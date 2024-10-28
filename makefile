.PHONY: help
help:
	@echo Usage:
	@echo run/api: Run the API server
	@echo run/api/origins: Run the API server with CORS origins
	@echo db/psql            -  connect to the db using psql

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