//go:build ignore

// 姓名區完整字模參考，只供 GUI 回讀，不建立或注入遊戲狀態。
package main

import (
	"encoding/json"
	"flag"
	"image/color"
	"image/png"
	"os"
	"path/filepath"
	"strings"

	"github.com/wicanr2/softworld_san1_remake/internal/cells"
	"github.com/wicanr2/softworld_san1_remake/internal/font"
	"github.com/wicanr2/softworld_san1_remake/internal/i18n"
	"github.com/wicanr2/softworld_san1_remake/internal/ui"
)

func uiMarkerInk() color.RGBA { return color.RGBA{255, 255, 255, 255} }

func main() {
	out := flag.String("out", "", "已存在的私人輸出目錄")
	flag.Parse()
	if s, err := os.Stat(*out); err != nil || !s.IsDir() {
		panic("需要既有輸出目錄")
	}
	load := func(name string, height int) *font.Face {
		f, err := os.Open(filepath.Join("fonts", name))
		if err != nil {
			panic(err)
		}
		defer f.Close()
		face, err := font.ParseHexGz(f, height)
		if err != nil {
			panic(err)
		}
		return face
	}
	face, small := load("unifont.hex.gz", 16), load("ascii6x10.hex.gz", 10)
	var records []map[string]any
	for index, raw := range []string{"呂布", "陳宮", "曹操", "夏侯惇", "夏侯淵"} {
		for _, locale := range i18n.Locales() {
			name := i18n.PersonNameFor(locale, raw)
			c := ui.NewCanvasPx(48, 16, face)
			c.SetSmallFace(small)
			if locale != i18n.En && cells.Width(name) == 4 {
				name = " " + name + " "
			}
			if cells.Width(name)*8 <= 48 || !c.FitsSmall(name) {
				c.DrawTextPx(0, 0, name, uiMarkerInk())
			} else if cells.Width(name)*6 <= 47 {
				c.DrawSmallTextPx(0, 3, name, uiMarkerInk())
			} else {
				groups := [][]int{{1, 2}, {3}, {4}, {5, 6}, {7}, {8}, {9}}
				for row, line := range cells.Wrap(name, 7) {
					for i, r := range strings.TrimSpace(line) {
						g, _ := small.Glyph(r)
						for y, sourceRows := range groups {
							for x := 0; x < 6; x++ {
								for _, gy := range sourceRows {
									if g.At(x, gy) {
										c.Img.SetRGBA(i*6+x, row*8+y, uiMarkerInk())
									}
								}
							}
						}
					}
				}
			}
			file := "marker-" + string(rune('0'+index)) + "-" + string(locale) + "-reference.png"
			f, err := os.OpenFile(filepath.Join(*out, file), os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0644)
			if err != nil {
				panic(err)
			}
			if err = png.Encode(f, c.Img); err != nil {
				panic(err)
			}
			f.Close()
			records = append(records, map[string]any{"raw": raw, "locale": locale, "name": strings.TrimSpace(name), "file": file})
		}
	}
	b, err := json.MarshalIndent(records, "", "  ")
	if err != nil {
		panic(err)
	}
	if err = os.WriteFile(filepath.Join(*out, "marker-reference.json"), append(b, '\n'), 0644); err != nil {
		panic(err)
	}
}
