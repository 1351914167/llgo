// llgo/cmd/internal/get/get.go
package get

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/goplus/llgo/_xtool/pyinstaller"
	"github.com/goplus/llgo/cmd/internal/base"
)

var Cmd = &base.Command{
	UsageLine: "llgo get [-t -u -v] [build flags] [modules...]",
	Short:     "Check local and remote llpkg; if available, fetch dependencies via Go Modules",
	Run:       run,
}

// removed: shortSpecRe

func run(cmd *base.Command, args []string) {
	if err := Main(args); err != nil {
		fmt.Fprintln(os.Stderr, "llgo get:", err)
		os.Exit(1)
	}
}

// Main is exposed for direct invocation by .gox commands
func Main(args []string) error {
	// Parse as "-flags... followed by module list"
	flags := make([]string, 0, len(args))
	var modules []string
	flagEndIndex := -1
	for idx, a := range args {
		if strings.HasPrefix(a, "-") {
			flags = append(flags, a)
			flagEndIndex = idx
		} else {
			break
		}
	}
	if flagEndIndex >= 0 {
		modules = args[flagEndIndex+1:]
	} else {
		modules = args
	}
	if len(modules) == 0 {
		return fmt.Errorf("usage: llgo get [-t -u -v] [build flags] [modules...]")
	}

	var firstErr error
	for _, m := range modules {
		if err := processModuleArg(m, flags); err != nil {
			if firstErr == nil {
				firstErr = err
			}
			fmt.Fprintln(os.Stderr, err)
		}
	}
	return firstErr
}

const llpkgPrefix = "github.com/Bigdata-shiyang/test/"

// Handle a single argument (could be module[@version] or name@Version)
func processModuleArg(arg string, flags []string) error {
	name, version, _ := strings.Cut(arg, "@")
	if name == "" {
		return fmt.Errorf("invalid module path: %s", arg)
	}
	// In processModuleArg:
	if strings.Contains(name, "/") {
		return handleModuleSpecWithFlags(name, version, flags)
	}
	if version == "" {
		// Short name without version → treat as llpkg/<name> and use latest
		return handleModuleSpecWithFlags(llpkgPrefix+name, "", flags)
	}
	return ensureLLPkgByNameVersionWithFlags(name, version, flags)
}

// Handle requests of the form module[@version] (supports flags)
func handleModuleSpecWithFlags(mod string, ver string, flags []string) error {
	if !strings.HasPrefix(mod, llpkgPrefix) {
		spec := mod
		if ver != "" {
			spec += "@" + ver
		}
		if err := runGoGetWithFlags(flags, spec); err != nil {
			return err
		}
		return runGoModTidy()
	}

	// For llpkg modules: if a version is specified, validate on the remote; otherwise use latest
	if ver == "" {
		// if inLocal(mod) {
		// 	vers, _ := listModuleVersionsLocal(mod) // GOPROXY=off
		// 	if len(vers) > 0 {
		// 		return runGoGetWithFlags(flags, mod+"@"+vers[len(vers)-1])
		// 	}
		// }
		if err := runGoGetWithFlags(flags, mod+"@latest"); err != nil {
			printLLPygHint()
			return fmt.Errorf("specified version does not exist in remote llpkg: %s@%s", mod, "latest")
		}
		// Additionally: after remote fetch succeeds, install the Python package
		pkgName := strings.TrimPrefix(mod, llpkgPrefix)
		if err := installPythonPackage(pkgName); err != nil {
			return fmt.Errorf("failed to install Python package: %w", err)
		}
		return runGoModTidy()
	}
	vers, err := listModuleVersions(mod) // remote version list
	if err != nil {
		return err
	}
	if !contains(vers, ver) {
		return fmt.Errorf("specified version does not exist in remote llpkg: %s@%s", mod, ver)
	}
	if err := runGoGetWithFlags(flags, mod+"@"+ver); err != nil {
		return err
	}
	return runGoModTidy()
}

// Validate name@Version on llpkg remote and fetch it
func ensureLLPkgByNameVersionWithFlags(name, ver string, flags []string) error {
	mod := llpkgPrefix + name
	// If the module already exists locally, still write explicit version into go.mod to pin it
	if inLocal(mod) {
		if err := runGoGetWithFlags(flags, mod+"@"+ver); err != nil {
			return err
		}
		if err := installPythonPackage(name); err != nil {
			return fmt.Errorf("failed to install Python package: %w", err)
		}
		return runGoModTidy()
	}
	vers, err := listModuleVersions(mod)
	if err != nil {
		return err
	}
	if !contains(vers, ver) {
		printLLPygHint()
		return fmt.Errorf("specified version does not exist in remote llpkg: %s@%s", mod, ver)
	}
	if err := runGoGetWithFlags(flags, mod+"@"+ver); err != nil {
		return err
	}
	if err := installPythonPackage(name); err != nil {
		return fmt.Errorf("failed to install Python package: %w", err)
	}
	return runGoModTidy()
}

type listMod struct {
	Path     string   `json:"Path"`
	Versions []string `json:"Versions"`
}

func listModuleVersions(mod string) ([]string, error) {
	cmd := exec.Command("go", "list", "-m", "-versions", "-json", mod)
	var out bytes.Buffer
	cmd.Stdout, cmd.Stderr = &out, os.Stderr
	if err := cmd.Run(); err != nil {
		return nil, fmt.Errorf("go list versions failed for %s: %w", mod, err)
	}
	var m listMod
	if err := json.Unmarshal(out.Bytes(), &m); err != nil {
		return nil, err
	}
	return m.Versions, nil
}

// func listModuleVersionsLocal(mod string) ([]string, error) {
// 	cmd := exec.Command("go", "list", "-m", "-versions", "-json", mod)
// 	cmd.Env = append(os.Environ(), "GOPROXY=off")
// 	var out bytes.Buffer
// 	cmd.Stdout, cmd.Stderr = &out, os.Stderr
// 	if err := cmd.Run(); err != nil {
// 		return nil, fmt.Errorf("go list versions failed for %s: %w", mod, err)
// 	}
// 	var m listMod
// 	if err := json.Unmarshal(out.Bytes(), &m); err != nil {
// 		return nil, err
// 	}
// 	return m.Versions, nil
// }

func contains(ss []string, want string) bool {
	for _, s := range ss {
		if s == want {
			return true
		}
	}
	return false
}

// Offline-only check if it already exists locally (workspace or GOMODCACHE)
func inLocal(importPath string) bool {
	cmd := exec.Command("go", "list", importPath)
	cmd.Env = append(os.Environ(), "GOPROXY=off")
	var out, errb bytes.Buffer
	cmd.Stdout, cmd.Stderr = &out, &errb
	return cmd.Run() == nil
}

func runGoGet(spec string) error { return runGoGetWithFlags(nil, spec) }

func runGoGetWithFlags(flags []string, spec string) error {
	args := append([]string{"get"}, flags...)
	args = append(args, spec)
	cmd := exec.Command("go", args...)
	cmd.Stdout, cmd.Stderr = os.Stdout, os.Stderr
	return cmd.Run()
}

func runGoModTidy() error {
	cmd := exec.Command("go", "mod", "tidy")
	cmd.Stdout, cmd.Stderr = os.Stdout, os.Stderr
	return cmd.Run()
}

func workCache() string {
	base := os.Getenv("XDG_CACHE_HOME")
	if base == "" {
		home, _ := os.UserHomeDir()
		if runtime.GOOS == "windows" {
			return filepath.Join(home, "AppData", "Local", "llgo", "cache")
		}
		return filepath.Join(home, ".cache", "llgo")
	}
	return filepath.Join(base, "llgo")
}

func printLLPygHint() {
	fmt.Fprintln(os.Stderr, "llgo get: llpyg toolchain is required to generate Python bindings.")
	fmt.Fprintln(os.Stderr, " - Option 1: Submit a PR to the official repository to integrate the automated workflow (recommended).")
	fmt.Fprintln(os.Stderr, " - Option 2: Install the llpyg toolchain locally and retry:")
	fmt.Fprintln(os.Stderr, "     go install github.com/goplus/llgo/chore/llpyg@latest")
}

// installPythonPackage installs the Python package
func installPythonPackage(packageName string) error {
	installer := pyinstaller.NewInstaller()
	return installer.InstallPackage(packageName)
}

// Install the corresponding Python package
// if err := installPythonPackage(name); err != nil {
// 	return fmt.Errorf("failed to install Python package: %w", err)
// }
