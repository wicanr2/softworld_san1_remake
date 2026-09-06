// Command san1 是《三國演義》remake 的引擎進入點。
//
// ⚠ **本儲存庫不含任何原版檔案。** 用 -root 指到你自己的原版目錄。
//
//	go run ./cmd/san1 -root path/to/三國演義 -faction 0 -ai enhanced
//
// 操作：
//
//	← → ↑ ↓ ／ Tab   在自己的郡之間移動
//	0–9              指令類別（查看／軍事／兵士／內政／商業／人事／君主／謀略）
//	1–9              子選單與挑選清單
//	Enter            結束這個月
//	Esc              返回上一層
package main

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"sort"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/inpututil"

	"github.com/wicanr2/softworld_san1_remake/internal/ai"
	"github.com/wicanr2/softworld_san1_remake/internal/assets"
	"github.com/wicanr2/softworld_san1_remake/internal/font"
	"github.com/wicanr2/softworld_san1_remake/internal/game"
	"github.com/wicanr2/softworld_san1_remake/internal/session"
	"github.com/wicanr2/softworld_san1_remake/internal/state"
	"github.com/wicanr2/softworld_san1_remake/internal/ui"
)

type app struct {
	canvas *ui.Canvas
	screen *ebiten.Image
	s      *session.Session

	view  ui.View
	menu  byte // 展開中的類別；0 表示在主選單
	pick  []pickItem
	dirty bool

	// saveDir 是存檔目錄，空字串表示這一局不能存。
	saveDir string
	// aiMode 記著讀檔要用哪一種電腦 AI。
	aiMode ai.Mode
}

// conscriptStep 是按一次「徵兵」募多少人。
//
// **原版是輸入數字，這裡先用固定量**。要問數字得有文字輸入框，
// 那是另一件事；固定量讓迴圈先跑得起來，而且不會假裝自己是原版行為。
const conscriptStep = 100

func (a *app) Update() error {
	if inpututil.IsKeyJustPressed(ebiten.KeyEscape) {
		a.menu, a.view.Menu, a.view.Items, a.pick = 0, "", nil, nil
		a.view.Prompt, a.view.Page = "", nil
		a.dirty = true
		return nil
	}
	if inpututil.IsKeyJustPressed(ebiten.KeyEnter) ||
		inpututil.IsKeyJustPressed(ebiten.KeyNumpadEnter) {
		a.s.EndMonth()
		a.menu, a.view.Menu, a.view.Items, a.pick = 0, "", nil, nil
		a.view.Prompt, a.view.Page = "", nil
		a.dirty = true
		return nil
	}
	if inpututil.IsKeyJustPressed(ebiten.KeyTab) ||
		inpututil.IsKeyJustPressed(ebiten.KeyRight) ||
		inpututil.IsKeyJustPressed(ebiten.KeyDown) {
		a.cycle(+1)
		return nil
	}
	if inpututil.IsKeyJustPressed(ebiten.KeyLeft) ||
		inpututil.IsKeyJustPressed(ebiten.KeyUp) {
		a.cycle(-1)
		return nil
	}
	for k := ebiten.Key0; k <= ebiten.Key9; k++ {
		if inpututil.IsKeyJustPressed(k) {
			a.press(byte('0' + (k - ebiten.Key0)))
			return nil
		}
	}
	return nil
}

// cycle 在玩家自己的郡之間移動選取。
func (a *app) cycle(d int) {
	own := a.s.PlayerTerritory()
	if len(own) == 0 {
		return
	}
	sort.Ints(own)
	i := 0
	for j, id := range own {
		if id == a.view.Sel {
			i = j
		}
	}
	i = (i + d + len(own)) % len(own)
	a.view.Sel = own[i]
	a.dirty = true
}

func (a *app) press(k byte) {
	defer func() { a.dirty = true }()

	// 展開中的挑選清單優先吃按鍵。
	if len(a.pick) > 0 {
		i := int(k - '1')
		if i < 0 || i >= len(a.pick) {
			a.view.Prompt = "沒有這個選項"
			return
		}
		it := a.pick[i]
		a.pick = nil
		it.do(it.id)
		return
	}
	if a.menu == 0 {
		title, items := ui.SubMenu(k)
		if items == nil {
			if k == '0' {
				a.view.Prompt = "本郡狀態就在右上角（不耗指令）"
			} else {
				a.view.Prompt = fmt.Sprintf("「%s」還沒實作", commandName(k))
			}
			return
		}
		a.menu, a.view.Menu, a.view.Items = k, title, items
		a.view.Prompt, a.view.Page = "", nil
		return
	}
	a.begin(a.menu, k)
}

// pickItem 是挑選清單的一個項目。id 是它代表的人物槽號或郡編號。
type pickItem struct {
	label string
	id    int
	do    func(id int)
}

// begin 收下「類別 ＋ 項目」，決定是直接執行還是先要玩家挑目標。
func (a *app) begin(cat, item byte) {
	sel := a.view.Sel
	g, s := a.s.G, a.s
	closeMenu := func() { a.menu, a.view.Menu, a.view.Items = 0, "", nil }

	switch {
	// ---- 9. 其他 ----
	case cat == '9' && item == '2':
		a.askSlot("存到第幾個進度", true, func(slot int) {
			_ = a.s.Save(a.saveDir, slot, "")
		})
	case cat == '9' && item == '7':
		// 原版的提示是 `使用%s年號`（`docs/re/04` §3）。
		if a.view.Calendar == game.Western {
			a.view.Calendar = game.ChineseEra
		} else {
			a.view.Calendar = game.Western
		}
		a.view.Prompt = fmt.Sprintf("使用%s年號", a.view.Calendar.Name())
		closeMenu()
	case cat == '9' && item == '1':
		// 「＊結束」在原版是回到主選單。這裡先提醒存檔——
		// **沒存就離開是最貴的一次誤按**。
		a.view.Prompt = "要離開請關視窗；離開前記得先存檔（其他 → 儲存）"
		closeMenu()

	// ---- 1. 查看（不耗指令）----
	case cat == '1' && item == '2':
		a.view.PageTitle, a.view.Page = ui.GeneralList(g, sel)
		closeMenu()
	case cat == '1' && item == '4':
		a.view.PageTitle, a.view.Page = ui.TerritoryList(g, s.Player)
		closeMenu()
	case cat == '1' && item == '6':
		a.view.PageTitle, a.view.Page = ui.TreasuryList(g, s.Player)
		closeMenu()

	// ---- 2. 軍事 ----
	case cat == '2' && item == '1':
		a.askGeneral("調動誰", func(gi int) {
			a.askNeighbour("調到哪個郡", true, func(to int) {
				a.run(game.MoveOrder{At: sel, To: to, General: gi})
			})
		})
	case cat == '2' && item == '2':
		a.askNeighbour("攻打哪個郡", false, func(to int) {
			var force []int
			var keep *game.General
			for _, x := range g.Garrison(sel) {
				if x.Faction != s.Player {
					continue
				}
				if keep == nil {
					keep = x
					continue
				}
				force = append(force, x.Index)
			}
			a.run(game.AttackOrder{At: sel, To: to, Force: force})
		})
	case cat == '2' && item == '3':
		a.askOwn("送到哪個郡", func(to int) {
			a.run(game.TransportOrder{At: sel, To: to, Gold: 500, Rice: 500})
		})

	// ---- 3. 兵士 ----
	case cat == '3' && item == '1':
		a.askGeneral("訓練誰的部隊", func(gi int) { a.run(game.TrainOrder{At: sel, General: gi}) })
	case cat == '3' && item == '2':
		a.askGeneral("誰去募兵", func(gi int) {
			a.run(game.ConscriptOrder{At: sel, General: gi, Count: conscriptStep})
		})
	case cat == '3' && item == '3':
		a.askGeneral("誰去購械", func(gi int) {
			a.run(game.ArmsOrder{At: sel, General: gi, Units: 500})
		})
	case cat == '3' && item == '4':
		var all []int
		for _, x := range g.Garrison(sel) {
			if x.Faction == s.Player {
				all = append(all, x.Index)
			}
		}
		a.run(game.RedistributeOrder{At: sel, Units: all})

	// ---- 4. 內政 ----
	case cat == '4' && item == '1':
		a.askGeneral("誰去開墾", func(gi int) { a.run(game.ReclaimOrder{At: sel, General: gi}) })
	case cat == '4' && item == '2':
		a.askGeneral("誰去治水", func(gi int) { a.run(game.FloodControlOrder{At: sel, General: gi}) })
	case cat == '4' && item == '3':
		a.askGeneral("誰去監工", func(gi int) { a.run(game.BuildFortOrder{At: sel, General: gi}) })
	case cat == '4' && item == '4':
		a.run(game.RestOrder{At: sel})

	// ---- 5. 商業 ----
	case cat == '5' && item == '1':
		a.run(game.BuyRiceOrder{At: sel, Units: 1000})
	case cat == '5' && item == '2':
		a.run(game.SellRiceOrder{At: sel, Units: 1000})
	case cat == '5' && item == '3':
		a.run(game.ReliefOrder{At: sel})

	// ---- 6. 人事 ----
	case cat == '6' && item == '1':
		a.askGeneral("誰去尋訪", func(gi int) { a.run(game.SearchOrder{At: sel, General: gi}) })
	case cat == '6' && item == '2':
		a.askFree("登用誰", func(gi int) { a.run(game.RecruitOrder{At: sel, Target: gi}) })
	case cat == '6' && item == '3':
		a.askGeneral("賞賜誰", func(gi int) {
			a.run(game.RewardOrder{At: sel, Target: gi, Gold: game.MaxReward})
		})
	case cat == '6' && item == '4':
		a.askGeneral("撤誰的職", func(gi int) { a.run(game.DismissOrder{At: sel, Target: gi}) })

	// ---- 7. 君主 ----
	case cat == '7' && item == '1':
		a.askGeneral("拜誰為軍師", func(gi int) { a.run(game.AppointChiefOrder{At: sel, Target: gi}) })
	case cat == '7' && item == '2':
		a.askGeneral("誰當太守", func(gi int) { a.run(game.AppointGovernorOrder{At: sel, Target: gi}) })
	case cat == '7' && item == '3':
		a.pickFrom("自治型態", []pickItem{
			{"正常", int(game.AutoNormal), a.setAutonomy},
			{"內政", int(game.AutoCivil), a.setAutonomy},
			{"軍事", int(game.AutoMilitary), a.setAutonomy},
			{"自治", int(game.AutoSelf), a.setAutonomy},
		})
	case cat == '7' && item == '4':
		a.pickFrom("賞賜哪一件", []pickItem{
			{"兵書", int(game.TreasureBook), a.giftThen},
			{"寶刀", int(game.TreasureBlade), a.giftThen},
			{"美女", int(game.TreasureBeauty), a.giftThen},
			{"駿馬", int(game.TreasureHorse), a.giftThen},
		})
	case cat == '7' && item == '5':
		a.askEnemyGeneral("挖角誰", func(gi int) { a.run(game.HeadhuntOrder{At: sel, Target: gi}) })

	// ---- 8. 謀略 ----
	case cat == '8':
		plot := game.Plot(item - '0')
		a.askGeneral("誰當使者", func(gi int) {
			a.askNeighbour("對哪個郡用計", false, func(to int) {
				a.run(game.PlotOrder{At: sel, To: to, What: plot, Envoy: gi})
			})
		})

	default:
		a.view.Prompt = "沒有這個項目"
	}
	if len(a.pick) == 0 {
		closeMenu()
	}
}

func (a *app) setAutonomy(mode int) {
	a.run(game.AutonomyOrder{At: a.view.Sel, Mode: game.Autonomy(mode)})
}

func (a *app) giftThen(what int) {
	a.askGeneral("賞給誰", func(gi int) {
		a.run(game.GiftOrder{At: a.view.Sel, Target: gi, What: game.Treasure(what)})
	})
}

// run 送出一個命令並清掉選單狀態。
func (a *app) run(o game.Order) {
	a.pick = nil
	a.menu, a.view.Menu, a.view.Items = 0, "", nil
	a.view.Page = nil
	if err := a.s.Do(o); err != nil {
		a.view.Prompt = err.Error()
		return
	}
	a.view.Prompt = ""
}

// pickFrom 展開一個挑選清單。
//
// **沒有可選對象時要說出來。** 靜靜地回到主選單，玩家會以為是按鍵沒進去。
func (a *app) pickFrom(title string, items []pickItem) {
	if len(items) == 0 {
		a.pick = nil
		a.menu, a.view.Menu, a.view.Items = 0, "", nil
		a.view.Prompt = "沒有可以選的對象"
		return
	}
	if len(items) > 9 {
		items = items[:9]
	}
	a.pick = items
	a.view.Menu = title
	a.view.Items = nil
	for i, it := range items {
		a.view.Items = append(a.view.Items, ui.Command{Key: byte('1' + i), Name: it.label})
	}
}

// askGeneral 讓玩家從當地自己的將領裡挑一位。
func (a *app) askGeneral(title string, then func(int)) {
	var items []pickItem
	for _, x := range a.s.G.Garrison(a.view.Sel) {
		if x.Faction != a.s.Player {
			continue
		}
		items = append(items, pickItem{x.Name, x.Index, then})
	}
	a.pickFrom(title, items)
}

// askFree 讓玩家從當地在野將領裡挑一位。
func (a *app) askFree(title string, then func(int)) {
	var items []pickItem
	for _, x := range a.s.G.Free(a.view.Sel) {
		items = append(items, pickItem{x.Name, x.Index, then})
	}
	a.pickFrom(title, items)
}

// askEnemyGeneral 讓玩家從鄰郡的敵方現役將領裡挑一位。
func (a *app) askEnemyGeneral(title string, then func(int)) {
	var items []pickItem
	p := a.s.G.Prefecture(a.view.Sel)
	if p == nil {
		a.view.Prompt = "沒有這個郡"
		return
	}
	for _, n := range p.Neighbours {
		q := a.s.G.Prefecture(n)
		if q == nil || !q.Owned() || q.Owner == a.s.Player {
			continue
		}
		for _, x := range a.s.G.Garrison(n) {
			if x.Faction == q.Owner && x.Status != state.StatusLord {
				items = append(items, pickItem{
					fmt.Sprintf("%s（%s）", x.Name, q.Name), x.Index, then})
			}
		}
	}
	a.pickFrom(title, items)
}

// askNeighbour 讓玩家挑一個鄰郡。own 為真時只列自己的。
func (a *app) askNeighbour(title string, own bool, then func(int)) {
	var items []pickItem
	p := a.s.G.Prefecture(a.view.Sel)
	if p == nil {
		a.view.Prompt = "沒有這個郡"
		return
	}
	for _, n := range p.Neighbours {
		q := a.s.G.Prefecture(n)
		if q == nil {
			continue
		}
		mine := q.Owned() && q.Owner == a.s.Player
		if own != mine {
			continue
		}
		items = append(items, pickItem{fmt.Sprintf("%d %s", q.ID, q.Name), n, then})
	}
	a.pickFrom(title, items)
}

// askOwn 讓玩家挑一個自己的郡（不限相鄰）。
func (a *app) askOwn(title string, then func(int)) {
	var items []pickItem
	for _, id := range a.s.PlayerTerritory() {
		if id == a.view.Sel {
			continue
		}
		q := a.s.G.Prefecture(id)
		items = append(items, pickItem{fmt.Sprintf("%d %s", q.ID, q.Name), id, then})
	}
	a.pickFrom(title, items)
}

// askSlot 讓玩家挑一個存檔槽。
//
// 空槽也列出來，而且**寫得出是空的**——原版的儲存畫面就是六格，
// 看得到哪幾格可以蓋、哪幾格會被蓋掉。
func (a *app) askSlot(title string, forSaving bool, then func(int)) {
	if a.saveDir == "" {
		a.view.Prompt = "這一局沒有存檔目錄（用 -saves 指定）"
		return
	}
	var items []pickItem
	for _, info := range session.Saves(a.saveDir) {
		if !forSaving && !info.Exists {
			continue
		}
		items = append(items, pickItem{info.Describe(), info.Slot, then})
	}
	if len(items) == 0 {
		a.view.Prompt = "沒有可讀的進度"
		return
	}
	a.pickFrom(title, items)
}

func commandName(k byte) string {
	for _, c := range ui.Commands() {
		if c.Key == k {
			return c.Name
		}
	}
	return string(k)
}

func (a *app) Draw(dst *ebiten.Image) {
	if a.dirty {
		// 畫面內容在 internal/ui，Ebiten 這一層只負責貼上去——
		// 同一張圖無頭環境也產得出來（cmd/san1dump -png）。
		a.view.Over = a.s.Over
		ui.DrawSession(a.canvas, a.s.G, a.s.Log, a.view)
		a.screen.WritePixels(a.canvas.Img.Pix)
		a.dirty = false
	}
	dst.DrawImage(a.screen, nil)
}

func (a *app) Layout(int, int) (int, int) {
	return ui.Cols * ui.CellW, ui.Rows * ui.CellH
}

func main() {
	root := flag.String("root", "", "原版遊戲目錄（必填，玩家自備）")
	saveDir := flag.String("saves", "saves", "存檔目錄（remake 自己的，不寫回原版）")
	load := flag.Int("load", 0, "開場就讀第幾個進度（1..6）；0 ＝ 開新局")
	calendar := flag.String("calendar", "中曆", "年月的表示方式：中曆／西曆（原版「其他 → 年號」）")
	fontPath := flag.String("font", "fonts/unifont.hex.gz", "點陣字型")
	slot := flag.String("slot", "001", "劇本：001..006")
	faction := flag.Int("faction", -1, "玩家的勢力槽號；−1 ＝ 第一個在用的")
	aiMode := flag.String("ai", string(ai.ModeEnhanced),
		"電腦 AI：base（原版還原）／plus（加強版還原）／enhanced（remake 強化）")
	difficulty := flag.Int("difficulty", 5, "難度 1..10")
	scale := flag.Int("scale", 2, "視窗放大倍率（整數倍，不做非整數縮放）")
	flag.Parse()
	if *root == "" {
		fmt.Fprintln(os.Stderr, "san1: 要用 -root 指到原版目錄（本儲存庫不含原版檔案）")
		flag.Usage()
		os.Exit(2)
	}

	fh, err := os.Open(*fontPath)
	if err != nil {
		die(err)
	}
	face, err := font.ParseHexGz(fh, 16)
	fh.Close()
	if err != nil {
		die(err)
	}

	c, err := openData2(*root)
	if err != nil {
		die(err)
	}
	sc, err := state.LoadScenario(c, state.Slot(*slot))
	if err != nil {
		die(err)
	}
	f := *faction
	if f < 0 {
		act := sc.ActiveFactions()
		if len(act) == 0 {
			die(fmt.Errorf("劇本 %s 裡沒有在用的勢力", *slot))
		}
		f = act[0]
	}
	var s *session.Session
	var brain ai.Brain
	var g *game.State
	if *load > 0 {
		s, err = session.Load(*saveDir, *load, ai.Mode(*aiMode))
		if err != nil {
			die(err)
		}
		g, brain = s.G, s.Brain
		f = int(s.Player)
	} else {
		g, err = game.New(sc, state.FactionID(f), *difficulty)
		if err != nil {
			die(err)
		}
		brain, err = ai.New(ai.Mode(*aiMode))
		if err != nil {
			die(err)
		}
		s = session.New(g, brain, state.FactionID(f))
	}

	a := &app{
		canvas:  ui.NewCanvas(ui.Cols, ui.Rows, face),
		screen:  ebiten.NewImage(ui.Cols*ui.CellW, ui.Rows*ui.CellH),
		s:       s,
		dirty:   true,
		saveDir: *saveDir,
		aiMode:  ai.Mode(*aiMode),
	}
	if *calendar == "西曆" {
		a.view.Calendar = game.Western
	}
	if own := s.PlayerTerritory(); len(own) > 0 {
		sort.Ints(own)
		a.view.Sel = own[0]
	}

	// 整數倍放大：CJK 點陣字非整數縮放會糊掉（`CLAUDE.md` §3.3）。
	if *scale < 1 {
		*scale = 1
	}
	ebiten.SetWindowSize(ui.Cols*ui.CellW**scale, ui.Rows*ui.CellH**scale)
	lord := g.Lord(state.FactionID(f))
	title := "三國演義 remake"
	if lord != nil {
		title = fmt.Sprintf("三國演義 remake ── %s（AI：%s）", lord.Name, brain.Name())
	}
	ebiten.SetWindowTitle(title)
	if err := ebiten.RunGame(a); err != nil {
		die(err)
	}
}

func openData2(root string) (*assets.Container, error) {
	base := filepath.Join(root, "DATA2")
	var parts [3][]byte
	for i, ext := range []string{".NAM", ".IDX", ".GRP"} {
		b, err := os.ReadFile(base + ext)
		if err != nil {
			return nil, fmt.Errorf("讀 DATA2%s：%w", ext, err)
		}
		parts[i] = b
	}
	return assets.OpenContainer(parts[0], parts[1], parts[2])
}

func die(err error) {
	fmt.Fprintln(os.Stderr, "san1:", err)
	os.Exit(1)
}
