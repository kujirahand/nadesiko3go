package vm_test

import (
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"github.com/kujirahand/nadesiko3go/internal/vm"
)

func runCUICommands(t *testing.T, code string) (string, error) {
	t.Helper()
	var out strings.Builder
	host := vm.NewCUIHost(&out, strings.NewReader(""), nil)
	err := vm.RunProgram(code, "main.nako3", host)
	return strings.TrimSpace(out.String()), err
}

func nakoString(s string) string { return strconv.Quote(s) }

func TestSQLiteOpenExecGetAllAndLastInsertID(t *testing.T) {
	db := filepath.Join(t.TempDir(), "日本語.sqlite3")
	code := strings.Join([]string{
		"DB=" + nakoString(db),
		"DBをSQLITE3開く",
		`「CREATE TABLE items(id INTEGER PRIMARY KEY, name TEXT, score REAL, note TEXT)」を[]でSQLITE3実行`,
		`「INSERT INTO items(name,score,note) VALUES(?,?,?)」を["クジラ",12.5,NULL]でSQLITE3実行`,
		`SQLITE3今挿入IDを表示`,
		`「INSERT INTO items(name,score,note) VALUES(:name,:score,:note)」を{"name":"イルカ","score":7,"note":"海"}でSQLITE3実行`,
		`R=「SELECT * FROM items WHERE id=?」を[1]でSQLITE3取得`,
		`R["name"]を表示`,
		`R["score"]を表示`,
		`R["note"]を表示`,
		`A=「SELECT name FROM items ORDER BY id」を[]でSQLITE3全取得`,
		`A[0]["name"]を表示`,
		`A[1]["name"]を表示`,
		`N=「SELECT name FROM items WHERE id=?」を[999]でSQLITE3取得`,
		`Nを表示`,
		`SQLITE3閉じる`,
	}, "\n")
	got, err := runCUICommands(t, code)
	if err != nil {
		t.Fatal(err)
	}
	if want := "1\nクジラ\n12.5\nnull\nクジラ\nイルカ\nundefined"; got != want {
		t.Fatalf("log = %q, want %q", got, want)
	}
}

func TestSQLiteSwitchKeepsIndependentHandles(t *testing.T) {
	code := strings.Join([]string{
		`A=「:memory:」をSQLITE3開く`,
		`「CREATE TABLE t(v TEXT)」を[]でSQLITE3実行`,
		`「INSERT INTO t VALUES(?)」を["A"]でSQLITE3実行`,
		`B=「:memory:」をSQLITE3開く`,
		`「CREATE TABLE t(v TEXT)」を[]でSQLITE3実行`,
		`「INSERT INTO t VALUES(?)」を["B"]でSQLITE3実行`,
		`AにSQLITE3切替`,
		`R=「SELECT v FROM t」を[]でSQLITE3取得`,
		`R["v"]を表示`,
		`SQLITE3閉じる`,
		`BにSQLITE3切替`,
		`R=「SELECT v FROM t」を[]でSQLITE3取得`,
		`R["v"]を表示`,
		`SQLITE3閉じる`,
	}, "\n")
	got, err := runCUICommands(t, code)
	if err != nil {
		t.Fatal(err)
	}
	if got != "A\nB" {
		t.Fatalf("log = %q, want %q", got, "A\nB")
	}
}

func TestSQLiteCallbackCommands(t *testing.T) {
	code := strings.Join([]string{
		`「:memory:」をSQLITE3開く`,
		`F=関数()`,
		`「作成」と表示`,
		`ここまで`,
		`Fに「CREATE TABLE t(v TEXT)」を[]でSQLITE3実行時`,
		`「作成予約後」と表示`,
		`H=関数()`,
		`「実行後」と表示`,
		`ここまで`,
		`Hに「CREATE TABLE u(v TEXT)」を[]でSQLITE3実行後`,
		`「INSERT INTO t VALUES(?)」を["値"]でSQLITE3実行`,
		`G=関数(ROWS)`,
		`ROWS[0]["v"]を表示`,
		`ここまで`,
		`Gに「SELECT v FROM t」を[]でSQLITE3取得時`,
		`「取得予約後」と表示`,
		`SQLITE3閉じる`,
	}, "\n")
	got, err := runCUICommands(t, code)
	if err != nil {
		t.Fatal(err)
	}
	if got != "作成予約後\n取得予約後\n作成\n実行後\n値" {
		t.Fatalf("log = %q, want callback execution after main", got)
	}
}

func TestSQLiteRequiresOpenDatabase(t *testing.T) {
	_, err := runCUICommands(t, `「SELECT 1」を[]でSQLITE3全取得`)
	if err == nil || !strings.Contains(err.Error(), "SQLITE3の命令を使う前に『SQLITE3開く』") {
		t.Fatalf("err = %v", err)
	}
}
