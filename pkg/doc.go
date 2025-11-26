package pkg

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
