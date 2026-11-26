package http

import (
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/golang-jwt/jwt/v4"
)

type jwtCustomClaims struct {
	Name string `json:"name"`
	jwt.RegisteredClaims
}

// Token godoc
// @Summary Generate JWT token
// @Description issues a JWT token valid for one minute
// @Tags HTTP API
// @Accept json
// @Produce json
// @Router /token [post]
// @Success 200 {object} http.TokenResponse
// @Failure 400 {object} http.ErrorMessage "invalid request body"
// @Failure 413 {object} http.ErrorMessage "request body too large"
// @Failure 500 {object} http.ErrorMessage "token signing failed"
func (s *Server) tokenGenerateHandler(w http.ResponseWriter, r *http.Request) {
	body, ok := s.readLimitedBody(w, r)
	if !ok {
		return
	}
	defer r.Body.Close()

	user := "anonymous"
	if len(body) > 0 {
		user = string(body)
	}

	expiresAt := time.Now().Add(time.Minute * 1)
	claims := &jwtCustomClaims{
		user,
		jwt.RegisteredClaims{
			Issuer:    "podinfo",
			ExpiresAt: jwt.NewNumericDate(expiresAt),
		},
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	t, err := token.SignedString([]byte(s.config.JWTSecret))
	if err != nil {
		s.ErrorResponse(w, r, err.Error(), http.StatusInternalServerError)
		return
	}

	var result = TokenResponse{
		Token:     t,
		ExpiresAt: claims.ExpiresAt.Time,
	}

	s.JSONResponse(w, r, result)
}

// TokenValidate godoc
// @Summary Validate JWT token
// @Description validates the JWT token
// @Tags HTTP API
// @Accept json
// @Produce json
// @Router /token/validate [get]
// @Success 200 {object} http.TokenValidationResponse
// @Failure 401 {object} http.ErrorMessage "invalid or missing token"
// Get: JWT=$(curl -s -d 'test' localhost:9898/token | jq -r .token)
// Post: curl -H "Authorization: Bearer ${JWT}" localhost:9898/token/validate
func (s *Server) tokenValidateHandler(w http.ResponseWriter, r *http.Request) {
	authorizationHeader := r.Header.Get("authorization")
	if authorizationHeader == "" {
		s.ErrorResponse(w, r, "authorization bearer header required", http.StatusUnauthorized)
		return
	}
	bearerToken := strings.Split(authorizationHeader, " ")
	if len(bearerToken) != 2 || strings.ToLower(bearerToken[0]) != "bearer" {
		s.ErrorResponse(w, r, "authorization bearer header required", http.StatusUnauthorized)
		return
	}

	claims := jwtCustomClaims{}
	token, err := jwt.ParseWithClaims(bearerToken[1], &claims, func(token *jwt.Token) (interface{}, error) {
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("invalid signing method")
		}
		return []byte(s.config.JWTSecret), nil
	})
	if err != nil {
		s.ErrorResponse(w, r, err.Error(), http.StatusUnauthorized)
		return
	}

	if token.Valid {
		if claims.Issuer != "podinfo" {
			s.ErrorResponse(w, r, "invalid issuer", http.StatusUnauthorized)
		} else if claims.ExpiresAt == nil {
			s.ErrorResponse(w, r, "token expiry required", http.StatusUnauthorized)
		} else {
			var result = TokenValidationResponse{
				TokenName: claims.Name,
				ExpiresAt: claims.ExpiresAt.Time,
			}
			s.JSONResponse(w, r, result)
		}
	} else {
		s.ErrorResponse(w, r, "Invalid authorization token", http.StatusUnauthorized)
	}
}

type TokenResponse struct {
	Token     string    `json:"token"`
	ExpiresAt time.Time `json:"expires_at"`
}

type TokenValidationResponse struct {
	TokenName string    `json:"token_name"`
	ExpiresAt time.Time `json:"expires_at"`
}
