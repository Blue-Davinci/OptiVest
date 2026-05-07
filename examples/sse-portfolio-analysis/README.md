# SSE portfolio-analysis demo client

A single-file HTML page that streams `GET /v1/investments/analysis/stream`
into a live transcript pane. Useful for:

- Eyeballing the wire format without spinning up the full frontend.
- Confirming a deployment's CORS, auth, and SSE flushing settings end to end.
- Demoing the streaming endpoint to non-technical reviewers.

## Why not `EventSource`?

The browser's native `EventSource` API does not support custom headers, and
`/v1/investments/analysis/stream` is gated by `Authorization: Bearer <token>`
just like every other `/v1` endpoint. The demo therefore uses
`fetch()` + a hand-written SSE frame parser over the response body's
`ReadableStream`. The wire format is identical &mdash; only the transport
differs. If you ever migrate auth to a cookie or a path-bound signed
URL, you can drop the parser and fall back to `EventSource`.

## Running it

1. Start the API locally (or point at a deployment):

   ```bash
   make run/api
   ```

2. If you are opening `index.html` directly from disk (`file://`) or
   from a different origin than the API, allow the origin in the API's
   CORS list. Browsers send `Origin: null` for `file://`:

   ```bash
   ./bin/api -cors-trusted-origins="null http://localhost:8000"
   ```

   Or serve the demo with any static server and add that origin:

   ```bash
   cd examples/sse-portfolio-analysis
   python3 -m http.server 8000
   ```

3. Authenticate once to get a bearer token:

   ```bash
   curl -s -X POST http://localhost:4000/v1/api/authentication \
     -H 'Content-Type: application/json' \
     -d '{"email":"you@example.com","password":"..."}' \
     | jq -r .authentication_token.token
   ```

4. Open `index.html`, paste the token, hit **Stream analysis**, and
   watch the model type out the analysis. The `done` event payload is
   pretty-printed below the transcript when the stream completes.

## What the page asserts about the wire format

The hand-written parser mirrors the
[SSE spec](https://html.spec.whatwg.org/multipage/server-sent-events.html#parsing-an-event-stream)
and exposes its assumptions to anyone reading the source:

| Wire shape                                | Demo behaviour                                    |
| ----------------------------------------- | ------------------------------------------------- |
| `data: {"delta":"..."}\n\n`               | Appends the delta to the live transcript pane.    |
| `event: done\ndata: {...}\n\n`            | Pretty-prints the JSON in the **Final payload** card and sets the status pill to `done`. |
| `event: error\ndata: {"error":"..."}\n\n` | Renders the message in the error card and flips the status to `error`. |
| Comment / heartbeat lines (`: keepalive`) | Silently ignored.                                 |

If a future change breaks any of these (missing flush, non-JSON
payload, dropped event name, wrong `Content-Type`) the demo surfaces
the failure visibly &mdash; making it a useful lightweight integration
check on top of the unit tests in `cmd/api/streaming_test.go`.
