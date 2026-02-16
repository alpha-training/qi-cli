package main

import (
	"bufio"
	_ "embed"
	"fmt"
	"os"
	"os/exec"
	"runtime"
	"strings"
)

//go:embed qi.q
var qibootstrap []byte

func main() {
	// Heartbeat to prove the binary is executing
	fmt.Printf("--- Qi CLI Starting (OS: %s, Arch: %s) ---\n", runtime.GOOS, runtime.GOARCH)

	var sslPath string
	if runtime.GOOS == "darwin" {
		sslPath = ensureOpenSSL()
	}

	// Create the bootstrap file with executable permissions (0755)
	bootstrapPath := "qi.bootstrap.q"
	err := os.WriteFile(bootstrapPath, qibootstrap, 0755)
	if err != nil {
		fmt.Printf("❌ Failed to write bootstrap file: %v\n", err)
		os.Exit(1)
	}

	// Prepare arguments
	qArgs := append([]string{bootstrapPath}, os.Args[1:]...)
	
	// Show the user what is happening
	fmt.Printf("DEBUG: Launching 'q %s'\n", strings.Join(qArgs, " "))

	cmd := exec.Command("q", qArgs...)

	// Setup Environment
	env := os.Environ()
	if runtime.GOOS == "darwin" && sslPath != "" {
		env = append(env, "DYLD_LIBRARY_PATH="+sslPath)
		env = append(env, "DYLD_FALLBACK_LIBRARY_PATH="+sslPath)
	}
	env = append(env, "KX_SSL_VERIFY_SERVER=NO")
	cmd.Env = env

	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Stdin = os.Stdin

	// Run q
	err = cmd.Run()
	if err != nil {
		fmt.Printf("❌ q process finished with error: %v\n", err)
	}

	// Cleanup
	os.Remove(bootstrapPath)
}

func ensureOpenSSL() string {
	// This makes bufio "used" for the compiler even on Linux builds
	var _ = bufio.NewReader(os.Stdin)

	possiblePaths := []string{
		"/opt/homebrew/opt/openssl@1.1/lib",
		"/usr/local/opt/openssl@1.1/lib",
	}

	for _, p := range possiblePaths {
		if _, err := os.Stat(fmt.Sprintf("%s/libssl.1.1.dylib", p)); err == nil {
			return p
		}
	}

	fmt.Println("🩺 OpenSSL 1.1 missing.")
	fmt.Print("🤔 Install via Homebrew? (y/n): ")
	
	reader := bufio.NewReader(os.Stdin)
	response, _ := reader.ReadString('\n')
	response = strings.ToLower(strings.TrimSpace(response))

	if response == "y" || response == "yes" {
		installCmd := exec.Command("brew", "install", "openssl@1.1")
		installCmd.Stdout = os.Stdout
		installCmd.Stderr = os.Stderr
		if err := installCmd.Run(); err == nil {
			for _, p := range possiblePaths {
				if _, err := os.Stat(fmt.Sprintf("%s/libssl.1.1.dylib", p)); err == nil {
					return p
				}
			}
		}
	}
	return ""
}