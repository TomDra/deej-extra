package startup

import (
    "fmt"
    "os"
    "os/exec"
    "path/filepath"
    "runtime"

    "golang.org/x/sys/windows/registry"
)

const (
    winRegistryKey   = `Software\Microsoft\Windows\CurrentVersion\Run`
    winRegistryValue = "deej"

    macPlistFilename = "com.deej.startup.plist"
    macPlistBody     = `<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN"
 "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0">
<dict>
  <key>Label</key><string>com.deej.startup</string>
  <key>ProgramArguments</key><array>
    <string>%s</string>
  </array>
  <key>RunAtLoad</key><true/>
</dict>
</plist>`

    linuxServiceFilename = "deej.service"
    linuxServiceBody     = `[Unit]
Description=deej autostart

[Service]
ExecStart=%s

[Install]
WantedBy=default.target`
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

func enableWindows() error {
    exe, err := os.Executable()
    if err != nil {
        return err
    }
    key, _, err := registry.CreateKey(registry.CURRENT_USER, winRegistryKey, registry.SET_VALUE)
    if err != nil {
        return err
    }
    defer key.Close()
    return key.SetStringValue(winRegistryValue, exe)
}

func disableWindows() error {
    key, err := registry.OpenKey(registry.CURRENT_USER, winRegistryKey, registry.SET_VALUE)
    if err != nil {
        return err
    }
    defer key.Close()
    return key.DeleteValue(winRegistryValue)
}

func enableMac() error {
    exe, err := os.Executable()
    if err != nil {
        return err
    }
    plist := fmt.Sprintf(macPlistBody, exe)
    path := filepath.Join(os.Getenv("HOME"), "Library/LaunchAgents", macPlistFilename)
    if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
        return err
    }
    return os.WriteFile(path, []byte(plist), 0644)
}

func disableMac() error {
    path := filepath.Join(os.Getenv("HOME"), "Library/LaunchAgents", macPlistFilename)
    return os.Remove(path)
}

func enableLinux() error {
    exe, err := os.Executable()
    if err != nil {
        return err
    }
    service := fmt.Sprintf(linuxServiceBody, exe)
    dir := filepath.Join(os.Getenv("HOME"), ".config", "systemd", "user")
    if err := os.MkdirAll(dir, 0755); err != nil {
        return err
    }
    path := filepath.Join(dir, linuxServiceFilename)
    if err := os.WriteFile(path, []byte(service), 0644); err != nil {
        return err
    }
    // enable & start
    exec.Command("systemctl", "--user", "enable", "deej.service").Run()
    exec.Command("systemctl", "--user", "start", "deej.service").Run()
    return nil
}

func disableLinux() error {
    exec.Command("systemctl", "--user", "disable", "deej.service").Run()
    path := filepath.Join(os.Getenv("HOME"), ".config", "systemd", "user", linuxServiceFilename)
    return os.Remove(path)
}
