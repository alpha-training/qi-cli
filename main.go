package main

import (
	_ "embed"
	"bufio"
	"fmt"
	"os"
	"os/exec"
	"runtime"
	"strings"
)

//go:embed qi.q
var qibootstrap []byte

func main() {
	// 1. Run the Doctor with confirmation logic
	sslPath := ensureOpenSSL()

	// 2. Create the bootstrap file
	bootstrapPath := "qi.bootstrap.q"
	err := os.WriteFile(bootstrapPath, qibootstrap, 0644)
	if err != nil {
		fmt.Printf("❌ Failed to write bootstrap: %v\n", err)
		os.Exit(1)
	}

	// 3. Prepare q arguments
	qArgs := append([]string{bootstrapPath}, os.Args[1:]...)
	cmd := exec.Command("q", qArgs...)

	// 4. Set Environment
	env := os.Environ()
	if sslPath != "" {
		env = append(env, "DYLD_LIBRARY_PATH="+sslPath)
		env = append(env, "DYLD_FALLBACK_LIBRARY_PATH="+sslPath)
	}
	env = append(env, "KX_SSL_VERIFY_SERVER=NO")
	cmd.Env = env

	// 5. Connect standard I/O
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Stdin = os.Stdin

	// 6. Run q and Cleanup
	_ = cmd.Run()
	os.Remove(bootstrapPath)
}

func ensureOpenSSL() string {
	if runtime.GOOS != "darwin" {
		return ""
	}

	possiblePaths := []string{
		"/opt/homebrew/opt/openssl@1.1/lib",
		"/usr/local/opt/openssl@1.1/lib",
	}

	// Check if already installed
	for _, p := range possiblePaths {
		if _, err := os.Stat(fmt.Sprintf("%s/libssl.1.1.dylib", p)); err == nil {
			return p
		}
	}

	// Logic for asking permission
	fmt.Println("🩺 Doctor Check: OpenSSL 1.1 is required for kdb+ SSL connections but was not found.")
	fmt.Print("🤔 Would you like me to install it via Homebrew? (y/n): ")
	
	reader := bufio.NewReader(os.Stdin)
	response, _ := reader.ReadString('\n')
	response = strings.ToLower(strings.TrimSpace(response))

	if response != "y" && response != "yes" {
		fmt.Println("⚠️  Skipping installation. SSL connections will likely fail.")
		return ""
	}

	// Proceed with installation
	fmt.Println("🚀 Starting installation: brew install openssl@1.1")
	
	if _, err := exec.LookPath("brew"); err != nil {
		fmt.Println("❌ Error: Homebrew not found. Please install it first: https://brew.sh")
		return ""
	}

	installCmd := exec.Command("brew", "install", "openssl@1.1")
	installCmd.Stdout = os.Stdout
	installCmd.Stderr = os.Stderr
	
	if err := installCmd.Run(); err != nil {
		fmt.Printf("❌ Installation failed: %v\n", err)
		return ""
	}

	// Re-verify after install
	for _, p := range possiblePaths {
		if _, err := os.Stat(fmt.Sprintf("%s/libssl.1.1.dylib", p)); err == nil {
			fmt.Println("✅ OpenSSL 1.1 installed and verified.")
			return p
		}
	}

	return ""
}