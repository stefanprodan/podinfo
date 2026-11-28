package http

import "net/http"

// Configs godoc
// @Summary Config files
// @Description returns the files in the config path as a JSON object of file name to content, empty when the config path is not a directory
// @Tags HTTP API
// @Produce json
// @Router /configs [get]
// @Success 200 {object} map[string]string
func (s *Server) configReadHandler(w http.ResponseWriter, r *http.Request) {
	files := make(map[string]string)
	if s.configs != nil {
		for name, data := range s.configs.Files() {
			files[name] = string(data)
		}
	}

	s.JSONResponse(w, r, files)
}
