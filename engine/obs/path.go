package obs

import (
	"strconv"
	"strings"

	"github.com/mio-cc/anywhere-door/gen"
)

// PathItem 是运行路径的一层（C.06.05 §4 的 chain）：从外到内，最后一项是节点本身，前面的是容器。
type PathItem struct {
	NodeID    string
	Label     string
	Iter      gen.Iter
	SameLabel bool // 同一层有重名节点（C.06.05-R1）
}

var pathRepl = strings.NewReplacer("/", "／", "#", "＃", "@", "＠", "~", "～")

func num(p *int64) int64 {
	if p == nil {
		return 0
	}
	return *p
}

// PathOf 是运行路径（C.06.05 §2）：根图/<容器>#r<轮>#e<事件>#<份>/…/<节点>#<份>@<重试>。只用于显示和按前缀查询（R4）。
func PathOf(chain []PathItem) string {
	var b strings.Builder
	b.WriteString("根图")
	for i, it := range chain {
		b.WriteByte('/')
		b.WriteString(pathRepl.Replace(it.Label))
		if it.SameLabel {
			id := it.NodeID
			b.WriteString("~" + id[max(0, len(id)-4):])
		}
		if r := num(it.Iter.Round); r > 0 {
			b.WriteString("#r" + strconv.FormatInt(r, 10))
		}
		if e := num(it.Iter.Event); e > 0 {
			b.WriteString("#e" + strconv.FormatInt(e, 10))
		}
		if f := num(it.Iter.Fan); f > 0 {
			b.WriteString("#" + strconv.FormatInt(f, 10))
		}
		if a := num(it.Iter.Attempt); i == len(chain)-1 && a > 1 {
			b.WriteString("@" + strconv.FormatInt(a, 10))
		}
	}
	return b.String()
}

// iterKey 是迭代号的写法（C.06.05-R5）：r、e、f、a 按这个顺序，用 . 连接；容器不写 attempt。
func iterKey(it *gen.Iter, self bool) string {
	var parts []string
	if it != nil {
		if r := num(it.Round); r > 0 {
			parts = append(parts, "r"+strconv.FormatInt(r, 10))
		}
		if e := num(it.Event); e > 0 {
			parts = append(parts, "e"+strconv.FormatInt(e, 10))
		}
		if f := num(it.Fan); f > 0 {
			parts = append(parts, "f"+strconv.FormatInt(f, 10))
		}
	}
	if self {
		a := int64(1)
		if it != nil && num(it.Attempt) > 0 {
			a = num(it.Attempt)
		}
		parts = append(parts, "a"+strconv.FormatInt(a, 10))
	}
	return strings.Join(parts, ".")
}

// ExecKey 是"一次节点执行"的唯一键（C.06.05-R5）：<node_id>|<容器 id>:<迭代>|…|<自身迭代>。
func ExecKey(loc gen.Loc) string {
	var b strings.Builder
	b.WriteString(string(loc.NodeID))
	if loc.Scopes != nil {
		for _, s := range *loc.Scopes {
			b.WriteString("|" + string(s.NodeID) + ":" + iterKey(s.Iter, false))
		}
	}
	b.WriteString("|" + iterKey(loc.Iter, true))
	return b.String()
}
