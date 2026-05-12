//go:build windows

package main

import _ "embed"

//go:embed cmd/tokenbridge/tray-icon.ico
var trayIcon []byte
