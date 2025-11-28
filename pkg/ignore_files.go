package pkg

import (
	"archive/zip"
	"io"
	"log"
	"os"
	"path/filepath"
	"strings"

	"github.com/codeglyph/go-dotignore"
)

// ZipWithExclusions creates a zip file from src, ignoring files that match
// patterns found in .gitignore plus a set of built-in defaults.
//
// Built-in ignore patterns (always applied):
//   - .git/     (Git repository directory)
//   - .vscode/  (VS Code settings directory)
//   - .DS_Store (macOS system file)
//
// If a .gitignore file exists in the source directory, its patterns will also be applied.
// The function uses gitignore-style pattern matching for consistent behavior.
func ZipWithExclusions(src, dst string) error {
	zipFile, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer zipFile.Close()

	zipWriter := zip.NewWriter(zipFile)
	defer zipWriter.Close()

	// Build unified ignore matcher: built-in patterns + optional .gitignore contents.
	builtinPatterns := []string{
		".git/",     // Git repository directory
		".vscode/",  // VS Code settings directory
		".DS_Store", // macOS system file
		".env",      // Environment variable file
	}
	var builder strings.Builder
	for _, p := range builtinPatterns {
		builder.WriteString(p)
		builder.WriteString("\n")
	}

	// Look for .gitignore in the source directory, not current working directory
	gitignorePath := filepath.Join(src, ".gitignore")
	if data, err := os.ReadFile(gitignorePath); err == nil {
		log.Printf("Found .gitignore at %s, applying additional patterns", gitignorePath)
		builder.Write(data)
		if !strings.HasSuffix(builder.String(), "\n") {
			builder.WriteString("\n")
		}
	}
	matcher, err := dotignore.NewPatternMatcherFromReader(strings.NewReader(builder.String()))
	if err != nil {
		return err
	}

	// traverse the src directory, check each file against the ignore patterns
	// and add it to the zip file if it doesn't match
	return filepath.WalkDir(src, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			log.Println("\t --err:", err)
			return err
		}

		// Relative path (POSIX style) for matching and zip header.
		relPath, err := filepath.Rel(src, path)
		if err != nil {
			return err
		}
		relPath = filepath.ToSlash(relPath)

		if d.IsDir() {
			if ignore, _ := matcher.Matches(relPath + "/"); ignore { // ensure directory semantics
				log.Printf("Ignoring directory: %s", relPath)
				return filepath.SkipDir
			}
			return nil
		}

		if ignore, _ := matcher.Matches(relPath); ignore {
			log.Printf("Ignoring file: %s", relPath)
			return nil
		}

		// add the file to the zip archive
		// Create zip header using the relative file path.
		fileInfo, err := d.Info()
		if err != nil {
			return err
		}
		header, err := zip.FileInfoHeader(fileInfo)
		if err != nil {
			return err
		}

		// relPath already slash-normalized above.
		header.Name = relPath
		header.Method = zip.Deflate

		writer, err := zipWriter.CreateHeader(header)
		if err != nil {
			return err
		}

		// Open the source file.
		f, err := os.Open(path)
		if err != nil {
			return err
		}
		defer func() {
			if closeErr := f.Close(); closeErr != nil {
				log.Printf("Warning: failed to close file %s: %v", path, closeErr)
			}
		}()

		_, err = io.Copy(writer, f)
		return err
	})
}
