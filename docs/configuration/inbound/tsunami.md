---
icon: material/new-box
---

!!! question "Since sing-box 1.15.0"

### Structure

```json
{
  "type": "tsunami",
  "tag": "tsunami-in",

  ... // Listen Fields

  "users": [
    {
      "name": "alice",
      "password": "secret"
    }
  ],
  "padding_scheme": [],
  "tls": {},
  "fronting": {
    "enabled": true,
    "path": "/assets/update",
    "secret": "front-secret",
    "site": {
      "mode": "local",
      "upstream": "http://127.0.0.1:8080"
    }
  }
}
```

### Listen Fields

See [Listen Fields](/configuration/shared/listen/) for details.

### Fields

#### users

==Required==

TSUNAMI users.

#### padding_scheme

TSUNAMI padding scheme line array. Pushed to clients after authentication; v2
clients are detected via the protocol capability flag and pre-v2 clients keep
their own scheme.

Default padding scheme:

```json
[
  "stop=6",
  "stream_bytes=12288",
  "ctl=~40-160",
  "c0=~80-200",
  "c1=~900-1200",
  "c2=~500-1200",
  "s0=~2600-4200",
  "s1=~700-1400",
  "s2=~700-1400",
  "3=~300-800",
  "4=~300-800",
  "5=~300-800",
  "keepalive=45000-90000:4-8"
]
```

#### tls

==Required==

TLS configuration, see [TLS](/configuration/shared/tls/#inbound).

#### fronting

HTTPS fronting layer. When omitted or disabled the inbound expects raw TSUNAMI
over TLS; when enabled the inbound serves an HTTP/2 or HTTP/1.1 website and
only requests signed with the fronting secret are upgraded into tunnels.

#### fronting.path

The HTTP route that accepts authenticated tunnels. Default: `/assets/update`.

#### fronting.secret

The HTTP-layer HMAC secret. Required when fronting is enabled; defaults to the
first user's password when omitted.

#### fronting.site

The decoy website served to unauthenticated visitors.

#### fronting.site.mode

One of `builtin`, `local` or `remote`. Default: `builtin` when no upstream is
configured.

- `builtin`: generated placeholder page.
- `local`: reverse proxy to a website running on the same machine (same-IP
  cover site).
- `remote`: reverse proxy to another website, used as borrowed content.

#### fronting.site.upstream

The decoy origin for `local` and `remote` modes, for example
`http://127.0.0.1:8080` or `https://www.example.com`.

#### fronting.site.preserve_host

Preserve the client `Host` header when proxying. Default: `true` for `local`,
`false` for `remote`.

#### fronting.site.insecure_skip_verify

Skip TLS certificate verification for HTTPS decoy origins.
