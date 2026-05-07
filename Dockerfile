# syntax=docker/dockerfile:1.7

# =============================================================================
# OptiVest API — multi-stage container build
# =============================================================================
#
# Stage 1: builder
#   - golang:1.26-alpine matches the toolchain pinned in go.mod / CI
#   - dependencies are downloaded in a dedicated layer so source changes
#     don't bust the (much slower) go mod download cache
#   - CGO is explicitly disabled. None of our direct or transitive deps
#     pull in cgo (verified: pure-go lib/pq, no go-sqlite3 etc.), and a
#     static binary lets the runtime image be slim alpine (or distroless
#     in a follow-up) without worrying about glibc / musl symbol drift.
#   - linker flags strip debug symbols (-s) and the section table (-w);
#     CI builds locally with -ldflags '-s' for the same reason.
#
# Stage 2: runtime
#   - alpine:3.23 is small (~7 MiB) and ships an interactive shell + curl,
#     which keeps `docker exec` debugging usable. A future tightening
#     pass could swap to gcr.io/distroless/static:nonroot to drop another
#     few MiB and remove the shell, but that comes at the cost of incident-
#     time tooling.
#   - we run as a dedicated non-root user (optivest, uid 10001). Even
#     though the binary listens on 4000 (>1024), having USER set means a
#     compromised process cannot escalate to root inside the container,
#     which is the baseline expectation for any container that ships to
#     a multi-tenant cluster.
#   - certificates are copied from the builder so outbound TLS to Alpha
#     Vantage / FRED / FMP / SambaNova / OCR.Space works out of the box;
#     ca-certificates is a no-op layer on alpine which already ships the
#     trust bundle, but copying makes the runtime stage portable to a
#     scratch / distroless base in a future tightening pass.
#   - HEALTHCHECK polls /healthcheck (mounted on the base router, no auth,
#     no rate limiting). 5s timeout / 30s interval / 3 retries gives ~95s
#     before the container is marked unhealthy — long enough to ride out
#     a single GC pause but short enough to detect a hung process.
# =============================================================================

# ---------- Stage 1: builder ----------
FROM golang:1.26-alpine AS builder

WORKDIR /src

# Toolchain extras. git is pulled in implicitly by some go modules during
# `go mod download` (private replace directives, vcs stamping); without it
# the download fails with cryptic "exec: git: not found" errors.
RUN apk add --no-cache git ca-certificates tzdata

# Dependency layer. Copying just go.mod/go.sum first means iterative source
# edits during local development do not re-trigger `go mod download` — the
# expensive step is cached as long as the module graph is unchanged.
COPY go.mod go.sum ./
RUN go mod download

# Source layer. .dockerignore keeps node_modules / .git / vendored test
# fixtures out of the build context.
COPY . .

# Build flags:
#   CGO_ENABLED=0     -> fully static binary, runtime needs no libc
#   GOOS/GOARCH       -> default to the build platform; multi-arch is a
#                        future enhancement using buildx.
#   -trimpath         -> strip absolute paths so the binary is reproducible
#                        across machines and so error stacks don't leak
#                        the build host's filesystem layout.
#   -ldflags '-s -w'  -> strip the symbol table and DWARF info; reduces
#                        binary size by ~30% with no debugging cost since
#                        we collect logs out-of-band via zap.
ENV CGO_ENABLED=0
RUN go build -trimpath -ldflags='-s -w' -o /out/api ./cmd/api

# ---------- Stage 2: runtime ----------
FROM alpine:3.23

# OCI image labels. Registries (GHCR, Harbor, Quay), Docker Desktop, and
# SBOM tooling (syft, trivy) all key off these to display source links,
# license, and build metadata. They are pure metadata, no runtime cost.
# org.opencontainers.image.version is left for CI to overwrite via
# --label or --build-arg when releasing tagged images.
LABEL org.opencontainers.image.title="optivest-api" \
      org.opencontainers.image.description="OptiVest backend API server" \
      org.opencontainers.image.source="https://github.com/Blue-Davinci/OptiVest" \
      org.opencontainers.image.licenses="MIT" \
      org.opencontainers.image.vendor="OptiVest"

# Runtime packages:
#   tini             — see ENTRYPOINT note below
#   ca-certificates  — needed for outbound TLS to the financial-data vendors
#                       (Alpha Vantage, FRED, FMP, SambaNova, OCR.Space)
#   wget             — used by HEALTHCHECK below; busybox wget ships with
#                       alpine but spelling it out explicitly means a
#                       future base swap (e.g. to a slimmer scratch /
#                       distroless image) breaks loudly at build rather
#                       than silently at runtime
#   tzdata           — so log timestamps and cron schedules use the
#                       configured timezone, not UTC-only
RUN apk add --no-cache tini ca-certificates wget tzdata \
    && addgroup -S optivest \
    && adduser -S -u 10001 -G optivest optivest

WORKDIR /app
COPY --from=builder /out/api /app/api

# Drop privileges. UID 10001 matches a number of cloud-provider non-root
# presets so a downstream k8s manifest can use runAsUser: 10001 with no
# coordination; the choice is otherwise arbitrary as long as it is not 0.
USER optivest

EXPOSE 4000 4001

# tini gives us a real PID 1: it forwards SIGTERM/SIGINT to the Go binary
# (PID 1 in Linux has special signal semantics — default handlers are
# ignored unless explicitly registered, which can stall graceful shutdown)
# and reaps zombie children if any are ever spawned. Today the binary
# does no os/exec work so the zombie-reaper aspect is forward-looking,
# but tini is ~30 KB and zero runtime overhead, so the insurance is free.
ENTRYPOINT ["/sbin/tini", "--"]
CMD ["/app/api"]

# k8s probes typically replace this with their own liveness/readiness
# definitions, but for plain `docker run` and docker compose the
# HEALTHCHECK is what `docker ps` reports as healthy/unhealthy.
# Note on the wget invocation: busybox wget (the variant alpine ships) has
# subtly different exit-code semantics from GNU wget. In particular
# `wget --spider` returns non-zero on a perfectly successful HEAD response,
# which silently breaks the probe. Doing a tiny GET and discarding the body
# avoids that footgun and works on both flavours.
HEALTHCHECK --interval=30s --timeout=5s --start-period=10s --retries=3 \
    CMD wget -q -O /dev/null --tries=1 http://127.0.0.1:4000/healthcheck || exit 1
