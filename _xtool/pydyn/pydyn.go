package pydyn

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
)

// 从环境变量 LLPYG_PYHOME 读取；若未设置则返回 defaultPath
func GetPyHome(defaultPath string) string {
	if v := os.Getenv("LLPYG_PYHOME"); v != "" {
		return v
	}
	return defaultPath
}

// 将 pyHome 注入到当前进程环境，影响后续 exec.Command 使用的 python3/pip3
// - 预置 PATH:  <pyHome>/bin:...（若未存在）
// - 设置 PYTHONHOME=<pyHome>
// - macOS 额外设置 DYLD_LIBRARY_PATH 追加 <pyHome>/lib（若未包含）
// - 移除 PYTHONPATH（避免干扰）
func ApplyEnv(pyHome string) error {
	if pyHome == "" {
		return nil
	}
	bin := filepath.Join(pyHome, "bin")
	lib := filepath.Join(pyHome, "lib")

	// PATH
	path := os.Getenv("PATH")
	parts := strings.Split(path, string(os.PathListSeparator))
	hasBin := false
	for _, p := range parts {
		if p == bin {
			hasBin = true
			break
		}
	}
	if !hasBin {
		newPath := bin
		if path != "" {
			newPath += string(os.PathListSeparator) + path
		}
		if err := os.Setenv("PATH", newPath); err != nil {
			return err
		}
	}

	// PYTHONHOME
	if err := os.Setenv("PYTHONHOME", pyHome); err != nil {
		return err
	}

	// macOS 动态库
	if runtime.GOOS == "darwin" {
		dyld := os.Getenv("DYLD_LIBRARY_PATH")
		if dyld == "" {
			if err := os.Setenv("DYLD_LIBRARY_PATH", lib); err != nil {
				return err
			}
		} else if !strings.Contains(dyld, lib) {
			if err := os.Setenv("DYLD_LIBRARY_PATH", lib+string(os.PathListSeparator)+dyld); err != nil {
				return err
			}
		}
	}

	// PKG_CONFIG_PATH
	pkgcfg := filepath.Join(pyHome, "lib", "pkgconfig")
	pcp := os.Getenv("PKG_CONFIG_PATH")
	if pcp == "" {
		_ = os.Setenv("PKG_CONFIG_PATH", pkgcfg)
	} else {
		parts := strings.Split(pcp, string(os.PathListSeparator))
		found := false
		for _, p := range parts {
			if p == pkgcfg {
				found = true
				break
			}
		}
		if !found {
			_ = os.Setenv("PKG_CONFIG_PATH", pkgcfg+string(os.PathListSeparator)+pcp)
		}
	}

	// 避免自定义 PYTHONPATH 干扰
	_ = os.Unsetenv("PYTHONPATH")
	return nil
}
