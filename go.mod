module github.com/wicanr2/softworld_san1_remake

go 1.24.0

toolchain go1.24.13

// 建置容器是 --network none，這些模組事先放進 workplace/gomodcache
// （gitignore）。見 README 的建置說明。
require golang.org/x/text v0.29.0
