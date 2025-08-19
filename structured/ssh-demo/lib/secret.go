package lib

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"os"
	"path"
	"strings"

	"github.com/hashicorp/vault/shamir" // sss
	"golang.org/x/crypto/ssh"
)

// CreateShamirShares splits a secret into n shares with threshold k
// and uploads each share as key_XX.json to remoteDir
func CreateShamirShares(client *ssh.Client, secret string, n, k int, remoteDir string) ([]string, error) {
	if secret == "" {
		b := make([]byte, 32)
		if _, err := rand.Read(b); err != nil {
			return nil, fmt.Errorf("generate random secret: %w", err)
		}
		secret = hex.EncodeToString(b)
	}

	parts, err := shamir.Split([]byte(secret), n, k)
	if err != nil {
		return nil, fmt.Errorf("split failed: %w", err)
	}

	names := make([]string, 0, n)
	for i, p := range parts {
		filename := fmt.Sprintf("key_%02d.json", i+1)
		tmpFile := path.Join(os.TempDir(), filename)
		if err := os.WriteFile(tmpFile, p, 0644); err != nil {
			return nil, fmt.Errorf("write temp share: %w", err)
		}

		remotePath := path.Join(remoteDir, filename)
		if err := UploadFile(client, tmpFile, remotePath); err != nil {
			return nil, fmt.Errorf("upload share: %w", err)
		}

		names = append(names, filename)
		_ = os.Remove(tmpFile)
	}

	return names, nil
}

// ListRemoteKeys lists key_*.json files in remoteDir
func ListRemoteKeys(client *ssh.Client, remoteDir string) ([]string, error) {
	session, err := client.NewSession()
	if err != nil {
		return nil, fmt.Errorf("new session: %w", err)
	}
	defer session.Close()

	cmd := fmt.Sprintf("ls %s/key_*.json 2>/dev/null || true", remoteDir)
	out, err := session.Output(cmd)
	if err != nil {
		return nil, fmt.Errorf("list remote keys: %w", err)
	}

	var files []string
	for _, f := range strings.Split(string(out), "\n") {
		f = strings.TrimSpace(f)
		if f != "" {
			files = append(files, path.Base(f))
		}
	}
	return files, nil
}

// DownloadRemoteKey downloads a single remote key file to local path
func DownloadRemoteKey(client *ssh.Client, remoteDir, filename, localPath string) error {
	remotePath := path.Join(remoteDir, filename)
	return DownloadFile(client, remotePath, localPath)
}
