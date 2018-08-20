package api

import (
	"html/template"
	"net/http"
	"path"
)

func (s *Server) indexHandler(w http.ResponseWriter, r *http.Request) {
	tmpl, err := template.New("vue.html").ParseFiles(path.Join(s.config.UIPath, "vue.html"))
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte(path.Join(s.config.UIPath, "vue.html") + err.Error()))
		return
	}

	data := struct {
		Title string
	}{
		Title: s.config.Hostname,
	}

	if err := tmpl.Execute(w, data); err != nil {
		http.Error(w, path.Join(s.config.UIPath, "vue.html")+err.Error(), http.StatusInternalServerError)
	}
}
