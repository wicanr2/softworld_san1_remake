//go:build ignore

// 正式提示與合法築城路徑的診斷參考；不代替正常 GUI 或原版 oracle。
package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"github.com/wicanr2/softworld_san1_remake/internal/ai"
	"github.com/wicanr2/softworld_san1_remake/internal/assets"
	"github.com/wicanr2/softworld_san1_remake/internal/font"
	"github.com/wicanr2/softworld_san1_remake/internal/game"
	"github.com/wicanr2/softworld_san1_remake/internal/i18n"
	"github.com/wicanr2/softworld_san1_remake/internal/session"
	"github.com/wicanr2/softworld_san1_remake/internal/state"
	"github.com/wicanr2/softworld_san1_remake/internal/ui"
	"image/png"
	"os"
	"path/filepath"
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
	emit := func(name string, lines []string, main bool) {
		c := ui.NewCanvasPx(176, 96, face)
		background := 3
		if main {
			background = 2
		}
		c.Fill(assets.EGAPalette[background])
		for row, line := range lines {
			c.DrawTextPx(0, row*16, line, assets.EGAPalette[14])
		}
		fp, err := os.Create(filepath.Join(*out, name+"-reference.png"))
		if err != nil {
			panic(err)
		}
		if err = png.Encode(fp, c.Img); err != nil {
			panic(err)
		}
		fp.Close()
	}
	emit("retreat-list", []string{i18n.S("bat.retreatTo")}, false)
	emit("new-governor", ui.MessageLines(i18n.S("ask.newGovernor"), 22), true)
	plans := []any{}
	for _, edition := range []string{"base", "plus"} {
		folder, ed, mode := "三國演義", state.EditionBase, ai.ModeBase
		if edition == "plus" {
			folder, ed, mode = "三國演義1加強版", state.EditionPlus, ai.ModePlus
		}
		var raw [3][]byte
		for i, ext := range []string{".NAM", ".IDX", ".GRP"} {
			b, err := os.ReadFile(filepath.Join("/orig", folder, "DATA2"+ext))
			if err != nil {
				panic(err)
			}
			raw[i] = b
		}
		c, err := assets.OpenContainer(raw[0], raw[1], raw[2])
		if err != nil {
			panic(err)
		}
		sc, err := state.LoadScenario(c, state.Scenario3)
		if err != nil {
			panic(err)
		}
		g, err := game.New(sc, 1, 5, ed)
		if err != nil {
			panic(err)
		}
		brain, err := ai.New(mode)
		if err != nil {
			panic(err)
		}
		s := session.New(g, brain, 1)
		waiting := []any{}
		at := s.AdvanceToHuman(1000)
		for step := 0; step < 42; step++ {
			p, gov := g.Prefecture(at), g.Governor(at)
			if p == nil || gov == nil {
				panic("沒有玩家停點")
			}
			emit(fmt.Sprintf("fort-%s-waiting%d", edition, at), ui.MessageLines(i18n.Sf("ask.main", ui.NameField(g.Lord(1).Name), at, p.Name), 22), true)
			waiting = append(waiting, map[string]any{"prefecture": at, "name": p.Name, "governor_portrait": fmt.Sprintf("F%03d", gov.Portrait)})
			if at == 15 {
				break
			}
			if err = s.Do(game.RestOrder{At: at}); err != nil {
				panic(err)
			}
			at = s.AdvanceToHuman(1000)
		}
		if at != 15 {
			panic("尚未到洛陽")
		}
		p := g.Prefecture(at)
		wise := g.PickRoster(at, game.PickWise, game.PickByIntel)
		if p.Gold < game.FortCost(p.PriceLevel) || p.Forts >= game.MaxForts || len(wise) == 0 {
			panic("築城前置條件未滿")
		}
		type node struct {
			col, row int
			keys     []string
		}
		queue := []node{{}}
		seen := map[[2]int]bool{{0, 0}: true}
		var chosen *node
		for len(queue) > 0 {
			n := queue[0]
			queue = queue[1:]
			if game.CanBuildFortOn(p.BattleField[n.row*12+n.col]) {
				chosen = &n
				break
			}
			for key := byte('1'); key <= '6'; key++ {
				col, row := game.FortSpotStep(n.col, n.row, key, p.BattleField)
				pos := [2]int{col, row}
				if seen[pos] {
					continue
				}
				seen[pos] = true
				keys := append(append([]string{}, n.keys...), string(key))
				queue = append(queue, node{col, row, keys})
			}
		}
		if chosen == nil {
			panic("沒有合法游標路徑")
		}
		cell := chosen.row*12 + chosen.col + 1
		before := p.Forts
		if err = s.Do(game.BuildFortOrder{At: at, General: wise[0].Index, Cell: cell}); err != nil || p.Forts != before+1 {
			panic(fmt.Sprintf("築城失敗 %v", err))
		}
		plan := map[string]any{"edition": edition, "scenario": "003", "player": "曹操", "waiting": waiting, "target": 15, "general": wise[0].Index, "general_name": wise[0].Name, "cell": cell, "cursor_keys": chosen.keys, "forts_before": before, "forts_after": p.Forts, "gold_after": p.Gold, "scope": "正式新局、休息與合法築城的診斷；不是正常 GUI 或原版 oracle"}
		plans = append(plans, plan)
		fmt.Printf("%s: %v；%s 築城 cell=%d keys=%v\n", edition, waiting, wise[0].Name, cell, chosen.keys)
	}
	b, err := json.MarshalIndent(plans, "", "  ")
	if err != nil {
		panic(err)
	}
	if err = os.WriteFile(filepath.Join(*out, "fort-plan.json"), append(b, '\n'), 0644); err != nil {
		panic(err)
	}
}
