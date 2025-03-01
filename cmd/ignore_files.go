package main

import (
	"archive/zip"
	"io"
	"log"
	"os"
	"path/filepath"

	"github.com/codeglyph/go-dotignore"
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

	vivMatcher, _ := dotignore.NewPatternMatcherFromFile(".vivgridignore")
	gitMatcher, _ := dotignore.NewPatternMatcherFromFile(".gitignore")

	// traverse the src directory, check each file against the ignore patterns
	// and add it to the zip file if it doesn't match
	return filepath.WalkDir(src, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			log.Println("\t --err:", err)
			return err
		}

		// ignore directories
		if d.IsDir() {
			return nil
		}

		// check if the file should be ignored
		isIgnored := false
		if vivMatcher != nil {
			isIgnored, _ = vivMatcher.Matches(path)
		}

		if gitMatcher != nil && !isIgnored {
			isIgnored, _ = gitMatcher.Matches(path)
		}

		if isIgnored {
			return nil
		}

		// Compute relative path for zip header.
		relPath, err := filepath.Rel(src, path)
		if err != nil {
			return err
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
		// Ensure consistent use of forward slashes.
		header.Name = filepath.ToSlash(relPath)
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
		defer f.Close()

		_, err = io.Copy(writer, f)
		return err
	})
}
