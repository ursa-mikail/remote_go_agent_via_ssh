package main

import (
	"flag"
	"fmt"
	"io"
	"log"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"golang.org/x/crypto/ssh"
)

// Package defines how to build/install a tarball
type Package struct {
	URL       string   // source URL
	Tarball   string   // local tarball filename
	DirName   string   // expected folder after extract
	Configure string   // configure or cmake step
	Build     string   // build step
	Install   string   // install step
	SelfTest  []string // test commands after install
}

func main() {
	// Read SSH info from environment
	hostEnv := os.Getenv("SSH_HOST")
	userEnv := os.Getenv("SSH_USER")
	passwordEnv := os.Getenv("SSH_PASS")
	keyEnv := os.Getenv("SSH_KEY")

	hostOnly := hostEnv
	if hostEnv != "" && strings.Contains(hostEnv, "@") {
		parts := strings.SplitN(hostEnv, "@", 2)
		userEnv = parts[0]
		hostOnly = parts[1]
	}

	if hostOnly == "" || userEnv == "" {
		log.Fatal("SSH_HOST must be set (user@host or host + SSH_USER env)")
	}

	// CLI flags
	remoteTmp := flag.String("remote-tmp", "/tmp", "Remote temp dir")
	installDir := flag.String("install-dir", "/usr/local", "Install prefix")
	sudo := flag.Bool("sudo", true, "Use sudo for install")
	flag.Parse()

	if *sudo && passwordEnv == "" {
		log.Fatal("SSH_PASS must be set in env for sudo")
	}

	// SSH config
	cfg, err := sshConfig(userEnv, keyEnv, passwordEnv)
	if err != nil {
		log.Fatalf("ssh config error: %v", err)
	}
	addr := net.JoinHostPort(hostOnly, "22")
	client, err := ssh.Dial("tcp", addr, cfg)
	if err != nil {
		log.Fatalf("ssh dial: %v", err)
	}
	defer client.Close()

	// Define packages
	pkgs := []Package{
		{
			URL:       "https://www.openssl.org/source/openssl-3.4.2.tar.gz",
			Tarball:   "openssl-3.4.2.tar.gz",
			DirName:   "openssl-3.4.2",
			Configure: fmt.Sprintf("./config --prefix=%s --openssldir=%s/ssl enable-legacy shared", *installDir, *installDir),
			Build:     "make -j$(nproc)",
			Install:   chooseSudo(*sudo, "make install", passwordEnv),
			SelfTest:  []string{}, // Will be populated dynamically
		},
	}

	// Process packages
	for _, pkg := range pkgs {
		if err := ensureTarball(pkg); err != nil {
			log.Fatalf("download failed for %s: %v", pkg.URL, err)
		}
		if err := installPackage(client, *remoteTmp, pkg, *sudo, passwordEnv, *installDir); err != nil {
			log.Fatalf("failed installing %s: %v", pkg.Tarball, err)
		}
	}
}

// sshConfig prepares SSH client config
func sshConfig(user, keyPath, password string) (*ssh.ClientConfig, error) {
	var auths []ssh.AuthMethod
	if keyPath != "" {
		key, err := os.ReadFile(keyPath)
		if err != nil {
			return nil, err
		}
		signer, err := ssh.ParsePrivateKey(key)
		if err != nil {
			return nil, err
		}
		auths = append(auths, ssh.PublicKeys(signer))
	}
	if password != "" {
		auths = append(auths, ssh.Password(password))
	}
	return &ssh.ClientConfig{
		User:            user,
		Auth:            auths,
		HostKeyCallback: ssh.InsecureIgnoreHostKey(),
		Timeout:         30 * time.Second,
	}, nil
}

// chooseSudo wraps a command in sudo with password
func chooseSudo(use bool, cmd, password string) string {
	if use {
		return fmt.Sprintf("echo %s | sudo -S %s", password, cmd)
	}
	return cmd
}

// ensureTarball downloads tarball if missing
func ensureTarball(pkg Package) error {
	if _, err := os.Stat(pkg.Tarball); os.IsNotExist(err) {
		log.Printf("Downloading %s...", pkg.URL)
		cmd := exec.Command("wget", "-O", pkg.Tarball, pkg.URL)
		cmd.Stdout, cmd.Stderr = os.Stdout, os.Stderr
		return cmd.Run()
	}
	return nil
}

// installPackage uploads, builds, installs, and tests the package
func installPackage(client *ssh.Client, remoteTmp string, pkg Package, sudo bool, password, installDir string) error {
	remoteTar := filepath.Join(remoteTmp, filepath.Base(pkg.Tarball))
	log.Printf("Uploading %s...", pkg.Tarball)
	if err := scpFile(client, pkg.Tarball, remoteTar); err != nil {
		return err
	}

	steps := []string{
		fmt.Sprintf("cd %s && tar -xzf %s", remoteTmp, filepath.Base(pkg.Tarball)),
		fmt.Sprintf("cd %s/%s && %s", remoteTmp, pkg.DirName, pkg.Configure),
		fmt.Sprintf("cd %s/%s && %s", remoteTmp, pkg.DirName, pkg.Build),
		fmt.Sprintf("cd %s/%s && %s", remoteTmp, pkg.DirName, pkg.Install),
	}

	// Library configuration steps
	if sudo {
		// Try multiple library paths - OpenSSL might install to lib64 on some systems
		libPaths := []string{
			fmt.Sprintf("%s/lib", installDir),
			fmt.Sprintf("%s/lib64", installDir),
		}
		
		// Create ld.so.conf entries for both possible paths
		for _, libPath := range libPaths {
			steps = append(steps,
				// Check if the lib directory exists and add it to ld.so.conf
				fmt.Sprintf("[ -d %s ] && %s || echo 'Directory %s does not exist'", 
					libPath, 
					chooseSudo(sudo, fmt.Sprintf("sh -c 'echo %s > /etc/ld.so.conf.d/openssl.conf'", libPath), password),
					libPath,
				),
			)
		}
		
		// Update ldconfig
		steps = append(steps, chooseSudo(sudo, "ldconfig -v", password))
	}

	// Execute build and install steps
	for _, c := range steps {
		if err := runRemote(client, c); err != nil {
			return fmt.Errorf("command failed: %s: %v", c, err)
		}
	}

	// Now try multiple approaches to test OpenSSL
	testApproaches := []string{
		// Approach 1: Use LD_LIBRARY_PATH with lib
		fmt.Sprintf("LD_LIBRARY_PATH=%s/lib:$LD_LIBRARY_PATH %s/bin/openssl version", installDir, installDir),
		// Approach 2: Use LD_LIBRARY_PATH with lib64
		fmt.Sprintf("LD_LIBRARY_PATH=%s/lib64:$LD_LIBRARY_PATH %s/bin/openssl version", installDir, installDir),
		// Approach 3: Try without LD_LIBRARY_PATH (in case ldconfig worked)
		fmt.Sprintf("%s/bin/openssl version", installDir),
		// Approach 4: Check what libraries the binary is trying to load
		fmt.Sprintf("ldd %s/bin/openssl", installDir),
	}

	log.Printf("Testing OpenSSL installation...")
	var lastErr error
	for i, testCmd := range testApproaches {
		log.Printf("Test approach %d: %s", i+1, testCmd)
		if err := runRemote(client, testCmd); err != nil {
			lastErr = err
			log.Printf("Test approach %d failed: %v", i+1, err)
			continue
		}
		log.Printf("✅ OpenSSL test successful with approach %d", i+1)
		return nil
	}

	// If all tests failed, run diagnostic commands
	log.Printf("All tests failed, running diagnostics...")
	diagnostics := []string{
		"ls -la " + installDir + "/lib*",
		"ls -la " + installDir + "/bin/openssl",
		"cat /etc/ld.so.conf.d/openssl.conf || echo 'openssl.conf not found'",
		"ldconfig -p | grep ssl || echo 'No ssl libs found in ldconfig'",
	}

	for _, diagnostic := range diagnostics {
		log.Printf("Diagnostic: %s", diagnostic)
		runRemote(client, diagnostic) // Ignore errors for diagnostics
	}

	return fmt.Errorf("all OpenSSL tests failed, last error: %v", lastErr)
}

// scpFile uploads a local file to remote via SCP
func scpFile(client *ssh.Client, localPath, remotePath string) error {
	session, err := client.NewSession()
	if err != nil {
		return err
	}
	defer session.Close()

	srcFile, err := os.Open(localPath)
	if err != nil {
		return err
	}
	defer srcFile.Close()

	info, _ := srcFile.Stat()
	go func() {
		w, _ := session.StdinPipe()
		defer w.Close()
		fmt.Fprintf(w, "C0644 %d %s\n", info.Size(), filepath.Base(localPath))
		io.Copy(w, srcFile)
		fmt.Fprint(w, "\x00")
	}()
	return session.Run(fmt.Sprintf("scp -tr %s", filepath.Dir(remotePath)))
}

// runRemote executes a command over SSH
func runRemote(client *ssh.Client, cmd string) error {
	session, err := client.NewSession()
	if err != nil {
		return err
	}
	defer session.Close()

	session.Stdout = os.Stdout
	session.Stderr = os.Stderr
	log.Printf("Running: %s", cmd)
	return session.Run(cmd)
}


/*
go mod init single_install
go mod tidy
go run single_install.go

🔑 Key Points:
- Pre-download OpenSSL tarball on Mac (wget won’t work inside VM without net)
- Uploads the tarball via SCP
- Extracts it in /tmp
- Runs ./config, make, sudo make install
- Runs openssl version with LD_LIBRARY_PATH to pick up the new libraries
- Works with either SSH password or SSH key
*/
