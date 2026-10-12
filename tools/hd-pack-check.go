//go:build ignore

// 正式載入器檢查本機完整 B 包；只在 Docker 中執行。
package main

import (
	"encoding/json"
	"fmt"
	"image/png"
	"os"
	"path/filepath"

	"github.com/wicanr2/softworld_san1_remake/internal/assets"
	"github.com/wicanr2/softworld_san1_remake/internal/ui"
)

func main() {
	if len(os.Args) != 3 && len(os.Args) != 4 {
		panic("用法：go run tools/hd-pack-check.go <hd-assets> <org_game> [新增肖像驗證圖目錄]")
	}
	b, err := os.ReadFile(filepath.Join(os.Args[1], "manifest.json"))
	if err != nil {
		panic(err)
	}
	var m ui.HDManifest
	if err = json.Unmarshal(b, &m); err != nil {
		panic(err)
	}
	if len(m.Entries) != 904 {
		panic("完整包必須有 904 筆")
	}
	expandedCount := 0
	if b, err := os.ReadFile(filepath.Join(os.Args[1], "expanded-portraits.json")); err == nil {
		var m struct {
			Schema  int               `json:"schema"`
			Entries []json.RawMessage `json:"entries"`
		}
		if err = json.Unmarshal(b, &m); err != nil || m.Schema != 1 || len(m.Entries) > 220 {
			panic("擴充肖像目錄不符")
		}
		expandedCount = len(m.Entries)
	} else if !os.IsNotExist(err) {
		panic(err)
	}
	for _, ed := range []struct{ name, folder string }{{"base", "三國演義"}, {"plus", "三國演義1加強版"}} {
		cs := map[string]*assets.Container{}
		for _, name := range []string{"DATA1", "DATA2", "DATA3"} {
			var data [3][]byte
			for i, ext := range []string{".NAM", ".IDX", ".GRP"} {
				data[i], err = os.ReadFile(filepath.Join(os.Args[2], ed.folder, name+ext))
				if err != nil {
					panic(err)
				}
			}
			cs[name], err = assets.OpenContainer(data[0], data[1], data[2])
			if err != nil {
				panic(err)
			}
		}
		p, err := ui.LoadHDPack(os.Args[1], ed.name, cs)
		if err != nil {
			panic(err)
		}
		if p.Count != 452 || p.ExpandedCount != expandedCount || len(p.Warnings) != 0 {
			panic(fmt.Sprintf("%s count=%d warnings=%v", ed.name, p.Count, p.Warnings))
		}
		fmt.Printf("%s：452/452，新增肖像 %d/220，警告 0\n", ed.name, p.ExpandedCount)
		if len(os.Args) == 4 {
			if err := os.MkdirAll(os.Args[3], 0o755); err != nil {
				panic(err)
			}
			c := &ui.Canvas{HD: p}
			for n := 0; n < 1000; n++ {
				im := c.HighImage(assets.ExpandedPortrait(n))
				if im == nil {
					continue
				}
				f, err := os.Create(filepath.Join(os.Args[3], fmt.Sprintf("%s-%04d.png", ed.name, n)))
				if err != nil {
					panic(err)
				}
				err = png.Encode(f, im)
				closeErr := f.Close()
				if err != nil || closeErr != nil {
					panic(fmt.Sprintf("驗證圖寫入失敗：%v %v", err, closeErr))
				}
			}
		}
	}
}
