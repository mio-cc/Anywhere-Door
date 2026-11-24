// Package datadir 规定数据目录的位置和布局（B.07.05），提供取路径、收紧权限、原子写文件的函数。
// 其他代码不自己拼数据目录下的路径（B.07-R6）。
package datadir

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"
	"time"
)

// EnvDataDir 是指定数据目录的环境变量（优先级在 --data-dir 之后）。
const EnvDataDir = "ANYDOOR_DATA_DIR"

// Dir 是一个已确定的数据目录；Root 是绝对路径。
type Dir struct{ Root string }

// subdirs 是 Open 时创建的子目录。
var subdirs = []string{"blobs", "workspaces", "extensions", "mcp", "skills", "logs", "tmp"}

// Resolve 确定数据目录：flag（--data-dir）> 环境变量 ANYDOOR_DATA_DIR > 开发模式 ./.anydoor-dev > 默认位置。
func Resolve(flag string, dev bool) (string, error) {
	if flag != "" {
		return ExpandPath(flag, "")
	}
	if v := os.Getenv(EnvDataDir); v != "" {
		return ExpandPath(v, "")
	}
	if dev {
		return filepath.Abs(".anydoor-dev")
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("datadir: 找不到用户主目录：%w", err)
	}
	return DefaultDir(runtime.GOOS, os.Getenv, home), nil
}

// DefaultDir 是 goos 上的默认位置（纯函数，三平台都能在任一平台上测）。
func DefaultDir(goos string, getenv func(string) string, home string) string {
	switch goos {
	case "windows":
		base := strings.TrimRight(getenv("APPDATA"), `\/`)
		if base == "" {
			base = strings.TrimRight(home, `\/`) + `\AppData\Roaming`
		}
		return base + `\AnywhereDoor`
	case "darwin":
		return path.Join(home, "Library", "Application Support", "AnywhereDoor")
	}
	if x := getenv("XDG_DATA_HOME"); strings.HasPrefix(x, "/") {
		return path.Join(x, "anydoor")
	}
	return path.Join(home, ".local", "share", "anydoor")
}

var percentVar = regexp.MustCompile(`%([A-Za-z_][A-Za-z0-9_]*)%`)

// ExpandPath 展开 ~、$VAR、${VAR}（Windows 另认 %VAR%），相对路径按 base 转绝对路径（base 为空时按当前目录）。
func ExpandPath(p, base string) (string, error) {
	if p == "~" || strings.HasPrefix(p, "~/") || strings.HasPrefix(p, `~\`) {
		home, err := os.UserHomeDir()
		if err != nil {
			return "", fmt.Errorf("datadir: 找不到用户主目录：%w", err)
		}
		p = home + p[1:]
	}
	if runtime.GOOS == "windows" {
		p = percentVar.ReplaceAllStringFunc(p, func(m string) string {
			if v := os.Getenv(m[1 : len(m)-1]); v != "" {
				return v
			}
			return m
		})
	}
	p = os.ExpandEnv(p)
	if filepath.IsAbs(p) {
		return filepath.Clean(p), nil
	}
	if base == "" {
		return filepath.Abs(p)
	}
	return filepath.Join(base, p), nil
}

// Open 创建缺失的目录、收紧权限、清空 tmp/。
func Open(root string) (Dir, error) {
	if !filepath.IsAbs(root) {
		return Dir{}, fmt.Errorf("datadir: 数据目录 %s 不是绝对路径", root)
	}
	d := Dir{Root: filepath.Clean(root)}
	fail := func(err error) (Dir, error) {
		return Dir{}, fmt.Errorf("datadir: 数据目录 %s 不可写：%w", d.Root, err)
	}
	if err := os.MkdirAll(d.Root, 0o700); err != nil {
		return fail(err)
	}
	if err := Restrict(d.Root); err != nil {
		return fail(err)
	}
	if err := os.RemoveAll(d.Tmp()); err != nil {
		return fail(err)
	}
	for _, s := range subdirs {
		p := filepath.Join(d.Root, s)
		if err := os.MkdirAll(p, 0o700); err != nil {
			return fail(err)
		}
		if err := Restrict(p); err != nil {
			return fail(err)
		}
	}
	return d, nil
}

func (d Dir) file(name string) string { return filepath.Join(d.Root, name) }

// DB 是 SQLite 数据库文件。
func (d Dir) DB() string { return d.file("anydoor.db") }

// Config 是配置文件。
func (d Dir) Config() string { return d.file("anydoor.yaml") }

// Token 是本机 token 文件。
func (d Dir) Token() string { return d.file("token") }

// Port 是运行时写的端口文件。
func (d Dir) Port() string { return d.file("port") }

// Lock 是单实例锁文件。
func (d Dir) Lock() string { return d.file("anydoor.lock") }

// Secrets 是加密的密钥文件（只在文件后端时存在）。
func (d Dir) Secrets() string { return d.file("secrets.enc") }

// SecretsIndex 是密钥索引（名字、末 4 位、更新时间）。
func (d Dir) SecretsIndex() string { return d.file("secrets.index.json") }

// Blobs 是内容寻址存储目录。
func (d Dir) Blobs() string { return d.file("blobs") }

// Workspaces 是运行工作区目录。
func (d Dir) Workspaces() string { return d.file("workspaces") }

// Extensions 是已安装扩展包目录。
func (d Dir) Extensions() string { return d.file("extensions") }

// MCP 是 MCP stdio 服务的工作目录。
func (d Dir) MCP() string { return d.file("mcp") }

// Skills 是用户安装的技能目录。
func (d Dir) Skills() string { return d.file("skills") }

// Logs 是日志目录。
func (d Dir) Logs() string { return d.file("logs") }

// Tmp 是临时目录，Open 时清空。
func (d Dir) Tmp() string { return d.file("tmp") }

// renameRetryFor 是 Windows 上改名覆盖遇到目标正被打开时最多重试多久（B.07.05 §5，P.09 K244）；测试里调短。
var renameRetryFor = 2 * time.Second

// renameMaxWait 是两次重试之间最长等多久（B.07.05 §5：1 毫秒起、每次翻倍、最长 100 毫秒一次）。
const renameMaxWait = 100 * time.Millisecond

// nextWait 是下一次重试前的等待：翻倍，不超过 renameMaxWait。
func nextWait(w time.Duration) time.Duration { return min(2*w, renameMaxWait) }

// WritePrivate 原子写：同目录临时文件 → 写入 → Restrict → 改名覆盖；失败时不留临时文件。
// Windows 上目标正被别的句柄打开时，改名会退避重试（renameReplace）。
func WritePrivate(p string, data []byte) error {
	dir, name := filepath.Split(p)
	if dir == "" {
		dir = "."
	}
	var rnd [6]byte
	if _, err := rand.Read(rnd[:]); err != nil {
		return fmt.Errorf("datadir: 写 %s：%w", p, err)
	}
	tmp := filepath.Join(dir, "."+name+".tmp-"+hex.EncodeToString(rnd[:]))
	f, err := os.OpenFile(tmp, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o600)
	if err != nil {
		return fmt.Errorf("datadir: 写 %s：%w", p, err)
	}
	_, werr := f.Write(data)
	if werr == nil {
		werr = f.Sync()
	}
	err = errors.Join(werr, f.Close())
	if err == nil {
		err = Restrict(tmp)
	}
	if err == nil {
		err = renameReplace(tmp, p, renameRetryFor)
	}
	if err != nil {
		_ = os.Remove(tmp)
		return fmt.Errorf("datadir: 写 %s：%w", p, err)
	}
	return nil
}
