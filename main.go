package main

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"

	"qi-cli/internal/doctor" // Ensure this matches your go.mod name
	"qi-cli/internal/kdb"
)

const qScriptName = "qi.q"

func main() {
	fmt.Printf("--- Qi CLI (OS: %s, Arch: %s) ---\n", runtime.GOOS, runtime.GOARCH)

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

	// 4. Verify the local q script
	bootstrapPath := filepath.Join(".", qScriptName)
	if _, err := os.Stat(bootstrapPath); os.IsNotExist(err) {
		abs, _ := filepath.Abs(bootstrapPath)
		fmt.Printf("❌ Cannot find %s at %s\n", qScriptName, abs)
		os.Exit(1)
	}

	// 5. Setup Command
	qArgs := append([]string{bootstrapPath}, os.Args[1:]...)
	cmd := exec.Command(qPath, qArgs...)
	
	// Affinity logic simplified for main
	if conf.UseTask && runtime.GOOS == "linux" {
		cmd = exec.Command("taskset", append([]string{"-c", conf.Cores, qPath}, qArgs...)...)
	}

	// 6. Set Environment
	cmd.Dir = "."
	env := os.Environ()
	if runtime.GOOS == "darwin" {
		// Add both possible Homebrew paths to DYLD
		env = append(env, "DYLD_LIBRARY_PATH=/usr/local/opt/openssl@1.1/lib:/opt/homebrew/opt/openssl@1.1/lib")
	}
	env = append(env, "QHOME="+conf.QHome)
	cmd.Env = env
	
	cmd.Stdout, cmd.Stderr, cmd.Stdin = os.Stdout, os.Stderr, os.Stdin

	if err := cmd.Run(); err != nil {
		fmt.Printf("\n❌ kdb+ exited: %v\n", err)
	}
}