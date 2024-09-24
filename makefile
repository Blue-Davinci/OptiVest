.PHONY: help
help:
	@echo Usage:
	@echo run/api: Run the API server
	@echo db/psql            -  connect to the db using psql

.PHONY: run/api
run/api:
	@echo Running API server..
	go run ./cmd/api

# db/psql: connect to the db using psql
.PHONY: db/psql
db/psql:
	psql ${OPTIVEST_DB_DSN}