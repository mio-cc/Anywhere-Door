// STUB(G.03): depcheck 的桩。O.01 的 gen / lint / check 要调用它，所以先放一个只打印提示、以 0 退出的桩；G.03 实施时整体替换（G.02.06-R10）。

package main

import "fmt"

func main() {
	fmt.Println("depcheck：依赖方向检查还没有实现，什么也没有检查（桩，G.03）")
}
