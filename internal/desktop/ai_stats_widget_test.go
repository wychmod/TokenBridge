package desktop

import (
	"strings"
	"testing"

	"github.com/wailsapp/wails/v2/pkg/options/windows"
)

func TestAIStatsWidgetHTMLResolvesMovedDesktopBridge(t *testing.T) {
	if !strings.Contains(aiStatsWidgetHTML, "window.go?.desktop?.AIStatsWidget") {
		t.Fatalf("widget bridge resolver must support the moved internal/desktop package namespace")
	}
	if !strings.Contains(aiStatsWidgetHTML, "window.go?.main?.AIStatsWidget") {
		t.Fatalf("widget bridge resolver must keep the legacy main package namespace")
	}
}

func TestAIStatsWidgetWindowsOptionsKeepRoundedTransparentWindow(t *testing.T) {
	opts := aiStatsWidgetWindowsOptions()
	if opts == nil {
		t.Fatalf("expected Windows options")
	}
	if !opts.WebviewIsTransparent {
		t.Fatalf("widget webview must stay transparent")
	}
	if !opts.WindowIsTranslucent {
		t.Fatalf("widget window must stay translucent so rounded corners are transparent")
	}
	if opts.BackdropType != windows.None {
		t.Fatalf("widget backdrop must not add an opaque system background, got %v", opts.BackdropType)
	}
}
