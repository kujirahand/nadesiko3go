package stdlib

import (
	"errors"
	"fmt"
	"math"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/kujirahand/nadesiko3go/internal/value"
)

type era struct{ name, start string }

var eras = []era{
	{"令和", "2019/05/01"}, {"平成", "1989/01/08"}, {"昭和", "1926/12/25"},
	{"大正", "1912/07/30"}, {"明治", "1868/10/23"},
}

func datetimeImpls(m map[string]Impl) {
	m["今"] = func(ctx Context, _ []value.Value) (value.Value, error) {
		return value.String(ctx.Now().Format("15:04:05")), nil
	}
	m["今日"] = dayOffset(0)
	m["明日"] = dayOffset(1)
	m["昨日"] = dayOffset(-1)
	m["今年"] = yearOffset(0)
	m["来年"] = yearOffset(1)
	m["去年"] = yearOffset(-1)
	m["今月"] = monthOffset(0)
	m["来月"] = monthOffset(1)
	m["先月"] = monthOffset(-1)
	m["システム時間"] = func(ctx Context, _ []value.Value) (value.Value, error) {
		return value.Number(float64(ctx.Now().Unix())), nil
	}
	m["システム時間ミリ秒"] = func(ctx Context, _ []value.Value) (value.Value, error) {
		return value.Number(float64(ctx.Now().UnixMilli())), nil
	}
	m["曜日"] = func(ctx Context, a []value.Value) (value.Value, error) {
		t, err := parseDate(str(a, 0), ctx.Now().Location())
		if err != nil {
			t = ctx.Now()
		}
		weekdays := []rune("日月火水木金土")
		return value.String(string(weekdays[int(t.Weekday())])), nil
	}
	m["曜日番号取得"] = func(ctx Context, a []value.Value) (value.Value, error) {
		t, err := parseDate(str(a, 0), ctx.Now().Location())
		if err != nil {
			t = ctx.Now()
		}
		return value.Number(float64(t.Weekday())), nil
	}
	m["UNIXTIME変換"] = unixTime
	m["UNIX時間変換"] = unixTime
	m["日時変換"] = func(ctx Context, a []value.Value) (value.Value, error) {
		t := time.Unix(int64(value.ToNumber(arg(a, 0))), 0).In(ctx.Now().Location())
		return value.String(t.Format("2006/01/02 15:04:05")), nil
	}
	m["和暦変換"] = wareki
	m["年数差"] = dateDiff("年")
	m["月数差"] = dateDiff("月")
	m["日数差"] = dateDiff("日")
	m["時間差"] = dateDiff("時間")
	m["分差"] = dateDiff("分")
	m["秒差"] = dateDiff("秒")
	m["日時差"] = func(ctx Context, a []value.Value) (value.Value, error) {
		return dateDiff(str(a, 2))(ctx, a[:2])
	}
	m["時間加算"] = addTime
	m["日付加算"] = addDate
	m["日時加算"] = addDateTime
}

func dayOffset(days int) Impl {
	return func(ctx Context, _ []value.Value) (value.Value, error) {
		return value.String(ctx.Now().AddDate(0, 0, days).Format("2006/01/02")), nil
	}
}
func yearOffset(years int) Impl {
	return func(ctx Context, _ []value.Value) (value.Value, error) {
		return value.Number(float64(ctx.Now().Year() + years)), nil
	}
}
func monthOffset(months int) Impl {
	return func(ctx Context, _ []value.Value) (value.Value, error) {
		m := (int(ctx.Now().Month())-1+months+12)%12 + 1
		return value.Number(float64(m)), nil
	}
}

func unixTime(ctx Context, a []value.Value) (value.Value, error) {
	t, err := parseDate(str(a, 0), ctx.Now().Location())
	if err != nil {
		return value.Undefined(), err
	}
	return value.Number(float64(t.Unix())), nil
}

func parseDate(s string, loc *time.Location) (time.Time, error) {
	s = strings.TrimSpace(s)
	for _, layout := range []string{"2006/01/02 15:04:05", "2006/01/02", "15:04:05", "15:04"} {
		if t, err := time.ParseInLocation(layout, s, loc); err == nil {
			return t, nil
		}
	}
	return time.Time{}, fmt.Errorf("日時の形式が不正です: %s", s)
}

func wareki(ctx Context, a []value.Value) (value.Value, error) {
	t, err := parseDate(str(a, 0), ctx.Now().Location())
	if err != nil {
		return value.Undefined(), err
	}
	for _, e := range eras {
		start, _ := parseDate(e.start, ctx.Now().Location())
		if !t.Before(start) {
			year := strconv.Itoa(t.Year() - start.Year() + 1)
			if year == "1" {
				year = "元"
			}
			return value.String(fmt.Sprintf("%s%s年%02d月%02d日", e.name, year, t.Month(), t.Day())), nil
		}
	}
	return value.Undefined(), errors.New("『和暦変換』は明治以前の日付には対応していません。")
}

func dateDiff(unit string) Impl {
	return func(ctx Context, a []value.Value) (value.Value, error) {
		left, err := parseDate(str(a, 0), ctx.Now().Location())
		if err != nil {
			return value.Undefined(), err
		}
		right, err := parseDate(str(a, 1), ctx.Now().Location())
		if err != nil {
			return value.Undefined(), err
		}
		var n float64
		switch unit {
		case "年":
			n = float64(right.Year() - left.Year())
		case "月":
			n = float64(right.Year()*12 + int(right.Month()) - left.Year()*12 - int(left.Month()))
		case "日":
			n = math.Ceil(right.Sub(left).Hours() / 24)
		case "時間":
			n = math.Ceil(right.Sub(left).Hours())
		case "分":
			n = math.Ceil(right.Sub(left).Minutes())
		case "秒":
			n = math.Ceil(right.Sub(left).Seconds())
		default:
			return value.Undefined(), errors.New("『日時差』で不明な単位です。")
		}
		return value.Number(n), nil
	}
}

func addTime(ctx Context, a []value.Value) (value.Value, error) {
	source, delta := str(a, 0), str(a, 1)
	negative := strings.HasPrefix(delta, "-")
	delta = strings.TrimLeft(delta, "+-")
	parts := strings.Split(delta+":0:0", ":")
	h, _ := strconv.Atoi(parts[0])
	min, _ := strconv.Atoi(parts[1])
	sec, _ := strconv.Atoi(parts[2])
	d := time.Duration(h)*time.Hour + time.Duration(min)*time.Minute + time.Duration(sec)*time.Second
	if negative {
		d = -d
	}
	t, err := parseDate(source, ctx.Now().Location())
	if err != nil {
		return value.Undefined(), err
	}
	return value.String(formatLike(t.Add(d), source)), nil
}

func addDate(ctx Context, a []value.Value) (value.Value, error) {
	source, delta := str(a, 0), str(a, 1)
	sign := 1
	if strings.HasPrefix(delta, "-") {
		sign = -1
	}
	delta = strings.TrimLeft(delta, "+-")
	parts := strings.Split(delta+"/0/0", "/")
	y, _ := strconv.Atoi(parts[0])
	mo, _ := strconv.Atoi(parts[1])
	d, _ := strconv.Atoi(parts[2])
	t, err := parseDate(source, ctx.Now().Location())
	if err != nil {
		return value.Undefined(), err
	}
	return value.String(formatLike(t.AddDate(sign*y, sign*mo, sign*d), source)), nil
}

var dateTimeDeltaRE = regexp.MustCompile(`^([+-]?)(\d+)(年|ヶ月|日|週間|時間|分|秒)$`)

func addDateTime(ctx Context, a []value.Value) (value.Value, error) {
	m := dateTimeDeltaRE.FindStringSubmatch(str(a, 1))
	if m == nil {
		return value.Undefined(), errors.New("『日付加算』は『(+｜-)1(年|ヶ月|日|時間|分|秒)』の書式で指定します。")
	}
	n, _ := strconv.Atoi(m[2])
	if m[1] == "-" {
		n = -n
	}
	source := str(a, 0)
	t, err := parseDate(source, ctx.Now().Location())
	if err != nil {
		return value.Undefined(), err
	}
	switch m[3] {
	case "年":
		t = t.AddDate(n, 0, 0)
	case "ヶ月":
		t = t.AddDate(0, n, 0)
	case "日":
		t = t.AddDate(0, 0, n)
	case "週間":
		t = t.AddDate(0, 0, n*7)
	case "時間":
		t = t.Add(time.Duration(n) * time.Hour)
	case "分":
		t = t.Add(time.Duration(n) * time.Minute)
	case "秒":
		t = t.Add(time.Duration(n) * time.Second)
	}
	return value.String(formatLike(t, source)), nil
}

func formatLike(t time.Time, source string) string {
	if strings.Contains(source, ":") && strings.Contains(source, "/") {
		return t.Format("2006/01/02 15:04:05")
	}
	if strings.Contains(source, ":") {
		return t.Format("15:04:05")
	}
	return t.Format("2006/01/02")
}
