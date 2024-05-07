package grpc

import (
	"context"
	"time"

	"github.com/golang-jwt/jwt/v4"

	"go.uber.org/zap"

	pb "github.com/stefanprodan/podinfo/pkg/api/grpc/token"
)

type TokenServer struct {
	pb.UnimplementedTokenServiceServer
	config *Config
	logger *zap.Logger
}

type jwtCustomClaims struct {
	Name string `json:"name"`
	jwt.StandardClaims
}

// SayHello implements helloworld.GreeterServer

func (s *TokenServer) Token(ctx context.Context, req *pb.TokenRequest) (*pb.TokenResponse, error) {

	user := "anonymous"
	expiresAt := time.Now().Add(time.Minute * 1).Unix()

	claims := &jwtCustomClaims{
		user,
		jwt.StandardClaims{
			Issuer:    "podinfo",
			ExpiresAt: expiresAt,
		},
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	t, err := token.SignedString([]byte("secret"))

	if err != nil {
		s.logger.Error("Failed to generate token", zap.Error(err))
		return &pb.TokenResponse{}, err
	}

	var result = pb.TokenResponse{
		Token:     t,
		ExpiresAt: time.Unix(claims.StandardClaims.ExpiresAt, 0).String(),
	}

	return &result, nil
}
