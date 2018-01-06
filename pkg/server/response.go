package server

type Response struct {
	Runtime     map[string]string `json:"runtime"`
	Labels      map[string]string `json:"labels"`
	Annotations map[string]string `json:"annotations"`
	Environment []string          `json:"environment"`
}
