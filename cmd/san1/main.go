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
//	Enter            這個郡這個月休息，換下一個郡
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
	"github.com/wicanr2/softworld_san1_remake/internal/opening"
	"github.com/wicanr2/softworld_san1_remake/internal/session"
	"github.com/wicanr2/softworld_san1_remake/internal/speaker"
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
	// afterCard 是人物資料卡按任意鍵收掉之後要接著做的事（查看再問
	// 「檢視那位」、賜物接著列物品表）。
	afterCard func()
	// cancel 是挑選清單或數字輸入被空 Enter／Esc 收掉時要接著做的事
	// （賞賜物品的「那一位」收掉回到「那一郡」）；nil 表示照一般規則收。
	cancel func()

	// fight 非 nil 表示正在打一場玩家親自指揮的戰役。
	fight *fight

	// saveDir 是存檔目錄，空字串表示這一局不能存。
	saveDir string
	// jb 是配樂；沒有原版的 DATA1 就是 nil。
	jb *jukebox

	// sfx 是 PC 喇叭的音效與語音（`docs/spec/008`）；放不出聲音就是 nil。
	sfx *voicebox

	// wipe 非 nil 表示正在把一張場景圖拉進畫面（`docs/spec/010`）；
	// scenePlayed 是拉過（或正在拉）的那一格，同一格不重播。
	wipe        *ui.Wipe
	scenePlayed any

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
	// cursorTick 驅動提示後面輸入游標的六格（`ui.CursorFrameAt`）。
	cursorTick int

	// c2 是 `DATA2`：主選單要重讀劇本，得留著。
	c2 *assets.Container

	// edition 是版本旗標，開新局時要用。
	edition state.Edition

	// quit 為真表示玩家選了「回作業系統」。
	quit bool

	// marchArt 是大地圖戰役動畫的 `CVSC00`–`CVSC23`；讀不到是 nil，那一格直接跳過。
	marchArt *[ui.MarchArtCount]*assets.Image
	// mapBattle 是播放中的那一場大地圖戰役動畫。
	mapBattle *mapBattlePlay
	// lure 是播放中的誘敵特效（主戰場對白佇列裡的那一格）。
	lure lurePlay

	// saving 非 nil 表示停在存檔那一格；saveMemo 是正在打的備註。
	saving   *ui.SaveScreen
	saveMemo string

	// bubbleFrames 是示範模式裡目前這一格訊息框停了幾幀。
	bubbleFrames int

	// newMenu 開一份新的主選單（示範模式按鍵回主選單用）；沒有主選單是 nil。
	newMenu func() *menu.Screen

	// opening 非 nil 表示正在播開機片頭（`docs/spec/005`「片頭」），
	// 播完才到主選單。
	opening *openingPlayer
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
	if a.cursorTick++; a.cursorTick%ui.CursorTicksPerFrame == 0 && (a.s != nil || a.menuScreen != nil) {
		a.dirty = true
	}
	// 統一之後播製作群。**要在其他輸入之前**：那一段自己收按鍵。
	if a.s != nil && a.s.Over {
		a.startCredits()
	}
	if a.updateCredits() {
		return nil
	}
	if a.opening != nil {
		changed, done := a.opening.update(anyKeyPressed())
		if done {
			a.opening.stop()
			a.opening = nil
		}
		if changed || done {
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
		demo := len(a.s.G.Players) == 0
		if demo && anyKeyPressed() && a.endDemo() {
			return nil
		}
		if a.art == nil {
			a.s.FlushBubbles()
			a.dirty = true
			return nil
		}
		if b := a.s.Bubble(); b.MapBattle != nil {
			// 大地圖上的戰役自己播完就收，不等鍵（原版不收鍵）。
			if a.updateMapBattle(b) {
				a.s.PopBubble()
			}
			a.dirty = true
			return nil
		}
		if b := a.s.Bubble(); b.Wiped() && a.scenePlayed != b {
			// 場景圖那一格先拉進來（Draw 那一層起頭），拉完才收鍵。
			return nil
		}
		// 示範模式沒有人按鍵：原版把延遲壓到 2（`es:0x2f72`）自己往下走；
		// remake 每一格停 demoBubbleFrames 幀（remake 差異），按鍵是結束示範。
		if demo {
			if a.bubbleFrames++; a.bubbleFrames >= demoBubbleFrames {
				a.bubbleFrames = 0
				a.s.PopBubble()
				a.dirty = true
			}
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
	// 存檔那一格：先問 1–6，再打備註（`docs/spec/005` §9.9）。
	if a.saving != nil {
		a.updateSaving()
		return nil
	}
	// 0 人的電腦自動示範模式：一幀推一格，按任意鍵回主選單（`docs/spec/019` §2）。
	if a.s != nil && len(a.s.G.Players) == 0 {
		if anyKeyPressed() && a.endDemo() {
			return nil
		}
		a.s.AdvanceToHuman(1)
		a.view.Prompt = t("msg.demoEnd")
		a.dirty = true
		return nil
	}
	// 輪流下令：沒停在玩家的郡就照這個月的順序往下跑，停在下一個玩家的郡。
	if a.s != nil && a.menu == 0 && a.pick == nil && a.num == nil && a.s.Waiting() == 0 && !a.s.Over {
		if at := a.s.AdvanceToHuman(0); at != 0 {
			a.view.Sel, a.view.Status = at, true
			a.view.Prompt = tf("msg.prefTurn", lordName(a.s.G, a.s.Player), prefName(a.s.G, at))
		}
		a.dirty = true
		return nil
	}
	// 人物資料卡（原版素材畫面的「查看→武將」）：按任意鍵收掉，
	// 右側面板回到原樣。郡地理誌同一個做法（原版畫在第二頁，等鍵切回）。
	if a.view.HasCard || a.view.Atlas != 0 {
		if anyKeyPressed() {
			a.view.HasCard, a.view.Atlas, a.view.AtlasBubble = false, 0, nil
			a.view.Prompt = ""
			a.dirty = true
			if next := a.afterCard; next != nil {
				a.afterCard = nil
				next()
			}
		}
		return nil
	}
	if next := a.cancel; next != nil && (len(a.pick) > 0 || a.num != nil) &&
		(inpututil.IsKeyJustPressed(ebiten.KeyEscape) || (len(a.pick) > 0 &&
			(inpututil.IsKeyJustPressed(ebiten.KeyEnter) ||
				inpututil.IsKeyJustPressed(ebiten.KeyNumpadEnter)))) {
		a.cancel = nil
		a.menu, a.view.Menu, a.view.Items, a.pick, a.num = 0, "", nil, nil, nil
		a.view.Page = nil
		next()
		a.dirty = true
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
			a.num, a.cancel = nil, nil
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
		// 這個郡這個月不下令，換下一個郡（原版主命令的「休息」）。
		if a.s.Waiting() != 0 {
			a.s.EndTurn()
		}
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
		a.pick, a.cancel = nil, nil
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
	a.num, a.cancel = &numEntry{title: title, hint: hint, max: max, then: then}, nil
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
		a.startSaving()
		closeMenu()
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
		// 挑選清單留著（結尾那一句只在沒有清單時收選單）——先前這裡
		// 多收一次，「檢視那位」的名單畫不出來。
		a.inspectGeneral(sel)
	case cat == '1' && item == '5':
		name := tf("msg.prefN", sel)
		if p := g.Prefecture(sel); p != nil {
			name = p.Name
		}
		f := g.Field(sel)
		// 原版素材畫面照原版畫整張地理誌（場地圖、通道編號、主事者那一句，
		// `docs/spec/005` §9.8）；文字版面仍走分頁。
		if a.artBattle != nil {
			a.view.Atlas, a.view.AtlasBubble = sel, g.AtlasBubble(sel)
			closeMenu()
			return
		}
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
							// 軍師勸諫（`0x18b08`）之後先播 `SCG06`（`0x18b82`），再是兩句
							// 宣戰（`0x202e1`／`0x20322`），對白收完才進主戰場。
							a.withAdvice(game.AdviceAttack, sel, game.AdviceTarget{To: to}, func() {
								a.s.Queue(g.WarScene(sel, s.Player))
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
		// 原版（`0x1cfd6`）是兩層迴圈：那一郡 → 那一位 → 物品 → 再問那一位；
		// 空 Enter 回那一郡，再空 Enter 收掉（`docs/spec/005` §9.2）。
		r, err := g.OpenGift(sel, s.Player)
		if err != nil {
			a.view.Prompt = game.ErrorText(err)
			break
		}
		closeMenu()
		a.giftPref(r)
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
	if len(a.pick) == 0 && a.num == nil {
		closeMenu()
	}
}

func (a *app) setAutonomy(mode int) {
	a.run(game.AutonomyOrder{At: a.view.Sel, Mode: game.Autonomy(mode)})
}

// giftPref 是「賞賜那一郡的將軍」（`0x1d4ec`：數字 1–42，不在清單裡就
// 重問）。空 Enter 收掉這道命令：賞出過東西就是這個郡的回合走完，
// 一件都沒送回主選單（`0x17791`）。
func (a *app) giftPref(r *game.GiftRound) {
	done := func() {
		// 地圖的選取回到下令的郡（`0x1c855`：`0x1058:0x6b2(0, es:0x30fc)`）。
		a.view.Page, a.view.Sel = nil, r.At
		if a.s.CloseGift(r) {
			a.view.Prompt = ""
		}
	}
	a.askNumber(t("ask.giftPref"), "", state.PrefectureCount, func(pref int) {
		if pref == 0 {
			done()
			return
		}
		if !a.s.G.GiftPrefectureOK(r, pref) {
			a.giftPref(r)
			return
		}
		a.view.Sel = pref
		a.giftWho(r, pref)
	})
	a.cancel = done
}

// giftWho 是「賞賜那一位」（`0x1d0b0`）：挑到人畫他的卡、等鍵看物品表；
// 這道命令裡賞過的印「%s已賞賜過了」再問；空 Enter 印「取消」回那一郡。
func (a *app) giftWho(r *game.GiftRound, pref int) {
	g := a.s.G
	back := func() {
		a.view.Prompt = t("msg.giftCancel")
		a.giftPref(r)
	}
	var items []pickItem
	for _, x := range g.GiftCandidates(pref) {
		items = append(items, pickItem{x.Name, x.Index, func(gi int) {
			if r.Gifted(gi) {
				a.view.Prompt = tf("msg.gifted", i18n.PersonName(g.General(gi).Name))
				a.giftWho(r, pref)
				return
			}
			if a.art == nil {
				a.giftPick(r, pref, gi)
				return
			}
			a.view.Card, a.view.HasCard = gi, true
			a.view.Prompt = t("ask.giftItems")
			a.afterCard = func() { a.giftPick(r, pref, gi) }
		}})
	}
	if len(items) == 0 {
		a.view.Prompt = t("msg.noTargets")
		a.giftPref(r)
		return
	}
	a.pickFrom(t("ask.giftTo"), items)
	a.cancel = back
}

// giftPick 列出君主物品表、問賞哪一件（`0x1d1e5`：2–5，沒有那一件重問）。
// 送完排道謝與卡片，收掉之後回到「賞賜那一位」。
func (a *app) giftPick(r *game.GiftRound, pref, gi int) {
	a.view.SetPage(ui.TreasuryList(a.s.G, a.s.Player))
	again := func() { a.giftWho(r, pref) }
	then := func(what int) {
		f := a.s.G.Faction(a.s.Player)
		if f == nil || f.Treasury[what] <= 0 {
			a.giftPick(r, pref, gi)
			return
		}
		a.view.Page = nil
		a.apply(game.GiftOrder{At: r.At, Target: gi, What: game.Treasure(what), Round: r})
		a.afterBubbles = again
	}
	a.pickFrom(t("ask.gift"), []pickItem{
		{t("tre.book"), int(game.TreasureBook), then},
		{t("tre.blade"), int(game.TreasureBlade), then},
		{t("tre.beauty"), int(game.TreasureBeauty), then},
		{t("tre.horse"), int(game.TreasureHorse), then},
	})
	a.cancel = func() {
		a.view.Prompt = t("msg.giftCancel")
		again()
	}
}

// inspectGeneral 是查看→3.檢視將軍（`0x17cba`，`docs/spec/005` §9.2）：
// 看別人的郡先問「此郡非我軍所有 確定查看(Y/N)」，然後「檢視那位」→
// 那一位的資料卡 → 任意鍵 → 再問「檢視那位」，直到取消。
// 文字版面沒有肖像，卡片走整頁的 `GeneralPage`，不循環。
func (a *app) inspectGeneral(sel int) {
	var ask func()
	ask = func() {
		a.askAnyGeneral(t("ask.inspect"), func(gi int) {
			if a.art == nil {
				a.view.SetPage(ui.GeneralPage(a.s.G, gi))
				return
			}
			a.view.Card, a.view.HasCard = gi, true
			a.afterCard = ask
		})
	}
	if p := a.s.G.Prefecture(sel); p != nil && p.Owned() && p.Owner != a.s.Player {
		a.askYN(t("ask.inspectForeign"), ask)
		return
	}
	ask()
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

// apply 真的送出一個命令。**只給輪到的那個郡**：原版一次只問那一郡的主人
// （`docs/spec/019` §2）；查看類不經過這裡。
func (a *app) apply(o game.Order) {
	if w := a.s.Waiting(); w != 0 && o.Prefecture() != 0 && o.Prefecture() != w {
		a.view.Prompt = tf("msg.notThisPref", prefName(a.s.G, w))
		return
	}
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
	a.cancel = nil
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

// askAnyGeneral 讓玩家從當地的現役將領裡挑一位，不分勢力（查看用）。
func (a *app) askAnyGeneral(title string, then func(int)) {
	var items []pickItem
	for _, x := range a.s.G.Garrison(a.view.Sel) {
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
	// **拉幕要在 Draw 這一層起頭，不能在 Update。** 場景圖那一格排進來
	// 的那一幀，畫布上還是**上一幀畫的舊畫面**；先照沒有它的樣子畫一次
	// 當底，再從那張底把場景圖一步一步拉進來。
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
		case a.opening != nil:
			ui.DrawPages(a.canvas, a.opening.beat.Pages)
		case a.menuScreen != nil:
			a.drawTitle()
		case a.fight != nil:
			if a.artBattle != nil {
				bv := a.fight.view
				sp := a.fight.speech(true)
				// 文字視窗在等鍵：最後一行後面畫游標（對白播著的時候原版在延遲，不讀鍵）。
				bv.Input = ui.InputCursor{On: bv.Window != "" && sp == nil, Frame: ui.CursorFrameAt(a.cursorTick)}
				ui.DrawArtBattle(a.canvas, a.artBattle, a.fight.pending.Battle(), bv, a.battleInfo())
				if sp != nil && sp.LureFlash {
					if l := a.lure; l.of == sp && l.step >= 0 {
						ui.DrawLureFlash(a.canvas, a.artBattle, sp.At, ui.LureFlashSteps()[l.step].Tile)
					}
				} else if sp != nil && (sp.Scene == 0 || a.scenePlayed == sp) {
					// 還沒拉過的場景圖先不畫：Draw 那一層要拿這張當拉幕的底。
					ui.DrawBattleSpeech(a.canvas, a.art, a.s.G, a.fight.pending.Battle(), sp)
				}
			} else {
				ui.DrawBattle(a.canvas, a.fight.pending.Battle(), a.fight.view)
			}
		case a.view.Atlas != 0 && a.artBattle != nil:
			// 郡地理誌：整張換成那個郡的場地圖（原版畫在第二頁再切過去）。
			p := a.s.G.Prefecture(a.view.Atlas)
			if p == nil {
				a.view.Atlas = 0
				break
			}
			ui.DrawArtAtlas(a.canvas, a.artBattle, p.BattleField, a.s.G.Field(a.view.Atlas))
			if b := a.view.AtlasBubble; b != nil {
				ui.DrawBubble(a.canvas, a.art, a.s.G, b)
			}
		default:
			a.view.Over = a.s.Over
			if a.art != nil {
				v := a.view
				// 等玩家輸入時下面板最後一行後面畫游標；對白與示範模式不讀鍵，不畫。
				v.Input = ui.InputCursor{On: a.s.Bubble() == nil && !a.s.Over && len(a.s.G.Players) > 0,
					Frame: ui.CursorFrameAt(a.cursorTick)}
				if b := a.s.Bubble(); b != nil && b.Panel != 0 {
					// 示範模式月底的鏡頭：右側面板畫那一郡的資料（`0x32fb:0x70`）。
					v.Status, v.Sel, v.HasCard, v.Page = true, b.Panel, false, nil
				}
				ui.DrawArtSession(a.canvas, a.art, a.s.G, a.s.Log, v)
				if b := a.s.Bubble(); b != nil && b.Panel != 0 {
					// 面板就是這一格，不畫泡泡。
				} else if b != nil && b.MapBattle != nil {
					if p := a.mapBattle; p != nil && p.b == b {
						ui.DrawMarch(a.canvas, p.m)
					}
				} else if b != nil && (!b.Wiped() || a.scenePlayed == b) {
					// 原版在對白之前把右側面板的內部清成藍色（`0x1058:0x27e8`，
					// 外框留著）；場景圖那一格不清（`0x2c8be` 也清藍，但整張
					// 176×96 蓋滿那一塊）。還沒拉過的場景圖先不畫：Draw 那一層
					// 要拿這張當拉幕的底。
					if b.Scene == 0 {
						ui.ClearPanel(a.canvas, 408, 36, 631, 291, assets.EGAPalette[1])
					}
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
	if f := a.fight; f.view.SkirmishActing != nil {
		f.blinkTick++
		if f.blinkTick%blinkFrames == 0 {
			f.view.Blink = !f.view.Blink
		}
	}
	// 戰場對白（肖像＋泡泡）一次一格，按任意鍵收掉——與主畫面的訊息框
	// 同一個做法（remake 差異：原版走延遲設定）。
	if sp := a.fight.speech(a.artBattle != nil); sp != nil {
		if sp.LureFlash {
			// 誘敵的特效自己播完就收，不等鍵（原版不收鍵）。
			if a.updateLureFlash(sp) {
				a.fight.speeches = a.fight.speeches[1:]
			}
			return nil
		}
		if sp.Scene > 0 && a.scenePlayed != sp {
			return nil // 場景圖先拉進來（Draw 那一層起頭），拉完才收鍵
		}
		if anyKeyPressed() {
			a.fight.speeches = a.fight.speeches[1:]
		}
		return nil
	}
	if a.fight.ending {
		a.endBattle()
		return nil
	}
	if inpututil.IsKeyJustPressed(ebiten.KeyEscape) {
		a.fight.view.ClosePage()
		// 查看收起之後回到命令提示的文字視窗。
		if f := a.fight; f.acting != nil && f.waiting == waitCommand && f.asking == nil {
			f.view.Window = a.commandWindow(f.acting)
		}
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
	// 子畫面的休息確認、叫陣的接受與行軍結束要 Y／N／Enter（`docs/re/05` §10.6）。
	for key, k := range map[ebiten.Key]byte{ebiten.KeyY: 'Y', ebiten.KeyN: 'N', ebiten.KeyEnter: '\r'} {
		if inpututil.IsKeyJustPressed(key) && (a.fight.asking != nil || a.fight.waiting.takesYN()) {
			a.battleKey(k)
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
	var openArt *opening.Art
	var marchArt *[ui.MarchArtCount]*assets.Image
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
			if art != nil {
				if ma, err := ui.MarchArt(c3); err == nil {
					marchArt = &ma
				} else {
					fmt.Fprintln(os.Stderr, "san1：大地圖戰役動畫的素材讀不進來：", err)
				}
			}
			if c1 != nil {
				if i, ok := c1.ByName(speaker.SFXName); ok {
					sfxSamples = speaker.NewClip(c1.Data(i)).Samples()
				}
			}
			if art != nil && c1 != nil {
				if artBattle, err = ui.NewArtBattle(c1, c3); err != nil {
					fmt.Fprintln(os.Stderr, "san1：主戰場的素材讀不進來：", err)
					artBattle = nil
				}
				// 片頭的圖讀不到就不播，直接到主選單。
				if openArt, err = opening.LoadArt(c1); err != nil {
					fmt.Fprintln(os.Stderr, "san1：片頭的素材讀不進來：", err)
					openArt = nil
				} else {
					openArt.PoemInk, openArt.PoemMask = opening.FontPoem(face)
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
	a.marchArt = marchArt
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
	// 片頭 → 主選單 → 遊戲。**指定了劇本以外的東西就直接進遊戲**：
	// `-load`／`-orig-load` 是「我要那一局」，中間再問一次沒有道理，
	// 而 `-title=false` 是給截圖與腳本用的。
	if *showTitle && titleScreen != nil && *load == 0 && *origLoad == 0 {
		a.newMenu = func() *menu.Screen { return menu.New(c, ed, ai.Mode(*aiMode), *saveDir, a.jb.Len()) }
		a.startTitle(titleScreen, a.newMenu())
		if openArt != nil {
			a.opening = newOpeningPlayer(openArt)
		}
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

// startSaving 開存檔那一格（原版「其他 → 儲存進度」，`0x1e440`）：右側面板列六筆
// 名稱，提示「儲存進度\n(1-6):」。沒有存檔目錄照舊說一句。
func (a *app) startSaving() {
	if a.saveDir == "" {
		a.view.Prompt = t("msg.noSaveDir")
		return
	}
	sv := &ui.SaveScreen{}
	for k, info := range session.Saves(a.saveDir) {
		if k < len(sv.Names) {
			sv.Names[k] = menu.LoadLine(info)
		}
	}
	a.saving, a.saveMemo = sv, ""
	a.view.Save, a.view.Page, a.view.HasCard = sv, nil, false
	a.view.Prompt = t("ask.saveOrig")
	a.dirty = true
}

// updateSaving 收存檔那一格的鍵。原版 `0x33d8:0x115e(1, 6)` 收 1–6，其他答案印
// 「取消儲存」退出；選好之後那一筆換成新名稱，`0x33d8:0x20b8` 從第 14 格起收 6 個
// 字元的備註：只收 0x20–0x5A（空白到大寫 Z，小寫字母不收）、Backspace 刪一格、
// Enter 寫檔（`0x35e9e`–`0x35f43`）。Esc 取消是 remake 加的。
func (a *app) updateSaving() {
	sv := a.saving
	enter := inpututil.IsKeyJustPressed(ebiten.KeyEnter) || inpututil.IsKeyJustPressed(ebiten.KeyNumpadEnter)
	at := a.s.Waiting()
	if at == 0 {
		at = a.view.Sel
	}
	if sv.Slot == 0 {
		for k := ebiten.Key1; k <= ebiten.Key6; k++ {
			if inpututil.IsKeyJustPressed(k) {
				sv.Slot = int(k-ebiten.Key1) + 1
				sv.Names[sv.Slot-1] = a.s.SaveName(sv.Slot, at, "")
				a.dirty = true
				return
			}
		}
		if anyKeyPressed() {
			a.stopSaving(t("msg.saveCancel"))
		}
		return
	}
	switch {
	case inpututil.IsKeyJustPressed(ebiten.KeyEscape):
		a.stopSaving(t("msg.saveCancel"))
		return
	case inpututil.IsKeyJustPressed(ebiten.KeyBackspace):
		if r := []rune(a.saveMemo); len(r) > 0 {
			a.saveMemo = string(r[:len(r)-1])
		}
	case enter:
		name := a.s.SaveName(sv.Slot, at, a.saveMemo)
		slot := sv.Slot
		a.stopSaving("")
		_ = a.s.Save(a.saveDir, slot, name)
		return
	}
	for _, r := range ebiten.AppendInputChars(nil) {
		if r >= 0x20 && r <= 0x5a && len(a.saveMemo) < 6 {
			a.saveMemo += string(r)
		}
	}
	sv.Names[sv.Slot-1] = a.s.SaveName(sv.Slot, at, a.saveMemo)
	a.dirty = true
}

// stopSaving 收掉存檔那一格；prompt 不是空字串就留一句。
func (a *app) stopSaving(prompt string) {
	a.saving, a.saveMemo = nil, ""
	a.view.Save = nil
	a.view.Prompt = prompt
	a.dirty = true
}

// demoBubbleFrames 是示範模式裡一格訊息框停幾幀（約一秒）。
const demoBubbleFrames = 60

// endDemo 結束示範模式回主選單；沒有主選單可回就回 false。
func (a *app) endDemo() bool {
	if a.newMenu == nil {
		return false
	}
	a.s = nil
	a.view = ui.View{}
	a.bubbleFrames = 0
	a.startTitle(a.titleArt, a.newMenu())
	return true
}

// lordName 是一個勢力的君主名字（沒有君主用「勢力 N」）。
func lordName(g *game.State, f state.FactionID) string {
	if lord := g.Lord(f); lord != nil {
		return i18n.PersonName(lord.Name)
	}
	return i18n.Sf("fld.factionN", f)
}

// prefName 是郡名。
func prefName(g *game.State, at int) string {
	if p := g.Prefecture(at); p != nil {
		return i18n.PlaceName(p.Name)
	}
	return fmt.Sprintf("%d", at)
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

// currentScene 是現在輪到畫的那一格如果要拉幕：主畫面的訊息框佇列
// 或戰場的對白佇列的頭一格。key 用來認「同一格」，pic 是拉進來的那張。
func (a *app) currentScene() (key any, pic *assets.Image, kind ui.WipeKind, x, y int, ok bool) {
	if a.art == nil || a.opening != nil || a.menuScreen != nil {
		return nil, nil, 0, 0, 0, false
	}
	if a.fight != nil {
		if sp := a.fight.speech(a.artBattle != nil); sp != nil && sp.Scene > 0 {
			return sp, a.art.Scene(sp.Scene), ui.WipeKind(sp.Style), assets.SceneBattleX, assets.SceneBattleY, true
		}
		return nil, nil, 0, 0, 0, false
	}
	if a.s != nil {
		b := a.s.Bubble()
		switch {
		case b == nil:
		case b.Scene > 0:
			return b, a.art.Scene(b.Scene), ui.WipeKind(b.Style), b.X1, b.Y1, true
		case b.WipeIn:
			portrait := -1
			if x := a.s.G.General(b.Speaker); x != nil {
				portrait = int(x.Portrait)
			}
			return b, ui.SearchPanel(a.art, portrait), ui.WipeKind(b.Style),
				assets.SceneMainX, assets.SceneMainY, true
		}
	}
	return nil, nil, 0, 0, 0, false
}

// startWipe 看輪到的那一格是不是還沒拉過的場景圖，是就起一段：先照
// **沒有它**的樣子畫一次當底（paint 看到 scenePlayed 不是它就跳過那一格），
// 再從那張底把場景圖拉進來。
func (a *app) startWipe() {
	if a.wipe != nil {
		return
	}
	key, pic, kind, x, y, ok := a.currentScene()
	if !ok || a.scenePlayed == key {
		return
	}
	a.paint()
	a.wipe = ui.NewSceneWipe(a.canvas, pic, kind, x, y)
	a.scenePlayed = key
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
