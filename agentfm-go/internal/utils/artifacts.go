package utils

import (
	"archive/zip"
	"fmt"
	"io"
	"os"
	"path/filepath"
)

func ZipDirectory(sourceDir, destZip string) error {
	info, err := os.Stat(sourceDir)
	if err != nil {
		return fmt.Errorf("source directory missing or invalid: %w", err)
	}
	if !info.IsDir() {
		return fmt.Errorf("source path is not a directory: %s", sourceDir)
	}

	zipFile, err := os.Create(destZip)
	if err != nil {
		return fmt.Errorf("failed to create zip: %w", err)
	}

	archive := zip.NewWriter(zipFile)

	walkErr := filepath.Walk(sourceDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		relPath, err := filepath.Rel(sourceDir, path)
		if err != nil {
			return err
		}

		if relPath == "." {
			return nil
		}

		if info.Mode()&os.ModeSymlink != 0 {
			return nil
		}

		header, err := zip.FileInfoHeader(info)
		if err != nil {
			return err
		}

		header.Name = filepath.ToSlash(relPath)

		if info.IsDir() {
			header.Name += "/"
		} else {
			header.Method = zip.Deflate
		}

		writer, err := archive.CreateHeader(header)
		if err != nil {
			return err
		}

		if info.IsDir() {
			return nil
		}

		return copyFileIntoZip(path, writer)
	})

	closeErr := archive.Close()
	if cerr := zipFile.Close(); cerr != nil && closeErr == nil {
		closeErr = cerr
	}
	if walkErr != nil {
		return walkErr
	}
	if closeErr != nil {
		return fmt.Errorf("finalize zip: %w", closeErr)
	}
	return nil
}

func copyFileIntoZip(path string, writer io.Writer) error {
	file, err := os.Open(path)
	if err != nil {
		return err
	}
	defer file.Close()

	_, err = io.Copy(writer, file)
	return err
}
