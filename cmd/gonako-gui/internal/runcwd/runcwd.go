// Package runcwd はGUI実行時の作業ディレクトリを扱う。
package runcwd

import (
	"fmt"
	"os"
	"path/filepath"
)

// EnterWorkDir は現在のCWDを記録し、workDirが指定されていればその
// ディレクトリへ移る。戻り値の関数を呼ぶと、記録できた場合に元のCWDへ戻る。
//
// CWDはプロセス全体で1つしかないため、呼び出し側は移動から復元までを
// execMuなどで直列化すること。ロックの外で移動・復元すると、実行中の
// 別の実行の作業ディレクトリを壊したり、復元が追い越してCWDが残留したりする。
//
// os.Getwd が失敗しても workDir への移動は試みる。この場合は元の場所が
// 分からないため復元は行わず、その旨を標準エラーへ出す。
// workDir が空、またはすでに現在のCWDと同じなら移動しない。
// 指定された workDir へ移動できなければエラーを返し、誤ったディレクトリで
// 実行を続けない。
func EnterWorkDir(workDir string) (func(), error) {
	originalDir, err := os.Getwd()
	if err != nil {
		fmt.Fprintf(os.Stderr, "元の作業ディレクトリを取得できず、復元できません: %v\n", err)
	}
	if workDir != "" && workDir != originalDir {
		if err := os.Chdir(workDir); err != nil {
			return nil, fmt.Errorf("作業ディレクトリ %q に移動できません: %w", workDir, err)
		}
	}
	return func() {
		if originalDir != "" {
			if err := os.Chdir(originalDir); err != nil {
				fmt.Fprintf(os.Stderr, "作業ディレクトリ %q を復元できません: %v\n", originalDir, err)
			}
		}
	}, nil
}

// NakoRunLocation はエディタからの実行指定（ファイルパス）から、VMへ渡す
// ソース名と作業ディレクトリを決める。
//   - 保存済みファイル: パスを絶対化し、親ディレクトリが実在すればそこを作業ディレクトリにする
//   - 未保存（filePath == ""）: GUI起動時のディレクトリ（initialWorkingDir）で動かす。
//     このパスも絶対化し、実在するディレクトリであることまで確認する
//
// 対象のディレクトリが実在しない場合は workDir を空で返し、呼び出し側で
// 「実行開始時点のCWD」へフォールバックさせる。
// 相対パスは呼び出し時点のプロセスCWDで絶対化し、実行中のCWD変更とは同期しない。
func NakoRunLocation(filePath, initialWorkingDir string) (runFile, workDir string) {
	absOrClean := func(path string) string {
		if absolute, err := filepath.Abs(path); err == nil {
			return absolute
		}
		return filepath.Clean(path)
	}
	if filePath == "" {
		runFile = "gui.nako3"
		if initialWorkingDir != "" {
			workDir = absOrClean(initialWorkingDir)
		}
	} else {
		runFile = absOrClean(filePath)
		workDir = filepath.Dir(runFile)
	}
	if workDir != "" {
		if stat, err := os.Stat(workDir); err != nil || !stat.IsDir() {
			workDir = ""
		}
	}
	return runFile, workDir
}
