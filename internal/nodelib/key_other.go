//go:build !darwin && !windows && !linux

package nodelib

import "fmt"

// sendKeyStrokes は未対応のプラットフォームでは常にエラーを返します。
func sendKeyStrokes(_ []keyStroke) error {
	return fmt.Errorf("このプラットフォームではキー送信に未対応です")
}

// releaseOSHeldModifiers は未対応のプラットフォームでは何もしません。
func releaseOSHeldModifiers(_ keyMods) {}
