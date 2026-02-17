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

	// 2. Find the q binary
	qPath, err := kdb.FindExecutable(conf.QHome)
	if err != nil {
		fmt.Printf("❌ %v\n", err)
		os.Exit(1)
	}

	// 3. Resolve the q script (Local file has priority for dev)
	bootstrapPath := resolveScriptPath()

	// 4. Setup Command and Arguments
	qArgs := append([]string{bootstrapPath}, os.Args[1:]...)
	cmd := buildCommand(qPath, qArgs, conf)

	// 5. Prepare Environment (The "Injection" Layer)
	cmd.Env = prepareEnv(conf)
	cmd.Dir = "."
	cmd.Stdout, cmd.Stderr, cmd.Stdin = os.Stdout, os.Stderr, os.Stdin

	// 6. Execute
	if err := cmd.Run(); err != nil {
		// Handle specific exit codes if necessary
		fmt.Printf("\n❌ kdb+ exited: %v\n", err)
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
	if conf.UseTask && (runtime.GOOS == "linux" || runtime.GOOS == "windows") {
		if runtime.GOOS == "linux" {
			return exec.Command("taskset", append([]string{"-c", conf.Cores, qPath}, qArgs...)...)
		}
		// Windows affinity
		winArgs := append([]string{"/c", "start", "/b", "/affinity", conf.Cores, "/wait", qPath}, qArgs...)
		return exec.Command("cmd", winArgs...)
	}
	return exec.Command(qPath, qArgs...)
}

// prepareEnv merges system env, config env, and doctor fixes
func prepareEnv(conf kdb.Config) []string {
	env := os.Environ()

	// 1. Essential kdb+ variables
	env = append(env, "QHOME="+conf.QHome)

	// 2. Add extra variables from .qi.conf (SSL_VERIFY_SERVER, etc.)
	// This loop assumes you've updated your kdb.Config to include a map called ExtraEnv
	for k, v := range conf.ExtraEnv {
		env = append(env, fmt.Sprintf("%s=%s", k, v))
	}

	// 3. OS-Specific Fixes (The Doctor's input)
	if runtime.GOOS == "darwin" {
		env = append(env, "DYLD_LIBRARY_PATH=/usr/local/opt/openssl@1.1/lib:/opt/homebrew/opt/openssl@1.1/lib")
	}

	// 4. Run Doctor-level SSL resolution for Linux
	sslFixes := doctor.ResolveSSL(runtime.GOOS)
	for k, v := range sslFixes {
		// Only apply if the user hasn't explicitly set it in their .qi.conf
		if _, overridden := conf.ExtraEnv[k]; !overridden {
			env = append(env, fmt.Sprintf("%s=%s", k, v))
		}
	}

	return env
}