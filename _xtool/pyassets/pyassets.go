package pyassets

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

// New: single-file archive fully contains everything under python/ (including files prefixed with _)
//
//go:embed python.tar.gz
var pyTarGz []byte

// Return the extracted PYTHONHOME directory path (.../tmpdir/python)
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
		return filepath.Dir(dstRoot), nil // keep old signature: return .../tmpdir/python
	}
	// When archive is missing, fall back to writing individual files (limited by go:embed filters)
	if err := extractFS(pyFS, "python", dstRoot); err != nil {
		return "", err
	}
	return dstRoot, nil
}

// Extract to a specified directory and return the PYTHONHOME path (.../dstRoot/python)
func ExtractToDir(dstRoot string) (string, error) {
	root := filepath.Join(dstRoot, "python")

	// Skip extraction if already exists
	if st, err := os.Stat(root); err == nil && st.IsDir() {
		return root, nil
	}

	if len(pyTarGz) > 0 {
		if err := extractTarGzTo(root, pyTarGz); err != nil {
			return "", err
		}
		return root, nil
	}
	// Fallback when archive is missing
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
		// The archive should contain paths like "python/..." which we expand to dstRoot (which is .../python)
		// If your archive includes the top-level directory python/, use hdr.Name directly as a relative path
		name := hdr.Name
		// Sanitize absolute and parent paths
		name = filepath.Clean(name)
		if strings.HasPrefix(name, "/") || strings.Contains(name, ".."+string(filepath.Separator)) {
			return fmt.Errorf("invalid path in tar: %s", name)
		}
		target := filepath.Join(filepath.Dir(dstRoot), name) // keep python/ prefix from archive
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
			// Remove any existing file/symlink first
			_ = os.Remove(target)
			if err := os.Symlink(hdr.Linkname, target); err != nil {
				return err
			}
		default:
			// Other types (e.g., hard links) are unlikely in this distribution; extend as needed
		}
	}
	return nil
}
