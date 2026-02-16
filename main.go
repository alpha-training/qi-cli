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
	Cores   string
}

func main() {
	fmt.Printf("--- Qi CLI (OS: %s, Arch: %s) ---\n", runtime.GOOS, runtime.GOARCH)

	conf := resolveConfig()
	
	// Resolve tilde for QHome before we try to find the executable
	conf.QHome = expandHome(conf.QHome)

	qPath, err := findQExecutable(conf.QHome)
	if err != nil {
		fmt.Printf("❌ %v\n", err)
		os.Exit(1)
	}

	var sslPath string
	if runtime.GOOS == "darwin" {
		sslPath = "/usr/local/opt/openssl@1.1/lib" 
	}

	bootstrapPath := filepath.Join(os.TempDir(), "qi.bootstrap.q")
	_ = os.WriteFile(bootstrapPath, qibootstrap, 0755)
	defer os.Remove(bootstrapPath)

	qArgs := append([]string{bootstrapPath}, os.Args[1:]...)
	var cmd *exec.Cmd

	if conf.UseTask {
		if runtime.GOOS == "linux" {
			cmd = exec.Command("taskset", append([]string{"-c", conf.Cores, qPath}, qArgs...)...)
			fmt.Printf("⚙️  Affinity: taskset -c %s\n", conf.Cores)
		} else if runtime.GOOS == "windows" {
			winArgs := append([]string{"/c", "start", "/b", "/affinity", conf.Cores, "/wait", qPath}, qArgs...)
			cmd = exec.Command("cmd", winArgs...)
		} else {
			cmd = exec.Command(qPath, qArgs...)
		}
	} else {
		cmd = exec.Command(qPath, qArgs...)
	}

	env := os.Environ()
	if sslPath != "" { env = append(env, "DYLD_LIBRARY_PATH="+sslPath) }
	env = append(env, "QHOME="+conf.QHome)
	cmd.Env = env
	cmd.Stdout, cmd.Stderr, cmd.Stdin = os.Stdout, os.Stderr, os.Stdin

	if err := cmd.Run(); err != nil {
		fmt.Printf("\n❌ kdb+ exited: %v\n", err)
	}
}

// expandHome replaces ~ at the start of a path with the user's home directory
func expandHome(path string) string {
	if strings.HasPrefix(path, "~") {
		home, err := os.UserHomeDir()
		if err != nil {
			return path
		}
		return filepath.Join(home, path[1:])
	}
	return path
}

func resolveConfig() QiConfig {
	if envQHome := os.Getenv("QHOME"); envQHome != "" {
		fmt.Printf("🌱 Using QHOME from environment: %s\n", envQHome)
		return QiConfig{QHome: envQHome, UseTask: false, Cores: "0,1"}
	}

	dirPath := filepath.Join(os.Getenv("HOME"), configDir)
	if runtime.GOOS == "windows" {
		dirPath = filepath.Join(os.Getenv("USERPROFILE"), configDir)
	}
	
	filePath := filepath.Join(dirPath, configFileName)
	if data, err := os.ReadFile(filePath); err == nil {
		fmt.Printf("📄 Using configuration from: %s\n", filePath)
		return parseConfigFile(string(data))
	}

	return runWizard(filePath)
}

func parseConfigFile(data string) QiConfig {
	conf := QiConfig{Cores: "0,1"}
	scanner := bufio.NewScanner(strings.NewReader(data))
	for scanner.Scan() {
		parts := strings.SplitN(scanner.Text(), "=", 2)
		if len(parts) != 2 { continue }
		key, val := strings.ToUpper(strings.TrimSpace(parts[0])), strings.TrimSpace(parts[1])
		switch key {
		case "QHOME": conf.QHome = val
		case "USE_TASK": conf.UseTask = (val == "true")
		case "CORES": conf.Cores = val
		}
	}
	return conf
}

func findQExecutable(qhome string) (string, error) {
	ext := ""
	if runtime.GOOS == "windows" { ext = ".exe" }
	
	var arch string
	switch runtime.GOOS {
	case "linux": arch = "l64"
	case "darwin": arch = "m64"
	case "windows": arch = "w64"
	}

	paths := []string{
		filepath.Join(qhome, "bin", "q"+ext),
		filepath.Join(qhome, arch, "q"+ext),
	}

	for _, p := range paths {
		if _, err := os.Stat(p); err == nil { return p, nil }
	}
	return "", fmt.Errorf("could not find q in %s/bin or %s/%s", qhome, qhome, arch)
}

func runWizard(configPath string) QiConfig {
	reader := bufio.NewReader(os.Stdin)
	fmt.Println("🧙 Qi Setup Wizard")
	fmt.Print("📂 Enter QHOME path (where kc.lic is located): ")
	qhome, _ := reader.ReadString('\n')
	qhome = strings.TrimSpace(qhome)

	useTask := false
	cores := "0,1"
	if runtime.GOOS == "linux" || runtime.GOOS == "windows" {
		fmt.Print("⚙️  Apply CPU affinity? (y/n): ")
		ans, _ := reader.ReadString('\n')
		if strings.ToLower(strings.TrimSpace(ans)) == "y" {
			useTask = true
			fmt.Print("🔢 Cores/Mask: ")
			cores, _ = reader.ReadString('\n')
			cores = strings.TrimSpace(cores)
		}
	}

	conf := QiConfig{QHome: qhome, UseTask: useTask, Cores: cores}
	_ = os.MkdirAll(filepath.Dir(configPath), 0755)
	content := fmt.Sprintf("QHOME=%s\nUSE_TASK=%t\nCORES=%s\n", qhome, useTask, cores)
	_ = os.WriteFile(configPath, []byte(content), 0644)
	
	fmt.Printf("✅ Config saved to %s\n", configPath)
	return conf
}