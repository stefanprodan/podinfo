package main

import (
	"bytes"
	"context"
	"fmt"
	"net/http"
	"os"
	"time"

	"github.com/spf13/cobra"
	"go.uber.org/zap"
)

var (
	retryCount int
	retryDelay time.Duration
	method     string
	body       string
	timeout    time.Duration
)

var checkCmd = &cobra.Command{
	Use:   `check`,
	Short: "Health check commands",
	Long:  "Commands for running health checks",
}

var checkUrlCmd = &cobra.Command{
	Use:     `http [--method POST] [--retry 2] [--delay 1s] [--timeout 1s] [--body '{"test"="testing"}']`,
	Short:   "HTTP/S health check",
	Example: `  check http https://httpbin.org/anything --retry=2 --delay=2s --timeout=1s --body='{"test"=1}'`,
	RunE:    runCheck,
}

func init() {
	checkUrlCmd.Flags().StringVar(&method, "method", "GET", "HTTP method")
	checkUrlCmd.Flags().StringVar(&body, "body", "", "HTTP POST/PUT content")
	checkUrlCmd.Flags().IntVar(&retryCount, "retry", 1, "times to retry the HTTP call")
	checkUrlCmd.Flags().DurationVar(&retryDelay, "delay", 1*time.Second, "wait duration between retries")
	checkUrlCmd.Flags().DurationVar(&timeout, "timeout", 2*time.Second, "timeout")
	checkCmd.AddCommand(checkUrlCmd)
	rootCmd.AddCommand(checkCmd)
}

func runCheck(cmd *cobra.Command, args []string) error {
	if retryCount < 0 {
		return fmt.Errorf("--retry is required")
	}
	if len(args) < 1 {
		return fmt.Errorf("address is required")
	}
	url := args[0]

	for n := 0; n <= retryCount; n++ {
		if n != 1 {
			time.Sleep(retryDelay)
		}

		req, err := http.NewRequest(method, url, bytes.NewBuffer([]byte(body)))
		if err != nil {
			logger.Info("check failed",
				zap.String("url", url),
				zap.Error(err))
			os.Exit(1)
		}

		ctx, cancel := context.WithTimeout(req.Context(), timeout)
		resp, err := http.DefaultClient.Do(req.WithContext(ctx))
		cancel()
		if err != nil {
			logger.Info("check failed",
				zap.String("url", url),
				zap.Error(err))
			continue
		}

		if resp.Body != nil {
			resp.Body.Close()
		}

		if resp.StatusCode >= 200 && resp.StatusCode < 400 {
			logger.Info("check succeed",
				zap.String("url", url),
				zap.Int("status code", resp.StatusCode),
				zap.String("response size", fmtContentLength(resp.ContentLength)))
			os.Exit(0)
		} else {
			logger.Info("check failed",
				zap.String("url", url),
				zap.Int("status code", resp.StatusCode))
			continue
		}
	}

	os.Exit(1)
	return nil
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
