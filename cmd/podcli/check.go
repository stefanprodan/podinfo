package main

import (
	"bytes"
	"context"
	"crypto/tls"
	"crypto/x509"
	"errors"
	"fmt"
	"net"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"

	"github.com/gorilla/websocket"
	"github.com/spf13/cobra"
	"go.uber.org/zap"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/health/grpc_health_v1"
	"google.golang.org/grpc/status"
)

var (
	retryCount      int
	retryDelay      time.Duration
	method          string
	body            string
	timeout         time.Duration
	grpcServiceName string
	grpcTLS         bool
	grpcSkipVerify  bool
	grpcCert        string
	grpcKey         string
)

var checkCmd = &cobra.Command{
	Use:   `check`,
	Short: "Health check commands",
	Long:  "Commands for running health checks",
}

var checkURLCmd = &cobra.Command{
	Use:     `http [address]`,
	Short:   "HTTP(S) health check",
	Example: `  check http https://httpbin.org/anything --method=POST --retry=2 --delay=2s --timeout=3s --body='{"test"=1}'`,
	RunE:    runCheck,
}

var checkTCPCmd = &cobra.Command{
	Use:     `tcp [address]`,
	Short:   "TCP health check",
	Example: `  check tcp httpbin.org:443 --retry=1 --delay=2s --timeout=2s`,
	RunE:    runCheckTCP,
}

var checkCertCmd = &cobra.Command{
	Use:     `cert [address]`,
	Short:   "SSL/TLS certificate validity check",
	Example: `  check cert httpbin.org`,
	RunE:    runCheckCert,
}

var checkgRPCCmd = &cobra.Command{
	Use:     `grpc [address]`,
	Short:   "gRPC health check",
	Example: `  check grpc localhost:8080 --service=podinfo --retry=1 --delay=2s --timeout=2s`,
	RunE:    runCheckGRPC,
}

var checkWsCmd = &cobra.Command{
	Use:     `ws [address]`,
	Short:   "WebSocket round-trip health check",
	Example: `  check ws ws://localhost:9898/ws/echo --retry=1 --delay=2s --timeout=5s`,
	RunE:    runCheckWs,
}

func init() {
	checkURLCmd.Flags().StringVar(&method, "method", "GET", "HTTP method")
	checkURLCmd.Flags().StringVar(&body, "body", "", "HTTP POST/PUT content")
	checkURLCmd.Flags().IntVar(&retryCount, "retry", 0, "times to retry the HTTP call")
	checkURLCmd.Flags().DurationVar(&retryDelay, "delay", 1*time.Second, "wait duration between retries")
	checkURLCmd.Flags().DurationVar(&timeout, "timeout", 5*time.Second, "timeout")
	checkCmd.AddCommand(checkURLCmd)

	checkTCPCmd.Flags().IntVar(&retryCount, "retry", 0, "times to retry the TCP check")
	checkTCPCmd.Flags().DurationVar(&retryDelay, "delay", 1*time.Second, "wait duration between retries")
	checkTCPCmd.Flags().DurationVar(&timeout, "timeout", 5*time.Second, "timeout")
	checkCmd.AddCommand(checkTCPCmd)

	checkgRPCCmd.Flags().IntVar(&retryCount, "retry", 0, "times to retry the TCP check")
	checkgRPCCmd.Flags().DurationVar(&retryDelay, "delay", 1*time.Second, "wait duration between retries")
	checkgRPCCmd.Flags().DurationVar(&timeout, "timeout", 5*time.Second, "timeout")
	checkgRPCCmd.Flags().StringVar(&grpcServiceName, "service", "", "gRPC service name")
	checkgRPCCmd.Flags().BoolVar(&grpcTLS, "tls", false, "use TLS for gRPC connection")
	checkgRPCCmd.Flags().BoolVar(&grpcSkipVerify, "insecure-skip-verify", false, "skip the server certificate verification, requires --tls")
	checkgRPCCmd.Flags().StringVar(&grpcCert, "cert", "", "client certificate PEM file for mTLS, requires --tls")
	checkgRPCCmd.Flags().StringVar(&grpcKey, "key", "", "client private key PEM file for mTLS, requires --tls")
	checkgRPCCmd.MarkFlagsRequiredTogether("cert", "key")
	checkCmd.AddCommand(checkgRPCCmd)

	checkCmd.AddCommand(checkCertCmd)

	checkWsCmd.Flags().IntVar(&retryCount, "retry", 0, "times to retry the WebSocket check")
	checkWsCmd.Flags().DurationVar(&retryDelay, "delay", 1*time.Second, "wait duration between retries")
	checkWsCmd.Flags().DurationVar(&timeout, "timeout", 5*time.Second, "timeout")
	checkCmd.AddCommand(checkWsCmd)

	rootCmd.AddCommand(checkCmd)
}

func runCheck(cmd *cobra.Command, args []string) error {
	if len(args) < 1 {
		return errors.New("address is required! example: check http https://httpbin.org")
	}

	address := args[0]
	if !strings.HasPrefix(address, "http://") && !strings.HasPrefix(address, "https://") {
		address = "http://" + address
	}

	return retryCheck(address, func() error {
		ctx, cancel := context.WithTimeout(cmd.Context(), timeout)
		defer cancel()
		req, err := http.NewRequestWithContext(ctx, method, address, bytes.NewBufferString(body))
		if err != nil {
			return permanent(err)
		}

		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			return err
		}
		_ = resp.Body.Close()

		if resp.StatusCode < 200 || resp.StatusCode >= 400 {
			return fmt.Errorf("status code %d", resp.StatusCode)
		}

		logger.Info("check succeed",
			zap.String("address", address),
			zap.Int("status code", resp.StatusCode),
			zap.String("response size", fmtContentLength(resp.ContentLength)))
		return nil
	})
}

func runCheckTCP(cmd *cobra.Command, args []string) error {
	if len(args) < 1 {
		return errors.New("address is required! example: check tcp httpbin.org:80")
	}
	address := args[0]

	return retryCheck(address, func() error {
		conn, err := (&net.Dialer{Timeout: timeout}).DialContext(cmd.Context(), "tcp", address)
		if err != nil {
			return err
		}
		_ = conn.Close()

		logger.Info("check succeed", zap.String("address", address))
		return nil
	})
}

func runCheckCert(cmd *cobra.Command, args []string) error {
	if len(args) < 1 {
		return errors.New("address is required! example: check cert httpbin.org")
	}
	host := args[0]
	if !strings.HasPrefix(host, "https://") {
		host = "https://" + host
	}

	u, err := url.Parse(host)
	if err != nil {
		logger.Info("check failed",
			zap.String("address", host),
			zap.Error(err))
		os.Exit(1)
	}

	address := u.Hostname() + ":443"
	cert, err := peerCertificate(cmd.Context(), address, u.Hostname())
	if err != nil {
		logger.Info("check failed",
			zap.String("address", address),
			zap.Error(err))
		os.Exit(1)
	}

	timeNow := time.Now()
	if timeNow.After(cert.NotAfter) {
		logger.Info("check failed",
			zap.String("address", address),
			zap.String("issuer", cert.Issuer.CommonName),
			zap.String("subject", cert.Subject.CommonName),
			zap.Time("expired", cert.NotAfter))
		os.Exit(1)
	}

	logger.Info("check succeed",
		zap.String("address", address),
		zap.String("issuer", cert.Issuer.CommonName),
		zap.String("subject", cert.Subject.CommonName),
		zap.Time("notAfter", cert.NotAfter),
		zap.Time("notBefore", cert.NotBefore))

	return nil
}

func peerCertificate(ctx context.Context, address, serverName string) (*x509.Certificate, error) {
	ipConn, err := (&net.Dialer{Timeout: 5 * time.Second}).DialContext(ctx, "tcp", address)
	if err != nil {
		return nil, err
	}
	defer ipConn.Close()

	conn := tls.Client(ipConn, &tls.Config{
		InsecureSkipVerify: true, //nolint:gosec // the check reports the expiry of any certificate, trusted or not
		ServerName:         serverName,
	})
	if err := conn.HandshakeContext(ctx); err != nil {
		return nil, err
	}
	defer conn.Close()

	if _, _, err := net.SplitHostPort(conn.RemoteAddr().String()); err != nil {
		return nil, err
	}

	return conn.ConnectionState().PeerCertificates[0], nil
}

func fmtContentLength(b int64) string {
	const unit = 1000
	if b < unit {
		return fmt.Sprintf("%d B", b)
	}
	div, exp := int64(unit), 0
	for n := b / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f %cB", float64(b)/float64(div), "kMGTPE"[exp])
}

func runCheckWs(_ *cobra.Command, args []string) error {
	if len(args) < 1 {
		return errors.New("address is required! example: check ws wss://localhost:9898/ws/echo")
	}

	address := args[0]
	if !strings.HasPrefix(address, "ws://") && !strings.HasPrefix(address, "wss://") {
		return errors.New("address must start with ws:// or wss://")
	}

	return retryCheck(address, func() error {
		dialer := websocket.Dialer{
			HandshakeTimeout: timeout,
		}

		conn, _, err := dialer.Dial(address, nil) //nolint:bodyclose // gorilla/websocket documents that the response body does not need closing
		if err != nil {
			return err
		}
		defer conn.Close()

		msg := "podinfo-check"
		start := time.Now()

		if err := conn.SetWriteDeadline(start.Add(timeout)); err != nil {
			return err
		}
		if err := conn.WriteMessage(websocket.TextMessage, []byte(msg)); err != nil {
			return err
		}

		if err := conn.SetReadDeadline(time.Now().Add(timeout)); err != nil {
			return err
		}
		_, resp, err := conn.ReadMessage()
		if err != nil {
			return err
		}

		logger.Info("check succeed",
			zap.String("address", address),
			zap.Duration("round-trip", time.Since(start)),
			zap.Int("response size", len(resp)))
		return nil
	})
}

func runCheckGRPC(_ *cobra.Command, args []string) error {
	if len(args) < 1 {
		return errors.New("address is required! example: check grpc localhost:8080")
	}
	address := args[0]

	if !grpcTLS && (grpcSkipVerify || grpcCert != "") {
		return errors.New("--insecure-skip-verify, --cert and --key require --tls")
	}

	var creds grpc.DialOption
	if grpcTLS {
		tlsConfig := &tls.Config{InsecureSkipVerify: grpcSkipVerify} //nolint:gosec // set by the --insecure-skip-verify flag
		if grpcCert != "" {
			cert, err := tls.LoadX509KeyPair(grpcCert, grpcKey)
			if err != nil {
				return err
			}
			tlsConfig.Certificates = []tls.Certificate{cert}
		}
		creds = grpc.WithTransportCredentials(credentials.NewTLS(tlsConfig))
	} else {
		creds = grpc.WithTransportCredentials(insecure.NewCredentials())
	}

	return retryCheck(address, func() error {
		conn, err := grpc.NewClient(address, creds)
		if err != nil {
			return err
		}
		defer conn.Close()

		ctx, cancel := context.WithTimeout(context.Background(), timeout)
		defer cancel()
		resp, err := grpc_health_v1.NewHealthClient(conn).Check(ctx, &grpc_health_v1.HealthCheckRequest{
			Service: grpcServiceName,
		})
		if err != nil {
			if stat, ok := status.FromError(err); ok && stat.Code() == codes.Unimplemented {
				return permanent(errors.New("gRPC health protocol not implemented"))
			}
			return err
		}

		if resp.GetStatus() != grpc_health_v1.HealthCheckResponse_SERVING {
			return fmt.Errorf("status %s", resp.GetStatus())
		}

		logger.Info("check succeed",
			zap.String("status", resp.GetStatus().String()))
		return nil
	})
}

// permanentError marks a check failure that retrying cannot fix.
type permanentError struct {
	err error
}

func (e *permanentError) Error() string { return e.err.Error() }
func (e *permanentError) Unwrap() error { return e.err }

func permanent(err error) error {
	return &permanentError{err: err}
}

// retryCheck runs the check once and then up to retryCount more times,
// waiting retryDelay between attempts. It exits with 0 on the first
// success and with 1 when all attempts fail or a failure is permanent.
func retryCheck(address string, check func() error) error {
	if retryCount < 0 {
		return errors.New("--retry is required")
	}

	for n := 0; n <= retryCount; n++ {
		if n > 0 {
			time.Sleep(retryDelay)
		}

		err := check()
		if err == nil {
			os.Exit(0)
		}

		logger.Info("check failed",
			zap.String("address", address),
			zap.Error(err))

		if _, ok := errors.AsType[*permanentError](err); ok {
			break
		}
	}

	os.Exit(1)
	return nil
}
