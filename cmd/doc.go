package main

import (
	"log/slog"
	"os"

	"github.com/spf13/cobra"
	"github.com/spf13/cobra/doc"
)

// GenDoc generates the documentation for the CLI commands
func GenDoc(cmd *cobra.Command) error {
	os.MkdirAll("./docs", os.ModePerm)
	err := doc.GenMarkdownTree(cmd, "./docs")
	if err != nil {
		slog.Error("failed to generate markdown documentation", "error", err)
		return err
	}
	return nil
}

// addDocCmd adds the documentation command to the root command
func addDocCmd(rootCmd *cobra.Command) {
	rootCmd.CompletionOptions.DisableDefaultCmd = true
	rootCmd.DisableAutoGenTag = true
	cmd := &cobra.Command{
		Use:    "doc",
		Short:  "Generate documentation for the CLI commands",
		Hidden: true,
		RunE: func(_ *cobra.Command, _ []string) error {
			return GenDoc(rootCmd)
		},
	}
	rootCmd.AddCommand(cmd)
}
