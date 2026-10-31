---
icon: material/new-box
---

!!! question "自 sing-box 1.15.0"

### 结构

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

  ... // 拨号字段
}
```

### 字段

#### server

==必填==

服务器地址。

#### server_port

==必填==

服务器端口。

#### password

==必填==

TSUNAMI 密码。

#### padding_scheme

初始填充方案行数组。省略时使用协议默认值，认证后服务端可能推送新方案。

#### idle_session_check_interval

检查空闲会话的间隔。默认：`30s`。

#### idle_session_timeout

检查时，关闭空闲时间超过该值的会话。默认：`5m`。

#### min_idle_session

检查时至少保留的前 `n` 个空闲会话。默认：`0`。

#### tls

==必填==

TLS 配置，参阅 [TLS](/zh/configuration/shared/tls/#出站)。可通过 `utls` 字段让 ClientHello 更接近浏览器，例如 `"utls": { "enabled": true, "fingerprint": "chrome" }`。

#### fronting

HTTPS 前置传输。省略或禁用时，出站直接在 TLS 上发送原始 TSUNAMI 协议（服务端必须禁用前置）；启用后，隧道承载在带签名的 HTTP 请求中。

#### fronting.path

接受已认证隧道的 HTTP 路径。默认：`/assets/update`。

#### fronting.host

HTTP `Host` 头（及 `:authority`）。默认为 TLS 服务器名称。

#### fronting.secret

HTTP 层 HMAC 密钥。默认为密码。

#### fronting.transport

`h2`（默认）或 `websocket`。

### 拨号字段

参阅 [拨号字段](/zh/configuration/shared/dial/)。
