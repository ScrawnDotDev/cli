package setup

import (
	"archive/zip"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/ScrawnDotDev/scrawn-cli/internal/apperr"
)

func scaffoldRepo(target string) error {
	tempDir, err := os.MkdirTemp("", "scrawn-cli-*")
	if err != nil {
		return &apperr.CommandError{Summary: "failed to create temp directory", Detail: err.Error()}
	}
	defer os.RemoveAll(tempDir)

	archivePath := filepath.Join(tempDir, "scrawn.zip")
	if err := downloadFile(GitHubZipURL, archivePath); err != nil {
		return err
	}

	extractDir := filepath.Join(tempDir, "extract")
	if err := unzip(archivePath, extractDir); err != nil {
		return err
	}

	entries, err := os.ReadDir(extractDir)
	if err != nil {
		return &apperr.CommandError{Summary: "failed to inspect extracted archive", Detail: err.Error()}
	}
	if len(entries) != 1 || !entries[0].IsDir() {
		return &apperr.CommandError{Summary: "unexpected archive layout", Detail: "GitHub download did not contain a single project directory"}
	}

	sourceRoot := filepath.Join(extractDir, entries[0].Name())
	if err := copyDirContents(sourceRoot, target); err != nil {
		return err
	}

	return nil
}

func downloadFile(downloadURL string, destination string) error {
	client := &http.Client{Timeout: 60 * time.Second}
	resp, err := client.Get(downloadURL)
	if err != nil {
		return &apperr.CommandError{Summary: "failed to download Scrawn", Detail: err.Error()}
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return &apperr.CommandError{Summary: "failed to download Scrawn", Detail: fmt.Sprintf("GitHub returned %s", resp.Status)}
	}

	file, err := os.Create(destination)
	if err != nil {
		return &apperr.CommandError{Summary: "failed to create download file", Detail: err.Error()}
	}
	defer file.Close()

	if _, err := io.Copy(file, resp.Body); err != nil {
		return &apperr.CommandError{Summary: "failed to save downloaded archive", Detail: err.Error()}
	}

	return nil
}

func unzip(archivePath string, destination string) error {
	reader, err := zip.OpenReader(archivePath)
	if err != nil {
		return &apperr.CommandError{Summary: "failed to open downloaded archive", Detail: err.Error()}
	}
	defer reader.Close()

	for _, file := range reader.File {
		targetPath := filepath.Join(destination, file.Name)
		cleanDestination := filepath.Clean(destination) + string(os.PathSeparator)
		cleanTarget := filepath.Clean(targetPath)
		if !strings.HasPrefix(cleanTarget, cleanDestination) && cleanTarget != filepath.Clean(destination) {
			return &apperr.CommandError{Summary: "unsafe archive path detected", Detail: file.Name}
		}

		if file.FileInfo().IsDir() {
			if err := os.MkdirAll(targetPath, 0o755); err != nil {
				return &apperr.CommandError{Summary: "failed to create extracted directory", Detail: err.Error()}
			}
			continue
		}

		if err := os.MkdirAll(filepath.Dir(targetPath), 0o755); err != nil {
			return &apperr.CommandError{Summary: "failed to prepare extracted directory", Detail: err.Error()}
		}

		src, err := file.Open()
		if err != nil {
			return &apperr.CommandError{Summary: "failed to read archive entry", Detail: err.Error()}
		}

		dst, err := os.OpenFile(targetPath, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, file.Mode())
		if err != nil {
			src.Close()
			return &apperr.CommandError{Summary: "failed to create extracted file", Detail: err.Error()}
		}

		_, copyErr := io.Copy(dst, src)
		closeErr := src.Close()
		writeCloseErr := dst.Close()
		if copyErr != nil {
			return &apperr.CommandError{Summary: "failed to extract file", Detail: copyErr.Error()}
		}
		if closeErr != nil {
			return &apperr.CommandError{Summary: "failed to close archive entry", Detail: closeErr.Error()}
		}
		if writeCloseErr != nil {
			return &apperr.CommandError{Summary: "failed to finalize extracted file", Detail: writeCloseErr.Error()}
		}
	}

	return nil
}

func copyDirContents(source string, target string) error {
	entries, err := os.ReadDir(source)
	if err != nil {
		return &apperr.CommandError{Summary: "failed to read scaffold contents", Detail: err.Error()}
	}

	for _, entry := range entries {
		srcPath := filepath.Join(source, entry.Name())
		dstPath := filepath.Join(target, entry.Name())

		if entry.IsDir() {
			if err := os.MkdirAll(dstPath, 0o755); err != nil {
				return &apperr.CommandError{Summary: "failed to create scaffold directory", Detail: err.Error()}
			}
			if err := copyDirContents(srcPath, dstPath); err != nil {
				return err
			}
			continue
		}

		if err := copyFile(srcPath, dstPath); err != nil {
			return err
		}
	}

	return nil
}

func copyFile(source string, target string) error {
	src, err := os.Open(source)
	if err != nil {
		return &apperr.CommandError{Summary: "failed to open scaffold file", Detail: err.Error()}
	}
	defer src.Close()

	info, err := src.Stat()
	if err != nil {
		return &apperr.CommandError{Summary: "failed to inspect scaffold file", Detail: err.Error()}
	}

	dst, err := os.OpenFile(target, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, info.Mode())
	if err != nil {
		return &apperr.CommandError{Summary: "failed to create scaffold file", Detail: err.Error()}
	}
	defer dst.Close()

	if _, err := io.Copy(dst, src); err != nil {
		return &apperr.CommandError{Summary: "failed to copy scaffold file", Detail: err.Error()}
	}

	return nil
}

func DownloadDockerCompose(targetDir string) error {
	client := &http.Client{Timeout: 60 * time.Second}
	resp, err := client.Get(DockerComposeURL)
	if err != nil {
		return &apperr.CommandError{Summary: "failed to download compose file", Detail: err.Error()}
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return &apperr.CommandError{Summary: "failed to download compose file", Detail: fmt.Sprintf("server returned %s", resp.Status)}
	}

	dst := filepath.Join(targetDir, DockerComposeFileName)
	out, err := os.Create(dst)
	if err != nil {
		return &apperr.CommandError{Summary: "failed to create compose file", Detail: err.Error()}
	}
	defer out.Close()

	if _, err := io.Copy(out, resp.Body); err != nil {
		return &apperr.CommandError{Summary: "failed to write compose file", Detail: err.Error()}
	}

	return nil
}
