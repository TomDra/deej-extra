package startup

import (
	"os"

	"golang.org/x/sys/windows/registry"
)

const (
	winRegistryKey   = `Software\Microsoft\Windows\CurrentVersion\Run`
	winRegistryValue = "deej-extra"
)

func enableWindows() error {
    exe, err := os.Executable()
    if err != nil {
        return err
    }

    // Quote the path so Windows handles spaces correctly
    quoted := `"` + exe + `"`

    key, _, err := registry.CreateKey(registry.CURRENT_USER, winRegistryKey, registry.SET_VALUE)
    if err != nil {
        return err
    }
    defer key.Close()
    return key.SetStringValue(winRegistryValue, quoted)
}


func disableWindows() error {
	key, err := registry.OpenKey(registry.CURRENT_USER, winRegistryKey, registry.SET_VALUE)
	if err != nil {
		return err
	}
	defer key.Close()
	return key.DeleteValue(winRegistryValue)
}
