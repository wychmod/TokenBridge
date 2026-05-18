//go:build windows

package desktop

import _ "embed"

//go:embed assets/tray-icon.ico
var trayIcon []byte
