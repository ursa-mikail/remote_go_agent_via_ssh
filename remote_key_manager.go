package main

import (
	"fmt"
	"io/ioutil"
	"log"
	"os"
	"path"
	"strings"

	"github.com/pkg/sftp"
	"golang.org/x/crypto/ssh"
)

func main() {
	sshHost := os.Getenv("SSH_HOST")
	sshPass := os.Getenv("SSH_PASS")

	if sshHost == "" || sshPass == "" {
		log.Fatal("SSH_HOST and SSH_PASS must be set")
	}

	user, host := parseUserHost(sshHost)

	config := &ssh.ClientConfig{
		User:            user,
		Auth:            []ssh.AuthMethod{ssh.Password(sshPass)},
		HostKeyCallback: ssh.InsecureIgnoreHostKey(),
	}

	client, err := ssh.Dial("tcp", host+":22", config)
	if err != nil {
		log.Fatalf("Failed to connect: %v", err)
	}
	defer client.Close()

	// Start SFTP
	sftpClient, err := sftp.NewClient(client)
	if err != nil {
		log.Fatalf("Failed to start SFTP: %v", err)
	}
	defer sftpClient.Close()

	remoteDir := "/tmp/keys"
	sftpClient.Mkdir(remoteDir) // ignore error if exists

	// Create key_01.json ... key_05.json on remote
	for i := 1; i <= 5; i++ {
		filename := fmt.Sprintf("key_%02d.json", i)
		remoteFile := path.Join(remoteDir, filename)
		content := fmt.Sprintf("{\"key\": \"%s\"}", filename)

		f, err := sftpClient.Create(remoteFile)
		if err != nil {
			log.Fatalf("Failed to create remote file %s: %v", filename, err)
		}
		f.Write([]byte(content))
		f.Close()
	}

	fmt.Println("Created key_01.json ... key_05.json on remote server.")

	// List available keys
	fmt.Println("Available keys on remote:")
	files, _ := sftpClient.ReadDir(remoteDir)
	for _, f := range files {
		if strings.HasPrefix(f.Name(), "key_") && strings.HasSuffix(f.Name(), ".json") {
			fmt.Println(" -", f.Name())
		}
	}

	// Download a specified key
	fmt.Print("Enter key filename to download (e.g., key_03.json): ")
	var chosen string
	fmt.Scanln(&chosen)

	remotePath := path.Join(remoteDir, chosen)
	localPath := path.Join(".", chosen)

	remoteFile, err := sftpClient.Open(remotePath)
	if err != nil {
		log.Fatalf("Failed to open remote file: %v", err)
	}
	defer remoteFile.Close()

	data, err := ioutil.ReadAll(remoteFile)
	if err != nil {
		log.Fatalf("Failed to read remote file: %v", err)
	}

	err = ioutil.WriteFile(localPath, data, 0644)
	if err != nil {
		log.Fatalf("Failed to write local file: %v", err)
	}

	fmt.Printf("Downloaded %s to local directory.\n", chosen)
}

func parseUserHost(s string) (string, string) {
	for i := 0; i < len(s); i++ {
		if s[i] == '@' {
			return s[:i], s[i+1:]
		}
	}
	log.Fatalf("Invalid SSH_HOST format: %s", s)
	return "", ""
}

/*
ssh <SSH_HOST>
sudo apt update
sudo apt install -y golang-go
go version

go mod tidy
go run main.go
*/
