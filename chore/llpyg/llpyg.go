/*
 * Copyright (c) 2024 The GoPlus Authors (goplus.org). All rights reserved.
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

package main

import (
	"bytes"
	"encoding/json"
	"flag"
	"fmt"
	"go/ast"
	"go/token"
	"go/types"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/goplus/gogen"
	"github.com/goplus/llgo/chore/llpyg/pysig"
	"github.com/goplus/llgo/ssa"
)

type symbol struct {
	Name string `json:"name"`
	Type string `json:"type"`
	Doc  string `json:"doc"`
	Sig  string `json:"sig"`
	URL  string `json:"url"`
}

type module struct {
	Name  string    `json:"name"`
	Items []*symbol `json:"items"`
}

func pydump(pyLib string) (mod module) {
	var out bytes.Buffer
	cmd := exec.Command("pydump", pyLib)
	cmd.Stdout = &out
	cmd.Stderr = os.Stderr
	cmd.Run()

	json.Unmarshal(out.Bytes(), &mod)
	return
}

func pysigfetch(pyLib string, names []string) (mod module) {
	var out bytes.Buffer
	cmd := exec.Command("pysigfetch", pyLib, "-")
	cmd.Stdin = strings.NewReader(strings.Join(names, " "))
	cmd.Stdout = &out
	cmd.Stderr = os.Stderr
	cmd.Run()

	json.Unmarshal(out.Bytes(), &mod)
	return
}

func printSymbolDetails(mod *module, name string) {
	for _, item := range mod.Items {
		if item.Name == name {
			fmt.Printf("Name: %s\n", item.Name)
			fmt.Printf("Type: %s\n", item.Type)
			fmt.Printf("Sig: %s\n", item.Sig)
			fmt.Printf("URL: %s\n", item.URL)
			fmt.Println("-----")
			return
		}
	}
	fmt.Printf("Symbol '%s' not found\n", name)
}

func main() {
	outDirFlag := flag.String("o", "", "output directory for generated Go bindings")
	// compatibility: support --out as alias
	outDirAlias := flag.String("out", "", "alias of -o")
	flag.Parse()

	args := flag.Args()
	if len(args) < 1 {
		fmt.Fprintln(os.Stderr, "Usage: llpyg <pythonLib> [-o outputDir]")
		return
	}
	pyLib := args[0]

	//解析python包名，类型，方法的签名(参数&返回值), 文档
	mod := pydump(pyLib)
	if mod.Name != pyLib {
		log.Printf("import module %s failed\n", pyLib)
		os.Exit(1)
	}
	// printSymbolDetails(&mod, "dot")
	for _, item := range mod.Items {
		fmt.Printf("Name: %s, Type: %s, Sig: %s, URL: %s\n", item.Name, item.Type, item.Sig, item.URL)
	}
	// 打印Name为"random"的所有信息
	for _, item := range mod.Items {
		if item.Name == "random" {
			fmt.Printf("Name: %s\nType: %s\nSig: %s\nURL: %s\n", item.Name, item.Type, item.Sig, item.URL)
		}
	}
	//新建go包，生成基础python
	pkg := gogen.NewPackage("", pkgName(pyLib), nil)
	pkg.Import("unsafe").MarkForceUsed(pkg)      // import _ "unsafe"
	py := pkg.Import("github.com/goplus/lib/py") // import "github.com/goplus/lib/py"

	f := func(cb *gogen.CodeBuilder) int {
		cb.Val("py." + mod.Name)
		return 1
	}

	defs := pkg.NewConstDefs(pkg.Types.Scope())
	defs.New(f, 0, 0, nil, "LLGoPackage")

	obj := py.Ref("Object").(*types.TypeName).Type().(*types.Named)
	objPtr := types.NewPointer(obj)
	ret := types.NewTuple(pkg.NewParam(0, "", objPtr))

	ctx := &context{pkg, obj, objPtr, ret, nil, py}
	ctx.genMod(pkg, &mod)
	skips := ctx.skips
	if n := len(skips); n > 0 {
		log.Printf("==> There are %d signatures not found, fetch from doc site\n", n)
		mod = pysigfetch(pyLib, skips)
		ctx.skips = skips[:0]
		ctx.genMod(pkg, &mod)
		if len(mod.Items) > 0 {
			skips = ctx.skips
		}
		if n := len(skips); n > 0 {
			log.Printf("==> Skip %d symbols:\n%v\n", n, skips)
		}
	}

	// 输出目录：优先使用 -o/--out 指定；未指定则使用 ./<mod.Name>
	outDir := strings.TrimSpace(*outDirFlag)
	if outDir == "" {
		out := strings.TrimSpace(*outDirAlias)
		if out != "" {
			outDir = out
		}
	}
	if outDir == "" {
		outDir = "./" + mod.Name
	}
	if err := os.MkdirAll(outDir, 0755); err != nil {
		log.Fatal(err)
	}
	file, err := os.Create(filepath.Join(outDir, mod.Name+".go"))
	if err != nil {
		log.Fatal(err)
	}
	defer file.Close()

	pkg.WriteTo(file)
}

func pkgName(pyLib string) string {
	if pos := strings.LastIndexByte(pyLib, '.'); pos >= 0 {
		return pyLib[pos+1:]
	}
	return pyLib
}

type context struct {
	pkg    *gogen.Package
	obj    *types.Named
	objPtr *types.Pointer
	ret    *types.Tuple
	skips  []string
	py     gogen.PkgRef
}

func (ctx *context) genMod(pkg *gogen.Package, mod *module) {
	for _, sym := range mod.Items {
		switch sym.Type {
		case "builtin_function_or_method", "function", "method", "ufunc", "method-wrapper":
			ctx.genFunc(pkg, sym)
		case "str", "float", "bool", "type", "dict", "tuple", "list", "object", "module",
			"int", "set", "frozenset", "flags", "bool_", "pybind11_type", "layout",
			"memory_format", "qscheme", "dtype", "tensortype", "ellipsis": // skip
		case "": // pysigfetch: page not found
			ctx.skips = append(ctx.skips, sym.Name)
		default:
			t := sym.Type
			if len(t) > 0 && (t[0] >= 'a' && t[0] <= 'z') && !strings.HasSuffix(t, "_info") {
				log.Panicln("unsupport type:", sym.Type)
			}
		}
	}
}

func (ctx *context) genFunc(pkg *gogen.Package, sym *symbol) {
	name, symSig := sym.Name, sym.Sig
	if len(name) == 0 || name[0] == '_' {
		return
	}
	if symSig == "<NULL>" {
		ctx.skips = append(ctx.skips, name)
		return
	}
	params, variadic := ctx.genParams(pkg, symSig)
	name = genName(name, -1)
	sig := types.NewSignatureType(nil, nil, nil, params, ctx.ret, variadic)
	fn := pkg.NewFuncDecl(token.NoPos, name, sig)
	list := ctx.genDoc(sym.Doc)
	if sym.URL != "" {
		if len(list) > 0 {
			list = append(list, emptyCommentLine)
		}
		list = append(list, genSee(sym.URL))
	}
	if len(list) > 0 {
		list = append(list, emptyCommentLine)
	}
	list = append(list, ctx.genLinkname(name, sym))
	fn.SetComments(pkg, &ast.CommentGroup{List: list})
	// fn.BodyStart(pkg).End()
}

func (ctx *context) genParams(pkg *gogen.Package, sig string) (*types.Tuple, bool) {
	args := pysig.Parse(sig)
	if len(args) == 0 {
		return nil, false
	}
	n := len(args)
	objPtr := ctx.objPtr
	list := make([]*types.Var, 0, n)
	for i := 0; i < n; i++ {
		name := args[i].Name
		if name == "/" {
			continue
		}
		if name == "*" || name == "\\*" {
			break
		}
		if strings.HasPrefix(name, "*") {
			if name[1] != '*' {
				list = append(list, ssa.VArg())
				return types.NewTuple(list...), true
			}
			return types.NewTuple(list...), false
		}
		list = append(list, pkg.NewParam(0, genName(name, 0), objPtr))
	}
	return types.NewTuple(list...), false
}

func genName(name string, idxDontTitle int) string {
	parts := strings.Split(name, "_")
	for i, part := range parts {
		if i != idxDontTitle && part != "" {
			if c := part[0]; c >= 'a' && c <= 'z' {
				part = string(c+'A'-'a') + part[1:]
			}
			parts[i] = part
		}
	}
	name = strings.Join(parts, "")
	switch name {
	case "default", "func", "var", "range", "":
		name += "_"
	}
	return name
}

func (ctx *context) genLinkname(name string, sym *symbol) *ast.Comment {
	return &ast.Comment{Text: "//go:linkname " + name + " py." + sym.Name}
}

func (ctx *context) genDoc(doc string) []*ast.Comment {
	if doc == "" {
		return make([]*ast.Comment, 0, 4)
	}
	lines := strings.Split(doc, "\n")
	list := make([]*ast.Comment, len(lines), len(lines)+4)
	for i, line := range lines {
		list[i] = &ast.Comment{Text: "// " + line}
	}
	return list
}

func genSee(url string) *ast.Comment {
	return &ast.Comment{Text: "// See " + url}
}

var (
	emptyCommentLine = &ast.Comment{Text: "//"}
)
