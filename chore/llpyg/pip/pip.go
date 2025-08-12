package pip

import (
	"github.com/goplus/lib/py"
	_ "unsafe"
)

const LLGoPackage = "py.pip"
// This is an internal API only meant for use by pip's own console scripts.
//
//     For additional details, see https://github.com/pypa/pip/issues/7498.
//
//
//go:linkname Main py.main
func Main(args *py.Object) *py.Object
