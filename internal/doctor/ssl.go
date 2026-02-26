package doctor

import (
	"fmt"
	"os"
)

func ResolveSSL(osName string) map[string]string {
    fixes := make(map[string]string)

    if osName == "windows" {
        // Set the SSL bypass specifically for Windows environment
        if os.Getenv("SSL_VERIFY_SERVER") == "" {
            fixes["SSL_VERIFY_SERVER"] = "NO"
        }
        // Return early for Windows so we don't hit the Linux path checks
        return fixes 
    } else if osName != "linux" { 
        return fixes 
    }

    // --- Everything below only runs if osName == "linux" ---

    // Common Linux CA paths
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

    // If no certs found on Linux, we suggest the insecure fallback
    if !found && os.Getenv("SSL_VERIFY_SERVER") == "" {
        fmt.Println("⚠️  No system SSL certificates found. Suggesting SSL_VERIFY_SERVER=NO")
        fixes["SSL_VERIFY_SERVER"] = "NO"
    }

    return fixes
}