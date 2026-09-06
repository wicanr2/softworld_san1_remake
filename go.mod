module github.com/wicanr2/softworld_san1_remake

go 1.24.0

// 建置容器是 --network none。模組事先放進 workplace/gomodcache（gitignore），
// tools/go.sh 把那份 cache 當成 file:// proxy 用。見 README 的建置說明。
require (
	github.com/hajimehoshi/ebiten/v2 v2.9.9
	golang.org/x/text v0.29.0
)

require (
	github.com/ebitengine/gomobile v0.0.0-20250923094054-ea854a63cce1 // indirect
	github.com/ebitengine/hideconsole v1.0.0 // indirect
	github.com/ebitengine/oto/v3 v3.4.0 // indirect
	github.com/ebitengine/purego v0.9.0 // indirect
	github.com/jezek/xgb v1.1.1 // indirect
	golang.org/x/sync v0.17.0 // indirect
	golang.org/x/sys v0.36.0 // indirect
)
