package grpc

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/golang-jwt/jwt/v4"
	"go.uber.org/zap"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"

	pb "github.com/stefanprodan/podinfo/pkg/api/grpc/token"
	"github.com/stefanprodan/podinfo/pkg/telemetry"
)

type TokenServer struct {
	pb.UnimplementedTokenServiceServer
	config *Config
	logger *zap.Logger
}

type jwtCustomClaims struct {
	Name string `json:"name"`
	jwt.RegisteredClaims
}

func (s *TokenServer) TokenGenerate(ctx context.Context, req *pb.TokenRequest) (*pb.TokenResponse, error) {

	user := "anonymous"
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
		s.logger.Error("Failed to generate token", zap.Error(err), telemetry.Context(ctx))
		return nil, status.Error(codes.Internal, "failed to generate token")
	}

	var result = pb.TokenResponse{
		Token:     t,
		ExpiresAt: claims.ExpiresAt.Time.String(),
		Message:   "Token generated successfully",
	}

	return &result, nil
}

func (s *TokenServer) TokenValidate(ctx context.Context, req *pb.TokenRequest) (*pb.TokenResponse, error) {
	md, _ := metadata.FromIncomingContext(ctx)
	authorization := md.Get("authorization")
	if len(authorization) == 0 {
		return nil, status.Error(codes.Unauthenticated, "authorization bearer token required")
	}

	bearerToken := strings.Split(authorization[0], " ")
	if len(bearerToken) != 2 || strings.ToLower(bearerToken[0]) != "bearer" {
		return nil, status.Error(codes.Unauthenticated, "authorization bearer token required")
	}

	claims := jwtCustomClaims{}
	token, err := jwt.ParseWithClaims(bearerToken[1], &claims, func(token *jwt.Token) (interface{}, error) {
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("invalid signing method")
		}
		return []byte(s.config.JWTSecret), nil
	})
	if err != nil {
		return nil, status.Error(codes.Unauthenticated, err.Error())
	}

	if !token.Valid {
		return nil, status.Error(codes.Unauthenticated, "invalid authorization token")
	}

	if claims.Issuer != "podinfo" {
		return nil, status.Error(codes.Unauthenticated, "invalid issuer")
	}

	result := pb.TokenResponse{
		Token: claims.Name,
	}
	if claims.ExpiresAt != nil {
		result.ExpiresAt = claims.ExpiresAt.Time.String()
	}

	return &result, nil
}
