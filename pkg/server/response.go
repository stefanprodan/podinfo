package server

type Response struct {
	Environment []string `json:"environment"`
	Runtime map[string]string `json:"runtime"`
}

