//go:build ignore

// 補出素材總覽的原版地形，沿用正式 BattleTiles 解碼器；只在 Docker 執行。
package main

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"image/png"
	"os"
	"path/filepath"

	"github.com/wicanr2/softworld_san1_remake/internal/assets"
)

type tileRecord struct {
	Path         string `json:"path"`
	From         string `json:"from"`
	Container    string `json:"container"`
	SourceSHA256 string `json:"source_sha256"`
	OutputSHA256 string `json:"output_sha256"`
}

func main() {
	if len(os.Args) != 3 {
		panic("用法：go run tools/asset-overview-tiles.go <org_game> <本機輸出目錄>")
	}
	for _, ed := range []struct{ name, folder string }{{"base", "三國演義"}, {"plus", "三國演義1加強版"}} {
		root := filepath.Join(os.Args[1], ed.folder)
		var data [3][]byte
		for i, ext := range []string{".NAM", ".IDX", ".GRP"} {
			b, err := os.ReadFile(filepath.Join(root, "DATA1"+ext))
			must(err)
			data[i] = b
		}
		c, err := assets.OpenContainer(data[0], data[1], data[2])
		must(err)
		tiles, err := assets.BattleTiles(c)
		must(err)
		i, ok := c.ByName("EICON.GRP")
		if !ok {
			panic("缺少 EICON.GRP")
		}
		raw := c.Data(i)
		stride := len(raw) / len(tiles)
		out := filepath.Join(os.Args[2], ed.name)
		must(os.MkdirAll(filepath.Join(out, "terrain"), 0755))
		var records []tileRecord
		for n, im := range tiles {
			name := fmt.Sprintf("terrain/EICON%02d.png", n)
			f, err := os.Create(filepath.Join(out, name))
			must(err)
			must(png.Encode(f, im.RGBA()))
			must(f.Close())
			b, err := os.ReadFile(filepath.Join(out, name))
			must(err)
			records = append(records, tileRecord{name, fmt.Sprintf("EICON.GRP#%02d", n), "DATA1", fmt.Sprintf("%x", sha256.Sum256(raw[n*stride:(n+1)*stride])), fmt.Sprintf("%x", sha256.Sum256(b))})
		}
		b, err := json.MarshalIndent(map[string]any{"files": records}, "", "  ")
		must(err)
		must(os.WriteFile(filepath.Join(out, "terrain.json"), append(b, '\n'), 0644))
		fmt.Printf("%s：%d 張原版地形\n", ed.name, len(tiles))
	}
}

func must(err error) {
	if err != nil {
		panic(err)
	}
}
