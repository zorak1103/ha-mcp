# Integration test environment: connectivity analysis

Goal: re-run the live-HA integration suites (`TestPersonIntegration`,
`TestZoneIntegration`, `TestPersonToolDispatch`, `TestZoneToolDispatch`) to
validate the person/zone WebSocket command-prefix fix end-to-end. Blocked on
reaching a real Home Assistant instance from this dev machine. Written so the
network side can be investigated in parallel.

## Timeline of what was tried

**1. Original config (`.env.integration`): `HA_INTEGRATION_TEST_URL=http://192.168.1.10:8123`**

```
go test -tags=integration ...
ERROR WebSocket dial failed error=failed to WebSocket dial: failed to send
handshake request: Get "http://192.168.1.10:8123/api/websocket": dial tcp
192.168.1.10:8123: connectex: No connection could be made because the target
machine actively refused it.
```

```
curl -v --max-time 5 http://192.168.1.10:8123/api/
* Trying 192.168.1.10:8123...
* connect to 192.168.1.10 port 8123 from 0.0.0.0 port 52535 failed: Connection refused
```

TCP RST on port 8123 — nothing is listening there (or a firewall is actively
rejecting, not just dropping). Consistent across ~4 retries over several
minutes.

**2. User reported: "server address has changed to `https://192.168.1.10`"**

Updated `.env.integration`:
```
HA_INTEGRATION_TEST_URL=https://192.168.1.10
```
(no explicit port → defaults to 443)

```
go test -tags=integration ...
ERROR WebSocket dial failed error=failed to WebSocket dial: expected handshake
response status code 101 but got 404
```

This is a *different* failure mode than before: TCP connects, TLS handshake
succeeds, an HTTP server answers — it just doesn't upgrade `/api/websocket` to
a WebSocket (no `101 Switching Protocols`).

**3. Probed the HTTPS endpoint directly to see what's actually there:**

```
curl -sk -i https://192.168.1.10/api/
HTTP/1.1 404 Not Found
Content-Type: text/plain; charset=utf-8
X-Content-Type-Options: nosniff
Content-Length: 19

404 page not found
```

Same exact response (headers, body, byte-for-byte) for `/`, `/api/`, and
`/api/websocket` — i.e. **every path returns the identical generic 404**, not
route-specific behavior.

## Diagnosis

The response signature (`404 page not found`, `text/plain; charset=utf-8`,
`X-Content-Type-Options: nosniff`, no body beyond that string) is the literal
output of Go's standard library `net/http` default `http.NotFound()` /
`http.ServeMux` no-route-registered handler. This is **not Home Assistant**:

- HA's frontend 404 is an HTML page (its SPA shell), and HA's REST API 404s
  return a JSON body (`{"message": "..."}`), not `text/plain`.
- Every path (including `/`) returning byte-identical output means there is no
  routing logic at all behind this listener — consistent with a bare
  `http.ListenAndServe(":443", nil)` (or a reverse proxy with zero rules
  matching, falling through to its own default backend/handler).

**Most likely explanations, roughly ranked:**

1. **Wrong backend behind a reverse proxy.** `192.168.1.10` now points at a
   proxy/gateway (e.g. Caddy, nginx, Traefik) that terminates TLS on 443, but
   its routing rule for the Home Assistant host/path isn't configured (or
   points at the wrong upstream) — so requests fall through to a default/catch-all
   Go service instead of being forwarded to `192.168.1.10:8123` (or wherever HA
   actually listens now).
2. **This is `ha-mcp` itself**, or another Go program on this host, answering
   on 443 instead of Home Assistant. Since ha-mcp uses `net/http`
   (`internal/mcp/server.go`) without a catch-all custom 404 handler, hitting
   it on an unregistered path would produce exactly this signature.
3. **HA moved to a different port/path** behind the new address (e.g.
   `https://192.168.1.10:8123` still, or `https://192.168.1.10/homeassistant/`)
   and 443-root simply has nothing mapped to it.

## Ruled out: self-signed certificate rejection

Considered whether Go's TLS stack was rejecting a self-signed cert on
`https://192.168.1.10`, rather than the 404 being a real routing problem. Ruled
out:

- If `websocket.Dial()` (`internal/homeassistant/ws_client.go:132`,
  `websocket.Dial(c.ctx, wsURL, nil)` — no custom TLS config,
  `InsecureSkipVerify` doesn't appear anywhere in the codebase) had rejected
  the certificate, the dial would fail *during the TLS handshake* with an
  `x509`/`tls` error, before any HTTP response is read.
- The actual error — `expected handshake response status code 101 but got
  404` — only occurs *after* a full TLS handshake and a complete HTTP
  request/response round trip succeed. Go received a well-formed HTTP 404.
  That proves the certificate was accepted.

Separately, plain `curl https://192.168.1.10/api/` (no `-k`) does fail with a
TLS error (`curl: (35) ... SSL connect error`), which looks contradictory at
first. This is a **trust-store mismatch between tools, not a real blocker**:
Go on Windows verifies against the Windows certificate store (which may
already trust this self-signed cert if it was manually installed there);
`curl` in this Git Bash/MSYS environment ships its own separate CA bundle that
doesn't include it, so only `curl -k` gets past it. Both tools are looking at
the same cert; only their trust anchors differ. The cert is a red herring —
the 404-on-every-path behavior is a real application-layer routing problem,
not a TLS problem.

## What would confirm/rule out each hypothesis

- `curl -sk https://192.168.1.10:8123/api/` — if this returns HA's real JSON
  response, the fix is just the URL still needing `:8123` even over HTTPS.
- Check what process/container is actually bound to `:443` on `192.168.1.10`
  (`netstat`/`ss`/`docker ps` on that host) — confirms hypothesis 1 vs 2.
- Check reverse-proxy config (nginx/Caddy/Traefik/HAProxy) for the vhost/rule
  routing to Home Assistant — confirms hypothesis 1.
- Confirm Home Assistant's own `http:` config (`configuration.yaml`) for
  `server_port` / `use_x_forwarded_for` / trusted proxies, in case HA itself
  changed its listening port during whatever migration prompted the address
  change.

## What I need to retry

Once the correct URL is confirmed reachable (`curl` returns HA's real `/api/`
response, e.g. `{"message":"API running."}` with a 401 if unauthenticated, or
200 with the configured token), update
`HA_INTEGRATION_TEST_URL`/`HA_URL` in `.env.integration` accordingly and the
integration run can be repeated:

```
set -a && source .env.integration && set +a
go test -tags=integration -v ./internal/handlers/integration/ \
  -run 'TestPersonToolDispatch|TestZoneToolDispatch|TestPersonIntegration|TestZoneIntegration' \
  -timeout 5m
```
