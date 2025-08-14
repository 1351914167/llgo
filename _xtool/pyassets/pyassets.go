package main

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"embed"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
)

//go:embed python/**
var pyFS embed.FS

// 新增：单文件归档，完整包含 python/ 下所有内容（含 _ 前缀文件等）
//
//go:embed python.tar.gz
var pyTarGz []byte

// 返回解压后的 PYTHONHOME 目录路径（.../tmpdir/python）
func ExtractToTemp() (string, error) {
	root, err := os.MkdirTemp("", "py-embed-*")
	if err != nil {
		return "", err
	}
	dstRoot := filepath.Join(root, "python")
	if len(pyTarGz) > 0 {
		if err := extractTarGzTo(dstRoot, pyTarGz); err != nil {
			return "", err
		}
		return filepath.Dir(dstRoot), nil // 与老签名保持一致：返回 .../tmpdir/python
	}
	// 归档缺失时，回退到逐文件落盘（受 go:embed 过滤限制）
	if err := extractFS(pyFS, "python", dstRoot); err != nil {
		return "", err
	}
	return dstRoot, nil
}

// 解压到指定目录，返回 PYTHONHOME 路径（.../dstRoot/python）
func ExtractToDir(dstRoot string) (string, error) {
	root := filepath.Join(dstRoot, "test", "python")
	if len(pyTarGz) > 0 {
		if err := extractTarGzTo(root, pyTarGz); err != nil {
			return "", err
		}
		return root, nil
	}
	// 归档缺失时，回退
	if err := extractFS(pyFS, "python", root); err != nil {
		return "", err
	}
	return root, nil
}

func extractFS(e embed.FS, srcRoot, dstRoot string) error {
	return fs.WalkDir(e, srcRoot, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, _ := filepath.Rel(srcRoot, p)
		target := filepath.Join(dstRoot, rel)
		if d.IsDir() {
			return os.MkdirAll(target, 0o755)
		}
		data, err := e.ReadFile(p)
		if err != nil {
			return err
		}
		mode := fs.FileMode(0o644)
		if strings.Contains(target, string(filepath.Separator)+"bin"+string(filepath.Separator)) {
			mode = 0o755
		}
		return os.WriteFile(target, data, mode)
	})
}

func extractTarGzTo(dstRoot string, tgz []byte) error {
	gr, err := gzip.NewReader(bytes.NewReader(tgz))
	if err != nil {
		return err
	}
	defer gr.Close()
	tr := tar.NewReader(gr)
	for {
		hdr, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return err
		}
		// 归档内应为 "python/..."，我们要把它展开到 dstRoot (正好是 .../python)
		// 如果你的归档包含顶层目录 python/，则这里直接用 hdr.Name 的相对路径
		name := hdr.Name
		// 屏蔽绝对路径与上级目录
		name = filepath.Clean(name)
		if strings.HasPrefix(name, "/") || strings.Contains(name, ".."+string(filepath.Separator)) {
			return fmt.Errorf("invalid path in tar: %s", name)
		}
		target := filepath.Join(filepath.Dir(dstRoot), name) // 保留归档内的 python/ 前缀
		switch hdr.Typeflag {
		case tar.TypeDir:
			if err := os.MkdirAll(target, hdr.FileInfo().Mode().Perm()); err != nil {
				return err
			}
		case tar.TypeReg, tar.TypeRegA:
			if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
				return err
			}
			f, err := os.OpenFile(target, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, hdr.FileInfo().Mode().Perm())
			if err != nil {
				return err
			}
			if _, err := io.Copy(f, tr); err != nil {
				_ = f.Close()
				return err
			}
			if err := f.Close(); err != nil {
				return err
			}
		case tar.TypeSymlink:
			if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
				return err
			}
			// 先删除可能存在的旧文件/链接
			_ = os.Remove(target)
			if err := os.Symlink(hdr.Linkname, target); err != nil {
				return err
			}
		default:
			// 其他类型（如硬链接等）一般不会在此发行物中出现，按需扩展
		}
	}
	return nil
}
