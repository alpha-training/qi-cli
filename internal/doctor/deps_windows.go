//go:build windows

package doctor

import (
    "os/exec"
    "strings"
)

// Changed name to GetDepsPath to match main.go
func GetDepsPath() string {
    // Run: q qi.q deps-win -quit
    out, err := exec.Command("q", "qi.q", "deps-win", "-quit").Output()
    if err != nil {
        return "" 
    }
    return strings.TrimSpace(string(out))
}