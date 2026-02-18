package doctor

import (
	"bufio"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
)

// CheckAlpaca is the main entry point called by main.go
func CheckAlpaca() error {
	// 1. Check for OpenSSL dependencies (Mac/Windows)
	if runtime.GOOS == "darwin" {
		if err := checkMacSSL(); err != nil {
			return err
		}
	} else if runtime.GOOS == "windows" {
		if err := checkWindowsSSL(); err != nil {
			return err
		}
	}

	// 2. Check for API Keys
	return HandleAlpacaSetup()
}

// HandleAlpacaSetup checks for keys and prompts user if missing
func HandleAlpacaSetup() error {
	home, err := os.UserHomeDir()
	if err != nil {
		return fmt.Errorf("could not find home directory: %v", err)
	}

	configPath := filepath.Join(home, ".qi", "qi.conf")

	// 1. Read the existing config file
	configData, _ := os.ReadFile(configPath)
	configStr := string(configData)

	// Check what we already have
	hasKey := strings.Contains(configStr, "ALPACAKEY=")
	hasSecret := strings.Contains(configStr, "ALPACASECRET=")

	// If both exist, we are done
	if hasKey && hasSecret {
		// Only print if you want verbose output, otherwise keep silent for success
		// fmt.Println("✨ ALPACAKEY and ALPACASECRET already configured.")
		return nil
	}

	// 2. Prompt for missing items
	reader := bufio.NewReader(os.Stdin)
	var keyToWrite, secretToWrite string

	fmt.Println("⚠️  Alpaca API keys missing.")

	// Prompt for Key if missing
	if !hasKey {
		fmt.Print("🔑 Please enter your ALPACAKEY: ")
		input, _ := reader.ReadString('\n')
		key := strings.TrimSpace(input)
		if key == "" {
			return fmt.Errorf("API key cannot be empty")
		}
		keyToWrite = "ALPACAKEY=" + key + "\n"
	}

	// Prompt for Secret if missing
	if !hasSecret {
		fmt.Print("🔐 Please enter your ALPACASECRET: ")
		input, _ := reader.ReadString('\n')
		secret := strings.TrimSpace(input)
		if secret == "" {
			return fmt.Errorf("secret key cannot be empty")
		}
		secretToWrite = "ALPACASECRET=" + secret + "\n"
	}

	// 3. Open file to save
	if err := os.MkdirAll(filepath.Dir(configPath), 0755); err != nil {
		return fmt.Errorf("could not create config directory: %v", err)
	}

	f, err := os.OpenFile(configPath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return fmt.Errorf("could not open config file: %v", err)
	}
	defer f.Close()

	// Ensure we start on a new line
	if len(configStr) > 0 && !strings.HasSuffix(configStr, "\n") {
		f.WriteString("\n")
	}

	// 4. Write the missing pieces
	if keyToWrite != "" {
		f.WriteString(keyToWrite)
	}
	if secretToWrite != "" {
		f.WriteString(secretToWrite)
	}

	f.Sync()
	fmt.Printf("✅ Alpaca configuration updated in %s\n", configPath)
	return nil
}

// checkMacSSL ensures OpenSSL 1.1 is available on macOS
func checkMacSSL() error {
	paths := []string{
		"/usr/local/opt/openssl@1.1/lib/libssl.1.1.dylib",
		"/opt/homebrew/opt/openssl@1.1/lib/libssl.1.1.dylib",
	}

	found := false
	for _, p := range paths {
		if _, err := os.Stat(p); err == nil {
			found = true
			break
		}
	}

	if !found {
		fmt.Println("⚠️  OpenSSL 1.1 not found (required for kdb+ on macOS).")
		fmt.Print("🤔 Would you like to install it via Homebrew? (y/n): ")

		reader := bufio.NewReader(os.Stdin)
		input, _ := reader.ReadString('\n')
		if strings.ToLower(strings.TrimSpace(input)) == "y" {
			cmd := exec.Command("brew", "install", "openssl@1.1")
			cmd.Stdout, cmd.Stderr = os.Stdout, os.Stderr
			return cmd.Run()
		}
		return fmt.Errorf("OpenSSL 1.1 is missing")
	}
	return nil
}

// checkWindowsSSL ensures OpenSSL is available on Windows
func checkWindowsSSL() error {
	path := `C:\Program Files\OpenSSL-Win64\bin`
	if _, err := os.Stat(path); os.IsNotExist(err) {
		fmt.Println("⚠️  OpenSSL 1.1 not found.")
		fmt.Println("👉 Download from: https://slproweb.com/products/Win32OpenSSL.html")
	}
	return nil
}