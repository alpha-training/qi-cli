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
	// --- ADDED DEBUG LINE ---
	fmt.Printf("--- Qi CLI Starting (OS: %s, Arch: %s) ---\n", runtime.GOOS, runtime.GOARCH)

	// 1. Only run Mac Doctor on Mac
	var sslPath string
	if runtime.GOOS == "darwin" {
		sslPath = ensureOpenSSL()
	}

	// 2. Create the bootstrap file
	bootstrapPath := "qi.bootstrap.q"
	// 0755 ensures the file is created with executable permissions
	err := os.WriteFile(bootstrapPath, qibootstrap, 0755) 
	if err != nil {
		fmt.Printf("❌ Failed to write bootstrap: %v\n", err)
		os.Exit(1)
	}

	// 3. Prepare q arguments
	qArgs := append([]string{bootstrapPath}, os.Args[1:]...)
	
	// --- DEBUG LINE: Show exactly what we are calling ---
	fmt.Printf("DEBUG: Executing 'q %s'\n", strings.Join(qArgs, " "))

	cmd := exec.Command("q", qArgs...)

	// 4. Set Environment
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

	// 5. Run and Cleanup
	err = cmd.Run()
	if err != nil {
		fmt.Printf("❌ q process exited with error: %v\n", err)
	}

	os.Remove(bootstrapPath)
}

func ensureOpenSSL() string {
	// ... (Rest of the ensureOpenSSL function remains the same)
	return "" 
}