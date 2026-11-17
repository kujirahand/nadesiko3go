package main

// guiRunErrorDestination は gonako-gui run の起動前エラーの届け先。
type guiRunErrorDestination int

const (
	// guiRunErrorToStderr は標準エラー（コンソール、または呼び出し元が
	// 繋いだパイプやファイル）へ書く。終了コード1と合わせて、スクリプトや
	// AIエージェントが失敗を読める。
	guiRunErrorToStderr guiRunErrorDestination = iota
	// guiRunErrorToDialog は標準エラーの宛先も親コンソールも無いときに使う。
	// Windowsのリリース版（windowsgui）をエクスプローラから起動した場合が
	// これに当たる。
	guiRunErrorToDialog
)

// Win32 の GetFileType が返す値。golang.org/x/sys/windows の FILE_TYPE_* と
// 同じ数値である。
const (
	stderrFileTypeDisk    uint32 = 0x0001
	stderrFileTypeChar    uint32 = 0x0002
	stderrFileTypePipe    uint32 = 0x0003
	stderrFileTypeRemote  uint32 = 0x8000
	stderrFileTypeUnknown uint32 = 0x0000
)

// showGUIRunErrorDialog はコンソールもリダイレクトも無いときにエラーを見せる。
// テストから差し替えられる。
var showGUIRunErrorDialog = platformShowGUIRunErrorDialog

// chooseGUIRunErrorDestination は起動前エラーをどこへ出すかを決める。
// stderrVisible は標準エラーがコンソール、パイプ、ファイルのいずれかで
// 呼び出し元から読めるときに真。parentConsole は親プロセスのコンソールへ
// 接続できるときに真。どちらも無ければダイアログにする。
func chooseGUIRunErrorDestination(stderrVisible, parentConsole bool) guiRunErrorDestination {
	if stderrVisible || parentConsole {
		return guiRunErrorToStderr
	}
	return guiRunErrorToDialog
}

// classifyStderrVisibility は標準エラーのハンドルが、呼び出し元から読める
// 出力先かを返す。コンソール、パイプ、ファイルは読める。無効なハンドル、
// 不明な種別、コンソールではない文字デバイス（windowsgui で標準エラーが
// 捨てられる場合）は読めない。fileType の FILE_TYPE_REMOTE ビットは無視する。
func classifyStderrVisibility(valid bool, fileType uint32, isConsole bool) bool {
	if !valid {
		return false
	}
	switch fileType &^ stderrFileTypeRemote {
	case stderrFileTypePipe, stderrFileTypeDisk:
		return true
	case stderrFileTypeChar:
		return isConsole
	default:
		return false
	}
}

// guiRunErrorText は起動前エラーと使い方を1つの文面にまとめる。
func guiRunErrorText(err error) string {
	return err.Error() + "\n使い方: gonako-gui run main.nako3"
}

// reportGUIRunError は起動前エラーを届ける。標準エラーへ書けるとき、または
// 親コンソールへ接続できるときはそちらへ書き、どちらも無いときだけ
// ダイアログを出す。終了は呼び出し元が行う。
func reportGUIRunError(err error) {
	text := guiRunErrorText(err)
	if deliverGUIRunError(text) {
		return
	}
	showGUIRunErrorDialog(text)
}
