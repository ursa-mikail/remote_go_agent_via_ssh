package lib

import (
	"fmt"
	"os"
	"time"

	"github.com/pkg/sftp"
	"golang.org/x/crypto/ssh"
)

// WriteRemoteLog appends a line to a remote log file (creating it if needed).
func WriteRemoteLog(client *ssh.Client, remotePath, message string) error {
	s, err := sftp.NewClient(client)
	if err != nil {
		return fmt.Errorf("sftp: %w", err)
	}
	defer s.Close()

	f, err := s.OpenFile(remotePath, os.O_CREATE|os.O_WRONLY|os.O_APPEND)
	if err != nil {
		return fmt.Errorf("open remote log: %w", err)
	}
	defer f.Close()

	line := fmt.Sprintf("%s %s\n", time.Now().Format(time.RFC3339), message)
	if _, err := f.Write([]byte(line)); err != nil {
		return fmt.Errorf("write: %w", err)
	}
	return nil
}
