package config

import (
	"bytes"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"sort"
	"strings"

	"github.com/goccy/go-yaml"
	"github.com/goccy/go-yaml/ast"
	"github.com/goccy/go-yaml/parser"

	"github.com/mio-cc/anywhere-door/internal/platform/datadir"
)

// Write 按语法树修改配置文件（保留注释和未知项），原子写；任何一个改动不合法时什么也不写。
func Write(d datadir.Dir, changes map[string]any) error {
	norm := make(map[string]any, len(changes))
	for p, v := range changes {
		it, ok := byPath[p]
		switch {
		case ok && (it.Hot == ReadOnly || p == "data.dir"):
			return fmt.Errorf("config: %s 不能写进配置文件", p)
		case ok:
			nv, err := coerce(it, v, "", false)
			if err != nil {
				return fmt.Errorf("config: %s：%v", p, err)
			}
			norm[p] = nv
		case strings.HasPrefix(p, "packages.") && len(strings.Split(p, ".")) >= 3:
			norm[p] = v
		default:
			return fmt.Errorf("config: 不认识的配置项 %s", p)
		}
	}
	src, err := os.ReadFile(d.Config())
	if err != nil && !errors.Is(err, fs.ErrNotExist) {
		return fmt.Errorf("config: 读不了 %s：%w", fileName, err)
	}
	text := string(src)
	paths := make([]string, 0, len(norm))
	for p := range norm {
		paths = append(paths, p)
	}
	sort.Strings(paths)
	for _, p := range paths {
		if text, err = setPath(text, strings.Split(p, "."), norm[p]); err != nil {
			return err
		}
	}
	if text != "" && !strings.HasSuffix(text, "\n") {
		text += "\n"
	}
	if err := datadir.WritePrivate(d.Config(), []byte(text)); err != nil {
		return fmt.Errorf("config: 写 %s：%w", fileName, err)
	}
	return nil
}

func pathOf(segs []string) *yaml.Path {
	b := (&yaml.PathBuilder{}).Root()
	for _, s := range segs {
		b = b.Child(s)
	}
	return b.Build()
}

// find 取 segs 对应的节点；没有时为 nil。
func find(f *ast.File, segs []string) ast.Node {
	if f == nil {
		return nil
	}
	n, err := pathOf(segs).FilterFile(f)
	if err != nil {
		return nil
	}
	return n
}

func nest(segs []string, v any) any {
	for i := len(segs) - 1; i >= 0; i-- {
		v = map[string]any{segs[i]: v}
	}
	return v
}

// setPath 把 segs 设为 v：已有的键替换值；没有的键补在最近的上级映射下面（上级也没有时追加在文件末尾）。
func setPath(text string, segs []string, v any) (string, error) {
	f, err := parser.ParseBytes([]byte(text), parser.ParseComments)
	if err != nil {
		return "", syntaxErr(err)
	}
	k := len(segs)
	for ; k > 0; k-- {
		if find(f, segs[:k]) != nil {
			break
		}
	}
	if k == len(segs) {
		b, err := yaml.Marshal(v)
		if err != nil {
			return "", fmt.Errorf("config: 编码 %s：%w", strings.Join(segs, "."), err)
		}
		if err := pathOf(segs).ReplaceWithReader(f, bytes.NewReader(b)); err != nil {
			return "", fmt.Errorf("config: 改 %s：%w", strings.Join(segs, "."), err)
		}
		return f.String(), nil
	}
	b, err := yaml.Marshal(nest(segs[k:], v))
	if err != nil {
		return "", fmt.Errorf("config: 编码 %s：%w", strings.Join(segs, "."), err)
	}
	if k == 0 {
		if text != "" && !strings.HasSuffix(text, "\n") {
			text += "\n"
		}
		return text + string(b), nil
	}
	parent := find(f, segs[:k])
	if t := parent.Type(); t != ast.MappingType && t != ast.MappingValueType {
		return "", fmt.Errorf("config: %s 不是映射，不能在下面加 %s", strings.Join(segs[:k], "."), segs[k])
	}
	if err := pathOf(segs[:k]).MergeFromReader(f, bytes.NewReader(b)); err != nil {
		return "", fmt.Errorf("config: 加 %s：%w", strings.Join(segs, "."), err)
	}
	return f.String(), nil
}
