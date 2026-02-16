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
	
	qPath, err := findQExecutable(conf.QHome)
	if err != nil {
		fmt.Printf("❌ %v\n", err)
		os.Exit(1)
	}

	var sslPath string
	if runtime.GOOS == "darwin" {
		sslPath = ensureOpenSSL()
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
	if sslPath != "" {
		env = append(env, "DYLD_LIBRARY_PATH="+sslPath)
	}
	env = append(env, "QHOME="+conf.QHome)
	env = append(env, "KX_SSL_VERIFY_SERVER=NO")
	cmd.Env = env

	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Stdin = os.Stdin

	_ = cmd.Run()
}

// --- Key-Value Configuration Logic ---

func resolveConfig() QiConfig {
	dirPath := getConfigPath()
	filePath := filepath.Join(dirPath, configFileName)
	_ = os.MkdirAll(dirPath, 0755)

	conf := QiConfig{Cores: "0,1"} // Defaults
	
	data, err := os.ReadFile(filePath)
	if err == nil {
		scanner := bufio.NewScanner(strings.NewReader(string(data)))
		for scanner.Scan() {
			line := strings.TrimSpace(scanner.Text())
			if line == "" || strings.HasPrefix(line, "#") { continue }
			
			parts := strings.SplitN(line, "=", 2)
			if len(parts) != 2 { continue }
			
			key := strings.ToUpper(strings.TrimSpace(parts[0]))
			val := strings.TrimSpace(parts[1])

			switch key {
			case "QHOME":    conf.QHome = val
			case "USE_TASK": conf.UseTask = (val == "true")
			case "CORES":    conf.Cores = val
			}
		}
	}

	if conf.QHome == "" {
		return runWizard(filePath)
	}
	return conf
}

func saveConfig(path string, conf QiConfig) {
	content := fmt.Sprintf("# Qi CLI Configuration\nQHOME=%s\nUSE_TASK=%t\nCORES=%s\n", 
		conf.QHome, conf.UseTask, conf.Cores)
	_ = os.WriteFile(path, []byte(content), 0644)
}

func runWizard(configPath string) QiConfig {
	reader := bufio.NewReader(os.Stdin)
	fmt.Println("🧙 Qi Setup Wizard")

	fmt.Print("📂 Enter QHOME path (where kc.lic is stored): ")
	qhome, _ := reader.ReadString('\n')
	qhome = strings.TrimSpace(qhome)

	useTask := false
	cores := "0,1"
	if runtime.GOOS == "linux" || runtime.GOOS == "windows" {
		fmt.Print("⚙️  Apply CPU affinity? (y/n): ")
		ans, _ := reader.ReadString('\n')
		if strings.ToLower(strings.TrimSpace(ans)) == "y" {
			useTask = true
			fmt.Print("🔢 Cores/Mask: e.g. 2,3")
			cores, _ = reader.ReadString('\n')
			cores = strings.TrimSpace(cores)
		}
	}

	conf := QiConfig{QHome: qhome, UseTask: useTask, Cores: cores}
	saveConfig(configPath, conf)
	fmt.Printf("✅ Config saved to %s\n", configPath)
	return conf
}

// ... (findQExecutable and ensureOpenSSL helpers remain the same as previous)

func findQExecutable(qhome string) (string, error) {
	ext := ""
	if runtime.GOOS == "windows" { ext = ".exe" }
	
	// Check bin first, then arch folder
	paths := []string{
		filepath.Join(qhome, "bin", "q"+ext),
		filepath.Join(qhome, "l64", "q"+ext),
		filepath.Join(qhome, "m64", "q"+ext),
		filepath.Join(qhome, "w64", "q"+ext),
	}

	for _, p := range paths {
		if _, err := os.Stat(p); err == nil { return p, nil }
	}
	return "", fmt.Errorf("could not find q binary in %s", qhome)
}

func getConfigPath() string {
	home, _ := os.UserHomeDir()
	return filepath.Join(home, configDir)
}

func ensureOpenSSL() string { return "" }