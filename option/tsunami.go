package option

import "github.com/sagernet/sing/common/json/badoption"

type TsunamiInboundOptions struct {
	ListenOptions
	InboundTLSOptionsContainer
	Users         []TsunamiUser              `json:"users,omitempty"`
	PaddingScheme badoption.Listable[string] `json:"padding_scheme,omitempty"`
	Fronting      *TsunamiInboundFronting    `json:"fronting,omitempty"`
}

type TsunamiUser struct {
	Name     string `json:"name,omitempty"`
	Password string `json:"password,omitempty"`
}

// TsunamiInboundFronting enables the HTTPS fronting layer for unauthenticated
// and authenticated requests. When disabled the inbound expects raw TSUNAMI
// over TLS.
type TsunamiInboundFronting struct {
	Enabled bool `json:"enabled,omitempty"`
	// Path is the HTTP route that accepts authenticated tunnels.
	Path string `json:"path,omitempty"`
	// Secret is the HTTP-layer HMAC secret. Empty falls back to the user
	// passwords (same behavior as tsunami-server).
	Secret string `json:"secret,omitempty"`
	// Site configures what unauthenticated visitors see.
	Site *TsunamiSite `json:"site,omitempty"`
}

// TsunamiSite selects the decoy website served to unauthenticated requests.
type TsunamiSite struct {
	// Mode is one of "builtin", "local" or "remote".
	Mode string `json:"mode,omitempty"`
	// Upstream is the decoy origin (URL or host:port) for local/remote modes.
	Upstream string `json:"upstream,omitempty"`
	// PreserveHost keeps the client Host header. Defaults to true for local
	// and false for remote.
	PreserveHost *bool `json:"preserve_host,omitempty"`
	// InsecureSkipVerify disables TLS verification for HTTPS upstreams.
	InsecureSkipVerify bool `json:"insecure_skip_verify,omitempty"`
}

type TsunamiOutboundOptions struct {
	DialerOptions
	ServerOptions
	OutboundTLSOptionsContainer
	Password      string                     `json:"password,omitempty"`
	PaddingScheme badoption.Listable[string] `json:"padding_scheme,omitempty"`
	Fronting      *TsunamiOutboundFronting   `json:"fronting,omitempty"`
	// IdleSessionCheckInterval controls how often idle sessions are scanned.
	IdleSessionCheckInterval badoption.Duration `json:"idle_session_check_interval,omitempty"`
	// IdleSessionTimeout controls how long an idle session is kept.
	IdleSessionTimeout badoption.Duration `json:"idle_session_timeout,omitempty"`
	// MinIdleSession is the minimum number of idle sessions to keep.
	MinIdleSession int `json:"min_idle_session,omitempty"`
}

// TsunamiOutboundFronting enables the HTTPS fronting transport. When disabled
// the outbound speaks raw TSUNAMI over TLS (server must run with fronting off).
type TsunamiOutboundFronting struct {
	Enabled bool `json:"enabled,omitempty"`
	// Path is the HTTP route that accepts authenticated tunnels.
	Path string `json:"path,omitempty"`
	// Host overrides the HTTP Host header; defaults to the TLS server name.
	Host string `json:"host,omitempty"`
	// Secret is the HTTP-layer HMAC secret. Empty falls back to the password.
	Secret string `json:"secret,omitempty"`
	// Transport is "h2" (default) or "websocket".
	Transport string `json:"transport,omitempty"`
}
