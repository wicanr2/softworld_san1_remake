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
	"strconv"
	"strings"

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

	// form 是正在進行的整編（`askAssign`，Issue #98）；沒有就是 nil。
	form *formation
	// afterCard 是人物資料卡按任意鍵收掉之後要接著做的事（查看再問
	// 「檢視那位」、賜物接著列物品表）。
	afterCard func()
	// treasuryWait 為真時查看→物品的物品表在等一鍵（`0x17b97`）。
	treasuryWait bool
	// fortSpot 非 nil 表示建築關寨正在挑位置（`0x1acba`）。
	fortSpot *fortSpotState
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

	// fontDir 是字型檔放哪（`-font` 的目錄）；fontKind 是現在用哪一套
	// （`game.FontKai`／`FontLi`，Issue #71）。fontCache 收已經讀進來的，
	// 換回上一套不必重讀（一份 1.7 MB）。
	fontDir   string
	fontKind  int
	fontCache map[int]*font.Face
	titleArt   *ui.TitleScreen
	// titleAnimTick／Frame 驅動主選單 `CURA0`～`CURA5` 的六格循環。
	titleAnimTick, titleAnimFrame int
	// cursorTick 驅動提示後面輸入游標的六格（`ui.CursorFrameAt`）。
	cursorTick int
	// roster 非 nil 表示正在「挑一位將軍」（`0x18024`）；rosterPage 是原版
	// `DS:0x66b2` 那一格：上一次停在第幾頁，下一次開清單接著用。
	roster          *rosterEntry
	rosterPage      int
	rosterPageMulti int

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
			no := a.confirm.no
			a.confirm, a.view.Prompt = nil, ""
			if no != nil {
				no()
			}
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
	// 電腦打過來、玩家要親自守的那一場（Issue #64）：接過指揮權。
	if a.s != nil && a.fight == nil && a.s.G.PendingDefence() != nil {
		a.startDefence()
		a.dirty = true
		return nil
	}
	// 輪流下令：沒停在玩家的郡就照這個月的順序往下跑，停在下一個玩家的郡。
	if a.s != nil && a.menu == 0 && a.pick == nil && a.num == nil && a.s.Waiting() == 0 && !a.s.Over {
		at := a.s.AdvanceToHuman(0)
		// 停下來的理由可能是「電腦打過來要玩家守」而不是「輪到玩家下令」
		// （Issue #64）：那一條不印主提示，直接接指揮權。
		if a.s.G.PendingDefence() != nil {
			a.view.Sel, a.view.Status = at, false
			a.startDefence()
			a.dirty = true
			return nil
		}
		if at != 0 {
			a.view.Sel, a.view.Status = at, true
			a.mainAsk(at)
		} else if at := a.s.Waiting(); at != 0 && a.view.Prompt == "" &&
			a.roster == nil && a.fortSpot == nil && !a.view.HasCard && a.view.Atlas == 0 {
			// **不耗回合的命令做完就要再問一次**：原版的主迴圈每一輪都清訊息、
			// 重印提示（`0x1766d`）。留著上一句訊息時先不問，等玩家按鍵——
			// 原版那一句後面跟著 `0x1058:0xe80`。
			a.mainAsk(at)
		}
		a.dirty = true
		return nil
	}
	// 人物資料卡（原版素材畫面的「查看→武將」）：按任意鍵收掉，
	// 右側面板回到原樣。郡地理誌同一個做法（原版畫在第二頁，等鍵切回）。
	if a.roster != nil {
		a.updateRoster()
		return nil
	}
	if a.fortSpot != nil {
		a.updateFortSpot()
		return nil
	}
	// 君主死掉之後由玩家挑繼承人（`0x14f7c`，Issue #65），以及主事者離開之後
	// 挑新任太守（`0x1d6ed`，Issue #84）。兩者都等對白播完才問——原版是
	// 死亡對白 → 問 → 繼承對白，這個順序靠「佇列空了才問」達成。
	if a.s != nil && a.roster == nil && a.num == nil && len(a.pick) == 0 &&
		!a.view.HasCard && a.s.Bubble() == nil {
		if _, list := a.s.G.NeedsHeir(); len(list) > 0 {
			a.askHeir(list)
			a.dirty = true
			return nil
		}
		if at := a.s.G.NeedsGovernor(); at != 0 {
			a.askNewGovernor(at)
			a.dirty = true
			return nil
		}
	}
	if a.treasuryWait {
		if anyKeyPressed() {
			a.treasuryWait, a.view.Treasury, a.view.Prompt = false, nil, ""
			a.dirty = true
		}
		return nil
	}
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
		a.view.Page, a.view.Treasury = nil, nil
		next()
		a.dirty = true
		return nil
	}
	if inpututil.IsKeyJustPressed(ebiten.KeyEscape) {
		a.menu, a.view.Menu, a.view.Items, a.pick, a.num = 0, "", nil, nil, nil
		a.view.Prompt, a.view.Page, a.view.Treasury = "", nil, nil
		a.dirty = true
		return nil
	}
	if a.num != nil {
		if inpututil.IsKeyJustPressed(ebiten.KeyBackspace) {
			if d := a.num.digits; d != "" {
				a.num.digits = d[:len(d)-1]
			}
			a.num.value, _ = strconv.Atoi(a.num.digits)
			a.num.typed = a.num.digits != ""
			a.showNumber()
			a.dirty = true
			return nil
		}
		if inpututil.IsKeyJustPressed(ebiten.KeyEnter) ||
			inpututil.IsKeyJustPressed(ebiten.KeyNumpadEnter) {
			n := a.num
			cancel := a.cancel
			a.num, a.cancel = nil, nil
			a.menu, a.view.Menu, a.view.Items = 0, "", nil
			switch {
			case !n.typed && cancel != nil:
				cancel()
			case !n.typed:
				// 空欄位 Enter 是取消（原版回 0xFFFF），不是 0。
				a.view.Prompt = ""
			case n.value < n.lo || n.value > n.max:
				// 超出範圍原版重問（`0x35046`／`0x35051`）。
				n.value, n.typed, n.digits = 0, false, ""
				a.num, a.cancel = n, cancel
				a.showNumber()
			default:
				n.then(n.value)
			}
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
	// 子選單開著時字母也算一個項目的鍵（`ui.MenuKey` 第十一項起用 `A`）。
	// 只在子選單裡收，主選單按字母仍然什麼都不做。
	if a.menu != 0 {
		for k := ebiten.KeyA; k <= ebiten.KeyZ; k++ {
			if inpututil.IsKeyJustPressed(k) {
				a.press(byte('A' + (k - ebiten.KeyA)))
				return nil
			}
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

// mainAsk 是輪到玩家那一郡時的主提示（`DS:0x685e`，`L0`）：下面板
// 「%s主公,請對(%d)\n%s下您的命令:」——君主姓名欄、郡編號、郡名——接著數字輸入 0–9
// （`0x115e`，打的數字回顯在提示後面，Enter 確定）。範圍字樣不另外接（`askBare`）。
func (a *app) mainAsk(at int) {
	g := a.s.G
	lord := ""
	if l := g.Lord(a.s.Player); l != nil {
		lord = ui.NameField(i18n.PersonName(l.Name))
	}
	a.askBare(tf("ask.main", lord, at, prefName(g, at)), 0, 9, func(n int) {
		a.press(byte('0' + n))
	})
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
				// 原版的「0.狀態」：上面板換成郡的資料，**什麼字都不印**
				// （`0x17706` → `0x32fb:0x70(郡)`，`docs/spec/014` §2.1）。
				a.view.Status = true
				a.view.Prompt = ""
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
	lo    int
	max   int
	value int
	// typed 為假表示還沒打任何數字：原版空欄位按 Enter 是取消（`0x35026` 回 0xFFFF）。
	typed  bool
	digits string
	// bare 為真時範圍字樣已經寫在提示裡（「那一種:」「那一樣(2-5):」），不再接「(下限-上限):」。
	bare bool
	then func(int)
}

// askNumber 開一個數字輸入。0 也是合法的答案。
func (a *app) askNumber(title, hint string, max int, then func(int)) {
	if max < 0 {
		max = 0
	}
	a.num, a.cancel = &numEntry{title: title, hint: hint, max: max, then: then}, nil
	a.showNumber()
}

// askRange 是下限不是 0 的數字輸入（挑郡 1–42）。
func (a *app) askRange(title string, lo, hi int, then func(int)) {
	a.askNumber(title, "", hi, then)
	a.num.lo = lo
	a.showNumber()
}

// askBare 是提示自己帶範圍字樣的數字輸入。
func (a *app) askBare(title string, lo, hi int, then func(int)) {
	a.askRange(title, lo, hi, then)
	a.num.bare = true
	a.showNumber()
}

// showNumber 把目前輸入的數字畫到選單欄上。
func (a *app) showNumber() {
	n := a.num
	if n == nil {
		return
	}
	if a.art != nil {
		// 原版：下面板寫提示，數字接在「(下限-上限):」後面回顯（`0x34ede`，
		// `docs/spec/014` §4.3）。remake 的說明字（hint）在這個版面不畫。
		a.view.Menu, a.view.Items = "", nil
		a.view.Prompt = n.title
		if !n.bare {
			a.view.Prompt += tf("pick.range", n.lo, n.max)
		}
		a.view.Prompt += n.digits
		return
	}
	a.view.Menu = strings.ReplaceAll(n.title, "\n", " ")
	a.view.Items = []ui.Command{
		{Key: '=', Name: fmt.Sprintf("%d", n.value)},
		{Key: ' ', Name: tf("fld.max", n.max)},
	}
	a.view.Prompt = n.hint + t("hint.number")
}

// numberKey 收數字輸入的一個按鍵。
func (a *app) numberKey(k byte) {
	n := a.num
	// 欄寬是上限的位數，滿了再打的數字丟掉（`0x34ff5`）；超過上限要等 Enter 才重問。
	if len(n.digits) >= len(strconv.Itoa(n.max)) {
		return
	}
	n.digits += string(rune(k))
	n.value, _ = strconv.Atoi(n.digits)
	n.typed = true
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
	case cat == '9' && item == 'A':
		// **remake 加的第十一項**：電腦來攻時誰指揮守方（Issue #64）。
		// 原版一律由玩家自己守；這裡的預設是自動打完，理由見
		// `game.Options.PlayerDefends`。
		a.view.Prompt = g.Options.TogglePlayerDefend()
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
		// 查看那一郡（`0x17c49`）：1–42 都收，選了右側面板換成那一郡的資料。
		a.askPref(t("ask.pref"), func(int) bool { return true }, func(id int) {
			a.view.Sel, a.view.Status = id, true
		}, nil)
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
		// 查看→物品（`0x17b64`）：右側面板畫君主物品表、下面板「請按任一鍵」、等一鍵回選單。
		closeMenu()
		if a.art == nil {
			a.view.SetPage(ui.TreasuryList(g, s.Player))
			break
		}
		a.view.Treasury, a.treasuryWait = &ui.TreasuryPanel{Faction: s.Player}, true
		a.view.Prompt = t("msg.anyKey")

	// ---- 2. 軍事 ----
	case cat == '2' && item == '1':
		a.moveTroops(sel)
	case cat == '2' && item == '2':
		// 從那一郡攻打（`0x18998`，主事者是君主才問）→ 攻打那一郡（`0x18a42`）：
		// 相鄰、有主、主人不同；取消印「取消」。
		closeMenu()
		a.askSource(sel, t("ask.attackFrom"), func(src int) {
			a.askPref(t("ask.attack"), func(to int) bool {
				q, p := g.Prefecture(to), g.Prefecture(src)
				return q != nil && p != nil && g.Adjacent(src, to) && q.Owned() && q.Owner != p.Owner
			}, func(to int) {
				a.attackFrom(sel, src, to)
			}, func() { a.view.Sel = sel })
		})
	case cat == '2' && item == '3':
		a.transport()

	// ---- 3. 兵士 ----
	case cat == '3' && item == '1':
		// 原版是對整個守軍訓練，不挑人（`game.State.Train`，`L0`），
		// 所以這裡不再問「訓練誰」。
		a.run(game.TrainOrder{At: sel})
	case cat == '3' && item == '2':
		a.askRoster(t("ask.conscript"), sel, game.PickServing, game.PickBySoldiers, func(gi int) {
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
			soldiers, name := 0, ""
			if x != nil {
				soldiers, name = int(x.Soldiers), ui.NameField(i18n.PersonName(x.Name))
			}
			a.askNumber(tf("ask.conscriptN", name, soldiers),
				tf("hint.conscript", g.Prefecture(sel).Population),
				cap, func(n int) {
					a.run(game.ConscriptOrder{At: sel, General: gi, Count: n})
				})
		}, nil)
	case cat == '3' && item == '3':
		a.askRoster(t("ask.arms"), sel, game.PickServing, game.PickByArms, func(gi int) {
			a.run(game.ArmsOrder{At: sel, General: gi, Units: 500})
		}, nil)
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
		a.askRoster(t("ask.reclaim"), sel, game.PickServing, game.PickByIntel, func(gi int) { a.run(game.ReclaimOrder{At: sel, General: gi}) }, nil)
	case cat == '4' && item == '2':
		a.askRoster(t("ask.flood"), sel, game.PickServing, game.PickByIntel, func(gi int) { a.run(game.FloodControlOrder{At: sel, General: gi}) }, nil)
	case cat == '4' && item == '3':
		closeMenu()
		a.buildFort(sel)
	case cat == '4' && item == '4':
		a.run(game.RestOrder{At: sel})

	// ---- 5. 商業 ----
	case cat == '5' && item == '1':
		closeMenu()
		a.buyRice(sel)
	case cat == '5' && item == '2':
		closeMenu()
		a.sellRice(sel)
	case cat == '5' && item == '3':
		closeMenu()
		a.relief(sel)

	// ---- 6. 人事 ----
	case cat == '6' && item == '1':
		a.askRoster(t("ask.search"), sel, game.PickServing, game.PickByIntel, func(gi int) { a.run(game.SearchOrder{At: sel, General: gi}) }, nil)
	case cat == '6' && item == '2':
		a.askRoster(t("ask.recruit"), sel, game.PickFree, game.PickByIntel, func(gi int) { a.run(game.RecruitOrder{At: sel, Target: gi}) }, nil)
	case cat == '6' && item == '3':
		closeMenu()
		a.withAdvice(game.AdviceReward, sel, game.AdviceTarget{}, func() { a.reward(sel) })
	case cat == '6' && item == '4':
		a.askRoster(t("ask.dismiss"), sel, game.PickOfficerOnly, game.PickByLoyalty, func(gi int) { a.run(game.DismissOrder{At: sel, Target: gi}) }, nil)

	// ---- 7. 君主 ----
	case cat == '7' && item == '1':
		// 指定軍師（`0x1c87e`）：清單是下令那一郡的人（模式 6、鍵 1）；指定成功之後下面板
		// 「%s將任／%s的軍師」（`DS:0x74f8`：新人、君主）。
		a.askRoster(t("ask.chief"), sel, game.PickWiseSub, game.PickByIntel, func(gi int) {
			a.run(game.AppointChiefOrder{At: sel, Target: gi})
			f, x, lord := g.Faction(s.Player), g.General(gi), g.Lord(s.Player)
			if f != nil && x != nil && lord != nil && f.Chief == gi {
				msg := tf("msg.chiefSet", ui.NameField(i18n.PersonName(x.Name)), ui.NameField(i18n.PersonName(lord.Name)))
				a.afterBubbles = func() { a.view.Prompt = msg }
			}
		}, nil)
	case cat == '7' && item == '2':
		closeMenu()
		a.appointGovernor(sel)
	case cat == '7' && item == '3':
		closeMenu()
		a.autonomy(sel)
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
		a.headhunt(sel)

	// ---- 8. 謀略 ----
	case cat == '8':
		a.plotFlow(sel, game.Plot(item-'0'))

	default:
		a.view.Prompt = t("msg.noSuchItem")
	}
	if len(a.pick) == 0 && a.num == nil {
		closeMenu()
	}
}

// reward 是人事→3.賞賜金帛（`0x1c1d2`）：金是 0 印「抱歉, 您沒有金」退出；「<賞賜金帛>／賞賜那一位
// 將軍」（模式 5、鍵 4）→ 這一道命令裡賞過的印「%s已賞賜過了」回到名單 → 「賞賜%s多少金／(0-%d):」
// （上限 min(金, 100)）→ 賞完回到名單。名單取消、金額打 0 或取消（印「取消」）都收掉命令；賞出過一位
// 就結束這個郡的回合（`GiftRound`，與賞賜物品共用）。
func (a *app) reward(sel int) {
	g := a.s.G
	p := g.Prefecture(sel)
	if p == nil {
		return
	}
	if p.Gold <= 0 {
		a.view.Prompt = t("msg.noGold")
		return
	}
	r, err := g.OpenReward(sel, a.s.Player)
	if err != nil {
		a.view.Prompt = game.ErrorText(err)
		return
	}
	done := func() {
		if a.s.CloseGift(r) {
			a.view.Prompt = ""
		}
	}
	var who func()
	who = func() {
		a.askRoster(t("ask.reward"), sel, game.PickSubject, game.PickByLoyalty, func(gi int) {
			x := g.General(gi)
			if x == nil {
				return
			}
			name := ui.NameField(i18n.PersonName(x.Name))
			if r.Gifted(gi) {
				a.view.Prompt = tf("msg.gifted", i18n.PersonName(x.Name))
				who()
				return
			}
			most := min(p.Gold, game.MaxReward)
			a.askBare(tf("ask.rewardGold", name, most), 0, most, func(n int) {
				if n <= 0 {
					a.view.Prompt = t("msg.cancel")
					done()
					return
				}
				a.apply(game.RewardOrder{At: sel, Target: gi, Gold: n, Round: r})
				a.afterBubbles = who
			})
			a.cancel = func() {
				a.view.Prompt = t("msg.cancel")
				done()
			}
		}, done)
	}
	who()
}

// buyRice 是商業→1.買入米糧（`0x1b1b2`）：沒有金印「抱歉, 您沒有金」；一金換 (100 − 物價) ÷ 10 米，
// 上限 min(金, (30000 − 米) ÷ 換率)，是 0 印「抱歉, 您的糧倉已滿／裝不下了」；問「1 金 = %d 米／您想買
// 多少金／的米(0-%d):」，0 或取消回選單；買完「%s現有／金:%d 米:%d 」（`DS:0x71ef`）。
func (a *app) buyRice(sel int) {
	p := a.s.G.Prefecture(sel)
	if p == nil {
		return
	}
	if p.Gold <= 0 {
		a.view.Prompt = t("msg.noGold")
		return
	}
	rate := game.RicePerGold(p.PriceLevel)
	most := min(p.Gold, (game.MaxRice-p.Rice)/max(rate, 1))
	if most <= 0 {
		a.view.Prompt = t("msg.granaryFull")
		return
	}
	a.askBare(tf("ask.buyRice", rate, most), 0, most, func(n int) {
		if n <= 0 {
			a.view.Prompt = ""
			return
		}
		a.commerceDone(sel, game.BuyRiceOrder{At: sel, Units: n * rate}, "msg.riceNow")
	})
}

// sellRice 是商業→2.賣出米糧（`0x1b442`）：沒有米印「抱歉, 您沒有米」；換率同買入，上限是米與金庫
// 裝得下的量取小，是 0 印「抱歉, 您的金庫已滿／不能再賣了」；問「%d 米 = 1 金／您想賣多少米／(0-%d):」。
func (a *app) sellRice(sel int) {
	p := a.s.G.Prefecture(sel)
	if p == nil {
		return
	}
	if p.Rice <= 0 {
		a.view.Prompt = t("msg.noRice")
		return
	}
	rate := game.RicePerGold(p.PriceLevel)
	most := min(p.Rice, (game.MaxGold-p.Gold)*rate)
	if most <= 0 {
		a.view.Prompt = t("msg.treasuryFull")
		return
	}
	a.askBare(tf("ask.sellRice", rate, most), 0, most, func(n int) {
		if n <= 0 {
			a.view.Prompt = ""
			return
		}
		a.commerceDone(sel, game.SellRiceOrder{At: sel, Units: n}, "msg.riceNow")
	})
}

// relief 是商業→3.開倉賑民（`0x1b6f6`）：沒有米印「抱歉,您沒有米」；上限 min(米, 5000)，問「您給多少米
// ／(0-%d):」；發完「%s現有%d米／人民忠心:%d 」（`DS:0x72bb`）。
func (a *app) relief(sel int) {
	p := a.s.G.Prefecture(sel)
	if p == nil {
		return
	}
	if p.Rice <= 0 {
		a.view.Prompt = t("msg.noRiceRelief")
		return
	}
	most := min(p.Rice, game.MaxReliefRice)
	a.askBare(tf("ask.relief", most), 0, most, func(n int) {
		if n <= 0 {
			a.view.Prompt = ""
			return
		}
		a.commerceDone(sel, game.ReliefOrder{At: sel, Gold: n}, "msg.reliefNow")
	})
}

// commerceDone 下令，成功之後在對白講完時印「現有」那一句。
func (a *app) commerceDone(sel int, o game.Order, key string) {
	g := a.s.G
	p := g.Prefecture(sel)
	gold, rice := p.Gold, p.Rice
	a.run(o)
	if p.Gold == gold && p.Rice == rice {
		return // 沒有成交（勸諫後取消或規則層擋下）
	}
	var msg string
	if key == "msg.reliefNow" {
		msg = tf(key, p.Name, p.Rice, p.PublicLoyalty)
	} else {
		msg = tf(key, p.Name, p.Gold, p.Rice)
	}
	a.afterBubbles = func() { a.view.Prompt = msg }
}

// buildFort 是內政→3.建築關寨（`0x1aa7e`）：關寨已有 5 個印「本郡已有%d個關寨／不能再建了」、
// 金不到 100 × 物價印「建關寨須%d金／您的金不夠」，都退出；提示「<建築關寨>須用%d金／且須一位謀略
// 大於79／的將軍,那一位去」後面直接接範圍，清單模式 3、鍵 1，取消收掉命令。建完下面板
// 「%s現有%d個關寨／剩餘金:%d」（`DS:0x70b3`）。挑位置的游標畫面（`0x1acba`）remake 還是自己挑。
func (a *app) buildFort(sel int) {
	g := a.s.G
	p := g.Prefecture(sel)
	if p == nil {
		return
	}
	cost := game.FortCost(p.PriceLevel)
	switch {
	case p.Forts >= game.MaxForts:
		a.view.Prompt = tf("msg.fortFull", p.Forts)
		return
	case cost > p.Gold:
		a.view.Prompt = tf("msg.fortGold", cost)
		return
	}
	a.askRoster(tf("ask.fort", cost), sel, game.PickWise, game.PickByIntel, func(gi int) {
		if a.artBattle == nil {
			a.fortBuild(sel, gi, 0)
			return
		}
		a.view.Prompt = ""
		a.fortSpot = &fortSpotState{at: sel, general: gi}
	}, nil)
}

// fortSpotState 是挑位置那個畫面的狀態：游標在第幾欄第幾列、是不是正在問確認。
type fortSpotState struct {
	at, general int
	col, row    int
	confirm     bool
}

// fortBuild 下建築關寨，cell 是戰場索引加一（0 表示 remake 自己挑）；蓋成之後在對白講完時印
// 「%s現有%d個關寨／剩餘金:%d」。
func (a *app) fortBuild(sel, gi, cell int) {
	p := a.s.G.Prefecture(sel)
	before := p.Forts
	a.run(game.BuildFortOrder{At: sel, General: gi, Cell: cell})
	if p.Forts > before {
		msg := tf("msg.fortBuilt", p.Name, p.Forts, p.Gold)
		a.afterBubbles = func() { a.view.Prompt = msg }
	}
}

// updateFortSpot 收挑位置畫面的一鍵（`0x1ae29`）：數字鍵走游標（`game.FortSpotStep`）；「0」在
// 能蓋的格子上改問「確認(Y/N):」，Y 才蓋、其他鍵回到走游標；ESC 印「取消」回選單（`0x1ab48`）。
func (a *app) updateFortSpot() {
	s := a.fortSpot
	p := a.s.G.Prefecture(s.at)
	if p == nil {
		a.fortSpot = nil
		return
	}
	if s.confirm {
		if !anyKeyPressed() {
			return
		}
		s.confirm = false
		if inpututil.IsKeyJustPressed(ebiten.KeyY) {
			a.fortSpot = nil
			a.fortBuild(s.at, s.general, s.row*12+s.col+1)
		}
		a.dirty = true
		return
	}
	if inpututil.IsKeyJustPressed(ebiten.KeyEscape) {
		a.fortSpot = nil
		a.view.Prompt = t("msg.cancel")
		a.dirty = true
		return
	}
	for k := byte('0'); k <= '6'; k++ {
		if !inpututil.IsKeyJustPressed(ebiten.Key0+ebiten.Key(k-'0')) &&
			!inpututil.IsKeyJustPressed(ebiten.KeyNumpad0+ebiten.Key(k-'0')) {
			continue
		}
		if k == '0' {
			if i := s.row*12 + s.col; i < len(p.BattleField) && game.CanBuildFortOn(p.BattleField[i]) {
				s.confirm = true
			}
		} else {
			s.col, s.row = game.FortSpotStep(s.col, s.row, k, p.BattleField)
		}
		a.dirty = true
		return
	}
}

// appointGovernor 是君主→2.指定太守（`0x1caea`）：「指定那一郡的太守」收自己的其他郡
// （`game.GovernorTarget`），挑到之後地圖選到那一郡、列那一郡的人（模式 2、鍵 3）；兩處取消
// 都收掉命令。指定完場景圖、兩格對白，下面板「%s將任／%s的太守」（`DS:0x753a`），回選單、不耗回合。
func (a *app) appointGovernor(sel int) {
	g := a.s.G
	done := func() { a.view.Sel = sel }
	a.askPref(t("ask.governorPref"), func(id int) bool { return g.GovernorTarget(sel, id) }, func(pref int) {
		a.view.Sel = pref
		a.askRoster(t("ask.governor"), pref, game.PickServing, game.PickByCharm, func(gi int) {
			done()
			a.apply(game.AppointGovernorOrder{At: sel, Pref: pref, Target: gi})
			// 訊息只在指定成功時才有：看新主事者是不是他。
			x, p, gov := g.General(gi), g.Prefecture(pref), g.Governor(pref)
			if x != nil && p != nil && gov != nil && gov.Index == gi {
				msg := tf("msg.governorSet", ui.NameField(i18n.PersonName(x.Name)), p.Name)
				a.afterBubbles = func() { a.view.Prompt = msg }
			}
		}, done)
	}, done)
}

// autonomy 是君主→3.郡縣自冶（`0x1cd68`）：「授權自冶那一郡」收同一主人、主事者不是君主的郡
// （`game.AutonomyTarget`），取消收掉命令；挑到之後地圖與右側面板換成那一郡（`0x1ce2d`），
// 下面板「授權某郡／1.正常 2.內政／3.軍事 4.自冶／那一種:」——範圍字樣在提示裡、不另接
// 「(1-4):」——`0x115e(1, 4)` 讀一位數，取消印「取消」收掉命令（`0x1ceb2`）；設定完、兩格
// 對白講完回到挑郡再問（`0x1cfd3`）。整道命令不耗回合（`game.AutonomyOrder`）。
func (a *app) autonomy(sel int) {
	g := a.s.G
	done := func() { a.view.Sel = sel }
	var ask func()
	ask = func() {
		done()
		a.askPref(t("ask.autonomyPref"), func(id int) bool { return g.AutonomyTarget(sel, id) }, func(pref int) {
			p := g.Prefecture(pref)
			a.view.Sel = pref
			a.askBare(tf("ask.autonomy", p.Name, t("autoMode.normal"), t("autoMode.civil"),
				t("autoMode.military"), t("autoMode.self")), 1, 4, func(n int) {
				a.apply(game.AutonomyOrder{At: sel, Pref: pref, Mode: game.Autonomy(n - 1)})
				a.afterBubbles = ask
			})
			a.cancel = func() {
				done()
				a.view.Prompt = t("msg.cancel")
			}
		}, done)
	}
	ask()
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
	a.askPref(t("ask.giftPref"), func(pref int) bool { return a.s.G.GiftPrefectureOK(r, pref) },
		func(pref int) {
			a.view.Sel = pref
			a.giftWho(r, pref)
		}, done)
}

// giftWho 是「賞賜那一位」（`0x1d0b0`）：挑到人畫他的卡、等鍵看物品表；
// 這道命令裡賞過的印「%s已賞賜過了」再問；空 Enter 印「取消」回那一郡。
func (a *app) giftWho(r *game.GiftRound, pref int) {
	g := a.s.G
	back := func() {
		a.view.Prompt = t("msg.cancel")
		a.giftPref(r)
	}
	a.askRoster(t("ask.giftTo"), pref, game.PickServing, game.PickByLoyalty, func(gi int) {
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
	}, back)
}

// giftPick 是賞賜物品的「那一樣」（`0x1d1ad`）：右側面板是君主物品表（`0x14de6`），下面板
// 「賞賜某人／那一樣(2-5):」——範圍字樣在提示裡——`0x115e(2, 5)` 減一就是寶物（1 兵書 … 4 駿馬）；
// 那一件是 0 件重問（`0x1d21a`），取消回到「賞賜那一位」。送完排道謝與卡片，收掉之後回到「賞賜那一位」。
func (a *app) giftPick(r *game.GiftRound, pref, gi int) {
	if a.art != nil {
		a.view.Treasury = &ui.TreasuryPanel{Faction: a.s.Player}
	} else {
		a.view.SetPage(ui.TreasuryList(a.s.G, a.s.Player))
	}
	again := func() { a.giftWho(r, pref) }
	name := ""
	if x := a.s.G.General(gi); x != nil {
		name = ui.NameField(i18n.PersonName(x.Name))
	}
	a.askBare(tf("ask.gift", name), 2, 5, func(n int) {
		what := game.Treasure(n - 1)
		f := a.s.G.Faction(a.s.Player)
		if f == nil || f.Treasury[what] <= 0 {
			a.giftPick(r, pref, gi)
			return
		}
		a.view.Page, a.view.Treasury = nil, nil
		a.apply(game.GiftOrder{At: r.At, Target: gi, What: what, Round: r})
		a.afterBubbles = again
	})
	a.cancel = func() {
		a.view.Treasury = nil
		a.view.Prompt = t("msg.cancel")
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
		a.askRoster(t("ask.inspect"), sel, game.PickAny, game.PickByStatus, func(gi int) {
			if a.art == nil {
				a.view.SetPage(ui.GeneralPage(a.s.G, gi))
				return
			}
			// 卡片之後下面板清掉、寫「請按任一鍵」（`0x17cd8`，`DS:0x69cf`）再讀鍵。
			a.view.Card, a.view.HasCard = gi, true
			a.view.Prompt = t("msg.anyKey")
			a.afterCard = ask
		}, nil)
	}
	if p := a.s.G.Prefecture(sel); p != nil && p.Owned() && p.Owner != a.s.Player {
		a.askYN(t("ask.inspectForeign"), ask)
		return
	}
	ask()
}

// confirmEntry 是等著的 Y/N；then 是按 Y 要做的事，
// no 是按 N（或 Esc）要做的事——nil 就只是收掉這一問。
//
// **「分配完畢(Y/N)」的 N 不是取消**（`0x21475`，Issue #98）：它是
// 「再分一位」，要回到整編的迴圈。沒有 no 這一格的話，N 會讓整編
// 靜靜地結束，而畫面上看起來像命令被取消了。
type confirmEntry struct{ then, no func() }

// askYN 問一句 Y/N。
func (a *app) askYN(prompt string, then func()) {
	a.confirm = &confirmEntry{then: then}
	a.view.Prompt = prompt
}

// askYNElse 是 N 也有事要做的那一種（「分配完畢(Y/N)」的 N ＝ 再分一位）。
func (a *app) askYNElse(prompt string, then, no func()) {
	a.confirm = &confirmEntry{then: then, no: no}
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
		if p := v.Plan; p != nil {
			to := p.At
			if v.What == game.PlotJointAttack {
				to = p.Strike
			}
			return game.AdviceTarget{Target: p.Envoy, To: to, What: v.What}
		}
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
		case a.fortSpot != nil && a.artBattle != nil:
			s := a.fortSpot
			p := a.s.G.Prefecture(s.at)
			ui.DrawArtFortSpot(a.canvas, a.artBattle, p.BattleField, a.s.G.Field(s.at), ui.FortSpot{
				Col: s.col, Row: s.row, Confirm: s.confirm,
				Marked: a.cursorTick/ui.CursorTicksPerFrame%2 == 0,
				Input:  ui.InputCursor{On: true, Frame: ui.CursorFrameAt(a.cursorTick)},
			})
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
	// 主選單那兩項字型（Issue #71）與存檔裡記的那一套。
	a.fontDir = filepath.Dir(*fontPath)
	if s != nil {
		a.setFont(s.G.Options.Font)
	}
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
		a.newMenu = func() *menu.Screen {
			m := menu.New(c, ed, ai.Mode(*aiMode), *saveDir, a.jb.Len())
			m.OnFont = a.setFont
			return m
		}
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
// setFont 換主選單那兩項指定的字模（Issue #71）：原版換完**直接重畫、
// 不印訊息**（`0x11bb4`／`0x11bc2`）。讀不到就留著現在這一套並說一句
// ——悄悄不換的話，玩家看到的是「這個選項沒作用」。
func (a *app) setFont(kind int) {
	if kind == a.fontKind {
		a.dirty = true
		return
	}
	name := game.FontFile(kind)
	if name == "" {
		return
	}
	f := a.fontCache[kind]
	if f == nil {
		fh, err := os.Open(filepath.Join(a.fontDir, name))
		if err == nil {
			f, err = font.ParseHexGz(fh, 16)
			fh.Close()
		}
		if err != nil {
			fmt.Fprintf(os.Stderr, "san1: %s 讀不進來，字型不換：%v\n", name, err)
			return
		}
		if a.fontCache == nil {
			a.fontCache = map[int]*font.Face{}
		}
		a.fontCache[kind] = f
	}
	a.fontKind = kind
	a.canvas.SetFace(f)
	if a.s != nil {
		a.s.G.Options.Font = kind
	}
	a.dirty = true
}

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

// transport 是運送錢糧（`0x18fee`）：主事者是君主本人才問「從那一郡送出」（`0x19019`）→
// 「送到那一郡」自己的其他郡 →「\n金(0-%d):」→「\n米(0-%d):」（兩問不清訊息，接在前面幾行後面）；
// 任何一格取消都收掉這道命令（`0x1909e` 回 −1）。
//
// ⚠ 原版「從那一郡送出」收的是任何自己的郡，remake 的規則層（`game.Transport`）還是
// 「下令的郡送出」，所以這裡只收下令的郡（Issue #82）。
func (a *app) transport() {
	g, me, at := a.s.G, a.s.Player, a.view.Sel
	own := func(pref int) bool {
		p := g.Prefecture(pref)
		return p != nil && p.Owned() && p.Owner == me
	}
	to := func(src int) {
		a.view.Sel = src
		a.askPref(t("ask.sendTo"), func(to int) bool { return to != src && own(to) }, func(to int) {
			// 金、米兩問不清訊息（`0x191a3`），接在「送到那一郡(1-42):N」後面。
			_, goldMax, riceMax := g.MoveLimits(src, to)
			head := t("ask.sendTo") + tf("pick.range", 1, 42) + strconv.Itoa(to) + t("ask.sendGold")
			a.askNumber(head, tf("hint.gold", goldMax), goldMax, func(gold int) {
				riceHead := head + tf("pick.range", 0, goldMax) + strconv.Itoa(gold) + t("ask.sendRice")
				a.askNumber(riceHead, tf("hint.rice", riceMax), riceMax, func(rice int) {
					a.run(game.TransportOrder{At: at, From: src, To: to, Gold: gold, Rice: rice})
				})
			})
		}, nil)
	}
	a.askSource(at, t("ask.sendFrom"), to)
}

// moveTroops 是調動軍隊（`0x18bc8`）：主事者是君主時先問「從那一郡移出」（remake 只收
// 下令的郡，Issue #82）→「調到那一郡」（相鄰、無主或同一個主人）→ 多選「調動那一位」
// （模式 2、第三欄兵士，最多 50 − 目標郡的現役將數）→「\n共調%d位將軍」接「\n金(0-%d):」
// →「\n米(0-%d):」；任何一格取消都收掉這道命令。
// attackFrom 是發動戰役選完來源與目標之後那一段（`0x18a9c` 起）：問攜帶的金米、
// 軍師勸諫、宣戰對白，然後進主戰場。**回合記在下令的郡 at，出兵的是 src**（Issue #82）。
func (a *app) attackFrom(at, src, to int) {
	g := a.s.G
	// **整編決定誰出征**（原版 `0x20a30`，Issue #98）：先前 remake 是
	// 「留一位在家、其餘全部出征」，那既不是原版的規則也不是玩家的選擇。
	var pool []int
	for _, x := range g.ActorRoster(src) {
		pool = append(pool, x.Index)
	}
	if len(pool) == 0 {
		a.view.Prompt = t("msg.noTargets")
		return
	}
	a.askAssign(&formation{pool: pool, groups: make([]int, len(pool)),
		then: func(force []int, groups []int) { a.attackWith(at, src, to, force, groups) }})
}

// attackWith 是整編之後那一段：攜帶錢糧 → 勸諫 → 宣戰 → 主戰場。
func (a *app) attackWith(at, src, to int, force []int, groups []int) {
	g, s := a.s.G, a.s
	// **玩家親自指揮**：先問攜帶的錢糧（原版 `攜帶多少金`／`攜帶多少米`，而且把
	// 三十天要多少米算給你看），再進主戰場。電腦諸侯的戰役還是走 AttackOrder → Auto。
	p := g.Prefecture(src)
	men := g.CampaignForce(force)
	need := game.RiceForCampaign(men)
	a.askNumber(t("ask.gold"), tf("hint.gold", p.Gold), p.Gold, func(gold int) {
		a.askNumber(t("ask.rice"), tf("hint.campaign", p.Rice, men, need), p.Rice, func(rice int) {
			// 軍師勸諫（`0x18b08`）之後先播 `SCG06`（`0x18b82`），再是兩句
			// 宣戰（`0x202e1`／`0x20322`），對白收完才進主戰場。
			a.withAdvice(game.AdviceAttack, src, game.AdviceTarget{To: to}, func() {
				s.Queue(g.WarScene(src, s.Player))
				s.Queue(g.WarDeclaration(src, to, s.Player))
				a.afterBubbles = func() {
					a.startBattle(at, src, to, force, groups,
						game.Supply{Gold: gold, Rice: rice})
				}
			})
		})
	})
}

// askNewGovernor 是「選擇新任太守」（`0x1d6ed`，`DS:0x764e`）：模式 2、鍵 3 的挑人清單，
// **不能取消**——原版回 −1 就再問一次（`0x1d6f8`）。
func (a *app) askNewGovernor(at int) {
	a.view.Sel = at
	a.askRoster(t("ask.newGovernor"), at, game.PickServing, game.PickByCharm, func(gi int) {
		if err := a.s.G.AssignGovernor(at, gi); err != nil {
			a.view.Prompt = game.ErrorText(err)
		}
	}, func() { a.askNewGovernor(at) })
}

// askHeir 是「請選擇繼任的將軍」（`0x14f7c`，`DS:0x65dd`，Issue #65）：君主死掉時
// 候選是**整個勢力**依魅力排好的名單（不限死者那一郡），第三欄是魅力。
// **不能取消**——原版收到 −1 或空白鍵換頁都只是跳回去重畫（`0x15110`）。
func (a *app) askHeir(list []int) {
	a.askRosterList(tf("ask.heir", len(list)), list, game.PickByCharm, func(gi int) {
		if err := a.s.G.AssignHeir(gi); err != nil {
			a.view.Prompt = game.ErrorText(err)
			return
		}
		// 繼承那一則對白到這裡才排進佇列（說話的是玩家挑的那一位）。
		a.s.Queue(a.s.G.PendingEvents())
	}, func() { a.askHeir(list) })
}

// formation 是一次整編的狀態（原版 `0x20a30` 的迴圈，Issue #98）。
//
// pool 的順序就是「分配那一位將軍(1-n)」的編號；groups 與它對齊，
// 0 表示還沒分配，1–5 是第幾軍。**沒有被分配的人不出征**——原版的
// 整編同時決定「誰去」與「分到哪一軍」（`0x20c9a` 只把編進部隊的人
// 的所在郡寫 0）。
type formation struct {
	pool   []int
	groups []int
	then   func(force []int, groups []int)

	// fillRest 為真時收工要把沒分到的人補進中軍：**主守軍必須派出所有
	// 兵力**（說明書 p.27），守方沒有「留在家裡」這個選項。
	fillRest bool
}

// count 是第 k 軍已經分了幾位。
func (f *formation) count(k int) int {
	n := 0
	for _, g := range f.groups {
		if g == k {
			n++
		}
	}
	return n
}

// askAssign 問整編的那一對問題，一位一位來。
//
// 原版的規則（`docs/re/05` §12）：「分配那一位將軍」→「將%s分到那一軍」
// → 回頭再問將軍；**空欄位 Enter 取消才出「分配完畢(Y/N)」**
// （`0x21493`），`N` 是再分一位、`Y` 收工。已經分配過的人再選一次會被
// 退回重問（`0x214af`）。
func (a *app) askAssign(f *formation) {
	a.form = f
	g := a.s.G
	a.view.SetPage(ui.FormationPage(g, f.pool, f.groups))
	a.askBare(tf("ask.assignWho", len(f.pool)), 1, len(f.pool), func(n int) {
		i := n - 1
		if i < 0 || i >= len(f.pool) {
			a.askAssign(f)
			return
		}
		if f.groups[i] != 0 {
			// 原版退回重問，不是拒絕命令。
			a.view.Prompt = tf("form.assigned", personName(g, f.pool[i]))
			a.askAssign(f)
			return
		}
		a.askBare(tf("ask.assignTo", personName(g, f.pool[i])), 1, 5, func(k int) {
			// 一軍最多 10 位（說明書 p.27）。滿了就退回重問——擋在這裡，
			// 不要等到 `Reform`：那時 `BeginAttack` 已經把主事者交接完了，
			// 失敗回去等於留下一個改過一半的盤面。
			if k >= 1 && k <= 5 && f.count(k) >= battle.MaxLeaders {
				a.view.Prompt = tf("form.armyFull", ui.FormationName(battle.DeployOrder()[k-1]),
					battle.MaxLeaders)
				a.askAssign(f)
				return
			}
			f.groups[i] = k
			a.askAssign(f)
		})
		a.cancel = func() { a.askAssign(f) }
	})
	// 空欄位 Enter ＝ 取消 ＝ 問「分配完畢(Y/N)」。
	a.cancel = func() { a.finishAssign(f) }
}

// finishAssign 是「分配完畢(Y/N)」：Y 收工，N（或任何不是 Y 的鍵）再分一位。
// 一位都沒分配時不能收工——那樣等於沒有人出征。
func (a *app) finishAssign(f *formation) {
	a.askYNElse(t("ask.assignDone"), func() {
		if f.fillRest {
			// 沒分到的補進**人最少的那一軍**，不是一律塞中軍——十位就滿了
			// （說明書 p.27），全部塞進去會讓 `Reform` 當場失敗，而那時
			// 盤面已經動過了。
			for i, k := range f.groups {
				if k != 0 {
					continue
				}
				least, n := 1, f.count(1)
				for g := 2; g <= 5; g++ {
					if c := f.count(g); c < n {
						least, n = g, c
					}
				}
				f.groups[i] = least
			}
		}
		var force []int
		for i, k := range f.groups {
			if k > 0 {
				force = append(force, f.pool[i])
			}
		}
		if len(force) == 0 {
			a.view.Prompt = t("form.needOne")
			a.askAssign(f)
			return
		}
		a.form, a.view.Page = nil, nil
		f.then(force, f.groups)
	}, func() { a.askAssign(f) })
}

// personName 取一位人物的譯名。
func personName(g *game.State, index int) string {
	if x := g.General(index); x != nil {
		return ui.PersonName(x.Name)
	}
	return ""
}

// askSource 是「從那一郡攻打／移出／送出」（`0x18998`／`0x18c6d`／`0x19092`，Issue #82）：
// **只在下令那一郡的主事者是君主本人時才問**（`0x18bf4`／`0x18ff9`），收的是任何自己的郡，
// 不限下令的那一郡；挑到之後地圖與右側面板換成來源郡（`0x18c8d`）。回合仍記在下令的郡——
// 郡回合入口 `0x1746e` 沒有「這個月下過令」的旗標，來源郡自己那一次還在。
func (a *app) askSource(sel int, prompt string, then func(src int)) {
	g := a.s.G
	home := g.Prefecture(sel)
	if home == nil {
		return
	}
	gov := g.Governor(sel)
	if gov == nil || gov.Status != state.StatusLord {
		then(sel)
		return
	}
	a.askPref(prompt, func(id int) bool {
		q := g.Prefecture(id)
		return q != nil && q.Owned() && q.Owner == home.Owner
	}, func(src int) {
		a.view.Sel = src
		then(src)
	}, func() { a.view.Sel = sel })
}

func (a *app) moveTroops(sel int) {
	g := a.s.G
	next := func(src int) {
		a.askPref(t("ask.moveTo"), func(to int) bool {
			q, p := g.Prefecture(to), g.Prefecture(src)
			return q != nil && p != nil && g.Adjacent(src, to) && (!q.Owned() || q.Owner == p.Owner)
		}, func(to int) {
			people, gold, rice := g.MoveLimits(src, to)
			roster := len(g.PickRoster(src, game.PickServing, game.PickByStatus))
			a.askRosterMulti(t("ask.moveWho"), src, game.PickServing, game.PickBySoldiers, people, func(list []int) {
				// 「共調%d位將軍」與金米兩問都不清訊息（`0x18da2`），接在清單那一行後面。
				head := t("ask.moveWho") + tf("pick.range", 1, roster) + tf("ask.moveCount", len(list)) + t("ask.sendGold")
				a.askNumber(head, "", gold, func(gd int) {
					riceHead := head + tf("pick.range", 0, gold) + strconv.Itoa(gd) + t("ask.sendRice")
					a.askNumber(riceHead, "", rice, func(rc int) {
						a.run(game.MoveOrder{At: sel, From: src, To: to, Generals: list, Gold: gd, Rice: rc})
					})
				})
			}, nil)
		}, nil)
	}
	a.askSource(sel, t("ask.moveFrom"), next)
}

// plotFlow 是計略（類別 8）的問法（`docs/spec/014` §4.4）：每一種先用挑郡清單問郡、有的問兩三個，
// 最後才用挑人清單（模式 2、鍵 3 魅力）問使者；任何一格取消印「取消」收掉這道命令。
//
//	驅虎吞狼 `0x2caf8`：出使那一郡（有主、不是自己）→ 驅使攻打那一郡（出使郡的鄰郡，有主、不是自己也不是出使郡的主人）
//	遠交近攻 `0x2c3de`：出使那一郡（有主、不是自己，而且鄰郡的鄰郡有自己的郡）→ 聯合攻打那一郡（出使郡的鄰郡，
//	         有主、不是自己也不是出使郡的主人，而且鄰接自己的郡）→ 聯合我方那一郡（攻打郡的鄰郡裡自己的）
//	偽書使疑 `0x2cf16`／策反人民 `0x2d41a`：派細作到那一郡（有主、不是自己）
//	聯合出兵 `0x2d93c`：從我方那一郡出兵（自己的）→ 聯合攻打那一郡（鄰郡，有主、不是自己）→
//	         聯合我方那一郡合攻（攻打郡的鄰郡裡自己的、不是出兵那一郡；一個都沒有印「無法聯合出兵」）
func (a *app) plotFlow(sel int, plot game.Plot) {
	g := a.s.G
	home := g.Prefecture(sel)
	if home == nil {
		return
	}
	mine := func(id int) bool {
		q := g.Prefecture(id)
		return q != nil && q.Owned() && q.Owner == home.Owner
	}
	enemy := func(id int) bool {
		q := g.Prefecture(id)
		return q != nil && q.Owned() && q.Owner != home.Owner
	}
	neighbours := func(id int, ok func(int) bool) bool {
		q := g.Prefecture(id)
		if q == nil {
			return false
		}
		for _, n := range q.Neighbours {
			if ok(n) {
				return true
			}
		}
		return false
	}
	cancel := func() { a.view.Prompt = t("msg.cancel") }
	envoy := func(prompt string, plan game.PlotPlan) {
		a.askRoster(prompt, sel, game.PickServing, game.PickByCharm, func(gi int) {
			plan.Envoy = gi
			a.run(game.PlotOrder{At: sel, What: plot, Plan: &plan})
		}, cancel)
	}
	switch plot {
	case game.PlotForgery, game.PlotIncite:
		key := map[game.Plot]string{game.PlotForgery: "plot.forge", game.PlotIncite: "plot.incite"}[plot]
		a.askPref(t(key+".at"), enemy, func(at int) {
			envoy(t(key+".envoy"), game.PlotPlan{At: at})
		}, cancel)
	case game.PlotTigerWolf:
		a.askPref(t("plot.tiger.at"), enemy, func(at int) {
			a.askPref(t("plot.tiger.strike"), func(id int) bool {
				return enemy(id) && g.Adjacent(at, id) && g.Prefecture(id).Owner != g.Prefecture(at).Owner
			}, func(strike int) {
				envoy(t("plot.tiger.envoy"), game.PlotPlan{At: at, Strike: strike})
			}, cancel)
		}, cancel)
	case game.PlotFarNear:
		a.askPref(t("plot.far.at"), func(id int) bool {
			return enemy(id) && neighbours(id, func(n int) bool { return neighbours(n, mine) })
		}, func(at int) {
			a.askPref(t("plot.far.strike"), func(id int) bool {
				return enemy(id) && g.Adjacent(at, id) && g.Prefecture(id).Owner != g.Prefecture(at).Owner &&
					neighbours(id, mine)
			}, func(strike int) {
				a.askPref(t("plot.far.ours"), func(id int) bool { return mine(id) && g.Adjacent(strike, id) },
					func(ours int) {
						envoy(t("plot.far.envoy"), game.PlotPlan{At: at, Strike: strike, Ours: ours})
					}, cancel)
			}, cancel)
		}, cancel)
	case game.PlotJointAttack:
		a.askPref(t("plot.joint.from"), mine, func(from int) {
			a.askPref(t("plot.joint.strike"), func(id int) bool { return enemy(id) && g.Adjacent(from, id) },
				func(strike int) {
					aid := func(id int) bool { return mine(id) && id != from && g.Adjacent(strike, id) }
					if !neighbours(strike, aid) {
						a.view.Prompt = t("plot.joint.none")
						return
					}
					a.askPref(t("plot.joint.aid"), aid, func(ours int) {
						a.run(game.PlotOrder{At: sel, What: plot, Plan: &game.PlotPlan{Ours: from, Strike: strike, OursAid: ours}})
					}, cancel)
				}, cancel)
		}, cancel)
	}
}

// headhunt 是君主→5.登用他國人才（`0x1d7b0`）：本郡現役將已達 50 或金不到 100 印訊息退出；
// 「登用那一郡的將軍」收任何別人的郡（不限相鄰，`0x1d8a1`），取消收掉這道命令；
// 「登用那一位將軍」是那一郡的挑人清單（模式 5、鍵 0），取消回到挑郡（`0x1da15`）。
func (a *app) headhunt(sel int) {
	g := a.s.G
	home := g.Prefecture(sel)
	if home == nil {
		return
	}
	switch {
	case g.StoredActiveGenerals(sel) >= game.MaxGeneralsPerPrefecture:
		a.view.Prompt = t("msg.headhuntFull")
		return
	case home.Gold < game.CostHeadhunt:
		a.view.Prompt = t("msg.headhuntGold")
		return
	}
	var ask func()
	ask = func() {
		a.askPref(t("ask.headhuntPref"), func(id int) bool {
			q := g.Prefecture(id)
			return q != nil && q.Owned() && q.Owner != home.Owner
		}, func(pref int) {
			a.askRoster(t("ask.headhunt"), pref, game.PickSubject, game.PickByStatus, func(gi int) {
				a.run(game.HeadhuntOrder{At: sel, Target: gi})
			}, ask)
		}, nil)
	}
	ask()
}
