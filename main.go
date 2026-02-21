package main

import (
	"bufio"
	_ "embed"
	"flag"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"

	"qi-cli/internal/api"
	"qi-cli/internal/doctor"
	"qi-cli/internal/kdb"
)

//go:embed qi.q
var embeddedQScript []byte

const qScriptName = "qi.q"

// AppConfig handles the hierarchy of settings
type AppConfig struct {
	ConnTimeout string
	PingTimeout string
	CertFile    string
	KeyFile     string
}

func main() {
	// 1. Load configuration from ~/.qi/qi.conf with OS-aware defaults
	appConf := loadQiConfig()

	// --- 2. API Mode Detection ---
	if len(os.Args) > 1 && os.Args[1] == "api" {
		apiCmd := flag.NewFlagSet("api", flag.ExitOnError)

		hubPort := apiCmd.String("hub", "8000", "Port of the running kdb process (hub)")
		listenPort := apiCmd.String("listen", "443", "Port for this REST API to listen on")

		// Flags default to values found in qi.conf
		certFile := apiCmd.String("cert", appConf.CertFile, "Path to SSL certificate")
		keyFile := apiCmd.String("key", appConf.KeyFile, "Path to SSL private key")

		apiCmd.Parse(os.Args[2:])

		// Pre-flight check for SSL files if running on standard HTTPS port
		if *listenPort == "443" {
			if _, err := os.Stat(*certFile); os.IsNotExist(err) {
				fmt.Printf("⚠️  SSL Certificate not found at %s\n", *certFile)
				fmt.Printf("💡 Check your ~/.qi/qi.conf or use -cert flag\n")
			}
		}

		fmt.Printf("🌐 Starting qi API Mode\n")
		fmt.Printf("📍 Hub: %s | Listen: %s | Timeout: %sms\n", *hubPort, *listenPort, appConf.ConnTimeout)

		api.Start(*hubPort, *listenPort, *certFile, *keyFile)
		return
	}

	// --- 3. Standard qi Launcher Logic ---
	fmt.Printf("--- qi CLI (OS: %s, Arch: %s) ---\n", runtime.GOOS, runtime.GOARCH)

	conf := kdb.ResolveConfig()
	conf.QHome = kdb.ExpandHome(conf.QHome)

	qPath, err := kdb.FindExecutable(conf.QHome)
	if err != nil {
		fmt.Printf("❌ %v\n", err)
		os.Exit(1)
	}

	bootstrapPath := resolveScriptPath()
	qArgs := append([]string{bootstrapPath}, os.Args[1:]...)
	cmd := buildCommand(qPath, qArgs, conf)

	cmd.Env = prepareEnv(conf)
	cmd.Dir = "."
	cmd.Stdout, cmd.Stderr, cmd.Stdin = os.Stdout, os.Stderr, os.Stdin

	if err := cmd.Run(); err != nil {
		fmt.Printf("\n❌ kdb+ exited: %v\n", err)
	}
}

// loadQiConfig handles OS-specific defaults and parses ~/.qi/qi.conf
func loadQiConfig() AppConfig {
	home, _ := os.UserHomeDir()
	confPath := filepath.Join(home, ".qi", "qi.conf")

	// Set baseline defaults
	config := AppConfig{
		ConnTimeout: "1000",
		PingTimeout: "100",
	}

	// Apply OS-specific SSL path defaults
	switch runtime.GOOS {
	case "windows":
		// Standard Certbot paths for Windows
		config.CertFile = "C:\\Certbot\\live\\api.qsharpe.com\\fullchain.pem"
		config.KeyFile = "C:\\Certbot\\live\\api.qsharpe.com\\privkey.pem"
	case "darwin":
		// Typical macOS local dev or Homebrew paths
		config.CertFile = filepath.Join(home, "Library/Application Support/qi/certs/fullchain.pem")
		config.KeyFile = filepath.Join(home, "Library/Application Support/qi/certs/privkey.pem")
	default:
		// RHEL / Ubuntu / Debian standard Let's Encrypt paths
		config.CertFile = "/etc/letsencrypt/live/api.qsharpe.com/fullchain.pem"
		config.KeyFile = "/etc/letsencrypt/live/api.qsharpe.com/privkey.pem"
	}

	// Parse the config file if it exists
	file, err := os.Open(confPath)
	if err == nil {
		defer file.Close()
		scanner := bufio.NewScanner(file)
		for scanner.Scan() {
			line := strings.TrimSpace(scanner.Text())
			if line == "" || strings.HasPrefix(line, "#") {
				continue
			}
			parts := strings.SplitN(line, "=", 2)
			if len(parts) == 2 {
				key := strings.TrimSpace(parts[0])
				val := strings.TrimSpace(parts[1])
				switch key {
				case "CONN_TIMEOUT":
					config.ConnTimeout = val
				case "PING_TIMEOUT":
					config.PingTimeout = val
				case "CERT_FILE":
					config.CertFile = val
				case "KEY_FILE":
					config.KeyFile = val
				}
			}
		}
	}

	return config
}

func resolveScriptPath() string {
	localPath := filepath.Join(".", qScriptName)
	if _, err := os.Stat(localPath); err == nil {
		return localPath
	}

	home, _ := os.UserHomeDir()
	qiDir := filepath.Join(home, kdb.ConfigDir)
	globalPath := filepath.Join(qiDir, qScriptName)

	_ = os.MkdirAll(qiDir, 0755)
	if err := os.WriteFile(globalPath, embeddedQScript, 0644); err != nil {
		fmt.Printf("❌ Failed to extract embedded script: %v\n", err)
		os.Exit(1)
	}
	return globalPath
}

func buildCommand(qPath string, qArgs []string, conf kdb.Config) *exec.Cmd {
	if conf.UseTask && (runtime.GOOS == "linux" || runtime.GOOS == "windows") {
		if runtime.GOOS == "linux" {
			return exec.Command("taskset", append([]string{"-c", conf.Cores, qPath}, qArgs...)...)
		}
		winArgs := append([]string{"/c", "start", "/b", "/affinity", conf.Cores, "/wait", qPath}, qArgs...)
		return exec.Command("cmd", winArgs...)
	}
	return exec.Command(qPath, qArgs...)
}

func prepareEnv(conf kdb.Config) []string {
	env := os.Environ()
	env = append(env, "QHOME="+conf.QHome)

	for k, v := range conf.ExtraEnv {
		env = append(env, fmt.Sprintf("%s=%s", k, v))
	}

	if runtime.GOOS == "darwin" {
		env = append(env, "DYLD_LIBRARY_PATH=/usr/local/opt/openssl@1.1/lib:/opt/homebrew/opt/openssl@1.1/lib")
	}

	sslFixes := doctor.ResolveSSL(runtime.GOOS)
	for k, v := range sslFixes {
		if _, overridden := conf.ExtraEnv[k]; !overridden {
			env = append(env, fmt.Sprintf("%s=%s", k, v))
		}
	}

	return env
}
