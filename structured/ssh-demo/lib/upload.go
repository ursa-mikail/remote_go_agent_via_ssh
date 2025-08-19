package lib

import (
	"fmt"
	"os"
	"path"
	"strings"

	"github.com/pkg/sftp"
	"golang.org/x/crypto/ssh"
)

// UploadFile sends a local file to a remote absolute path using SFTP.
// It creates the parent directory if needed and sets 0644 perms by default.
func UploadFile(client *ssh.Client, localPath, remotePath string) error {
	s, err := sftp.NewClient(client)
	if err != nil {
		return fmt.Errorf("sftp: %w", err)
	}
	defer s.Close()

	parent := path.Dir(remotePath)
	_ = s.MkdirAll(parent)

	src, err := os.Open(localPath)
	if err != nil {
		return fmt.Errorf("open local: %w", err)
	}
	defer src.Close()

	dst, err := s.Create(remotePath)
	if err != nil {
		return fmt.Errorf("create remote: %w", err)
	}
	defer dst.Close()

	if _, err := dst.ReadFrom(src); err != nil {
		return fmt.Errorf("copy: %w", err)
	}

	_ = s.Chmod(remotePath, 0644)

	if !strings.HasPrefix(remotePath, "/") {
		remotePath = "/" + remotePath
	}

	return nil
}

// DownloadFile downloads a remote file via SFTP to a local path
func DownloadFile(client *ssh.Client, remotePath, localPath string) error {
	s, err := sftp.NewClient(client)
	if err != nil {
		return fmt.Errorf("sftp: %w", err)
	}
	defer s.Close()

	src, err := s.Open(remotePath)
	if err != nil {
		return fmt.Errorf("open remote: %w", err)
	}
	defer src.Close()

	dst, err := os.Create(localPath)
	if err != nil {
		return fmt.Errorf("create local: %w", err)
	}
	defer dst.Close()

	if _, err := dst.ReadFrom(src); err != nil {
		return fmt.Errorf("copy: %w", err)
	}
	return nil
}

