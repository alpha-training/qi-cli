package doctor

import (
	"fmt"
	"os"
	"runtime"
)

// ResolveSSL identifies the OS and returns a map of environment variables
// that force kdb+ to find the correct OpenSSL libraries and skip verification.
func ResolveSSL(osName string) map[string]string {
	fixes := make(map[string]string)
	arch := runtime.GOARCH

	switch osName {
	case "windows":
		if os.Getenv("SSL_VERIFY_SERVER") == "" {
			fixes["SSL_VERIFY_SERVER"] = "NO"
		}

	case "darwin":
		// 1. Determine Homebrew base path based on Architecture
		brewBase := "/opt/homebrew"
		if arch == "amd64" {
			brewBase = "/usr/local"
		}

		// 2. Search for ANY available OpenSSL version (3 is the new default)
		// We check 3, 3.0, and 1.1 in order of modern preference.
		versions := []string{"openssl@3", "openssl@3.0", "openssl@1.1"}
		var foundLibPath string
		var foundCertPath string

		for _, v := range versions {
			libPath := fmt.Sprintf("%s/opt/%s/lib", brewBase, v)
			certPath := fmt.Sprintf("%s/etc/%s/cert.pem", brewBase, v)

			if _, err := os.Stat(libPath); err == nil {
				foundLibPath = libPath
				// If we found the lib, check if the cert exists in the same version
				if _, err := os.Stat(certPath); err == nil {
					foundCertPath = certPath
				}
				break
			}
		}

		// 3. Apply the fixes if libraries were found
		if foundLibPath != "" {
			// This tells kdb+ exactly where to look for .dylib files
			// without needing symlinks in /usr/local/lib
			existingDyld := os.Getenv("DYLD_LIBRARY_PATH")
			if existingDyld != "" {
				fixes["DYLD_LIBRARY_PATH"] = foundLibPath + ":" + existingDyld
			} else {
				fixes["DYLD_LIBRARY_PATH"] = foundLibPath
			}

			if foundCertPath != "" {
				fixes["SSL_CA_CERT_FILE"] = foundCertPath
			}
		} else {
			// If we literally can't find OpenSSL anywhere, we warn the user.
			fmt.Println("❌ Error: OpenSSL not detected. Please run 'brew install openssl'")
		}

		// 4. Force bypass to solve the "Protocol not available" error
		fixes["SSL_VERIFY_SERVER"] = "NO"

	case "linux":
		certPaths := []string{
			"/etc/pki/tls/certs/ca-bundle.crt",
			"/etc/ssl/certs/ca-certificates.crt",
		}

		found := false
		for _, p := range certPaths {
			if _, err := os.Stat(p); err == nil {
				fixes["SSL_CA_CERT_FILE"] = p
				found = true
				break
			}
		}

		if !found && os.Getenv("SSL_VERIFY_SERVER") == "" {
			fixes["SSL_VERIFY_SERVER"] = "NO"
		}
	}

	return fixes
}
