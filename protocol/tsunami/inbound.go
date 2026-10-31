package tsunami

import (
	"context"
	"net"
	"net/http"
	"sync"

	"github.com/sagernet/sing-box/adapter"
	"github.com/sagernet/sing-box/adapter/inbound"
	"github.com/sagernet/sing-box/common/listener"
	"github.com/sagernet/sing-box/common/tls"
	"github.com/sagernet/sing-box/common/uot"
	C "github.com/sagernet/sing-box/constant"
	"github.com/sagernet/sing-box/log"
	"github.com/sagernet/sing-box/option"
	"github.com/sagernet/sing/common"
	E "github.com/sagernet/sing/common/exceptions"
	M "github.com/sagernet/sing/common/metadata"
	N "github.com/sagernet/sing/common/network"

	singtsunami "github.com/RavenholmAlpha/sing-tsunami"
	tsfronting "github.com/RavenholmAlpha/sing-tsunami/fronting"
)

func RegisterInbound(registry *inbound.Registry) {
	inbound.Register[option.TsunamiInboundOptions](registry, C.TypeTsunami, NewInbound)
}

type Inbound struct {
	inbound.Adapter
	tlsConfig tls.ServerConfig
	router    adapter.ConnectionRouterEx
	logger    log.ContextLogger
	listener  *listener.Listener
	service   *singtsunami.Service
	fronting  tsfronting.Config
	decoy     http.Handler
}

func NewInbound(ctx context.Context, router adapter.Router, logger log.ContextLogger, tag string, options option.TsunamiInboundOptions) (adapter.Inbound, error) {
	inbound := &Inbound{
		Adapter: inbound.NewAdapter(C.TypeTsunami, tag),
		router:  uot.NewRouter(router, logger),
		logger:  logger,
	}
	if options.TLS == nil || !options.TLS.Enabled {
		return nil, C.ErrTLSRequired
	}
	tlsConfig, err := tls.NewServer(ctx, logger, common.PtrValueOrDefault(options.TLS))
	if err != nil {
		return nil, err
	}
	inbound.tlsConfig = tlsConfig

	if options.Fronting != nil && options.Fronting.Enabled {
		secret := options.Fronting.Secret
		if secret == "" && len(options.Users) > 0 {
			secret = options.Users[0].Password
		}
		fronting := tsfronting.Config{
			Enabled: true,
			Path:    options.Fronting.Path,
			Secret:  secret,
		}
		if options.Fronting.Site != nil {
			site := options.Fronting.Site
			fronting.Site = tsfronting.SiteConfig{
				Mode:               tsfronting.SiteMode(site.Mode),
				Upstream:           site.Upstream,
				PreserveHost:       site.PreserveHost,
				InsecureSkipVerify: site.InsecureSkipVerify,
			}
		}
		if err = fronting.Normalize(); err != nil {
			return nil, E.Cause(err, "tsunami fronting")
		}
		decoy, err := tsfronting.NewSiteHandler(fronting)
		if err != nil {
			return nil, E.Cause(err, "tsunami fronting site")
		}
		inbound.fronting = fronting
		inbound.decoy = decoy
	}

	service, err := singtsunami.NewService(singtsunami.ServiceConfig{
		Users: common.Map(options.Users, func(it option.TsunamiUser) singtsunami.User {
			return singtsunami.User{Name: it.Name, Password: it.Password}
		}),
		PaddingScheme: paddingScheme(options.PaddingScheme),
		Handler:       inbound.handleStream,
	})
	if err != nil {
		return nil, err
	}
	inbound.service = service

	inbound.listener = listener.New(listener.Options{
		Context:           ctx,
		Logger:            logger,
		Network:           []string{N.NetworkTCP},
		Listen:            options.ListenOptions,
		ConnectionHandler: inbound,
	})
	return inbound, nil
}

func (h *Inbound) Start(stage adapter.StartStage, scope *adapter.Scope) error {
	if stage != adapter.StartStateStart {
		return nil
	}
	if h.tlsConfig != nil {
		err := h.tlsConfig.Start()
		if err != nil {
			return err
		}
		scope.Add(h.tlsConfig.Close)
	}
	err := h.listener.Start()
	if err != nil {
		return err
	}
	scope.Add(h.listener.Close)
	return nil
}

func (h *Inbound) NewConnection(ctx context.Context, conn net.Conn, metadata adapter.InboundContext, onClose N.CloseHandlerFunc) {
	tlsConn, err := tls.ServerHandshake(ctx, conn, h.tlsConfig)
	if err != nil {
		N.CloseOnHandshakeFailure(conn, onClose, err)
		h.logger.ErrorContext(ctx, E.Cause(err, "process connection from ", metadata.Source, ": TLS handshake"))
		return
	}
	conn = tlsConn

	if h.fronting.Enabled {
		// The fronting layer is selected by ALPN: h2 for the default transport,
		// http/1.1 for WebSocket fronting. An empty protocol means the peer
		// speaks raw TSUNAMI over TLS (the native server's --fronting=false
		// mode also advertises no matching ALPN).
		switch tlsConn.ConnectionState().NegotiatedProtocol {
		case "h2":
			err = tsfronting.ServeHTTP2(ctx, conn, h.fronting, func(tunnel net.Conn) error {
				return h.service.NewConnection(ctx, tunnel, metadata.Source)
			}, h.decoy)
		case "http/1.1":
			err = tsfronting.ServeHTTP1(ctx, conn, h.fronting, func(tunnel net.Conn) error {
				return h.service.NewConnection(ctx, tunnel, metadata.Source)
			}, h.decoy)
		default:
			err = h.service.NewConnection(ctx, conn, metadata.Source)
		}
	} else {
		err = h.service.NewConnection(ctx, conn, metadata.Source)
	}
	if err != nil {
		N.CloseOnHandshakeFailure(conn, onClose, err)
		h.logger.ErrorContext(ctx, E.Cause(err, "process connection from ", metadata.Source))
	}
}

func (h *Inbound) handleStream(ctx context.Context, conn net.Conn, source net.Addr, destination string) {
	var metadata adapter.InboundContext
	metadata.Inbound = h.Tag()
	metadata.InboundType = h.Type()
	if source != nil {
		metadata.Source = M.SocksaddrFromNet(source).Unwrap()
	}
	metadata.Destination = M.ParseSocksaddr(destination).Unwrap()
	h.logger.InfoContext(ctx, "inbound connection to ", metadata.Destination)
	// RouteConnectionEx takes ownership of conn and returns immediately, so the
	// handler must not return until routing is done: the TSUNAMI service closes
	// the stream when the handler returns.
	done := make(chan struct{})
	var closeOnce sync.Once
	h.router.RouteConnectionEx(ctx, conn, metadata, func(error) {
		closeOnce.Do(func() {
			close(done)
		})
	})
	select {
	case <-done:
	case <-ctx.Done():
		_ = conn.Close()
	}
}
