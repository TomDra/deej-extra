package startup

import (
	"fmt"
	"runtime"
)

// Apply turns startup on or off based on `enable`
func Apply(enable bool) error {
	switch runtime.GOOS {
	case "windows":
		if enable {
			return enableWindows()
		}
		return disableWindows()
	case "darwin":
		if enable {
			return enableMac()
		}
		return disableMac()
	case "linux":
		if enable {
			return enableLinux()
		}
		return disableLinux()
	default:
		return fmt.Errorf("unsupported OS for run-on-startup: %s", runtime.GOOS)
	}
}
