<p align="center">
  <a href="" rel="noopener">
 <img width=200px height=190px src="https://i.ibb.co/hZdMWvh/optivest-cropped.png" alt="Project logo"></a>
</p>

<h3 align="center">OptiVest </h3>

<div align="center">

[![CI](https://github.com/Blue-Davinci/OptiVest/actions/workflows/ci.yml/badge.svg?branch=main)](https://github.com/Blue-Davinci/OptiVest/actions/workflows/ci.yml)
[![Go Report Card](https://goreportcard.com/badge/github.com/Blue-Davinci/OptiVest)](https://goreportcard.com/report/github.com/Blue-Davinci/OptiVest)
[![Go Version](https://img.shields.io/badge/go-1.26-00ADD8.svg?logo=go&logoColor=white)](https://go.dev/doc/go1.26)
[![Status](https://img.shields.io/badge/status-active-success.svg)]()
[![GitHub Issues](https://img.shields.io/github/issues/Blue-Davinci/OptiVest.svg)](https://github.com/Blue-Davinci/OptiVest/issues)
[![GitHub Pull Requests](https://img.shields.io/github/issues-pr/Blue-Davinci/OptiVest.svg)](https://github.com/Blue-Davinci/OptiVest/pulls)
[![License](https://img.shields.io/badge/license-MIT-blue.svg)](/LICENSE)

</div>

---

<p align="center"> Optivest <b>[Back-End]: </b> This is the backend sister project of OptiVest Project. To Check out the Frontend, go <a href="https://github.com/Blue-Davinci/OptiVest-Frontend">here</a>
</p>

<hr /> 

## 📝 Table of Contents

- [About](#about)
- [Features](#features)
- [Getting Started](#getting_started)
- [Endpoints](#endpoints)
- [Deployment](#deployment)
- [Usage](#usage)
- [Built Using](#built_using)
- [TODO](./TODO.md)
- [Authors](#authors)
- [Acknowledgments](#acknowledgement)

## 🧐 About <a name = "about"></a>

The OptiVest project is a cutting-edge, AI-driven personal financial advisor platform designed to empower users with smarter financial management tools. Built with a focus on automation and real-time data insights, OptiVest combines dynamic portfolio analysis, personalized investment recommendations, and a suite of tools for budgeting, goal setting, and debt tracking. The backend, developed in Go, integrates financial data from sources like Alpha Vantage and FRED to provide up-to-date insights and robust portfolio optimization.

A standout feature of OptiVest is its focus on actionable financial insights. Users receive real-time portfolio alerts, performance metrics, and risk management tips, helping them make well-informed decisions. The platform’s intelligent algorithms highlight top-performing assets and assist in sector diversification, while additional tools for budgeting and debt tracking offer a holistic approach to personal finance. By merging AI-driven recommendations with user-centric design, OptiVest delivers an all-in-one financial advisory experience tailored to individual financial goals and preferences.

## ✨ Features <a name="features"></a>
1. **AI-Driven Financial Insights**
- Provides intelligent financial advice using pre-trained AI models, enabling users to make data-backed investment decisions.
- Customizable recommendations on portfolio rebalancing, risk management, and asset allocation.
2. **Real-Time Portfolio Analysis**
- Integrates with Alpha Vantage and FRED for up-to-date data, delivering real-time analysis of investments, market trends, and external factors like interest rates and market sentiment.
- Calculates key performance metrics such as ROI, Sharpe ratio, and sector performance.
3. **Automated Portfolio Management**
- Supports automated portfolio rebalancing based on individual risk tolerance and investment goals.
- Uses advanced algorithms to identify top-performing stocks and bonds, updating recommendations regularly.
4. **Personal Finance Tools**
- Budgeting and Goal Setting: Tracks spending, monitors goals, and provides summaries for financial planning.
- Debt Management: Analyzes debt information, including payment history, interest rates, and payoff estimates, and visualizes debt progress.
5. **Notification Center**
- Real-time notifications for market updates, investment alerts, and goal progress. **In Progress**
- Allows users to view messages with detailed metadata, including links and images, for quick navigation.
6. **Advanced Security and Integration**
- Secure WebSocket connection for real-time updates and data handling.
- Implements Redis caching for efficient data retrieval, reducing load on API calls and improving performance.
7. **Prediction Capability**
- Based on your spending, expense, income and debt rates, OptiVest is able to come up with predictions of future habits
using the OptiVest Predictor Micr-Service.

## 🏁 Getting Started <a name = "getting_started"></a>

These instructions will get you a copy of the project up and running on your local machine for development and testing purposes. See [deployment](#deployment) for notes on how to deploy the project on a live system.

### Prerequisites

Before you can run or contribute to this project, you'll need to have the following software installed:

- [Go](https://golang.org/dl/): The project is written in Go, so you'll need to have Go installed to run or modify the code.
- [PostgreSQL](https://www.postgresql.org/download/): The project uses a PostgreSQL database, so you'll need to have PostgreSQL installed and know how to create a database.
- A Go IDE or text editor: While not strictly necessary, a Go IDE or a text editor with Go support can make it easier to work with the code. I use vscode.
- [Git](https://git-scm.com/downloads): You'll need Git to clone the repo.
- [Redis](https://redis.io/): OptiVest uses Redis for caching to enhance performance and reduce API load.
- [OptiVest-Predictor-Microservice](https://github.com/Blue-Davinci/OptiVest_Finance_Predictor_Micro_Service_V1): Clone and set up this micro-service, which is esential for financial predictions and recommendations


### Installing

A step by step series of examples that tell you how to get a development env running.

1. **Clone the repository:** Start by cloning the repository to your local machine. Open a terminal, navigate to the directory where you want to clone the repository, and run the following command:
    ```bash
    git clone https://github.com/Blue-Davinci/OptiVest.git
    ```
2. **Navigate to the project directory:** Use the `cd` command to navigate to the project directory:

    ```bash
    cd optivest
    ```
3. **Install the Go dependencies:** The Go tools will automatically download and install the dependencies listed in the `go.mod` file when you build or run the project. To download the dependencies without building or running the project, you can use the `go mod download` command:

    ```bash
    go mod download
    ```
4. **Set up the database:** The project uses a PostgreSQL database. You'll need to create a new database and update the connection string in your configuration file or environment variables.
We use `GOOSE` for all the data migrations and `SQLC` as the abstraction layer for the DB. To proceed
with the migration, navigate to the `Schema` director:
```bash
cd internal\sql\schema
```
- Then proceed by using the `goose {connection string} up` to execute an <b>Up migration</b> as shown:
- <b>Note:</b> You can use your own environment variable or load it from the env file.

```bash
goose postgres postgres://aggregate:password@localhost/aggregate  up
```

> **Regenerating the SQLC layer.** `sqlc` is tracked as a Go tool dependency
> in `go.mod`, so the version is pinned alongside the rest of the toolchain
> and Dependabot's `gomod` ecosystem will surface upgrade PRs against it.
> After editing anything under `internal/sql/queries/` or the schema, run:
>
> ```bash
> make generate          # regenerates internal/database/*.sql.go
> make verify-generate   # fails locally if you forgot to commit the regen
> ```
>
> CI runs the same `verify-generate` check as a dedicated job, so a PR
> that ships a query change without the regenerated Go won't merge. There
> is intentionally no `go install sqlc@<floating>` fallback — pinning via
> the `tool` directive is what kept the previous regen-version drift from
> repeating.

5. **Download and Setup the MIcroService:** Follow the instructions highlighted [here](https://github.com/Blue-Davinci/OptiVest_Finance_Predictor_Micro_Service_V1) to get the micro-service up and running.

6. **Environment variables:** Copy the committed template into the location
   the API loads at boot, then fill in real values:

   ```bash
   cp cmd/api/.env.example cmd/api/.env
   # then edit cmd/api/.env
   ```

   `cmd/api/.env.example` documents every variable, marks each one as
   `[REQUIRED]` or `[OPTIONAL]`, calls out which ones are tolerated as
   warnings in `-env=development` and fatal in non-development, and
   includes a one-liner for generating the AES encryption key with
   `openssl rand -hex 32`. The actual `cmd/api/.env` you create is
   gitignored, so real secrets never enter the repo.

   `godotenv.Load` runs against `cmd/api/.env` (or `.env` if your CWD is
   already `cmd/api/`); environment variables exported in your shell take
   precedence over the file, so per-shell overrides work without editing
   it.

5. **Build the project:** You can build the project using the makefile's command:

    ```bash
    make build/api
    ```
    This will create an executable file in the current directory.
    <b>Note: The generated executable is for the windows environment</b>
      <b>- However, You can find the linux build command within the makefile!</b>

6. **Run the project:** You can run the project using the `go run` or use <b>`MakeFile`</b> and do:

    ```bash
    make run/api
    ```

7. **MakeFile Help:** For additional supported commands run `make help`:

  ```makefile
  make help
  ```
**output:**
```bash
Usage:
run/api: Run the API server
run/api/origins: Run the API server with CORS origins
db/psql            -  connect to the db using psql
build/api          -  build the cmd/api application
```

### Local stack with Docker / Podman

If you don't want to install Postgres + Redis on your host, the repo ships
a `docker-compose.yml` that brings up the entire stack (Postgres 17, Redis 7,
a one-shot `goose` migration runner, and the API itself) with a single
command. The same compose file works on Docker Desktop and on rootless
Podman with the docker-compatibility shim.

Prerequisites:
- A container runtime (Docker Engine, Docker Desktop, or rootless Podman 5+)
- Two populated `.env` files (templates committed, real files gitignored):
  - `cmd/api/.env` for the API binary's runtime config (smtp, vendor API
    keys, encryption key, etc) — copy `cmd/api/.env.example`
  - `/.env` at the repo root for the docker stack's database credentials
    (`POSTGRES_USER` / `POSTGRES_PASSWORD` / `POSTGRES_DB`) — copy
    `/.env.example`

The two files are deliberately separate: `cmd/api/.env` is loaded by
the binary via `godotenv`, `/.env` is loaded by `docker compose` itself
for `${VAR}` interpolation in the compose file. Compose `${VAR:?...}`
syntax fails fast if anything in `/.env` is unset, so a missing variable
shows up as a clear error at `make docker/up` time rather than as a
"password authentication failed" deep in a migration.

Bring everything up:

```bash
cp .env.example .env                     # first time only — set POSTGRES_PASSWORD
cp cmd/api/.env.example cmd/api/.env     # first time only — fill the keys you have
make docker/up
```

That builds the API image (multi-stage: `golang:1.26-alpine` builder →
`alpine:3.23` runtime, non-root `optivest` user, static binary), pulls
`postgres:17-alpine` + `redis:7-alpine`, runs every migration in
`internal/sql/schema/` against the fresh database, and then starts the
API. The API container is wired to a `HEALTHCHECK` that polls
`/healthcheck` every 30s, so `docker compose ps` reflects healthy /
unhealthy state. The first run takes 1-3 minutes (image pulls + Go module
download); subsequent runs are layer-cached and start in seconds.

Once it's up, the same observability surface that's exposed locally is
available from the host:

```bash
curl http://localhost:4000/healthcheck     # JSON liveness payload
curl http://localhost:4000/readyz          # JSON readiness payload (postgres + redis)
curl http://localhost:4000/metrics         # Prometheus exposition
curl http://localhost:4000/debug/vars      # raw JSON expvars
psql "$OPTIVEST_DB_DSN"                    # interactive DB shell
```

Other useful targets:

| Command                  | Effect                                                              |
|--------------------------|---------------------------------------------------------------------|
| `make docker/logs`       | tail just the API container logs                                    |
| `make docker/ps`         | list the running compose services                                   |
| `make docker/migrate`    | re-run `goose up` against the running Postgres                      |
| `make docker/build`      | rebuild the API image without restarting the stack                  |
| `make docker/down`       | stop and remove containers, **preserving** the Postgres data volume |
| `make docker/down/clean` | same as above but **drops** the Postgres volume too                 |

How the API in the container finds its config: docker-compose loads
`cmd/api/.env` via `env_file`, then overrides the DSN host (`localhost`
→ `postgres`) and the Redis password (empty in dev) via an explicit
`environment:` block on the api service. The DSN itself is built from
the `${POSTGRES_USER}` / `${POSTGRES_PASSWORD}` / `${POSTGRES_DB}` values
that compose interpolates from the root `/.env`, so there is exactly one
source of truth for the database credential. The binary itself was
already tolerant of a missing `.env` file at runtime (it falls back to
the process environment), which is what makes the same image work in
cluster deployments where config comes from a Kubernetes Secret rather
than a file. CLI flags overriding `-redis-addr=redis:6379` are passed
via the service `command:` so the binary connects to the in-network
service name rather than the localhost default.

### Description

The application accepts command-line flags for configuration, establishes a connection pool to a database, and publishes variables for monitoring the application. The published variables include the application version, the number of active goroutines and the current Unix timestamp.
  - This will start the application. You should be able to access it at `http://localhost:4000`.

<hr />
You can view the **parameters** by utilizing the `-help` command. Here is a rundown of 
the available commands for a quick lookup (INCOMPLETE, use help for full list).

> **Security note:** All `-api-key-*` and `-encryption-key` flags default to
> the corresponding environment variable. Never commit real keys here, and
> never paste them into the example output below. See
> [`SECURITY.md`](./SECURITY.md) for the rotation runbook and the list of
> required env vars (`OPTIVEST_*`). In non-development environments the API
> refuses to start if any required secret is missing.

```bash
- api-author string
        API author (default "Blue_Davinci")
  -api-default-currency string
        Default currency (default "USD")
  -api-key-alphavantage string
        Alpha Vantage API key (env OPTIVEST_ALPHAVANTAGE_API_KEY)
  -api-key-exchangerates string
        Exchange-Rate API key (env OPTIVEST_EXCHANGERATE_API_KEY)
  -api-key-fmp string
        FMP API key (env OPTIVEST_FINANCIALMODELINGPREP_API_KEY)
  -api-key-fred string
        FRED API key (env OPTIVEST_FRED_API_KEY)
  -api-key-ocrspace string
        OCR.Space API key (env OPTIVEST_OCRSPACE_API_KEY)
  -api-key-optivestmicroservice string
        OptiVest predictor microservice API key (env OPTIVEST_PREDICTOR_API_KEY)
  -api-key-sambanova string
        SambaNova API key (env OPTIVEST_SAMBA_NOVA_LLM_API_KEY)
  -api-name string
        API name (default "OptiVest")
  -api-url-alphavantage string
        Alpha Vantage API URL (default "https://www.alphavantage.co/query?")
  -api-url-exchangerates string
        Exchange-Rate API URL (default "https://v6.exchangerate-api.com/v6")
  -api-url-fmp string
        FMP API base URL (the legacy /api/v3 family was retired 2025-08-31) (default "https://financialmodelingprep.com/stable")
  -api-url-fred string
        FRED API URL (default "https://api.stlouisfed.org/fred/series/observations?")
  -api-url-ocrspace string
        OCR.Space API URL (default "https://api.ocr.space/parse/image")
  -api-url-optivestmicroservice string
        OptiVest predictor microservice URL (default "http://127.0.0.1:8000/v1/predict")
  -api-url-sambanova string
        SambaNova (or OpenAI-compatible) chat-completions URL (default "https://api.sambanova.ai/v1/chat/completions")
  -cors-trusted-origins value
        Trusted CORS origins (space separated)
  -db-dsn string
        PostgreSQL DSN (env OPTIVEST_DB_DSN)
  -db-max-idle-conns int
        PostgreSQL max idle connections (default 25)
  -db-max-idle-time string
        PostgreSQL max connection idle time (default "15m")
  -db-max-open-conns int
        PostgreSQL max open connections (default 25)
  -encryption-key string
        AES-GCM encryption key, hex-encoded, 16/24/32 bytes (env OPTIVEST_DATA_ENCRYPTION_KEY)
  -env string
        Environment (development|staging|production) (default "development")
  -expired-notification-burst-limit int
        Batch Limit for Expired Notification Tracker (default 100)
  -frontend-activation-url string
        Frontend Activation URL (default "http://localhost:5173/verify?token=")
  -frontend-callback-url string
        Frontend Callback URL (default "https://adapted-healthy-monitor.ngrok-free.app/v1")
  -frontend-login-url string
        Frontend Login URL (default "http://localhost:5173/login")
  -frontend-password-reset-url string
        Frontend Password Reset URL (default "http://localhost:5173/passwordreset/password?token=")
  -frontend-url string
        Frontend URL (default "http://localhost:5173")
  -http-client-retrymax int
        HTTP client maximum retries (default 3)
  -http-client-timeout duration
        HTTP client timeout (default 10s)
  -limiter-burst int
        Rate limiter maximum burst (default 10)
  -limiter-enabled
        Enable rate limiter (default true)
  -limiter-rps float
        Rate limiter maximum requests per second (default 5)
  -llm-idle-timeout duration
        Abort the LLM stream if no chunk arrives within this window (default 15s)
  -llm-max-retries int
        Max retries for the LLM call before the first SSE chunk; mid-stream errors never retry (default 2)
  -llm-total-budget duration
        Wallclock cap for an LLM streaming call across retries (default 1m30s)
  -portfolio-worker-limit int
        Max concurrent per-asset workers in portfolio analysis (1 = serial; tune to upstream API rate limits) (default 6)
```

> **Rate limiter note:** As of P2, the limiter is backed by Redis using
> [`go-redis/redis_rate/v10`](https://github.com/go-redis/redis_rate) so the
> configured `-limiter-rps` and `-limiter-burst` apply **across the cluster**,
> not per-instance. The previous in-memory implementation under-counted by a
> factor of N when running N pods behind a load balancer.
>
> If Redis is unreachable the limiter **fails open** (allows the request
> through, increments `rate_limiter_redis_errors_total` and
> `rate_limiter_fail_open_total` on `/debug/vars`, and logs a WARN).
> Operators should alert on a non-zero `rate_limiter_redis_errors_total`
> rate. Successful and denied requests are counted separately as
> `rate_limiter_allowed_total` and `rate_limiter_denied_total`.
>
> Sub-1-rps configurations are not supported by the underlying GCRA bucket
> type; values <1 are rounded up to 1 and a warning is logged at startup.

> **Portfolio analysis concurrency (P3):** `performInvestmentPortfolioAnalysis`
> now fans out per-asset workers (stocks + bonds) into a bounded
> [`errgroup`](https://pkg.go.dev/golang.org/x/sync/errgroup) capped at
> `-portfolio-worker-limit` (default **6**). For a typical 10-15 asset
> portfolio this drops cache-cold analysis from ~25-30s to ~3-5s on Premium
> Alpha Vantage tiers. First-error-cancels semantics + the P2 ctx
> propagation work mean a client disconnect aborts every in-flight
> upstream HTTP call and DB INSERT.
>
> Tune to your vendor plan: Alpha Vantage Free (5 req/min) -> set to **1**;
> Premium (75/min) -> default **6**; Premium Plus (600/min) -> up to **16+**.
>
> A process-wide [`singleflight`](https://pkg.go.dev/golang.org/x/sync/singleflight)
> registry collapses concurrent identical upstream calls (FMP sector
> snapshot, per-symbol Alpha Vantage / FRED fetches), so even at higher
> worker limits we never N-multiply duplicate fetches when the cache is cold.
>
> Operational metrics on `/debug/vars`:
> - `portfolio_analysis_runs_total` (counter, request rate)
> - `portfolio_analysis_errors_total` (counter, alert on ratio)
> - `portfolio_analysis_duration_ms` (last run, scrape for p99)
> - `portfolio_analysis_workers_active` (live in-flight, saturation)
> - `portfolio_analysis_workers_max_observed` (process-wide high-water mark)
> - `portfolio_singleflight_collapsed_total` (counter; sustained zero is
>   normal if your cache is warm or workload is sequential)

> **Structured request logging (P3.B):** Every HTTP request emits exactly
> one structured log line through `zap`, after the response is fully
> written. The middleware sits OUTSIDE `recoverPanic` so a panic still
> produces a 500-status log line (the one ops should be paged on); 401s
> from `authenticate` and 429s from the rate limiter also each get a
> single line. Long-lived SSE connections deliberately skip per-request
> logging to avoid misleading "completed" lines on disconnect.
>
> Every request is also stamped with a correlation ID. Inbound
> `X-Request-ID` headers are accepted (cap 128 chars, allowlisted to
> `[A-Za-z0-9._-]` to block log injection); anything else is regenerated
> server-side as 16 hex chars from `crypto/rand`. The chosen ID is echoed
> back in the response header so clients can include it in bug reports.
>
> Per-request log fields (`msg="http request"`):
> - `method`, `path` (no query string; tokens often live there)
> - `status`, `bytes`, `latency_ms`
> - `req_id` - request correlation ID
> - `conn_id` - per-connection ID stamped by `ConnContext` (P1)
> - `user_id` - `0` for unauthenticated, else `user.ID` populated by
>   `authenticate` via the `requestLog` holder
> - `remote_ip` (via `tomasen/realip`, same source the limiter uses)
> - `user_agent` (truncated at 256 bytes)
>
> Example line:
> ```json
> {"level":"info","msg":"http request","method":"GET","path":"/v1/investments/analysis","status":200,"bytes":4821,"latency_ms":1843,"remote_ip":"203.0.113.7","req_id":"a1b2c3d4e5f60718","conn_id":42,"user_id":7,"user_agent":"OptiVest-Web/2.1"}
> ```
>
> Handlers that want their own log lines to participate in the same
> correlation should call `app.loggerFromRequest(r)` instead of using
> `app.logger` directly: the returned logger is pre-enriched with
> `req_id`, `conn_id`, and `user_id`. Background helpers that receive a
> `context.Context` but no `*http.Request` (the financial helpers under
> `investment_operations.go` are the canonical example) call
> `app.loggerFromContext(ctx)` for the same enrichment - the ctx must
> have flowed from the originating request, otherwise the correlation
> fields fall back to their zero values.
>
> Operational metrics on `/debug/vars`:
> - `request_log_total` - total requests observed
> - `request_log_4xx_total` - subset that returned a client error
> - `request_log_5xx_total` - subset that returned a server error (alert)
> - `request_id_generated_total` - inbound requests without an ID header
> - `request_id_rejected_total` - inbound IDs replaced for failing the
>   sanitization policy (sustained non-zero suggests a misbehaving caller
>   or active log-injection attempt)
>
> Outbound calls (Alpha Vantage, FRED, FMP, OCR.Space, the predictor
> micro-service, SambaNova, RSS scraping) are wrapped through
> `cmd/api/http_clients.go`. Each call:
> - is bound to the caller's `context.Context`, so a client disconnect
>   or timeout cancels the upstream request promptly;
> - forwards the inbound `X-Request-ID` on the outbound when it is set
>   on the ctx (and never clobbers a header the caller already chose);
> - emits one structured `http outbound` log line per call carrying
>   `method`, `host`, `path`, `status`, `bytes`, `latency_ms`, `req_id`,
>   `conn_id`, `user_id`. The full URL is deliberately NOT logged because
>   several upstream providers carry their API key in the query string;
>   see `SECURITY.md` for the rotation runbook if a leak occurs.
>
> Log levels mirror the inbound logger: `5xx` and transport errors are
> `Error` (page on this), `ctx` cancel/deadline is `Info` (expected
> client behaviour, not an upstream fault), everything else is `Info`.

> **Prometheus `/metrics` endpoint:** Every operational signal we already
> publish to `/debug/vars` is also exposed in [Prometheus text exposition
> format](https://prometheus.io/docs/instrumenting/exposition_formats/) at
> `GET /metrics`. The handler walks the existing `expvar` registry on each
> scrape and emits a curated, allow-listed subset — runtime noise such as
> `cmdline` and `memstats` is deliberately suppressed because neither
> flattens cleanly into the Prom data model and Prom servers ship their own
> Go runtime exporters.
>
> Currently exposed metrics (counters unless noted):
> - `request_log_total`, `request_log_4xx_total`, `request_log_5xx_total`
> - `request_id_generated_total`, `request_id_rejected_total`
> - `rate_limiter_allowed_total`, `rate_limiter_denied_total`,
>   `rate_limiter_redis_errors_total`, `rate_limiter_fail_open_total`,
>   `rate_limiter_disabled_total`
> - `portfolio_analysis_runs_total`, `portfolio_analysis_errors_total`,
>   `portfolio_singleflight_collapsed_total`
> - `portfolio_analysis_duration_ms` (gauge)
> - `portfolio_analysis_workers_active`,
>   `portfolio_analysis_workers_max_observed` (gauges)
> - `total_requests_received`, `total_responses_sent`,
>   `total_processing_time_us` (the underlying expvar uses the non-ASCII
>   `μs` suffix, which Prom's metric-name regex rejects; the exposition
>   renames it to `_us` while leaving `/debug/vars` untouched)
> - `total_responses_sent_by_status` with a `code="<status>"` label per
>   bucket
> - `goroutines`, `timestamp` (gauges)
> - `optivest_build_info{version="..."} 1` — canonical Prom info-pattern
>   for build identity
>
> The `/metrics`, `/debug/vars`, `/healthcheck`, and `/readyz` endpoints
> are all mounted on the **base router**, deliberately outside the global
> middleware chain that wraps `/v1`. Probe and scrape traffic therefore
> does not flow through `logRequests` (no log-line-per-scrape noise), is
> not counted by the `metrics` middleware (no circular self-incrementing
> of the very counters being read), and is not subject to the per-IP
> rate limiter (a fast or misconfigured scraper cannot lock itself out
> of the diagnosis endpoint, and a load balancer probing `/readyz` more
> aggressively during a degradation cannot be told the instance is
> unready by the limiter). All four endpoints are unauthenticated; the
> deployment is expected to scope reachability to the internal scrape /
> probe network — see `SECURITY.md` for the operator-side assumptions.
>
> Adding a new metric to the `/metrics` surface is one line in
> `cmd/api/metrics.go` (the `knownMetrics` allow-list); the friction is
> intentional, since this is an operator-facing surface, not a debug dump.

> **SambaNova streaming + retry budget (P4.A):** The SambaNova chat-completions
> path is special-cased away from the generic `Optivet_Client` because every
> call holds a connection slot open for 10–30s while the model emits SSE
> chunks. `LLMStream` in `cmd/api/http_clients.go` provides three guarantees
> the generic path cannot:
>
> - A **wallclock budget context** (`-llm-total-budget`, default `90s`) caps
>   the entire call across pre-first-byte retries plus the streaming read.
>   The retry layer's backoff sleeps inherit this deadline, so a tight
>   budget naturally compresses the effective retry window.
> - An **idle deadline** (`-llm-idle-timeout`, default `15s`) aborts the
>   stream when no chunk has arrived within the window. Protects connection
>   slots from a stalled upstream that has sent headers but no body, and
>   caps the impact of a slowloris-style upstream.
> - **Bounded pre-first-byte retries** (`-llm-max-retries`, default `2`).
>   Retries fire only on dial / TLS / non-2xx errors that happen *before*
>   the response body is streamed. Once a chunk has been read, mid-stream
>   errors return verbatim — replaying a 30s prompt for a transient blip
>   would double user-visible latency and the token bill, with no
>   reliability gain because SambaNova's API is not resumable.
>
> `LLMRequest` is preserved as a thin back-compat wrapper for callers that
> only need the joined string (`buildLLMRequestHelper` and friends in
> `cmd/api/ai_operations.go`).

> **Streaming portfolio analysis (P4.B):**
> `GET /v1/investments/analysis/stream` is the user-facing payoff for the
> streaming client. It runs the same DB pre-checks as the synchronous
> `/v1/investments/analysis` endpoint (goals + non-empty portfolio), and
> only switches into `text/event-stream` mode once those 4xx-eligible
> validations pass — so client-side error handling for "no goals set"
> stays a normal JSON envelope, not a malformed SSE frame.
>
> Wire format (all events are JSON-encoded so the client always parses
> `JSON.parse(event.data)`, regardless of which event name fires):
>
> ```text
> data: {"delta": "<token text>"}             // one per LLM chunk, default event
> event: done
> data: {"finish_reason":"stop","usage":{...},"analyzed":{...}}
>
> event: error
> data: {"error":"upstream stalled"}          // terminal failure
> ```
>
> The handler reuses one prompt template across the streaming and
> non-streaming paths via `renderInvestmentPortfolioPrompt` in
> `ai_operations.go`, so a user gets the same analysis regardless of
> which endpoint they hit. On stream completion the analyzed portfolio
> is persisted to the same table read by `/v1/investments/analysis/summary`;
> a persist failure logs but does not abort the user's stream, since
> the result is already in the browser by then. Mid-stream errors are
> reported via a single `event: error` event because by that point the
> response status code is already on the wire and unchangeable.
>
> A single-file browser smoke-test client lives at
> [`examples/sse-portfolio-analysis/`](examples/sse-portfolio-analysis/).
> It uses `fetch()` + a manual SSE parser (rather than `EventSource`,
> which cannot send `Authorization` headers) and renders deltas live
> against a running API. Useful for eyeballing CORS, auth, and flushing
> end-to-end without spinning up the full frontend.

Using `make run`, will run the API with a default connection string located 
in `cmd\api\.env`. If you're using `powershell`, you need to load the values otherwise you will get
a `cannot load env file` error. Use the PS code below to load it or change the env variable:

```powershell
$env:OPTIVEST_DB_DSN=(Get-Content -Path .\cmd\api\.env | Where-Object { $_ -match "OPTIVEST_DB_DSN" } | ForEach-Object { $($_.Split("=", 2)[1]) })
```

Alternatively, in unix systems you can make a .envrc file and load it directly in the makefile by importing like so:
```makefile
include .envrc
```

A succesful run will output something like this:

```bash
make run/api
Running API server..
go run ./cmd/api
{"level":"info","time":"2024-1","caller":"api/main.go:374","msg":"Loading Environment Variables","path":"cmd\\api\\.env"}
{"level":"info","time":"2024-1]","caller":"api/main.go:268","msg":"Redis connection established","addr":"localhost:6379"}
{"level":"info","time":"2024-1","caller":"api/main.go:277","msg":"database connection pool established","dsn":"postgres://optivest:yourpassword@localhost/optivest?sslmode=disable"}
{"level":"info","time":"2024-1","caller":"api/helpers.go:215","msg":"currency certified, using cached currencies","currency":"USD"}
{"level":"info","time":"2024-1-2","caller":"api/main.go:337","msg":"Starting Schedulers"}
{"level":"info","time":"2024-0-2","caller":"api/schedulers.go:64","msg":"Starting the recurring expenses tracking cron job..","time":"2024-00-2"}
{"level":"info","time":"2024-1","caller":"api/schedulers.go:199","msg":"Tracking recurring expenses","time":"2024-1 EAT m=+0.533360001"}
```

<hr />

### API Endpoints <a name="endpoints"></a>

The API surface is split across an unauthenticated **operational** plane (mounted on the base router so probes bypass auth and rate limiting) and an authenticated **`/v1`** application plane that runs through the full middleware chain (request ID, structured logging, panic recovery, rate limiting, bearer-token auth).

Auth tokens are obtained via `POST /v1/api/authentication` and sent as `Authorization: Bearer <token>` on every protected route. Every response carries an `X-Request-ID` header, propagated through outbound calls for end-to-end tracing.

#### Operational (unauthenticated)

| Method | Path | Purpose |
|---|---|---|
| GET | `/healthcheck` | Liveness probe; returns `{status, version, env, uptime_sec}`. Always 200 unless the process itself is unable to answer. Wire to k8s `livenessProbe` or Docker `HEALTHCHECK`. Failure causes a container restart. |
| GET | `/readyz` | Readiness probe; pings Postgres and Redis in parallel, returns `200 {status: "ready", checks: {...}}` when both are reachable, `503 {status: "not_ready", checks: {...}}` when any required dep is down. Wire to k8s `readinessProbe` or load-balancer health checks. Failure pulls the instance out of rotation without restarting it. |
| GET | `/metrics` | Prometheus text exposition of the curated metric set |
| GET | `/debug/vars` | Raw `expvar` JSON dump for ad-hoc inspection |

#### Resource map (authenticated, all under `/v1`)

| Group | Mount | Endpoints | Auth required |
|---|---|---|---|
| Users & MFA | `/users` | 9 | Mixed (registration is open) |
| API tokens | `/api` | 5 | Open (login flow) |
| Budgets | `/budgets` | 5 | Yes |
| Goals | `/goals` | 7 | Yes |
| Groups | `/groups` | 19 | Yes |
| Incomes | `/incomes` | 3 | Yes |
| Debts | `/debts` | 5 | Yes |
| Expenses | `/expenses` | 7 | Yes |
| Investments | `/investments` | 14 | Yes |
| Personal finance | `/personalfinance` | 4 | Yes |
| Feeds (RSS) | `/feeds` | 7 | Yes |
| Awards | `/awards` | 1 | Yes |
| Search options | `/search-options` | 3 | Yes |
| Notifications | `/notifications` | 4 | Yes |
| Comments & reactions | `/comments` | 6 | Yes |
| Contact | `/contact-us` | 1 | Yes |
| Server-Sent Events | `/sse` | 1 | Yes |

<details>
<summary><b>Users &amp; MFA</b> &mdash; <code>/v1/users</code></summary>

| Method | Path | Description |
|---|---|---|
| POST   | `/v1/users` | Register a new user |
| PUT    | `/v1/users/activated` | Activate a newly-registered account |
| PUT    | `/v1/users/password` | Reset password using a recovery token |
| POST   | `/v1/users/recovery` | Validate a recovery code |
| POST   | `/v1/users/mfa` | Enroll in MFA (TOTP) |
| POST   | `/v1/users/mfa/verify` | Verify the MFA enrollment OTP |
| GET    | `/v1/users/account` | Read the authenticated user's profile |
| PATCH  | `/v1/users/account` | Update the authenticated user's profile |
| POST   | `/v1/users/logout` | Terminate the current session / SSE channel |

</details>

<details>
<summary><b>API tokens (login flow)</b> &mdash; <code>/v1/api</code></summary>

| Method | Path | Description |
|---|---|---|
| POST | `/v1/api/authentication` | Issue an authentication token (login) |
| POST | `/v1/api/authentication/verify` | Validate the MFA challenge after login |
| POST | `/v1/api/password-reset` | Send a password-reset token by email |
| POST | `/v1/api/recovery` | Initiate account recovery via recovery codes |
| POST | `/v1/api/resend-activation` | Re-send an account activation token |

</details>

<details>
<summary><b>Budgets</b> &mdash; <code>/v1/budgets</code></summary>

| Method | Path | Description |
|---|---|---|
| GET    | `/v1/budgets` | List all budgets for the authenticated user |
| GET    | `/v1/budgets/summary` | Aggregate budget / goal / expense summary |
| POST   | `/v1/budgets` | Create a budget |
| PATCH  | `/v1/budgets/{budgetID}` | Update a budget |
| DELETE | `/v1/budgets/{budgetID}` | Delete a budget |

</details>

<details>
<summary><b>Goals &amp; goal plans</b> &mdash; <code>/v1/goals</code></summary>

| Method | Path | Description |
|---|---|---|
| POST  | `/v1/goals` | Create a goal |
| PATCH | `/v1/goals/{goalID}` | Update a goal |
| GET   | `/v1/goals/progression` | Goals enriched with progression % |
| GET   | `/v1/goals/tracking` | Snapshot history of goal progression |
| POST  | `/v1/goals/plan` | Create a goal plan |
| PATCH | `/v1/goals/plan/{goalPlanID}` | Update a goal plan |
| GET   | `/v1/goals/plan` | List the user's goal plans |

</details>

<details>
<summary><b>Groups, invitations, group goals, group transactions, group expenses</b> &mdash; <code>/v1/groups</code></summary>

| Method | Path | Description |
|---|---|---|
| GET    | `/v1/groups` | Groups the user is a member of |
| GET    | `/v1/groups/{groupID}` | Detailed view of one group |
| POST   | `/v1/groups` | Create a new group |
| PATCH  | `/v1/groups/{groupID}` | Update a group |
| PATCH  | `/v1/groups/member/{groupID}` | Update a member's role |
| DELETE | `/v1/groups/member/{groupID}/{memberID}` | Admin removes a member |
| DELETE | `/v1/groups/member/{groupID}` | User leaves the group |
| GET    | `/v1/groups/created` | Groups created by the authenticated user |
| POST   | `/v1/groups/invite` | Send a group invitation |
| PATCH  | `/v1/groups/invite/{groupID}` | Accept / decline an invitation |
| POST   | `/v1/groups/goal` | Create a group goal |
| PATCH  | `/v1/groups/goal/{groupGoalID}` | Update a group goal |
| GET    | `/v1/groups/transactions/{groupID}` | List transactions for a group |
| POST   | `/v1/groups/transactions` | Create a group transaction |
| DELETE | `/v1/groups/transactions/{groupTransactionID}` | Delete a group transaction |
| GET    | `/v1/groups/expenses/{groupID}` | List expenses for a group |
| POST   | `/v1/groups/expenses` | Create a group expense |
| DELETE | `/v1/groups/expenses/{groupExpenseID}` | Delete a group expense |
| GET    | `/v1/groups/public` | Browse public groups |
| POST   | `/v1/groups/public` | Join a public group |

</details>

<details>
<summary><b>Incomes</b> &mdash; <code>/v1/incomes</code></summary>

| Method | Path | Description |
|---|---|---|
| GET   | `/v1/incomes` | List all incomes for the authenticated user |
| POST  | `/v1/incomes` | Record a new income |
| PATCH | `/v1/incomes/{incomeID}` | Update an income |

</details>

<details>
<summary><b>Debts</b> &mdash; <code>/v1/debts</code></summary>

| Method | Path | Description |
|---|---|---|
| GET   | `/v1/debts` | List all debts for the authenticated user |
| POST  | `/v1/debts` | Create a debt |
| PATCH | `/v1/debts/{debtID}` | Update a debt |
| GET   | `/v1/debts/installment/{debtID}` | Repayment history for a debt |
| PATCH | `/v1/debts/installment/{debtID}` | Make a debt payment |

</details>

<details>
<summary><b>Expenses (one-shot, recurring, OCR receipts)</b> &mdash; <code>/v1/expenses</code></summary>

| Method | Path | Description |
|---|---|---|
| GET   | `/v1/expenses` | List all expenses for the authenticated user |
| POST  | `/v1/expenses` | Create an expense |
| PATCH | `/v1/expenses/{expenseID}` | Update an expense |
| POST  | `/v1/expenses/recurring` | Create a recurring expense |
| GET   | `/v1/expenses/recurring` | List recurring expenses |
| PATCH | `/v1/expenses/recurring/{expenseID}` | Update a recurring expense |
| POST  | `/v1/expenses/receipts` | OCR-parse a receipt image and analyze it |

</details>

<details>
<summary><b>Investments (stocks, bonds, alternatives, transactions, analysis)</b> &mdash; <code>/v1/investments</code></summary>

| Method | Path | Description |
|---|---|---|
| GET    | `/v1/investments/stocks` | List stock holdings |
| POST   | `/v1/investments/stocks` | Create a stock holding |
| PATCH  | `/v1/investments/stocks/{stockID}` | Update a stock holding |
| DELETE | `/v1/investments/stocks/{stockID}` | Delete a stock holding |
| GET    | `/v1/investments/bonds` | List bond holdings |
| POST   | `/v1/investments/bonds` | Create a bond holding |
| PATCH  | `/v1/investments/bonds/{bondID}` | Update a bond holding |
| DELETE | `/v1/investments/bonds/{bondID}` | Delete a bond holding |
| GET    | `/v1/investments/alternative` | List alternative investments |
| POST   | `/v1/investments/alternative` | Create an alternative investment |
| PATCH  | `/v1/investments/alternative/{alternativeID}` | Update an alternative investment |
| DELETE | `/v1/investments/alternative/{alternativeID}` | Delete an alternative investment |
| POST   | `/v1/investments/transactions` | Record an investment transaction |
| DELETE | `/v1/investments/transactions/{transactionID}` | Delete an investment transaction |
| GET    | `/v1/investments/analysis` | Run a portfolio analysis (LLM-backed) |
| GET    | `/v1/investments/analysis/stream` | Stream a portfolio analysis as Server-Sent Events |
| GET    | `/v1/investments/analysis/summary` | Latest LLM analysis snapshot |

</details>

<details>
<summary><b>Personal finance (analysis, predictions, summaries)</b> &mdash; <code>/v1/personalfinance</code></summary>

| Method | Path | Description |
|---|---|---|
| GET | `/v1/personalfinance/analysis` | Aggregated financial detail for analysis |
| GET | `/v1/personalfinance/summary` | Investment summary across all asset types |
| GET | `/v1/personalfinance/prediction` | ML-backed personal-finance prediction |
| GET | `/v1/personalfinance/expense-income/summary` | Expense vs. income summary report |

</details>

<details>
<summary><b>Feeds (curated RSS &amp; favorites)</b> &mdash; <code>/v1/feeds</code></summary>

| Method | Path | Description |
|---|---|---|
| GET    | `/v1/feeds` | RSS posts filtered by the user's favorite tags |
| POST   | `/v1/feeds` | Submit a new feed for ingestion |
| GET    | `/v1/feeds/{postID}` | Read a single RSS post |
| PATCH  | `/v1/feeds/{feedID}` | Update a feed |
| DELETE | `/v1/feeds/{feedID}` | Delete a feed |
| POST   | `/v1/feeds/favorites` | Mark a post as a favorite |
| DELETE | `/v1/feeds/favorites/{postID}` | Remove a favorite |

</details>

<details>
<summary><b>Awards, search options, notifications, comments</b></summary>

**Awards** (`/v1/awards`)

| Method | Path | Description |
|---|---|---|
| GET | `/v1/awards` | All awards earned by the authenticated user |

**Search options** (`/v1/search-options`)

| Method | Path | Description |
|---|---|---|
| GET | `/v1/search-options/budget-categories` | Distinct budget categories |
| GET | `/v1/search-options/currencies` | Supported currency codes |
| GET | `/v1/search-options/budget-id-names` | Budget id / name pairs |

**Notifications** (`/v1/notifications`)

| Method | Path | Description |
|---|---|---|
| GET    | `/v1/notifications` | List notifications |
| PATCH  | `/v1/notifications/{notificationID}` | Mark a notification as read |
| DELETE | `/v1/notifications/{notificationID}` | Delete one notification |
| DELETE | `/v1/notifications` | Clear all notifications |

**Comments &amp; reactions** (`/v1/comments`)

| Method | Path | Description |
|---|---|---|
| GET    | `/v1/comments` | List comments for an associated entity |
| POST   | `/v1/comments` | Post a new comment |
| PATCH  | `/v1/comments/{commentID}` | Edit a comment |
| DELETE | `/v1/comments/{commentID}` | Delete a comment |
| POST   | `/v1/comments/reaction` | React to a comment |
| DELETE | `/v1/comments/reaction/{commentID}` | Remove a reaction |

</details>

<details>
<summary><b>Other</b> &mdash; <code>/v1/contact-us</code>, <code>/v1/sse</code></summary>

| Method | Path | Description |
|---|---|---|
| POST | `/v1/contact-us` | Submit a contact-us message |
| GET  | `/v1/sse` | Server-Sent Events stream for live notifications |

</details>

A machine-readable OpenAPI/Swagger document is on the roadmap; the source of truth in the meantime is `cmd/api/routes.go`.

## 🔧 Running the tests <a name = "tests"></a>

The project has existing tests represented by files ending with the word `"_test"` e.g `internal_helpers_test.go`

### Break down into end to end tests

Each test file contains a myriad of tests to run on various entities mainly functions.
The test files are organized into `structs of tests` and their corresponding test logic.

You can run them directly from the vscode test UI. Below represents test results for the scraper:

```bash
=== RUN   Test_generateSecurityKey
=== RUN   Test_generateSecurityKey/Valid_AES-128_key_(16_bytes)
--- PASS: Test_generateSecurityKey/Valid_AES-128_key_(16_bytes) (0.00s)
=== RUN   Test_generateSecurityKey/Valid_AES-192_key_(24_bytes)
--- PASS: Test_generateSecurityKey/Valid_AES-192_key_(24_bytes) (0.00s)
=== RUN   Test_generateSecurityKey/Valid_AES-256_key_(32_bytes)
--- PASS: Test_generateSecurityKey/Valid_AES-256_key_(32_bytes) (0.00s)
=== RUN   Test_generateSecurityKey/Invalid_key_length_(0_bytes)
--- PASS: Test_generateSecurityKey/Invalid_key_length_(0_bytes) (0.00s)
=== RUN   Test_generateSecurityKey/Invalid_key_length_(-1_bytes)
--- PASS: Test_generateSecurityKey/Invalid_key_length_(-1_bytes) (0.00s)
--- PASS: Test_generateSecurityKey (0.00s)
PASS
ok      github.com/Blue-Davinci/OptiVest/internal/data  0.674s
```
- <b>All other tests follow a similar prologue.</b>

### Continuous integration

Every pull request to `main` and every push to `main` triggers the
GitHub Actions workflow at [`.github/workflows/ci.yml`](.github/workflows/ci.yml).
The pipeline is parallel where it can be — eight independent jobs fan
out from a single trigger and rejoin in a `ci-summary` aggregator that
makes branch-protection wiring trivial:

| Job | Tool | What it gates on |
|---|---|---|
| `lint` | golangci-lint v2 | govet, staticcheck, errcheck, ineffassign, unused, bodyclose, nolintlint, misspell, gofmt, goimports |
| `test` | `go test -race` | Build, vet, race-tested unit tests, coverage summary in the run page |
| `vuln` | govulncheck | Known CVEs in the dependency graph |
| `secrets` | gitleaks | Hardcoded secrets in the diff (defense-in-depth alongside GitGuardian) |
| `dockerfile-lint` | hadolint | Dockerfile + Dockerfile.migrate best practices |
| `docker-build` | buildx + GHA cache | Builds both images, exports tarballs as artifacts |
| `image-scan` | trivy | HIGH/CRITICAL CVEs and Dockerfile misconfig; SARIF uploaded to the Security tab |
| `sbom` | syft | SPDX-JSON SBOM per image, retained 30 days |
| `e2e` | docker compose | Brings the full stack up in CI, probes `/healthcheck`, `/readyz`, `/metrics`, `/debug/vars`, tears down |

A new commit on the same PR auto-cancels the in-flight run via the
workflow's `concurrency` block, so the canonical "ready to merge"
status is always the one on the latest commit.

Caching is layered so warm runs are fast: `actions/setup-go` caches the
Go module + build cache, the golangci-lint action caches its own
analysis state, and `docker/build-push-action` uses `type=gha` so layer
reuse persists across runs and across branches that share a base.

Locally, `make audit` runs the same lint + vet + govulncheck + race-tested
test suite that CI runs (minus the docker / scan / e2e jobs).


As earlier mentioned, the api uses a myriad of flags which you can use to launch the application.
An example of launching the application with your `smtp server's setting` includes:
```bash
make build/api ## build api using the makefile
./bin/api.exe -smtp-username=pigbearman -smtp-password=algor ## run the built api with your own values

Direct Run: 
go run main.go
```


## 🚀 Deployment <a name = "deployment"></a>

**(Will Be Added Soon.)**

## ⛏️ Built Using <a name = "built_using"></a>
- [Go](https://golang.org/) - Backend
- [PostgreSQL](https://www.postgresql.org/) - Database
- [Redis](https://redis.io/) - Cacheing
- [Paystack](https://paystack.com) - Payment processing
- [SQLC](https://github.com/kyleconroy/sqlc) - Generate type safe Go from SQL
- [Goose](https://github.com/pressly/goose) - Database migration tool
- [HTML/CSS](https://developer.mozilla.org/en-US/docs/Web/HTML) - Templates

## ✍️ Authors <a name = "authors"></a>

- [@Blue-Davinci](https://github.com/Blue-Davinci) - Idea & Initial work

See also the list of [contributors](https://github.com/Blue-Davinci/OptiVest/contributors) who participated in this project.

## 🎉 Acknowledgements <a name = "acknowledgement"></a>

- Hat tip to anyone whose code was used
- Inspiration

## 📚 References <a name = "references"></a>

- [Go Documentation](https://golang.org/doc/): Official Go documentation and tutorials.
- [PostgreSQL Documentation](https://www.postgresql.org/docs/): Official PostgreSQL documentation.
- [SQLC Documentation](https://docs.sqlc.dev/en/latest/): Official SQLC documentation and guides.
