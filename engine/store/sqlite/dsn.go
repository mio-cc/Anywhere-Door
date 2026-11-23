package sqlite

import (
	"path/filepath"
	"strings"
)

// dsnFor 是 path 的连接串：WAL、synchronous NORMAL、忙等 5s、外键开（E.07.02 §4）。
// 不用 file: URI（Windows 盘符路径在 URI 里容易出错），驱动会把 ? 后面的 _pragma 拆出来执行。
func dsnFor(path string) string {
	pragmas := []string{"journal_mode(WAL)", "synchronous(NORMAL)", "busy_timeout(5000)", "foreign_keys(ON)"}
	return filepath.ToSlash(path) + "?_pragma=" + strings.Join(pragmas, "&_pragma=")
}
