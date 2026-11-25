package nodelib

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/pbkdf2"
	crand "crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"fmt"
	"hash/crc32"
	"math"
	"strings"

	"github.com/kujirahand/nadesiko3go/internal/stdlib"
	"github.com/kujirahand/nadesiko3go/internal/value"
)

const (
	cryptSaltLen   = 16
	cryptNonceLen  = 12
	cryptPBKDFIter = 100000
)

// bytesValue はバイト列を「0〜255の数値の配列」に変換する。
func bytesValue(data []byte) value.Value {
	items := make([]value.Value, len(data))
	for i, b := range data {
		items[i] = value.Number(float64(b))
	}
	return value.ArrayValue(value.NewArray(items...))
}

// anyBytes は文字列(UTF-8)またはバイト配列(数値配列)をバイト列にする。
func anyBytes(v value.Value) ([]byte, error) {
	return fileData(v)
}

// packItem は書式を分解した1要素。
type packItem struct {
	kind  byte
	count int
}

// parsePackFormat は Python struct のサブセットの書式を解析する。
func parsePackFormat(format string) (bigEndian bool, items []packItem, err error) {
	rs := []byte(format)
	i := 0
	if i < len(rs) {
		switch rs[i] {
		case '<', '=':
			i++
		case '>':
			bigEndian = true
			i++
		}
	}
	for i < len(rs) {
		c := rs[i]
		if c == ' ' || c == '\t' {
			i++
			continue
		}
		count, hasCount := 0, false
		for i < len(rs) && rs[i] >= '0' && rs[i] <= '9' {
			count = count*10 + int(rs[i]-'0')
			hasCount = true
			if count > 1<<24 {
				return false, nil, errors.New("書式の繰り返し数が大きすぎます。")
			}
			i++
		}
		if i >= len(rs) {
			return false, nil, fmt.Errorf("書式『%s』の末尾に型文字がありません。", format)
		}
		k := rs[i]
		i++
		switch k {
		case 'B', 'H', 'I', 'Q', 'b', 'h', 'i', 'q', 's':
		default:
			return false, nil, fmt.Errorf("書式『%s』に不正な文字『%c』があります。", format, k)
		}
		if !hasCount {
			count = 1
		}
		items = append(items, packItem{kind: k, count: count})
	}
	return bigEndian, items, nil
}

// packInfo は整数型のバイト幅と符号を返す。
func packInfo(k byte) (size int, signed bool) {
	switch k {
	case 'B':
		return 1, false
	case 'b':
		return 1, true
	case 'H':
		return 2, false
	case 'h':
		return 2, true
	case 'I':
		return 4, false
	case 'i':
		return 4, true
	case 'Q':
		return 8, false
	default: // 'q'
		return 8, true
	}
}

func packValues(format string, vals []value.Value) ([]byte, error) {
	big, items, err := parsePackFormat(format)
	if err != nil {
		return nil, err
	}
	var out []byte
	vi := 0
	for _, it := range items {
		if it.kind == 's' {
			if vi >= len(vals) {
				return nil, errors.New("書式に対して値の数が足りません。")
			}
			data, err := anyBytes(vals[vi])
			if err != nil {
				return nil, err
			}
			vi++
			buf := make([]byte, it.count)
			copy(buf, data)
			out = append(out, buf...)
			continue
		}
		size, signed := packInfo(it.kind)
		for n := 0; n < it.count; n++ {
			if vi >= len(vals) {
				return nil, errors.New("書式に対して値の数が足りません。")
			}
			f, ok := vals[vi].Number()
			if !ok || math.IsNaN(f) || math.IsInf(f, 0) || math.Trunc(f) != f {
				return nil, fmt.Errorf("%d番目の値は整数で指定してください。", vi+1)
			}
			vi++
			if size == 8 && (f > 9007199254740992 || f < -9007199254740992) {
				// なでしこの数値は float64 のため、2^53 を超える64ビット整数は
				// 正確に表せない。静かに丸めずエラーにする。
				return nil, fmt.Errorf("%d番目の値%vは64ビット整数として正確に表せる範囲(±2^53)を超えています。", vi, f)
			}
			var u uint64
			if signed {
				bits := uint(size*8 - 1)
				lo, hi := -math.Ldexp(1, int(bits)), math.Ldexp(1, int(bits))
				if f < lo || f >= hi {
					return nil, fmt.Errorf("%d番目の値%vは型『%c』の範囲外です。", vi, f, it.kind)
				}
				u = uint64(int64(f))
			} else {
				if f < 0 || f >= math.Ldexp(1, size*8) {
					return nil, fmt.Errorf("%d番目の値%vは型『%c』の範囲外です。", vi, f, it.kind)
				}
				u = uint64(f)
			}
			for b := 0; b < size; b++ {
				shift := uint(b * 8)
				if big {
					shift = uint((size - 1 - b) * 8)
				}
				out = append(out, byte(u>>shift))
			}
		}
	}
	if vi != len(vals) {
		return nil, errors.New("書式に対して値の数が多すぎます。")
	}
	if out == nil {
		out = []byte{}
	}
	return out, nil
}

func unpackValues(format string, data []byte) ([]value.Value, error) {
	big, items, err := parsePackFormat(format)
	if err != nil {
		return nil, err
	}
	var out []value.Value
	pos := 0
	for _, it := range items {
		if it.kind == 's' {
			if pos+it.count > len(data) {
				return nil, errors.New("データが書式に対して不足しています。")
			}
			out = append(out, bytesValue(data[pos:pos+it.count]))
			pos += it.count
			continue
		}
		size, signed := packInfo(it.kind)
		for n := 0; n < it.count; n++ {
			if pos+size > len(data) {
				return nil, errors.New("データが書式に対して不足しています。")
			}
			var u uint64
			for b := 0; b < size; b++ {
				shift := uint(b * 8)
				if big {
					shift = uint((size - 1 - b) * 8)
				}
				u |= uint64(data[pos+b]) << shift
			}
			pos += size
			if signed {
				// 符号拡張
				s := int64(u<<uint(64-size*8)) >> uint(64-size*8)
				if size == 8 && (s > 9007199254740992 || s < -9007199254740992) {
					return nil, errors.New("64ビット整数が正確に表せる範囲(±2^53)を超えています。")
				}
				out = append(out, value.Number(float64(s)))
			} else {
				if size == 8 && u > 9007199254740992 {
					return nil, errors.New("64ビット整数が正確に表せる範囲(±2^53)を超えています。")
				}
				out = append(out, value.Number(float64(u)))
			}
		}
	}
	return out, nil
}

func deriveKey(password string, salt []byte) ([]byte, error) {
	return pbkdf2.Key(sha256.New, password, salt, cryptPBKDFIter, 32)
}

func encryptData(plain []byte, password string) (string, error) {
	salt := make([]byte, cryptSaltLen)
	nonce := make([]byte, cryptNonceLen)
	if _, err := crand.Read(salt); err != nil {
		return "", err
	}
	if _, err := crand.Read(nonce); err != nil {
		return "", err
	}
	key, err := deriveKey(password, salt)
	if err != nil {
		return "", err
	}
	block, err := aes.NewCipher(key)
	if err != nil {
		return "", err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", err
	}
	buf := append(append([]byte{}, salt...), nonce...)
	buf = gcm.Seal(buf, nonce, plain, nil)
	return base64.StdEncoding.EncodeToString(buf), nil
}

func decryptData(b64, password string) ([]byte, error) {
	raw, err := base64.StdEncoding.DecodeString(strings.TrimSpace(b64))
	if err != nil {
		return nil, errors.New("復号化できません。BASE64として不正なデータです。")
	}
	if len(raw) < cryptSaltLen+cryptNonceLen+16 {
		return nil, errors.New("復号化できません。データが短すぎます。")
	}
	salt := raw[:cryptSaltLen]
	nonce := raw[cryptSaltLen : cryptSaltLen+cryptNonceLen]
	key, err := deriveKey(password, salt)
	if err != nil {
		return nil, err
	}
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}
	plain, err := gcm.Open(nil, nonce, raw[cryptSaltLen+cryptNonceLen:], nil)
	if err != nil {
		return nil, errors.New("復号化できません。パスワードが違うか、データが改ざんされています。")
	}
	return plain, nil
}

func binaryCommands(m map[string]command) {
	m["CRC32"] = command{ // @文字列またはバイト配列AのCRC32(IEEE)を符号なし整数で返す // @CRC32
		josi: [][]string{{"の", "を"}},
		fn: func(_ stdlib.Context, a []value.Value) (value.Value, error) {
			data, err := anyBytes(argAt(a, 0))
			if err != nil {
				return value.Undefined(), err
			}
			return value.Number(float64(crc32.ChecksumIEEE(data))), nil
		},
	}

	m["バイナリ変換"] = command{ // @書式Aで値の配列Bをバイト配列にパックして返す(例:『<I』で[1]をバイナリ変換。『=』はリトルエンディアン扱い。Q/qは±2^53まで) // @ばいなりへんかん
		josi: [][]string{{"で"}, {"を"}},
		fn: func(_ stdlib.Context, a []value.Value) (value.Value, error) {
			arr, ok := argAt(a, 1).Array()
			if !ok {
				return value.Undefined(), errors.New("バイナリ変換には値の配列を指定してください。")
			}
			data, err := packValues(str(a, 0), arr.Values())
			if err != nil {
				return value.Undefined(), err
			}
			return bytesValue(data), nil
		},
	}

	m["バイナリ取得"] = command{ // @書式Aでバイト配列Bをアンパックして値の配列を返す(データが余ってもエラーにしない。ヘッダーだけ読む用途のため) // @ばいなりしゅとく
		josi: [][]string{{"で"}, {"を"}},
		fn: func(_ stdlib.Context, a []value.Value) (value.Value, error) {
			data, err := anyBytes(argAt(a, 1))
			if err != nil {
				return value.Undefined(), err
			}
			vals, err := unpackValues(str(a, 0), data)
			if err != nil {
				return value.Undefined(), err
			}
			return value.ArrayValue(value.NewArray(vals...)), nil
		},
	}

	m["暗号化"] = command{ // @平文(文字列またはバイト配列)AをパスワードBでAES-256-GCM暗号化しBASE64文字列で返す // @あんごうか
		josi: [][]string{{"を"}, {"で"}},
		fn: func(_ stdlib.Context, a []value.Value) (value.Value, error) {
			data, err := anyBytes(argAt(a, 0))
			if err != nil {
				return value.Undefined(), err
			}
			s, err := encryptData(data, str(a, 1))
			if err != nil {
				return value.Undefined(), err
			}
			return value.String(s), nil
		},
	}

	m["復号化"] = command{ // @暗号化されたBASE64文字列AをパスワードBで復号して文字列で返す // @ふくごうか
		josi: [][]string{{"を"}, {"で"}},
		fn: func(_ stdlib.Context, a []value.Value) (value.Value, error) {
			plain, err := decryptData(str(a, 0), str(a, 1))
			if err != nil {
				return value.Undefined(), err
			}
			return value.String(string(plain)), nil
		},
	}
}
