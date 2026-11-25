package graph

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"strconv"
	"strings"

	"github.com/mio-cc/anywhere-door/gen"
)

// Hash 是图文档的规范化哈希："sha256:<hex>"（C.02.01 §6）。文档不能序列化时返回 ""。
func Hash(d *gen.Graph) string {
	b, err := Canonical(d)
	if err != nil {
		return ""
	}
	return HashBytes(b)
}

// HashBytes 是一段规范化 JSON 的哈希："sha256:<hex>"。
func HashBytes(canonical []byte) string {
	sum := sha256.Sum256(canonical)
	return "sha256:" + hex.EncodeToString(sum[:])
}

// Canonical 是规范化序列化（C.02.01 §6、E.02.03 §5，P.09 K93）：对象的键按 Unicode 码点排序；字符串按 JSON 标准转义、
// 不做 HTML 转义；数字按 JavaScript JSON.stringify 的写法；没有多余空白。前端（W.05 的 ops/hash.ts）必须逐字节相同，
// 以 schema/examples/hash_vectors.json 为准。
func Canonical(v any) ([]byte, error) {
	var buf bytes.Buffer
	enc := json.NewEncoder(&buf)
	enc.SetEscapeHTML(false)
	if err := enc.Encode(v); err != nil {
		return nil, fmt.Errorf("graph: 规范化序列化：%w", err)
	}
	dec := json.NewDecoder(&buf)
	dec.UseNumber()
	var x any
	if err := dec.Decode(&x); err != nil {
		return nil, fmt.Errorf("graph: 规范化序列化：%w", err)
	}
	var out bytes.Buffer
	if err := writeValue(&out, x); err != nil {
		return nil, err
	}
	return out.Bytes(), nil
}

func writeValue(b *bytes.Buffer, v any) error {
	switch x := v.(type) {
	case nil:
		b.WriteString("null")
	case bool:
		b.WriteString(strconv.FormatBool(x))
	case json.Number:
		s, err := jsNumber(string(x))
		if err != nil {
			return err
		}
		b.WriteString(s)
	case string:
		writeString(b, x)
	case []any:
		b.WriteByte('[')
		for i, e := range x {
			if i > 0 {
				b.WriteByte(',')
			}
			if err := writeValue(b, e); err != nil {
				return err
			}
		}
		b.WriteByte(']')
	case map[string]any:
		keys := make([]string, 0, len(x))
		for k := range x {
			keys = append(keys, k)
		}
		sort.Strings(keys) // 按 UTF-8 字节比较，等于按码点比较
		b.WriteByte('{')
		for i, k := range keys {
			if i > 0 {
				b.WriteByte(',')
			}
			writeString(b, k)
			b.WriteByte(':')
			if err := writeValue(b, x[k]); err != nil {
				return err
			}
		}
		b.WriteByte('}')
	default:
		return fmt.Errorf("graph: 规范化序列化：不认识的值 %T", v)
	}
	return nil
}

// writeString 按 JSON.stringify 的写法转义：" \ 和控制字符转义（\b \f \n \r \t 用短写法，其余 \u00xx），其他字符原样写出。
func writeString(b *bytes.Buffer, s string) {
	b.WriteByte('"')
	for _, r := range s {
		switch r {
		case '"':
			b.WriteString(`\"`)
		case '\\':
			b.WriteString(`\\`)
		case '\b':
			b.WriteString(`\b`)
		case '\f':
			b.WriteString(`\f`)
		case '\n':
			b.WriteString(`\n`)
		case '\r':
			b.WriteString(`\r`)
		case '\t':
			b.WriteString(`\t`)
		default:
			if r < 0x20 {
				fmt.Fprintf(b, `\u%04x`, r)
			} else {
				b.WriteRune(r)
			}
		}
	}
	b.WriteByte('"')
}

var errNumber = errors.New("graph: 规范化序列化：数字超出范围")

// jsNumber 按 ECMAScript Number::toString 写数字：先转成 float64，再取最短表示；整数不带小数点和指数，
// 指数部分写成 e+21、e-7。-0 写成 0。
func jsNumber(s string) (string, error) {
	f, err := strconv.ParseFloat(s, 64)
	if err != nil {
		return "", fmt.Errorf("%w：%s", errNumber, s)
	}
	if f == 0 {
		return "0", nil
	}
	neg := f < 0
	if neg {
		f = -f
	}
	mant, exp, _ := strings.Cut(strconv.FormatFloat(f, 'e', -1, 64), "e")
	digits := strings.Replace(mant, ".", "", 1)
	e, _ := strconv.Atoi(exp)
	k, n := len(digits), e+1
	var out string
	switch {
	case k <= n && n <= 21:
		out = digits + strings.Repeat("0", n-k)
	case 0 < n && n <= 21:
		out = digits[:n] + "." + digits[n:]
	case -6 < n && n <= 0:
		out = "0." + strings.Repeat("0", -n) + digits
	default:
		m := digits[:1]
		if k > 1 {
			m += "." + digits[1:]
		}
		sign, x := "+", n-1
		if x < 0 {
			sign, x = "-", -x
		}
		out = m + "e" + sign + strconv.Itoa(x)
	}
	if neg {
		out = "-" + out
	}
	return out, nil
}
