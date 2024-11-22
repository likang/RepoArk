package main

import (
	"archive/tar"
	"compress/gzip"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// RootDir represents a directory to be archived with a prefix
type RootDir struct {
	Prefix string
	Dir    string
}

// archiveGitRepo is the main function to create a gzip tar archive of a Git repository
func archiveGitRepo(repoPath string) error {
	// Validate repository path
	info, err := os.Stat(repoPath)
	if err != nil {
		return fmt.Errorf("error accessing path: %v", err)
	}

	if !info.IsDir() {
		return fmt.Errorf("%s is not a directory", repoPath)
	}

	// Check if it's a valid Git repository
	cmd := exec.Command("git", "-C", repoPath, "rev-parse", "--is-inside-work-tree")
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("%s is not a valid Git repository", repoPath)
	}

	// Create output archive file
	archiveName := filepath.Base(repoPath) + ".tar.gz"
	archiveFile, err := os.Create(archiveName)
	if err != nil {
		return fmt.Errorf("error creating archive file: %v", err)
	}
	defer archiveFile.Close()

	// Create gzip writer
	gzWriter := gzip.NewWriter(archiveFile)
	defer gzWriter.Close()

	// Create tar writer
	tarWriter := tar.NewWriter(gzWriter)
	defer tarWriter.Close()

	// Initialize directory list
	dirList := []RootDir{
		{
			Prefix: "",
			Dir:    repoPath,
		},
	}

	// Add entries to archive
	if err := addEntry(tarWriter, dirList); err != nil {
		return err
	}

	fmt.Printf("Successfully created archive: %s\n", archiveName)
	return nil
}

// addEntry recursively adds files and directories to the tar archive
func addEntry(tarWriter *tar.Writer, dirList []RootDir) error {
	// Check if directory list is empty
	if len(dirList) == 0 {
		return nil
	}

	// Pop the first RootDir
	rootDir := (dirList)[0]
	dirList = (dirList)[1:]

	// Get tracked and untracked files
	cmd := exec.Command("git", "-C", rootDir.Dir, "ls-files", "--others", "--exclude-standard", "--cached")
	output, err := cmd.Output()
	if err != nil {
		return fmt.Errorf("error listing files in %s: %v", rootDir.Dir, err)
	}

	// Process each file/directory
	files := strings.Split(string(output), "\n")
	for _, entry := range files {
		if entry == "" {
			continue
		}

		fullPath := filepath.Join(rootDir.Dir, entry)
		archivePath := filepath.Join(rootDir.Prefix, entry)

		// Skip non-existent paths
		info, err := os.Stat(fullPath)
		if err != nil {
			continue
		}

		if info.IsDir() {
			// Check if it's a submodule
			cmd := exec.Command("git", "-C", rootDir.Dir, "submodule", "status", entry)
			if err := cmd.Run(); err == nil {
				// Add submodule to directory list
				dirList = append(dirList, RootDir{
					Prefix: filepath.Join(rootDir.Prefix, archivePath),
					Dir:    fullPath,
				})
				continue
			}
		} else {
			// Add file to archive
			if err := addFileToArchive(tarWriter, fullPath, archivePath); err != nil {
				return err
			}
		}
	}

	// Add .git directory contents
	gitDir := filepath.Join(rootDir.Dir, ".git")
	if err := filepath.Walk(gitDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		// Create archive path relative to .git directory
		relativePath, err := filepath.Rel(rootDir.Dir, path)
		if err != nil {
			return err
		}
		archivePath := filepath.Join(rootDir.Prefix, relativePath)

		if !info.IsDir() {
			return addFileToArchive(tarWriter, path, archivePath)
		}
		return nil
	}); err != nil {
		return fmt.Errorf("error walking .git directory: %v", err)
	}

	// Recursively process remaining directories
	return addEntry(tarWriter, dirList)
}

// addFileToArchive adds a single file to the tar archive
func addFileToArchive(tarWriter *tar.Writer, sourcePath, archivePath string) error {
	file, err := os.Open(sourcePath)
	if err != nil {
		return err
	}
	defer file.Close()

	info, err := file.Stat()
	if err != nil {
		return err
	}

	// Create tar header
	header := &tar.Header{
		Name:    archivePath,
		Size:    info.Size(),
		Mode:    int64(info.Mode()),
		ModTime: info.ModTime(),
	}

	// Write header
	if err := tarWriter.WriteHeader(header); err != nil {
		return err
	}

	// Copy file contents
	_, err = io.Copy(tarWriter, file)
	return err
}

// main function to handle command-line input
func main() {
	// Check for repository path argument
	if len(os.Args) < 2 {
		fmt.Println("Usage: git-archiver <repository-path>")
		os.Exit(1)
	}

	repoPath := os.Args[1]
	if err := archiveGitRepo(repoPath); err != nil {
		fmt.Printf("Error: %v\n", err)
		os.Exit(1)
	}
}