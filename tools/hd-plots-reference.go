//go:build ignore

// 正式謀略的合法輸入與提示參考；不代替正常 GUI 或原版 oracle。
package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"image/png"
	"os"
	"path/filepath"

	"github.com/wicanr2/softworld_san1_remake/internal/ai"
	"github.com/wicanr2/softworld_san1_remake/internal/assets"
	"github.com/wicanr2/softworld_san1_remake/internal/font"
	"github.com/wicanr2/softworld_san1_remake/internal/game"
	"github.com/wicanr2/softworld_san1_remake/internal/i18n"
	"github.com/wicanr2/softworld_san1_remake/internal/menu"
	"github.com/wicanr2/softworld_san1_remake/internal/session"
	"github.com/wicanr2/softworld_san1_remake/internal/state"
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
		m := menu.New(c, ed, mode, "", 0)
		m.Confirm(0)
		m.Confirm(0)
		if len(m.Lords()) < 3 || m.Lords()[2] != 2 || m.Game().Lord(2).Name != "孫堅" {
			panic("正常選君主第三格不是孫堅")
		}
		fresh := func() (*game.State, *session.Session) {
			sc, err := state.LoadScenario(c, state.Scenario1)
			if err != nil {
				panic(err)
			}
			g, err := game.New(sc, 2, 5, ed)
			if err != nil {
				panic(err)
			}
			brain, err := ai.New(mode)
			if err != nil {
				panic(err)
			}
			return g, session.New(g, brain, 2)
		}
		g, s := fresh()
		at := s.AdvanceToHuman(1000)
		before := g.Date
		wise := g.PickRoster(at, game.PickWiseSub, game.PickByIntel)
		if at != 31 || len(wise) == 0 || wise[0].Name != "程普" || wise[0].Index != 37 {
			panic("開局軍師候選不符")
		}
		if err = s.Do(game.AppointChiefOrder{At: at, Target: wise[0].Index}); err != nil {
			panic(err)
		}
		at = s.AdvanceToHuman(1000)
		envoys := g.PickRoster(at, game.PickServing, game.PickByCharm)
		if at != 31 || g.Date == before || g.Chief(2) == nil || g.Chief(2).Index != 37 ||
			len(envoys) == 0 || envoys[0].Name != "孫堅" || envoys[0].Index != 15 {
			panic("任命後停點或使者不符")
		}
		enemy := func(id int) bool {
			p := g.Prefecture(id)
			return p != nil && p.Owned() && p.Owner != 2
		}
		if !enemy(2) || !enemy(3) || !g.Adjacent(2, 3) || g.Prefecture(2).Owner == g.Prefecture(3).Owner ||
			!enemy(27) || !enemy(29) || !g.Adjacent(27, 29) || !g.Adjacent(29, 31) ||
			g.Prefecture(27).Owner == g.Prefecture(29).Owner || g.Prefecture(31).Owner != 2 {
			panic("謀略選郡條件不符")
		}
		cvs := ui.NewCanvasPx(176, 96, face)
		cvs.Fill(assets.EGAPalette[2])
		prompt := i18n.Sf("ask.main", ui.NameField(g.Lord(2).Name), at, g.Prefecture(at).Name)
		for row, line := range ui.MessageLines(prompt, 22) {
			cvs.DrawTextPx(0, row*16, line, assets.EGAPalette[14])
		}
		fp, err := os.Create(filepath.Join(*out, "plots-"+edition+"-main-reference.png"))
		if err != nil {
			panic(err)
		}
		if err = png.Encode(fp, cvs.Img); err != nil {
			panic(err)
		}
		fp.Close()
		plan := map[string]any{"edition": edition, "scenario": "001", "player": "孫堅", "player_faction": 2,
			"lord_menu_position": 3, "difficulty": 5, "waiting": at, "date_before": before, "date_after": g.Date,
			"chief": wise[0], "envoy": envoys[0], "portrait": "F041", "state_injection": false,
			"SCG23": game.PlotPlan{Envoy: 15, At: 2, Strike: 3},
			"SCG18": game.PlotPlan{Envoy: 15, At: 27, Strike: 29, Ours: 31},
			"SCG22": game.PlotPlan{Envoy: 15, At: 2},
			"scope": "正式劇本、任命與合法謀略輸入的診斷；不是正常 GUI 或原版 oracle"}
		success, err := g.UsePlotPlan(at, game.PlotIncite, game.PlotPlan{Envoy: 15, At: 2}, 2)
		if err != nil || !success {
			panic(fmt.Sprintf("正常策反未得手：%v", err))
		}
		plan["incite_succeeded"] = success
		plans = append(plans, plan)
		fmt.Printf("%s: 孫堅任命程普，%v → %v，長沙合法謀略；策反得手\n", edition, before, plan["date_after"])
	}
	b, err := json.MarshalIndent(plans, "", "  ")
	if err != nil {
		panic(err)
	}
	if err = os.WriteFile(filepath.Join(*out, "plots-plan.json"), append(b, '\n'), 0644); err != nil {
		panic(err)
	}
}
