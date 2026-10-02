// san1hdpack 依 spec/021 準備首批私人 B 高清包，不加入發行產物。
package main

import (
	"crypto/sha256"
	"encoding/json"
	"flag"
	"fmt"
	"image/png"
	"os"
	"path/filepath"

	"github.com/wicanr2/softworld_san1_remake/internal/assets"
	"github.com/wicanr2/softworld_san1_remake/internal/ui"
)

func main() {
	base := flag.String("root", "", "原版資料目錄")
	plus := flag.String("peer-root", "", "加強版資料目錄")
	input := flag.String("input", "workplace/hd-preview", "4× 首批素材目錄")
	out := flag.String("out", "workplace/hd-assets", "私人輸出目錄")
	flag.Parse()
	if *base == "" || *plus == "" {
		panic("需提供兩版來源")
	}
	if err := os.MkdirAll(*out, 0755); err != nil {
		panic(err)
	}
	m := ui.HDManifest{Schema: 1, Style: "b", Scale: 4}
	for _, key := range []string{"F000", "F005", "F228", "SCG01"} {
		data, err := os.ReadFile(filepath.Join(*input, "base-"+key+"-prepared-4x.png"))
		if err != nil {
			panic(err)
		}
		f, err := os.Open(filepath.Join(*input, "base-"+key+"-prepared-4x.png"))
		if err != nil {
			panic(err)
		}
		config, err := png.DecodeConfig(f)
		f.Close()
		if err != nil {
			panic(err)
		}
		name := key + ".FAC"
		w, h := 256, 320
		if key == "SCG01" {
			name = key + ".IMG"
			w, h = 704, 384
		}
		if config.Width != w || config.Height != h {
			panic("首批素材尺寸不符")
		}
		if err := os.WriteFile(filepath.Join(*out, key+".png"), data, 0644); err != nil {
			panic(err)
		}
		for _, ed := range []struct{ edition, root string }{{"base", *base}, {"plus", *plus}} {
			c := container(ed.root, "DATA3")
			i, ok := c.ByName(name)
			if !ok {
				panic(name + " 來源缺失")
			}
			m.Entries = append(m.Entries, ui.HDEntry{Edition: ed.edition, Container: "DATA3", Name: name,
				SourceSHA256: fmt.Sprintf("%x", sha256.Sum256(c.Data(i))), File: key + ".png",
				SHA256: fmt.Sprintf("%x", sha256.Sum256(data)), Width: w, Height: h})
		}
	}
	b, err := json.MarshalIndent(m, "", "  ")
	if err != nil {
		panic(err)
	}
	if err := os.WriteFile(filepath.Join(*out, "manifest.json"), append(b, '\n'), 0644); err != nil {
		panic(err)
	}
	for _, ed := range []struct{ edition, root string }{{"base", *base}, {"plus", *plus}} {
		pack, err := ui.LoadHDPack(*out, ed.edition, map[string]*assets.Container{"DATA3": container(ed.root, "DATA3")})
		if err != nil || pack.Count != 4 || len(pack.Warnings) > 0 {
			panic(fmt.Sprintf("%s 素材包驗證失敗：%v / %+v", ed.edition, err, pack))
		}
		fmt.Printf("%s: 4 張高清圖驗證通過\n", ed.edition)
	}
}

func container(root, key string) *assets.Container {
	read := func(ext string) []byte {
		b, err := os.ReadFile(filepath.Join(root, key+ext))
		if err != nil {
			panic(err)
		}
		return b
	}
	c, err := assets.OpenContainer(read(".NAM"), read(".IDX"), read(".GRP"))
	if err != nil {
		panic(err)
	}
	return c
}
