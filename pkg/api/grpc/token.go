package grpc

import (
	"context"
	"strings"
	"time"

	"github.com/golang-jwt/jwt/v4"
	"go.uber.org/zap"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"

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

func (s *TokenServer) TokenGenerate(ctx context.Context, req *pb.TokenRequest) (*pb.TokenResponse, error) {

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
	t, err := token.SignedString([]byte(s.config.JWTSecret))

	if err != nil {
		s.logger.Error("Failed to generate token", zap.Error(err))
		return &pb.TokenResponse{}, err
	}

	var result = pb.TokenResponse{
		Token:     t,
		ExpiresAt: time.Unix(claims.StandardClaims.ExpiresAt, 0).String(),
		Message:   "Token generated successfully",
	}

	return &result, nil
}

func (s *TokenServer) TokenValidate(ctx context.Context, req *pb.TokenRequest) (*pb.TokenResponse, error) {
	md, ok := metadata.FromIncomingContext(ctx)
	if !ok {
		return nil, status.Errorf(codes.DataLoss, "UnaryEcho: failed to get metadata")
	}

	authorization := md.Get("authorization")

	if len(authorization) == 0 {
		return nil, status.Errorf(codes.Unauthenticated, "Authorization token not found in metadata")
	}

	token := strings.TrimSpace(strings.TrimPrefix(authorization[0], "Bearer"))

	claims := jwtCustomClaims{}

	parsed_token, err := jwt.ParseWithClaims(token, &claims, func(parsed_token *jwt.Token) (interface{}, error) {
		if _, ok := parsed_token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, status.Errorf(codes.Canceled, "invalid signing method")
		}
		return []byte(s.config.JWTSecret), nil
	})
	if err != nil {
		if strings.Contains(err.Error(), "token is expired") || strings.Contains(err.Error(), "signature is invalid") {
			return &pb.TokenResponse{
				Message: err.Error(),
			}, nil
		}
		return nil, status.Errorf(codes.Unauthenticated, "Unable to parse token")

	}

	if parsed_token.Valid {
		if claims.StandardClaims.Issuer != "podinfo" {
			return nil, status.Errorf(codes.OK, "Invalid issuer")
		} else {
			var result = pb.TokenResponse{
				Token:     claims.Name,
				ExpiresAt: time.Unix(claims.StandardClaims.ExpiresAt, 0).String(),
			}
			return &result, nil
		}
	} else {
		return nil, status.Errorf(codes.Unauthenticated, "Unauthenticated")
	}
}
