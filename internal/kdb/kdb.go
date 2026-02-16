package kdb

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
)

type Config struct {
	QHome   string
	UseTask bool
	Cores   string
}

const (
	ConfigDir      = ".qi"
	ConfigFileName = "config"
)

func ResolveConfig() Config {
	if envQHome := os.Getenv("QHOME"); envQHome != "" {
		return Config{QHome: envQHome, UseTask: false, Cores: "0,1"}
	}

	home, _ := os.UserHomeDir()
	filePath := filepath.Join(home, ConfigDir, ConfigFileName)
	
	if data, err := os.ReadFile(filePath); err == nil {
		return parseConfigFile(string(data))
	}

	return RunWizard(filePath)
}

func parseConfigFile(data string) Config {
	conf := Config{Cores: "0,1"}
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

func ExpandHome(path string) string {
	if strings.HasPrefix(path, "~") {
		home, _ := os.UserHomeDir()
		return filepath.Join(home, path[1:])
	}
	return path
}

func FindExecutable(qhome string) (string, error) {
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

func RunWizard(configPath string) Config {
	reader := bufio.NewReader(os.Stdin)
	fmt.Print("📂 Enter QHOME path: ")
	qhome, _ := reader.ReadString('\n')
	qhome = strings.TrimSpace(qhome)

	// 1. Validate path immediately
	if _, err := FindExecutable(ExpandHome(qhome)); err != nil {
		fmt.Printf("❌ Invalid path: %v\n", err)
		os.Exit(1)
	}

	useTask := false
	cores := "0,1"

	// 2. Only ask affinity on supported OSs
	if runtime.GOOS == "linux" || runtime.GOOS == "windows" {
		fmt.Print("⚙️  Apply CPU affinity? (y/n): ")
		ans, _ := reader.ReadString('\n')
		if strings.ToLower(strings.TrimSpace(ans)) == "y" {
			useTask = true
			fmt.Print("🔢 Cores/Mask (e.g. 0,1): ")
			cores, _ = reader.ReadString('\n')
			cores = strings.TrimSpace(cores)
		}
	}

	// 3. Save validated config
	conf := Config{QHome: qhome, UseTask: useTask, Cores: cores}
	_ = os.MkdirAll(filepath.Dir(configPath), 0755)
	content := fmt.Sprintf("QHOME=%s\nUSE_TASK=%t\nCORES=%s\n", qhome, useTask, cores)
	_ = os.WriteFile(configPath, []byte(content), 0644)
	
	fmt.Printf("✅ Config saved to %s\n", configPath)
	return conf
}