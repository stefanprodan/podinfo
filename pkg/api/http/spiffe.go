package http

import (
	"context"
	"crypto/tls"
	"fmt"
	"net"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"sync/atomic"

	"github.com/spiffe/go-spiffe/v2/spiffeid"
	"github.com/spiffe/go-spiffe/v2/spiffetls/tlsconfig"
	"github.com/spiffe/go-spiffe/v2/svid/x509svid"
	"github.com/spiffe/go-spiffe/v2/workloadapi"
	"go.uber.org/zap"
)

// spiffeState holds the SPIFFE mTLS state of the secure server.
type spiffeState struct {
	// authorizedIDs are the client IDs allowed to connect.
	// When empty, any ID in the server's own trust domain is allowed.
	authorizedIDs []spiffeid.ID

	// backendMTLS enables SPIFFE mTLS for calls to https backends.
	backendMTLS bool

	// backendIDs are the server IDs accepted from https backends.
	// When empty, any ID in the server's own trust domain is accepted.
	backendIDs []spiffeid.ID

	// clientOptions are extra Workload API client options (used by tests).
	clientOptions []workloadapi.ClientOption

	// ctx bounds the wait for the first SVID; Stop cancels it.
	ctx    context.Context
	cancel context.CancelFunc

	mu     sync.Mutex
	source *workloadapi.X509Source
	// backendTransport presents the SVID to https backends.
	backendTransport *http.Transport
	// closed is set by Close, after which backend mTLS calls fail.
	closed bool

	// started is set when the secure server goroutine is launched,
	// waited is closed when that goroutine stops waiting for the SVID.
	started atomic.Bool
	waited  chan struct{}
}

// newSPIFFEState validates the SPIFFE flags and returns nil when SPIFFE is disabled.
func newSPIFFEState(config *Config) (*spiffeState, error) {
	if config.SPIFFEBackendMTLS && config.SPIFFEEndpointSocket == "" {
		return nil, fmt.Errorf("--spiffe-backend-mtls requires --spiffe-endpoint-socket")
	}
	if len(config.SPIFFEBackendIDs) > 0 && !config.SPIFFEBackendMTLS {
		return nil, fmt.Errorf("--spiffe-backend-ids requires --spiffe-backend-mtls")
	}
	if config.SPIFFEEndpointSocket == "" {
		return nil, nil
	}

	if config.SecurePort == "" || config.SecurePort == "0" {
		return nil, fmt.Errorf("--spiffe-endpoint-socket requires --secure-port")
	}

	ids, err := parseSPIFFEIDs("--spiffe-authorized-ids", config.SPIFFEAuthorizedIDs)
	if err != nil {
		return nil, err
	}

	backendIDs, err := parseSPIFFEIDs("--spiffe-backend-ids", config.SPIFFEBackendIDs)
	if err != nil {
		return nil, err
	}

	ctx, cancel := context.WithCancel(context.Background())
	return &spiffeState{
		authorizedIDs: ids,
		backendMTLS:   config.SPIFFEBackendMTLS,
		backendIDs:    backendIDs,
		ctx:           ctx,
		cancel:        cancel,
		waited:        make(chan struct{}),
	}, nil
}

func parseSPIFFEIDs(flag string, values []string) ([]spiffeid.ID, error) {
	var ids []spiffeid.ID
	for _, v := range values {
		v = strings.TrimSpace(v)
		if v == "" {
			continue
		}
		id, err := spiffeid.FromString(v)
		if err != nil {
			return nil, fmt.Errorf("invalid SPIFFE ID %q in %s: %w", v, flag, err)
		}
		ids = append(ids, id)
	}
	return ids, nil
}

// spiffeAuthorizer allows the given peer IDs, or any ID in the
// server's own trust domain when the list is empty.
func spiffeAuthorizer(ids []spiffeid.ID, own spiffeid.ID) tlsconfig.Authorizer {
	if len(ids) > 0 {
		return tlsconfig.AuthorizeOneOf(ids...)
	}
	return tlsconfig.AuthorizeMemberOf(own.TrustDomain())
}

// spiffeTLSConfig blocks until the Workload API issues an X.509 SVID,
// then returns the mTLS server config and the server's SPIFFE ID.
// It retries until the SVID arrives or Stop is called.
func (s *Server) spiffeTLSConfig() (*tls.Config, spiffeid.ID, error) {
	st := s.spiffe
	defer close(st.waited)

	log := &spiffeLogger{logger: s.logger.Named("spiffe")}
	opts := append([]workloadapi.ClientOption{
		workloadapi.WithAddr(s.config.SPIFFEEndpointSocket),
		workloadapi.WithLogger(log),
	}, st.clientOptions...)

	s.logger.Info("Waiting for an X.509 SVID from the SPIFFE Workload API",
		zap.String("addr", s.config.SPIFFEEndpointSocket))

	source, err := workloadapi.NewX509Source(st.ctx, workloadapi.WithClientOptions(opts...))
	if err != nil {
		return nil, spiffeid.ID{}, err
	}
	log.ready.Store(true)

	svid, err := source.GetX509SVID()
	if err != nil {
		_ = source.Close()
		return nil, spiffeid.ID{}, err
	}

	// The client config reads the SVID and the bundle from the source on
	// every handshake, so rotation applies to backend calls too.
	var transport *http.Transport
	if st.backendMTLS {
		transport = newHTTPTransport()
		transport.TLSClientConfig = tlsconfig.MTLSClientConfig(source, source, spiffeAuthorizer(st.backendIDs, svid.ID))
	}

	st.mu.Lock()
	st.source = source
	st.backendTransport = transport
	st.mu.Unlock()

	return tlsconfig.MTLSServerConfig(source, source, spiffeAuthorizer(st.authorizedIDs, svid.ID)), svid.ID, nil
}

// serveSPIFFE waits for the SVID, then serves srv with SPIFFE mTLS.
// The server is marked ready only after the TLS listener is up.
func (s *Server) serveSPIFFE(srv *http.Server) {
	st := s.spiffe

	tlsConfig, id, err := s.spiffeTLSConfig()
	if err != nil {
		if st.ctx.Err() != nil {
			// stopped before the SVID arrived
			return
		}
		s.logger.Fatal("SPIFFE Workload API failed", zap.Error(err))
	}
	srv.TLSConfig = tlsConfig

	ln, err := net.Listen("tcp", srv.Addr)
	if err != nil {
		s.logger.Fatal("HTTPS server crashed", zap.Error(err))
	}

	authorized := make([]string, 0, len(st.authorizedIDs))
	for _, a := range st.authorizedIDs {
		authorized = append(authorized, a.String())
	}
	s.logger.Info("Starting HTTPS Server with SPIFFE mTLS",
		zap.String("addr", srv.Addr),
		zap.String("spiffe_id", id.String()),
		zap.Strings("authorized_ids", authorized))

	st.markReady(s.config.Unready)

	if err := srv.ServeTLS(ln, "", ""); err != http.ErrServerClosed {
		s.logger.Fatal("HTTPS server crashed", zap.Error(err))
	}
}

// markReady sets the ready state unless --unready is set or shutdown has started.
func (st *spiffeState) markReady(unready bool) {
	st.mu.Lock()
	defer st.mu.Unlock()
	if !unready && st.ctx.Err() == nil {
		atomic.StoreInt32(&ready, 1)
	}
}

// Stop aborts a pending wait for the SVID and prevents the server
// from becoming ready. Call it when shutdown starts.
func (s *Server) Stop() {
	if s.spiffe == nil {
		return
	}
	s.spiffe.mu.Lock()
	s.spiffe.cancel()
	s.spiffe.mu.Unlock()
}

// Close stops the server and closes the SPIFFE Workload API source.
// Call it after the HTTPS server has been shut down.
func (s *Server) Close() error {
	st := s.spiffe
	if st == nil {
		return nil
	}
	s.Stop()
	if st.started.Load() {
		<-st.waited
	}

	st.mu.Lock()
	source := st.source
	st.source = nil
	transport := st.backendTransport
	st.backendTransport = nil
	st.closed = true
	st.mu.Unlock()

	if transport != nil {
		transport.CloseIdleConnections()
	}
	if source != nil {
		return source.Close()
	}
	return nil
}

// newHTTPTransport clones the default transport, or builds one with the
// same proxy and HTTP/2 settings when http.DefaultTransport was replaced.
func newHTTPTransport() *http.Transport {
	if t, ok := http.DefaultTransport.(*http.Transport); ok {
		return t.Clone()
	}
	return &http.Transport{
		Proxy:             http.ProxyFromEnvironment,
		ForceAttemptHTTP2: true,
	}
}

// backendTransport returns the transport for a backend call. With
// --spiffe-backend-mtls, https backends are called with SPIFFE mTLS, which
// fails until the SVID is issued. Other backends use the default transport.
func (s *Server) backendTransport(backendURL string) (http.RoundTripper, error) {
	if s.spiffe == nil || !s.spiffe.backendMTLS {
		return http.DefaultTransport, nil
	}
	u, err := url.Parse(backendURL)
	if err != nil {
		return nil, err
	}
	if !strings.EqualFold(u.Scheme, "https") {
		return http.DefaultTransport, nil
	}

	s.spiffe.mu.Lock()
	defer s.spiffe.mu.Unlock()
	if s.spiffe.closed {
		return nil, fmt.Errorf("SPIFFE Workload API source closed")
	}
	if s.spiffe.backendTransport == nil {
		return nil, fmt.Errorf("SPIFFE X.509 SVID not issued yet")
	}
	return s.spiffe.backendTransport, nil
}

// peerSPIFFEID returns the SPIFFE ID of the TLS client certificate, if any.
func peerSPIFFEID(r *http.Request) (spiffeid.ID, bool) {
	if r.TLS == nil || len(r.TLS.PeerCertificates) == 0 {
		return spiffeid.ID{}, false
	}
	id, err := x509svid.IDFromCert(r.TLS.PeerCertificates[0])
	if err != nil {
		return spiffeid.ID{}, false
	}
	return id, true
}

// spiffeLogger adapts zap to the go-spiffe logger.
// go-spiffe logs every failed Workload API attempt as an error; while
// waiting for the first SVID these are expected (e.g. the agent socket is
// not mounted yet or the pod has no registration entry), so they are
// logged at info until the first SVID arrives, and at warn after.
type spiffeLogger struct {
	logger *zap.Logger
	ready  atomic.Bool
}

func (l *spiffeLogger) Debugf(format string, args ...any) {
	l.logger.Debug(fmt.Sprintf(format, args...))
}

func (l *spiffeLogger) Infof(format string, args ...any) {
	l.logger.Info(fmt.Sprintf(format, args...))
}

func (l *spiffeLogger) Warnf(format string, args ...any) {
	l.logger.Warn(fmt.Sprintf(format, args...))
}

func (l *spiffeLogger) Errorf(format string, args ...any) {
	if l.ready.Load() {
		l.logger.Warn(fmt.Sprintf(format, args...))
		return
	}
	l.logger.Info(fmt.Sprintf(format, args...))
}
