// Package sqlitelib implements the commands from nadesiko3-sqlite3 for the
// command-line runtime. It uses a pure Go SQLite driver so gonako remains a
// single, cross-compilable binary without CGO.
package sqlitelib

import (
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/kujirahand/nadesiko3go/internal/lexer"
	"github.com/kujirahand/nadesiko3go/internal/stdlib"
	"github.com/kujirahand/nadesiko3go/internal/value"
	_ "modernc.org/sqlite"
)

const errOpenDB = "SQLITE3の命令を使う前に『SQLITE3開く』でデータベースを開いてください。"

// Plugin owns the database handles opened by one gonako runtime.
// Handles cross the value boundary as numbers; *sql.DB never does.
type Plugin struct {
	dbs    map[int64]*sql.DB
	active int64
	next   int64
}

func New() *Plugin { return &Plugin{dbs: map[int64]*sql.DB{}} }

type command struct {
	josi       [][]string
	async      bool
	returnNone bool
	fn         stdlib.Impl
}

func (p *Plugin) FuncList() lexer.FuncList {
	list := lexer.FuncList{
		"SQLITE3今挿入ID": {Name: "SQLITE3今挿入ID", Type: "const", Value: "?"},
	}
	for name, cmd := range p.commands() {
		list[name] = &lexer.FuncItem{
			Name: name, Type: "func", Josi: cmd.josi, Pure: false,
			AsyncFn: cmd.async, ReturnNone: cmd.returnNone,
		}
	}
	return list
}

func (p *Plugin) Impls() map[string]stdlib.Impl {
	out := map[string]stdlib.Impl{}
	for name, cmd := range p.commands() {
		out[name] = cmd.fn
	}
	return out
}

func (p *Plugin) commands() map[string]command {
	return map[string]command{
		"SQLITE3開": {
			josi: [][]string{{"を", "の"}}, fn: p.open,
		},
		"SQLITE3閉": {
			returnNone: true, fn: p.close,
		},
		"SQLITE3切替": {
			josi: [][]string{{"に", "へ"}}, returnNone: true, fn: p.switchDB,
		},
		"SQLITE3実行": {
			josi: [][]string{{"を"}, {"で"}}, async: true, returnNone: true, fn: p.exec,
		},
		"SQLITE3取得": {
			josi: [][]string{{"を"}, {"で"}}, async: true, fn: p.get,
		},
		"SQLITE3全取得": {
			josi: [][]string{{"を"}, {"で"}}, async: true, fn: p.all,
		},
		"SQLITE3実行時": {
			josi: [][]string{{"に"}, {"を"}, {"で"}}, returnNone: true, fn: p.execCallback,
		},
		"SQLITE3実行後": {
			josi: [][]string{{"に"}, {"を"}, {"で"}}, returnNone: true, fn: p.execCallback,
		},
		"SQLITE3取得時": {
			josi: [][]string{{"に"}, {"を"}, {"で"}}, returnNone: true, fn: p.allCallback,
		},
	}
}

func arg(args []value.Value, i int) value.Value {
	if i < 0 || i >= len(args) {
		return value.Undefined()
	}
	return args[i]
}

func (p *Plugin) open(_ stdlib.Context, args []value.Value) (value.Value, error) {
	filename := value.ToString(arg(args, 0))
	db, err := sql.Open("sqlite", filename)
	if err != nil {
		return value.Undefined(), err
	}
	// A single connection keeps :memory: databases and connection-local state
	// stable, matching better-sqlite3's one-connection model.
	db.SetMaxOpenConns(1)
	db.SetMaxIdleConns(1)
	if err := db.Ping(); err != nil {
		db.Close()
		return value.Undefined(), err
	}
	p.next++
	p.dbs[p.next] = db
	p.active = p.next
	return value.Number(float64(p.next)), nil
}

func (p *Plugin) close(_ stdlib.Context, _ []value.Value) (value.Value, error) {
	db, err := p.activeDB()
	if err != nil {
		return value.Undefined(), err
	}
	id := p.active
	p.active = 0
	delete(p.dbs, id)
	return value.Undefined(), db.Close()
}

func (p *Plugin) switchDB(_ stdlib.Context, args []value.Value) (value.Value, error) {
	id := int64(value.ToNumber(arg(args, 0)))
	if id <= 0 || p.dbs[id] == nil {
		return value.Undefined(), errors.New("SQLITE3切替で指定されたデータベースは開かれていません。")
	}
	p.active = id
	return value.Undefined(), nil
}

func (p *Plugin) exec(ctx stdlib.Context, args []value.Value) (value.Value, error) {
	_, err := p.execSQL(ctx, value.ToString(arg(args, 0)), arg(args, 1))
	return value.Undefined(), err
}

func (p *Plugin) execSQL(ctx stdlib.Context, query string, params value.Value) (sql.Result, error) {
	db, err := p.activeDB()
	if err != nil {
		return nil, err
	}
	bound, err := bindParams(params)
	if err != nil {
		return nil, err
	}
	result, err := db.Exec(query, bound...)
	if err != nil {
		return nil, err
	}
	if id, idErr := result.LastInsertId(); idErr == nil && id != 0 {
		ctx.SetSysVar("SQLITE3今挿入ID", value.Number(float64(id)))
	}
	return result, nil
}

func (p *Plugin) get(_ stdlib.Context, args []value.Value) (value.Value, error) {
	rows, err := p.query(value.ToString(arg(args, 0)), arg(args, 1), true)
	if err != nil {
		return value.Undefined(), err
	}
	if len(rows) == 0 {
		return value.Undefined(), nil
	}
	return rows[0], nil
}

func (p *Plugin) all(_ stdlib.Context, args []value.Value) (value.Value, error) {
	rows, err := p.query(value.ToString(arg(args, 0)), arg(args, 1), false)
	if err != nil {
		return value.Undefined(), err
	}
	return value.ArrayValue(value.NewArray(rows...)), nil
}

func (p *Plugin) execCallback(ctx stdlib.Context, args []value.Value) (value.Value, error) {
	if _, err := p.execSQL(ctx, value.ToString(arg(args, 1)), arg(args, 2)); err != nil {
		return value.Undefined(), fmt.Errorf("SQLITE3実行時のエラー『%s』%w", value.ToString(arg(args, 1)), err)
	}
	return post(ctx, arg(args, 0), nil)
}

func (p *Plugin) allCallback(ctx stdlib.Context, args []value.Value) (value.Value, error) {
	rows, err := p.query(value.ToString(arg(args, 1)), arg(args, 2), false)
	if err != nil {
		return value.Undefined(), err
	}
	result := value.ArrayValue(value.NewArray(rows...))
	return post(ctx, arg(args, 0), []value.Value{result})
}

func post(ctx stdlib.Context, callable value.Value, args []value.Value) (value.Value, error) {
	fn, ok := callable.Func()
	if !ok && callable.Kind() == value.KindString {
		fn = ctx.FindFunc(value.ToString(callable))
		ok = fn != nil
	}
	if !ok {
		return value.Undefined(), errors.New("コールバックに指定できるのは関数だけです。")
	}
	return value.Undefined(), ctx.PostFunc(fn, args)
}

func (p *Plugin) activeDB() (*sql.DB, error) {
	if p.active == 0 || p.dbs[p.active] == nil {
		return nil, errors.New(errOpenDB)
	}
	return p.dbs[p.active], nil
}

func (p *Plugin) query(query string, params value.Value, one bool) ([]value.Value, error) {
	db, err := p.activeDB()
	if err != nil {
		return nil, err
	}
	bound, err := bindParams(params)
	if err != nil {
		return nil, err
	}
	rows, err := db.Query(query, bound...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	columns, err := rows.Columns()
	if err != nil {
		return nil, err
	}
	out := []value.Value{}
	for rows.Next() {
		raw := make([]any, len(columns))
		dest := make([]any, len(columns))
		for i := range raw {
			dest[i] = &raw[i]
		}
		if err := rows.Scan(dest...); err != nil {
			return nil, err
		}
		dict := value.NewDict()
		for i, name := range columns {
			dict.Set(name, fromSQL(raw[i]))
		}
		out = append(out, value.DictValue(dict))
		if one {
			break
		}
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return out, nil
}

func bindParams(params value.Value) ([]any, error) {
	switch params.Kind() {
	case value.KindUndefined, value.KindNull:
		return nil, nil
	case value.KindArray:
		array, _ := params.Array()
		out := make([]any, array.Len())
		for i, item := range array.Values() {
			v, err := toSQL(item)
			if err != nil {
				return nil, err
			}
			out[i] = v
		}
		return out, nil
	case value.KindDict:
		dict, _ := params.Dict()
		out := make([]any, 0, dict.Len())
		for _, key := range dict.Keys() {
			item, _ := dict.Get(key)
			v, err := toSQL(item)
			if err != nil {
				return nil, err
			}
			out = append(out, sql.Named(strings.TrimLeft(key, "@$:"), v))
		}
		return out, nil
	default:
		v, err := toSQL(params)
		if err != nil {
			return nil, err
		}
		return []any{v}, nil
	}
}

func toSQL(v value.Value) (any, error) {
	switch v.Kind() {
	case value.KindUndefined, value.KindNull:
		return nil, nil
	case value.KindBool:
		b, _ := v.Bool()
		if b {
			return int64(1), nil
		}
		return int64(0), nil
	case value.KindNumber:
		n, _ := v.Number()
		return n, nil
	case value.KindString:
		s, _ := v.String()
		return s, nil
	default:
		return nil, fmt.Errorf("SQLiteのパラメータに%sは指定できません。", value.ToString(v))
	}
}

func fromSQL(v any) value.Value {
	switch x := v.(type) {
	case nil:
		return value.Null()
	case bool:
		return value.Bool(x)
	case int64:
		return value.Number(float64(x))
	case float64:
		return value.Number(x)
	case string:
		return value.String(x)
	case []byte:
		return value.String(string(x))
	case time.Time:
		return value.String(x.Format("2006-01-02 15:04:05"))
	default:
		return value.String(fmt.Sprint(x))
	}
}
