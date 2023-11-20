package http

import (
	"html/template"
	"net/http"
	"path"
)

// Index godoc
// @Summary Index
// @Description renders podinfo UI
// @Tags HTTP API
// @Produce html
// @Router / [get]
// @Success 200 {string} string "OK"
func (s *Server) indexHandler(w http.ResponseWriter, r *http.Request) {
	_, span := s.tracer.Start(r.Context(), "indexHandler")
	defer span.End()

	tmpl, err := template.New("vue.html").ParseFiles(path.Join(s.config.UIPath, "vue.html"))
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte(path.Join(s.config.UIPath, "vue.html") + err.Error()))
		return
	}

	data := struct {
		Title string
		Logo  string
	}{
		Title: s.config.Hostname,
		Logo:  s.config.UILogo,
	}

	if err := tmpl.Execute(w, data); err != nil {
		http.Error(w, path.Join(s.config.UIPath, "vue.html")+err.Error(), http.StatusInternalServerError)
	}
}
