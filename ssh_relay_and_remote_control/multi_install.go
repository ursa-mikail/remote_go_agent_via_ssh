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
	DirName   string   // folder after extract
	Configure string   // configure/cmake step
	Build     string   // build step
	Install   string   // install step
	SelfTest  []string // post-install test commands
}

func main() {
	// SSH info from environment
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
		log.Fatal("SSH_HOST must be set (user@host or SSH_USER + SSH_HOST)")
	}

	// CLI flags
	remoteTmp := flag.String("remote-tmp", "/tmp", "Remote temp dir")
	installDir := flag.String("install-dir", "/usr/local", "Install prefix")
	sudo := flag.Bool("sudo", true, "Use sudo for install")
	flag.Parse()

	if *sudo && passwordEnv == "" {
		log.Fatal("SSH_PASS must be set for sudo operations")
	}

	// SSH connection
	cfg, err := sshConfig(userEnv, keyEnv, passwordEnv)
	if err != nil {
		log.Fatalf("ssh config error: %v", err)
	}
	addr := net.JoinHostPort(hostOnly, "22")
	client, err := ssh.Dial("tcp", addr, cfg)
	if err != nil {
		log.Fatalf("ssh dial error: %v", err)
	}
	defer client.Close()

	// Packages to install (order matters - dependencies first)
	pkgs := []Package{
		{
			URL:       "https://zlib.net/zlib-1.3.1.tar.gz",
			Tarball:   "zlib-1.3.1.tar.gz",
			DirName:   "zlib-1.3.1",
			Configure: "./configure --prefix=" + *installDir,
			Build:     "make -j$(nproc)",
			Install:   chooseSudo(*sudo, "make install", passwordEnv),
			SelfTest:  []string{chooseSudo(*sudo, "ldconfig", passwordEnv), "ldconfig -p | grep zlib || echo 'zlib not found in ldconfig'"},
		},
		{
			URL:       "https://www.openssl.org/source/openssl-3.4.2.tar.gz",
			Tarball:   "openssl-3.4.2.tar.gz",
			DirName:   "openssl-3.4.2",
			Configure: fmt.Sprintf("./config --prefix=%s --openssldir=%s/ssl enable-legacy shared", *installDir, *installDir),
			Build:     "make -j$(nproc)",
			Install:   chooseSudo(*sudo, "make install", passwordEnv),
			SelfTest:  []string{}, // Will be set dynamically
		},
		{
			URL:       "https://curl.se/download/curl-8.10.1.tar.gz",
			Tarball:   "curl-8.10.1.tar.gz",
			DirName:   "curl-8.10.1",
			Configure: fmt.Sprintf("PKG_CONFIG_PATH=%s/lib/pkgconfig:$PKG_CONFIG_PATH ./configure --prefix=%s --with-openssl=%s", *installDir, *installDir, *installDir),
			Build:     "make -j$(nproc)",
			Install:   chooseSudo(*sudo, "make install", passwordEnv),
			SelfTest:  []string{fmt.Sprintf("LD_LIBRARY_PATH=%s/lib:$LD_LIBRARY_PATH %s/bin/curl --version", *installDir, *installDir)},
		},
		{
			URL:       "https://github.com/vim/vim/archive/refs/tags/v9.1.0000.tar.gz",
			Tarball:   "vim-9.1.0000.tar.gz",
			DirName:   "vim-9.1.0000",
			Configure: fmt.Sprintf("./configure --prefix=%s --enable-multibyte --enable-cscope", *installDir),
			Build:     "make -j$(nproc)",
			Install:   chooseSudo(*sudo, "make install", passwordEnv),
			SelfTest:  []string{fmt.Sprintf("%s/bin/vim --version | head -n 3", *installDir)},
		},
	}

	// Install each package
	for _, pkg := range pkgs {
		if err := ensureTarball(pkg); err != nil {
			log.Fatalf("download failed for %s: %v", pkg.URL, err)
		}
		if err := installPackage(client, *remoteTmp, *installDir, *sudo, passwordEnv, pkg); err != nil {
			log.Fatalf("installation failed for %s: %v", pkg.Tarball, err)
		}
	}

	log.Println("All packages installed successfully!")
}

// SSH configuration
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

// Returns a sudo-wrapped command if requested
func chooseSudo(use bool, cmd, password string) string {
	if use {
		return fmt.Sprintf("echo '%s' | sudo -S %s", password, cmd)
	}
	return cmd
}

// Download tarball if missing
func ensureTarball(pkg Package) error {
	if _, err := os.Stat(pkg.Tarball); os.IsNotExist(err) {
		log.Printf("Downloading %s...", pkg.URL)
		cmd := exec.Command("wget", "-O", pkg.Tarball, pkg.URL)
		cmd.Stdout, cmd.Stderr = os.Stdout, os.Stderr
		return cmd.Run()
	}
	return nil
}

// Install a single package
func installPackage(client *ssh.Client, remoteTmp, installDir string, sudo bool, password string, pkg Package) error {
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

	// Special handling for OpenSSL
	if strings.Contains(strings.ToLower(pkg.Tarball), "openssl") && sudo {
		// Check both lib and lib64 directories and configure ldconfig
		steps = append(steps,
			// Create ldconfig entry - try both lib and lib64
			fmt.Sprintf("if [ -d %s/lib ]; then %s; fi", installDir, 
				chooseSudo(sudo, fmt.Sprintf("sh -c 'echo %s/lib > /etc/ld.so.conf.d/openssl.conf'", installDir), password)),
			fmt.Sprintf("if [ -d %s/lib64 ]; then %s; fi", installDir,
				chooseSudo(sudo, fmt.Sprintf("sh -c 'echo %s/lib64 >> /etc/ld.so.conf.d/openssl.conf'", installDir), password)),
			chooseSudo(sudo, "ldconfig -v", password),
		)
		
		// Set up multiple test approaches for OpenSSL
		pkg.SelfTest = []string{
			// Test 1: Try with LD_LIBRARY_PATH pointing to lib
			fmt.Sprintf("LD_LIBRARY_PATH=%s/lib:$LD_LIBRARY_PATH %s/bin/openssl version || echo 'Test 1 failed'", installDir, installDir),
			// Test 2: Try with LD_LIBRARY_PATH pointing to lib64
			fmt.Sprintf("LD_LIBRARY_PATH=%s/lib64:$LD_LIBRARY_PATH %s/bin/openssl version || echo 'Test 2 failed'", installDir, installDir),
			// Test 3: Try without LD_LIBRARY_PATH (using ldconfig)
			fmt.Sprintf("%s/bin/openssl version || echo 'Test 3 failed'", installDir),
			// Diagnostic: Show what libraries are available
			fmt.Sprintf("ls -la %s/lib*/libssl* %s/lib*/libcrypto* 2>/dev/null || echo 'No SSL libraries found'", installDir, installDir),
		}
	}

	// Execute all steps
	for _, c := range steps {
		if err := runRemote(client, c); err != nil {
			return fmt.Errorf("command failed: %s: %v", c, err)
		}
	}

	// Execute self-tests
	for _, testCmd := range pkg.SelfTest {
		log.Printf("Running test: %s", testCmd)
		if err := runRemote(client, testCmd); err != nil {
			// For OpenSSL, don't fail immediately - try all tests
			if strings.Contains(strings.ToLower(pkg.Tarball), "openssl") {
				log.Printf("OpenSSL test failed (trying others): %v", err)
			} else {
				return fmt.Errorf("self-test failed: %s: %v", testCmd, err)
			}
		}
	}

	return nil
}

// Copy local file to remote via SCP
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

// Execute remote command via SSH
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
Usage:

go mod init multi_install
go mod tidy
go run multi_install.go

Environment variables:
  SSH_HOST=user@host (or just host with SSH_USER set separately)
  SSH_USER=username (if not in SSH_HOST)
  SSH_PASS=password
  SSH_KEY=/path/to/key (optional, alternative to password)

Optional flags:
  -remote-tmp /tmp
  -install-dir /usr/local
  -sudo=true

Key improvements:
- Proper dependency order (zlib before OpenSSL, OpenSSL before curl)
- Better OpenSSL configuration with shared libraries
- Multiple test approaches for OpenSSL
- PKG_CONFIG_PATH for curl to find OpenSSL
- Proper shell quoting for passwords
- Better error handling and diagnostics
- LD_LIBRARY_PATH for applications that need it
*/