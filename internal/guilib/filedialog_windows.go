//go:build windows

package guilib

import (
	"fmt"
	"runtime"
	"syscall"
	"unicode/utf16"
	"unsafe"
)

const (
	ofnOverwritePrompt   = 0x00000002
	ofnNoChangeDir       = 0x00000008
	ofnPathMustExist     = 0x00000800
	ofnFileMustExist     = 0x00001000
	ofnExplorer          = 0x00080000
	bifReturnOnlyFSDirs  = 0x00000001
	bifEditBox           = 0x00000010
	bifNewDialogStyle    = 0x00000040
	bffmInitialized      = 1
	bffmSetSelectionWide = 0x0400 + 103
)

type openFileNameW struct {
	structSize       uint32
	owner            uintptr
	instance         uintptr
	filter           *uint16
	customFilter     *uint16
	maxCustomFilter  uint32
	filterIndex      uint32
	file             *uint16
	maxFile          uint32
	fileTitle        *uint16
	maxFileTitle     uint32
	initialDir       *uint16
	title            *uint16
	flags            uint32
	fileOffset       uint16
	fileExtension    uint16
	defaultExtension *uint16
	customData       uintptr
	hook             uintptr
	templateName     *uint16
	reserved         unsafe.Pointer
	reservedSize     uint32
	flagsEx          uint32
}

type browseInfoW struct {
	owner       uintptr
	root        uintptr
	displayName *uint16
	title       *uint16
	flags       uint32
	callback    uintptr
	param       uintptr
	image       int32
}

var (
	comdlg32             = syscall.NewLazyDLL("comdlg32.dll")
	getOpenFileNameW     = comdlg32.NewProc("GetOpenFileNameW")
	getSaveFileNameW     = comdlg32.NewProc("GetSaveFileNameW")
	commDlgExtendedError = comdlg32.NewProc("CommDlgExtendedError")
	shell32              = syscall.NewLazyDLL("shell32.dll")
	shBrowseForFolderW   = shell32.NewProc("SHBrowseForFolderW")
	shGetPathFromIDListW = shell32.NewProc("SHGetPathFromIDListW")
	ole32                = syscall.NewLazyDLL("ole32.dll")
	coTaskMemFree        = ole32.NewProc("CoTaskMemFree")
	user32               = syscall.NewLazyDLL("user32.dll")
	getActiveWindow      = user32.NewProc("GetActiveWindow")
	sendMessageW         = user32.NewProc("SendMessageW")
	browseCallback       = syscall.NewCallback(func(window uintptr, message uint32, _ uintptr, data uintptr) uintptr {
		if message == bffmInitialized && data != 0 {
			sendMessageW.Call(window, bffmSetSelectionWide, 1, data)
		}
		return 0
	})
)

func utf16WithNULs(value string) []uint16 {
	return append(utf16.Encode([]rune(value)), 0)
}

func windowsDialogFilter(extension string) []uint16 {
	if extension == "" {
		return utf16WithNULs("すべてのファイル (*.*)\x00*.*\x00")
	}
	pattern := "*" + extension
	return utf16WithNULs("対象ファイル (" + pattern + ")\x00" + pattern + "\x00すべてのファイル (*.*)\x00*.*\x00")
}

func activeWindowHandle() uintptr {
	handle, _, _ := getActiveWindow.Call()
	return handle
}

func commonDialogError() error {
	code, _, _ := commDlgExtendedError.Call()
	if code == 0 {
		return nil
	}
	return fmt.Errorf("Windowsファイルダイアログエラー: 0x%04X", code)
}

func openFileDialogPlatform(defaultDir, extension string) (string, error) {
	fileBuffer := make([]uint16, 32768)
	initialDir, err := syscall.UTF16PtrFromString(defaultDir)
	if err != nil {
		return "", err
	}
	title, _ := syscall.UTF16PtrFromString("ファイルを開く")
	filter := windowsDialogFilter(extension)
	dialog := openFileNameW{
		owner:       activeWindowHandle(),
		filter:      &filter[0],
		filterIndex: 1,
		file:        &fileBuffer[0],
		maxFile:     uint32(len(fileBuffer)),
		initialDir:  initialDir,
		title:       title,
		flags:       ofnExplorer | ofnFileMustExist | ofnPathMustExist | ofnNoChangeDir,
	}
	dialog.structSize = uint32(unsafe.Sizeof(dialog))
	if ok, _, _ := getOpenFileNameW.Call(uintptr(unsafe.Pointer(&dialog))); ok == 0 {
		return "", commonDialogError()
	}
	return syscall.UTF16ToString(fileBuffer), nil
}

func saveFileDialogPlatform(defaultDir, defaultName, extension string) (string, error) {
	fileBuffer := make([]uint16, 32768)
	name, err := syscall.UTF16FromString(defaultName)
	if err != nil {
		return "", err
	}
	copy(fileBuffer, name)
	initialDir, err := syscall.UTF16PtrFromString(defaultDir)
	if err != nil {
		return "", err
	}
	title, _ := syscall.UTF16PtrFromString("名前を付けて保存")
	filter := windowsDialogFilter(extension)
	var defaultExtension *uint16
	if extension != "" {
		defaultExtension, _ = syscall.UTF16PtrFromString(extension[1:])
	}
	dialog := openFileNameW{
		owner:            activeWindowHandle(),
		filter:           &filter[0],
		filterIndex:      1,
		file:             &fileBuffer[0],
		maxFile:          uint32(len(fileBuffer)),
		initialDir:       initialDir,
		title:            title,
		flags:            ofnExplorer | ofnOverwritePrompt | ofnPathMustExist | ofnNoChangeDir,
		defaultExtension: defaultExtension,
	}
	dialog.structSize = uint32(unsafe.Sizeof(dialog))
	if ok, _, _ := getSaveFileNameW.Call(uintptr(unsafe.Pointer(&dialog))); ok == 0 {
		return "", commonDialogError()
	}
	return syscall.UTF16ToString(fileBuffer), nil
}

func folderDialogPlatform(defaultDir string) (string, error) {
	displayName := make([]uint16, 260)
	title, _ := syscall.UTF16PtrFromString("フォルダを選択")
	initialDir, err := syscall.UTF16PtrFromString(defaultDir)
	if err != nil {
		return "", err
	}
	dialog := browseInfoW{
		owner:       activeWindowHandle(),
		displayName: &displayName[0],
		title:       title,
		flags:       bifReturnOnlyFSDirs | bifEditBox | bifNewDialogStyle,
		callback:    browseCallback,
		param:       uintptr(unsafe.Pointer(initialDir)),
	}
	itemIDList, _, _ := shBrowseForFolderW.Call(uintptr(unsafe.Pointer(&dialog)))
	runtime.KeepAlive(initialDir)
	if itemIDList == 0 {
		return "", nil
	}
	defer coTaskMemFree.Call(itemIDList)
	pathBuffer := make([]uint16, 32768)
	if ok, _, callErr := shGetPathFromIDListW.Call(itemIDList, uintptr(unsafe.Pointer(&pathBuffer[0]))); ok == 0 {
		return "", fmt.Errorf("選択したフォルダのパスを取得できません: %v", callErr)
	}
	return syscall.UTF16ToString(pathBuffer), nil
}
