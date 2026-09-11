// Command san1dump 把原版的劇本資料印出來。
//
// 這支的用途是**把整條管線接起來驗一次**：容器 → 劇本表 → 排版。
// 它不是遊戲，是「讀得對不對」的最短路徑；出現任何怪東西，
// 在這裡就會看到，不必等引擎畫出來。
//
// ⚠ **本儲存庫不含任何原版檔案**，`-root` 由玩家自備。
//
//	go run ./cmd/san1dump -root path/to/三國演義 -slot 001
package main

import (
	"flag"
	"fmt"
	imgpng "image/png"
	"os"
	"path/filepath"

	"github.com/wicanr2/softworld_san1_remake/internal/ai"
	"github.com/wicanr2/softworld_san1_remake/internal/assets"
	"github.com/wicanr2/softworld_san1_remake/internal/battle"
	"github.com/wicanr2/softworld_san1_remake/internal/cells"
	"github.com/wicanr2/softworld_san1_remake/internal/font"
	"github.com/wicanr2/softworld_san1_remake/internal/game"
	"github.com/wicanr2/softworld_san1_remake/internal/i18n"
	"github.com/wicanr2/softworld_san1_remake/internal/session"
	"github.com/wicanr2/softworld_san1_remake/internal/state"
	"github.com/wicanr2/softworld_san1_remake/internal/ui"
)

func main() {
	root := flag.String("root", "", "原版遊戲目錄（必填，玩家自備）")
	slot := flag.String("slot", "001", "劇本或存檔槽：001..006、SV1..SV6")
	what := flag.String("what", "all", "印什麼：pref／gen／master／all")
	cols := flag.Int("cols", 6, "郡名槽位寬度（半形格），用來檢查裝不裝得下")
	png := flag.String("png", "", "把畫面存成 PNG（無頭環境驗版面用）")
	screen := flag.String("screen", "list", "畫哪一張：list（州郡一覽）／main（遊戲主畫面）／art（接原版素材的主畫面）／title（主選單）／artfield（接原版素材的戰場地形）／artbattle（接原版素材的整張主戰場）／poem（開場詞）／titleart（開場的三英圖）／battle（主戰場）")
	faction := flag.Int("faction", -1, "main 畫面的玩家勢力；−1 ＝ 用第一個在用的勢力")
	sel := flag.Int("sel", 0, "main 畫面訊息欄要顯示哪一個郡；0 ＝ 玩家的第一個郡")
	months := flag.Int("months", 0, "main 畫面先讓電腦跑幾個月再畫；battle／artfield 畫面是先打幾天")
	aiMode := flag.String("ai", "enhanced", "電腦 AI：base／plus／enhanced")
	lang := flag.String("lang", "zh-Hant", "介面語言：zh-Hant／en／ja")
	fontPath := flag.String("font", "fonts/unifont.hex.gz", "點陣字型（-png 時才需要）")
	saveDir := flag.String("saves", "", "存檔目錄（配 -save／-load 用）")
	saveTo := flag.Int("save", 0, "跑完 -months 之後存到第幾個進度（1..6）")
	loadFrom := flag.Int("load", 0, "改成從第幾個進度開始（1..6）")
	flag.Parse()
	if l, ok := i18n.Parse(*lang); ok {
		i18n.Current = l
	} else {
		fmt.Fprintf(os.Stderr, "不認識的語言 %q，用繁體中文\n", *lang)
	}

	if *root == "" {
		flag.Usage()
		os.Exit(2)
	}

	c, err := openData2(*root)
	if err != nil {
		die(err)
	}
	sc, err := state.LoadScenario(c, state.Slot(*slot))
	if err != nil {
		die(err)
	}
	fmt.Printf("劇本 %s（來源 %s）\n\n", *slot, *root)

	if *png != "" {
		if err := writePNG(*png, *fontPath, *root, sc, *slot, *screen, *aiMode, *faction, *sel, *months); err != nil {
			die(err)
		}
		fmt.Printf("畫面存到 %s\n\n", *png)
	}

	if *saveTo > 0 || *loadFrom > 0 {
		if err := runSaves(*saveDir, *loadFrom, *saveTo, sc, *aiMode, *faction, *months); err != nil {
			die(err)
		}
	}

	if *what == "pref" || *what == "all" {
		fmt.Printf("州郡（%d 個，編號 1-based）\n", len(sc.Prefectures()))
		for i, p := range sc.Prefectures() {
			fmt.Printf("  %2d %s", p.ID, cells.Pad(p.Name, *cols))
			if (i+1)%6 == 0 {
				fmt.Println()
			}
		}
		fmt.Println()
		// 槽位檢查：多語系時英文譯名塞不回來的，在這裡就看得到。
		over := 0
		for _, p := range sc.Prefectures() {
			if !cells.Fits(p.Name, *cols) {
				over++
			}
		}
		fmt.Printf("  槽寬 %d 格：%d 個郡名放不下\n\n", *cols, over)
	}

	if *what == "gen" || *what == "all" {
		people := sc.People()
		fmt.Printf("人物（%d 個槽，其中 %d 個是人）\n", len(sc.Generals()), len(people))
		for i, g := range people {
			fmt.Printf("  %s", cells.Pad(g.Name, 8))
			if (i+1)%8 == 0 {
				fmt.Println()
			}
		}
		fmt.Println()
		byLen := map[int]int{}
		for _, g := range people {
			byLen[len([]rune(g.Name))]++
		}
		fmt.Printf("  字數分佈 %v\n\n", byLen)
	}

	if *what == "master" || *what == "all" {
		fmt.Printf("諸侯（%d 位）——欄位版面未解，只印原始 bytes 前 16 個\n",
			len(sc.Masters()))
		for _, m := range sc.Masters() {
			fmt.Printf("  [%2d] % X\n", m.Index, m.Raw[:16])
		}
	}
}

// writePNG 把引擎會畫的那一張畫面存成圖。
//
// **和 cmd/san1 畫的是同一張**（都走 ui.DrawPrefectureList）。
// 畫面 bug 測試看不到，但存成圖就看得到，而且無頭環境也產得出來。
func writePNG(out, fontPath, root string, sc *state.Scenario, slot, screen, aiMode string, faction, sel, months int) error {
	fh, err := os.Open(fontPath)
	if err != nil {
		return err
	}
	face, err := font.ParseHexGz(fh, 16)
	fh.Close()
	if err != nil {
		return err
	}
	c := ui.NewCanvas(ui.Cols, ui.Rows, face)
	if screen == "art" || screen == "title" || screen == "artfield" ||
		screen == "artbattle" || screen == "poem" || screen == "titleart" ||
		screen == "credits" || screen == "hall" {
		c = ui.NewCanvasPx(assets.ScreenW, assets.ScreenH, face)
	}
	// 小字級與 cmd/san1 同一份（英文在原版版面放不下的地方用）。
	c.SetSmallFace(loadSmallFace(fontPath))
	switch screen {
	case "artfield", "artbattle":
		c1, err := openContainer(root, "DATA1")
		if err != nil {
			return fmt.Errorf("地形圖塊要讀 DATA1：%w", err)
		}
		c3, err := openContainer(root, "DATA3")
		if err != nil {
			return fmt.Errorf("上方花邊要讀 DATA3：%w", err)
		}
		ab, err := ui.NewArtBattle(c1, c3)
		if err != nil {
			return err
		}
		at := sel
		if at < 1 {
			at = 25
		}
		var p *state.Prefecture
		for _, q := range sc.Prefectures() {
			if q.ID == at {
				x := q
				p = &x
			}
		}
		if p == nil {
			return fmt.Errorf("沒有郡 %d", at)
		}
		// 部隊要接上原版的旗幟，就得真的有一場戰役。**優先用劇本裡真的
		// 兩個勢力**——那樣連統帥的姓名與肖像都是原版的資料；湊不出
		// 對手才退回自組的樣本局面。
		b, chiefs := realBattle(sc, at)
		if b == nil {
			fmt.Fprintf(os.Stderr,
				"san1dump：郡 %d 湊不出真的兩軍（沒有駐軍或鄰郡同屬一方），"+
					"改用自組的樣本局面\n", at)
			b = sampleBattle(at, p.Neighbours, p.BattleField)
		}
		for d := 0; d < months && !b.Over; d++ {
			for _, u := range b.Order() {
				b.AutoTurn(u)
			}
			b.EndDay()
		}
		var hi *battle.Unit
		if order := b.Order(); len(order) > 0 {
			hi = order[0]
		}
		if screen == "artfield" {
			ui.DrawArtField(c, ab, p.Name, p.BattleField, b.Units, hi)
			break
		}
		info := ui.ArtBattleInfo{
			Prefecture: p.Name,
			Province:   state.ProvinceName(int(p.Province)),
			Field:      p.BattleField,
			ID:         p.ID,
			Portrait:   [2]int{-1, -1},
			Date:       game.ScenarioStart[sc.Slot],
		}
		for k, side := range []battle.Side{battle.MainAttacker, battle.MainDefender} {
			for _, u := range b.Units {
				if u.Side != side || len(u.Leaders) == 0 {
					continue
				}
				info.Commander[k] = u.Leaders[0].Name
				break
			}
			if chiefs[k] != nil {
				info.Commander[k] = chiefs[k].Name
				info.Portrait[k] = int(chiefs[k].Portrait)
			}
		}
		ui.DrawArtBattle(c, ab, b, ui.BattleView{Acting: hi,
			Prompt: promptFor(hi)}, info)
	case "credits", "hall":
		// 製作群：字幕從山後面升起來（`docs/spec/012`）。
		// `-months` 借來當捲動量，一格一個像素。
		c2, err := openContainer(root, "DATA2")
		if err != nil {
			return fmt.Errorf("製作群要讀 DATA2：%w", err)
		}
		cr, err := assets.LoadCredits(c2)
		if err != nil {
			return err
		}
		if screen == "hall" {
			ui.DrawCreditHall(c, cr)
			break
		}
		scroll := months
		if scroll <= 0 {
			scroll = ui.CreditsLength(cr) / 2
		}
		ui.DrawCredits(c, cr, scroll)
	case "poem":
		c1, err := openContainer(root, "DATA1")
		if err != nil {
			return fmt.Errorf("開場詩的底圖要讀 DATA1：%w", err)
		}
		im, err := assets.PoemScreen(c1)
		if err != nil {
			return err
		}
		ui.DrawPoem(c, im)
	case "titleart":
		c1, err := openContainer(root, "DATA1")
		if err != nil {
			return fmt.Errorf("三英圖要讀 DATA1：%w", err)
		}
		im, err := assets.TitleArt(c1)
		if err != nil {
			return err
		}
		ui.DrawImage(c, im)
	case "title":
		c3, err := openContainer(root, "DATA3")
		if err != nil {
			return fmt.Errorf("主選單要讀 DATA3：%w", err)
		}
		ts, err := ui.NewTitleScreen(c3)
		if err != nil {
			return err
		}
		ui.DrawTitle(c, ts, 0)
	case "art", "main":
		f := faction
		if f < 0 {
			act := sc.ActiveFactions()
			if len(act) == 0 {
				return fmt.Errorf("劇本 %s 裡沒有在用的勢力", slot)
			}
			f = act[0]
		}
		g, err := game.New(sc, state.FactionID(f), 5, state.EditionBase)
		if err != nil {
			return err
		}
		brain, err := ai.New(ai.Mode(aiMode))
		if err != nil {
			return err
		}
		s := session.New(g, brain, state.FactionID(f))
		for i := 0; i < months; i++ {
			s.EndMonth()
		}
		if sel == 0 {
			if t := g.Territory(state.FactionID(f)); len(t) > 0 {
				sel = t[0]
			}
		}
		if screen == "art" {
			c3, err := openContainer(root, "DATA3")
			if err != nil {
				return fmt.Errorf("接原版素材要讀 DATA3：%w", err)
			}
			// 州郡的填色圖樣在 `DATA1`（`EGAFILL.PAL`）；讀不到就退回
			// remake 自己的色號。
			c1, _ := openContainer(root, "DATA1")
			art, err := ui.NewArtScreen(c3, c1)
			if err != nil {
				return err
			}
			ui.DrawArtSession(c, art, g, s.Log, ui.View{Sel: sel, Over: s.Over})
			break
		}
		ui.DrawSession(c, g, s.Log, ui.View{Sel: sel, Over: s.Over})
	case "battle":
		// 用某個郡的地形開一場，讓電腦打 `months` 天再畫。
		// **戰場的版面壞掉在無頭環境看不出來**，所以要有這一張。
		at := sel
		if at < 1 {
			at = 15
		}
		p, err := sc.Prefecture(at)
		if err != nil {
			return err
		}
		b := sampleBattle(at, p.Neighbours, p.BattleField)
		r := battle.NewRunner(b, nil)
		for d := 0; d < months && !b.Over; d++ {
			for _, u := range b.Order() {
				b.AutoTurn(u)
			}
			b.EndDay()
		}
		_ = r
		var acting *battle.Unit
		for _, u := range b.Units {
			if u.Alive() {
				acting = u
				break
			}
		}
		cur := ui.Hexer{}
		if acting != nil {
			cur = ui.Hexer{At: acting.At, Shown: true}
		}
		ui.DrawBattle(c, b, ui.BattleView{
			Cursor: cur, Acting: acting,
			Menu:   "指令",
			Items:  ui.BattleCommandLines(),
			Prompt: fmt.Sprintf("第 %d 郡的戰場（原版的郡地理誌）", at),
		})
	case "list":
		ui.DrawPrefectureList(c, sc, slot)
	default:
		return fmt.Errorf("不認識的畫面 %q（收 list 或 main）", screen)
	}
	if n := len(c.Missing); n > 0 {
		fmt.Fprintf(os.Stderr, "⚠ %d 個字沒有字模：%q\n", n, string(missingRunes(c)))
	}
	f, err := os.Create(out)
	if err != nil {
		return err
	}
	defer f.Close()
	return imgpng.Encode(f, c.Img)
}

func missingRunes(c *ui.Canvas) []rune {
	var rs []rune
	for r := range c.Missing {
		rs = append(rs, r)
	}
	return rs
}

// openData2 開 DATA2 容器。三個檔缺一不可。
func openData2(root string) (*assets.Container, error) { return openContainer(root, "DATA2") }

// openContainer 開任一組三件套。三個檔缺一不可——少一個就是切出垃圾。
func openContainer(root, name string) (*assets.Container, error) {
	base := filepath.Join(root, name)
	var parts [3][]byte
	for i, ext := range []string{".NAM", ".IDX", ".GRP"} {
		b, err := os.ReadFile(base + ext)
		if err != nil {
			return nil, fmt.Errorf("讀 %s%s：%w", name, ext, err)
		}
		parts[i] = b
	}
	return assets.OpenContainer(parts[0], parts[1], parts[2])
}

func die(err error) {
	fmt.Fprintln(os.Stderr, "san1dump:", err)
	os.Exit(1)
}

// runSaves 是存讀檔的無頭路徑。
//
// Ebiten 那一層要有視窗才跑得起來，而存讀檔是**最不該只有手動驗過**
// 的功能之一：壞掉的時候玩家失去的是幾個小時的進度。
func runSaves(dir string, load, saveTo int, sc *state.Scenario, aiMode string, faction, months int) error {
	if dir == "" {
		return fmt.Errorf("要用 -saves 指定存檔目錄")
	}
	var s *session.Session
	if load > 0 {
		var err error
		s, err = session.Load(dir, load, ai.Mode(aiMode))
		if err != nil {
			return err
		}
		fmt.Printf("讀入第 %d 個進度：%d 年 %d 月\n", load, s.G.Date.Year, s.G.Date.Month)
	} else {
		f := faction
		if f < 0 {
			act := sc.ActiveFactions()
			if len(act) == 0 {
				return fmt.Errorf("這個劇本沒有在用的勢力")
			}
			f = act[0]
		}
		g, err := game.New(sc, state.FactionID(f), 5, state.EditionBase)
		if err != nil {
			return err
		}
		brain, err := ai.New(ai.Mode(aiMode))
		if err != nil {
			return err
		}
		s = session.New(g, brain, state.FactionID(f))
	}
	for i := 0; i < months; i++ {
		s.EndMonth()
	}
	if saveTo > 0 {
		if err := s.Save(dir, saveTo, ""); err != nil {
			return err
		}
		fmt.Printf("存入第 %d 個進度：%d 年 %d 月\n", saveTo, s.G.Date.Year, s.G.Date.Month)
	}
	for _, info := range session.Saves(dir) {
		fmt.Printf("  %d. %s\n", info.Slot, info.Describe())
	}
	fmt.Println()
	return nil
}

// sampleBattle 開一場給畫面用的戰役。
func sampleBattle(at int, neighbours []int, field []byte) *battle.Battle {
	mk := func(n int, name string, war uint8, men int, base int) []battle.Leader {
		var out []battle.Leader
		for i := 0; i < n; i++ {
			out = append(out, battle.Leader{
				Index: base + i, Name: name, War: war - uint8(i), Intel: 75,
				Stamina: 100, Charm: 50, Soldiers: men, Training: 60, Arms: 60,
				Troop: battle.TroopLand,
			})
		}
		return out
	}
	from := 0
	if len(neighbours) > 0 {
		from = neighbours[0]
	}
	f, err := battle.Load(field, neighbours)
	if err != nil {
		// 劇本沒帶地圖時（自組的局面）退回生成器，它也是決定性的。
		f = battle.Generate(battle.Params{
			Prefecture: at, Neighbours: neighbours, LandValue: 60, FloodRate: 40,
		})
	}
	return battle.New(battle.Setup{
		Field:        f,
		Weather:      battle.Windy,
		Seed:         uint32(at),
		Attackers:    mk(8, "攻將", 90, 3000, 0),
		Defenders:    mk(6, "守將", 80, 2500, 100),
		FromGate:     from,
		AttackerGold: 3000, AttackerRice: 9000,
		DefenderGold: 2000, DefenderRice: 8000,
	})
}

// promptFor 是主戰場下方那一行提示（原版 `%s 的命令(0-8)`）。
func promptFor(u *battle.Unit) string {
	if u == nil {
		return ""
	}
	return i18n.Sf("bat.unitMoves", u.Name(), u.Move)
}

// realBattle 用劇本裡真的兩個勢力開一場：守方是這個郡的主人，攻方是
// 第一個**不同勢力**又有駐軍的鄰郡。湊不出來就回 nil。
func realBattle(sc *state.Scenario, at int) (*battle.Battle, [2]*game.General) {
	var chiefs [2]*game.General
	act := sc.ActiveFactions()
	if len(act) == 0 {
		return nil, chiefs
	}
	g, err := game.New(sc, state.FactionID(act[0]), 5, state.EditionBase)
	if err != nil {
		return nil, chiefs
	}
	to := g.Prefecture(at)
	if to == nil || len(g.Garrison(at)) == 0 {
		return nil, chiefs
	}
	for _, n := range to.Neighbours {
		from := g.Prefecture(n)
		if from == nil || from.Owner == to.Owner {
			continue
		}
		var idx []int
		for _, x := range g.Garrison(n) {
			idx = append(idx, x.Index)
		}
		// **要留一位在家**：原版不准把郡治理的人全部帶走
		//（`移出之後這個郡沒有人治理`）。
		if len(idx) < 2 {
			continue
		}
		idx = idx[:len(idx)-1]
		p, err := g.BeginAttack(n, at, idx, from.Owner, game.HalfSupply())
		if err != nil {
			continue
		}
		att, def := p.Chiefs()
		chiefs[0], chiefs[1] = att, def
		return p.Battle(), chiefs
	}
	return nil, chiefs
}

// loadSmallFace 讀小字級（與大字型同一個目錄的 `ascii6x10.hex.gz`）；
// 讀不到回 nil，那時英文照原尺寸退回別的排法（`docs/spec/014` §3.2）。
func loadSmallFace(bigFont string) *font.Face {
	fh, err := os.Open(filepath.Join(filepath.Dir(bigFont), "ascii6x10.hex.gz"))
	if err != nil {
		return nil
	}
	defer fh.Close()
	f, err := font.ParseHexGz(fh, ui.SmallH)
	if err != nil {
		return nil
	}
	return f
}
