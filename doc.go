// Package yc provides access to the documentation for the CLI commands
package yc

import (
	"embed"
	"fmt"
	"log/slog"
	"strings"
)

//go:embed docs
var docs embed.FS

// Doc retrieves the documentation for a specific command
func Doc(cmd string) (string, error) {
	if cmd == "" {
		cmd = "yc"
	}
	if !strings.HasPrefix(cmd, "yc") {
		cmd = "yc " + cmd
	}

	doc := strings.ReplaceAll(cmd, " ", "_")
	doc = fmt.Sprintf("docs/%s.md", doc)
	content, err := docs.ReadFile(doc)
	if err != nil {
		return "", fmt.Errorf("failed to read documentation for command %s: %w", cmd, err)
	}
	// add example.md content if the command is "yc"
	if cmd == "yc" {
		doc = "docs/example.md"
		example, err := docs.ReadFile(doc)
		if err != nil {
			return "", fmt.Errorf("failed to read root documentation: %w", err)
		}
		content = append(content, example...)
	}
	slog.Info("yc documentation", "command", cmd, "doc", doc)
	return string(content), nil
}
