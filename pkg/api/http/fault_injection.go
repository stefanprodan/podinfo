package http

import (
	"io"
	"net/http"
	"strings"
	"sync/atomic"
)

// faultInjection is 1 while the server answers 500 on the application
// endpoints, to test client-side circuit breakers and outlier detection
// against a single failing replica.
var faultInjection int32

// faultInjectionExcluded reports whether the path bypasses fault injection.
// The probes, metrics, pprof and the control endpoints keep working, so
// Kubernetes does not restart the pod while a circuit breaker detects the fault.
func faultInjectionExcluded(p string) bool {
	switch {
	case strings.HasPrefix(p, "/fault_injection"):
		return true
	case p == "/healthz", p == "/readyz":
		return true
	case p == "/metrics":
		return true
	case strings.HasPrefix(p, "/debug/"):
		return true
	}
	return false
}

func writeInjectedFault(w http.ResponseWriter) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.Header().Set("X-Content-Type-Options", "nosniff")
	w.WriteHeader(http.StatusInternalServerError)
	io.WriteString(w, `{"status":"fault injection enabled"}`+"\n")
}

func faultInjectionMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if atomic.LoadInt32(&faultInjection) == 1 && !faultInjectionExcluded(r.URL.Path) {
			writeInjectedFault(w)
			return
		}
		next.ServeHTTP(w, r)
	})
}

// faultInjectionMiddleware strips the --prefix from the path before checking
// the exclusion list.
func (s *Server) faultInjectionMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		p := r.URL.Path
		if prefix := s.config.Prefix; prefix != "" && prefix != "/" {
			if trimmed := strings.TrimPrefix(p, prefix); trimmed != p {
				if trimmed == "" {
					trimmed = "/"
				}
				p = trimmed
			}
		}
		if atomic.LoadInt32(&faultInjection) == 1 && !faultInjectionExcluded(p) {
			writeInjectedFault(w)
			return
		}
		next.ServeHTTP(w, r)
	})
}

// EnableFaultInjection godoc
// @Summary Enable fault injection
// @Description makes the server respond with HTTP 500 for all non-probe endpoints
// @Tags Fault Injection
// @Accept json
// @Produce json
// @Router /fault_injection/enable [post]
// @Success 202 {string} string "OK"
func (s *Server) enableFaultInjectionHandler(w http.ResponseWriter, r *http.Request) {
	atomic.StoreInt32(&faultInjection, 1)
	s.JSONResponseCode(w, r, map[string]string{"fault_injection": "enabled"}, http.StatusAccepted)
}

// DisableFaultInjection godoc
// @Summary Disable fault injection
// @Description restores normal responses
// @Tags Fault Injection
// @Accept json
// @Produce json
// @Router /fault_injection/disable [post]
// @Success 202 {string} string "OK"
func (s *Server) disableFaultInjectionHandler(w http.ResponseWriter, r *http.Request) {
	atomic.StoreInt32(&faultInjection, 0)
	s.JSONResponseCode(w, r, map[string]string{"fault_injection": "disabled"}, http.StatusAccepted)
}

// FaultInjectionStatus godoc
// @Summary Get fault injection status
// @Tags Fault Injection
// @Accept json
// @Produce json
// @Router /fault_injection/status [get]
// @Success 200 {object} map[string]string
func (s *Server) faultInjectionStatusHandler(w http.ResponseWriter, r *http.Request) {
	state := "disabled"
	if atomic.LoadInt32(&faultInjection) == 1 {
		state = "enabled"
	}
	s.JSONResponse(w, r, map[string]string{"fault_injection": state})
}
