package main

import (
	"fmt"
	"os"
	"path/filepath"
)

// Output 是一个生成物：相对仓库根目录的路径（用 /）和内容。
type Output struct {
	Path string
	Data []byte
}

// 生成物的位置（C.14 §2）。
const (
	outGoTypes   = "gen/types.go"
	outGoUnion   = "gen/union.go"
	outGoConsts  = "gen/consts.go"
	outGoTables  = "gen/tables.go"
	outTSTypes   = "web/src/lib/protocol/generated/types.ts"
	outTSConsts  = "web/src/lib/protocol/generated/consts.ts"
	outTSTables  = "web/src/lib/protocol/generated/tables.ts"
	outTSWire    = "web/src/lib/protocol/generated/wire.ts"
	outTSMessage = "web/src/lib/i18n/generated/messages.ts"
)

// Generate 读 root 下的 schema/，返回全部生成物（顺序固定）。
func Generate(root string) ([]Output, error) {
	schemaDir := filepath.Join(root, "schema")
	builtin := filepath.Join(schemaDir, "builtin")
	outs, m, err := generateCore(schemaDir, filepath.Join(builtin, "aliases.yaml"), filepath.Join(builtin, "sse_control.yaml"))
	if err != nil {
		return nil, err
	}
	goTables, tsTables, err := emitTables(m, builtin)
	if err != nil {
		return nil, err
	}
	msgs, err := emitMessages(m, filepath.Join(builtin, "messages"))
	if err != nil {
		return nil, err
	}
	outs = append(outs,
		Output{outGoTables, goTables},
		Output{outTSTables, tsTables},
		Output{outTSMessage, msgs},
	)
	return outs, nil
}

// generateCore 生成不依赖内置数据表的部分：类型、联合、常量、字段映射表。黄金文件测试（C14-T3）也用它。
// ssePath 不存在时不生成 SseControl。
func generateCore(schemaDir, aliasesPath, ssePath string) ([]Output, *model, error) {
	set, err := loadSchemas(schemaDir)
	if err != nil {
		return nil, nil, err
	}
	al, err := readAliases(aliasesPath)
	if err != nil {
		return nil, nil, err
	}
	var sse []string
	if _, err := os.Stat(ssePath); err == nil {
		if sse, err = readStringList(ssePath); err != nil {
			return nil, nil, err
		}
	}
	m, err := buildModel(set, al.Roots)
	if err != nil {
		return nil, nil, err
	}
	cs, err := collectConsts(m, al.Nodes, sse)
	if err != nil {
		return nil, nil, err
	}
	goTypes, goUnion, err := emitGo(m)
	if err != nil {
		return nil, nil, err
	}
	goConsts, err := emitGoConsts(m, cs)
	if err != nil {
		return nil, nil, err
	}
	tsTypes, err := emitTS(m, cs)
	if err != nil {
		return nil, nil, err
	}
	tsConsts := emitTSConsts(cs)
	wire, err := emitWire(m)
	if err != nil {
		return nil, nil, err
	}
	return []Output{
		{outGoTypes, goTypes},
		{outGoUnion, goUnion},
		{outGoConsts, goConsts},
		{outTSTypes, tsTypes},
		{outTSConsts, tsConsts},
		{outTSWire, wire},
	}, m, nil
}

// genErrors 收集生成过程中的问题，一次报出全部（C.14 §3：报出位置并失败，不猜）。
type genErrors []string

func (e *genErrors) add(format string, args ...any) { *e = append(*e, fmt.Sprintf(format, args...)) }

func (e genErrors) err() error {
	if len(e) == 0 {
		return nil
	}
	msg := fmt.Sprintf("%d 个问题：", len(e))
	for _, s := range e {
		msg += "\n  " + s
	}
	return fmt.Errorf("%s", msg)
}
