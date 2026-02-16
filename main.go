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
	// 1. Only run Mac Doctor on Mac
	var sslPath string
	if runtime.GOOS == "darwin" {
		sslPath = ensureOpenSSL()
	}

	// 2. Create the bootstrap file
	bootstrapPath := "qi.bootstrap.q"
	err := os.WriteFile(bootstrapPath, qibootstrap, 0755) // 0755 makes it executable immediately
	if err != nil {
		fmt.Printf("❌ Failed to write bootstrap: %v\n", err)
		os.Exit(1)
	}

	// 3. Prepare q arguments
	qArgs := append([]string{bootstrapPath}, os.Args[1:]...)
	cmd := exec.Command("q", qArgs...)

	// 4. Set Environment
	env := os.Environ()
	if runtime.GOOS == "darwin" && sslPath != "" {
		env = append(env, "DYLD_LIBRARY_PATH="+sslPath)
		env = append(env, "DYLD_FALLBACK_LIBRARY_PATH="+sslPath)
	}
	// Note: Linux kdb+ usually finds its own libssl via ldconfig, 
	// but we could add LD_LIBRARY_PATH here if needed.
	
	env = append(env, "KX_SSL_VERIFY_SERVER=NO")
	cmd.Env = env

	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Stdin = os.Stdin

	// 5. Run and Cleanup
	_ = cmd.Run()
	os.Remove(bootstrapPath)
}

func ensureOpenSSL() string {
	// Double check we are on Mac before doing anything
	if runtime.GOOS != "darwin" {
		return ""
	}

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
			// Re-verify
			for _, p := range possiblePaths {
				if _, err := os.Stat(fmt.Sprintf("%s/libssl.1.1.dylib", p)); err == nil {
					return p
				}
			}
		}
	}
	return ""
}