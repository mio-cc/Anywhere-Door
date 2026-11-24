package main

import (
	"math"
	"reflect"
	"strings"
	"testing"
)

// 样例：engine/op 两段（各 3、1 条语句，被两个测试二进制各记一次，只有一次执行过第一段）；
// engine/store 一段 4 条执行过、一段 6 条没执行；gen/ 和 _test.go 的不算。
const sample = `mode: atomic
example.com/x/engine/op/apply.go:10.2,12.3 3 0
example.com/x/engine/op/apply.go:14.2,15.3 1 0
example.com/x/engine/op/apply.go:10.2,12.3 3 5
example.com/x/engine/op/apply.go:14.2,15.3 1 0
example.com/x/engine/store/store.go:1.1,2.2 4 1
example.com/x/engine/store/store.go:3.1,4.2 6 0
example.com/x/gen/types.go:1.1,2.2 100 0
example.com/x/engine/op/apply_test.go:1.1,2.2 9 0
`

func TestCover(t *testing.T) {
	t.Run("O02-T4 cover", func(t *testing.T) {
		pkgs, err := Parse(strings.NewReader(sample), "example.com/x")
		if err != nil {
			t.Fatal(err)
		}
		want := []Pkg{{"example.com/x/engine/op", 3, 4}, {"example.com/x/engine/store", 4, 10}}
		if !reflect.DeepEqual(pkgs, want) {
			t.Fatalf("按包汇总：%+v，应当是 %+v（同一段去重、gen/ 和 _test.go 排除）", pkgs, want)
		}
		if math.Abs(pkgs[0].Percent()-75) > 1e-9 || math.Abs(pkgs[1].Percent()-40) > 1e-9 {
			t.Errorf("百分比：%v %v", pkgs[0].Percent(), pkgs[1].Percent())
		}
		if bad := Check(pkgs, nil); bad != nil {
			t.Errorf("没有门槛时不失败：%v", bad)
		}
		rules, err := ReadRules("# 门槛\nexample.com/x/engine/op 80\nexample.com/x/engine/... 30%\n")
		if err != nil {
			t.Fatal(err)
		}
		bad := Check(pkgs, rules)
		if len(bad) != 1 || !strings.Contains(bad[0], "engine/op") || !strings.Contains(bad[0], "75.0%") || !strings.Contains(bad[0], "80%") {
			t.Errorf("engine/op 低于 80%% 应当失败、engine/store 按第二条 30%% 通过：%v", bad)
		}
		if _, err := ReadRules("engine/op eighty\n"); err == nil {
			t.Error("门槛文件格式不对应当报错")
		}
		if _, err := Parse(strings.NewReader("mode: set\nbroken line\n"), ""); err == nil {
			t.Error("cover.out 格式不对应当报错")
		}
	})
}
