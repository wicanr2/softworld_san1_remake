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
//	M                換下一首配樂
package main

import (
	"flag"
	"fmt"
	"os"
	"sort"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/inpututil"

	"github.com/wicanr2/softworld_san1_remake/internal/ai"
	"github.com/wicanr2/softworld_san1_remake/internal/assets"
	"github.com/wicanr2/softworld_san1_remake/internal/battle"
	"github.com/wicanr2/softworld_san1_remake/internal/font"
	"github.com/wicanr2/softworld_san1_remake/internal/game"
	"github.com/wicanr2/softworld_san1_remake/internal/i18n"
	"github.com/wicanr2/softworld_san1_remake/internal/menu"
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
	num   *numEntry
	dirty bool

	// fight 非 nil 表示正在打一場玩家親自指揮的戰役。
	fight *fight

	// saveDir 是存檔目錄，空字串表示這一局不能存。
	saveDir string
	// aiMode 記著讀檔要用哪一種電腦 AI。
	aiMode ai.Mode

	// jb 是配樂；沒有原版的 DATA1 就是 nil。
	jb *jukebox

	// sfx 是 PC 喇叭的音效與語音（`docs/spec/008`）；放不出聲音就是 nil。
	sfx *voicebox

	// art 是接上原版素材的主畫面；沒有原版的 DATA3 就是 nil，
	// 那時退回 remake 自己的文字版面。
	art *ui.ArtScreen

	// artBattle 是接上原版素材的主戰場，要 `DATA1` 與 `DATA3` 兩個。
	artBattle *ui.ArtBattle

	// menuScreen 非 nil 表示停在主選單那一層（開場詞按完就到這裡）；
	// titleArt 是那一張的底圖。
	menuScreen *menu.Screen
	titleArt   *ui.TitleScreen

	// c2 是 `DATA2`：主選單要重讀劇本，得留著。
	c2 *assets.Container

	// edition 是版本旗標，開新局時要用。
	edition state.Edition

	// quit 為真表示玩家選了「回作業系統」。
	quit bool

	// titlePic 非 nil 表示停在開場的三英圖，按任意鍵進開場詞。
	titlePic *assets.Image

	// poem 非 nil 表示停在開場詞那一張，按任意鍵進主選單。
	poem *assets.Image
}

// uiReliefGold 是介面上「開倉賑民」一次撥出去的金。**remake 自選**：
// 原版的量由電腦諸侯的回合預算決定，玩家那一邊的取值還沒讀
// （`docs/mechanics/70-ai` §2.13.3）。
const uiReliefGold = 100

func (a *app) Update() error {
	if a.quit {
		return ebiten.Termination
	}
	if a.titlePic != nil {
		if anyKeyPressed() {
			a.titlePic = nil
			a.dirty = true
		}
		return nil
	}
	if a.poem != nil {
		if anyKeyPressed() {
			a.poem = nil
			a.dirty = true
		}
		return nil
	}
	if a.menuScreen != nil {
		return a.updateTitle()
	}
	if a.fight != nil {
		return a.updateBattle()
	}
	if inpututil.IsKeyJustPressed(ebiten.KeyEscape) {
		a.menu, a.view.Menu, a.view.Items, a.pick, a.num = 0, "", nil, nil, nil
		a.view.Prompt, a.view.Page = "", nil
		a.dirty = true
		return nil
	}
	if a.num != nil {
		if inpututil.IsKeyJustPressed(ebiten.KeyBackspace) {
			a.num.value /= 10
			a.showNumber()
			a.dirty = true
			return nil
		}
		if inpututil.IsKeyJustPressed(ebiten.KeyEnter) ||
			inpututil.IsKeyJustPressed(ebiten.KeyNumpadEnter) {
			n := a.num
			a.num = nil
			a.menu, a.view.Menu, a.view.Items = 0, "", nil
			n.then(n.value)
			a.dirty = true
			return nil
		}
		for k := ebiten.Key0; k <= ebiten.Key9; k++ {
			if inpututil.IsKeyJustPressed(k) {
				a.numberKey(byte('0' + (k - ebiten.Key0)))
				a.dirty = true
				return nil
			}
		}
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
	// M 換下一首配樂。原版的「其他」底下沒有這一格——那是主選單的
	// 「音樂欣賞」，開局之後回不去——所以放在鍵盤上。
	if inpututil.IsKeyJustPressed(ebiten.KeyM) {
		a.jb.Play(a.jb.Next())
		if name := a.jb.Name(); name != "" {
			a.view.Prompt = tf("msg.music", name)
		} else {
			a.view.Prompt = t("msg.noMusic")
		}
		a.dirty = true
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
			a.view.Prompt = t("msg.noSuchItem")
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
				a.view.Prompt = t("msg.statusHere")
			} else {
				a.view.Prompt = tf("msg.notYet", commandName(k))
			}
			return
		}
		a.menu, a.view.Menu, a.view.Items = k, title, items
		a.view.Prompt, a.view.Page = "", nil
		return
	}
	a.begin(a.menu, k)
}

// numEntry 是「請輸入一個數字」。
//
// 原版問數字的地方很多：徵兵幾人、賞賜多少金、攜帶多少金米、
// 延遲時間。**那些都不是固定量**，用固定量等於把玩家的決定拿掉了。
type numEntry struct {
	title string
	hint  string
	max   int
	value int
	then  func(int)
}

// askNumber 開一個數字輸入。0 也是合法的答案。
func (a *app) askNumber(title, hint string, max int, then func(int)) {
	if max < 0 {
		max = 0
	}
	a.num = &numEntry{title: title, hint: hint, max: max, then: then}
	a.showNumber()
}

// showNumber 把目前輸入的數字畫到選單欄上。
func (a *app) showNumber() {
	n := a.num
	if n == nil {
		return
	}
	a.view.Menu = n.title
	a.view.Items = []ui.Command{
		{Key: '=', Name: fmt.Sprintf("%d", n.value)},
		{Key: ' ', Name: tf("fld.max", n.max)},
	}
	a.view.Prompt = n.hint + t("hint.number")
}

// numberKey 收數字輸入的一個按鍵。
func (a *app) numberKey(k byte) {
	n := a.num
	v := n.value*10 + int(k-'0')
	// **超過上限就停在上限**，不要讓它捲成一個小數字：
	// 玩家多按一位卻換來「只徵了三個兵」是最難查的那種意外。
	if v > n.max {
		v = n.max
	}
	n.value = v
	a.showNumber()
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
		a.askSlot(t("ask.saveSlot"), true, func(slot int) {
			_ = a.s.Save(a.saveDir, slot, "")
		})
	case cat == '9' && item == '3':
		a.view.Prompt = g.Options.ToggleMusic()
		// 開關要真的動到聲音——按下去什麼都不會變的選項，
		// 與「這個功能還沒做」在畫面上長得一模一樣。
		a.jb.SetSilent(g.Options.MusicOff)
		if name := a.jb.Name(); name != "" && !g.Options.MusicOff {
			a.view.Prompt += "　（" + name + "）"
		}
		closeMenu()
	case cat == '9' && item == '5':
		// 原版收 0–100，0 表示訊息停在畫面上等按鍵
		//（`設定延遲時間(%d)\n0:等待按鍵(0-100):`，`docs/re/04` §3）。
		a.askNumber(t("ask.delay"), tf("hint.delay", g.Options.Delay()), 100, func(v int) {
			if err := g.Options.SetDelay(v); err != nil {
				a.view.Prompt = game.ErrorText(err)
				return
			}
			a.view.Prompt = tf("oth.delay.set", v)
		})
	case cat == '9' && item == '4':
		a.view.Prompt = g.Options.ToggleSound()
		// 關音效連語音都不出聲——原版的 `speak()` 自己就擋在這一道
		//（`docs/spec/008` §3）。
		a.sfx.SetGates(g.Options.SoundOff, g.Options.VoiceOff)
		a.sfx.Click()
		closeMenu()
	case cat == '9' && item == '8':
		a.view.Prompt = g.Options.ToggleVoice()
		a.sfx.SetGates(g.Options.SoundOff, g.Options.VoiceOff)
		closeMenu()
	case cat == '9' && item == '7':
		a.view.Prompt = g.Options.ToggleCalendar()
		a.view.Calendar = g.Options.Calendar
		closeMenu()
	case cat == '9' && item == '6':
		// 原版的 `查看電腦戰役%s` 是開關；順便把最近幾場列出來。
		a.view.Prompt = g.Options.ToggleAIWar()
		a.view.PageTitle, a.view.Page = ui.BattleList(g, s.Battles())
		closeMenu()
	case cat == '9' && item == '1':
		// 「＊結束」在原版是回到主選單。這裡先提醒存檔——
		// **沒存就離開是最貴的一次誤按**。
		a.view.Prompt = t("msg.quitHint")
		closeMenu()

	// ---- 1. 查看（不耗指令）----
	case cat == '1' && item == '1':
		a.askOwn(t("ask.pref"), func(id int) { a.view.Sel = id })
	case cat == '1' && item == '3':
		a.askGeneral(t("ask.inspect"), func(gi int) {
			a.view.PageTitle, a.view.Page = ui.GeneralPage(g, gi)
		})
		closeMenu()
	case cat == '1' && item == '5':
		name := tf("msg.prefN", sel)
		if p := g.Prefecture(sel); p != nil {
			name = p.Name
		}
		f := g.Field(sel)
		a.view.PageTitle, a.view.Page = ui.TerrainPage(name, f, f.Gates)
		closeMenu()
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
		a.askGeneral(t("ask.moveWho"), func(gi int) {
			a.askNeighbour(t("ask.moveTo"), true, func(to int) {
				a.run(game.MoveOrder{At: sel, To: to, General: gi})
			})
		})
	case cat == '2' && item == '2':
		a.askNeighbour(t("ask.attack"), false, func(to int) {
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
			// **玩家親自指揮**：先問攜帶的錢糧（原版 `攜帶多少金`／
			// `攜帶多少米`，而且把三十天要多少米算給你看），
			// 再進主戰場。電腦諸侯的戰役還是走 AttackOrder → Auto。
			closeMenu()
			p := g.Prefecture(sel)
			men := g.CampaignForce(force)
			need := game.RiceForCampaign(men)
			a.askNumber(t("ask.gold"), tf("hint.gold", p.Gold), p.Gold,
				func(gold int) {
					a.askNumber(t("ask.rice"),
						tf("hint.campaign", p.Rice, men, need), p.Rice,
						func(rice int) {
							a.startBattle(sel, to, force,
								game.Supply{Gold: gold, Rice: rice})
						})
				})
		})
	case cat == '2' && item == '3':
		a.askOwn(t("ask.sendTo"), func(to int) {
			p := g.Prefecture(sel)
			a.askNumber(t("ask.sendGold"), tf("hint.gold", p.Gold), p.Gold,
				func(gold int) {
					a.askNumber(t("ask.sendRice"), tf("hint.rice", p.Rice), p.Rice,
						func(rice int) {
							a.run(game.TransportOrder{At: sel, To: to,
								Gold: gold, Rice: rice})
						})
				})
		})

	// ---- 3. 兵士 ----
	case cat == '3' && item == '1':
		// 原版是對整個守軍訓練，不挑人（`game.State.Train`，`L0`），
		// 所以這裡不再問「訓練誰」。
		a.run(game.TrainOrder{At: sel})
	case cat == '3' && item == '2':
		a.askGeneral(t("ask.conscript"), func(gi int) {
			x := g.General(gi)
			cap := 0
			if x != nil {
				cap = x.TroopCap() - x.Soldiers
			}
			room := g.Prefecture(sel).Population - game.MinPopulationToConscript
			if room < 0 {
				room = 0
			}
			if cap > room {
				cap = room
			}
			a.askNumber(t("ask.conscriptN"),
				tf("hint.conscript", g.Prefecture(sel).Population),
				cap, func(n int) {
					a.run(game.ConscriptOrder{At: sel, General: gi, Count: n})
				})
		})
	case cat == '3' && item == '3':
		a.askGeneral(t("ask.arms"), func(gi int) {
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
		a.askGeneral(t("ask.reclaim"), func(gi int) { a.run(game.ReclaimOrder{At: sel, General: gi}) })
	case cat == '4' && item == '2':
		a.askGeneral(t("ask.flood"), func(gi int) { a.run(game.FloodControlOrder{At: sel, General: gi}) })
	case cat == '4' && item == '3':
		a.askGeneral(t("ask.fort"), func(gi int) { a.run(game.BuildFortOrder{At: sel, General: gi}) })
	case cat == '4' && item == '4':
		a.run(game.RestOrder{At: sel})

	// ---- 5. 商業 ----
	case cat == '5' && item == '1':
		a.run(game.BuyRiceOrder{At: sel, Units: 1000})
	case cat == '5' && item == '2':
		a.run(game.SellRiceOrder{At: sel, Units: 1000})
	case cat == '5' && item == '3':
		a.run(game.ReliefOrder{At: sel, Gold: uiReliefGold})

	// ---- 6. 人事 ----
	case cat == '6' && item == '1':
		a.askGeneral(t("ask.search"), func(gi int) { a.run(game.SearchOrder{At: sel, General: gi}) })
	case cat == '6' && item == '2':
		a.askFree(t("ask.recruit"), func(gi int) { a.run(game.RecruitOrder{At: sel, Target: gi}) })
	case cat == '6' && item == '3':
		a.askGeneral(t("ask.reward"), func(gi int) {
			// 原版問的是「賞賜%s多少金」，上限 100（手冊 p.23）。
			max := game.MaxReward
			if p := g.Prefecture(sel); p != nil && p.Gold < max {
				max = p.Gold
			}
			a.askNumber(t("ask.rewardGold"), t("hint.reward"), max, func(n int) {
				a.run(game.RewardOrder{At: sel, Target: gi, Gold: n})
			})
		})
	case cat == '6' && item == '4':
		a.askGeneral(t("ask.dismiss"), func(gi int) { a.run(game.DismissOrder{At: sel, Target: gi}) })

	// ---- 7. 君主 ----
	case cat == '7' && item == '1':
		a.askGeneral(t("ask.chief"), func(gi int) { a.run(game.AppointChiefOrder{At: sel, Target: gi}) })
	case cat == '7' && item == '2':
		a.askGeneral(t("ask.governor"), func(gi int) { a.run(game.AppointGovernorOrder{At: sel, Target: gi}) })
	case cat == '7' && item == '3':
		a.pickFrom(t("ask.autonomy"), []pickItem{
			{t("auto.normal"), int(game.AutoNormal), a.setAutonomy},
			{t("auto.civil"), int(game.AutoCivil), a.setAutonomy},
			{t("auto.military"), int(game.AutoMilitary), a.setAutonomy},
			{t("auto.self"), int(game.AutoSelf), a.setAutonomy},
		})
	case cat == '7' && item == '4':
		a.pickFrom(t("ask.gift"), []pickItem{
			{t("tre.book"), int(game.TreasureBook), a.giftThen},
			{t("tre.blade"), int(game.TreasureBlade), a.giftThen},
			{t("tre.beauty"), int(game.TreasureBeauty), a.giftThen},
			{t("tre.horse"), int(game.TreasureHorse), a.giftThen},
		})
	case cat == '7' && item == '5':
		a.askEnemyGeneral(t("ask.headhunt"), func(gi int) { a.run(game.HeadhuntOrder{At: sel, Target: gi}) })

	// ---- 8. 謀略 ----
	case cat == '8':
		plot := game.Plot(item - '0')
		a.askGeneral(t("ask.envoy"), func(gi int) {
			a.askNeighbour(t("ask.plotAt"), false, func(to int) {
				a.run(game.PlotOrder{At: sel, To: to, What: plot, Envoy: gi})
			})
		})

	default:
		a.view.Prompt = t("msg.noSuchItem")
	}
	if len(a.pick) == 0 {
		closeMenu()
	}
}

func (a *app) setAutonomy(mode int) {
	a.run(game.AutonomyOrder{At: a.view.Sel, Mode: game.Autonomy(mode)})
}

func (a *app) giftThen(what int) {
	a.askGeneral(t("ask.giftTo"), func(gi int) {
		a.run(game.GiftOrder{At: a.view.Sel, Target: gi, What: game.Treasure(what)})
	})
}

// run 送出一個命令並清掉選單狀態。
func (a *app) run(o game.Order) {
	a.pick = nil
	a.menu, a.view.Menu, a.view.Items = 0, "", nil
	a.view.Page = nil
	if err := a.s.Do(o); err != nil {
		a.view.Prompt = game.ErrorText(err)
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
		a.view.Prompt = t("msg.noTargets")
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
		a.view.Prompt = t("msg.noSuchPref")
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
		a.view.Prompt = t("msg.noSuchPref")
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
		a.view.Prompt = t("msg.noSaveDir")
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
		a.view.Prompt = t("msg.noSaves")
		return
	}
	a.pickFrom(title, items)
}

// t／tf 取一句介面文字。語系與畫面同一份（`internal/ui`）。
func t(key string) string            { return i18n.S(key) }
func tf(key string, a ...any) string { return i18n.Sf(key, a...) }

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
		switch {
		case a.titlePic != nil:
			ui.DrawImage(a.canvas, a.titlePic)
		case a.poem != nil:
			ui.DrawPoem(a.canvas, a.poem)
		case a.menuScreen != nil:
			a.drawTitle()
		case a.fight != nil:
			if a.artBattle != nil {
				ui.DrawArtBattle(a.canvas, a.artBattle, a.fight.pending.Battle(),
					a.fight.view, a.battleInfo())
			} else {
				ui.DrawBattle(a.canvas, a.fight.pending.Battle(), a.fight.view)
			}
		default:
			a.view.Over = a.s.Over
			if a.art != nil {
				ui.DrawArtSession(a.canvas, a.art, a.s.G, a.s.Log, a.view)
			} else {
				ui.DrawSession(a.canvas, a.s.G, a.s.Log, a.view)
			}
		}
		a.screen.WritePixels(a.canvas.Img.Pix)
		a.dirty = false
	}
	dst.DrawImage(a.screen, nil)
}

// updateBattle 是戰場上的輸入。
//
// 方向鍵移游標（查看與用計要先指目標），數字鍵是指令，
// Esc 收起覆蓋頁——**Esc 不會離開戰役**：出兵是不能反悔的。
func (a *app) updateBattle() error {
	defer func() { a.dirty = true }()
	if inpututil.IsKeyJustPressed(ebiten.KeyEscape) {
		a.fight.view.Page, a.fight.view.PageTitle = nil, ""
		return nil
	}
	for key, d := range map[ebiten.Key]battle.Dir{
		ebiten.KeyUp:    battle.DirUp,
		ebiten.KeyDown:  battle.DirDown,
		ebiten.KeyLeft:  battle.DirUpLeft,
		ebiten.KeyRight: battle.DirDownRight,
	} {
		if inpututil.IsKeyJustPressed(key) {
			a.battleMove(d)
			return nil
		}
	}
	for k := ebiten.Key0; k <= ebiten.Key9; k++ {
		if inpututil.IsKeyJustPressed(k) {
			a.battleKey(byte('0' + (k - ebiten.Key0)))
			return nil
		}
	}
	return nil
}

func (a *app) Layout(int, int) (int, int) {
	return ui.Cols * ui.CellW, ui.Rows * ui.CellH
}

func main() {
	root := flag.String("root", "", "原版遊戲目錄（必填，玩家自備）")
	saveDir := flag.String("saves", "saves", "存檔目錄（remake 自己的，不寫回原版）")
	load := flag.Int("load", 0, "開場就讀第幾個進度（1..6）；0 ＝ 開新局")
	origLoad := flag.Int("orig-load", 0,
		"改讀**原版**存的第幾個進度（1..6，從 DATA2.GRP 讀，只讀不寫）；0 ＝ 不讀")
	calendar := flag.String("calendar", "中曆", "年月的表示方式：中曆／西曆（原版「其他 → 年號」）")
	fontPath := flag.String("font", "fonts/unifont.hex.gz", "點陣字型")
	slot := flag.String("slot", "001", "劇本：001..006")
	faction := flag.Int("faction", -1, "玩家的勢力槽號；−1 ＝ 第一個在用的")
	aiMode := flag.String("ai", string(ai.ModeEnhanced),
		"電腦 AI：base（原版還原）／plus（加強版還原）／enhanced（remake 強化）")
	edition := flag.String("edition", string(state.EditionBase),
		"版本：base（原版）／plus（加強版）——只切已量到的規則差異，見 docs/spec/004")
	difficulty := flag.Int("difficulty", 5, "難度；上限看版本，原版 1..10、加強版 1..20")
	scale := flag.Int("scale", 2, "視窗放大倍率（整數倍，不做非整數縮放）")
	lang := flag.String("lang", "zh-Hant", "介面語言：zh-Hant／en／ja")
	music := flag.Bool("music", true, "播配樂（從原版的 DATA1 邊播邊合成）")
	sound := flag.Bool("sound", true, "播 PC 喇叭的音效與語音（原版的 S000.SND／R???.OKR）")
	useArt := flag.Bool("art", true, "主畫面用原版素材（從玩家自己的 DATA3 讀）")
	showTitle := flag.Bool("title", true, "先進開場詞與主選單；false ＝ 直接開局")
	flag.Parse()
	if l, ok := i18n.Parse(*lang); ok {
		i18n.Current = l
	} else {
		fmt.Fprintf(os.Stderr, "不認識的語言 %q，用繁體中文\n", *lang)
	}
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

	c, err := openContainer(*root, "DATA2")
	if err != nil {
		die(err)
	}
	ed, err := state.ParseEdition(*edition)
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
	if *origLoad > 0 {
		s, err = session.LoadOriginal(c, *origLoad, ed, ai.Mode(*aiMode))
		if err != nil {
			die(err)
		}
		g, brain = s.G, s.Brain
		f = int(s.Player)
	} else if *load > 0 {
		s, err = session.Load(*saveDir, *load, ai.Mode(*aiMode))
		if err != nil {
			die(err)
		}
		g, brain = s.G, s.Brain
		f = int(s.Player)
	} else {
		g, err = game.New(sc, state.FactionID(f), *difficulty, ed)
		if err != nil {
			die(err)
		}
		if err := ai.CheckEdition(ai.Mode(*aiMode), ed); err != nil {
			die(err)
		}
		brain, err = ai.New(ai.Mode(*aiMode))
		if err != nil {
			die(err)
		}
		s = session.New(g, brain, state.FactionID(f))
	}

	// 接原版素材：主畫面的底圖與肖像來自玩家自己那一份 `DATA3`。
	// **讀不到就退回文字版面**，不要讓少一個檔案變成開不起來。
	var art *ui.ArtScreen
	var artBattle *ui.ArtBattle
	var titleScreen *ui.TitleScreen
	var poem *assets.Image
	var titlePic *assets.Image
	if *useArt {
		if c3, err := openContainer(*root, "DATA3"); err == nil {
			if titleScreen, err = ui.NewTitleScreen(c3); err != nil {
				fmt.Fprintln(os.Stderr, "san1：主選單的素材讀不進來：", err)
				titleScreen = nil
			}
			// `DATA1` 給的是州郡的填色圖樣與主戰場的素材；讀不到就
			// 各自退回 remake 自己的版面，主畫面照樣接得上。
			c1, _ := openContainer(*root, "DATA1")
			if art, err = ui.NewArtScreen(c3, c1); err != nil {
				fmt.Fprintln(os.Stderr, "san1：原版素材讀不進來，改用文字版面：", err)
				art = nil
			}
			if art != nil && c1 != nil {
				if artBattle, err = ui.NewArtBattle(c1, c3); err != nil {
					fmt.Fprintln(os.Stderr, "san1：主戰場的素材讀不進來：", err)
					artBattle = nil
				}
				if poem, err = assets.PoemScreen(c1); err != nil {
					poem = nil
				}
				if titlePic, err = assets.TitleArt(c1); err != nil {
					titlePic = nil
				}
			}
		} else {
			fmt.Fprintln(os.Stderr, "san1：沒有 DATA3，改用文字版面：", err)
		}
	}
	cw, ch := ui.Cols*ui.CellW, ui.Rows*ui.CellH
	if art != nil {
		cw, ch = assets.ScreenW, assets.ScreenH
	}
	a := &app{
		canvas:  ui.NewCanvasPx(cw, ch, face),
		screen:  ebiten.NewImage(cw, ch),
		s:       s,
		dirty:   true,
		saveDir: *saveDir,
		aiMode:  ai.Mode(*aiMode),
		art:     art,
	}
	a.artBattle = artBattle
	a.c2 = c
	a.edition = ed
	if *music {
		a.jb = newJukebox(*root)
		a.jb.Play(0)
	}
	if *sound {
		a.sfx = newVoicebox(*root)
		// 開場就把選項接上：載進來的存檔可能本來就關著音效。
		a.sfx.SetGates(g.Options.SoundOff, g.Options.VoiceOff)
	}
	// 開場詞 → 主選單 → 遊戲。**指定了劇本以外的東西就直接進遊戲**：
	// `-load`／`-orig-load` 是「我要那一局」，中間再問一次沒有道理，
	// 而 `-title=false` 是給截圖與腳本用的。
	if *showTitle && titleScreen != nil && *load == 0 && *origLoad == 0 {
		a.startTitle(titleScreen, menu.New(c, ed, ai.Mode(*aiMode), *saveDir, a.jb.Len()))
		a.poem = poem
		a.titlePic = titlePic
	}
	if *calendar == "西曆" {
		g.Options.Calendar = game.Western
	}
	a.view.Calendar = g.Options.Calendar
	if own := s.PlayerTerritory(); len(own) > 0 {
		sort.Ints(own)
		a.view.Sel = own[0]
	}

	// 整數倍放大：CJK 點陣字非整數縮放會糊掉（`CLAUDE.md` §3.3）。
	if *scale < 1 {
		*scale = 1
	}
	ebiten.SetWindowSize(cw**scale, ch**scale)
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

func die(err error) {
	fmt.Fprintln(os.Stderr, "san1:", err)
	os.Exit(1)
}
