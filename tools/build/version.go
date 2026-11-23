package main

import (
	"errors"
	"fmt"
	"os/exec"
	"strconv"
	"strings"
	"time"
)

// 仓库不在 git 里（例如源码压缩包）或还没有提交时用的缺省值（设计文档 O.01 §8）。
const (
	unknownCommit = "unknown"
	unknownDate   = "1970-01-01T00:00:00Z"
)

// Version 按设计文档 O.01 §4.2 从 git 算出版本号、提交号和提交时间（UTC，RFC 3339）。
// 时间取提交时间而不是当前时间，保证同一提交两次构建的二进制相同（O-R3，P.09 K186）。
// 读不到 git 信息时返回缺省值和一个非空的 err，调用方只打印警告、继续构建。
func Version(repoDir string, preview bool) (version, commit, date string, err error) {
	commit, err = git(repoDir, "rev-parse", "HEAD")
	if err != nil {
		return versionString("0.0.0", -1, preview, false), unknownCommit, unknownDate,
			fmt.Errorf("读不到 git 信息，版本按 0.0.0-dev 处理：%w", err)
	}
	ct, err := git(repoDir, "show", "-s", "--format=%ct", "HEAD")
	if err != nil {
		return "", "", "", err
	}
	sec, err := strconv.ParseInt(ct, 10, 64)
	if err != nil {
		return "", "", "", fmt.Errorf("提交时间格式不对（%q）：%w", ct, err)
	}
	date = time.Unix(sec, 0).UTC().Format(time.RFC3339)

	status, err := git(repoDir, "status", "--porcelain")
	if err != nil {
		return "", "", "", err
	}
	dirty := status != ""

	base, distance := "0.0.0", -1
	if desc, derr := git(repoDir, "describe", "--tags", "--match", "v*", "--long"); derr == nil {
		tag, n, perr := parseDescribe(desc)
		if perr != nil {
			return "", "", "", perr
		}
		base, distance = strings.TrimPrefix(tag, "v"), n
	}
	return versionString(base, distance, preview, dirty), commit, date, nil
}

// parseDescribe 解析 `git describe --long` 的输出：v0.1.0-3-gabc1234 → ("v0.1.0", 3)。
func parseDescribe(desc string) (tag string, distance int, err error) {
	i := strings.LastIndex(desc, "-g")
	if i < 0 {
		return "", 0, fmt.Errorf("看不懂 git describe 的输出：%q", desc)
	}
	rest := desc[:i]
	j := strings.LastIndex(rest, "-")
	if j < 0 {
		return "", 0, fmt.Errorf("看不懂 git describe 的输出：%q", desc)
	}
	n, err := strconv.Atoi(rest[j+1:])
	if err != nil {
		return "", 0, fmt.Errorf("看不懂 git describe 的输出：%q", desc)
	}
	return rest[:j], n, nil
}

// versionString 拼出版本号。distance < 0 表示仓库里还没有 v* 标签。
//
//	在标签上            0.1.0
//	标签后第 3 个提交    0.1.0-dev.3
//	还没有标签          0.0.0-dev
//	预览版              0.1.0-preview、0.1.0-preview-dev.3
//	有未提交的修改      再加 +dirty
func versionString(base string, distance int, preview, dirty bool) string {
	var pre []string
	if preview {
		pre = append(pre, "preview")
	}
	switch {
	case distance < 0:
		pre = append(pre, "dev")
	case distance > 0:
		pre = append(pre, "dev."+strconv.Itoa(distance))
	}
	v := base
	if len(pre) > 0 {
		v += "-" + strings.Join(pre, "-")
	}
	if dirty {
		v += "+dirty"
	}
	return v
}

// git 在 dir 里执行 git 命令，返回去掉首尾空白的标准输出。
func git(dir string, args ...string) (string, error) {
	full := append([]string{"-C", dir, "-c", "core.quotepath=false"}, args...)
	out, err := exec.Command("git", full...).Output()
	if err != nil {
		var ee *exec.ExitError
		if errors.As(err, &ee) {
			return "", fmt.Errorf("git %s：%s", strings.Join(args, " "), strings.TrimSpace(string(ee.Stderr)))
		}
		return "", fmt.Errorf("git %s：%w", strings.Join(args, " "), err)
	}
	return strings.TrimSpace(string(out)), nil
}
