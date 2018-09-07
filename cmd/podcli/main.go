package main

import (
	"fmt"
	"os"
	"strings"

	"github.com/spf13/cobra"
	"go.uber.org/zap"
	"log"
)

var rootCmd = &cobra.Command{
	Use:   "podcli",
	Short: "podinfo command line",
	Long: `
podinfo command line utilities`,
}

var (
	configFile string
	logger *zap.Logger
)

func main() {

	var err error
	logger, err = zap.NewDevelopment()
	if err != nil {
		log.Fatalf("can't initialize zap logger: %v", err)
	}
	defer logger.Sync()

	rootCmd.PersistentFlags().StringVarP(&configFile, "config", "f", "", "path to config file")
	rootCmd.SetArgs(os.Args[1:])
	if err := rootCmd.Execute(); err != nil {
		e := err.Error()
		fmt.Println(strings.ToUpper(e[:1]) + e[1:])
		os.Exit(1)
	}
}
