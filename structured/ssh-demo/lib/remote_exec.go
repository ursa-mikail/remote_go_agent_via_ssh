package lib

import (
	"bytes"
	"fmt"

	"golang.org/x/crypto/ssh"
)

// RunRemoteCommand executes a command over SSH and returns exit code, stdout, stderr
func RunRemoteCommand(client *ssh.Client, cmd string) (int, string, string, error) {
	session, err := client.NewSession()
	if err != nil {
		return -1, "", "", fmt.Errorf("new session: %w", err)
	}
	defer session.Close()

	var stdout, stderr bytes.Buffer
	session.Stdout = &stdout
	session.Stderr = &stderr

	err = session.Run(cmd)
	exitCode := 0
	if err != nil {
		if e, ok := err.(*ssh.ExitError); ok {
			exitCode = e.ExitStatus()
		} else {
			return -1, "", "", fmt.Errorf("run: %w", err)
		}
	}

	return exitCode, stdout.String(), stderr.String(), nil
}
