package server

type Response struct {
	Environment []string          `json:"environment"`
	Runtime     map[string]string `json:"runtime"`
	Labels      map[string]string `json:"labels"`
	Annotations map[string]string `json:"annotations"`
}
