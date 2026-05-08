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
	@echo "  generate            - Regenerate sqlc Go code from internal/sql/queries"
	@echo "  verify-generate     - Fail if sqlc generated code drifted from internal/sql/queries"
	@echo "  docker/up           - Build images and bring up the local stack (postgres, redis, migrations, api)"
	@echo "  docker/down         - Stop and remove the local stack (volumes preserved)"
	@echo "  docker/down/clean   - Stop the stack AND drop the postgres data volume"
	@echo "  docker/logs         - Tail the api service logs"
	@echo "  docker/migrate      - Re-run the goose up migrations against the running postgres"
	@echo "  docker/build        - Build the api image without starting anything"
	@echo "  docker/ps           - Show the running compose services"
	@echo "  dev/up              - Start the host valkey service for local API runs"
	@echo "  dev/down            - Stop the host valkey service"
	@echo "  dev/status          - Show whether the host valkey service is running"
	@echo "  k8s/cluster/up      - Create the local kind cluster (rootless podman)"
	@echo "  k8s/cluster/down    - Delete the local kind cluster"
	@echo "  k8s/cluster/status  - Show kind cluster + node info"
	@echo "  k8s/dev-deps/up     - Apply postgres + redis dev-dep manifests"
	@echo "  k8s/dev-deps/down   - Remove the postgres + redis dev-dep manifests"
	@echo "  k8s/image/build     - Build optivest-api + optivest-migrate images locally"
	@echo "  k8s/image/load      - Side-load both images into the kind cluster"
	@echo "  k8s/secret/create   - Create/refresh the optivest-secrets Secret from cmd/api/.env"
	@echo "  k8s/migrate         - Run goose up as a one-shot Job (chart v0.1 manual step)"
	@echo "  k8s/install         - helm upgrade --install the chart against the kind cluster"
	@echo "  k8s/uninstall       - helm uninstall the chart"
	@echo "  k8s/forward         - Port-forward API:4000 + SSE:4001 to localhost"
	@echo "  k8s/logs            - Tail the optivest API pod logs"
	@echo "  k8s/smoke           - Full bring-up: cluster + deps + image + secret + migrate + install"

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

## generate: regenerate sqlc Go from internal/sql/queries.
## Uses the sqlc version pinned in go.mod's tool directive, so a fresh
## clone or a CI runner with no preinstalled sqlc still produces
## byte-identical output. Edit internal/sql/queries/*.sql, run this,
## and commit the diff in internal/database/.
.PHONY: generate
generate:
	@echo 'Running sqlc generate (pinned via go.mod tool directive)...'
	go tool sqlc generate

## verify-generate: regenerate, then fail if anything drifted.
## Mirrors the verify-generate CI job. The intent is that any PR that
## touches internal/sql/ or the schema must also commit the regenerated
## Go, otherwise reviewers see a quiet drift between the SQL source of
## truth and the typed Go façade.
.PHONY: verify-generate
verify-generate:
	@echo 'Verifying generated code is up to date...'
	go tool sqlc generate
	@if ! git diff --quiet -- internal/database/ internal/sql/; then \
		echo ""; \
		echo "Generated code is out of date. Run 'make generate' and commit the result."; \
		echo "Drift detected:"; \
		git --no-pager diff --stat -- internal/database/ internal/sql/; \
		exit 1; \
	fi

## audit: full local equivalent of CI (tidy + generate-drift + vet + lint + govulncheck + test)
.PHONY: audit
audit:
	@echo 'Verifying go.mod is tidy...'
	go mod tidy
	git diff --exit-code go.mod go.sum
	@$(MAKE) verify-generate
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

# -----------------------------------------------------------------------------
# Host-side dev dependencies (valkey/redis on the loopback interface).
# -----------------------------------------------------------------------------
# These targets manage the systemd service for the locally-installed
# valkey package so the binary in `make run/api` has a Redis-compatible
# server to talk to without needing the full docker-compose stack up.
#
# The service is intentionally NOT enabled at boot — `make dev/up` starts
# it for the current session and `make dev/down` stops it again. This
# keeps idle hosts free of a listening Redis when you're not developing.
#
# If your distro packages valkey under a different unit name (e.g.
# `redis.service` on the legacy AUR build), override DEV_REDIS_UNIT:
#   make dev/up DEV_REDIS_UNIT=redis
DEV_REDIS_UNIT ?= valkey

## dev/up: start the host valkey service for local API runs
.PHONY: dev/up
dev/up:
	@echo 'Starting $(DEV_REDIS_UNIT)...'
	sudo systemctl start $(DEV_REDIS_UNIT)
	@systemctl is-active --quiet $(DEV_REDIS_UNIT) && \
		echo "$(DEV_REDIS_UNIT) running on 127.0.0.1:6379"

## dev/down: stop the host valkey service
.PHONY: dev/down
dev/down:
	@echo 'Stopping $(DEV_REDIS_UNIT)...'
	sudo systemctl stop $(DEV_REDIS_UNIT)

## dev/status: show whether the host valkey service is running
.PHONY: dev/status
dev/status:
	@systemctl status $(DEV_REDIS_UNIT) --no-pager || true

# -----------------------------------------------------------------------------
# Kubernetes / Helm — local kind cluster + chart smoke workflow.
# -----------------------------------------------------------------------------
# These targets wrap the smoke-test path the chart at deploy/charts/optivest
# was developed against: a rootless-podman-backed kind cluster, side-loaded
# images (no registry), the postgres + redis dev-deps from deploy/dev/, and
# the chart deployed via `helm upgrade --install`.
#
# All variables below are overridable on the command line, e.g.
#   make k8s/install CHART_ENV=staging IMAGE_TAG=0.2.0
#
# The chart appVersion lives in deploy/charts/optivest/Chart.yaml; IMAGE_TAG
# here just has to match whatever tag the local images were built with.
KIND_CLUSTER         ?= optivest
K8S_NAMESPACE        ?= optivest
KIND_PROVIDER        ?= podman
CHART_PATH           ?= deploy/charts/optivest
CHART_RELEASE        ?= optivest
IMAGE_TAG            ?= 0.1.0
DEV_DEPS_MANIFEST    ?= deploy/dev/dev-deps.yaml
MIGRATE_JOB_MANIFEST ?= deploy/dev/migrate-job.yaml
SCHEMA_DIR           ?= internal/sql/schema
ENV_FILE             ?= cmd/api/.env
# Service hostname the in-cluster postgres dev-dep listens on. Used to
# rewrite OPTIVEST_DB_DSN when seeding the optivest-secrets Secret so the
# DSN baked into cmd/api/.env (which points at localhost) doesn't follow
# us into the cluster.
K8S_DB_DSN           ?= postgres://optivest:optivest@postgres:5432/optivest?sslmode=disable
# `helm install --set` passes config.env=$(CHART_ENV) to the chart so
# missing optional secrets (SMTP, vendor API keys) are warnings rather
# than fatal startup errors. Override to "staging" or "production" once
# the optivest-secrets Secret is fully populated.
CHART_ENV            ?= development
# Rootless podman tags side-loaded images with a `localhost/` prefix,
# which the chart's image.repository default ("optivest-api") does not
# match. Override the chart value so kubelet can resolve the image.
CHART_IMAGE_REPO     ?= localhost/optivest-api

# kind respects the env var on every command (create/get/delete/load), so
# we set it once here and reuse via $(KIND).
KIND := KIND_EXPERIMENTAL_PROVIDER=$(KIND_PROVIDER) kind

## k8s/cluster/up: create the local kind cluster (idempotent)
.PHONY: k8s/cluster/up
k8s/cluster/up:
	@if $(KIND) get clusters 2>/dev/null | grep -qx '$(KIND_CLUSTER)'; then \
		echo "kind cluster '$(KIND_CLUSTER)' already exists"; \
	else \
		echo "creating kind cluster '$(KIND_CLUSTER)' (rootless $(KIND_PROVIDER))..."; \
		$(KIND) create cluster --name $(KIND_CLUSTER); \
	fi

## k8s/cluster/down: delete the local kind cluster
.PHONY: k8s/cluster/down
k8s/cluster/down:
	@echo 'Deleting kind cluster $(KIND_CLUSTER)...'
	$(KIND) delete cluster --name $(KIND_CLUSTER)

## k8s/cluster/status: show kind cluster + node info
.PHONY: k8s/cluster/status
k8s/cluster/status:
	@if $(KIND) get clusters 2>/dev/null | grep -qx '$(KIND_CLUSTER)'; then \
		echo "kind cluster: $(KIND_CLUSTER)"; \
		kubectl --context kind-$(KIND_CLUSTER) get nodes -o wide; \
	else \
		echo "no kind cluster '$(KIND_CLUSTER)' (run 'make k8s/cluster/up')"; \
	fi

## k8s/dev-deps/up: deploy in-cluster postgres + redis for the smoke
.PHONY: k8s/dev-deps/up
k8s/dev-deps/up:
	@echo 'Ensuring namespace $(K8S_NAMESPACE)...'
	kubectl create namespace $(K8S_NAMESPACE) --dry-run=client -o yaml | kubectl apply -f -
	@echo 'Applying $(DEV_DEPS_MANIFEST)...'
	kubectl -n $(K8S_NAMESPACE) apply -f $(DEV_DEPS_MANIFEST)
	@echo 'Waiting for postgres + redis rollouts to complete...'
	kubectl -n $(K8S_NAMESPACE) rollout status deployment/postgres --timeout=180s
	kubectl -n $(K8S_NAMESPACE) rollout status deployment/redis --timeout=120s

## k8s/dev-deps/down: remove the in-cluster postgres + redis dev-deps
.PHONY: k8s/dev-deps/down
k8s/dev-deps/down:
	kubectl -n $(K8S_NAMESPACE) delete -f $(DEV_DEPS_MANIFEST) --ignore-not-found

## k8s/image/build: build optivest-api + optivest-migrate locally
.PHONY: k8s/image/build
k8s/image/build:
	@echo 'Building optivest-api:$(IMAGE_TAG)...'
	docker build -t optivest-api:$(IMAGE_TAG) -f Dockerfile .
	@echo 'Building optivest-migrate:$(IMAGE_TAG)...'
	docker build -t optivest-migrate:$(IMAGE_TAG) -f Dockerfile.migrate .

## k8s/image/load: side-load both images into the kind cluster
.PHONY: k8s/image/load
k8s/image/load:
	@echo 'Side-loading optivest-api:$(IMAGE_TAG) into kind...'
	$(KIND) load docker-image optivest-api:$(IMAGE_TAG) --name $(KIND_CLUSTER)
	@echo 'Side-loading optivest-migrate:$(IMAGE_TAG) into kind...'
	$(KIND) load docker-image optivest-migrate:$(IMAGE_TAG) --name $(KIND_CLUSTER)

## k8s/secret/create: create/refresh the optivest-secrets Secret from cmd/api/.env
##
## kubectl rejects mixing --from-env-file with --from-literal, so we
## merge them in shell first: drop comments + blank lines + any
## existing OPTIVEST_DB_DSN entry from the .env file (which typically
## points at host-side localhost), append the cluster-local DSN, then
## feed the merged result as a single --from-env-file. mktemp keeps
## the intermediate file out of the source tree, and the trap cleans
## it up even if kubectl errors.
.PHONY: k8s/secret/create
k8s/secret/create:
	@test -f $(ENV_FILE) || { echo "$(ENV_FILE) not found - copy cmd/api/.env.example and fill values"; exit 1; }
	kubectl create namespace $(K8S_NAMESPACE) --dry-run=client -o yaml | kubectl apply -f -
	@TMPENV=$$(mktemp) && trap 'rm -f $$TMPENV' EXIT INT TERM && \
		grep -E '^[A-Za-z_][A-Za-z0-9_]*=' $(ENV_FILE) | grep -v '^OPTIVEST_DB_DSN=' > $$TMPENV && \
		echo 'OPTIVEST_DB_DSN=$(K8S_DB_DSN)' >> $$TMPENV && \
		kubectl -n $(K8S_NAMESPACE) create secret generic optivest-secrets \
			--from-env-file=$$TMPENV \
			--dry-run=client -o yaml | kubectl apply -f -

## k8s/migrate: run goose up as a one-shot Job
##
## chart v0.1 doesn't manage migrations itself — this rebuilds the
## `migrations` ConfigMap from $(SCHEMA_DIR) (so a freshly-edited schema
## file shows up without rebuilding the migrate image), tears down any
## previous Job, and applies $(MIGRATE_JOB_MANIFEST). Chart v0.2 will
## replace this with a templated Job rendered as a Helm hook.
.PHONY: k8s/migrate
k8s/migrate:
	@echo 'Refreshing migrations ConfigMap from $(SCHEMA_DIR)/...'
	kubectl -n $(K8S_NAMESPACE) create configmap migrations \
		--from-file=$(SCHEMA_DIR)/ \
		--dry-run=client -o yaml | kubectl apply -f -
	@echo 'Replacing migrate Job...'
	kubectl -n $(K8S_NAMESPACE) delete job migrate --ignore-not-found
	kubectl -n $(K8S_NAMESPACE) apply -f $(MIGRATE_JOB_MANIFEST)
	@echo 'Waiting for migrate Job to complete...'
	kubectl -n $(K8S_NAMESPACE) wait --for=condition=complete job/migrate --timeout=180s
	@echo 'Migration logs:'
	kubectl -n $(K8S_NAMESPACE) logs job/migrate

## k8s/install: helm upgrade --install the chart against the kind cluster
.PHONY: k8s/install
k8s/install:
	helm upgrade --install $(CHART_RELEASE) $(CHART_PATH) \
		--namespace $(K8S_NAMESPACE) \
		--create-namespace \
		--set image.repository=$(CHART_IMAGE_REPO) \
		--set image.tag=$(IMAGE_TAG) \
		--set config.env=$(CHART_ENV) \
		--wait --timeout=180s

## k8s/uninstall: helm uninstall the chart
.PHONY: k8s/uninstall
k8s/uninstall:
	helm uninstall $(CHART_RELEASE) --namespace $(K8S_NAMESPACE) --ignore-not-found

## k8s/forward: port-forward API:4000 + SSE:4001 to localhost (Ctrl-C to stop)
.PHONY: k8s/forward
k8s/forward:
	@echo 'Forwarding $(CHART_RELEASE) API->localhost:4000, SSE->localhost:4001 (Ctrl-C to stop)'
	kubectl -n $(K8S_NAMESPACE) port-forward svc/$(CHART_RELEASE) 4000:4000 4001:4001

## k8s/logs: tail the optivest API pod logs
.PHONY: k8s/logs
k8s/logs:
	kubectl -n $(K8S_NAMESPACE) logs -f deployment/$(CHART_RELEASE)

## k8s/smoke: full bring-up of the chart against a fresh kind cluster
##
## Order matters here:
##   1. cluster up           — kubectl context exists
##   2. dev-deps/up          — postgres + redis are reachable inside the cluster
##   3. image/build + load   — kubelet can pull localhost/optivest-{api,migrate}
##   4. secret/create        — DSN + env keys are available to the migrate Job
##                              and the API pod via envFrom
##   5. migrate              — schema is applied before the API tries to read it
##   6. install              — chart rolls out and probes go green
.PHONY: k8s/smoke
k8s/smoke:
	@$(MAKE) k8s/cluster/up
	@$(MAKE) k8s/dev-deps/up
	@$(MAKE) k8s/image/build
	@$(MAKE) k8s/image/load
	@$(MAKE) k8s/secret/create
	@$(MAKE) k8s/migrate
	@$(MAKE) k8s/install
	@echo ''
	@echo '--- smoke complete --------------------------------------------------'
	@echo 'tail logs:           make k8s/logs'
	@echo 'forward ports:       make k8s/forward'
	@echo 'helm test:           helm test $(CHART_RELEASE) -n $(K8S_NAMESPACE)'
	@echo 'tear down chart:     make k8s/uninstall'
	@echo 'tear down deps:      make k8s/dev-deps/down'
	@echo 'tear down cluster:   make k8s/cluster/down'