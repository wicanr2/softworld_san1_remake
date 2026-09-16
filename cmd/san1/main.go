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
	"image"
	"os"
	"path/filepath"
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

	// afterBubbles 是訊息框全部收掉之後要接著做的事（軍師勸諫之後問
	// Y/N、宣戰對白之後開打）；confirm 是等著的 Y/N。
	afterBubbles func()
	confirm      *confirmEntry

	// fight 非 nil 表示正在打一場玩家親自指揮的戰役。
	fight *fight

	// saveDir 是存檔目錄，空字串表示這一局不能存。
	saveDir string
	// jb 是配樂；沒有原版的 DATA1 就是 nil。
	jb *jukebox

	// sfx 是 PC 喇叭的音效與語音（`docs/spec/008`）；放不出聲音就是 nil。
	sfx *voicebox

	// wipe 非 nil 表示正在跑一段畫面轉場（`docs/spec/010`）。
	wipe *ui.Wipe

	// credits 非 nil 表示正在播製作群（`docs/spec/012`）；
	// creditsDone 記住這一局播過了，不要每一幀重播。
	credits     *creditsPlay
	creditsDone bool

	// art 是接上原版素材的主畫面；沒有原版的 DATA3 就是 nil，
	// 那時退回 remake 自己的文字版面。
	art *ui.ArtScreen

	// artBattle 是接上原版素材的主戰場，要 `DATA1` 與 `DATA3` 兩個。
	artBattle *ui.ArtBattle

	// menuScreen 非 nil 表示停在主選單那一層（開場詞按完就到這裡）；
	// titleArt 是那一張的底圖。
	menuScreen *menu.Screen
	titleArt   *ui.TitleScreen
	// titleAnimTick／Frame 驅動主選單 `CURA0`～`CURA5` 的六格循環。
	titleAnimTick, titleAnimFrame int

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
	// 轉場進行中就只走轉場：原版那一段是**阻塞**的（`docs/re/09` §5），
	// 期間不收輸入。24 步 ×一幀 ≈ 0.4 秒。
	if a.wipe != nil {
		a.stepWipe()
		return nil
	}
	// 統一之後播製作群。**要在其他輸入之前**：那一段自己收按鍵。
	if a.s != nil && a.s.Over {
		a.startCredits()
	}
	if a.updateCredits() {
		return nil
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
	// 訊息框（原版的「肖像＋對白」）一次一格，按任意鍵收掉。
	// **原版沒有這一步的按鍵**：語音開著時等語音播完，關著時走延遲設定
	// （`docs/spec/005` §9）；remake 用按鍵收，登記為 remake 差異。
	// 沒有原版素材的文字版面畫不出肖像，直接把它們寫進訊息列。
	if a.s != nil && a.s.Bubble() != nil {
		if a.art == nil {
			a.s.FlushBubbles()
			a.dirty = true
			return nil
		}
		if anyKeyPressed() {
			a.s.PopBubble()
			a.dirty = true
		}
		return nil
	}
	if next := a.afterBubbles; next != nil {
		a.afterBubbles = nil
		next()
		a.dirty = true
		return nil
	}
	// Y/N（軍師勸諫之後的「主公是否繼續呢」）：Y 做下去，N 或 Esc 取消。
	if a.confirm != nil {
		switch {
		case inpututil.IsKeyJustPressed(ebiten.KeyY):
			then := a.confirm.then
			a.confirm, a.view.Prompt = nil, ""
			then()
			a.dirty = true
		case inpututil.IsKeyJustPressed(ebiten.KeyN), inpututil.IsKeyJustPressed(ebiten.KeyEscape):
			a.confirm, a.view.Prompt = nil, ""
			a.dirty = true
		}
		return nil
	}
	// 人物資料卡（原版素材畫面的「查看→武將」）：按任意鍵收掉，
	// 右側面板回到原樣。
	if a.view.HasCard {
		if anyKeyPressed() {
			a.view.HasCard = false
			a.dirty = true
		}
		return nil
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
	// 分頁打開時 ↑↓ 捲一行、PgUp／PgDn 翻一頁——先前沒有捲動，
	// 一頁放不下的戰報與將軍列表，其餘的部分玩家讀不到。
	if len(a.view.Page) > 0 {
		if d := pageScrollKey(ui.PageSize(a.art != nil)); d != 0 {
			a.view.ScrollPage(d, a.art != nil)
			a.dirty = true
			return nil
		}
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
// pageScrollKey 是分頁的捲動量：↑↓ 一行、PgUp／PgDn 一頁；沒按回 0。
func pageScrollKey(_, rows int) int {
	switch {
	case inpututil.IsKeyJustPressed(ebiten.KeyUp):
		return -1
	case inpututil.IsKeyJustPressed(ebiten.KeyDown):
		return 1
	case inpututil.IsKeyJustPressed(ebiten.KeyPageUp):
		return -rows
	case inpututil.IsKeyJustPressed(ebiten.KeyPageDown):
		return rows
	}
	return 0
}

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
	// 換了郡就要看得到那一郡：上面板換成郡的資料（`docs/spec/014` §3.1）。
	a.view.Status = true
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
				// 原版的「0.狀態」：上面板換成郡的資料（`docs/spec/014` §2.1）。
				a.view.Status = true
				a.view.Prompt = t("msg.statusHere")
			} else {
				a.view.Prompt = tf("msg.notYet", commandName(k))
			}
			return
		}
		a.menu, a.view.Menu, a.view.Items = k, title, items
		a.view.Prompt, a.view.Page = "", nil
		// 子選單打開時上面板回到指令表，與原版相同（`docs/spec/014` §2.2）。
		a.view.Status = false
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
		a.view.SetPage(ui.BattleList(g, s.Battles()))
		closeMenu()
	case cat == '9' && item == '1':
		// 「＊結束」在原版是回到主選單。這裡先提醒存檔——
		// **沒存就離開是最貴的一次誤按**。
		a.view.Prompt = t("msg.quitHint")
		closeMenu()
	case cat == '9' && item == '9':
		// **remake 加的第九項**：換電腦用哪一版 AI（`docs/design/02` §5）。
		// `NextMode` 只在「跑得動這一版規則」的版本裡繞，所以按下去
		// 不會跳出「這個 AI 配不上這一版」——那種錯誤訊息玩家看起來
		// 就像功能壞了。
		next := ai.NextMode(a.s.Brain.Mode(), g.Edition)
		brain, err := ai.New(next)
		if err != nil {
			a.view.Prompt = game.ErrorText(err)
			break
		}
		a.s.SetBrain(brain)
		g.Options.SetAIMode(string(next))
		a.view.Prompt = tf("oth.ai.state", brain.Name())
		closeMenu()
	case cat == '9' && item == '0':
		// **remake 加的第十項**：強化 AI 一個郡一個月下幾道令。
		// 原版的電腦不受「每郡每月一道令」管（`docs/mechanics/70-ai`
		// §2.12），所以這一格調的是**強化 AI 有多強**不是規則；
		// 還原型的 AI 不看它。
		a.askNumber(t("ask.aiOrders"),
			tf("hint.aiOrders", g.Options.AIOrders(), game.AIOrdersMax),
			game.AIOrdersMax, func(v int) {
				if err := g.Options.SetAIOrders(v); err != nil {
					a.view.Prompt = game.ErrorText(err)
					return
				}
				a.view.Prompt = tf("oth.orders.set", v)
			})

	// ---- 1. 查看（不耗指令）----
	case cat == '1' && item == '1':
		a.askOwn(t("ask.pref"), func(id int) { a.view.Sel = id })
	case cat == '1' && item == '3':
		a.askGeneral(t("ask.inspect"), func(gi int) {
			// 原版素材畫面照原版畫人物資料卡在右側面板（`docs/spec/005`
			// §9.2）；文字版面沒有肖像，仍走整頁的 `GeneralPage`。
			if a.art != nil {
				a.view.Card, a.view.HasCard = gi, true
				return
			}
			a.view.SetPage(ui.GeneralPage(g, gi))
		})
		closeMenu()
	case cat == '1' && item == '5':
		name := tf("msg.prefN", sel)
		if p := g.Prefecture(sel); p != nil {
			name = p.Name
		}
		f := g.Field(sel)
		a.view.SetPage(ui.TerrainPage(name, f, f.Gates))
		closeMenu()
	case cat == '1' && item == '2':
		a.view.SetPage(ui.GeneralList(g, sel))
		closeMenu()
	case cat == '1' && item == '4':
		a.view.SetPage(ui.TerritoryList(g, s.Player))
		closeMenu()
	case cat == '1' && item == '6':
		a.view.SetPage(ui.TreasuryList(g, s.Player))
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
							// 軍師勸諫（`0x18b08`）之後是兩句宣戰（`0x202e1`／`0x20322`），
							// 對白收完才進主戰場。
							a.withAdvice(game.AdviceAttack, sel, game.AdviceTarget{To: to}, func() {
								a.s.Queue(g.WarDeclaration(sel, to, s.Player))
								a.afterBubbles = func() {
									a.startBattle(sel, to, force, game.Supply{Gold: gold, Rice: rice})
								}
							})
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

// confirmEntry 是等著的 Y/N；then 是按 Y 要做的事。
type confirmEntry struct{ then func() }

// askYN 問一句 Y/N。
func (a *app) askYN(prompt string, then func()) {
	a.confirm = &confirmEntry{then: then}
	a.view.Prompt = prompt
}

// run 送出一個命令並清掉選單狀態。**軍師先勸諫**（`docs/spec/005` §9.6）：
// 他開口就先秀那一格訊息框，再問「主公是否繼續呢(Y/N)」，N 取消。
func (a *app) run(o game.Order) {
	a.pick = nil
	a.menu, a.view.Menu, a.view.Items = 0, "", nil
	a.view.Page = nil
	a.withAdvice(adviceFor(o), o.Prefecture(), adviceTargetOf(o), func() { a.apply(o) })
}

// withAdvice 在做 then 之前讓軍師勸諫；沒開口就直接做。
func (a *app) withAdvice(kind game.AdviceKind, at int, target game.AdviceTarget, then func()) {
	adv := a.s.G.Advise(kind, at, a.s.Player, target)
	if adv == nil {
		then()
		return
	}
	a.s.Queue(adv.Events)
	a.afterBubbles = func() { a.askYN(t("ask.continue"), then) }
}

// adviceFor 是一道命令對應哪一種勸諫。
func adviceFor(o game.Order) game.AdviceKind {
	switch o.(type) {
	case game.MoveOrder:
		return game.AdviceMove
	case game.TrainOrder:
		return game.AdviceTrain
	case game.ConscriptOrder:
		return game.AdviceConscript
	case game.ArmsOrder:
		return game.AdviceArms
	case game.RedistributeOrder:
		return game.AdviceBalance
	case game.RestOrder:
		return game.AdviceRest
	case game.ReclaimOrder:
		return game.AdviceReclaim
	case game.FloodControlOrder:
		return game.AdviceFlood
	case game.BuildFortOrder:
		return game.AdviceFort
	case game.BuyRiceOrder:
		return game.AdviceBuy
	case game.SellRiceOrder:
		return game.AdviceSell
	case game.ReliefOrder:
		return game.AdviceRelief
	case game.SearchOrder:
		return game.AdviceSearch
	case game.RecruitOrder:
		return game.AdviceRecruit
	case game.RewardOrder:
		return game.AdviceReward
	case game.DismissOrder:
		return game.AdviceDismiss
	case game.HeadhuntOrder:
		return game.AdviceHeadhunt
	case game.PlotOrder:
		return game.AdvicePlot
	}
	return game.AdviceNone
}

// adviceTargetOf 是勸諫要看的對象（登用／挖角的目標、計略的目的郡與種類）。
func adviceTargetOf(o game.Order) game.AdviceTarget {
	switch v := o.(type) {
	case game.RecruitOrder:
		return game.AdviceTarget{Target: v.Target}
	case game.HeadhuntOrder:
		return game.AdviceTarget{Target: v.Target}
	case game.PlotOrder:
		return game.AdviceTarget{Target: v.Envoy, To: v.To, What: v.What}
	}
	return game.AdviceTarget{}
}

// apply 真的送出一個命令。
func (a *app) apply(o game.Order) {
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
	// **轉場要在 Draw 這一層起頭，不能在 Update。** 規則層設好
	// `PendingWipe` 的那一幀，畫布上還是**上一幀畫的舊畫面**；等到下一次
	// Update 才去拿，畫布早就被重畫成新的，兩張圖一樣，轉場等於沒跑。
	if a.wipe == nil {
		a.startWipe()
	}
	if a.wipe != nil {
		a.screen.WritePixels(a.canvas.Img.Pix)
		dst.DrawImage(a.screen, nil)
		return
	}
	if a.dirty {
		a.paint()
		a.screen.WritePixels(a.canvas.Img.Pix)
		a.dirty = false
	}
	dst.DrawImage(a.screen, nil)
}

// paint 把目前的狀態畫到畫布上。
//
// 從 Draw 抽出來是為了**轉場**：拉幕要先有「新畫面」才有東西可以露出來
// （`docs/spec/010`），而那張圖就是「照現在的狀態畫一次」。
func (a *app) paint() {
	{
		// 畫面內容在 internal/ui，Ebiten 這一層只負責貼上去——
		// 同一張圖無頭環境也產得出來（cmd/san1dump -png）。
		switch {
		case a.credits != nil && a.credits.hall:
			ui.DrawCreditHall(a.canvas, a.credits.art)
		case a.credits != nil:
			ui.DrawCredits(a.canvas, a.credits.art, a.credits.scroll)
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
				if sp := a.fight.speech(true); sp != nil {
					ui.DrawBattleSpeech(a.canvas, a.art, a.s.G, sp)
				}
			} else {
				ui.DrawBattle(a.canvas, a.fight.pending.Battle(), a.fight.view)
			}
		default:
			a.view.Over = a.s.Over
			if a.art != nil {
				ui.DrawArtSession(a.canvas, a.art, a.s.G, a.s.Log, a.view)
				if b := a.s.Bubble(); b != nil {
					// 原版在對白之前把右側面板的內部清成藍色（`0x1058:0x27e8`，
					// 外框留著）。
					ui.ClearPanel(a.canvas, 408, 36, 631, 291, assets.EGAPalette[1])
					ui.DrawBubble(a.canvas, a.art, a.s.G, b)
				}
			} else {
				ui.DrawSession(a.canvas, a.s.G, a.s.Log, a.view)
			}
		}
	}
}

// updateBattle 是戰場上的輸入。
//
// 方向鍵移游標（查看與用計要先指目標），數字鍵是指令，
// Esc 收起覆蓋頁——**Esc 不會離開戰役**：出兵是不能反悔的。
func (a *app) updateBattle() error {
	defer func() { a.dirty = true }()
	// 戰場對白（肖像＋泡泡）一次一格，按任意鍵收掉——與主畫面的訊息框
	// 同一個做法（remake 差異：原版走延遲設定）。
	if a.fight.speech(a.artBattle != nil) != nil {
		if anyKeyPressed() {
			a.fight.speeches = a.fight.speeches[1:]
		}
		return nil
	}
	if inpututil.IsKeyJustPressed(ebiten.KeyEscape) {
		a.fight.view.Page, a.fight.view.PageTitle = nil, ""
		return nil
	}
	// 分頁（查看部隊）打開時方向鍵捲動，不移游標。
	if len(a.fight.view.Page) > 0 {
		if d := pageScrollKey(ui.BattlePageSize(a.artBattle != nil)); d != 0 {
			a.fight.view.ScrollPage(d, a.artBattle != nil)
			return nil
		}
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
	// 探測音訊裝置的子行程（`audioprobe.go`）。要在 `flag.Parse` 之前攔下來，
	// 而且不能碰任何 Ebiten 的東西——那會把 oto 唯一的名額用掉。
	if len(os.Args) == 2 && os.Args[1] == probeAudioArg {
		os.Exit(probeAudioChild())
	}
	music := flag.Bool("music", true, "播配樂（從原版的 DATA1 邊播邊合成）")
	sound := flag.Bool("sound", true, "播 PC 喇叭的音效與語音（原版的 S000.SND／R???.OKR）")
	voiceDiv := flag.Int("voice-divisor", 0, "語音的分頻值（越大越慢；0 ＝ 預設，見 docs/spec/008 R8）")
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
	small := loadSmallFace(*fontPath)

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
			// `DATA1` 給的是小飾框動畫、州郡填色圖樣與主戰場素材。
			// 讀不到時主選單仍以靜態 MENU3 啟動，其餘各自退回文字版面。
			c1, _ := openContainer(*root, "DATA1")
			if titleScreen, err = ui.NewTitleScreen(c3, c1); err != nil {
				fmt.Fprintln(os.Stderr, "san1：主選單的素材讀不進來：", err)
				titleScreen = nil
			}
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
		art:     art,
	}
	a.canvas.SetSmallFace(small)
	a.artBattle = artBattle
	a.c2 = c
	a.edition = ed
	// **先確認音訊裝置開得起來**（`audioprobe.go`）：Ebiten 把驅動開不起來
	// 當成致命錯誤，沒有音效卡的機器會連遊戲都開不了。
	if *music || *sound {
		if ok, why := audioAvailable(); !ok {
			fmt.Fprintln(os.Stderr, "san1: 沒有可用的音訊裝置，配樂與音效都關掉：", why)
			*music, *sound = false, false
		}
	}
	if *music {
		a.jb = newJukebox(*root)
		a.jb.Play(0)
	}
	if *sound {
		a.sfx = newVoicebox(*root)
		a.sfx.SetDivisors(0, *voiceDiv)
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

// 畫面轉場（`docs/spec/010`）。
//
// 原版在計謀得手之後擲 `RND(4)` 選一個方向，把訊息面板那一塊逐條換掉，
// **每一步送一聲 PC 喇叭的音效**——音效在這一款不是事件音，是動畫的
// 節拍聲（`docs/spec/008` §4）。

// startWipe 看規則層有沒有留下待播的轉場，有就起一段。
//
// 拉幕要「舊畫面」與「新畫面」兩張：舊的是現在畫布上的，新的是
// **照現在的狀態再畫一次**。所以這裡畫一次、複製走、再把畫布還原。
func (a *app) startWipe() {
	if a.wipe != nil || a.s == nil || a.s.G == nil {
		return
	}
	k := a.s.G.TakeWipe()
	if k == game.NoWipe {
		return
	}
	// 沒有原版素材時退回文字版面，那個版面的訊息面板不在同一個位置
	// ——**寧可不轉場，也不要在錯的地方拉幕**。同理，只有主畫面那一層
	// 有那塊面板：標題、開場詞、戰場都不是。
	if a.art == nil || a.titlePic != nil || a.poem != nil ||
		a.menuScreen != nil || a.fight != nil {
		return
	}
	from := cloneCanvas(a.canvas.Img)
	a.paint()
	to := cloneCanvas(a.canvas.Img)
	copy(a.canvas.Img.Pix, from.Pix)
	a.wipe = &ui.Wipe{Kind: ui.WipeKind(k), Rect: ui.WipeRect, From: from, To: to}
	a.dirty = true
}

// stepWipe 走一步。走完就收掉，並讓下一幀重畫一次（收尾要是完整的新畫面）。
func (a *app) stepWipe() {
	if a.wipe == nil {
		return
	}
	if !a.wipe.Advance(a.canvas.Img) {
		a.wipe = nil
		a.dirty = true
		return
	}
	// **每一步一聲音效**——音效在這一款是動畫的節拍聲（`docs/spec/008` §4）。
	a.sfx.Click()
}

// cloneCanvas 複製一張畫布。
func cloneCanvas(src *image.RGBA) *image.RGBA {
	dst := image.NewRGBA(src.Bounds())
	copy(dst.Pix, src.Pix)
	return dst
}

// loadSmallFace 讀小字級（與大字型同一個目錄的 `ascii6x10.hex.gz`）。
// **讀不到不是錯誤**：沒有小字級只是英文在原版版面放不下的地方照原尺寸
// 退回別的排法（`docs/spec/014` §3.2），不該擋著開遊戲。
func loadSmallFace(bigFont string) *font.Face {
	p := filepath.Join(filepath.Dir(bigFont), "ascii6x10.hex.gz")
	fh, err := os.Open(p)
	if err != nil {
		fmt.Fprintln(os.Stderr, "san1: 沒有小字級，照原尺寸排：", err)
		return nil
	}
	defer fh.Close()
	f, err := font.ParseHexGz(fh, ui.SmallH)
	if err != nil {
		fmt.Fprintln(os.Stderr, "san1: 小字級讀不進來，照原尺寸排：", err)
		return nil
	}
	return f
}
