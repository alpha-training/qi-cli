package doctor

import (
    "os"
    "os/exec"
    "strings"
)

// ResolveWindowsPath runs the kdb command to find where the DLLs live
func ResolveWindowsPath() string {
    // Run: q qi.q deps-win -quit
    out, err := exec.Command("q", "qi.q", "deps-win", "-quit").Output()
    if err != nil {
        return "" 
    }
    return strings.TrimSpace(string(out))
}