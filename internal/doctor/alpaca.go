package doctor

import (
	"bufio"
	"fmt"
	"os"
	"os/exec"
	"runtime"
	"strings"
)

func CheckAlpaca() error {
	if runtime.GOOS == "darwin" {
		return checkMacSSL()
	} else if runtime.GOOS == "windows" {
		return checkWindowsSSL()
	}
	return nil
}

func checkMacSSL() error {
	// Check both Homebrew locations (Intel and ARM)
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

func checkWindowsSSL() error {
	path := `C:\Program Files\OpenSSL-Win64\bin`
	if _, err := os.Stat(path); os.IsNotExist(err) {
		fmt.Println("⚠️  OpenSSL 1.1 not found.")
		fmt.Println("👉 Download from: https://slproweb.com/products/Win32OpenSSL.html")
	}
	return nil
}