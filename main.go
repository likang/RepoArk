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
	"time"
)

// RootDir represents a directory to be archived with a prefix
type RootDir struct {
	Prefix string
	Dir    string
}

// archiveGitRepo is the main function to create a gzip tar archive of a Git repository
func archiveGitRepo(repoPath string, outputPath string) error {
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
	archiveFile, err := os.Create(outputPath)
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

	fmt.Printf("Successfully created archive: %s\n", outputPath)
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

	fmt.Printf("add %s\n", archivePath)
	// Write header
	if err := tarWriter.WriteHeader(header); err != nil {
		return err
	}

	// Copy file contents
	_, err = io.Copy(tarWriter, file)
	return err
}

// restoreGitRepo restores a Git repository from a gzip tar archive
func restoreGitRepo(repoPath, archiveName string) error {
    // Open the archive file
    archiveFile, err := os.Open(archiveName)
    if err != nil {
    return fmt.Errorf("error opening archive file: %v", err)
    }
    defer archiveFile.Close()

    // Create gzip reader
    gzReader, err := gzip.NewReader(archiveFile)
    if err != nil {
        return fmt.Errorf("error creating gzip reader: %v", err)
    }
    defer gzReader.Close()

    // Create tar reader
    tarReader := tar.NewReader(gzReader)

    // Ensure the repository directory exists
    if err := os.MkdirAll(repoPath, 0755); err != nil {
        return fmt.Errorf("error creating repository directory: %v", err)
    }

    // Extract files from the archive
    for {
        header, err := tarReader.Next()
        if err == io.EOF {
            break // End of archive
        }
        if err != nil {
            return fmt.Errorf("error reading next file from archive: %v", err)
        }

        // Create the file or directory
        targetPath := filepath.Join(repoPath, header.Name)
        switch header.Typeflag {
        case tar.TypeDir:
            if err := os.MkdirAll(targetPath, os.FileMode(header.Mode)); err != nil {
                return fmt.Errorf("error creating directory: %v", err)
            }
        case tar.TypeReg:
			// check localfile first, if exist, and ModTime is the same with header.ModeTime, skip
			if _, err := os.Stat(targetPath); err == nil {
				stat, _ := os.Stat(targetPath)
				if stat.ModTime().Round(time.Second) == header.ModTime.Round(time.Second) {
					fmt.Printf("skip %s\n", targetPath)
					continue
				}
			}
			// Ensure the directory exists
			dir := filepath.Dir(targetPath)
			if err := os.MkdirAll(dir, 0755); err != nil {
				return fmt.Errorf("error creating directory: %v", err)
			}
            file, err := os.OpenFile(targetPath, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, os.FileMode(header.Mode))
            if err != nil {
                return fmt.Errorf("error creating file: %v", err)
            }
            defer file.Close()

			fmt.Printf("restore %s\n", targetPath)

            if _, err := io.Copy(file, tarReader); err != nil {
                return fmt.Errorf("error writing file content: %v", err)
            }
			// restore header.ModTime
			if err := os.Chtimes(targetPath, header.ModTime, header.ModTime); err != nil {
				return fmt.Errorf("error setting file modification time: %v", err)
			}
        default:
            return fmt.Errorf("unsupported file type: %v", header.Typeflag)
        }
    }

    fmt.Printf("Successfully restored repository to: %s\n", repoPath)
    return nil
}

func findAvailableArchiveName(repoPath string) string {
	baseName := filepath.Base(repoPath)
	// change to current directory
	dirName, _ := os.Getwd()

	archiveName := fmt.Sprintf("%s.tar.gz", baseName)

	// Check if the file exists
	_, err := os.Stat(filepath.Join(dirName, archiveName))
	if err != nil {
		// File does not exist, return the path
		return archiveName
	}

	i := 1
	for {
		archiveName := fmt.Sprintf("%s-%d.tar.gz", baseName, i)
		_, err := os.Stat(filepath.Join(dirName, archiveName))
		if err != nil {
			return archiveName
		}
		i++
	}
}


// print usage information
func printUsage() {
	fmt.Println(`Usage:
repoark <repository-path> [<output-file>]
repoark restore <repository-path> <archive-file>`)
}

// main function to handle command-line input
func main() {
	// Check for repository path argument
	argsLen := len(os.Args)
	if argsLen < 2 {
		printUsage()
		os.Exit(1)
	}

	if os.Args[1] == "restore" {
		if argsLen != 4 {
			printUsage()
			os.Exit(1)
		}
		if err := restoreGitRepo(os.Args[2], os.Args[3]); err != nil {
			fmt.Printf("Error: %v\n", err)
			os.Exit(1)
		}
		return
	}

	if argsLen > 3 {
		printUsage()
		os.Exit(1)
	}

	repoPath := os.Args[1]
	var outputFile string
	if argsLen == 3 {
		outputFile = os.Args[2]
	} else {
		outputFile = findAvailableArchiveName(repoPath)
	}

	if err := archiveGitRepo(repoPath, outputFile); err != nil {
		fmt.Printf("Error: %v\n", err)
		os.Exit(1)
	}
}
