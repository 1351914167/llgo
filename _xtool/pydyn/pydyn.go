package pydyn

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
)

// Read from env LLPYG_PYHOME; if unset, return defaultPath
func GetPyHome(defaultPath string) string {
	if v := os.Getenv("LLPYG_PYHOME"); v != "" {
		return v
	}
	return defaultPath
}

// Inject pyHome into current process environment to affect subsequent exec.Command python3/pip3
// - Prepend PATH:  <pyHome>/bin:... (if not present)
// - Set PYTHONHOME=<pyHome>
// - On macOS, append <pyHome>/lib to DYLD_LIBRARY_PATH (if not present)
// - Unset PYTHONPATH (to avoid interference)
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

	// macOS dynamic libraries
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

	// Avoid interference from custom PYTHONPATH
	_ = os.Unsetenv("PYTHONPATH")
	return nil
}
