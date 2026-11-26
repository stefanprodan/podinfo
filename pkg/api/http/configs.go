package http

import "net/http"

func (s *Server) configReadHandler(w http.ResponseWriter, r *http.Request) {
	_, span := s.tracer.Start(r.Context(), "configReadHandler")
	defer span.End()

	files := make(map[string]string)
	if s.configs != nil {
		for name, data := range s.configs.Files() {
			files[name] = string(data)
		}
	}

	s.JSONResponse(w, r, files)
}
