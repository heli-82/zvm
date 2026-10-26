package main

import (
	"archive/tar"
	"archive/zip"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/ulikunitz/xz"
)

var UnsupportedArchiveType = errors.New("Unsupported archive type")

func Extract(archivePath, targetDir string) (string, error) {
	if filepath.Ext(archivePath) == ".zip" {
		return ExtractZip(archivePath, targetDir)
	} else if filepath.Ext(archivePath) == ".xz" {
		return ExtractTarXz(archivePath, targetDir)
	}
	return "", UnsupportedArchiveType
}

func archiveBaseName(p string) string {
	n := filepath.Base(p)
	for _, s := range []string{".tar.xz", ".zip"} {
		n = strings.TrimSuffix(n, s)
	}
	return n
}

func ExtractTarXz(archivePath, targetDir string) (string, error) {
	file, err := os.Open(archivePath)
	if err != nil {
		return "", fmt.Errorf("failed to open archive: %w", err)
	}
	defer file.Close()

	xzReader, err := xz.NewReader(file)
	if err != nil {
		return "", fmt.Errorf("failed to initialize xz reader: %w", err)
	}
	tarReader := tar.NewReader(xzReader)

	tmp, err := os.MkdirTemp(targetDir, ".extract-*")
	if err != nil {
		return "", fmt.Errorf("failed to create temp dir: %w", err)
	}
	defer os.RemoveAll(tmp) // no-op after a successful rename

	prefix := filepath.Clean(tmp) + string(os.PathSeparator)

	for {
		header, err := tarReader.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return "", fmt.Errorf("failed to read tar header: %w", err)
		}

		target := filepath.Join(tmp, header.Name)
		if target != filepath.Clean(tmp) && !strings.HasPrefix(target, prefix) {
			return "", fmt.Errorf("illegal path in archive: %s", header.Name)
		}

		switch header.Typeflag {
		case tar.TypeDir:
			if err := os.MkdirAll(target, 0755); err != nil {
				return "", fmt.Errorf("failed to create directory: %w", err)
			}

		case tar.TypeReg:
			if err := os.MkdirAll(filepath.Dir(target), 0755); err != nil {
				return "", fmt.Errorf("failed to create parent directory: %w", err)
			}
			outFile, err := os.OpenFile(target, os.O_CREATE|os.O_RDWR|os.O_TRUNC, header.FileInfo().Mode())
			if err != nil {
				return "", fmt.Errorf("failed to create output file: %w", err)
			}
			if _, err := io.Copy(outFile, tarReader); err != nil {
				outFile.Close()
				return "", fmt.Errorf("failed to write file content: %w", err)
			}
			outFile.Close()
		}
	}

	entries, err := os.ReadDir(tmp)
	if err != nil {
		return "", err
	}

	var name, src string
	if len(entries) == 1 && entries[0].IsDir() {
		name = entries[0].Name()
		src = filepath.Join(tmp, name)
	} else {
		name = archiveBaseName(archivePath)
		src = tmp
		if err := os.Chmod(tmp, 0755); err != nil { // MkdirTemp makes it 0700
			return "", err
		}
	}

	dst := filepath.Join(targetDir, name)
	if err := os.RemoveAll(dst); err != nil {
		return "", err
	}
	if err := os.Rename(src, dst); err != nil {
		return "", err
	}
	return dst, nil
}

func ExtractZip(archivePath, targetDir string) (string, error) {
	file, err := os.Open(archivePath)
	if err != nil {
		return "", fmt.Errorf("failed to open archive: %w", err)
	}
	defer file.Close()

	info, err := file.Stat()
	if err != nil {
		return "", fmt.Errorf("failed to stat archive: %w", err)
	}

	zipReader, err := zip.NewReader(file, info.Size())
	if err != nil {
		return "", fmt.Errorf("failed to initialize zip reader: %w", err)
	}

	var topDir string

	for _, f := range zipReader.File {
		if topDir == "" {
			name := filepath.ToSlash(f.Name)
			if i := strings.IndexByte(name, '/'); i >= 0 {
				topDir = name[:i]
			} else {
				topDir = name
			}
		}

		target := filepath.Join(targetDir, f.Name)

		if f.FileInfo().IsDir() {
			if err := os.MkdirAll(target, 0755); err != nil {
				return "", fmt.Errorf("failed to create directory: %w", err)
			}
			continue
		}

		if err := os.MkdirAll(filepath.Dir(target), 0755); err != nil {
			return "", fmt.Errorf("failed to create parent directory: %w", err)
		}

		rc, err := f.Open()
		if err != nil {
			return "", fmt.Errorf("failed to open file in archive: %w", err)
		}

		outFile, err := os.OpenFile(target, os.O_CREATE|os.O_RDWR|os.O_TRUNC, f.Mode())
		if err != nil {
			rc.Close()
			return "", fmt.Errorf("failed to create output file: %w", err)
		}

		if _, err := io.Copy(outFile, rc); err != nil {
			outFile.Close()
			rc.Close()
			return "", fmt.Errorf("failed to write file content: %w", err)
		}
		outFile.Close()
		rc.Close()
	}

	return filepath.Join(targetDir, topDir), nil
}
