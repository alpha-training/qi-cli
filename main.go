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

	// 1. Resolve Configuration (Alias -> Env -> Config -> Wizard)
	conf := resolveConfig()
	
	// 2. Locate binary
	qPath, err := findQExecutable(conf.QHome)
	if err != nil {
		fmt.Printf("❌ %v\n", err)
		os.Exit(1)
	}

	// 3. Mac-specific OpenSSL
	var sslPath string
	if runtime.GOOS == "darwin" {
		sslPath = "/usr/local/opt/openssl@1.1/lib" // Simplified for brevity
	}

	// 4. Temporary Bootstrap
	bootstrapPath := filepath.Join(os.TempDir(), "qi.bootstrap.q")
	_ = os.WriteFile(bootstrapPath, qibootstrap, 0755)
	defer os.Remove(bootstrapPath)

	// 5. Build Command
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

	// 6. Env & Run
	env := os.Environ()
	if sslPath != "" { env = append(env, "DYLD_LIBRARY_PATH="+sslPath) }
	env = append(env, "QHOME="+conf.QHome)
	cmd.Env = env
	cmd.Stdout, cmd.Stderr, cmd.Stdin = os.Stdout, os.Stderr, os.Stdin

	if err := cmd.Run(); err != nil {
		fmt.Printf("\n❌ kdb+ exited: %v\n", err)
	}
}

// --- Hierarchy of Truth ---

func resolveConfig() QiConfig {
	// A. Check for Shell Alias (Linux/Mac only)
	if runtime.GOOS != "windows" {
		// Try to extract alias from interactive bash
		aliasCmd := exec.Command("bash", "-i", "-c", "alias q")
		out, err := aliasCmd.CombinedOutput()
		if err == nil && strings.Contains(string(out), "alias q=") {
			conf := parseAlias(string(out))
			if conf.QHome != "" {
				fmt.Println("✨ Using settings from shell alias 'q'")
				return conf
			}
		}
	}

	// B. Check Environment
	if envQHome := os.Getenv("QHOME"); envQHome != "" {
		return QiConfig{QHome: envQHome, UseTask: false, Cores: "0,1"}
	}

	// C. Check Config File
	dirPath := filepath.Join(os.Getenv("HOME"), configDir)
	if runtime.GOOS == "windows" { dirPath = filepath.Join(os.Getenv("USERPROFILE"), configDir) }
	
	filePath := filepath.Join(dirPath, configFileName)
	if data, err := os.ReadFile(filePath); err == nil {
		return parseConfigFile(string(data))
	}

	// D. Wizard
	return runWizard(filePath)
}

func parseAlias(line string) QiConfig {
	conf := QiConfig{Cores: "0,1", UseTask: false}
	// Extract cores if taskset present
	if strings.Contains(line, "taskset -c") {
		conf.UseTask = true
		parts := strings.Split(line, "taskset -c")
		if len(parts) > 1 {
			conf.Cores = strings.Fields(parts[1])[0]
		}
	}
	// Extract path to derive QHOME
	fields := strings.Fields(line)
	for _, f := range fields {
		f = strings.Trim(f, "'\"")
		if strings.HasSuffix(f, "/q") {
			conf.QHome = filepath.Dir(filepath.Dir(f))
			break
		}
	}
	return conf
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
	
	// Priority: bin/ -> arch/
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
	return "", fmt.Errorf("could not find q in %s", qhome)
}

func runWizard(configPath string) QiConfig {
	reader := bufio.NewReader(os.Stdin)
	fmt.Println("🧙 Qi Setup Wizard")
	fmt.Print("📂 Enter QHOME path (which contains kc.lic): ")
	qhome, _ := reader.ReadString('\n')
	qhome = strings.TrimSpace(qhome)

	useTask := false
	cores := "0,1"
	if runtime.GOOS == "linux" || runtime.GOOS == "windows" {
		fmt.Print("⚙️  Apply CPU affinity? (y/n): ")
		ans, _ := reader.ReadString('\n')
		if strings.ToLower(strings.TrimSpace(ans)) == "y" {
			useTask = true
			fmt.Print("🔢 Cores/Mask (e.g. 2,3): ")
			cores, _ = reader.ReadString('\n')
			cores = strings.TrimSpace(cores)
		}
	}

	conf := QiConfig{QHome: qhome, UseTask: useTask, Cores: cores}
	_ = os.MkdirAll(filepath.Dir(configPath), 0755)
	content := fmt.Sprintf("QHOME=%s\nUSE_TASK=%t\nCORES=%s\n", qhome, useTask, cores)
	_ = os.WriteFile(configPath, []byte(content), 0644)
	return conf
}