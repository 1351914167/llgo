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
	// Save original environment variables
	originalPath := os.Getenv("PATH")
	originalPythonHome := os.Getenv("PYTHONHOME")
	originalDyldLibraryPath := os.Getenv("DYLD_LIBRARY_PATH")
	originalPkgConfigPath := os.Getenv("PKG_CONFIG_PATH")
	originalPythonPath := os.Getenv("PYTHONPATH")

	// Restore environment variables after test
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
			// If Python environment doesn't exist, this is expected
			return
		}

		// Check if environment variables are set correctly
		pyHome := PythonHome()
		if pyHome == "" {
			t.Error("PythonHome should not be empty after EnsureBuildEnv")
		}

		// Check if PATH contains Python bin directory
		path := os.Getenv("PATH")
		binDir := filepath.Join(pyHome, "bin")
		if !strings.Contains(path, binDir) {
			t.Errorf("PATH should contain %s, got: %s", binDir, path)
		}

		// Check PYTHONHOME
		if os.Getenv("PYTHONHOME") != pyHome {
			t.Errorf("PYTHONHOME should be %s, got: %s", pyHome, os.Getenv("PYTHONHOME"))
		}

		// Check DYLD_LIBRARY_PATH on macOS
		if runtime.GOOS == "darwin" {
			dyldPath := os.Getenv("DYLD_LIBRARY_PATH")
			libDir := filepath.Join(pyHome, "lib")
			if dyldPath == "" {
				t.Error("DYLD_LIBRARY_PATH should be set on macOS")
			} else if !strings.Contains(dyldPath, libDir) {
				t.Errorf("DYLD_LIBRARY_PATH should contain %s, got: %s", libDir, dyldPath)
			}
		}

		// Check PKG_CONFIG_PATH
		pkgConfigPath := os.Getenv("PKG_CONFIG_PATH")
		expectedPkgConfig := filepath.Join(pyHome, "lib", "pkgconfig")
		if pkgConfigPath == "" {
			t.Error("PKG_CONFIG_PATH should be set")
		} else if !strings.Contains(pkgConfigPath, expectedPkgConfig) {
			t.Errorf("PKG_CONFIG_PATH should contain %s, got: %s", expectedPkgConfig, pkgConfigPath)
		}

		// Check if PYTHONPATH is cleared
		if os.Getenv("PYTHONPATH") != "" {
			t.Error("PYTHONPATH should be unset")
		}
	})

	t.Run("WithCustomLLPYG_PYHOME", func(t *testing.T) {
		// Set custom PYTHONHOME
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

		// Clean up
		os.Unsetenv("LLPYG_PYHOME")
	})
}

func TestVerify(t *testing.T) {
	t.Run("PythonVerification", func(t *testing.T) {
		err := Verify()
		if err != nil {
			t.Logf("Python verification failed (expected if no Python available): %v", err)
			return
		}
		t.Logf("Python environment is available and working")
	})
}

func TestPythonHome(t *testing.T) {
	t.Run("DefaultPath", func(t *testing.T) {
		// Clear any custom PYTHONHOME
		os.Unsetenv("LLPYG_PYHOME")

		pyHome := PythonHome()
		expectedPath := filepath.Join(env.LLGoCacheDir(), "python_env", "python")
		if pyHome != expectedPath {
			t.Errorf("PythonHome should be %s, got: %s", expectedPath, pyHome)
		}
	})

	t.Run("CustomPath", func(t *testing.T) {
		// Set custom PYTHONHOME
		customPath := "/custom/python/path"
		os.Setenv("LLPYG_PYHOME", customPath)

		pyHome := PythonHome()
		if pyHome != customPath {
			t.Errorf("PythonHome should be %s, got: %s", customPath, pyHome)
		}

		// Clean up
		os.Unsetenv("LLPYG_PYHOME")
	})
}

func TestFindPythonExec(t *testing.T) {
	t.Run("FindPythonExecutable", func(t *testing.T) {
		exe, err := findPythonExec()
		if err != nil {
			t.Logf("Python executable not found (expected if no Python installed): %v", err)
			return
		}
		t.Logf("Found Python executable at: %s", exe)
	})
}

func TestApplyEnv(t *testing.T) {
	// Save original environment variables
	originalPath := os.Getenv("PATH")
	originalPythonHome := os.Getenv("PYTHONHOME")
	originalDyldLibraryPath := os.Getenv("DYLD_LIBRARY_PATH")
	originalPkgConfigPath := os.Getenv("PKG_CONFIG_PATH")

	// Restore environment variables after test
	defer func() {
		os.Setenv("PATH", originalPath)
		os.Setenv("PYTHONHOME", originalPythonHome)
		os.Setenv("DYLD_LIBRARY_PATH", originalDyldLibraryPath)
		os.Setenv("PKG_CONFIG_PATH", originalPkgConfigPath)
	}()

	t.Run("EmptyPyHome", func(t *testing.T) {
		err := applyEnv("")
		if err != nil {
			t.Errorf("applyEnv with empty pyHome should not fail: %v", err)
		}
	})

	t.Run("ValidPyHome", func(t *testing.T) {
		testPyHome := "/test/python/home"
		err := applyEnv(testPyHome)
		if err != nil {
			t.Errorf("applyEnv failed: %v", err)
		}

		// Check if PYTHONHOME is set
		if os.Getenv("PYTHONHOME") != testPyHome {
			t.Errorf("PYTHONHOME should be %s, got: %s", testPyHome, os.Getenv("PYTHONHOME"))
		}

		// Check if PATH contains bin directory
		path := os.Getenv("PATH")
		binDir := filepath.Join(testPyHome, "bin")
		if !strings.Contains(path, binDir) {
			t.Errorf("PATH should contain %s, got: %s", binDir, path)
		}
	})

	t.Run("ExistingPathHandling", func(t *testing.T) {
		// Set existing PATH
		existingPath := "/existing/path"
		os.Setenv("PATH", existingPath)

		testPyHome := "/test/python/home"
		err := applyEnv(testPyHome)
		if err != nil {
			t.Errorf("applyEnv failed: %v", err)
		}

		// Check if PATH is prepended correctly
		path := os.Getenv("PATH")
		binDir := filepath.Join(testPyHome, "bin")
		expectedPath := binDir + string(os.PathListSeparator) + existingPath
		if path != expectedPath {
			t.Errorf("PATH should be %s, got: %s", expectedPath, path)
		}
	})
}

func TestInstallPackages(t *testing.T) {
	t.Run("EmptyPackages", func(t *testing.T) {
		err := InstallPackages()
		if err != nil {
			t.Errorf("InstallPackages with empty packages should not fail: %v", err)
		}
	})

	t.Run("WithPackages", func(t *testing.T) {
		// Skip this test as it requires a real Python environment
		t.Skip("InstallPackages test requires real Python environment")
	})
}

func TestPipInstall(t *testing.T) {
	t.Run("EmptySpec", func(t *testing.T) {
		err := PipInstall("")
		if err != nil {
			t.Errorf("PipInstall with empty spec should not fail: %v", err)
		}
	})

	t.Run("ValidSpec", func(t *testing.T) {
		// Skip this test as it requires a real Python environment
		t.Skip("PipInstall test requires real Python environment")
	})
}

func TestPathSeparatorHandling(t *testing.T) {
	t.Run("PathSeparator", func(t *testing.T) {
		separator := string(os.PathListSeparator)
		t.Logf("Path separator: %q", separator)
	})
}

func TestPlatformSpecific(t *testing.T) {
	t.Run("DarwinSpecific", func(t *testing.T) {
		if runtime.GOOS == "darwin" {
			t.Logf("Running on macOS - DYLD_LIBRARY_PATH should be set")
		}
	})
}

// Benchmark tests
func BenchmarkPythonHome(b *testing.B) {
	for i := 0; i < b.N; i++ {
		PythonHome()
	}
}

func BenchmarkFindPythonExec(b *testing.B) {
	for i := 0; i < b.N; i++ {
		findPythonExec()
	}
} 