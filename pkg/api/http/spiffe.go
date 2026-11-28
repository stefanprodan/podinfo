// Copyright 2026 Stefan Prodan.
// SPDX-License-Identifier: Apache-2.0

package http

import (
	"errors"
	"net"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"sync/atomic"

	"github.com/spiffe/go-spiffe/v2/spiffeid"
	"github.com/spiffe/go-spiffe/v2/svid/x509svid"
	"github.com/spiffe/go-spiffe/v2/workloadapi"
	"go.uber.org/zap"

	"github.com/stefanprodan/podinfo/pkg/spiffe"
)

// spiffeState holds the SPIFFE mTLS state of the secure server.
type spiffeState struct {
	source *spiffe.Source

	// backendMTLS enables SPIFFE mTLS for calls to https backends.
	backendMTLS bool

	// backendIDs are the server IDs accepted from https backends.
	// When empty, any ID in the server's own trust domain is accepted.
	backendIDs []spiffeid.ID

	// listen opens the listener of the secure server.
	listen func(network, address string) (net.Listener, error)

	mu sync.Mutex
	// backendTransport presents the SVID to https backends.
	backendTransport *http.Transport
	// backendClient calls https backends through backendTransport.
	backendClient *http.Client
	// stopped is set by Stop, after which the server never becomes ready.
	stopped bool
	// closed is set by Close, after which backend mTLS calls fail.
	closed bool
}

// newSPIFFEState validates the SPIFFE flags and returns nil when SPIFFE is disabled.
func newSPIFFEState(config *Config, logger *zap.Logger, options ...workloadapi.ClientOption) (*spiffeState, error) {
	if config.SPIFFEBackendMTLS && config.SPIFFEEndpointSocket == "" {
		return nil, errors.New("--spiffe-backend-mtls requires --spiffe-endpoint-socket")
	}
	if len(config.SPIFFEBackendIDs) > 0 && !config.SPIFFEBackendMTLS {
		return nil, errors.New("--spiffe-backend-ids requires --spiffe-backend-mtls")
	}
	if config.SPIFFEEndpointSocket == "" {
		return nil, nil
	}

	if config.SecurePort == "" || config.SecurePort == "0" {
		return nil, errors.New("--spiffe-endpoint-socket requires --secure-port")
	}

	ids, err := spiffe.ParseIDs("--spiffe-authorized-ids", config.SPIFFEAuthorizedIDs)
	if err != nil {
		return nil, err
	}

	backendIDs, err := spiffe.ParseIDs("--spiffe-backend-ids", config.SPIFFEBackendIDs)
	if err != nil {
		return nil, err
	}

	return &spiffeState{
		source:      spiffe.NewSource(config.SPIFFEEndpointSocket, ids, logger, options...),
		backendMTLS: config.SPIFFEBackendMTLS,
		backendIDs:  backendIDs,
		listen:      net.Listen,
	}, nil
}

// SPIFFESource returns the SPIFFE source of the secure server, or nil when SPIFFE is disabled.
func (s *Server) SPIFFESource() *spiffe.Source {
	if s.spiffe == nil {
		return nil
	}
	return s.spiffe.source
}

// serveSPIFFE waits for the SVID, then serves srv with SPIFFE mTLS.
// The server is marked ready only after the TLS listener is up.
func (s *Server) serveSPIFFE(srv *http.Server) {
	st := s.spiffe

	id, err := st.source.Wait()
	if err != nil {
		if st.isStopped() {
			return
		}
		s.logger.Fatal("SPIFFE Workload API failed", zap.Error(err))
	}

	// The client config reads the SVID and the bundle from the source on
	// every handshake, so rotation applies to backend calls too.
	if st.backendMTLS {
		transport := newHTTPTransport()
		transport.TLSClientConfig = st.source.ClientTLSConfig(st.backendIDs)
		st.mu.Lock()
		if !st.closed {
			st.backendTransport = transport
			st.backendClient = newBackendClient(transport)
		}
		st.mu.Unlock()
	}
	srv.TLSConfig = st.source.ServerTLSConfig()

	ln, err := st.listen("tcp", srv.Addr)
	if err != nil {
		s.logger.Fatal("HTTPS server crashed", zap.Error(err))
	}

	authorized := make([]string, 0, len(st.source.AuthorizedIDs()))
	for _, a := range st.source.AuthorizedIDs() {
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

func (st *spiffeState) isStopped() bool {
	st.mu.Lock()
	defer st.mu.Unlock()
	return st.stopped
}

// markReady sets the ready state unless --unready is set or shutdown has started.
func (st *spiffeState) markReady(unready bool) {
	st.mu.Lock()
	defer st.mu.Unlock()
	if !unready && !st.stopped {
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
	s.spiffe.stopped = true
	s.spiffe.mu.Unlock()
	s.spiffe.source.Stop()
}

func (s *Server) closeSPIFFE() error {
	st := s.spiffe
	if st == nil {
		return nil
	}
	s.Stop()

	st.mu.Lock()
	transport := st.backendTransport
	st.backendTransport = nil
	st.backendClient = nil
	st.closed = true
	st.mu.Unlock()

	if transport != nil {
		transport.CloseIdleConnections()
	}
	return st.source.Close()
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

// backendClient returns the client for a backend call. With
// --spiffe-backend-mtls, https backends are called with SPIFFE mTLS, which
// fails until the SVID is issued. Other backends use the shared default client.
func (s *Server) backendClient(backendURL string) (*http.Client, error) {
	if s.spiffe == nil || !s.spiffe.backendMTLS {
		return s.backend, nil
	}
	u, err := url.Parse(backendURL)
	if err != nil {
		return nil, err
	}
	if !strings.EqualFold(u.Scheme, "https") {
		return s.backend, nil
	}

	s.spiffe.mu.Lock()
	defer s.spiffe.mu.Unlock()
	if s.spiffe.closed {
		return nil, errors.New("SPIFFE Workload API source closed")
	}
	if s.spiffe.backendClient == nil {
		return nil, errors.New("SPIFFE X.509 SVID not issued yet")
	}
	return s.spiffe.backendClient, nil
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
