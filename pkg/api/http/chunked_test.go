package http

import (
	"context"
	"net/http"
	"net/http/httptest"
	"regexp"
	"testing"
	"time"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"go.uber.org/zap/zaptest/observer"
)

func TestChunkedHandler(t *testing.T) {
	req, err := http.NewRequest("GET", "/chunked/0", nil)
	if err != nil {
		t.Fatal(err)
	}

	rr := httptest.NewRecorder()
	srv := NewMockServer()

	srv.router.HandleFunc("/chunked/{wait}", srv.chunkedHandler)
	srv.router.ServeHTTP(rr, req)

	if status := rr.Code; status != http.StatusOK {
		t.Errorf("handler returned wrong status code: got %v want %v",
			status, http.StatusOK)
	}
	if ct := rr.Result().Header.Get("Content-Type"); ct != "application/json; charset=utf-8" {
		t.Errorf("got Content-Type %q, want application/json; charset=utf-8", ct)
	}

	expected := ".*delay.*0.*"
	r := regexp.MustCompile(expected)
	if !r.MatchString(rr.Body.String()) {
		t.Fatalf("handler returned unexpected body:\ngot \n%v \nwant \n%s",
			rr.Body.String(), expected)
	}
}

func TestChunkedHandler_HeadersBeforeBody(t *testing.T) {
	srv := NewMockServer()
	srv.router.HandleFunc("/chunked/{wait}", srv.chunkedHandler)
	ts := httptest.NewServer(srv.router)
	defer ts.Close()

	resp, err := ts.Client().Get(ts.URL + "/chunked/60")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("got status %d, want %d", resp.StatusCode, http.StatusOK)
	}
	if ct := resp.Header.Get("Content-Type"); ct != "application/json; charset=utf-8" {
		t.Errorf("got Content-Type %q, want application/json; charset=utf-8", ct)
	}
}

func TestChunkedHandlerCanceled(t *testing.T) {
	logs, observed := observer.New(zapcore.ErrorLevel)
	srv := NewMockServer()
	srv.logger = zap.New(logs)
	srv.router.HandleFunc("/chunked/{wait}", srv.chunkedHandler)

	ctx, cancel := context.WithCancel(context.Background())
	time.AfterFunc(50*time.Millisecond, cancel)
	req := httptest.NewRequest("GET", "/chunked/60", nil).WithContext(ctx)
	rr := httptest.NewRecorder()

	start := time.Now()
	srv.router.ServeHTTP(rr, req)

	if elapsed := time.Since(start); elapsed > 10*time.Second {
		t.Errorf("handler took %s after cancellation, want prompt return", elapsed)
	}
	if rr.Code != http.StatusOK || !rr.Flushed {
		t.Errorf("got status %d, flushed %v, want %d flushed before the wait", rr.Code, rr.Flushed, http.StatusOK)
	}
	if rr.Body.Len() != 0 {
		t.Errorf("got body %q, want none", rr.Body.String())
	}
	if n := observed.Len(); n != 0 {
		t.Errorf("got %d errors for a canceled request: %v", n, observed.All())
	}
}

// TestRandomDelaySeconds covers the delay of the bare /chunked route: every
// timeout, even zero or negative, must give a delay of at least 10s and not
// make rand.Intn panic.
func TestRandomDelaySeconds(t *testing.T) {
	timeouts := []time.Duration{30 * time.Second, 12 * time.Second, time.Second, 0, -1}
	for _, timeout := range timeouts {
		for range 100 {
			d := randomDelaySeconds(timeout)
			if d < 10 {
				t.Fatalf("timeout %s: delay %d below floor of 10", timeout, d)
			}
		}
	}
}
