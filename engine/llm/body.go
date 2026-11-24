package llm

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"path"
	"strconv"
	"strings"
)

// BodyOp 的取值（E.10.01 §5.2）。
const (
	OpSet    = "set"
	OpDelete = "delete"
	OpRename = "rename"
)

// normalize 把协议编码出的请求体变成只有 map[string]any、[]any、json.Number 等基本类型的值，覆盖规则才能按路径走。
func normalize(body map[string]any) (any, error) {
	b, err := json.Marshal(body)
	if err != nil {
		return nil, err
	}
	return decodeJSON(b)
}

func decodeJSON(b []byte) (any, error) {
	dec := json.NewDecoder(bytes.NewReader(b))
	dec.UseNumber()
	var v any
	if err := dec.Decode(&v); err != nil {
		return nil, err
	}
	return v, nil
}

// marshalBody 序列化请求体（不转义 <、>、&）。
func marshalBody(v any) ([]byte, error) {
	var buf bytes.Buffer
	enc := json.NewEncoder(&buf)
	enc.SetEscapeHTML(false)
	if err := enc.Encode(v); err != nil {
		return nil, err
	}
	return bytes.TrimRight(buf.Bytes(), "\n"), nil
}

// applyOverrides 按顺序执行提供方的请求体覆盖（E.10.01 §5.2）；upstream 用来过滤 Models。
func applyOverrides(body any, ops []BodyOp, upstream string) (any, error) {
	for i, op := range ops {
		if !opApplies(op, upstream) {
			continue
		}
		var err error
		body, err = applyOp(body, op)
		if err != nil {
			return nil, fmt.Errorf("第 %d 条请求体覆盖（%s %s）：%w", i+1, op.Op, op.Path, err)
		}
	}
	return body, nil
}

func opApplies(op BodyOp, upstream string) bool {
	if len(op.Models) == 0 {
		return true
	}
	for _, p := range op.Models {
		if ok, _ := path.Match(p, upstream); ok {
			return true
		}
	}
	return false
}

func applyOp(body any, op BodyOp) (any, error) {
	segs := strings.Split(op.Path, ".")
	switch op.Op {
	case OpSet:
		v, err := decodeJSON(op.Value)
		if err != nil {
			return nil, fmt.Errorf("value 不是合法的 JSON：%w", err)
		}
		return setAt(body, segs, v)
	case OpDelete:
		return deleteAt(body, segs)
	case OpRename:
		v, ok, err := getAt(body, segs)
		if err != nil || !ok {
			return body, err
		}
		body, err = deleteAt(body, segs)
		if err != nil {
			return nil, err
		}
		return setAt(body, strings.Split(op.To, "."), v)
	}
	return nil, fmt.Errorf("不认识的 op %q", op.Op)
}

var errNotContainer = errors.New("路径穿过的值不是对象或数组")

func index(seg string, n int) (int, error) {
	i, err := strconv.Atoi(seg)
	if err != nil || i < 0 || i >= n {
		return 0, fmt.Errorf("数组下标 %q 无效（数组长度 %d）", seg, n)
	}
	return i, nil
}

// setAt 设置路径上的值，路径上缺的对象自动建；返回新的根。
func setAt(cur any, segs []string, v any) (any, error) {
	if len(segs) == 0 {
		return v, nil
	}
	switch c := cur.(type) {
	case nil:
		return setAt(map[string]any{}, segs, v)
	case map[string]any:
		nv, err := setAt(c[segs[0]], segs[1:], v)
		if err != nil {
			return nil, err
		}
		c[segs[0]] = nv
		return c, nil
	case []any:
		i, err := index(segs[0], len(c))
		if err != nil {
			return nil, err
		}
		nv, err := setAt(c[i], segs[1:], v)
		if err != nil {
			return nil, err
		}
		c[i] = nv
		return c, nil
	}
	return nil, errNotContainer
}

// getAt 取路径上的值；ok 为假表示路径不存在。
func getAt(cur any, segs []string) (any, bool, error) {
	for _, s := range segs {
		switch c := cur.(type) {
		case map[string]any:
			v, ok := c[s]
			if !ok {
				return nil, false, nil
			}
			cur = v
		case []any:
			i, err := strconv.Atoi(s)
			if err != nil || i < 0 || i >= len(c) {
				return nil, false, nil
			}
			cur = c[i]
		case nil:
			return nil, false, nil
		default:
			return nil, false, errNotContainer
		}
	}
	return cur, true, nil
}

// deleteAt 删除路径上的值，返回新的根；路径不存在时什么也不做。
func deleteAt(cur any, segs []string) (any, error) {
	s, rest := segs[0], segs[1:]
	switch c := cur.(type) {
	case nil:
		return nil, nil
	case map[string]any:
		if len(rest) == 0 {
			delete(c, s)
			return c, nil
		}
		child, ok := c[s]
		if !ok {
			return c, nil
		}
		nv, err := deleteAt(child, rest)
		if err != nil {
			return nil, err
		}
		c[s] = nv
		return c, nil
	case []any:
		i, err := strconv.Atoi(s)
		if err != nil || i < 0 || i >= len(c) {
			return c, nil
		}
		if len(rest) == 0 {
			return append(c[:i:i], c[i+1:]...), nil
		}
		nv, err := deleteAt(c[i], rest)
		if err != nil {
			return nil, err
		}
		c[i] = nv
		return c, nil
	}
	return nil, errNotContainer
}
