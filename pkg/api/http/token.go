package http

import (
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/golang-jwt/jwt/v4"
	"go.uber.org/zap"
)

type jwtCustomClaims struct {
	Name string `json:"name"`
	jwt.StandardClaims
}

// Token godoc
// @Summary Generate JWT token
// @Description issues a JWT token valid for one minute
// @Tags HTTP API
// @Accept json
// @Produce json
// @Router /token [post]
// @Success 200 {object} http.TokenResponse
func (s *Server) tokenGenerateHandler(w http.ResponseWriter, r *http.Request) {
	_, span := s.tracer.Start(r.Context(), "tokenGenerateHandler")
	defer span.End()

	body, err := io.ReadAll(r.Body)
	if err != nil {
		s.logger.Error("reading the request body failed", zap.Error(err))
		s.ErrorResponse(w, r, span, "invalid request body", http.StatusBadRequest)
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
		jwt.StandardClaims{
			Issuer:    "podinfo",
			ExpiresAt: expiresAt.Unix(),
		},
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	t, err := token.SignedString([]byte(s.config.JWTSecret))
	if err != nil {
		s.ErrorResponse(w, r, span, err.Error(), http.StatusBadRequest)
		return
	}

	var result = TokenResponse{
		Token:     t,
		ExpiresAt: time.Unix(claims.StandardClaims.ExpiresAt, 0),
	}

	s.JSONResponse(w, r, result)
}

// TokenValidate godoc
// @Summary Validate JWT token
// @Description validates the JWT token
// @Tags HTTP API
// @Accept json
// @Produce json
// @Router /token/validate [post]
// @Success 200 {object} http.TokenValidationResponse
// @Failure 401 {string} string "Unauthorized"
// Get: JWT=$(curl -s -d 'test' localhost:9898/token | jq -r .token)
// Post: curl -H "Authorization: Bearer ${JWT}" localhost:9898/token/validate
func (s *Server) tokenValidateHandler(w http.ResponseWriter, r *http.Request) {
	_, span := s.tracer.Start(r.Context(), "tokenValidateHandler")
	defer span.End()

	authorizationHeader := r.Header.Get("authorization")
	if authorizationHeader == "" {
		s.ErrorResponse(w, r, span, "authorization bearer header required", http.StatusUnauthorized)
		return
	}
	bearerToken := strings.Split(authorizationHeader, " ")
	if len(bearerToken) != 2 || strings.ToLower(bearerToken[0]) != "bearer" {
		s.ErrorResponse(w, r, span, "authorization bearer header required", http.StatusUnauthorized)
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
		s.ErrorResponse(w, r, span, err.Error(), http.StatusUnauthorized)
		return
	}

	if token.Valid {
		if claims.StandardClaims.Issuer != "podinfo" {
			s.ErrorResponse(w, r, span, "invalid issuer", http.StatusUnauthorized)
		} else {
			var result = TokenValidationResponse{
				TokenName: claims.Name,
				ExpiresAt: time.Unix(claims.StandardClaims.ExpiresAt, 0),
			}
			s.JSONResponse(w, r, result)
		}
	} else {
		s.ErrorResponse(w, r, span, "Invalid authorization token", http.StatusUnauthorized)
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
