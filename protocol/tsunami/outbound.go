package tsunami

import (
	"context"
	"net"
	"os"
	"strings"

	"github.com/sagernet/sing-box/adapter"
	"github.com/sagernet/sing-box/adapter/outbound"
	"github.com/sagernet/sing-box/common/dialer"
	"github.com/sagernet/sing-box/common/tls"
	C "github.com/sagernet/sing-box/constant"
	"github.com/sagernet/sing-box/log"
	"github.com/sagernet/sing-box/option"
	"github.com/sagernet/sing/common"
	E "github.com/sagernet/sing/common/exceptions"
	"github.com/sagernet/sing/common/json/badoption"
	M "github.com/sagernet/sing/common/metadata"
	N "github.com/sagernet/sing/common/network"
	"github.com/sagernet/sing/common/uot"

	singtsunami "github.com/RavenholmAlpha/sing-tsunami"
	tsfronting "github.com/RavenholmAlpha/sing-tsunami/fronting"
)

func RegisterOutbound(registry *outbound.Registry) {
	outbound.Register[option.TsunamiOutboundOptions](registry, C.TypeTsunami, NewOutbound)
}

var _ adapter.OutboundWithMultiplex = (*Outbound)(nil)

type Outbound struct {
	outbound.Adapter
	ctx           context.Context
	dialer        tls.Dialer
	server        M.Socksaddr
	tlsConfig     tls.Config
	fronting      tsfronting.Config
	client        *singtsunami.Client
	uotClient     *uot.Client
	options       option.TsunamiOutboundOptions
	paddingScheme []byte
	logger        log.ContextLogger
}

func NewOutbound(ctx context.Context, router adapter.Router, logger log.ContextLogger, tag string, options option.TsunamiOutboundOptions) (adapter.Outbound, error) {
	outbound := &Outbound{
		Adapter: outbound.NewAdapterWithDialerOptions(C.TypeTsunami, tag, []string{N.NetworkTCP, N.NetworkUDP}, options.DialerOptions),
		ctx:     ctx,
		server:  options.ServerOptions.Build(),
		logger:  logger,
	}
	if options.TLS == nil || !options.TLS.Enabled {
		return nil, C.ErrTLSRequired
	}
	// TCP Fast Open creates a lazy connection that only establishes on first
	// write; the TSUNAMI handshake needs an established connection eagerly.
	if options.DialerOptions.TCPFastOpen {
		return nil, E.New("tcp_fast_open is not supported with tsunami outbound")
	}

	if options.Fronting != nil && options.Fronting.Enabled {
		transport := options.Fronting.Transport
		if transport == "" {
			transport = "h2"
		}
		host := options.Fronting.Host
		if host == "" {
			host = options.Server
		}
		secret := options.Fronting.Secret
		if secret == "" {
			secret = options.Password
		}
		outbound.fronting = tsfronting.Config{
			Enabled:   true,
			Path:      options.Fronting.Path,
			Host:      host,
			Secret:    secret,
			Transport: transport,
		}
		if err := outbound.fronting.Normalize(); err != nil {
			return nil, E.Cause(err, "tsunami fronting")
		}
		// Keep the fronting ClientHello consistent with the negotiated HTTP
		// protocol unless the user set ALPN explicitly.
		if len(options.TLS.ALPN) == 0 {
			if transport == "websocket" {
				options.TLS.ALPN = []string{"http/1.1"}
			} else {
				options.TLS.ALPN = []string{"h2"}
			}
		}
	}

	tlsConfig, err := tls.NewClient(ctx, logger, options.Server, common.PtrValueOrDefault(options.TLS))
	if err != nil {
		return nil, err
	}
	outbound.tlsConfig = tlsConfig

	outboundDialer, err := dialer.NewWithOptions(dialer.Options{
		Context:        ctx,
		Options:        options.DialerOptions,
		RemoteIsDomain: options.ServerIsDomain(),
	})
	if err != nil {
		return nil, err
	}
	outbound.dialer = tls.NewDialer(outboundDialer, tlsConfig)
	outbound.paddingScheme = paddingScheme(options.PaddingScheme)
	outbound.options = options
	return outbound, nil
}

func (h *Outbound) Start(stage adapter.StartStage, scope *adapter.Scope) error {
	if stage != adapter.StartStateInitialize {
		return nil
	}
	client, err := singtsunami.NewClient(h.ctx, singtsunami.ClientConfig{
		Password:                 h.options.Password,
		DialOut:                  h.dialOut,
		PaddingScheme:            h.paddingScheme,
		IdleSessionCheckInterval: h.options.IdleSessionCheckInterval.Build(),
		IdleSessionTimeout:       h.options.IdleSessionTimeout.Build(),
		MinIdleSession:           h.options.MinIdleSession,
	})
	if err != nil {
		return err
	}
	scope.Add(func() error {
		return client.Close()
	})
	h.client = client
	h.uotClient = &uot.Client{
		Dialer:  tsunamiDialer(h.createProxy),
		Version: uot.Version,
	}
	return nil
}

func (h *Outbound) dialOut(ctx context.Context) (net.Conn, error) {
	conn, err := h.dialer.DialTLSContext(adapter.ContextForMultiplexSession(ctx), h.server)
	if err != nil {
		return nil, err
	}
	if !h.fronting.Enabled {
		return conn, nil
	}
	fronted, err := tsfronting.NewClientConn(conn, h.fronting)
	if err != nil {
		conn.Close()
		return nil, E.Cause(err, "tsunami fronting")
	}
	return fronted, nil
}

type tsunamiDialer func(ctx context.Context, destination M.Socksaddr) (net.Conn, error)

func (d tsunamiDialer) DialContext(ctx context.Context, network string, destination M.Socksaddr) (net.Conn, error) {
	return d(ctx, destination)
}

func (d tsunamiDialer) ListenPacket(ctx context.Context, destination M.Socksaddr) (net.PacketConn, error) {
	return nil, os.ErrInvalid
}

func (h *Outbound) createProxy(ctx context.Context, destination M.Socksaddr) (net.Conn, error) {
	conn, err := h.client.CreateStream(ctx)
	if err != nil {
		return nil, err
	}
	err = singtsunami.WriteAddr(conn, destination.String())
	if err != nil {
		conn.Close()
		return nil, E.Cause(err, "write destination address")
	}
	return conn, nil
}

func (h *Outbound) DialContext(ctx context.Context, network string, destination M.Socksaddr) (net.Conn, error) {
	ctx, metadata := adapter.ExtendContext(ctx)
	metadata.Outbound = h.Tag()
	metadata.Destination = destination
	switch N.NetworkName(network) {
	case N.NetworkTCP:
		h.logger.InfoContext(ctx, "outbound connection to ", destination)
		return h.createProxy(ctx, destination)
	case N.NetworkUDP:
		h.logger.InfoContext(ctx, "outbound UoT packet connection to ", destination)
		return h.uotClient.DialContext(ctx, network, destination)
	}
	return nil, os.ErrInvalid
}

func (h *Outbound) ListenPacket(ctx context.Context, destination M.Socksaddr) (net.PacketConn, error) {
	ctx, metadata := adapter.ExtendContext(ctx)
	metadata.Outbound = h.Tag()
	metadata.Destination = destination
	h.logger.InfoContext(ctx, "outbound UoT packet connection to ", destination)
	return h.uotClient.ListenPacket(ctx, destination)
}

func (h *Outbound) MultiplexEnabled() bool {
	return true
}

func (h *Outbound) Close() error {
	return common.Close(h.tlsConfig)
}

func paddingScheme(scheme badoption.Listable[string]) []byte {
	if len(scheme) == 0 {
		return nil
	}
	return []byte(strings.Join(scheme, "\n"))
}
