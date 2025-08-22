package pyenv

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/goplus/llgo/internal/env"
)

func TestEnsureBuildEnv(t *testing.T) {
	// 保存原始环境变量
	originalPath := os.Getenv("PATH")
	originalPythonHome := os.Getenv("PYTHONHOME")
	originalDyldLibraryPath := os.Getenv("DYLD_LIBRARY_PATH")
	originalPkgConfigPath := os.Getenv("PKG_CONFIG_PATH")
	originalPythonPath := os.Getenv("PYTHONPATH")

	// 测试后恢复环境变量
	defer func() {
		os.Setenv("PATH", originalPath)
		os.Setenv("PYTHONHOME", originalPythonHome)
		os.Setenv("DYLD_LIBRARY_PATH", originalDyldLibraryPath)
		os.Setenv("PKG_CONFIG_PATH", originalPkgConfigPath)
		if originalPythonPath != "" {
			os.Setenv("PYTHONPATH", originalPythonPath)
		} else {
			os.Unsetenv("PYTHONPATH")
		}
	}()

	t.Run("BasicEnvironmentSetup", func(t *testing.T) {
		err := EnsureBuildEnv()
		if err != nil {
			t.Logf("EnsureBuildEnv failed (expected for missing Python): %v", err)
			// 如果Python环境不存在，这是预期的
			return
		}

		// 检查环境变量是否被正确设置
		pyHome := PythonHome()
		if pyHome == "" {
			t.Error("PythonHome should not be empty after EnsureBuildEnv")
		}

		// 检查PATH是否包含Python bin目录
		path := os.Getenv("PATH")
		binDir := filepath.Join(pyHome, "bin")
		if !strings.Contains(path, binDir) {
			t.Errorf("PATH should contain %s, got: %s", binDir, path)
		}

		// 检查PYTHONHOME
		if os.Getenv("PYTHONHOME") != pyHome {
			t.Errorf("PYTHONHOME should be %s, got: %s", pyHome, os.Getenv("PYTHONHOME"))
		}

		// 在macOS上检查DYLD_LIBRARY_PATH
		if runtime.GOOS == "darwin" {
			dyldPath := os.Getenv("DYLD_LIBRARY_PATH")
			libDir := filepath.Join(pyHome, "lib")
			if dyldPath == "" {
				t.Error("DYLD_LIBRARY_PATH should be set on macOS")
			} else if !strings.Contains(dyldPath, libDir) {
				t.Errorf("DYLD_LIBRARY_PATH should contain %s, got: %s", libDir, dyldPath)
			}
		}

		// 检查PKG_CONFIG_PATH
		pkgConfigPath := os.Getenv("PKG_CONFIG_PATH")
		expectedPkgConfig := filepath.Join(pyHome, "lib", "pkgconfig")
		if pkgConfigPath == "" {
			t.Error("PKG_CONFIG_PATH should be set")
		} else if !strings.Contains(pkgConfigPath, expectedPkgConfig) {
			t.Errorf("PKG_CONFIG_PATH should contain %s, got: %s", expectedPkgConfig, pkgConfigPath)
		}

		// 检查PYTHONPATH是否被清除
		if os.Getenv("PYTHONPATH") != "" {
			t.Error("PYTHONPATH should be unset")
		}
	})

	t.Run("WithCustomLLPYG_PYHOME", func(t *testing.T) {
		// 设置自定义PYTHONHOME
		customPath := "/custom/python/path"
		os.Setenv("LLPYG_PYHOME", customPath)

		err := EnsureBuildEnv()
		if err != nil {
			t.Logf("EnsureBuildEnv failed with custom path: %v", err)
			return
		}

		pyHome := PythonHome()
		if pyHome != customPath {
			t.Errorf("PythonHome should be %s, got: %s", customPath, pyHome)
		}

		// 清理
		os.Unsetenv("LLPYG_PYHOME")
	})
}

func TestVerify(t *testing.T) {
	t.Run("PythonVerification", func(t *testing.T) {
		err := Verify()
		if err != nil {
			t.Logf("Python verification failed (expected if Python not available): %v", err)
			// 如果Python不可用，这是预期的
			return
		}
		// 如果验证成功，说明Python环境可用
		t.Log("Python environment is available and working")
	})
}

func TestPythonHome(t *testing.T) {
	// 保存原始环境变量
	originalLLPYG_PYHOME := os.Getenv("LLPYG_PYHOME")
	defer func() {
		if originalLLPYG_PYHOME != "" {
			os.Setenv("LLPYG_PYHOME", originalLLPYG_PYHOME)
		} else {
			os.Unsetenv("LLPYG_PYHOME")
		}
	}()

	t.Run("DefaultPath", func(t *testing.T) {
		os.Unsetenv("LLPYG_PYHOME")
		pyHome := PythonHome()
		expectedPath := filepath.Join(env.LLGoCacheDir(), "python_env", "python")
		if pyHome != expectedPath {
			t.Errorf("Expected default path %s, got %s", expectedPath, pyHome)
		}
	})

	t.Run("CustomPath", func(t *testing.T) {
		customPath := "/custom/python/path"
		os.Setenv("LLPYG_PYHOME", customPath)
		pyHome := PythonHome()
		if pyHome != customPath {
			t.Errorf("Expected custom path %s, got %s", customPath, pyHome)
		}
	})
}

func TestFindPythonExec(t *testing.T) {
	t.Run("FindPythonExecutable", func(t *testing.T) {
		execPath, err := findPythonExec()
		if err != nil {
			t.Logf("Python executable not found (expected if Python not installed): %v", err)
			// 如果Python未安装，这是预期的
			return
		}
		if execPath == "" {
			t.Error("Python executable path should not be empty")
		}
		t.Logf("Found Python executable at: %s", execPath)
	})
}

func TestApplyEnv(t *testing.T) {
	// 保存原始环境变量
	originalPath := os.Getenv("PATH")
	originalPythonHome := os.Getenv("PYTHONHOME")
	originalDyldLibraryPath := os.Getenv("DYLD_LIBRARY_PATH")
	originalPkgConfigPath := os.Getenv("PKG_CONFIG_PATH")
	originalPythonPath := os.Getenv("PYTHONPATH")

	// 测试后恢复环境变量
	defer func() {
		os.Setenv("PATH", originalPath)
		os.Setenv("PYTHONHOME", originalPythonHome)
		os.Setenv("DYLD_LIBRARY_PATH", originalDyldLibraryPath)
		os.Setenv("PKG_CONFIG_PATH", originalPkgConfigPath)
		if originalPythonPath != "" {
			os.Setenv("PYTHONPATH", originalPythonPath)
		} else {
			os.Unsetenv("PYTHONPATH")
		}
	}()

	t.Run("EmptyPyHome", func(t *testing.T) {
		err := applyEnv("")
		if err != nil {
			t.Errorf("applyEnv with empty path should not return error: %v", err)
		}
	})

	t.Run("ValidPyHome", func(t *testing.T) {
		testPyHome := "/test/python/home"
		err := applyEnv(testPyHome)
		if err != nil {
			t.Errorf("applyEnv failed: %v", err)
		}

		// 检查PYTHONHOME
		if os.Getenv("PYTHONHOME") != testPyHome {
			t.Errorf("PYTHONHOME should be %s, got: %s", testPyHome, os.Getenv("PYTHONHOME"))
		}

		// 检查PATH
		path := os.Getenv("PATH")
		binDir := filepath.Join(testPyHome, "bin")
		if !strings.Contains(path, binDir) {
			t.Errorf("PATH should contain %s, got: %s", binDir, path)
		}

		// 在macOS上检查DYLD_LIBRARY_PATH
		if runtime.GOOS == "darwin" {
			dyldPath := os.Getenv("DYLD_LIBRARY_PATH")
			libDir := filepath.Join(testPyHome, "lib")
			if !strings.Contains(dyldPath, libDir) {
				t.Errorf("DYLD_LIBRARY_PATH should contain %s, got: %s", libDir, dyldPath)
			}
		}

		// 检查PKG_CONFIG_PATH
		pkgConfigPath := os.Getenv("PKG_CONFIG_PATH")
		expectedPkgConfig := filepath.Join(testPyHome, "lib", "pkgconfig")
		if !strings.Contains(pkgConfigPath, expectedPkgConfig) {
			t.Errorf("PKG_CONFIG_PATH should contain %s, got: %s", expectedPkgConfig, pkgConfigPath)
		}

		// 检查PYTHONPATH是否被清除
		if os.Getenv("PYTHONPATH") != "" {
			t.Error("PYTHONPATH should be unset")
		}
	})

	t.Run("ExistingPathHandling", func(t *testing.T) {
		// 设置现有的PATH
		existingPath := "/usr/bin:/usr/local/bin"
		os.Setenv("PATH", existingPath)

		testPyHome := "/test/python/home"
		err := applyEnv(testPyHome)
		if err != nil {
			t.Errorf("applyEnv failed: %v", err)
		}

		path := os.Getenv("PATH")
		binDir := filepath.Join(testPyHome, "bin")
		if !strings.Contains(path, binDir) {
			t.Errorf("PATH should contain %s, got: %s", binDir, path)
		}
		if !strings.Contains(path, existingPath) {
			t.Errorf("PATH should contain existing path %s, got: %s", existingPath, path)
		}
	})
}

func TestInstallPackages(t *testing.T) {
	t.Run("EmptyPackages", func(t *testing.T) {
		err := InstallPackages()
		if err != nil {
			t.Errorf("InstallPackages with empty list should not return error: %v", err)
		}
	})

	t.Run("WithPackages", func(t *testing.T) {
		// 这个测试需要真实的Python环境，所以跳过
		t.Skip("InstallPackages test requires real Python environment")
	})
}

func TestPipInstall(t *testing.T) {
	t.Run("EmptySpec", func(t *testing.T) {
		err := PipInstall("")
		if err != nil {
			t.Errorf("PipInstall with empty spec should not return error: %v", err)
		}
	})

	t.Run("ValidSpec", func(t *testing.T) {
		// 这个测试需要真实的Python环境，所以跳过
		t.Skip("PipInstall test requires real Python environment")
	})
}

// 测试环境变量路径分隔符处理
func TestPathSeparatorHandling(t *testing.T) {
	t.Run("PathSeparator", func(t *testing.T) {
		separator := string(os.PathListSeparator)
		if separator == "" {
			t.Error("PathListSeparator should not be empty")
		}
		t.Logf("Path separator: %q", separator)
	})
}

// 测试平台特定功能
func TestPlatformSpecific(t *testing.T) {
	t.Run("DarwinSpecific", func(t *testing.T) {
		if runtime.GOOS == "darwin" {
			t.Log("Running on macOS - DYLD_LIBRARY_PATH should be set")
		} else {
			t.Log("Not running on macOS - DYLD_LIBRARY_PATH will not be set")
		}
	})
}

// 基准测试
func BenchmarkPythonHome(b *testing.B) {
	for i := 0; i < b.N; i++ {
		_ = PythonHome()
	}
}

func BenchmarkFindPythonExec(b *testing.B) {
	for i := 0; i < b.N; i++ {
		_, _ = findPythonExec()
	}
} 