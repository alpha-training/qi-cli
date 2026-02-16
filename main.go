package main

import (
	"bufio"
	_ "embed"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
)

//go:embed qi.q
var qibootstrap []byte

const configFileName = ".qi.conf"

func main() {
	fmt.Printf("--- Qi CLI (OS: %s) ---\n", runtime.GOOS)

	// 1. Resolve q path (Environment -> Config -> Wizard)
	qPath := resolveQPath()

	// 2. Mac-specific OpenSSL Check
	var sslPath string
	if runtime.GOOS == "darwin" {
		sslPath = ensureOpenSSL()
	}

	// 3. Create bootstrap
	bootstrapPath := "qi.bootstrap.q"
	_ = os.WriteFile(bootstrapPath, qibootstrap, 0755)

	// 4. Build Command
	qArgs := append([]string{bootstrapPath}, os.Args[1:]...)
	cmd := exec.Command(qPath, qArgs...)

	// Setup Environment
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

	// Run
	_ = cmd.Run()
	os.Remove(bootstrapPath)
}

func resolveQPath() string {
	// A. Check current PATH
	if path, err := exec.LookPath("q"); err == nil {
		return path
	}

	// B. Check ~/.qi.conf
	home, _ := os.UserHomeDir()
	configPath := filepath.Join(home, configFileName)
	if savedPath := loadConfig(configPath); savedPath != "" {
		fullPath := getExecutablePath(savedPath)
		if _, err := os.Stat(fullPath); err == nil {
			os.Setenv("QHOME", savedPath)
			return fullPath
		}
	}

	// C. Launch Wizard
	return runWizard(configPath)
}

func runWizard(configPath string) string {
	fmt.Println("❌ kdb+ (q) not found in PATH or config.")
	fmt.Println("🧙 Where is your kdb+ folder? (Folder containing 'l64'/'m64' and kc.lic)")
	fmt.Print("📂 Path: ")

	reader := bufio.NewReader(os.Stdin)
	input, _ := reader.ReadString('\n')
	qhome := strings.TrimSpace(input)

	// Resolve tilde
	if strings.HasPrefix(qhome, "~") {
		home, _ := os.UserHomeDir()
		qhome = filepath.Join(home, qhome[1:])
	}

	fullPath := getExecutablePath(qhome)
	if _, err := os.Stat(fullPath); err != nil {
		fmt.Printf("❌ No kdb+ executable found at %s. Please check the path and try again.\n", fullPath)
		os.Exit(1)
	}

	// Save to config for next time
	saveConfig(configPath, qhome)
	os.Setenv("QHOME", qhome)
	
	fmt.Printf("✅ Path saved to %s\n", configPath)
	return fullPath
}

func getExecutablePath(qhome string) string {
	var sub string
	switch runtime.GOOS {
	case "linux": sub = "l64"
	case "darwin": sub = "m64"
	case "windows": sub = "w64"
	}
	bin := filepath.Join(qhome, sub, "q")
	if runtime.GOOS == "windows" { bin += ".exe" }
	return bin
}

func loadConfig(path string) string {
	data, err := os.ReadFile(path)
	if err != nil { return "" }
	return strings.TrimSpace(string(data))
}

func saveConfig(path, qhome string) {
	_ = os.WriteFile(path, []byte(qhome), 0644)
}

func ensureOpenSSL() string {
	// ... (Previous OpenSSL logic)
	return ""
}