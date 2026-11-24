package main

import (
	"archive/tar"
	"archive/zip"
	"bytes"
	"compress/gzip"
	_ "embed"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"text/template"
	"time"
)

//go:embed README.txt.tmpl
var readmeTmpl string

// entry 是归档里的一个文件。
type entry struct {
	name string
	data []byte
	mode fs.FileMode
}

// Archive 把 dir/tmp/<os>-<arch>/ 下的两个程序连同说明文件打成 dir/anydoor-<版本>-<os>-<arch>.<zip|tar.gz>（O.03 §4）。
// 文件按名字排序，修改时间统一为 mtime，可执行文件 0755、其余 0644，同样的输入得到同样的字节（§6.1 第 4 步）。
func Archive(dir string, t Target, version string, mtime time.Time) (string, error) {
	name := fmt.Sprintf("anydoor-%s-%s-%s", version, t.OS, t.Arch)
	bins := filepath.Join(dir, "tmp", t.OS+"-"+t.Arch)
	var es []entry
	for _, p := range []string{"anydoor", "anydoorctl"} {
		b, err := os.ReadFile(filepath.Join(bins, t.Exe(p)))
		if err != nil {
			return "", err
		}
		es = append(es, entry{t.Exe(p), b, 0o755})
	}
	readme, err := renderReadme(t, version)
	if err != nil {
		return "", err
	}
	es = append(es, entry{"README.txt", readme, 0o644})
	for _, p := range []string{"LICENSE", "NOTICE"} {
		b, err := os.ReadFile(filepath.Join(root, p))
		if err != nil {
			warnf("仓库里没有 %s，归档里不放（G.17 还没有落地，O.03 §8）", p)
			continue
		}
		es = append(es, entry{p, b, 0o644})
	}
	cl, err := changelogSection(version)
	if err != nil {
		return "", err
	}
	es = append(es, entry{"CHANGELOG.md", cl, 0o644})
	sort.Slice(es, func(i, j int) bool { return es[i].name < es[j].name })

	var buf bytes.Buffer
	if t.OS == "windows" {
		err = writeZip(&buf, name, es, mtime)
	} else {
		err = writeTarGz(&buf, name, es, mtime)
	}
	if err != nil {
		return "", err
	}
	out := filepath.Join(dir, name+t.Ext())
	return out, os.WriteFile(out, buf.Bytes(), 0o644)
}

func writeZip(w io.Writer, top string, es []entry, mtime time.Time) error {
	zw := zip.NewWriter(w)
	dh := &zip.FileHeader{Name: top + "/", Method: zip.Store, Modified: mtime}
	dh.SetMode(fs.ModeDir | 0o755)
	if _, err := zw.CreateHeader(dh); err != nil {
		return err
	}
	for _, e := range es {
		h := &zip.FileHeader{Name: top + "/" + e.name, Method: zip.Deflate, Modified: mtime}
		h.SetMode(e.mode)
		f, err := zw.CreateHeader(h)
		if err != nil {
			return err
		}
		if _, err := f.Write(e.data); err != nil {
			return err
		}
	}
	return zw.Close()
}

func writeTarGz(w io.Writer, top string, es []entry, mtime time.Time) error {
	gz := gzip.NewWriter(w) // 头里的时间和文件名都留空，保证可复现
	tw := tar.NewWriter(gz)
	if err := tw.WriteHeader(&tar.Header{Typeflag: tar.TypeDir, Name: top + "/", Mode: 0o755, ModTime: mtime, Format: tar.FormatUSTAR}); err != nil {
		return err
	}
	for _, e := range es {
		h := &tar.Header{Typeflag: tar.TypeReg, Name: top + "/" + e.name, Mode: int64(e.mode), Size: int64(len(e.data)), ModTime: mtime, Format: tar.FormatUSTAR}
		if err := tw.WriteHeader(h); err != nil {
			return err
		}
		if _, err := tw.Write(e.data); err != nil {
			return err
		}
	}
	if err := tw.Close(); err != nil {
		return err
	}
	return gz.Close()
}

// dataDirs 是三个系统上的默认数据目录（B.07.05 §4）。
var dataDirs = map[string]string{
	"windows": `%APPDATA%\AnywhereDoor`,
	"darwin":  "~/Library/Application Support/AnywhereDoor",
	"linux":   "$XDG_DATA_HOME/anydoor（没设 XDG_DATA_HOME 时是 ~/.local/share/anydoor）",
}

func renderReadme(t Target, version string) ([]byte, error) {
	tmpl, err := template.New("README.txt").Parse(readmeTmpl)
	if err != nil {
		return nil, err
	}
	var b bytes.Buffer
	err = tmpl.Execute(&b, map[string]any{
		"Version": version, "OS": t.OS, "Arch": t.Arch, "Exe": t.Exe("anydoor"), "DataDir": dataDirs[t.OS],
	})
	return b.Bytes(), err
}

// changelogSection 是 CHANGELOG.md 里本版本那一节（## [0.1.0] …）；没有就用"未发布"那一节并打印警告。
func changelogSection(version string) ([]byte, error) {
	b, err := os.ReadFile(filepath.Join(root, "CHANGELOG.md"))
	if err != nil {
		return nil, fmt.Errorf("读不了 CHANGELOG.md：%w", err)
	}
	base, _, _ := strings.Cut(version, "-")
	lines := strings.Split(strings.ReplaceAll(string(b), "\r\n", "\n"), "\n")
	section := func(prefix string) []string {
		start := -1
		for i, l := range lines {
			switch {
			case start < 0 && strings.HasPrefix(l, prefix):
				start = i
			case start >= 0 && strings.HasPrefix(l, "## ["):
				return lines[start:i]
			}
		}
		if start < 0 {
			return nil
		}
		return lines[start:]
	}
	s := section("## [" + base + "]")
	if s == nil {
		warnf("CHANGELOG.md 里没有 [%s] 这一节，用\"未发布\"那一节", base)
		s = section("## [未发布]")
	}
	if s == nil {
		return nil, fmt.Errorf("CHANGELOG.md 里既没有 [%s] 也没有 [未发布]", base)
	}
	return []byte("# 变更日志\n\n" + strings.TrimRight(strings.Join(s, "\n"), "\n") + "\n"), nil
}
