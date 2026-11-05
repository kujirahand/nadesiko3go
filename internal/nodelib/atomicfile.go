package nodelib

import (
	"crypto/rand"
	"errors"
	"fmt"
	"os"
	"path/filepath"
)

// writeFileAtomic は data を path へ書き込む。os.WriteFile と違い、
// 既存の通常ファイルがある場合は実体の隣に一時ファイルを作って書き込み、
// 書き込みと Sync が成功した場合だけ rename で置き換える。
//
// os.WriteFile は既存ファイルを開くときに先に切り詰めるため、容量不足
// などで書き込みが途中で失敗すると、既存の内容が失われた状態で残る。
// 『圧縮』で #330 → #424 と段階的に対処したのと同じ構造の問題が保存系の
// 命令にもあるため、こちらも同じ方式に揃える (#425)。
//
// 種別ごとの振る舞いは #424 で確定した方針に倣う。
//   - 通常ファイルと symlink: symlink は辿り、実体の隣で一時ファイルを作って
//     rename で実体を置き換える（リンクは維持される）。失敗時は既存の実体が
//     無傷で、一時ファイルは残らない。
//   - 特殊ファイル（デバイス・FIFO・ソケット等）とディレクトリ: 従来どおり
//     os.WriteFile で直接書く。削除はしない。
//   - 壊れた symlink: 辿る先が無いため、リンク位置に通常ファイルができる
//     （現行はリンク先を新規作成する。#424 計画で許容された軽微な変更）。
//   - rename が使えない場合（保存先フォルダへ書けない、バインドマウント等）
//     は、直接書き込みへは戻さずエラーにする。既存ファイルは無傷。
//
// rename で置き換えるため、所有者・xattr・ハードリンクは引き継がれない
// （実体のモードだけを引き継ぐ）。梱包（bundle）や整形の atomicWriteFile と
// 同じ制限で、#425 の第2部に記録した許容事項である。
func writeFileAtomic(path string, data []byte, perm os.FileMode) error {
	info, err := os.Stat(path)
	switch {
	case err == nil:
		if !info.Mode().IsRegular() {
			// ディレクトリや特殊ファイルを rename で置き換えない。
			// /dev/full や FIFO などは従来どおり直接書き込む。
			return os.WriteFile(path, data, perm)
		}
		// rename はフォルダの権限だけで成功してしまうため、この確認が
		// 無いと現行はエラーになる読み取り専用ファイルを上書きしてしまう。
		// 切り詰めはせず、開いて閉じるだけで書き込み可否を確認する。
		check, err := os.OpenFile(path, os.O_WRONLY, 0)
		if err != nil {
			return err
		}
		if err := check.Close(); err != nil {
			return err
		}
		return replaceFileAtomic(path, data, info.Mode().Perm(), true)
	case errors.Is(err, os.ErrNotExist):
		// 存在しないパスと壊れた symlink はここに来る。resolveExisting は
		// 壊れた symlink ではリンク位置そのものを返すため、rename で
		// リンク位置に通常ファイルができる。
		return replaceFileAtomic(path, data, perm, false)
	default:
		return err
	}
}

// replaceFileAtomic は、path の実体の隣に一時ファイルを作って data を
// 書き込み、成功した場合だけ rename で実体を置き換える。
// mode は一時ファイルの作成権限であり、既存ファイルの場合は書き込み後に
// Chmod で同じ値へ戻し、作成時の umask の影響でモードが変わらないように
// する。新規ファイルでは Chmod せず、作成権限と umask の結果が現行の
// os.WriteFile と一致する。
func replaceFileAtomic(path string, data []byte, mode os.FileMode, exists bool) (err error) {
	final := resolveExisting(path)
	tmp, err := createAtomicTemp(filepath.Dir(final), mode)
	if err != nil {
		// 一時ファイル名を利用者へ見せないよう、保存先のパスで返す。
		return retargetPathError(err, path)
	}
	tmpPath := tmp.Name()
	committed := false
	defer func() {
		// Windows では開いたまま削除できないため、閉じてから消す。
		_ = tmp.Close()
		if committed {
			return
		}
		// 失敗時に一時ファイルを残さない。削除も失敗したら、元のエラーと
		// 合わせて利用者へ伝える（「残さない」が要件のため捨てない）。
		if removeErr := os.Remove(tmpPath); removeErr != nil && !errors.Is(removeErr, os.ErrNotExist) {
			err = errors.Join(err, fmt.Errorf("一時ファイル『%s』を削除できません: %w", tmpPath, removeErr))
		}
	}()

	if _, err := tmp.Write(data); err != nil {
		return retargetPathError(err, final)
	}
	if exists {
		if err := tmp.Chmod(mode); err != nil {
			return retargetPathError(err, final)
		}
	}
	if err := tmp.Sync(); err != nil {
		return retargetPathError(err, final)
	}
	if err := tmp.Close(); err != nil {
		return retargetPathError(err, final)
	}
	if err := os.Rename(tmpPath, final); err != nil {
		return fmt.Errorf("ファイル『%s』を置き換えられません: %w", path, err)
	}
	committed = true
	return nil
}

// createAtomicTemp は、bundle の createOutputTemp と同じ方式で、
// 指定した権限と umask を反映した一時ファイルを排他的に作る。
// os.CreateTemp は 0600 固定のため、新規成果物の権限が現行より
// 狭くなる後退を避けてこちらを使う (#424)。
// 『圧縮』(#424) が一時出力へ移るときも、この土台を共有する想定である。
func createAtomicTemp(dir string, mode os.FileMode) (*os.File, error) {
	for range 10 {
		name := filepath.Join(dir, ".gonako-save-"+rand.Text())
		out, err := os.OpenFile(name, os.O_CREATE|os.O_EXCL|os.O_WRONLY, mode)
		if os.IsExist(err) {
			continue
		}
		return out, err
	}
	return nil, errors.New("一時ファイル名が重複しました")
}

// retargetPathError は、一時ファイルに対する操作のエラーが示すパスを
// display へ付け替える。利用者の知らない一時ファイル名をエラー文面に
// 出さないためで、*os.PathError の形と errors.Is の判定は元のまま保つ。
func retargetPathError(err error, display string) error {
	var pe *os.PathError
	if errors.As(err, &pe) {
		retargeted := *pe
		retargeted.Path = display
		return &retargeted
	}
	return err
}
