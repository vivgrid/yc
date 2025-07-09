// Package yc provides access to the documentation for the CLI commands
package yc

import (
	"embed"
	"fmt"
	"strings"
)

//go:embed docs
var docs embed.FS

// Doc retrieves the documentation for a specific command
func Doc(cmd string) (string, error) {
	if cmd == "" {
		cmd = "yc"
	}

	doc := strings.ReplaceAll(cmd, " ", "_")
	content, err := docs.ReadFile(fmt.Sprintf("docs/%s.md", doc))
	if err != nil {
		return "", fmt.Errorf("failed to read documentation for command %s: %w", cmd, err)
	}
	// add example.md content if the command is "yc"
	if cmd == "yc" {
		example, err := docs.ReadFile("docs/example.md")
		if err != nil {
			return "", fmt.Errorf("failed to read root documentation: %w", err)
		}
		content = append(content, example...)
	}
	return string(content), nil
}
