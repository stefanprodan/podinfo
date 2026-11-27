package grpc

import (
	"context"
	"log"
	"net"
	"strings"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v4"
	tokenv1 "github.com/stefanprodan/podinfo/pkg/api/grpc/token/v1"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
	"google.golang.org/grpc/test/bufconn"
)

func TestGrpcToken(t *testing.T) {

	lis := bufconn.Listen(1024 * 1024)
	t.Cleanup(func() {
		lis.Close()
	})

	srv := grpc.NewServer()
	t.Cleanup(func() {
		srv.Stop()
	})
	config := &Config{}
	config.JWTSecret = "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9"
	tokenv1.RegisterTokenServiceServer(srv, &TokenServer{config: config})

	go func() {
		if err := srv.Serve(lis); err != nil {
			log.Fatalf("srv.Serve %v", err)
		}
	}()

	dialer := func(context.Context, string) (net.Conn, error) {
		return lis.Dial()
	}

	conn, err := grpc.NewClient("passthrough:///bufnet", grpc.WithContextDialer(dialer), grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		t.Fatalf("grpc.NewClient %v", err)
	}
	t.Cleanup(func() {
		conn.Close()
	})

	client := tokenv1.NewTokenServiceClient(conn)
	res, err := client.TokenGenerate(context.Background(), &tokenv1.TokenGenerateRequest{})

	if _, ok := status.FromError(err); !ok {
		t.Errorf("Token Handler returned type %T, want %T", err, status.Error)
	}

	var token = tokenv1.TokenGenerateResponse{
		Token:     res.Token,
		ExpiresAt: res.ExpiresAt,
	}

	if token.Token == "" {
		t.Fatalf("Handler returned no token")
	}
}

func TestGrpcTokenValidate(t *testing.T) {
	const secret = "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9"
	srv := &TokenServer{config: &Config{JWTSecret: secret}}

	sign := func(method jwt.SigningMethod, key interface{}, issuer string, expiresAt time.Time) string {
		claims := &jwtCustomClaims{
			"test",
			jwt.RegisteredClaims{
				Issuer:    issuer,
				ExpiresAt: jwt.NewNumericDate(expiresAt),
			},
		}
		signed, err := jwt.NewWithClaims(method, claims).SignedString(key)
		if err != nil {
			t.Fatalf("signing token: %v", err)
		}
		return signed
	}
	hmac := func(key, issuer string, expiresAt time.Time) string {
		return sign(jwt.SigningMethodHS256, []byte(key), issuer, expiresAt)
	}
	future := time.Now().Add(time.Minute)

	tests := []struct {
		name          string
		noMetadata    bool
		authorization []string
		want          codes.Code
		wantMsg       string
	}{
		{name: "valid", authorization: []string{"Bearer " + hmac(secret, "podinfo", future)}, want: codes.OK},
		{name: "no metadata", noMetadata: true, want: codes.Unauthenticated},
		{name: "missing header", want: codes.Unauthenticated},
		{name: "malformed header", authorization: []string{hmac(secret, "podinfo", future)}, want: codes.Unauthenticated},
		{name: "expired", authorization: []string{"Bearer " + hmac(secret, "podinfo", time.Now().Add(-time.Minute))}, want: codes.Unauthenticated},
		{name: "bad signature", authorization: []string{"Bearer " + hmac("other-secret", "podinfo", future)}, want: codes.Unauthenticated},
		{name: "wrong issuer", authorization: []string{"Bearer " + hmac(secret, "other", future)}, want: codes.Unauthenticated},
		{name: "non-HMAC signing method", authorization: []string{"Bearer " + sign(jwt.SigningMethodNone, jwt.UnsafeAllowNoneSignatureType, "podinfo", future)}, want: codes.Unauthenticated, wantMsg: "invalid signing method"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := context.Background()
			if !tt.noMetadata {
				md := metadata.MD{}
				if tt.authorization != nil {
					md.Set("authorization", tt.authorization...)
				}
				ctx = metadata.NewIncomingContext(ctx, md)
			}

			res, err := srv.TokenValidate(ctx, &tokenv1.TokenValidateRequest{})
			if code := status.Code(err); code != tt.want {
				t.Fatalf("TokenValidate returned %s (%v), want %s", code, err, tt.want)
			}
			if tt.wantMsg != "" && !strings.Contains(status.Convert(err).Message(), tt.wantMsg) {
				t.Errorf("TokenValidate returned %q, want it to contain %q", status.Convert(err).Message(), tt.wantMsg)
			}
			if tt.want != codes.OK {
				return
			}
			if res.GetToken() != "test" {
				t.Errorf("TokenValidate returned token name %q, want %q", res.GetToken(), "test")
			}
			if want := jwt.NewNumericDate(future).Time.String(); res.GetExpiresAt() != want {
				t.Errorf("TokenValidate returned expires at %q, want %q", res.GetExpiresAt(), want)
			}
		})
	}
}
