package main

import (
	"archive/zip"
	"bufio"
	"io"
	"os"
	"path/filepath"
	"strings"

	gitignore "github.com/sabhiram/go-gitignore"
)

// ToZipWithExclusions creates a zip file from src, ignoring files that match
// the ignore patterns from vivgridIgnoreFile or any applicable .gitignore rule.
func ZipWithExclusions(src, dst, vivgridIgnoreFile string) error {
	zipFile, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer zipFile.Close()

	zipWriter := zip.NewWriter(zipFile)
	defer zipWriter.Close()

	// Read ignore patterns from vivgridIgnoreFile.
	f, err := os.Open(vivgridIgnoreFile)
	if err != nil {
		return err
	}
	defer f.Close()

	scanner := bufio.NewScanner(f)
	var baseIgnorePatterns []string
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		// Skip empty lines and comments.
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		baseIgnorePatterns = append(baseIgnorePatterns, line)
	}
	if err := scanner.Err(); err != nil {
		return err
	}

	// Cache for compiled .gitignore files: key is directory path.
	gitIgnoreCache := make(map[string]*gitignore.GitIgnore)

	// Helper to check if the path should be skipped by base ignore rules.
	shouldSkipBase := func(relPath string, info os.FileInfo) bool {
		parts := strings.Split(relPath, string(os.PathSeparator))
		// Check each segment for exact match.
		for _, part := range parts {
			for _, pattern := range baseIgnorePatterns {
				// If pattern contains wildcard, match against the filename.
				if strings.Contains(pattern, "*") {
					matched, err := filepath.Match(pattern, part)
					if err == nil && matched {
						return true
					}
				} else if part == pattern {
					return true
				}
			}
		}
		return false
	}

	// Helper to check .gitignore rules in effect for a given file.
	shouldSkipGitignore := func(absPath, relPath string) bool {
		// Walk from src up to the file's directory.
		dir := filepath.Dir(absPath)
		// Build a slice of directories from src to dir.
		var dirs []string
		for {
			absSrc, err := filepath.Abs(src)
			if err != nil {
				break
			}
			absDir, err := filepath.Abs(dir)
			if err != nil {
				break
			}
			if !strings.HasPrefix(absDir, absSrc) {
				break
			}
			dirs = append([]string{dir}, dirs...) // prepend
			if absDir == absSrc {
				break
			}
			dir = filepath.Dir(dir)
		}

		// For each directory, if a .gitignore exists, check if it ignores the file.
		for _, d := range dirs {
			ignoreFile := filepath.Join(d, ".gitignore")
			gi, ok := gitIgnoreCache[d]
			if !ok {
				// Try to load .gitignore from this directory.
				if _, err := os.Stat(ignoreFile); err == nil {
					compiled, err := gitignore.CompileIgnoreFile(ignoreFile)
					if err == nil {
						gi = compiled
					}
				}
				// Cache nil if not present or failed.
				gitIgnoreCache[d] = gi
			}
			if gi != nil {
				// Determine the path to test relative to the directory containing .gitignore.
				relTest, err := filepath.Rel(d, absPath)
				if err != nil {
					continue
				}
				if gi.MatchesPath(relTest) {
					return true
				}
			}
		}
		return false
	}

	// Walk through src.
	err = filepath.Walk(src, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		// Compute path relative to src.
		relPath, err := filepath.Rel(src, path)
		if err != nil {
			return err
		}
		// Skip the root directory.
		if relPath == "." {
			return nil
		}

		// Check base ignore patterns.
		if shouldSkipBase(relPath, info) {
			// If it's a directory then skip its children.
			if info.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}

		// Check .gitignore rules.
		absPath, err := filepath.Abs(path)
		if err != nil {
			return err
		}
		if shouldSkipGitignore(absPath, relPath) {
			if info.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}

		// If it's a directory, continue walking.
		if info.IsDir() {
			return nil
		}

		// Open the file.
		file, err := os.Open(path)
		if err != nil {
			return err
		}
		defer file.Close()

		// Create zip header using the relative file path.
		header, err := zip.FileInfoHeader(info)
		if err != nil {
			return err
		}
		// Ensure consistent use of forward slashes.
		header.Name = filepath.ToSlash(relPath)
		header.Method = zip.Deflate

		writer, err := zipWriter.CreateHeader(header)
		if err != nil {
			return err
		}
		_, err = io.Copy(writer, file)
		return err
	})
	if err != nil {
		return err
	}
	return nil
}
