# TSUNAMI protocol support for sing-box — integration plan

Status: development plan (pre-implementation). Branch: `feat/tsunami-protocol`,
based on `v1.14.4` (`c005f3530`). The previous, abandoned 2026-05 outbound
attempt is preserved as `archive/tsunami-protocol-20260509`.

## 1. Goal

Make sing-box a first-class peer of the TSUNAMI proxy protocol
(<https://github.com/RavenholmAlpha/tsunami>, MIT):

- **outbound** `type: tsunami` — sing-box connects to an existing
  `tsunami-server`, authenticates, multiplexes all destinations over one
  session (default: HTTPS/HTTP2 fronting, optional raw TLS), supports UoT for
  UDP.
- **inbound** `type: tsunami` — sing-box accepts connections from existing
  `tsunami-client` peers (raw TLS first, then HTTP/2 fronting with the same HMAC
  scheme, optional decoy/site-camouflage responses).

Interoperability with the native TSUNAMI binaries is the acceptance bar; a
sing-box↔sing-box path is tested afterwards.

## 2. Wire protocol summary (from the TSUNAMI repo)

- Transport: TLS 1.3. Production transport is HTTPS fronting (HTTP/2 POST to a
  configured path carrying an HMAC-authenticated request); raw TLS without
  fronting is supported by the server only for controlled/testing setups.
- Auth: 32-byte `SHA-256(password)`, then `uint16` padding length, then
  `padding0`. After auth the client sends a `cmdSettings` frame (version,
  client string, padding-scheme MD5, shaping flag); the server replies
  `cmdServerSettings` only on success. Version is `3` today and must match
  exactly.
- Frames: `cmd(1) | streamID(4, BE) | dataLen(2, BE) | data` (7-byte header,
  max data 65535). Commands: `0 waste`, `1 SYN`, `2 PSH`, `3 FIN`,
  `4 settings`, `5 alert`, `6 update_padding_scheme`, `7 SYNACK`,
  `8 heart_request`, `9 heart_response`, `10 server_settings`.
- Session/stream model: one transport connection multiplexes many streams;
  `SYN`/`SYNACK`/`PSH`/`FIN`, read-buffer backpressure, heartbeats, idle
  cleanup, session pool with Surge-style expansion (client).
- Padding scheme v2 (server-pushed, MD5-negotiated): line-based text
  (`stop`, `stream_bytes`, `ctl`, `c<n>`/`s<n>` directional rules,
  `~min-max` pad-to segments, `keepalive`). Applied per stream and per
  direction by a padding writer that appends `waste` frames inside the same
  write. Clients advertise `padding-v2=1` in their settings so old clients are
  never pushed syntax they cannot parse.
- UDP: UoT v2 via the magic address `sp.v2.udp-over-tcp.arpa`
  (`love.uot.world.anytls` accepted for migration).
- Fronting HMAC headers: `X-Api-Version: v1`, `X-Date` (unix seconds),
  `X-Trace-Id` (random nonce), `Authorization: HMAC-SHA256 Signature=<hex>`;
  MAC covers method, path, Host, timestamp, nonce. Requests are
  `POST <path>` with `Content-Type: application/octet-stream`; request and
  response bodies are the tunnel byte stream.
- Server-side site camouflage (optional): unauthenticated requests are answered
  by a reverse proxy to a local same-IP site (`local`), another website
  (`remote`), or a built-in page; the tunnel path wins only when the HMAC is
  valid.

## 3. Architecture inside sing-box

Follow the in-tree pattern (`protocol/vless`, `protocol/trojan`) instead of a
separate `sing-*` module. The wire/session code is self-contained under
`protocol/tsunami`; it can later be extracted into
`github.com/tsunami-protocol/sing-tsunami` if upstreaming is desired.

```
protocol/tsunami/
  frame.go        frame encode/decode (copy of tsunami pkg/protocol/frame.go)
  command.go      command constants, UoT magic addresses
  auth.go         auth request/response, user lookup
  settings.go     client/server settings encode/decode + version
  session.go      session event loop + write path (padding-aware)
  stream.go       stream read/write buffers + flow control
  padding/        scheme v2 parser + writer (ported from tsunami pkg/padding)
  fronting/       h2/ws client transport + server h2 handler + HMAC + decoy
  inbound.go      adapter.Inbound: listener, TLS, fronting dispatch, router
  outbound.go     adapter.Outbound: TLS/uTLS, fronting dial, session pool
  service.go      server-side session lifecycle (mirrors sing-anytls Service)
  client.go       client-side session pool (mirrors sing-anytls Client)
```

Files touched elsewhere:

- `constant/proxy.go`: `TypeTsunami = "tsunami"` + `ProxyDisplayName`.
- `option/tsunami.go`: inbound/outbound option structs (see §5).
- `include/registry.go`: import `protocol/tsunami`; call
  `tsunami.RegisterInbound(registry)` and `tsunami.RegisterOutbound(registry)`
  next to the `anytls`/`trojan` entries.
- `docs/configuration/inbound/tsunami.md` + `outbound/tsunami.md` (+ `.zh.md`
  siblings, and the protocol index tables).
- `protocol/tsunami/*_test.go`.

Key sing-box APIs already verified for this base:

- `common/tls.NewClient` / `NewDialer` / `NewServer` / `ServerHandshake`
  support uTLS fingerprints through `option.OutboundTLSOptions.UTLS`
  (`fingerprint: chrome`), which is required to keep the fronting ClientHello
  browser-like.
- `golang.org/x/net` v0.57.0 is already a dependency → `http2.Transport` for
  the fronting client and `http2.ServeConn` for the fronting server.
- `common/listener` for TCP listeners, `adapter.ConnectionRouterEx
  .RouteConnectionEx` for inbound routing, `common/uot` for UoT v2.
- `adapter.OutboundWithMultiplex` + `MultiplexEnabled()` mirrors AnyTLS and
  lets sing-box know the outbound multiplexes streams itself.
- JSON option decoding is registry-driven: `inbound.Register[T]` /
  `outbound.Register[T]` register both constructor and option type, so no
  manual edits are needed in `option/inbound.go` / `option/outbound.go`.

## 4. Milestones

### M1 — outbound, raw TLS (foundation)

Port `frame`, `command`, `auth`, `settings`, `session`, `stream`, `padding`
(compressed to the TSUNAMI v2 semantics), `client` session pool. Outbound
constructor mirrors `protocol/anytls/outbound.go`; `dialOut` = TLS connect
(no fronting) → auth handshake → settings exchange → session pool.
Acceptance: sing-box outbound dials a local `tsunami-server --fronting=false`,
TCP and UoT work, `go build ./...`, unit tests for frame/padding/session.

### M2 — outbound, HTTP/2 fronting (production path)

Fronting client: TLS (uTLS Chrome fingerprint, ALPN `h2`), `http2.Transport`
POST with HMAC headers and pipe-backed streaming body; expose the response body
as the session `net.Conn`. Option surface: `fronting.enabled`, `fronting.path`,
`fronting.host`, `fronting.secret`, `fronting.transport`
(`h2` only initially; `websocket` later).
Acceptance: sing-box outbound ↔ real `tsunami-server` (fronting enabled);
`curl` through sing-box mixed inbound works; padding scheme push from server is
applied.

### M3 — inbound, raw TLS

Inbound mirrors `protocol/anytls/inbound.go`: `tls.NewServer` +
`tls.ServerHandshake`, `Service` auth/session, per-stream
`router.RouteConnectionEx` with destination from the SOCKS address written by
the client; UoT router wrapper. Users/passwords + padding scheme options.
Acceptance: real `tsunami-client --fronting=false` through sing-box inbound,
TCP + UDP.

### M4 — inbound, HTTP/2 fronting + decoy

Accept the fronting transport: after TLS handshake run `http2.ServeConn` with a
handler that verifies the HMAC, hijacks the POST request/response pair as a
`net.Conn` (request body + flushing response writer), then feeds `Service`;
unauthenticated requests go to the decoy handler (built-in page or
`httputil.ReverseProxy` to a configured origin, mirroring TSUNAMI's
`fronting.site` modes). Server pushes padding scheme on mismatch
(`cmdUpdatePaddingScheme`) and honours `padding-v2` capability.
Acceptance: real `tsunami-client` (fronting h2) through sing-box inbound;
active-probe response looks like a normal site; mixed-version client does not
receive v2 scheme text.

### M5 — hardening + docs

Heartbeats/idle semantics aligned with TSUNAMI defaults (jittered, no permanent
idle session), concurrency/race tests, e2e tests (sing-box↔sing-box,
sing-box↔native in both directions), docs pages + examples, changelog note.
Optional: WebSocket fronting transport, site-camouflage `remote` mode,
traffic shaping flags.

## 5. Proposed configuration schema

Outbound:

```json
{
  "type": "tsunami",
  "tag": "proxy",
  "server": "example.com",
  "server_port": 443,
  "password": "secret",
  "tls": {
    "enabled": true,
    "server_name": "example.com",
    "utls": { "enabled": true, "fingerprint": "chrome" }
  },
  "fronting": {
    "enabled": true,
    "path": "/static/js/main.1a2b3c4d.js",
    "host": "example.com",
    "secret": "front-secret",
    "transport": "h2"
  },
  "idle_session_check_interval": "30s",
  "idle_session_timeout": "5m",
  "min_idle_session": 0
}
```

Inbound:

```json
{
  "type": "tsunami",
  "tag": "tsunami-in",
  "listen": "::",
  "listen_port": 443,
  "users": [{ "name": "alice", "password": "secret" }],
  "padding_scheme": [
    "stop=6",
    "stream_bytes=12288",
    "ctl=~40-160",
    "c0=~80-200",
    "c1=~900-1200",
    "s0=~2600-4200",
    "keepalive=45000-90000:4-8"
  ],
  "tls": { "enabled": true, "certificate_path": "/path/fullchain.pem", "key_path": "/path/privkey.pem" },
  "fronting": {
    "enabled": true,
    "path": "/static/js/main.1a2b3c4d.js",
    "secret": "front-secret",
    "site": { "mode": "local", "upstream": "http://127.0.0.1:8080" }
  }
}
```

Defaults aim to match `tsunami-server`/`tsunami-client` defaults so a user can
move a working TSUNAMI deployment into sing-box by changing only
`type`/`server`/`password` (and TLS paths on the server).

## 6. Porting map (TSUNAMI repo → sing-box package)

| TSUNAMI file | sing-box location | Adaptation |
| --- | --- | --- |
| `pkg/protocol/frame.go`, `command.go` | `protocol/tsunami/frame.go`, `command.go` | package rename, no behavior change |
| `pkg/protocol/auth.go`, `settings.go` | `protocol/tsunami/auth.go`, `settings.go` | keep wire format, add capability flag |
| `pkg/protocol/session.go`, `stream.go` | `protocol/tsunami/session.go`, `stream.go` | keep; plug padding writer |
| `pkg/padding/*` | `protocol/tsunami/padding/` | keep v2 semantics + tests |
| `pkg/mux/Pool` (client pool) | `protocol/tsunami/client.go` | adapt to sing context/logger; idle jitter |
| `pkg/fronting/auth.go`, `httpconn.go` | `protocol/tsunami/fronting/` | HMAC + net.Conn adapters |
| `pkg/fronting/config.go` (`site`) | `protocol/tsunami/fronting/decoy.go` | reverse proxy + built-in page |
| `pkg/server/server.go` (session handling) | `protocol/tsunami/service.go` + `inbound.go` | route through sing router |
| `pkg/client/client.go` (dial/auth) | `protocol/tsunami/outbound.go` | use sing TLS/uTLS + dialer |

## 7. Risks and decisions to confirm

1. **In-fork vs. external module.** Plan uses in-fork packages (one repo, one
   build). External `sing-tsunami` repo would mirror `sing-anytls` and be
   cleaner for upstreaming, but adds release/versioning overhead.
2. **Fronting server implementation.** Use `x/net/http2.ServeConn` on the
   already-handshaked TLS conn, not `net/http.Server`, to match TSUNAMI's
   streaming/flush behaviour. Confirm acceptable.
3. **uTLS fingerprint.** sing-box's uTLS option lets us keep the Chrome
   ClientHello; must validate that `http2.Transport` is used over that conn
   (fronting) without breaking fingerprint.
4. **Padding compatibility.** Server pushes v2 text only to clients advertising
   `padding-v2`; new sing-box client must advertise it and parse v2.
5. **Version handshake.** TSUNAMI requires exact protocol version 3 in client
   settings; a sing-box client must send version 3 and valid settings, or the
   native server rejects the session.
6. **Site camouflage scope.** Initially built-in + `local`/`remote` reverse
   proxy; WebSocket fronting and non-h2 transports deferred to M5.
7. **License.** TSUNAMI is MIT, sing-box is GPL-3.0-or-later; MIT code can be
   included with attribution (keep the copyright notice in adapted files).
8. **Base version.** `v1.14.4` stable now; rebase onto `testing` before
   upstreaming.

## 8. Build / verification commands

```bash
# toolchain (repo requires Go 1.25.5; GOTOOLCHAIN=auto downloads it)
go build ./...
go test ./protocol/tsunami/...
go vet ./protocol/tsunami/...

# interop fixtures (from the TSUNAMI repo)
go build ./cmd/tsunami-server ./cmd/tsunami-client
./tsunami-server --listen :8443 --password test --fronting --front-path /x ...
./tsunami-client --server 127.0.0.1:8443 --password test --fronting ...
```

## 9. Immediate next steps

1. Approve the architecture/config schema above (or request changes).
2. Scaffold `option/tsunami.go`, `constant` entry, `include/registry.go`
   wiring, and empty (compiling) `protocol/tsunami` constructors.
3. Implement M1 (frame/session/padding + raw-TLS outbound) with unit tests.
4. Validate against native `tsunami-server`; then M2 fronting outbound.
