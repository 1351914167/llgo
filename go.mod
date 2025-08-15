module github.com/goplus/llgo

go 1.24.5

require (
	github.com/goplus/cobra v1.9.12 //gop:class
	github.com/goplus/gogen v1.19.0
	github.com/goplus/lib v0.2.0
	github.com/goplus/llgo/_xtool v0.0.0-00010101000000-000000000000
	github.com/goplus/llgo/runtime v0.0.0-00010101000000-000000000000
	github.com/goplus/llvm v0.8.3
	github.com/goplus/mod v0.17.1
	github.com/qiniu/x v1.15.1
	golang.org/x/tools v0.35.0
)

require (
	github.com/google/go-github/v69 v69.2.0 // indirect
	github.com/google/go-querystring v1.1.0 // indirect
	golang.org/x/mod v0.26.0 // indirect
	golang.org/x/sync v0.16.0 // indirect
)

replace github.com/goplus/llgo/runtime => ./runtime

replace github.com/goplus/llgo/_xtool => ./_xtool
