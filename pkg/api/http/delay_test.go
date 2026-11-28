package http

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"net/http/httptest"
	"regexp"
	"testing"
	"time"
)

func TestWaitHandlersReturnOnDisconnect(t *testing.T) {
	srv := NewMockServer()
	srv.router.HandleFunc("/delay/{wait}", srv.delayHandler)
	srv.router.HandleFunc("/chunked/{wait}", srv.chunkedHandler)
	done := make(chan struct{}, 1)
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		srv.router.ServeHTTP(w, r)
		done <- struct{}{}
	}))
	defer ts.Close()

	for _, tc := range []struct {
		name, method, path string
		length             int
	}{
		{"delay partial body", "GET", "/delay/10", 100},
		{"delay full body", "GET", "/delay/10", 1},
		{"chunked partial body", "POST", "/chunked/10", 100},
		{"chunked full body", "POST", "/chunked/10", 1},
	} {
		t.Run(tc.name, func(t *testing.T) {
			conn, err := net.Dial("tcp", ts.Listener.Addr().String())
			if err != nil {
				t.Fatal(err)
			}
			fmt.Fprintf(conn, "%s %s HTTP/1.1\r\nHost: podinfo\r\nContent-Length: %d\r\n\r\nx", tc.method, tc.path, tc.length)
			time.Sleep(50 * time.Millisecond)
			conn.Close()

			select {
			case <-done:
			case <-time.After(2 * time.Second):
				t.Fatal("handler still waiting after the client disconnected")
			}
		})
	}
}

func TestDelayHandler(t *testing.T) {
	req, err := http.NewRequest("GET", "/delay/0", nil)
	if err != nil {
		t.Fatal(err)
	}

	rr := httptest.NewRecorder()
	srv := NewMockServer()

	srv.router.HandleFunc("/delay/{wait}", srv.delayHandler)
	srv.router.ServeHTTP(rr, req)

	// Check the status code is what we expect.
	if status := rr.Code; status != http.StatusOK {
		t.Errorf("handler returned wrong status code: got %v want %v",
			status, http.StatusOK)
	}

	// Check the response body is what we expect.
	expected := ".*delay.*0.*"
	r := regexp.MustCompile(expected)
	if !r.MatchString(rr.Body.String()) {
		t.Fatalf("handler returned unexpected body:\ngot \n%v \nwant \n%s",
			rr.Body.String(), expected)
	}
}

func TestDelayHandlerCanceled(t *testing.T) {
	srv := NewMockServer()
	srv.router.HandleFunc("/delay/{wait}", srv.delayHandler)

	ctx, cancel := context.WithCancel(context.Background())
	time.AfterFunc(50*time.Millisecond, cancel)
	req := httptest.NewRequest("GET", "/delay/10", nil).WithContext(ctx)
	rr := httptest.NewRecorder()

	start := time.Now()
	srv.router.ServeHTTP(rr, req)

	if elapsed := time.Since(start); elapsed > time.Second {
		t.Errorf("handler took %s after cancellation, want prompt return", elapsed)
	}
	if rr.Body.Len() != 0 {
		t.Errorf("got body %q, want none", rr.Body.String())
	}
}

func TestRandomDelayMiddleware(t *testing.T) {
	m := NewRandomDelayMiddleware(0, 1, "ms")
	called := false
	handler := m.Handler(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called = true
		w.WriteHeader(http.StatusOK)
	}))

	req, _ := http.NewRequest("GET", "/", nil)
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	if !called {
		t.Error("next handler was not called")
	}
	if rr.Code != http.StatusOK {
		t.Errorf("got status %d, want %d", rr.Code, http.StatusOK)
	}
}

func TestRandomDelayMiddleware_Milliseconds(t *testing.T) {
	m := NewRandomDelayMiddleware(0, 1, "ms")
	handler := m.Handler(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req, _ := http.NewRequest("GET", "/", nil)
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("got status %d, want %d", rr.Code, http.StatusOK)
	}
}

func TestRandomDelayMiddleware_FixedDelay(t *testing.T) {
	m := NewRandomDelayMiddleware(1, 1, "ms")
	handler := m.Handler(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req, _ := http.NewRequest("GET", "/", nil)
	rr := httptest.NewRecorder()
	start := time.Now()
	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("got status %d, want %d", rr.Code, http.StatusOK)
	}
	if elapsed := time.Since(start); elapsed < time.Millisecond {
		t.Errorf("got delay %s, want at least 1ms", elapsed)
	}
}

func TestRandomErrorMiddleware(t *testing.T) {
	handler := randomErrorMiddleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	gotOK := false
	gotError := false
	for i := 0; i < 50; i++ {
		req, _ := http.NewRequest("GET", "/", nil)
		rr := httptest.NewRecorder()
		handler.ServeHTTP(rr, req)
		if rr.Code == http.StatusOK {
			gotOK = true
		} else {
			gotError = true
		}
		if gotOK && gotError {
			break
		}
	}
	if !gotOK {
		t.Error("never got a successful response")
	}
	if !gotError {
		t.Error("never got an error response")
	}
}
