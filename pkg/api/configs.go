package api

import "net/http"

func (s *Server) configReadHandler(w http.ResponseWriter, r *http.Request) {
	_, span := s.tracer.Start(r.Context(), "configReadHandler")
	defer span.End()

	files := make(map[string]string)
	if watcher != nil {
		watcher.Cache.Range(func(key interface{}, value interface{}) bool {
			files[key.(string)] = value.(string)
			return true
		})
	}

	s.JSONResponse(w, r, files)
}
