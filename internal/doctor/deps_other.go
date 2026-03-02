//go:build !windows

package doctor

func GetDepsPath() string {
	return ""
}
