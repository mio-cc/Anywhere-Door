package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/goccy/go-yaml"
)

// assignRules 是有提示文案的兼容规则号（C.04.02 §2、G.16）。
var assignRules = []string{"C1", "C10", "C2", "C4", "C6", "C8"}

func readMessageFile(path string) (map[string]string, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var m map[string]string
	if err := yaml.Unmarshal(b, &m); err != nil {
		return nil, fmt.Errorf("%s：%w", filepath.Base(path), err)
	}
	return m, nil
}

func sameKeys(got map[string]string, want []string) (missing, extra []string) {
	w := map[string]bool{}
	for _, k := range want {
		w[k] = true
		if _, ok := got[k]; !ok {
			missing = append(missing, k)
		}
	}
	for _, k := range sortedKeys(got) {
		if !w[k] {
			extra = append(extra, k)
		}
	}
	return missing, extra
}

// emitMessages 是 i18n/generated/messages.ts：ERRORS、ASSIGN（W.10 §4.5、C.14 §5.3）。
func emitMessages(m *model, dir string) ([]byte, error) {
	var errs genErrors
	errorsMsg, err := readMessageFile(filepath.Join(dir, "errors.zh.yaml"))
	if err != nil {
		return nil, err
	}
	assign, err := readMessageFile(filepath.Join(dir, "assign.zh.yaml"))
	if err != nil {
		return nil, err
	}
	var codes []string
	if n, ok := m.types["ApiErrorCode"]; ok {
		codes = n.enum
	} else {
		errs.add("schema 里没有 ApiErrorCode")
	}
	if miss, extra := sameKeys(errorsMsg, codes); len(miss)+len(extra) > 0 {
		errs.add("errors.zh.yaml 要恰好覆盖 ApiErrorCode：缺 %v，多 %v（G.16）", miss, extra)
	}
	if miss, extra := sameKeys(assign, assignRules); len(miss)+len(extra) > 0 {
		errs.add("assign.zh.yaml 要恰好是 C1、C2、C4、C6、C8、C10：缺 %v，多 %v（G.16）", miss, extra)
	}
	if err := errs.err(); err != nil {
		return nil, err
	}
	var w strings.Builder
	w.WriteString(tsHeader)
	w.WriteString("// 来源：schema/builtin/messages/errors.zh.yaml、assign.zh.yaml（W.10 §4.5）。\n\n")
	write := func(name, doc string, msgs map[string]string) {
		fmt.Fprintf(&w, "/** %s */\nexport const %s: Readonly<Record<string, string>> = {\n", doc, name)
		for _, k := range sortedKeys(msgs) {
			fmt.Fprintf(&w, "  %s: %s,\n", tsString(k), tsString(msgs[k]))
		}
		w.WriteString("};\n")
	}
	write("ERRORS", "错误码 → 文案（C.11 §3）", errorsMsg)
	w.WriteString("\n")
	write("ASSIGN", "兼容规则号 → 提示（C.04.02 §5）", assign)
	return []byte(w.String()), nil
}
