package main

import (
	"archive/zip"
	"bufio"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"
)

var (
	directory   string
	listFile    string
	outputPath  string
	outputName  string
	fileNames   []string
	filePaths   []string
	directories []string
	verbose     bool
)

var zipWriter *zip.Writer
var archiveFile *os.File

func init() {
	flag.StringVar(&directory, "d", "", "Directory to search for files")
	flag.StringVar(&listFile, "l", "", "Text file with file lists")
	flag.StringVar(&outputPath, "p", ".", "Optional: Output path for the zip archive")
	flag.StringVar(&outputName, "n", "", "Optional: Output archive name")
	flag.BoolVar(&verbose, "v", false, "Enable verbose mode")
}

func main() {
	flag.Parse()

	if directory == "" || listFile == "" {
		fmt.Println("Usage: pathfinder -d <directory> -l <text_file> [-p <output_path>] [-n <output_archive_name>] [-v]")
		os.Exit(1)
	}

	// Read the text file
	readTextFile(listFile)

	// Create a new zip archive
	outputFilename := generateOutputFilename(outputName)
	outputPathAndName := filepath.Join(outputPath, outputFilename)

	var err error
	zipWriter, archiveFile, err = createZipArchive(outputPathAndName)
	if err != nil {
		fmt.Println("Error creating zip archive:", err)
		os.Exit(1)
	}
	defer func() {
		if zipWriter != nil {
			// Close the zip writer
			err := zipWriter.Close()
			if err != nil {
				fmt.Println("Error closing zip writer:", err)
			}
		}
		if archiveFile != nil {
			// Close the archive file
			err := archiveFile.Close()
			if err != nil {
				fmt.Println("Error closing archive file:", err)
			}
		}
	}()

	// Search for files in the specified directory
	searchFiles(directory)

	if verbose {
		fmt.Printf("New zip archive created: %s\n", outputFilename)
	}
}

func readTextFile(filename string) {
	file, err := os.Open(filename)
	if err != nil {
		fmt.Println("Error opening text file:", err)
		os.Exit(1)
	}
	defer file.Close()

	var section string

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := scanner.Text()

		if strings.HasPrefix(line, "[") && strings.HasSuffix(line, "]") {
			section = line[1 : len(line)-1]
		} else {
			switch section {
			case "files":
				fileNames = append(fileNames, line)
			case "paths":
				filePaths = append(filePaths, line)
			case "directories":
				directories = append(directories, line)
			}
		}
	}

	if err := scanner.Err(); err != nil {
		fmt.Println("Error reading text file:", err)
		os.Exit(1)
	}
}

func searchFiles(dir string) {
	filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		// Handle files by names
		if !info.IsDir() && contains(info.Name(), fileNames) {
			if verbose {
				fmt.Printf("Found by name: %s\n", path)
			}

			// Add the file to the new zip archive
			err := addToZipArchive(path)
			if err != nil {
				fmt.Println("Error adding file to archive:", err)
			}
		}

		// Handle files by paths
		if !info.IsDir() {
			// Check if the path contains any specified paths
			for _, specifiedPath := range filePaths {
				if strings.HasPrefix(path, specifiedPath) {
					if verbose {
						fmt.Printf("Found by path: %s\n", path)
					}

					// Add the file to the new zip archive
					err := addToZipArchive(path)
					if err != nil {
						fmt.Println("Error adding file to archive:", err)
					}
					break
				}
			}
		}

		// Handle directories
		if info.IsDir() && isUnderDirectory(path, directories) {
			if verbose {
				fmt.Printf("Found under directory: %s\n", path)
			}

			// Add all files under the directory to the new zip archive
			err := filepath.Walk(path, func(subPath string, subInfo os.FileInfo, subErr error) error {
				if subErr != nil {
					return subErr
				}
				if !subInfo.IsDir() {
					// Add the file to the new zip archive
					err := addToZipArchive(subPath)
					if err != nil {
						fmt.Println("Error adding file to archive:", err)
					}
				}
				return nil
			})
			if err != nil {
				fmt.Println("Error walking through directory:", err)
			}
		}

		return nil
	})
}

func generateOutputFilename(userProvidedName string) string {
	if userProvidedName != "" {
		return userProvidedName
	}
	return fmt.Sprintf("request-%s.zip", time.Now().Format("2006-Jan-02-15-04"))
}

func contains(needle string, haystack []string) bool {
	for _, item := range haystack {
		if needle == item {
			return true
		}
	}
	return false
}

func isUnderDirectory(filePath string, directories []string) bool {
	for _, dir := range directories {
		if strings.HasPrefix(filePath, dir) {
			return true
		}
	}
	return false
}

func createZipArchive(outputPathAndName string) (*zip.Writer, *os.File, error) {
	archiveFile, err := os.Create(outputPathAndName)
	if err != nil {
		return nil, nil, err
	}

	// Create a new zip writer
	zipWriter := zip.NewWriter(archiveFile)

	return zipWriter, archiveFile, nil
}

func addToZipArchive(filePath string) error {
	sourceFile, err := os.Open(filePath)
	if err != nil {
		return fmt.Errorf("failed to open source file: %w", err)
	}
	defer sourceFile.Close()

	entry, err := zipWriter.Create(filepath.Base(filePath))
	if err != nil {
		return fmt.Errorf("failed to create entry in zip file: %w", err)
	}

	_, err = io.Copy(entry, sourceFile)
	if err != nil {
		return fmt.Errorf("failed to copy file content to zip archive: %w", err)
	}

	return nil
}
