package main

import (
	"fmt"
	"github.com/spf13/cobra"
	"github.com/stefanprodan/podinfo/pkg/version"
)

func init() {
	rootCmd.AddCommand(versionCmd)
}

var versionCmd = &cobra.Command{
	Use:   `version`,
	Short: "Prints podcli version",
	RunE: func(cmd *cobra.Command, args []string) error {
		fmt.Println(version.VERSION)
		return nil
	},
}
