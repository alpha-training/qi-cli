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

const (
	configDir      = ".qi"
	configFileName = "config"
)

type QiConfig struct {
	QHome   string
	UseTask bool
	Cores   string // e.g., "0,1" for Linux, "3" (hex/mask) for Windows
}

func main() {
	// Heartbeat
	fmt.Printf("--- Qi CLI (OS: %s, Arch: %s) ---\n", runtime.GOOS, runtime.GOARCH)

	// 1. Resolve Configuration and QPath
	conf := resolveConfig()
	qPath := getExecutablePath(conf.QHome)

	// 2. Check for OpenSSL (Mac only)
	var sslPath string
	if runtime.GOOS == "darwin" {
		sslPath = ensureOpenSSL()
	}

	// 3. Create the bootstrap file
	bootstrapPath := filepath.Join(os.TempDir(), "qi.bootstrap.q")
	err := os.WriteFile(bootstrapPath, qibootstrap, 0755)
	if err != nil {
		fmt.Printf("❌ Failed to write bootstrap: %v\n", err)
		os.Exit(1)
	}
	defer os.Remove(bootstrapPath)

	// 4. Build execution command based on OS Affinity tools
	qArgs := append([]string{bootstrapPath}, os.Args[1:]...)
	var cmd *exec.Cmd

	if conf.UseTask {
		switch runtime.GOOS {
		case "linux":
			// taskset -c 0,1 /path/to/q ...
			taskArgs := append([]string{"-c", conf.Cores, qPath}, qArgs...)
			cmd = exec.Command("taskset", taskArgs...)
			fmt.Printf("⚙️  Affinity: taskset -c %s\n", conf.Cores)

		case "windows":
			// Windows affinity uses a hex bitmask. 
			// start /b /affinity <Mask> /wait <Path> <Args>
			// We wrap in cmd /c to access 'start'
			winArgs := []string{"/c", "start", "/b", "/affinity", conf.Cores, "/wait", qPath}
			winArgs = append(winArgs, qArgs...)
			cmd = exec.Command("cmd", winArgs...)
			fmt.Printf("⚙️  Affinity: Windows mask 0x%s\n", conf.Cores)

		default:
			// macOS/Others: No native CLI affinity tool
			cmd = exec.Command(qPath, qArgs...)
		}
	} else {
		cmd = exec.Command(qPath, qArgs...)
	}

	// 5. Environment Setup
	env := os.Environ()
	if sslPath != "" {
		env = append(env, "DYLD_LIBRARY_PATH="+sslPath)
		env = append(env, "DYLD_FALLBACK_LIBRARY_PATH="+sslPath)
	}
	env = append(env, "QHOME="+conf.QHome)
	env = append(env, "KX_SSL_VERIFY_SERVER=NO")
	cmd.Env = env

	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Stdin = os.Stdin

	// 6. Run
	if err := cmd.Run(); err != nil {
		fmt.Printf("\n❌ Process exited: %v\n", err)
	}
}

// --- Logic Helpers ---

func resolveConfig() QiConfig {
	home, _ := os.UserHomeDir()
	dirPath := filepath.Join(home, configDir)
	filePath := filepath.Join(dirPath, configFileName)

	_ = os.MkdirAll(dirPath, 0755)

	if data, err := os.ReadFile(filePath); err == nil {
		lines := strings.Split(string(data), "\n")
		if len(lines) >= 3 {
			return QiConfig{
				QHome:   strings.TrimSpace(lines[0]),
				UseTask: strings.TrimSpace(lines[1]) == "true",
				Cores:   strings.TrimSpace(lines[2]),
			}
		}
	}

	return runWizard(filePath)
}

func runWizard(configPath string) QiConfig {
	reader := bufio.NewReader(os.Stdin)
	fmt.Println("🧙 Qi Setup Wizard")

	// 1. QHOME
	fmt.Print("📂 Enter QHOME (folder containing kc.lic): ")
	qhome, _ := reader.ReadString('\n')
	qhome = strings.TrimSpace(qhome)
	if strings.HasPrefix(qhome, "~") {
		h, _ := os.UserHomeDir()
		qhome = filepath.Join(h, qhome[1:])
	}

	// 2. Affinity
	useTask := false
	cores := "1" // Default for bitmask or list
	if runtime.GOOS == "linux" || runtime.GOOS == "windows" {
		fmt.Printf("⚙️  Apply CPU affinity? (Avoids kdb+ license core errors) (y/n): ")
		ans, _ := reader.ReadString('\n')
		if strings.ToLower(strings.TrimSpace(ans)) == "y" {
			useTask = true
			if runtime.GOOS == "linux" {
				fmt.Print("🔢 Linux Cores (e.g. 0,1): ")
			} else {
				fmt.Print("🔢 Windows Affinity Mask (Hex, e.g. 3 for cores 0&1): ")
			}
			cores, _ = reader.ReadString('\n')
			cores = strings.TrimSpace(cores)
		}
	}

	// 3. Save
	content := fmt.Sprintf("%s\n%t\n%s", qhome, useTask, cores)
	_ = os.WriteFile(configPath, []byte(content), 0644)
	fmt.Printf("✅ Config saved to %s\n", configPath)

	return QiConfig{QHome: qhome, UseTask: useTask, Cores: cores}
}

func getExecutablePath(qhome string) string {
	var sub string
	switch runtime.GOOS {
	case "linux": sub = "l64"
	case "darwin": sub = "m64"
	case "windows": sub = "w64"
	}
	ext := ""
	if runtime.GOOS == "windows" { ext = ".exe" }
	
	return filepath.Join(qhome, sub, "q"+ext)
}

func ensureOpenSSL() string {
	// (Your previous Mac OpenSSL detection logic)
	return ""
}