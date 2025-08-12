package main

import (
	_ "unsafe"

	"github.com/goplus/lib/py"
	"github.com/goplus/lib/py/std"
)

//go:linkname Add PyNumber_Add
func Add(__llgo_va_list ...interface{}) *py.Object

// Numpy 封装 numpy 操作
type Numpy struct{}

func (n Numpy) Add(a, b *py.Object) *py.Object {
	np := py.Import(py.Str("numpy"))
	addFunc := np.GetAttr(py.Str("add"))
	return addFunc.Call(py.Tuple(a, b), nil)
}

func main() {
	a := py.List(
		py.List(1.0, 2.0, 3.0),
		py.List(4.0, 5.0, 6.0),
		py.List(7.0, 8.0, 9.0),
	)
	b := py.List(
		py.List(9.0, 8.0, 7.0),
		py.List(6.0, 5.0, 4.0),
		py.List(3.0, 2.0, 1.0),
	)
	x := Add(a, b)
	std.Print(py.Str("a+b ="), x)

	np := Numpy{}

	np_x := np.Add(a, b)
	std.Print(py.Str("a+b ="), np_x)
}
