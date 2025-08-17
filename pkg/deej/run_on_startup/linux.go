//Never Tested

package startup

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
)

const (
	linuxServiceFilename = "deej.service"
	linuxServiceBody     = `[Unit]
Description=deej autostart

[Service]
ExecStart=%s

[Install]
WantedBy=default.target`
)

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
