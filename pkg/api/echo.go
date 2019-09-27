package api

import (
	"bytes"
	"context"
	"fmt"
	"io/ioutil"
	"net/http"
	"sync"

	"github.com/stefanprodan/podinfo/pkg/version"
	"go.uber.org/zap"
)

// Echo godoc
// @Summary Echo
// @Description forwards the call to the backend service and echos the posted content
// @Tags HTTP API
// @Accept json
// @Produce json
// @Router /api/echo [post]
// @Success 202 {object} api.MapResponse
func (s *Server) echoHandler(w http.ResponseWriter, r *http.Request) {
	body, err := ioutil.ReadAll(r.Body)
	if err != nil {
		s.logger.Error("reading the request body failed", zap.Error(err))
		s.ErrorResponse(w, r, "invalid request body", http.StatusBadRequest)
		return
	}
	defer r.Body.Close()
	if len(s.config.BackendURL) > 0 {
		result := make([]string, len(s.config.BackendURL))
		var wg sync.WaitGroup
		wg.Add(len(s.config.BackendURL))
		for i, b := range s.config.BackendURL {
			go func(index int, backend string) {
				defer wg.Done()
				backendReq, err := http.NewRequest("POST", backend, bytes.NewReader(body))
				if err != nil {
					s.logger.Error("backend call failed", zap.Error(err), zap.String("url", backend))
					return
				}

				// forward headers
				copyTracingHeaders(r, backendReq)

				backendReq.Header.Set("X-API-Version", version.VERSION)
				backendReq.Header.Set("X-API-Revision", version.REVISION)

				ctx, cancel := context.WithTimeout(backendReq.Context(), s.config.HttpClientTimeout)
				defer cancel()

				// call backend
				resp, err := http.DefaultClient.Do(backendReq.WithContext(ctx))
				if err != nil {
					s.logger.Error("backend call failed", zap.Error(err), zap.String("url", backend))
					result[index] = fmt.Sprintf("backend %v call failed %v", backend, err)
					return
				}
				defer resp.Body.Close()

				// copy error status from backend and exit
				if resp.StatusCode >= 400 {
					s.logger.Error("backend call failed", zap.Int("status", resp.StatusCode), zap.String("url", backend))
					result[index] = fmt.Sprintf("backend %v response status code %v", backend, resp.StatusCode)
					return
				}

				// forward the received body
				rbody, err := ioutil.ReadAll(resp.Body)
				if err != nil {
					s.logger.Error(
						"reading the backend request body failed",
						zap.Error(err),
						zap.String("url", backend))
					result[index] = fmt.Sprintf("backend %v call failed %v", backend, err)
					return
				}

				s.logger.Debug(
					"payload received from backend",
					zap.String("response", string(rbody)),
					zap.String("url", backend))

				result[index] = string(rbody)
			}(i, b)
		}
		wg.Wait()

		w.Header().Set("X-Color", s.config.UIColor)
		s.JSONResponse(w, r, result)

	} else {
		w.Header().Set("X-Color", s.config.UIColor)
		w.WriteHeader(http.StatusAccepted)
		w.Write(body)
	}
}
