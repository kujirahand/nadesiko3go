package nodelib

import (
	"os"
	"path/filepath"
	"testing"
)

// TestN01ResolveExistingNormalizesSlashSeparatedPath は N01 の回帰テスト。
//
// なでしこプログラムは「/」区切りでパスを書くことが多い。resolveExisting は
// os.PathSeparator でコンポーネントに分割するため、Windows では区切りを
// 正規化しないと「C:/work/alias/new」が1コンポーネント扱いになり、未作成の
// 末尾で EvalSymlinks が失敗して symlink 祖先を解決しないまま cleaned path を
// 返す。ここでは「/」区切りの絶対パスを渡しても、実在する symlink 祖先が
// 解決され、未作成の末尾が実体パスへ連結されることを固定する。
//
// 注意: Windows 固有の分岐（ドライブ、ドライブを持たない rooted path）は
// Unix では実行できない。Unix では filepath.FromSlash が no-op なので、
// このテストは修正前後で同じ結果になる。固定するのは「区切り正規化の後に
// コンポーネント分割が行われる」という契約までで、Windows の不具合そのものを
// 再現するものではない。
func TestN01ResolveExistingNormalizesSlashSeparatedPath(t *testing.T) {
	dir := t.TempDir()
	inner := filepath.Join(dir, "src", "inner")
	if err := os.MkdirAll(inner, 0o755); err != nil {
		t.Fatal(err)
	}
	alias := filepath.Join(dir, "alias")
	if err := os.Symlink(inner, alias); err != nil {
		t.Skipf("シンボリックリンクを作れません: %v", err)
	}

	// 末尾の new は未作成。symlink 祖先 alias は src/inner を指す。
	// macOS の /var → /private/var のように作業フォルダ自体の祖先も
	// symlink のことがあるため、期待値も実体解決して求める。
	realInner, err := filepath.EvalSymlinks(inner)
	if err != nil {
		t.Fatal(err)
	}
	dest := filepath.ToSlash(filepath.Join(alias, "new"))
	got := resolveExisting(dest)
	want := filepath.Join(realInner, "new")
	if got != want {
		t.Errorf("resolveExisting(%q) = %q, want %q", dest, got, want)
	}
}
