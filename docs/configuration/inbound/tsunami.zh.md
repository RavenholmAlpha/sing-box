---
icon: material/new-box
---

!!! question "自 sing-box 1.15.0"

### 结构

```json
{
  "type": "tsunami",
  "tag": "tsunami-in",

  ... // 监听字段

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

### 监听字段

参阅 [监听字段](/zh/configuration/shared/listen/)。

### 字段

#### users

==必填==

TSUNAMI 用户。

#### padding_scheme

TSUNAMI 填充方案行数组。服务端在认证后推送给客户端；支持 v2 语法的新客户端通过协议能力标记识别，旧客户端保留自己的方案。

默认填充方案：

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

==必填==

TLS 配置，参阅 [TLS](/zh/configuration/shared/tls/#入站)。

#### fronting

HTTPS 前置层。省略或禁用时，入站直接接受 TLS 上的原始 TSUNAMI 协议；启用后，入站先作为 HTTP/2 或 HTTP/1.1 网站响应，只有携带正确前置签名的请求才会被升级为隧道。

#### fronting.path

接受已认证隧道的 HTTP 路径。默认：`/assets/update`。

#### fronting.secret

HTTP 层 HMAC 密钥。启用前置时必填；省略时使用第一个用户的密码。

#### fronting.site

未认证访客看到的伪装网站。

#### fronting.site.mode

`builtin`、`local` 或 `remote`。未配置上游时默认为 `builtin`。

- `builtin`：内置占位页面。
- `local`：反向代理到本机运行的真实网站（同 IP 伪装站点）。
- `remote`：反向代理到其他网站，借用其内容作为伪装。

#### fronting.site.upstream

`local` 与 `remote` 模式下的伪装源站，例如 `http://127.0.0.1:8080` 或 `https://www.example.com`。

#### fronting.site.preserve_host

反向代理时保留客户端 `Host` 头。默认：`local` 为 `true`，`remote` 为 `false`。

#### fronting.site.insecure_skip_verify

跳过 HTTPS 伪装源站的 TLS 证书校验。
