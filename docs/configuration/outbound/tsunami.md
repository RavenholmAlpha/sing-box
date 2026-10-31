---
icon: material/new-box
---

!!! question "Since sing-box 1.15.0"

### Structure

```json
{
  "type": "tsunami",
  "tag": "tsunami-out",

  "server": "127.0.0.1",
  "server_port": 443,
  "password": "secret",
  "padding_scheme": [],
  "idle_session_check_interval": "30s",
  "idle_session_timeout": "5m",
  "min_idle_session": 0,
  "tls": {},
  "fronting": {
    "enabled": true,
    "path": "/assets/update",
    "host": "example.com",
    "secret": "front-secret",
    "transport": "h2"
  },

  ... // Dial Fields
}
```

### Fields

#### server

==Required==

The server address.

#### server_port

==Required==

The server port.

#### password

==Required==

The TSUNAMI password.

#### padding_scheme

Initial padding scheme line array. When omitted the protocol default is used and
the server may push a new scheme after authentication.

#### idle_session_check_interval

Interval checking for idle sessions. Default: `30s`.

#### idle_session_timeout

In the check, close sessions that have been idle for longer than this.
Default: `5m`.

#### min_idle_session

In the check, at least the first `n` idle sessions are kept open. Default: `0`.

#### tls

==Required==

TLS configuration, see [TLS](/configuration/shared/tls/#outbound). The
ClientHello can be made browser-like with the `utls` fields, for example
`"utls": { "enabled": true, "fingerprint": "chrome" }`.

#### fronting

HTTPS fronting transport. When omitted or disabled the outbound speaks raw
TSUNAMI over TLS (the server must run with fronting disabled); when enabled the
tunnel is carried inside a signed HTTP request.

#### fronting.path

The HTTP route that accepts authenticated tunnels. Default: `/assets/update`.

#### fronting.host

The HTTP `Host` header (and `:authority`). Defaults to the TLS server name.

#### fronting.secret

The HTTP-layer HMAC secret. Defaults to the password.

#### fronting.transport

`h2` (default) or `websocket`.

### Dial Fields

See [Dial Fields](/configuration/shared/dial/) for details.
