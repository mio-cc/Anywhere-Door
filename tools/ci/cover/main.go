// Command cover 按包汇总 Go 覆盖率（设计文档 O.02 §4.3、Q.01-R5）：
//
//	go run ./tools/ci/cover [-min <门槛文件>] cover.out
//
// cover.out 来自 go test ./... -coverpkg=./... -coverprofile=cover.out。同一段代码会被多个测试二进制各记一次，
// 按"文件:位置"去重，任何一次执行过就算覆盖。排除 gen/ 和 _test.go（Q.01-R5）。
// 没给门槛文件时只打印报告、退出码 0（M0 不设门槛，P.09 K243）。门槛文件每行"<包路径或前缀/...> <百分比>"，
// # 开头为注释，按顺序第一条匹配的生效（K246）。只用标准库。
package main

import (
	"bufio"
	"flag"
	"fmt"
	"io"
	"os"
	"path"
	"sort"
	"strconv"
	"strings"
)

// Pkg 是一个包的覆盖情况。
type Pkg struct {
	Path           string
	Covered, Total int
}

// Percent 是语句覆盖率（没有语句时算 100）。
func (p Pkg) Percent() float64 {
	if p.Total == 0 {
		return 100
	}
	return float64(p.Covered) * 100 / float64(p.Total)
}

type block struct {
	stmts   int
	covered bool
}

// Parse 读 cover.out，按包汇总；结果按包路径排序。
func Parse(r io.Reader, modulePath string) ([]Pkg, error) {
	blocks := map[string]*block{}
	sc := bufio.NewScanner(r)
	sc.Buffer(make([]byte, 1<<20), 1<<20)
	line := 0
	for sc.Scan() {
		line++
		t := strings.TrimSpace(sc.Text())
		if t == "" || strings.HasPrefix(t, "mode:") {
			continue
		}
		// <文件>:<起>.<列>,<止>.<列> <语句数> <次数>
		f := strings.Fields(t)
		if len(f) != 3 {
			return nil, fmt.Errorf("cover.out 第 %d 行格式不对：%q", line, t)
		}
		stmts, err1 := strconv.Atoi(f[1])
		count, err2 := strconv.Atoi(f[2])
		if err1 != nil || err2 != nil {
			return nil, fmt.Errorf("cover.out 第 %d 行格式不对：%q", line, t)
		}
		b := blocks[f[0]]
		if b == nil {
			b = &block{stmts: stmts}
			blocks[f[0]] = b
		}
		b.covered = b.covered || count > 0
	}
	if err := sc.Err(); err != nil {
		return nil, err
	}
	pkgs := map[string]*Pkg{}
	for pos, b := range blocks {
		file := pos[:strings.LastIndex(pos, ":")]
		if excluded(file, modulePath) {
			continue
		}
		dir := path.Dir(file)
		p := pkgs[dir]
		if p == nil {
			p = &Pkg{Path: dir}
			pkgs[dir] = p
		}
		p.Total += b.stmts
		if b.covered {
			p.Covered += b.stmts
		}
	}
	out := make([]Pkg, 0, len(pkgs))
	for _, p := range pkgs {
		out = append(out, *p)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Path < out[j].Path })
	return out, nil
}

func excluded(file, modulePath string) bool {
	rel := strings.TrimPrefix(strings.TrimPrefix(file, modulePath), "/")
	return strings.HasSuffix(file, "_test.go") || rel == "gen" || strings.HasPrefix(rel, "gen/")
}

// Rule 是门槛文件的一行。
type Rule struct {
	Pattern string
	Min     float64
}

// ReadRules 读门槛文件。
func ReadRules(text string) ([]Rule, error) {
	var rs []Rule
	for i, l := range strings.Split(text, "\n") {
		l = strings.TrimSpace(l)
		if l == "" || strings.HasPrefix(l, "#") {
			continue
		}
		f := strings.Fields(l)
		if len(f) != 2 {
			return nil, fmt.Errorf("门槛文件第 %d 行应当是\"<包> <百分比>\"：%q", i+1, l)
		}
		v, err := strconv.ParseFloat(strings.TrimSuffix(f[1], "%"), 64)
		if err != nil {
			return nil, fmt.Errorf("门槛文件第 %d 行的百分比不对：%q", i+1, l)
		}
		rs = append(rs, Rule{f[0], v})
	}
	return rs, nil
}

func (r Rule) matches(pkg string) bool {
	if p, ok := strings.CutSuffix(r.Pattern, "/..."); ok {
		return pkg == p || strings.HasPrefix(pkg, p+"/")
	}
	return pkg == r.Pattern
}

// Check 返回低于门槛的包（每个包按第一条匹配的规则）。
func Check(pkgs []Pkg, rules []Rule) []string {
	var bad []string
	for _, p := range pkgs {
		for _, r := range rules {
			if r.matches(p.Path) {
				if p.Percent() < r.Min {
					bad = append(bad, fmt.Sprintf("%s：%.1f%%，门槛 %.0f%%", p.Path, p.Percent(), r.Min))
				}
				break
			}
		}
	}
	return bad
}

func modulePath() string {
	b, err := os.ReadFile("go.mod")
	if err != nil {
		return ""
	}
	for _, l := range strings.Split(string(b), "\n") {
		if m, ok := strings.CutPrefix(strings.TrimSpace(l), "module "); ok {
			return strings.TrimSpace(m)
		}
	}
	return ""
}

func main() {
	minFile := flag.String("min", "", "门槛文件；不给时只打印报告")
	flag.Parse()
	if flag.NArg() != 1 {
		fmt.Fprintln(os.Stderr, "用法：go run ./tools/ci/cover [-min <门槛文件>] cover.out")
		os.Exit(2)
	}
	f, err := os.Open(flag.Arg(0))
	if err != nil {
		fmt.Fprintln(os.Stderr, "错误：", err)
		os.Exit(1)
	}
	pkgs, err := Parse(f, modulePath())
	_ = f.Close()
	if err != nil {
		fmt.Fprintln(os.Stderr, "错误：", err)
		os.Exit(1)
	}
	covered, total := 0, 0
	for _, p := range pkgs {
		fmt.Printf("%-60s %6.1f%%  (%d/%d)\n", p.Path, p.Percent(), p.Covered, p.Total)
		covered, total = covered+p.Covered, total+p.Total
	}
	fmt.Printf("%-60s %6.1f%%  (%d/%d)\n", "合计", Pkg{Covered: covered, Total: total}.Percent(), covered, total)
	if *minFile == "" {
		fmt.Println("cover：没有门槛文件，只出报告（M0，P.09 K243）")
		return
	}
	b, err := os.ReadFile(*minFile)
	if err != nil {
		fmt.Fprintln(os.Stderr, "错误：", err)
		os.Exit(1)
	}
	rules, err := ReadRules(string(b))
	if err != nil {
		fmt.Fprintln(os.Stderr, "错误：", err)
		os.Exit(1)
	}
	if bad := Check(pkgs, rules); len(bad) > 0 {
		for _, l := range bad {
			fmt.Println(l)
		}
		os.Exit(1)
	}
	fmt.Println("cover：全部达到门槛")
}
