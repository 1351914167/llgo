// llgo/cmd/internal/get/get.go
package get

import (
	"fmt"
	"os"
	"os/exec"
	"strings"

	"github.com/goplus/llgo/cmd/internal/base"
)

var Cmd = &base.Command{
	UsageLine: "llgo get [-t -u -v] [build flags] [modules...]",
	Short:     "Fetch modules with Go Modules (no Python actions)",
	Run:       run,
}

func run(cmd *base.Command, args []string) {
	if err := Main(args); err != nil {
		fmt.Fprintln(os.Stderr, "llgo get:", err)
		os.Exit(1)
	}
}

// Main 仅执行 go get/go mod tidy，不做任何 Python 安装/检测
func Main(args []string) error {
	// 解析为 “flags... + modules...”
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

// 处理单个参数：module[@version] 或 简写 name[@version]（映射到 llpkg 前缀）
func processModuleArg(arg string, flags []string) error {
	name, version, _ := strings.Cut(arg, "@")
	if name == "" {
		return fmt.Errorf("invalid module path: %s", arg)
	}

	// 含 / 视为完整 module path；否则映射到 llpkg 前缀
	if strings.Contains(name, "/") {
		return handleModuleSpecWithFlags(name, version, flags)
	}
	if version == "" {
		return handleModuleSpecWithFlags(llpkgPrefix+name, "latest", flags)
	}
	return handleModuleSpecWithFlags(llpkgPrefix+name, version, flags)
}

// 仅执行 go get（带版本），随后 go mod tidy
func handleModuleSpecWithFlags(mod string, ver string, flags []string) error {
	spec := mod
	if ver != "" {
		spec = mod + "@" + ver
	}
	if err := runGoGetWithFlags(flags, spec); err != nil {
		return err
	}
	return runGoModTidy()
}

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
