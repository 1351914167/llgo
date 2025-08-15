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
	Short:     "检查本地与云端 llpkg, 若存在则通过 Go Modules 获取依赖",
	Run:       run,
}

// removed: shortSpecRe

func run(cmd *base.Command, args []string) {
	if err := Main(args); err != nil {
		fmt.Fprintln(os.Stderr, "llgo get:", err)
		os.Exit(1)
	}
}

// Main 提供给 .gox 命令直接调用
func Main(args []string) error {
	// 按“-参数... 后跟模块列表”的方式解析
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

// 处理一个参数（可能是 module[@version] 或 name@Version）
func processModuleArg(arg string, flags []string) error {
	name, version, _ := strings.Cut(arg, "@")
	if name == "" {
		return fmt.Errorf("invalid module path: %s", arg)
	}
	// 在 processModuleArg 中：
	if strings.Contains(name, "/") {
		return handleModuleSpecWithFlags(name, version, flags)
	}
	if version == "" {
		// 无版本的短名 → 视为 llpkg/<name> 且使用 latest
		return handleModuleSpecWithFlags(llpkgPrefix+name, "", flags)
	}
	return ensureLLPkgByNameVersionWithFlags(name, version, flags)
}

// 处理形如 module[@version] 的请求（支持 flags）
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

	// llpkg 模块：如果指定版本，先校验云端是否存在；未指定则取 latest
	if ver == "" {
		if inLocal(mod) {
			vers, _ := listModuleVersionsLocal(mod) // GOPROXY=off
			if len(vers) > 0 {
				return runGoGetWithFlags(flags, mod+"@"+vers[len(vers)-1])
			}
		}
		if err := runGoGetWithFlags(flags, mod+"@latest"); err != nil {
			printLLPygHint()
			return fmt.Errorf("指定版本不存在于云端 llpkg: %s@%s", mod, "latest")
		}
		// 新增：远端拉取成功后安装 Python 包
		pkgName := strings.TrimPrefix(mod, llpkgPrefix)
		if err := installPythonPackage(pkgName); err != nil {
			return fmt.Errorf("安装 Python 包失败: %w", err)
		}
		return runGoModTidy()
	}
	vers, err := listModuleVersions(mod) // 云端版本列表
	if err != nil {
		return err
	}
	if !contains(vers, ver) {
		return fmt.Errorf("指定版本不存在于云端 llpkg: %s@%s", mod, ver)
	}
	if err := runGoGetWithFlags(flags, mod+"@"+ver); err != nil {
		return err
	}
	return runGoModTidy()
}

// 直接将 name@Version 在 llpkg 云端校验并获取
func ensureLLPkgByNameVersionWithFlags(name, ver string, flags []string) error {
	mod := llpkgPrefix + name
	// 如果本地已有该模块，仍用明确版本写入 go.mod，确保锁定
	if inLocal(mod) {
		if err := runGoGetWithFlags(flags, mod+"@"+ver); err != nil {
			return err
		}
		if err := installPythonPackage(name); err != nil {
			return fmt.Errorf("安装 Python 包失败: %w", err)
		}
		return runGoModTidy()
	}
	vers, err := listModuleVersions(mod)
	if err != nil {
		return err
	}
	if !contains(vers, ver) {
		printLLPygHint()
		return fmt.Errorf("指定版本不存在于云端 llpkg: %s@%s", mod, ver)
	}
	if err := runGoGetWithFlags(flags, mod+"@"+ver); err != nil {
		return err
	}
	if err := installPythonPackage(name); err != nil {
		return fmt.Errorf("安装 Python 包失败: %w", err)
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

func listModuleVersionsLocal(mod string) ([]string, error) {
	cmd := exec.Command("go", "list", "-m", "-versions", "-json", mod)
	cmd.Env = append(os.Environ(), "GOPROXY=off")
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

func contains(ss []string, want string) bool {
	for _, s := range ss {
		if s == want {
			return true
		}
	}
	return false
}

// 仅离线检查是否已在本地（工作区或 GOMODCACHE）
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
	fmt.Fprintln(os.Stderr, "llgo get: 需要 llpyg 工具链以生成 Python 绑定。")
	fmt.Fprintln(os.Stderr, " - 方案一：向官方仓库提交 PR 以集成自动流程（建议）。")
	fmt.Fprintln(os.Stderr, " - 方案二：本地安装 llpyg 工具链后重试：")
	fmt.Fprintln(os.Stderr, "     go install github.com/goplus/llgo/chore/llpyg@latest")
}

// installPythonPackage 安装 Python 包
func installPythonPackage(packageName string) error {
	installer := pyinstaller.NewInstaller()
	return installer.InstallPackage(packageName)
}

// 安装对应 Python 包
// if err := installPythonPackage(name); err != nil {
// 	return fmt.Errorf("安装 Python 包失败: %w", err)
// }
