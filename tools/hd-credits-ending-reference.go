//go:build ignore

// 自然晚期存檔經正式規則續局，生成結束提示頁參照。此為顯示參照，不是正常 GUI 或原版 oracle。
package main

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"image/png"
	"os"
	"path/filepath"

	"github.com/wicanr2/softworld_san1_remake/internal/ai"
	"github.com/wicanr2/softworld_san1_remake/internal/assets"
	"github.com/wicanr2/softworld_san1_remake/internal/font"
	"github.com/wicanr2/softworld_san1_remake/internal/i18n"
	"github.com/wicanr2/softworld_san1_remake/internal/menu"
	"github.com/wicanr2/softworld_san1_remake/internal/state"
	"github.com/wicanr2/softworld_san1_remake/internal/ui"
)

func must(e error) {
	if e != nil {
		panic(e)
	}
}
func sha(b []byte) string { s := sha256.Sum256(b); return hex.EncodeToString(s[:]) }
func main() {
	out := os.Getenv("SAN1_HD_CREDITS_OUT") + "/ending-reference"
	if os.Getenv("SAN1_HD_CREDITS_OUT") == "" {
		panic("missing output")
	}
	if _, e := os.Stat(out); !os.IsNotExist(e) {
		panic("output exists")
	}
	must(os.Mkdir(out, 0755))
	inputs := map[string]string{}
	read := func(p string) []byte { b, e := os.ReadFile(p); must(e); inputs[p] = sha(b); return b }
	fontFile := "/src/fonts/unifont.hex.gz"
	f, e := os.Open(fontFile)
	must(e)
	face, e := font.ParseHexGz(f, 16)
	must(e)
	must(f.Close())
	read(fontFile)
	f, e = os.Open("/src/fonts/ascii6x10.hex.gz")
	must(e)
	small, e := font.ParseHexGz(f, ui.SmallH)
	must(e)
	must(f.Close())
	read("/src/fonts/ascii6x10.hex.gz")
	rows := []map[string]any{}
	for _, ed := range []string{"base", "plus"} {
		folder := "三國演義"
		mode := ai.ModeBase
		if ed == "plus" {
			folder = "三國演義1加強版"
			mode = ai.ModePlus
		}
		containers := map[string]*assets.Container{}
		for _, slot := range []string{"DATA1", "DATA2", "DATA3"} {
			root := filepath.Join("/orig", folder, slot)
			c, e := assets.OpenContainer(read(root+".NAM"), read(root+".IDX"), read(root+".GRP"))
			must(e)
			containers[slot] = c
		}
		fixture := filepath.Join("/src/workplace/credits-v44-fixture-r1", ed)
		var expected struct {
			Passed      bool
			LoadedFinal map[string]any `json:"loaded_final"`
		}
		must(json.Unmarshal(read(filepath.Join(fixture, "receipt.json")), &expected))
		if !expected.Passed {
			panic("fixture failed")
		}
		must(filepath.WalkDir(filepath.Join(fixture, "saves"), func(p string, d os.DirEntry, e error) error {
			if e != nil {
				return e
			}
			if !d.IsDir() {
				read(p)
			}
			return nil
		}))
		edition, e := state.ParseEdition(ed)
		must(e)
		m := menu.New(containers["DATA2"], edition, mode, filepath.Join(fixture, "saves"), 0)
		m.Confirm(1)
		s := m.Confirm(0)
		if s == nil || s.Over {
			panic("not a lawful pre-conclusion save")
		}
		cells := 0
		for !s.Over && cells < 45*24 {
			for s.Bubble() != nil {
				s.PopBubble()
			}
			s.AdvanceToHuman(1)
			cells++
		}
		if !s.Over {
			panic("natural conclusion did not occur")
		}
		mas, sta, gen, e := s.G.Tables()
		must(e)
		winner, unified := s.G.Winner()
		if !unified || float64(winner) != expected.LoadedFinal["winner"] {
			panic("natural unification outcome differs")
		}
		historicalTablesEqual := map[string]bool{
			"master":  sha(mas) == expected.LoadedFinal["master_sha256"],
			"state":   sha(sta) == expected.LoadedFinal["state_sha256"],
			"general": sha(gen) == expected.LoadedFinal["general_sha256"],
		}
		for s.Bubble() != nil {
			s.PopBubble()
		}
		art, e := ui.NewArtScreen(containers["DATA3"], containers["DATA1"], containers["DATA2"])
		must(e)
		pack, e := ui.LoadHDPack("/src/workplace/hd-assets-mapcursor-v50-r1", ed, containers)
		must(e)
		if len(pack.Warnings) != 0 {
			panic("HD warnings")
		}
		c := ui.NewCanvasPx(640, 408, face)
		c.SetSmallFace(small)
		c.HD = pack
		ui.DrawArtSession(c, art, s.G, s.Log, ui.View{Over: s.Over, Prompt: i18n.Sf("msg.demoEnd")})
		for _, high := range []bool{false, true} {
			name := ed + "-command-original.png"
			if high {
				name = ed + "-command-native.png"
			}
			im := c.Output(high)
			p := filepath.Join(out, name)
			f, e := os.Create(p)
			must(e)
			must(png.Encode(f, im))
			must(f.Close())
			b, e := os.ReadFile(p)
			must(e)
			rows = append(rows, map[string]any{"edition": ed, "high": high, "file": name, "sha256": sha(b), "width": im.Bounds().Dx(), "height": im.Bounds().Dy(), "rect": []int{0, 0, im.Bounds().Dx(), im.Bounds().Dy()}, "loaded_cells": cells, "over": s.Over, "unified": unified, "winner": winner, "date": s.G.Date, "historical_tables_equal": historicalTablesEqual, "master_sha256": sha(mas), "state_sha256": sha(sta), "general_sha256": sha(gen), "scope": "complete normal command page after natural conclusion and cleared normal event queue, formal renderer reference only"})
		}
	}
	for _, p := range []string{"tools/hd-credits-ending-reference.go", "internal/ui/artscreen.go", "internal/ui/canvas.go", "internal/ui/hd.go", "internal/ui/hd_panels.go", "internal/session/session.go", "cmd/san1/main.go", "internal/i18n/lang/zh-Hant.json"} {
		read(filepath.Join("/src", p))
	}
	read("/src/workplace/hd-assets-mapcursor-v50-r1/manifest.json")
	b, e := json.MarshalIndent(map[string]any{"passed": true, "overlay": false, "normal_GUI": false, "source_sha256": inputs, "rows": rows}, "", "  ")
	must(e)
	must(os.WriteFile(filepath.Join(out, "receipt.json"), append(b, '\n'), 0644))
}
