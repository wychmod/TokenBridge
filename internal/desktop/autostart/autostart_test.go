package autostart

import (
	"path/filepath"
	"strings"
	"testing"
)

func TestWindowsCommandQuotesExecutablePath(t *testing.T) {
	exePath := filepath.Join("C:\\Program Files", "TokenBridge", "TokenBridge.exe")

	got := windowsCommand(exePath)

	if got != `"C:\Program Files\TokenBridge\TokenBridge.exe"` {
		t.Fatalf("windowsCommand() = %q", got)
	}
}

func TestLaunchAgentPlistUsesCurrentExecutable(t *testing.T) {
	exePath := "/Applications/TokenBridge.app/Contents/MacOS/TokenBridge"

	got := launchAgentPlist(exePath)

	for _, want := range []string{
		"<key>Label</key>",
		"<string>dev.tokenbridge.app</string>",
		"<key>ProgramArguments</key>",
		"<string>/Applications/TokenBridge.app/Contents/MacOS/TokenBridge</string>",
		"<key>RunAtLoad</key>",
		"<true/>",
	} {
		if !strings.Contains(got, want) {
			t.Fatalf("launchAgentPlist() missing %q in:\n%s", want, got)
		}
	}
}

func TestLaunchAgentPlistEscapesExecutablePath(t *testing.T) {
	got := launchAgentPlist("/Applications/Token&Bridge.app/Contents/MacOS/Token<Bridge>")

	if !strings.Contains(got, "<string>/Applications/Token&amp;Bridge.app/Contents/MacOS/Token&lt;Bridge&gt;</string>") {
		t.Fatalf("launchAgentPlist() did not escape executable path:\n%s", got)
	}
}
