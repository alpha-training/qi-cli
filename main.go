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

	// 1. Load Configuration (Hierarchy: Local > Global > Defaults)
	conf := kdb.ResolveConfig()
	conf.QHome = kdb.ExpandHome(conf.QHome)

	// 2. Doctor Check (Alpaca Mode)
	if len(os.Args) > 1 && os.Args[1] == "alpaca" {
		// We run the doctor unconditionally here.
		// It checks for SSL dependencies AND valid API keys.
		// It is "quiet" if everything is already found.
		if err := doctor.CheckAlpaca(); err != nil {
			fmt.Printf("❌ Setup failed: %v\n", err)
			os.Exit(1)
		}

		// IMPORTANT: Reload config so 'conf' now contains the new keys
		// if the user just entered them.
		conf = kdb.ResolveConfig()
	}

	// 3. Find the q binary
	qPath, err := kdb.FindExecutable(conf.QHome)
	if err != nil {
		fmt.Printf("❌ %v\n", err)
		os.Exit(1)
	}

	// 4. Resolve the q script (Local file has priority for dev)
	bootstrapPath := resolveScriptPath()

	// 5. Setup Command and Arguments
	qArgs := append([]string{bootstrapPath}, os.Args[1:]...)
	cmd := buildCommand(qPath, qArgs, conf)

	// 6. Prepare Environment (The "Injection" Layer)
	cmd.Env = prepareEnv(conf)
	cmd.Dir = "."
	cmd.Stdout, cmd.Stderr, cmd.Stdin = os.Stdout, os.Stderr, os.Stdin

	// 7. Execute
	if err := cmd.Run(); err != nil {
		fmt.Printf("\n❌ kdb+ exited: %v\n", err)
		// Propagate exit code if possible, essentially transparent wrapping
		if exitError, ok := err.(*exec.ExitError); ok {
			os.Exit(exitError.ExitCode())
		}
		os.Exit(1)
	}
}

// resolveScriptPath handles the hybrid embedded/local script logic
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

// buildCommand handles OS-specific affinity wrapping
func buildCommand(qPath string, qArgs []string, conf kdb.Config) *exec.Cmd {
	if conf.UseTask {
		if runtime.GOOS == "linux" {
			// taskset -c 0-3 q ...
			return exec.Command("taskset", append([]string{"-c", conf.Cores, qPath}, qArgs...)...)
		} else if runtime.GOOS == "windows" {
			// start /affinity F q ...
			// Note: Windows 'start' is a shell built-in, so we must invoke "cmd /c"
			winArgs := append([]string{"/c", "start", "/b", "/affinity", conf.Cores, "/wait", qPath}, qArgs...)
			return exec.Command("cmd", winArgs...)
		}
	}
	// Default: run q directly
	return exec.Command(qPath, qArgs...)
}

// prepareEnv merges system env, config env, and doctor fixes
func prepareEnv(conf kdb.Config) []string {
	env := os.Environ()

	// 1. Essential kdb+ variables
	env = append(env, "QHOME="+conf.QHome)

	// 2. Add extra variables from .qi.conf
	for k, v := range conf.ExtraEnv {
		env = append(env, fmt.Sprintf("%s=%s", k, v))
	}

	// 3. OS-Specific Fixes (The Doctor's input)
	if runtime.GOOS == "darwin" {
		// Ensure OpenSSL libraries are discoverable by kdb+
		env = append(env, "DYLD_LIBRARY_PATH=/usr/local/opt/openssl@1.1/lib:/opt/homebrew/opt/openssl@1.1/lib")
	}

	// 4. Run Doctor-level SSL resolution for Linux (if applicable)
	// Ensure you have implemented ResolveSSL in your doctor package,
	// or remove this block if you only target Mac/Windows.
	if runtime.GOOS == "linux" {
		sslFixes := doctor.ResolveSSL(runtime.GOOS)
		for k, v := range sslFixes {
			if _, overridden := conf.ExtraEnv[k]; !overridden {
				env = append(env, fmt.Sprintf("%s=%s", k, v))
			}
		}
	}
	return env
}