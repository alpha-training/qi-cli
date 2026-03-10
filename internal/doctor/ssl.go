package doctor

import (
	"fmt"
	"os"
	"runtime"
)

// ResolveSSL takes the osName (e.g., "darwin", "linux", "windows")
// and returns a map of environment variables required for kdb+ SSL.
func ResolveSSL(osName string) map[string]string {
	fixes := make(map[string]string)
	arch := runtime.GOARCH

	switch osName {
	case "windows":
		// Windows: Typically just needs the verification bypass
		if os.Getenv("SSL_VERIFY_SERVER") == "" {
			fixes["SSL_VERIFY_SERVER"] = "NO"
		}

	case "darwin":
		// macOS: The critical issue is finding libssl.1.1.dylib
		// Apple Silicon (arm64) Homebrew uses /opt/homebrew
		// Intel (amd64) Homebrew uses /usr/local
		homebrewPath := "/opt/homebrew"
		if arch == "amd64" {
			homebrewPath = "/usr/local"
		}

		sslLibPath := fmt.Sprintf("%s/opt/openssl@1.1/lib", homebrewPath)
		sslCertFile := fmt.Sprintf("%s/etc/openssl@1.1/cert.pem", homebrewPath)

		// 1. Point kdb+ to the physical OpenSSL 1.1 .dylib files
		if _, err := os.Stat(sslLibPath); err == nil {
			existingDyld := os.Getenv("DYLD_LIBRARY_PATH")
			if existingDyld != "" {
				fixes["DYLD_LIBRARY_PATH"] = sslLibPath + ":" + existingDyld
			} else {
				fixes["DYLD_LIBRARY_PATH"] = sslLibPath
			}
		} else {
			// If the library is missing entirely, we can't fix it with env vars.
			fmt.Printf("⚠️  OpenSSL 1.1 not found at %s. \nRun: brew install openssl@1.1\n", sslLibPath)
		}

		// 2. Point to the CA bundle provided by Homebrew
		if _, err := os.Stat(sslCertFile); err == nil {
			fixes["SSL_CA_CERT_FILE"] = sslCertFile
		}

		// 3. Set the verification bypass if not already set
		if os.Getenv("SSL_VERIFY_SERVER") == "" {
			fixes["SSL_VERIFY_SERVER"] = "NO"
		}

	case "linux":
		// Linux: Standard CA certificate bundle paths
		certPaths := []string{
			"/etc/pki/tls/certs/ca-bundle.crt",   // RHEL/CentOS
			"/etc/ssl/certs/ca-certificates.crt", // Ubuntu/Debian
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
			fmt.Println("⚠️  No system SSL certificates found. Suggesting SSL_VERIFY_SERVER=NO")
			fixes["SSL_VERIFY_SERVER"] = "NO"
		}
	}

	return fixes
}
