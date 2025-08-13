package main

import (
	"archive/zip"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestZipWithExclusions(t *testing.T) {
	// Create a temporary directory for testing
	tempDir, err := os.MkdirTemp("", "zip_test_*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	// Create test project structure
	testProject := filepath.Join(tempDir, "test_project")

	// Create directories
	dirs := []string{
		filepath.Join(testProject, ".git"),
		filepath.Join(testProject, ".git", "refs"),
		filepath.Join(testProject, ".vscode"),
		filepath.Join(testProject, "src"),
		filepath.Join(testProject, "node_modules"),
		filepath.Join(testProject, "temp"),
	}

	for _, dir := range dirs {
		if err := os.MkdirAll(dir, 0755); err != nil {
			t.Fatalf("Failed to create directory %s: %v", dir, err)
		}
	}

	// Create test files
	testFiles := map[string]string{
		// Files that should be ignored (built-in patterns)
		filepath.Join(testProject, ".DS_Store"):                "DS_Store content",
		filepath.Join(testProject, ".git", "config"):           "git config",
		filepath.Join(testProject, ".git", "refs", "HEAD"):     "ref: refs/heads/main",
		filepath.Join(testProject, ".vscode", "settings.json"): `{"editor.fontSize": 14}`,

		// Files that should be included
		filepath.Join(testProject, "README.md"):       "# Test Project",
		filepath.Join(testProject, "src", "main.go"):  "package main\n\nfunc main() {}",
		filepath.Join(testProject, "src", "utils.go"): "package main\n\nfunc helper() {}",
		filepath.Join(testProject, "go.mod"):          "module test\n\ngo 1.21",

		// Files that should be ignored via .gitignore
		filepath.Join(testProject, "node_modules", "package.js"): "// some dependency",
		filepath.Join(testProject, "temp", "cache.log"):          "log content",
		filepath.Join(testProject, "debug.log"):                  "debug info",

		// The .gitignore file itself
		filepath.Join(testProject, ".gitignore"): "node_modules/\n*.log\ntemp/\n",
	}

	for filePath, content := range testFiles {
		if err := os.WriteFile(filePath, []byte(content), 0644); err != nil {
			t.Fatalf("Failed to create file %s: %v", filePath, err)
		}
	}

	// Create zip file
	zipPath := filepath.Join(tempDir, "test_output.zip")
	err = ZipWithExclusions(testProject, zipPath)
	if err != nil {
		t.Fatalf("ZipWithExclusions failed: %v", err)
	}

	// Verify zip file was created
	if _, err := os.Stat(zipPath); os.IsNotExist(err) {
		t.Fatal("Zip file was not created")
	}

	// Read and verify zip contents
	zipReader, err := zip.OpenReader(zipPath)
	if err != nil {
		t.Fatalf("Failed to open zip file: %v", err)
	}
	defer zipReader.Close()

	// Collect all files in the zip
	zipContents := make(map[string]bool)
	for _, file := range zipReader.File {
		zipContents[file.Name] = true
	}

	// Define expected files (should be included)
	expectedFiles := []string{
		"README.md",
		"src/main.go",
		"src/utils.go",
		"go.mod",
		".gitignore", // .gitignore itself should be included
	}

	// Define files that should be excluded
	excludedFiles := []string{
		".DS_Store",
		".git/config",
		".git/refs/HEAD",
		".vscode/settings.json",
		"node_modules/package.js",
		"temp/cache.log",
		"debug.log",
	}

	// Verify expected files are present
	for _, expectedFile := range expectedFiles {
		if !zipContents[expectedFile] {
			t.Errorf("Expected file %s is missing from zip", expectedFile)
		}
	}

	// Verify excluded files are not present
	for _, excludedFile := range excludedFiles {
		if zipContents[excludedFile] {
			t.Errorf("Excluded file %s should not be in zip", excludedFile)
		}
	}

	// Verify we don't have any .git directory entries
	for fileName := range zipContents {
		if strings.HasPrefix(fileName, ".git/") {
			t.Errorf("Git directory file %s should be excluded", fileName)
		}
		if strings.HasPrefix(fileName, ".vscode/") {
			t.Errorf("VSCode directory file %s should be excluded", fileName)
		}
		if strings.HasPrefix(fileName, "node_modules/") {
			t.Errorf("node_modules directory file %s should be excluded", fileName)
		}
		if strings.HasPrefix(fileName, "temp/") {
			t.Errorf("temp directory file %s should be excluded", fileName)
		}
	}

	t.Logf("Zip created successfully with %d files", len(zipContents))
}

func TestZipWithExclusionsNoGitignore(t *testing.T) {
	// Test case where there's no .gitignore file
	tempDir, err := os.MkdirTemp("", "zip_test_no_gitignore_*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	testProject := filepath.Join(tempDir, "simple_project")

	// Create directories
	dirs := []string{
		filepath.Join(testProject, ".git"),
		filepath.Join(testProject, ".vscode"),
		filepath.Join(testProject, "src"),
	}

	for _, dir := range dirs {
		if err := os.MkdirAll(dir, 0755); err != nil {
			t.Fatalf("Failed to create directory %s: %v", dir, err)
		}
	}

	// Create test files (no .gitignore)
	testFiles := map[string]string{
		filepath.Join(testProject, ".DS_Store"):                "DS_Store content",
		filepath.Join(testProject, ".git", "config"):           "git config",
		filepath.Join(testProject, ".vscode", "settings.json"): `{"editor.fontSize": 14}`,
		filepath.Join(testProject, "README.md"):                "# Simple Project",
		filepath.Join(testProject, "src", "main.go"):           "package main\n\nfunc main() {}",
		filepath.Join(testProject, "some.log"):                 "log content", // This should be included (no .gitignore)
	}

	for filePath, content := range testFiles {
		if err := os.WriteFile(filePath, []byte(content), 0644); err != nil {
			t.Fatalf("Failed to create file %s: %v", filePath, err)
		}
	}

	// Create zip file
	zipPath := filepath.Join(tempDir, "simple_output.zip")
	err = ZipWithExclusions(testProject, zipPath)
	if err != nil {
		t.Fatalf("ZipWithExclusions failed: %v", err)
	}

	// Read and verify zip contents
	zipReader, err := zip.OpenReader(zipPath)
	if err != nil {
		t.Fatalf("Failed to open zip file: %v", err)
	}
	defer zipReader.Close()

	zipContents := make(map[string]bool)
	for _, file := range zipReader.File {
		zipContents[file.Name] = true
	}

	// Should include these files
	expectedFiles := []string{
		"README.md",
		"src/main.go",
		"some.log", // No .gitignore, so *.log should be included
	}

	// Should exclude these (built-in patterns only)
	excludedFiles := []string{
		".DS_Store",
		".git/config",
		".vscode/settings.json",
	}

	for _, expectedFile := range expectedFiles {
		if !zipContents[expectedFile] {
			t.Errorf("Expected file %s is missing from zip", expectedFile)
		}
	}

	for _, excludedFile := range excludedFiles {
		if zipContents[excludedFile] {
			t.Errorf("Excluded file %s should not be in zip", excludedFile)
		}
	}
}

func TestZipWithExclusionsErrorCases(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "zip_error_test_*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	// Test with non-existent source directory
	nonExistentSrc := filepath.Join(tempDir, "does_not_exist")
	zipPath := filepath.Join(tempDir, "error_test.zip")

	err = ZipWithExclusions(nonExistentSrc, zipPath)
	if err == nil {
		t.Error("Expected error when source directory does not exist")
	}

	// Test with invalid destination path (directory that doesn't exist)
	testProject := filepath.Join(tempDir, "valid_project")
	if err := os.MkdirAll(testProject, 0755); err != nil {
		t.Fatalf("Failed to create test project: %v", err)
	}

	// Create a simple file
	testFile := filepath.Join(testProject, "test.txt")
	if err := os.WriteFile(testFile, []byte("test content"), 0644); err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	invalidZipPath := filepath.Join(tempDir, "nonexistent", "test.zip")
	err = ZipWithExclusions(testProject, invalidZipPath)
	if err == nil {
		t.Error("Expected error when destination directory does not exist")
	}
}

func TestZipWithExclusionsEmptyDirectory(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "zip_empty_test_*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	// Create empty project directory
	emptyProject := filepath.Join(tempDir, "empty_project")
	if err := os.MkdirAll(emptyProject, 0755); err != nil {
		t.Fatalf("Failed to create empty project: %v", err)
	}

	// Create zip from empty directory
	zipPath := filepath.Join(tempDir, "empty.zip")
	err = ZipWithExclusions(emptyProject, zipPath)
	if err != nil {
		t.Fatalf("ZipWithExclusions failed on empty directory: %v", err)
	}

	// Verify zip file exists and is valid
	zipReader, err := zip.OpenReader(zipPath)
	if err != nil {
		t.Fatalf("Failed to open zip file: %v", err)
	}
	defer zipReader.Close()

	// Should have no files
	if len(zipReader.File) != 0 {
		t.Errorf("Expected empty zip, but got %d files", len(zipReader.File))
	}
}
