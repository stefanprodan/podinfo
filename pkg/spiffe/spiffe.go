package spiffe

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"errors"
	"fmt"
	"strings"
	"sync"
	"sync/atomic"

	"github.com/spiffe/go-spiffe/v2/bundle/x509bundle"
	"github.com/spiffe/go-spiffe/v2/spiffeid"
	"github.com/spiffe/go-spiffe/v2/spiffetls/tlsconfig"
	"github.com/spiffe/go-spiffe/v2/svid/x509svid"
	"github.com/spiffe/go-spiffe/v2/workloadapi"
	"go.uber.org/zap"
)

var (
	errNotIssued = errors.New("SPIFFE X.509 SVID not issued yet")
	errClosed    = errors.New("SPIFFE Workload API source closed")
)

// Source is an X.509 SVID and bundle source backed by one Workload API client.
// It returns errors until the first SVID is issued.
type Source struct {
	addr          string
	authorizedIDs []spiffeid.ID
	logger        *zap.Logger
	options       []workloadapi.ClientOption

	ctx    context.Context //nolint:containedctx // cancels the wait for the first SVID started by Start
	cancel context.CancelFunc

	started atomic.Bool
	done    chan struct{}

	mu     sync.Mutex
	source *workloadapi.X509Source
	err    error
	closed bool
}

// NewSource returns a Source for the Workload API at addr whose server
// configs allow the authorizedIDs, or any ID in the own trust domain when empty.
func NewSource(addr string, authorizedIDs []spiffeid.ID, logger *zap.Logger, options ...workloadapi.ClientOption) *Source {
	ctx, cancel := context.WithCancel(context.Background())
	return &Source{
		addr:          addr,
		authorizedIDs: authorizedIDs,
		logger:        logger,
		options:       options,
		ctx:           ctx,
		cancel:        cancel,
		done:          make(chan struct{}),
	}
}

// Start waits for the first SVID in the background; later calls do nothing.
func (s *Source) Start() {
	if s.started.Swap(true) {
		return
	}
	go s.run()
}

func (s *Source) run() {
	defer close(s.done)

	log := &logger{logger: s.logger.Named("spiffe")}
	opts := append([]workloadapi.ClientOption{
		workloadapi.WithAddr(s.addr),
		workloadapi.WithLogger(log),
	}, s.options...)

	s.logger.Info("Waiting for an X.509 SVID from the SPIFFE Workload API", zap.String("addr", s.addr))

	source, err := workloadapi.NewX509Source(s.ctx, workloadapi.WithClientOptions(opts...))
	if err == nil {
		log.ready.Store(true)
		if _, err = source.GetX509SVID(); err != nil {
			_ = source.Close()
			source = nil
		}
	}

	s.mu.Lock()
	if s.closed && source != nil {
		_ = source.Close()
		source = nil
	}
	s.source = source
	s.err = err
	s.mu.Unlock()
}

// Wait starts the source and blocks until the first SVID is issued or Stop is called.
func (s *Source) Wait() (spiffeid.ID, error) {
	s.Start()
	<-s.done
	s.mu.Lock()
	err := s.err
	s.mu.Unlock()
	if err != nil {
		return spiffeid.ID{}, err
	}
	svid, err := s.GetX509SVID()
	if err != nil {
		return spiffeid.ID{}, err
	}
	return svid.ID, nil
}

// AuthorizedIDs returns the client IDs allowed by ServerTLSConfig.
func (s *Source) AuthorizedIDs() []spiffeid.ID {
	return s.authorizedIDs
}

// GetX509SVID implements x509svid.Source.
func (s *Source) GetX509SVID() (*x509svid.SVID, error) {
	source, err := s.x509Source()
	if err != nil {
		return nil, err
	}
	return source.GetX509SVID()
}

// GetX509BundleForTrustDomain implements x509bundle.Source.
func (s *Source) GetX509BundleForTrustDomain(td spiffeid.TrustDomain) (*x509bundle.Bundle, error) {
	source, err := s.x509Source()
	if err != nil {
		return nil, err
	}
	return source.GetX509BundleForTrustDomain(td)
}

func (s *Source) x509Source() (*workloadapi.X509Source, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	switch {
	case s.closed:
		return nil, errClosed
	case s.source == nil:
		return nil, errNotIssued
	}
	return s.source, nil
}

// ServerTLSConfig returns the mTLS server config that presents the SVID
// and authorizes clients with the source's authorized IDs.
func (s *Source) ServerTLSConfig() *tls.Config {
	return tlsconfig.MTLSServerConfig(s, s, s.authorizer(s.authorizedIDs))
}

// ClientTLSConfig returns the mTLS client config that presents the SVID
// and accepts the given server IDs, or any ID in the own trust domain when empty.
func (s *Source) ClientTLSConfig(ids []spiffeid.ID) *tls.Config {
	return tlsconfig.MTLSClientConfig(s, s, s.authorizer(ids))
}

func (s *Source) authorizer(ids []spiffeid.ID) tlsconfig.Authorizer {
	return func(id spiffeid.ID, chains [][]*x509.Certificate) error {
		svid, err := s.GetX509SVID()
		if err != nil {
			return err
		}
		return Authorizer(ids, svid.ID)(id, chains)
	}
}

// Stop aborts a pending wait for the first SVID.
func (s *Source) Stop() {
	s.cancel()
}

// Close stops the source and closes the Workload API client.
func (s *Source) Close() error {
	s.Stop()
	if s.started.Load() {
		<-s.done
	}

	s.mu.Lock()
	source := s.source
	s.source = nil
	s.closed = true
	s.mu.Unlock()

	if source != nil {
		return source.Close()
	}
	return nil
}

// ParseIDs parses the SPIFFE IDs set with flag, skipping empty values.
func ParseIDs(flag string, values []string) ([]spiffeid.ID, error) {
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

// Authorizer allows the given peer IDs, or any ID in the
// own trust domain when the list is empty.
func Authorizer(ids []spiffeid.ID, own spiffeid.ID) tlsconfig.Authorizer {
	if len(ids) > 0 {
		return tlsconfig.AuthorizeOneOf(ids...)
	}
	return tlsconfig.AuthorizeMemberOf(own.TrustDomain())
}

// logger adapts zap to the go-spiffe logger.
// go-spiffe logs every failed Workload API attempt as an error; while
// waiting for the first SVID these are expected (e.g. the agent socket is
// not mounted yet or the pod has no registration entry), so they are
// logged at info until the first SVID arrives, and at warn after.
type logger struct {
	logger *zap.Logger
	ready  atomic.Bool
}

func (l *logger) Debugf(format string, args ...any) {
	l.logger.Debug(fmt.Sprintf(format, args...))
}

func (l *logger) Infof(format string, args ...any) {
	l.logger.Info(fmt.Sprintf(format, args...))
}

func (l *logger) Warnf(format string, args ...any) {
	l.logger.Warn(fmt.Sprintf(format, args...))
}

func (l *logger) Errorf(format string, args ...any) {
	if l.ready.Load() {
		l.logger.Warn(fmt.Sprintf(format, args...))
		return
	}
	l.logger.Info(fmt.Sprintf(format, args...))
}
