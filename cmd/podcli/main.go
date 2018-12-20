package main

import (
	"fmt"
	"github.com/spf13/cobra"
	"go.uber.org/zap"
	"log"
	"os"
	"strings"
)

var rootCmd = &cobra.Command{
	Use:   "podcli",
	Short: "podinfo command line",
	Long: `
podinfo command line utilities`,
}

var (
	logger *zap.Logger
)

func main() {

	var err error
	logger, err = zap.NewDevelopment()
	if err != nil {
		log.Fatalf("can't initialize zap logger: %v", err)
	}
	defer logger.Sync()

	rootCmd.SetArgs(os.Args[1:])
	if err := rootCmd.Execute(); err != nil {
		e := err.Error()
		fmt.Println(strings.ToUpper(e[:1]) + e[1:])
		os.Exit(1)
	}
}
