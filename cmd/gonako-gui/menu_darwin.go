//go:build darwin

package main

/*
#cgo LDFLAGS: -framework Cocoa
void gonakoInstallEditMenu(void);
*/
import "C"

// installPlatformMenus installs the standard macOS edit commands. WKWebView
// handles copy and paste through the responder chain, but Command-C and
// Command-V are only dispatched when the application has matching menu items.
func installPlatformMenus() {
	C.gonakoInstallEditMenu()
}
