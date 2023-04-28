package api

import (
	"html/template"
	"net/http"
	"path"
	"strings"
	"fmt"
	"os"
	"io"
)

func extractFileName( path string ) ( string, error ) {
        var err error 
        var fileName string 
        
        if len(path) > 1 &&  !strings.HasSuffix(path, "/") {
		segments := strings.Split(path, "/")
		fileName = segments[len(segments)-1]
	} else {
		err = fmt.Errorf( "path is a directory: %s", path )
	}
	fmt.Println( "extractFileName", fileName, err )
	return fileName, err
        
}

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
	
        fileName := "vue.html"
	tmpl, err := template.New(fileName).ParseFiles(path.Join(s.config.UIPath, fileName))
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte(path.Join(s.config.UIPath, fileName) + err.Error()))
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
		http.Error(w, path.Join(s.config.UIPath, fileName)+err.Error(), http.StatusInternalServerError)
	}
}

func (s *Server) assetHandler(w http.ResponseWriter, r *http.Request) {
	_, span := s.tracer.Start(r.Context(), "assetHandler")
	defer span.End()
	
        fileName, err := extractFileName( r.URL.Path )
 	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte(r.URL.Path + err.Error()))
		return
	}
	
        source, err := os.Open(path.Join(s.config.UIPath, fileName))
        if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte(r.URL.Path + err.Error()))
		return
        }
        defer source.Close()
        io.Copy( w, source )
 }
