package http

import (
	"bufio"
	"net"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"
	"time"
)

func TestCacheWriteHandler_NoPool(t *testing.T) {
	srv := NewMockServer()
	srv.pool = nil
	srv.router.HandleFunc("/cache/{key}", srv.cacheWriteHandler)

	req, _ := http.NewRequest("POST", "/cache/key1", strings.NewReader("value1"))
	rr := httptest.NewRecorder()
	srv.router.ServeHTTP(rr, req)

	if !strings.Contains(rr.Body.String(), "cache server is offline") {
		t.Errorf("expected offline error, got: %s", rr.Body.String())
	}
}

func TestCacheDeleteHandler_NoPool(t *testing.T) {
	srv := NewMockServer()
	srv.pool = nil
	srv.router.HandleFunc("/cache/{key}", srv.cacheDeleteHandler).Methods("DELETE")

	req, _ := http.NewRequest("DELETE", "/cache/key1", nil)
	rr := httptest.NewRecorder()
	srv.router.ServeHTTP(rr, req)

	if !strings.Contains(rr.Body.String(), "cache server is offline") {
		t.Errorf("expected offline error, got: %s", rr.Body.String())
	}
}

func TestCacheReadHandler_NoPool(t *testing.T) {
	srv := NewMockServer()
	srv.pool = nil
	srv.router.HandleFunc("/cache/{key}", srv.cacheReadHandler).Methods("GET")

	req, _ := http.NewRequest("GET", "/cache/key1", nil)
	rr := httptest.NewRecorder()
	srv.router.ServeHTTP(rr, req)

	if !strings.Contains(rr.Body.String(), "cache server is offline") {
		t.Errorf("expected offline error, got: %s", rr.Body.String())
	}
}

func TestCloseReleasesCachePool(t *testing.T) {
	srv := NewMockServer()
	srv.config.CacheServer = "tcp://" + startFakeRedis(t)
	srv.startCachePool(time.Hour)
	srv.router.HandleFunc("/cache/{key}", srv.cacheWriteHandler)

	srv.Stop()

	req, _ := http.NewRequest("POST", "/cache/key1", strings.NewReader("value1"))
	rr := httptest.NewRecorder()
	srv.router.ServeHTTP(rr, req)
	if rr.Code != http.StatusAccepted {
		t.Fatalf("cache write after Stop returned %d, want %d", rr.Code, http.StatusAccepted)
	}

	if err := srv.Close(); err != nil {
		t.Fatalf("Close returned %v", err)
	}
	if err := srv.pool.Get().Err(); err == nil {
		t.Error("cache pool is open after Close")
	}
	select {
	case <-srv.cacheStopped:
	default:
		t.Error("cache version goroutine is running after Close")
	}
}

func startFakeRedis(t *testing.T) string {
	t.Helper()

	lis, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("net.Listen %v", err)
	}
	t.Cleanup(func() {
		lis.Close()
	})

	go func() {
		for {
			conn, err := lis.Accept()
			if err != nil {
				return
			}
			go func() {
				defer conn.Close()
				r := bufio.NewReader(conn)
				for {
					line, err := r.ReadString('\n')
					if err != nil {
						return
					}
					n, _ := strconv.Atoi(strings.TrimSpace(strings.TrimPrefix(line, "*")))
					for range 2 * n {
						if _, err := r.ReadString('\n'); err != nil {
							return
						}
					}
					if _, err := conn.Write([]byte("+OK\r\n")); err != nil {
						return
					}
				}
			}()
		}
	}()

	return lis.Addr().String()
}
