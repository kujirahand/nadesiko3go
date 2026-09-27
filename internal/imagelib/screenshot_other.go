//go:build !darwin && !linux && !windows

package imagelib

import (
	"errors"
	"image"
)

// captureScreen は未対応プラットフォーム向けのフォールバック。
func captureScreen(_ string) (*image.RGBA, error) {
	return nil, errors.New("このプラットフォームではスクリーンショット撮影に未対応です。")
}
