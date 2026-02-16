package doctor

import (
	"fmt"
	"os"
	"os/exec"
	"runtime"
)

func init() {
	Register("alpaca", checkAlpaca)
}

func checkAlpaca() error {
	fmt.Println("🩺 Checking Alpaca dependencies...")

	// 1. OS-Specific OpenSSL Check
	if runtime.GOOS == "darwin" {
		return checkMacSSL()
	} else if runtime.GOOS == "windows" {
		return checkWindowsSSL()
	}
	
	return nil
}

func checkMacSSL() error {
	path := "/opt/homebrew/opt/openssl@1.1/lib/libssl.1.1.dylib"
	_, err := os.Stat(path)
	
	if os.IsNotExist(err) {
		fmt.Println("⚠️  OpenSSL 1.1 not found (required for kdb+ on macOS).")
		
		if AskConfirm("Would you like to install it via Homebrew?") {
			fmt.Println("Installing openssl@1.1...")
			cmd := exec.Command("brew", "install", "openssl@1.1")
			cmd.Stdout = os.Stdout
			cmd.Stderr = os.Stderr
			return cmd.Run()
		}
		return fmt.Errorf("OpenSSL 1.1 is required to continue")
	}
	
	fmt.Println("✅ OpenSSL 1.1 found at " + path)
	return nil
}

func checkWindowsSSL() error {
	// Standard Windows path for OpenSSL 1.1 Light
	defaultPath := `C:\Program Files\OpenSSL-Win64\bin`
	fmt.Printf("🔍 Checking for OpenSSL in %s...\n", defaultPath)
	
	if _, err := os.Stat(defaultPath); os.IsNotExist(err) {
		fmt.Println("⚠️  OpenSSL 1.1 not found in default location.")
		if AskConfirm("Would you like to open the OpenSSL download page?") {
			// Open browser to ShiningLight or similar
			return exec.Command("rundll32", "url.dll,FileProtocolHandler", "https://slproweb.com/products/Win32OpenSSL.html").Run()
		}
	}
	return nil
}