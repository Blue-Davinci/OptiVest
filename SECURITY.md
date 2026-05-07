# Security Policy

## Reporting a vulnerability

Please email security disclosures to the maintainer listed in `README.md` and
do not file public issues for unpatched vulnerabilities.

## Required environment variables

The API loads every secret from the environment. There are no production-safe
defaults. In any environment other than `development`, the API refuses to
start if any of the following is empty (see `validateConfig` in
`cmd/api/main.go`):

| Variable                              | Purpose                                              |
| ------------------------------------- | ---------------------------------------------------- |
| `OPTIVEST_DB_DSN`                     | PostgreSQL connection string                         |
| `OPTIVEST_DATA_ENCRYPTION_KEY`        | Hex-encoded AES-GCM key (16, 24, or 32 bytes)        |
| `OPTIVEST_ALPHAVANTAGE_API_KEY`       | Alpha Vantage stock data                             |
| `OPTIVEST_EXCHANGERATE_API_KEY`       | exchangerate-api.com FX rates                        |
| `OPTIVEST_FRED_API_KEY`               | FRED bond data                                       |
| `OPTIVEST_FINANCIALMODELINGPREP_API_KEY` | FMP sector performance                            |
| `OPTIVEST_SAMBA_NOVA_LLM_API_KEY`     | SambaNova LLM                                        |
| `OPTIVEST_PREDICTOR_API_KEY`          | OptiVest predictor microservice                      |
| `OPTIVEST_OCRSPACE_API_KEY`           | OCR.Space receipt parsing                            |
| `OPTIVEST_SMTP_HOST`                  | Mailer host                                          |
| `OPTIVEST_SMTP_USERNAME`              | Mailer username                                      |
| `OPTIVEST_SMTP_PASSWORD`              | Mailer password                                      |
| `OPTIVEST_SMTP_SENDER`                | Mailer sender address                                |

In `development` the same checks log warnings but do not abort startup, so
local work with a partial `.env` continues to function.

## Secrets must NEVER be passed on the command line

The flag definitions in `cmd/api/main.go` default-from-env for every
secret-bearing variable, so an operator who exports the corresponding
`OPTIVEST_*` env var gets the expected behavior. What's banned at startup
is passing the secret value LITERALLY on the command line - e.g.
`./api -encryption-key=<hex>` or `./api -api-key-sambanova=<value>`.

The `rejectSecretFlags` startup gate scans `os.Args` BEFORE `flag.Parse`
and exits with code 2 if any of these prefixes appears:

- `-encryption-key`, `--encryption-key`
- `-redis-password`, `--redis-password`
- `-smtp-password`, `--smtp-password`
- `-db-dsn`, `--db-dsn` (DSNs typically embed credentials)
- `-api-key-*` for any vendor (Alpha Vantage, FRED, FMP, SambaNova, etc.)

The reason matters: command-line values land in `/proc/<pid>/cmdline`
(world-readable on most Linux distros), `ps -ef`, shell history, and any
process supervisor that records argv. The startup gate makes that leak
impossible to introduce via a stressed engineer doing a quick experiment
or a runbook that hardcodes the wrong example.

The curated `/debug/vars` handler (`cmd/api/metrics.go`) provides a
second layer: even if the gate is bypassed (e.g. by a forked build),
the handler strips `cmdline` from the JSON output so operational
endpoints don't echo back what was passed.

## Generating a fresh AES-GCM key

```bash
# 32-byte (AES-256) hex-encoded key
openssl rand -hex 32
```

Set the result as `OPTIVEST_DATA_ENCRYPTION_KEY`. Rotating this key
invalidates every previously-encrypted column; rotation must be coordinated
with a re-encryption migration.

## Key rotation runbook

If a key is leaked in a commit, build artifact, log file, or chat:

1. **Rotate immediately at the upstream provider** before touching the repo.
   Old key must be revoked, not just superseded.
2. **Update the environment** in every place the key is consumed:
   production, staging, CI secret stores, your local `.env`, and any
   developer machines.
3. **Verify the leak is actually purged** — if the key was committed, it
   stays in git history forever. Do not rely on a follow-up commit to
   "remove" it. Force-push history rewrites are out of scope here; revoke
   instead.
4. **Re-run `govulncheck` and `staticcheck`**:
   ```bash
   make audit
   ```
5. **Tail logs for unauthorized use** of the old key for at least 24 hours
   (most upstreams record the last-used IP).

## Rate limiter operations

The HTTP rate limiter is backed by Redis (P2). Each request consults a
GCRA bucket via `go-redis/redis_rate/v10` keyed on the client's real IP
(`X-Forwarded-For`-aware via `tomasen/realip`). The configured
`-limiter-rps` and `-limiter-burst` are now **cluster-wide**, not
per-pod. Replicas no longer multiply the effective rate.

### Behaviour during a Redis outage

The middleware **fails open**: if Redis is unreachable or the limiter
call errors, the request is allowed through. The cost of fail-open is
bounded (upstream LB, downstream connection pools, etc. still apply).
The cost of fail-closed during a Redis blip would be a total API
outage — unacceptable for a non-critical-path dependency.

### Required alerting

Operators should configure alerts on these counters. They are exposed on
both `/debug/vars` (JSON, raw expvar) and `/metrics` (Prometheus text
exposition):

| Metric                                  | Alert when                                         |
| --------------------------------------- | -------------------------------------------------- |
| `rate_limiter_redis_errors_total`       | Rate > 0 sustained for > 1m → Redis health issue   |
| `rate_limiter_fail_open_total`          | Rate > 0 sustained for > 1m → limiter is bypassed  |
| `rate_limiter_denied_total`             | Sudden spike → potential abuse                     |
| `rate_limiter_disabled_total`           | Should be 0 in production; nonzero → misconfig     |

`rate_limiter_configured` (string) reports the active settings on each
boot for sanity-checking config rollouts; it is published on
`/debug/vars` only because Prometheus has no native string type.

### Operational endpoint exposure

`/metrics`, `/debug/vars`, `/healthcheck`, and `/readyz` are all mounted
on the base router and bypass the global middleware chain on purpose:
they are not authenticated, not rate-limited, and not counted by the
request-log middleware. The deployment is responsible for **scoping
reachability to the internal scrape / probe network** (cluster-local
Prometheus and the kubelet, an IP allow-list at the load balancer, or
an mTLS sidecar).

What each endpoint discloses, by design:

- `/healthcheck` — version, env name, process uptime in seconds. Wired
  to the container `livenessProbe`; failure causes a restart.
- `/readyz` — version, env, uptime, plus a `checks` map naming the
  required dependencies (`postgres`, `redis`) and reporting each as
  `ok` or `down`. The body deliberately does **not** include driver
  error strings, which can leak hostnames, ports, and version banners;
  operators read the structured logs for the "why". Wired to the load
  balancer / `readinessProbe`; failure pulls the instance from rotation
  without restart.
- `/debug/vars` — raw runtime memstats and goroutine counts. Not
  strictly secret but should not be publicly browsable.
- `/metrics` — the curated subset documented in `README.md`.

If a deployment must expose these endpoints publicly, gate them at the
ingress rather than weakening the bypass semantics here. In particular,
do NOT add authentication to `/healthcheck` or `/readyz`: orchestrators
do not carry tokens on probe requests, and an authenticated probe
endpoint will report the instance as failing for the wrong reason.

### Latency budget

The Redis call is wrapped in a 200ms `context.WithTimeout`. Hitting that
ceiling counts as a Redis error and triggers fail-open. The expected p99
is well below 5ms on a same-AZ Redis; if you see higher, suspect Redis
saturation rather than the limiter middleware.

## Portfolio analysis concurrency (P3)

`performInvestmentPortfolioAnalysis` fans out per-asset workers (stocks +
bonds) into a bounded `errgroup` capped by `-portfolio-worker-limit`
(default 6). The flag interacts with two security-adjacent concerns:

1. **Upstream API rate limits.** Each in-flight worker may issue 2-3
   Alpha Vantage / FRED / FMP calls plus one DB INSERT. Exceeding your
   plan's quota will trigger HTTP 429s from the vendor. Tune the limit
   to your plan: AV Free (5 req/min) -> 1; Premium (75/min) -> 6;
   Premium Plus (600/min) -> 16+.
2. **Resource exhaustion.** A `-portfolio-worker-limit` above ~64 lets
   a single authenticated user open dozens of upstream HTTP sockets and
   DB connections concurrently. `validateConfig` rejects values >64 at
   boot to force operators to opt in to that risk. The HTTP rate limiter
   is your first defence against authenticated abuse; the worker cap is
   the second (limits damage per request that does get in).

A process-wide singleflight registry collapses concurrent identical
upstream fetches (e.g. two stocks of the same symbol), so even under
fan-out we never N-multiply duplicate vendor calls. Watch
`portfolio_singleflight_collapsed_total` to confirm dedup is firing.

## SambaNova streaming budget (P4.A)

The SambaNova chat-completions client (`LLMStream` in
`cmd/api/http_clients.go`) holds a TCP connection open for 10–30s while
the model emits SSE chunks. Three knobs gate that exposure:

| Flag                   | Default | Role                                                                                                                  |
| ---------------------- | ------- | --------------------------------------------------------------------------------------------------------------------- |
| `-llm-total-budget`    | `90s`   | Wallclock cap across pre-first-byte retries plus the streaming read. Bounds the worst-case slot-hold per call.        |
| `-llm-idle-timeout`    | `15s`   | Aborts the call when no chunk arrives within the window. Slowloris-style mitigation against a stalled or hostile API. |
| `-llm-max-retries`     | `2`     | Pre-first-byte retry cap. Mid-stream errors **never** retry, regardless of this value.                                |

The mid-stream no-retry rule is a deliberate trade-off: SambaNova's API
is non-resumable, so replaying a partially-streamed prompt costs the
full prompt evaluation again (latency + tokens) for no reliability
gain. Operators wanting tighter blast-radius control should lower
`-llm-total-budget` rather than raise `-llm-max-retries`.

The idle timeout doubles as the primary defence against a hostile or
compromised LLM upstream holding our connection slots indefinitely.
Lowering it past 5s is not recommended - the model's first-token
latency is genuinely variable, and false-positive aborts manifest as
user-visible 5xxs.

## Streaming portfolio analysis endpoint (P4.B)

`GET /v1/investments/analysis/stream` is the inbound counterpart to the
SambaNova streaming client. It is mounted on the regular `/v1` router
(not the long-lived `sseRoutes` server), so it inherits the full
middleware chain: per-IP rate limiting, bearer-token auth + activated-user
check, structured request logging, and X-Request-ID correlation. This is
deliberate — each call is bounded by `-llm-total-budget` (default 90s)
and behaves much more like a slow HTTP request than a persistent push
channel, so the same controls that protect `/v1/investments/analysis`
must apply here.

Two wire-level rules limit information leakage on the SSE error path:

- **No raw upstream errors hit the wire.** `classifyLLMStreamError`
  collapses transport / dial / TLS errors to a fixed phrase (`internal
  stream error`) and only surfaces messages we author ourselves
  (`upstream stalled`, `upstream timed out`, `request canceled`,
  `llm: non-2xx response: <code>`). Hostnames, retry counts, and
  socket-level diagnostics stay in the structured server logs.
- **No partial deltas after a failure.** `streamLLMToSSE` returns the
  underlying error to the handler rather than emitting a synthetic
  empty delta. The handler writes a single `event: error` and the
  connection ends. Clients can therefore treat any received delta as
  authoritative, with no need to reconcile partial frames.

A persist failure on stream completion (writing the analyzed portfolio
to the table read by `/v1/investments/analysis/summary`) is logged at
Error level and surfaced via the standard request log line, but does
not turn a successful stream into a visible error — the user already
has the result in their browser. Operators see this as a
`portfolio analysis stream persist failed` log entry, the metric
counters are unchanged, and the user has to re-run the analysis to
see it in their history view. We accept that trade-off because
emitting a failure event for a *successful* stream would mislead the
client into discarding a valid analysis.

## CI security scanning

Every push and PR to `main` runs:

- `go vet ./...`
- `staticcheck ./...`
- `govulncheck ./...`
- `go test -race ./...`

See `.github/workflows/ci.yml`. Failures block merges.

## Known historical exposure

The git history of this repository contains the following keys that were
committed prior to the P0 security PR. Treat all of them as compromised
and rotate at the upstream provider — the project documentation no longer
references them, but they remain in `git log`:

- Hardcoded Alpha Vantage key in `cmd/api/investment_operations.go`
- Default values shown in the README's flag table for FRED, FMP,
  OCR.Space, SambaNova, the predictor microservice, and ExchangeRate-API
- Default `-encryption-key` value in the README

If any of those keys was used outside of testing, regenerate it now. After
this PR ships, the API will not boot in production without the env vars
listed above being set.
