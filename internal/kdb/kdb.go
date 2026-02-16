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

	// 1. Validate before saving
	if _, err := FindExecutable(ExpandHome(qhome)); err != nil {
		fmt.Printf("❌ Invalid path: %v\n", err)
		os.Exit(1) // Stop here so we don't save a broken config
	}

	// 2. Only runs if FindExecutable succeeded
	conf := Config{QHome: qhome, UseTask: false, Cores: "0,1"}
	_ = os.MkdirAll(filepath.Dir(configPath), 0755)
	_ = os.WriteFile(configPath, []byte(fmt.Sprintf("QHOME=%s\nUSE_TASK=false\nCORES=0,1\n", qhome)), 0644)
	
	return conf
}