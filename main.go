package main

import (
	_ "embed"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"

	"qi-cli/internal/doctor"
	"qi-cli/internal/kdb"
)

//go:embed qi.q
var embeddedQScript []byte

const qScriptName = "qi.q"

func main() {
	fmt.Printf("--- qi CLI (OS: %s, Arch: %s) ---\n", runtime.GOOS, runtime.GOARCH)

	// 1. Resolve Config via KDB package
	conf := kdb.ResolveConfig()
	conf.QHome = kdb.ExpandHome(conf.QHome)

	// 2. Run the Doctor for Alpaca/SSL
	if err := doctor.CheckAlpaca(); err != nil {
		fmt.Printf("❌ Health check failed: %v\n", err)
	}

	// 3. Find the q binary
	qPath, err := kdb.FindExecutable(conf.QHome)
	if err != nil {
		fmt.Printf("❌ %v\n", err)
		os.Exit(1)
	}

	// 4. Resolve the q script (Hybrid Approach)
	// Priority 1: Local file in CWD (Great for development)
	bootstrapPath := filepath.Join(".", qScriptName)
	
	if _, err := os.Stat(bootstrapPath); os.IsNotExist(err) {
		// Priority 2: Use Embedded script and sync to ~/.qi/qi.q
		home, _ := os.UserHomeDir()
		qiDir := filepath.Join(home, kdb.ConfigDir)
		bootstrapPath = filepath.Join(qiDir, qScriptName)

		// Create ~/.qi if missing and write the embedded bytes
		_ = os.MkdirAll(qiDir, 0755)
		if err := os.WriteFile(bootstrapPath, embeddedQScript, 0644); err != nil {
			fmt.Printf("❌ Failed to extract embedded script: %v\n", err)
			os.Exit(1)
		}
	}

	// 5. Setup Command
	qArgs := append([]string{bootstrapPath}, os.Args[1:]...)
	var cmd *exec.Cmd
	
	if conf.UseTask && (runtime.GOOS == "linux" || runtime.GOOS == "windows") {
		if runtime.GOOS == "linux" {
			cmd = exec.Command("taskset", append([]string{"-c", conf.Cores, qPath}, qArgs...)...)
		} else {
			// Windows affinity via 'start'
			winArgs := append([]string{"/c", "start", "/b", "/affinity", conf.Cores, "/wait", qPath}, qArgs...)
			cmd = exec.Command("cmd", winArgs...)
		}
	} else {
		cmd = exec.Command(qPath, qArgs...)
	}

	// 6. Set Environment
	cmd.Dir = "."
	env := os.Environ()
	if runtime.GOOS == "darwin" {
		// Support both Intel and Apple Silicon Homebrew paths for SSL
		env = append(env, "DYLD_LIBRARY_PATH=/usr/local/opt/openssl@1.1/lib:/opt/homebrew/opt/openssl@1.1/lib")
	}
	env = append(env, "QHOME="+conf.QHome)
	cmd.Env = env
	
	cmd.Stdout, cmd.Stderr, cmd.Stdin = os.Stdout, os.Stderr, os.Stdin

	// 7. Execute
	if err := cmd.Run(); err != nil {
		fmt.Printf("\n❌ kdb+ exited: %v\n", err)
	}
}