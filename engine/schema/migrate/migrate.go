// Package migrate 把旧版本的图文档逐级升级到当前格式（C.12-R4、E.03.01）。只处理文档本身；
// 升级结果作为一个 migrate Op 写入由 E.05.01 负责（C.12-R2）。
package migrate

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"strings"
)

// Current 是本程序写出的图文档格式版本。
const Current = "1.0"

// Func 升级一级：输入是旧版本的文档，返回新版本的文档（可以原地修改后返回）。
type Func func(doc map[string]any) (map[string]any, error)

// Step 是迁移链的一级。
type Step struct {
	From, To string
	Fn       Func
}

// Chain 是内置迁移链；v1 的格式从 1.0 开始，现在为空。
var Chain []Step

// ErrTooNew：图文档的版本比本程序认识的都新（C.12-R3）。
var ErrTooNew = errors.New("这张图由更新版本的 Anywhere Door 创建，请升级")

// ErrNoVersion：图文档没有 schema_version，或者写法不对。
var ErrNoVersion = errors.New("migrate: 图文档没有可用的 schema_version")

type version struct{ major, minor int }

func parse(s string) (version, bool) {
	ma, mi, ok := strings.Cut(s, ".")
	if !ok {
		return version{}, false
	}
	a, err1 := strconv.Atoi(ma)
	b, err2 := strconv.Atoi(mi)
	if err1 != nil || err2 != nil || a < 0 || b < 0 {
		return version{}, false
	}
	return version{a, b}, true
}

func (v version) less(o version) bool {
	return v.major < o.major || (v.major == o.major && v.minor < o.minor)
}

// Apply 按 schema_version 逐级升级到链上最后能到的版本（E.03.01 §5 第 7 条）。from 是输入的版本。
// 没有可用的步骤时原样返回 raw；高于 Current 和链上所有版本时返回 ErrTooNew。raw 不会被修改。
func Apply(raw []byte, chain []Step) (out []byte, from string, err error) {
	dec := json.NewDecoder(bytes.NewReader(raw))
	dec.UseNumber()
	var doc map[string]any
	if err := dec.Decode(&doc); err != nil {
		return nil, "", fmt.Errorf("migrate: 图文档不是 JSON 对象：%w", err)
	}
	from, _ = doc["schema_version"].(string)
	cur, ok := parse(from)
	if !ok {
		return nil, from, ErrNoVersion
	}
	newest, _ := parse(Current)
	for _, s := range chain {
		if v, ok := parse(s.To); ok && newest.less(v) {
			newest = v
		}
	}
	if newest.less(cur) {
		return nil, from, ErrTooNew
	}
	ver, changed := from, false
	for steps := 0; steps <= len(chain); steps++ {
		var step *Step
		for i := range chain {
			if chain[i].From == ver {
				step = &chain[i]
				break
			}
		}
		if step == nil {
			break
		}
		next, err := step.Fn(doc)
		if err != nil {
			return nil, from, fmt.Errorf("migrate: %s → %s 失败：%w", step.From, step.To, err)
		}
		if next == nil {
			return nil, from, fmt.Errorf("migrate: %s → %s 返回了空文档", step.From, step.To)
		}
		doc, ver, changed = next, step.To, true
		doc["schema_version"] = ver
	}
	if !changed {
		return raw, from, nil
	}
	out, err = json.Marshal(doc)
	if err != nil {
		return nil, from, fmt.Errorf("migrate: 序列化失败：%w", err)
	}
	return out, from, nil
}
