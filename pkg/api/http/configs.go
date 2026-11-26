package http

import "net/http"

func (s *Server) configReadHandler(w http.ResponseWriter, r *http.Request) {
	files := make(map[string]string)
	if s.configs != nil {
		for name, data := range s.configs.Files() {
			files[name] = string(data)
		}
	}

	s.JSONResponse(w, r, files)
}
