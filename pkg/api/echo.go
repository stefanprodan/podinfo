package api

import (
	"bytes"
	"context"
	"io/ioutil"
	"net/http"
	"github.com/stefanprodan/k8s-podinfo/pkg/version"
	"go.uber.org/zap"
)

func (s *Server) echoHandler(w http.ResponseWriter, r *http.Request) {
	body, err := ioutil.ReadAll(r.Body)
	if err != nil {
		s.logger.Error("reading the request body failed", zap.Error(err))
		s.ErrorResponse(w, r, "invalid request body", http.StatusBadRequest)
		return
	}

	if len(s.config.BackendURL) > 0 {
		backendReq, err := http.NewRequest("POST", s.config.BackendURL, bytes.NewReader(body))
		if err != nil {
			s.logger.Error("backend call failed", zap.Error(err), zap.String("url", s.config.BackendURL))
			s.ErrorResponse(w, r, "backend call failed", http.StatusInternalServerError)
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
			s.logger.Error("backend call failed", zap.Error(err), zap.String("url", s.config.BackendURL))
			s.ErrorResponse(w, r, "backend call failed", http.StatusInternalServerError)
			return
		}

		defer resp.Body.Close()

		// copy error status from backend and exit
		if resp.StatusCode >= 400 {
			s.ErrorResponse(w, r, "backend error", resp.StatusCode)
			return
		}

		// forward the received body
		rbody, err := ioutil.ReadAll(resp.Body)
		if err != nil {
			s.logger.Error(
				"reading the backend request body failed",
				zap.Error(err),
				zap.String("url", s.config.BackendURL))
			s.ErrorResponse(w, r, "backend call failed", http.StatusInternalServerError)
			return
		}

		s.logger.Debug(
			"payload received from backend",
			zap.String("response", string(rbody)),
			zap.String("url", s.config.BackendURL))

		w.Header().Set("X-Color", s.config.UIColor)
		w.WriteHeader(http.StatusAccepted)
		w.Write(rbody)
	} else {
		w.Header().Set("X-Color", s.config.UIColor)
		w.WriteHeader(http.StatusAccepted)
		w.Write(body)
	}
}


