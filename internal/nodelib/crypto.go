package nodelib

import (
	"crypto/md5"
	crand "crypto/rand"
	"crypto/sha1"
	"crypto/sha256"
	"crypto/sha512"
	"encoding/hex"
	"fmt"
	"hash"
	mrand "math/rand/v2"
	"strings"

	"github.com/kujirahand/nadesiko3go/internal/stdlib"
	"github.com/kujirahand/nadesiko3go/internal/value"
)

func cryptoCommands(m map[string]command) {
	m["ハッシュ関数一覧取得"] = command{fn: func(_ stdlib.Context, _ []value.Value) (value.Value, error) {
		algos := []string{"md5", "sha1", "sha256", "sha384", "sha512"}
		items := make([]value.Value, len(algos))
		for i, a := range algos {
			items[i] = value.String(a)
		}
		return value.ArrayValue(value.NewArray(items...)), nil
	}}

	m["ハッシュ値計算"] = command{
		josi: [][]string{{"を", "の"}, {"で", "の"}},
		fn: func(_ stdlib.Context, a []value.Value) (value.Value, error) {
			text := str(a, 0)
			algo := strings.ToLower(strings.TrimSpace(str(a, 1)))
			if algo == "" {
				algo = "sha256"
			}

			var h hash.Hash
			switch algo {
			case "md5":
				h = md5.New()
			case "sha1":
				h = sha1.New()
			case "sha256":
				h = sha256.New()
			case "sha384":
				h = sha512.New384()
			case "sha512":
				h = sha512.New()
			default:
				return value.Undefined(), fmt.Errorf("未対応のハッシュ関数『%s』です。", algo)
			}
			h.Write([]byte(text))
			return value.String(hex.EncodeToString(h.Sum(nil))), nil
		},
	}

	m["ランダムUUID生成"] = command{fn: func(_ stdlib.Context, _ []value.Value) (value.Value, error) {
		var uuid [16]byte
		if _, err := crand.Read(uuid[:]); err != nil {
			return value.Undefined(), err
		}
		// UUID v4 format
		uuid[6] = (uuid[6] & 0x0f) | 0x40
		uuid[8] = (uuid[8] & 0x3f) | 0x80
		s := fmt.Sprintf("%08x-%04x-%04x-%04x-%012x",
			uuid[0:4], uuid[4:6], uuid[6:8], uuid[8:10], uuid[10:16])
		return value.String(s), nil
	}}

	m["ランダム配列生成"] = command{
		josi: [][]string{{"の", "を"}},
		fn: func(_ stdlib.Context, a []value.Value) (value.Value, error) {
			n := int(value.ToNumber(argAt(a, 0)))
			if n <= 0 {
				return value.ArrayValue(value.NewArray()), nil
			}
			perm := mrand.Perm(n)
			items := make([]value.Value, n)
			for i, v := range perm {
				items[i] = value.Number(float64(v + 1))
			}
			return value.ArrayValue(value.NewArray(items...)), nil
		},
	}
}
