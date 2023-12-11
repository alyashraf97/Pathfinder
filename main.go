package main

import (
	"archive/zip"
	"bufio"
	"flag"
	"fmt"
	"io"
	"log"
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

func main() {
	// Define flags at the global scope
	var (
		defaultListFile   = "pathfinder.txt"
		defaultOutputPath = "."
	)

	// Use Pathfinder directory as default search directory
	cwd, _ := os.Getwd()
	defaultDirectory := filepath.Join(cwd, "Pathfinder")

	flag.StringVar(&directory, "d", defaultDirectory, "Directory to search for files")
	flag.StringVar(&listFile, "l", filepath.Join(".", defaultListFile), "Text file with file lists")
	flag.StringVar(&outputPath, "p", defaultOutputPath, "Optional: Output path for the zip archive")
	flag.StringVar(&outputName, "n", "", "Optional: Output archive name")
	flag.BoolVar(&verbose, "v", false, "Enable verbose mode")

	flag.Parse()

	// Check if the specified directory exists
	if _, err := os.Stat(directory); os.IsNotExist(err) {
		fmt.Println("Error: The specified directory does not exist.")
		os.Exit(1)
	}

	// Check if the specified list file exists
	if _, err := os.Stat(listFile); os.IsNotExist(err) {
		fmt.Println("Error: The specified list file does not exist.")
		os.Exit(1)
	}

	// Read the text file
	readTextFile(listFile)

	// Create a new zip archive
	outputFilename := generateOutputFilename(outputName)
	outputPathAndName := filepath.Join(outputPath, outputFilename)

	if err := createZipArchive(outputPathAndName); err != nil {
		fmt.Println("Error creating zip archive:", err)
		os.Exit(1)
	}

	defer closeResources()

	// Search for files in the specified directory
	searchFiles(directory)

	if verbose {
		fmt.Printf("New zip archive created: %s\n", outputFilename)
	}
}

// readTextFile reads a text file and categorizes lines into sections.
func readTextFile(filename string) {
	// Open the file
	file, err := os.Open(filename)
	if err != nil {
		log.Fatalf("Error opening text file: %v", err)
	}
	defer file.Close()

	var section string
	scanner := bufio.NewScanner(file)

	// Scan the file line by line
	for scanner.Scan() {
		line := scanner.Text()

		// Check if the line is a section header
		isSectionHeader := strings.HasPrefix(line, "[") && strings.HasSuffix(line, "]")
		if isSectionHeader {
			section = line[1 : len(line)-1]
			continue
		}

		// Categorize the line based on the current section
		switch section {
		case "files":
			fileNames = append(fileNames, line)
		case "paths":
			filePaths = append(filePaths, line)
		case "directories":
			directories = append(directories, line)
		}
	}

	// Check for errors during scanning
	if err := scanner.Err(); err != nil {
		fmt.Printf("Error reading text file: %v", err)
	}
}

func searchFiles(dir string) {
	filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		handleFileByNames(path, info)
		handleFileByPaths(path, info)
		handleDirectories(path, info)

		return nil
	})
}

func handleFileByNames(path string, info os.FileInfo) {
	if !info.IsDir() && contains(info.Name(), fileNames) {
		if verbose {
			fmt.Printf("Found by name: %s\n", path)
		}

		// Add the file to the new zip archive
		if err := addToZipArchive(path); err != nil {
			fmt.Println("Error adding file to archive:", err)
		}
	}
}

func handleFileByPaths(path string, info os.FileInfo) {
	if info.IsDir() {
		return
	}

	// Check if the path contains any specified paths
	for _, specifiedPath := range filePaths {
		if strings.HasPrefix(path, specifiedPath) {
			handleFoundPath(path)
			break
		}
	}
}

func handleFoundPath(path string) {
	if verbose {
		fmt.Printf("Found by path: %s\n", path)
	}

	// Add the file to the new zip archive
	if err := addToZipArchive(path); err != nil {
		fmt.Println("Error adding file to archive:", err)
	}
}

func handleDirectories(path string, info os.FileInfo) {
	if info.IsDir() && isUnderDirectory(path, directories) {
		if verbose {
			fmt.Printf("Found under directory: %s\n", path)
		}

		// Add all files under the directory to the new zip archive
		if err := filepath.Walk(path, addFilesToZip); err != nil {
			fmt.Println("Error walking through directory:", err)
		}
	}
}

func addFilesToZip(subPath string, subInfo os.FileInfo, subErr error) error {
	if subErr != nil {
		return subErr
	}
	if !subInfo.IsDir() {
		// Add the file to the new zip archive
		if err := addToZipArchive(subPath); err != nil {
			fmt.Println("Error adding file to archive:", err)
		}
	}
	return nil
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

func createZipArchive(outputPathAndName string) error {
	archiveFile, err := os.Create(outputPathAndName)
	if err != nil {
		return err
	}

	// Create a new zip writer
	zipWriter = zip.NewWriter(archiveFile)
	return nil
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

func closeResources() {
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
}
