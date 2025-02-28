package main

import (
	"archive/zip"
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
	// if err != nil {
	// 	if !os.IsNotExist(err) {
	// 		return err
	// 	}
	// }

	gitMatcher, _ := dotignore.NewPatternMatcherFromFile(".gitignore")
	// if err != nil {
	// 	log.Println("** .gitignore absence")
	// }

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

		log.Println("\t ** add to zip, [", path, "]")
		return nil

		// // add the file to the zip archive
		// relPath, err := filepath.Rel(src, path)
		// if err != nil {
		// 	return err
		// }

		// file, err := os.Open(path)
		// if err != nil {
		// 	return err
		// }
		// defer file.Close()

		// zipFile, err := zipWriter.Create(relPath)
		// if err != nil {
		// 	return err
		// }

		// _, err = io.Copy(zipFile, file)
		// return err
	})
}
