//go:build !darwin && !windows && !linux

package nodelib

import "errors"

var errWindowUnsupported = errors.New("このプラットフォームではウィンドウ操作に未対応です")

type systemWindowDriver struct{}

func (systemWindowDriver) List() ([]windowInfo, error)  { return nil, errWindowUnsupported }
func (systemWindowDriver) Activate(int64) error         { return errWindowUnsupported }
func (systemWindowDriver) Move(int64, int, int) error   { return errWindowUnsupported }
func (systemWindowDriver) Size(int64) (int, int, error) { return 0, 0, errWindowUnsupported }
func (systemWindowDriver) Resize(int64, int, int) error { return errWindowUnsupported }
