package main

import (
	"archive/tar"
	"compress/gzip"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
)

func main() {
	if len(os.Args) < 3 {
		fmt.Println("Usage: go run download_coredns.go <url> <target_dir>")
		os.Exit(1)
	}
	url := os.Args[1]
	targetDir := os.Args[2]

	fmt.Printf("Downloading %s to %s...\n", url, targetDir)

	resp, err := http.Get(url)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error downloading: %v\n", err)
		os.Exit(1)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		fmt.Fprintf(os.Stderr, "Bad status: %s\n", resp.Status)
		os.Exit(1)
	}

	gr, err := gzip.NewReader(resp.Body)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error creating gzip reader: %v\n", err)
		os.Exit(1)
	}
	defer gr.Close()

	tr := tar.NewReader(gr)

	for {
		hdr, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error reading tar: %v\n", err)
			os.Exit(1)
		}

		// Strip the first directory component (e.g. coredns-1.14.4/)
		parts := strings.Split(hdr.Name, "/")
		if len(parts) <= 1 {
			continue
		}
		relPath := filepath.Join(parts[1:]...)
		destPath := filepath.Join(targetDir, relPath)

		info := hdr.FileInfo()
		if info.IsDir() {
			if err := os.MkdirAll(destPath, info.Mode()); err != nil {
				fmt.Fprintf(os.Stderr, "Error making dir: %v\n", err)
				os.Exit(1)
			}
			continue
		}

		// Ensure parent directory exists
		if err := os.MkdirAll(filepath.Dir(destPath), 0755); err != nil {
			fmt.Fprintf(os.Stderr, "Error making parent dir: %v\n", err)
			os.Exit(1)
		}

		// Create/open the file
		f, err := os.OpenFile(destPath, os.O_CREATE|os.O_RDWR|os.O_TRUNC, info.Mode())
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error opening file: %v\n", err)
			os.Exit(1)
		}

		if _, err := io.Copy(f, tr); err != nil {
			f.Close()
			fmt.Fprintf(os.Stderr, "Error copying file: %v\n", err)
			os.Exit(1)
		}
		f.Close()
	}

	fmt.Println("Download and extraction completed successfully!")
}
