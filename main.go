package main

import (
	"bufio"
	_ "embed"
	"fmt"
	"os"
	"os/exec"
	"runtime"
	"strings"
	"path/filepath"
)

//go:embed qi.q
var qibootstrap []byte

func main() {
	fmt.Printf("--- Qi CLI (OS: %s) ---\n", runtime.GOOS)

	// 1. Ensure kdb+ is accessible
	qPath := ensureQReady()

	// 2. Mac-specific OpenSSL Check
	var sslPath string
	if runtime.GOOS == "darwin" {
		sslPath = ensureOpenSSL()
	}

	// 3. Create bootstrap
	bootstrapPath := "qi.bootstrap.q"
	_ = os.WriteFile(bootstrapPath, qibootstrap, 0755)

	// 4. Run q
	qArgs := append([]string{bootstrapPath}, os.Args[1:]...)
	cmd := exec.Command(qPath, qArgs...)

	env := os.Environ()
	if sslPath != "" {
		env = append(env, "DYLD_LIBRARY_PATH="+sslPath)
		env = append(env, "DYLD_FALLBACK_LIBRARY_PATH="+sslPath)
	}
	env = append(env, "KX_SSL_VERIFY_SERVER=NO")
	cmd.Env = env

	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Stdin = os.Stdin

	_ = cmd.Run()
	os.Remove(bootstrapPath)
}

func ensureQReady() string {
	// Try finding q in current PATH
	path, err := exec.LookPath("q")
	if err == nil {
		return path
	}

	fmt.Println("❌ kdb+ (q) was not found in your PATH.")
	fmt.Println("🧙 Let's set it up. Where is your kdb+ folder located?")
	fmt.Println("   (This is the folder containing 'l64', 'm64', or 'w64' and your kc.lic)")
	fmt.Print("📂 Path: ")

	reader := bufio.NewReader(os.Stdin)
	input, _ := reader.ReadString('\n')
	qhome := strings.TrimSpace(input)

	// Resolve tilde if user used ~/
	if strings.HasPrefix(qhome, "~") {
		home, _ := os.UserHomeDir()
		qhome = filepath.Join(home, qhome[1:])
	}

	// Architecture subfolder
	var sub string
	switch runtime.GOOS {
	case "linux": sub = "l64"
	case "darwin": sub = "m64"
	case "windows": sub = "w64"
	}

	fullPath := filepath.Join(qhome, sub, "q")
	if runtime.GOOS == "windows" {
		fullPath += ".exe"
	}

	// Verify License and Executable
	if _, err := os.Stat(fullPath); err != nil {
		fmt.Printf("❌ Could not find executable at %s\n", fullPath)
		os.Exit(1)
	}

	licPath := filepath.Join(qhome, "kc.lic")
	if _, err := os.Stat(licPath); err != nil {
		fmt.Printf("⚠️  Warning: kc.lic not found in %s. q might not start.\n", qhome)
	}

	// Set for current process
	os.Setenv("QHOME", qhome)
	fmt.Printf("✅ Success! Using QHOME=%s\n", qhome)
	
	return fullPath
}

func ensureOpenSSL() string {
	// (Your previous OpenSSL logic here)
	return ""
}