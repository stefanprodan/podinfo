package http

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/json"
	"errors"
	"io"
	"math/big"
	"net"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/spiffe/go-spiffe/v2/bundle/x509bundle"
	"github.com/spiffe/go-spiffe/v2/proto/spiffe/workload"
	"github.com/spiffe/go-spiffe/v2/spiffeid"
	"github.com/spiffe/go-spiffe/v2/spiffetls/tlsconfig"
	"github.com/spiffe/go-spiffe/v2/svid/x509svid"
	"github.com/spiffe/go-spiffe/v2/workloadapi"
	"go.uber.org/zap"
	"go.uber.org/zap/zaptest/observer"
	"google.golang.org/grpc"
)

var (
	serverID  = spiffeid.RequireFromString("spiffe://example.org/ns/default/sa/podinfo")
	clientID  = spiffeid.RequireFromString("spiffe://example.org/ns/flux-system/sa/source-watcher")
	otherID   = spiffeid.RequireFromString("spiffe://example.org/ns/default/sa/other")
	foreignID = spiffeid.RequireFromString("spiffe://evil.org/ns/default/sa/attacker")
)

// testCA issues X.509 SVIDs for a single trust domain.
type testCA struct {
	cert *x509.Certificate
	key  *ecdsa.PrivateKey
}

func newTestCA(t *testing.T) *testCA {
	t.Helper()
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	tmpl := &x509.Certificate{
		SerialNumber:          big.NewInt(1),
		Subject:               pkix.Name{CommonName: "test-ca"},
		NotBefore:             time.Now().Add(-time.Hour),
		NotAfter:              time.Now().Add(time.Hour),
		KeyUsage:              x509.KeyUsageCertSign | x509.KeyUsageCRLSign,
		BasicConstraintsValid: true,
		IsCA:                  true,
	}
	der, err := x509.CreateCertificate(rand.Reader, tmpl, tmpl, &key.PublicKey, key)
	if err != nil {
		t.Fatal(err)
	}
	cert, err := x509.ParseCertificate(der)
	if err != nil {
		t.Fatal(err)
	}
	return &testCA{cert: cert, key: key}
}

func (ca *testCA) newSVID(t *testing.T, id spiffeid.ID) *x509svid.SVID {
	t.Helper()
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	uri, err := url.Parse(id.String())
	if err != nil {
		t.Fatal(err)
	}
	serial, _ := rand.Int(rand.Reader, big.NewInt(1<<62))
	tmpl := &x509.Certificate{
		SerialNumber: serial,
		NotBefore:    time.Now().Add(-time.Hour),
		NotAfter:     time.Now().Add(time.Hour),
		KeyUsage:     x509.KeyUsageDigitalSignature,
		ExtKeyUsage:  []x509.ExtKeyUsage{x509.ExtKeyUsageClientAuth, x509.ExtKeyUsageServerAuth},
		URIs:         []*url.URL{uri},
	}
	der, err := x509.CreateCertificate(rand.Reader, tmpl, ca.cert, &key.PublicKey, ca.key)
	if err != nil {
		t.Fatal(err)
	}
	cert, err := x509.ParseCertificate(der)
	if err != nil {
		t.Fatal(err)
	}
	return &x509svid.SVID{ID: id, Certificates: []*x509.Certificate{cert}, PrivateKey: key}
}

func (ca *testCA) bundle(td spiffeid.TrustDomain) *x509bundle.Bundle {
	return x509bundle.FromX509Authorities(td, []*x509.Certificate{ca.cert})
}

// fakeWorkloadAPI serves a single X.509 SVID over the Workload API.
type fakeWorkloadAPI struct {
	workload.UnimplementedSpiffeWorkloadAPIServer
	resp *workload.X509SVIDResponse
}

func (f *fakeWorkloadAPI) FetchX509SVID(_ *workload.X509SVIDRequest,
	stream grpc.ServerStreamingServer[workload.X509SVIDResponse]) error {
	if err := stream.Send(f.resp); err != nil {
		return err
	}
	<-stream.Context().Done()
	return nil
}

func newSocketPath(t *testing.T) string {
	t.Helper()
	// unix socket paths are limited to ~104 bytes, so avoid long TMPDIRs
	base := ""
	if len(os.TempDir()) > 60 {
		base = "/tmp"
	}
	dir, err := os.MkdirTemp(base, "podinfo-spiffe")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.RemoveAll(dir) })
	return filepath.Join(dir, "api.sock")
}

// startWorkloadAPI serves svid (signed by ca) on socketPath.
func startWorkloadAPI(t *testing.T, socketPath string, ca *testCA, svid *x509svid.SVID) {
	t.Helper()
	keyDER, err := x509.MarshalPKCS8PrivateKey(svid.PrivateKey)
	if err != nil {
		t.Fatal(err)
	}
	handler := &fakeWorkloadAPI{resp: &workload.X509SVIDResponse{Svids: []*workload.X509SVID{{
		SpiffeId:    svid.ID.String(),
		X509Svid:    svid.Certificates[0].Raw,
		X509SvidKey: keyDER,
		Bundle:      ca.cert.Raw,
	}}}}

	ln, err := net.Listen("unix", socketPath)
	if err != nil {
		t.Fatal(err)
	}
	server := grpc.NewServer()
	workload.RegisterSpiffeWorkloadAPIServer(server, handler)
	go func() { _ = server.Serve(ln) }()
	t.Cleanup(server.Stop)
}

type fastBackoff struct{}

func (fastBackoff) NewBackoff() workloadapi.Backoff { return fastBackoff{} }
func (fastBackoff) Next() time.Duration             { return 10 * time.Millisecond }
func (fastBackoff) Reset()                          {}

func freePort(t *testing.T) string {
	t.Helper()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer ln.Close()
	return strconv.Itoa(ln.Addr().(*net.TCPAddr).Port)
}

// newSPIFFEServer returns a mock server with SPIFFE enabled on socketPath
// and a free secure port. The global ready state is reset for the test.
func newSPIFFEServer(t *testing.T, socketPath string, authorized ...string) *Server {
	t.Helper()
	return newSPIFFEServerWithConfig(t, socketPath, func(c *Config) {
		c.SPIFFEAuthorizedIDs = authorized
	})
}

// newSPIFFEServerWithConfig is newSPIFFEServer with a hook to set the
// config before the SPIFFE flags are validated.
func newSPIFFEServerWithConfig(t *testing.T, socketPath string, configure func(*Config)) *Server {
	t.Helper()
	srv := NewMockServer()
	srv.config.Host = "127.0.0.1"
	srv.config.SecurePort = freePort(t)
	srv.config.SPIFFEEndpointSocket = "unix://" + socketPath
	configure(srv.config)

	st, err := newSPIFFEState(srv.config)
	if err != nil {
		t.Fatal(err)
	}
	st.clientOptions = []workloadapi.ClientOption{workloadapi.WithBackoffStrategy(fastBackoff{})}
	srv.spiffe = st

	srv.registerHandlers()
	srv.handler = srv.router

	atomic.StoreInt32(&ready, 0)
	t.Cleanup(func() { atomic.StoreInt32(&ready, 0) })
	return srv
}

// startSPIFFEServer starts the secure server and stops it at the end of the test.
func startSPIFFEServer(t *testing.T, srv *Server) *http.Server {
	t.Helper()
	httpsSrv := srv.startSecureServer()
	t.Cleanup(func() {
		srv.Stop()
		_ = httpsSrv.Close()
		_ = srv.Close()
	})
	return httpsSrv
}

func mtlsClient(svid *x509svid.SVID, bundle *x509bundle.Bundle) *http.Client {
	var cfg *tls.Config
	if svid == nil {
		cfg = tlsconfig.TLSClientConfig(bundle, tlsconfig.AuthorizeID(serverID))
	} else {
		cfg = tlsconfig.MTLSClientConfig(svid, bundle, tlsconfig.AuthorizeID(serverID))
	}
	return &http.Client{
		Timeout:   5 * time.Second,
		Transport: &http.Transport{TLSClientConfig: cfg},
	}
}

func waitFor(t *testing.T, what string, cond func() bool) {
	t.Helper()
	deadline := time.Now().Add(10 * time.Second)
	for time.Now().Before(deadline) {
		if cond() {
			return
		}
		time.Sleep(20 * time.Millisecond)
	}
	t.Fatalf("timed out waiting for %s", what)
}

func isReady() bool { return atomic.LoadInt32(&ready) == 1 }

func TestParseSPIFFEIDs(t *testing.T) {
	ids, err := parseSPIFFEIDs("--spiffe-authorized-ids", []string{"spiffe://example.org/a", " ", " spiffe://example.org/b "})
	if err != nil {
		t.Fatal(err)
	}
	if len(ids) != 2 || ids[1].String() != "spiffe://example.org/b" {
		t.Fatalf("unexpected IDs: %v", ids)
	}

	for _, bad := range []string{"not-an-id", "https://example.org/a", "spiffe://"} {
		if _, err := parseSPIFFEIDs("--spiffe-authorized-ids", []string{bad}); err == nil {
			t.Errorf("parseSPIFFEIDs(%q) returned no error", bad)
		}
	}
}

func TestNewSPIFFEState(t *testing.T) {
	cfg := &Config{SecurePort: "0"}
	if st, err := newSPIFFEState(cfg); st != nil || err != nil {
		t.Fatalf("SPIFFE disabled: got %v, %v", st, err)
	}

	cfg.SPIFFEEndpointSocket = "unix:///tmp/agent.sock"
	if _, err := newSPIFFEState(cfg); err == nil {
		t.Fatal("expected an error without --secure-port")
	}

	cfg.SecurePort = "9899"
	cfg.SPIFFEAuthorizedIDs = []string{"bad"}
	if _, err := NewServer(cfg, zap.NewNop()); err == nil {
		t.Fatal("expected NewServer to reject an invalid authorized ID")
	}

	cfg.SPIFFEAuthorizedIDs = nil
	cfg.SPIFFEBackendMTLS = true
	cfg.SPIFFEBackendIDs = []string{"bad"}
	if _, err := NewServer(cfg, zap.NewNop()); err == nil {
		t.Fatal("expected NewServer to reject an invalid backend ID")
	}

	cfg.SPIFFEBackendMTLS = false
	cfg.SPIFFEBackendIDs = []string{"spiffe://example.org/backend"}
	if _, err := newSPIFFEState(cfg); err == nil {
		t.Fatal("expected an error for --spiffe-backend-ids without --spiffe-backend-mtls")
	}

	cfg = &Config{SecurePort: "9899", SPIFFEBackendMTLS: true}
	if _, err := newSPIFFEState(cfg); err == nil {
		t.Fatal("expected an error for --spiffe-backend-mtls without --spiffe-endpoint-socket")
	}

	cfg.SPIFFEEndpointSocket = "unix:///tmp/agent.sock"
	cfg.SPIFFEBackendIDs = []string{"spiffe://example.org/backend"}
	st, err := newSPIFFEState(cfg)
	if err != nil {
		t.Fatal(err)
	}
	if !st.backendMTLS || len(st.backendIDs) != 1 || st.backendIDs[0].String() != "spiffe://example.org/backend" {
		t.Fatalf("backend flags not applied: mtls=%v ids=%v", st.backendMTLS, st.backendIDs)
	}
}

// startSPIFFEBackend starts a podinfo backend serving SPIFFE mTLS with
// serverID, authorizing the given client IDs, and returns its echo URL.
func startSPIFFEBackend(t *testing.T, ca *testCA, authorized ...string) string {
	t.Helper()
	socket := newSocketPath(t)
	startWorkloadAPI(t, socket, ca, ca.newSVID(t, serverID))
	backend := newSPIFFEServer(t, socket, authorized...)
	startSPIFFEServer(t, backend)

	addr := net.JoinHostPort(backend.config.Host, backend.config.SecurePort)
	waitFor(t, "backend listener", func() bool {
		conn, err := net.Dial("tcp", addr)
		if err != nil {
			return false
		}
		conn.Close()
		return true
	})
	return "https://" + addr + "/api/echo"
}

// startSPIFFEFrontend starts a podinfo frontend with the clientID SVID that
// calls backendURL with SPIFFE mTLS, accepting the given backend IDs.
func startSPIFFEFrontend(t *testing.T, ca *testCA, backendURL string, backendIDs ...spiffeid.ID) *Server {
	t.Helper()
	socket := newSocketPath(t)
	startWorkloadAPI(t, socket, ca, ca.newSVID(t, clientID))
	frontend := newSPIFFEServerWithConfig(t, socket, func(c *Config) {
		c.SPIFFEBackendMTLS = true
		for _, id := range backendIDs {
			c.SPIFFEBackendIDs = append(c.SPIFFEBackendIDs, id.String())
		}
		c.BackendURL = []string{backendURL}
	})
	startSPIFFEServer(t, frontend)

	waitFor(t, "frontend SVID", func() bool {
		_, err := frontend.backendTransport(backendURL)
		return err == nil
	})
	return frontend
}

func callEcho(srv *Server, body string) string {
	req := httptest.NewRequest("POST", "/api/echo", strings.NewReader(body))
	rr := httptest.NewRecorder()
	http.HandlerFunc(srv.echoHandler).ServeHTTP(rr, req)
	return rr.Body.String()
}

func TestSPIFFEBackendMTLS(t *testing.T) {
	ca := newTestCA(t)

	t.Run("authorized client and backend", func(t *testing.T) {
		backendURL := startSPIFFEBackend(t, ca, clientID.String())
		frontend := startSPIFFEFrontend(t, ca, backendURL, serverID)
		if got := callEcho(frontend, "hello"); !strings.Contains(got, "hello") || strings.Contains(got, "call failed") {
			t.Fatalf("expected the backend to echo the payload, got: %s", got)
		}
	})

	t.Run("same trust domain default", func(t *testing.T) {
		backendURL := startSPIFFEBackend(t, ca)
		frontend := startSPIFFEFrontend(t, ca, backendURL)
		if got := callEcho(frontend, "hello"); !strings.Contains(got, "hello") || strings.Contains(got, "call failed") {
			t.Fatalf("expected a same trust domain backend to be accepted, got: %s", got)
		}
	})

	t.Run("backend rejects the client", func(t *testing.T) {
		backendURL := startSPIFFEBackend(t, ca, otherID.String())
		frontend := startSPIFFEFrontend(t, ca, backendURL, serverID)
		if got := callEcho(frontend, "hello"); !strings.Contains(got, "call failed") {
			t.Fatalf("expected the backend to reject the client, got: %s", got)
		}
	})

	t.Run("client rejects the backend", func(t *testing.T) {
		backendURL := startSPIFFEBackend(t, ca, clientID.String())
		frontend := startSPIFFEFrontend(t, ca, backendURL, otherID)
		if got := callEcho(frontend, "hello"); !strings.Contains(got, "call failed") {
			t.Fatalf("expected the client to reject the backend ID, got: %s", got)
		}
	})

	t.Run("foreign trust domain backend", func(t *testing.T) {
		backendURL := startSPIFFEBackend(t, newTestCA(t), clientID.String())
		frontend := startSPIFFEFrontend(t, ca, backendURL, serverID)
		if got := callEcho(frontend, "hello"); !strings.Contains(got, "call failed") {
			t.Fatalf("expected the client to reject a backend from another CA, got: %s", got)
		}
	})
}

func TestSPIFFEBackendTransport(t *testing.T) {
	srv := newSPIFFEServerWithConfig(t, newSocketPath(t), func(c *Config) {
		c.SPIFFEBackendMTLS = true
	})

	if _, err := srv.backendTransport("https://backend:9899/api/echo"); err == nil {
		t.Fatal("expected an error before the SVID is issued")
	}

	// http backends keep the default transport
	plain := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`"plain"`))
	}))
	defer plain.Close()
	srv.config.BackendURL = []string{plain.URL}
	if got := callEcho(srv, "hello"); !strings.Contains(got, "plain") {
		t.Fatalf("expected the http backend to be called, got: %s", got)
	}

	srv.config.BackendURL = []string{"https://127.0.0.1:1/api/echo"}
	if got := callEcho(srv, "hello"); !strings.Contains(got, "not issued yet") {
		t.Fatalf("expected the https backend call to wait for the SVID, got: %s", got)
	}
}

func TestSPIFFEAuthorizer(t *testing.T) {
	cases := []struct {
		name    string
		ids     []spiffeid.ID
		peer    spiffeid.ID
		allowed bool
	}{
		{name: "listed ID", ids: []spiffeid.ID{clientID}, peer: clientID, allowed: true},
		{name: "unlisted ID", ids: []spiffeid.ID{clientID}, peer: otherID, allowed: false},
		{name: "same trust domain", peer: otherID, allowed: true},
		{name: "foreign trust domain", peer: foreignID, allowed: false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			err := spiffeAuthorizer(c.ids, serverID)(c.peer, nil)
			if c.allowed && err != nil {
				t.Fatalf("expected %s to be allowed: %v", c.peer, err)
			}
			if !c.allowed && err == nil {
				t.Fatalf("expected %s to be rejected", c.peer)
			}
		})
	}
}

func TestSPIFFEMTLS(t *testing.T) {
	ca := newTestCA(t)
	bundle := ca.bundle(serverID.TrustDomain())
	socket := newSocketPath(t)
	startWorkloadAPI(t, socket, ca, ca.newSVID(t, serverID))

	srv := newSPIFFEServer(t, socket, clientID.String())
	startSPIFFEServer(t, srv)
	waitFor(t, "ready", isReady)

	infoURL := "https://" + srv.config.Host + ":" + srv.config.SecurePort + "/headers"

	t.Run("authorized client", func(t *testing.T) {
		resp, err := mtlsClient(ca.newSVID(t, clientID), bundle).Get(infoURL)
		if err != nil {
			t.Fatal(err)
		}
		defer resp.Body.Close()
		if resp.StatusCode != http.StatusOK {
			t.Fatalf("got status %d", resp.StatusCode)
		}
		var headers map[string][]string
		if err := json.NewDecoder(resp.Body).Decode(&headers); err != nil {
			t.Fatal(err)
		}
		if got := headers[spiffeIDHeader]; len(got) != 1 || got[0] != clientID.String() {
			t.Fatalf("X-Spiffe-Id = %v, want %s", got, clientID)
		}
	})

	t.Run("unauthorized client", func(t *testing.T) {
		resp, err := mtlsClient(ca.newSVID(t, otherID), bundle).Get(infoURL)
		if err == nil {
			resp.Body.Close()
			t.Fatal("expected the request to be rejected")
		}
	})

	t.Run("no client certificate", func(t *testing.T) {
		resp, err := mtlsClient(nil, bundle).Get(infoURL)
		if err == nil {
			resp.Body.Close()
			t.Fatal("expected the request to be rejected")
		}
	})

	t.Run("foreign trust domain", func(t *testing.T) {
		evil := newTestCA(t)
		client := mtlsClient(evil.newSVID(t, foreignID), bundle)
		resp, err := client.Get(infoURL)
		if err == nil {
			resp.Body.Close()
			t.Fatal("expected the request to be rejected")
		}
	})
}

func TestSPIFFEMTLS_TrustDomainDefault(t *testing.T) {
	ca := newTestCA(t)
	bundle := ca.bundle(serverID.TrustDomain())
	socket := newSocketPath(t)
	startWorkloadAPI(t, socket, ca, ca.newSVID(t, serverID))

	srv := newSPIFFEServer(t, socket)
	startSPIFFEServer(t, srv)
	waitFor(t, "ready", isReady)

	resp, err := mtlsClient(ca.newSVID(t, otherID), bundle).Get(
		"https://" + srv.config.Host + ":" + srv.config.SecurePort + "/api/info")
	if err != nil {
		t.Fatalf("expected a same trust domain client to be allowed: %v", err)
	}
	io.Copy(io.Discard, resp.Body)
	resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("got status %d", resp.StatusCode)
	}
}

func TestSPIFFEReadiness(t *testing.T) {
	ca := newTestCA(t)
	socket := newSocketPath(t)

	// no Workload API yet: the server keeps retrying and stays not ready
	srv := newSPIFFEServer(t, socket)
	startSPIFFEServer(t, srv)

	time.Sleep(300 * time.Millisecond)
	rr := httptest.NewRecorder()
	srv.readyzHandler(rr, httptest.NewRequest("GET", "/readyz", nil))
	if rr.Code != http.StatusServiceUnavailable {
		t.Fatalf("/readyz before the SVID: got %d, want 503", rr.Code)
	}

	// the agent comes up and issues the SVID
	startWorkloadAPI(t, socket, ca, ca.newSVID(t, serverID))
	waitFor(t, "ready", isReady)

	rr = httptest.NewRecorder()
	srv.readyzHandler(rr, httptest.NewRequest("GET", "/readyz", nil))
	if rr.Code != http.StatusOK {
		t.Fatalf("/readyz after the SVID: got %d, want 200", rr.Code)
	}
}

func TestSPIFFEReadiness_Unready(t *testing.T) {
	ca := newTestCA(t)
	bundle := ca.bundle(serverID.TrustDomain())
	socket := newSocketPath(t)
	startWorkloadAPI(t, socket, ca, ca.newSVID(t, serverID))

	srv := newSPIFFEServer(t, socket)
	srv.config.Unready = true
	startSPIFFEServer(t, srv)

	client := mtlsClient(ca.newSVID(t, clientID), bundle)
	waitFor(t, "the TLS listener", func() bool {
		resp, err := client.Get("https://" + srv.config.Host + ":" + srv.config.SecurePort + "/healthz")
		if err != nil {
			return false
		}
		resp.Body.Close()
		return true
	})
	if isReady() {
		t.Fatal("--unready must keep the server not ready")
	}
}

func TestSPIFFEStopWhileWaiting(t *testing.T) {
	socket := newSocketPath(t)
	srv := newSPIFFEServer(t, socket)
	httpsSrv := srv.startSecureServer()

	time.Sleep(100 * time.Millisecond)

	done := make(chan error, 1)
	go func() {
		srv.Stop()
		_ = httpsSrv.Close()
		done <- srv.Close()
	}()

	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("Close returned an error: %v", err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("Close hung while waiting for the SVID")
	}
	if isReady() {
		t.Fatal("server became ready after Stop")
	}
}

func TestSPIFFEStopBeforeReady(t *testing.T) {
	ca := newTestCA(t)
	socket := newSocketPath(t)
	startWorkloadAPI(t, socket, ca, ca.newSVID(t, serverID))

	srv := newSPIFFEServer(t, socket)
	// shutdown started before the server got its SVID
	srv.Stop()
	httpsSrv := srv.startSecureServer()

	<-srv.spiffe.waited
	_ = httpsSrv.Close()
	if err := srv.Close(); err != nil {
		t.Fatal(err)
	}
	if isReady() {
		t.Fatal("server became ready after Stop")
	}
}

func TestSPIFFELogger(t *testing.T) {
	core, logs := observer.New(zap.DebugLevel)
	l := &spiffeLogger{logger: zap.New(core)}

	l.Errorf("Failed to watch the Workload API: %v", errors.New("no identity issued"))
	l.ready.Store(true)
	l.Errorf("Failed to watch the Workload API: %v", errors.New("connection reset"))

	entries := logs.All()
	if len(entries) != 2 {
		t.Fatalf("got %d log entries, want 2", len(entries))
	}
	if entries[0].Level != zap.InfoLevel {
		t.Errorf("error before the first SVID logged at %s, want info", entries[0].Level)
	}
	if entries[1].Level != zap.WarnLevel {
		t.Errorf("error after the first SVID logged at %s, want warn", entries[1].Level)
	}
}

func TestLoggingMiddleware_SPIFFEID(t *testing.T) {
	ca := newTestCA(t)
	svid := ca.newSVID(t, clientID)

	core, logs := observer.New(zap.DebugLevel)
	handler := NewLoggingMiddleware(zap.New(core)).Handler(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {}))

	req := httptest.NewRequest("GET", "/artifact", nil)
	req.TLS = &tls.ConnectionState{PeerCertificates: svid.Certificates}
	handler.ServeHTTP(httptest.NewRecorder(), req)

	req = httptest.NewRequest("GET", "/artifact", nil)
	handler.ServeHTTP(httptest.NewRecorder(), req)

	entries := logs.All()
	if len(entries) != 2 {
		t.Fatalf("got %d log entries, want 2", len(entries))
	}
	if got := entries[0].ContextMap()["spiffe_id"]; got != clientID.String() {
		t.Errorf("spiffe_id = %v, want %s", got, clientID)
	}
	if _, ok := entries[1].ContextMap()["spiffe_id"]; ok {
		t.Error("spiffe_id logged for a request without a client certificate")
	}
}

func TestEchoHeadersHandler_SPIFFEID(t *testing.T) {
	srv := NewMockServer()

	// a client-supplied header is never echoed
	req := httptest.NewRequest("GET", "/headers", nil)
	req.Header.Set(spiffeIDHeader, "spiffe://evil.org/spoofed")
	rr := httptest.NewRecorder()
	srv.echoHeadersHandler(rr, req)

	var headers map[string][]string
	if err := json.Unmarshal(rr.Body.Bytes(), &headers); err != nil {
		t.Fatal(err)
	}
	if got, ok := headers[spiffeIDHeader]; ok {
		t.Fatalf("spoofed X-Spiffe-Id echoed: %v", got)
	}
	if req.Header.Get(spiffeIDHeader) == "" {
		t.Fatal("handler modified the request headers")
	}
}

func TestSPIFFEBackendMTLSDisabled(t *testing.T) {
	ca := newTestCA(t)
	socket := newSocketPath(t)
	startWorkloadAPI(t, socket, ca, ca.newSVID(t, clientID))
	srv := newSPIFFEServer(t, socket)
	startSPIFFEServer(t, srv)
	waitFor(t, "server ready", isReady)

	// without --spiffe-backend-mtls, https backends use the default transport
	rt, err := srv.backendTransport("https://backend:9899/api/echo")
	if err != nil {
		t.Fatal(err)
	}
	if rt != http.DefaultTransport {
		t.Fatalf("expected the default transport, got %T", rt)
	}
}

func TestSPIFFEBackendTransportClosed(t *testing.T) {
	ca := newTestCA(t)
	socket := newSocketPath(t)
	startWorkloadAPI(t, socket, ca, ca.newSVID(t, clientID))
	srv := newSPIFFEServerWithConfig(t, socket, func(c *Config) {
		c.SPIFFEBackendMTLS = true
	})
	httpsSrv := startSPIFFEServer(t, srv)

	backendURL := "https://backend:9899/api/echo"
	waitFor(t, "SVID", func() bool {
		_, err := srv.backendTransport(backendURL)
		return err == nil
	})

	srv.Stop()
	_ = httpsSrv.Close()
	if err := srv.Close(); err != nil {
		t.Fatal(err)
	}
	if _, err := srv.backendTransport(backendURL); err == nil || !strings.Contains(err.Error(), "closed") {
		t.Fatalf("expected a closed error, got %v", err)
	}
}
