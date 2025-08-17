//Never Tested and no full support for mac. so just added in case mac supported in future.

package startup

import (
	"fmt"
	"os"
	"path/filepath"
)

const (
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
)

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
