// Copyright 2018 Stefan Prodan.
// SPDX-License-Identifier: Apache-2.0

package http

import (
	"net/http"

	"strconv"

	"github.com/gorilla/mux"
)

// Status godoc
// @Summary Status code
// @Description sets the response status code to the specified code, 204 and 304 have no body
// @Tags HTTP API
// @Produce json
// @Param code path int true "status code to return, 200 to 999"
// @Router /status/{code} [get]
// @Router /status/{code} [post]
// @Router /status/{code} [put]
// @Success default {object} map[string]int "the specified status code"
// @Failure 400 {object} http.ErrorMessage "invalid status code"
func (s *Server) statusHandler(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)

	code, err := strconv.Atoi(vars["code"])
	// A response cannot end with a 1xx status: net/http sends it as interim, then a 200.
	if err != nil || code < 200 || code > 999 {
		s.ErrorResponse(w, r, "invalid status code", http.StatusBadRequest)
		return
	}

	s.JSONResponseCode(w, r, map[string]int{"status": code}, code)
}
