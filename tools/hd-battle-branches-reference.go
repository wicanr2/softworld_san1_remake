//go:build ignore

// 正式文字視窗的就緒辨識參考；不是玩家路徑或原版 oracle 收據。
package main

import (
	"crypto/sha256"
	"encoding/json"
	"flag"
	"fmt"
	"image/png"
	"os"
	"path/filepath"
	"strings"

	"github.com/wicanr2/softworld_san1_remake/internal/assets"
	"github.com/wicanr2/softworld_san1_remake/internal/battle"
	"github.com/wicanr2/softworld_san1_remake/internal/font"
	"github.com/wicanr2/softworld_san1_remake/internal/i18n"
	"github.com/wicanr2/softworld_san1_remake/internal/ui"
)

func main() {
	out := flag.String("out", "", "既有私人驗證目錄")
	flag.Parse()
	st, err := os.Stat(*out)
	if err != nil || !st.IsDir() {
		panic("需要既有輸出目錄")
	}
	f, err := os.Open("fonts/unifont.hex.gz")
	if err != nil {
		panic(err)
	}
	face, err := font.ParseHexGz(f, 16)
	f.Close()
	if err != nil {
		panic(err)
	}
	texts := map[string][]string{
		"waiting15":        ui.MessageLines(i18n.Sf("ask.main", ui.NameField("董卓"), 15, "洛陽"), 22),
		"waiting16":        ui.MessageLines(i18n.Sf("ask.main", ui.NameField("董卓"), 16, "京兆"), 22),
		"continue":         ui.MessageLines(i18n.S("ask.continue"), 22),
		"waiting14":        ui.MessageLines(i18n.Sf("ask.main", ui.NameField("董卓"), 14, "弘農"), 22),
		"camp":             ui.BattleWindowLines(ui.BattleCampWindow(15, "洛陽", battle.MainAttacker, battle.Centre, "呂布"), ui.BattleWindowCols, 6),
		"command":          strings.Split(i18n.S("bat.win.menu"), "\n"),
		"captive":          strings.Split(i18n.S("bat.captiveLines"), "|"),
		"skirmish":         strings.Split(i18n.S("skm.menu"), "|"),
		"skirmish-rest":    {i18n.S("skm.restConfirm")},
		"quick-direction":  ui.BattleWindowLines(ui.BattleDirWindow(battle.CmdQuick, nil), ui.BattleWindowCols, 6),
		"engage-direction": ui.BattleWindowLines(ui.BattleDirWindow(battle.CmdEngage, nil), ui.BattleWindowCols, 6),
	}
	outputs := map[string]any{}
	for name, lines := range texts {
		c := ui.NewCanvasPx(176, 96, face)
		c.Fill(assets.EGAPalette[3])
		if strings.HasPrefix(name, "waiting") || name == "continue" {
			c.Fill(assets.EGAPalette[2])
		}
		for row, line := range lines {
			ink := assets.EGAPalette[14]
			if name == "skirmish-rest" {
				ink = assets.EGAPalette[15]
			}
			c.DrawTextPx(0, row*16, line, ink)
		}
		path := filepath.Join(*out, name+"-reference.png")
		file, err := os.Create(path)
		if err != nil {
			panic(err)
		}
		if err = png.Encode(file, c.Img); err != nil {
			panic(err)
		}
		file.Close()
		data, err := os.ReadFile(path)
		if err != nil {
			panic(err)
		}
		outputs[name] = map[string]any{"file": filepath.Base(path), "sha256": fmt.Sprintf("%x", sha256.Sum256(data)), "lines": lines}
	}
	data, err := json.MarshalIndent(map[string]any{"scope": "正式提示的靜態就緒辨識參考；不是正常玩家或原版 oracle", "outputs": outputs}, "", "  ")
	if err != nil {
		panic(err)
	}
	if err = os.WriteFile(filepath.Join(*out, "prompt-reference.json"), append(data, '\n'), 0644); err != nil {
		panic(err)
	}
}
