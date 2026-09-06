package ui

import (
	"fmt"
	"image/color"
	"strings"

	"github.com/wicanr2/softworld_san1_remake/internal/cells"
	"github.com/wicanr2/softworld_san1_remake/internal/game"
	"github.com/wicanr2/softworld_san1_remake/internal/i18n"
	"github.com/wicanr2/softworld_san1_remake/internal/state"
)

// Command 是主畫面的一類指令。編號與原版相同（`docs/re/02`）。
type Command struct {
	Key  byte
	Name string
}

// t 取一句介面文字。語系是 `i18n.Current`。
//
// **繁體中文是原文不是譯文**（`internal/i18n`）；換語系只換譯文，
// 不會動到遊戲資料裡的專有名詞（郡名、人名）——那是玩家自己那一份
// 原版檔案的內容。
func t(key string) string { return i18n.S(key) }

// tf 取一句帶參數的介面文字。
func tf(key string, a ...any) string { return i18n.Sf(key, a...) }

// Commands 是主畫面右下的十類指令，順序與編號與原版相同。
func Commands() []Command {
	keys := []string{"cmd.status", "cmd.view", "cmd.military", "cmd.troops",
		"cmd.civil", "cmd.trade", "cmd.people", "cmd.lord", "cmd.plot", "cmd.other"}
	out := make([]Command, 0, len(keys))
	for i, k := range keys {
		out = append(out, Command{Key: byte('0' + i), Name: t(k)})
	}
	return out
}

// 版面（格）。原版把畫面分成左邊的時間、中間的地圖、右上的訊息欄與
// 右下的指令欄（說明書 p.16–17）。這裡照同一個分區，**但地圖是
// remake 自己畫的**——原版的地圖是美術素材，不重製也不散布。
const (
	timeCol  = 0
	timeW    = 4
	mapCol   = timeW
	panelCol = 52
	panelW   = Cols - panelCol
)

// View 是畫面要顯示的東西。**畫面不決定任何規則**——它只讀 Session。
type View struct {
	Sel int // 訊息欄要顯示哪一個郡

	// Menu 是目前展開的子選單；空字串表示在主選單。
	Menu string

	// Items 是子選單的項目（`Menu` 非空時才有意義）。
	Items []Command

	// Prompt 是提示列要顯示的一行字。
	Prompt string

	// Page 是覆蓋在地圖區的整頁內容（將軍列表、領土列表…）；
	// 非空時蓋掉訊息紀錄。
	PageTitle string
	Page      []string

	// Over 為真表示這一局結束了（勝、敗、或被消滅）。
	Over bool

	// Calendar 是年月的表示方式（原版「其他 → 年號」，手冊 p.26）。
	// 零值是中曆，與原版的預設相同。
	Calendar game.Calendar
}

// DrawMainScreen 畫遊戲主畫面。
func DrawMainScreen(c *Canvas, g *game.State, sel int) {
	DrawSession(c, g, nil, View{Sel: sel})
}

// DrawSession 畫一局進行中的遊戲。log 可以是 nil。
//
// v.Over 為真時在提示列說出結局——**被消滅之後畫面只是變空白的話，
// 玩家會以為是壞掉。**
func DrawSession(c *Canvas, g *game.State, log []string, v View) {
	c.Fill(ColBG)
	drawTimeColumn(c, g.Date, v.Calendar)
	drawMap(c, g, v.Sel)
	drawInfoPanel(c, g, v.Sel)
	if v.Menu == "" {
		drawCommandPanel(c, t("page.command"), Commands())
	} else {
		drawCommandPanel(c, v.Menu, v.Items)
	}
	if len(v.Page) > 0 {
		drawPage(c, v.PageTitle, v.Page)
	} else {
		drawLog(c, log)
	}
	if v.Over {
		c.DrawText(mapCol+2, Rows-3, t("msg.over"), ColWarn)
	}
	if v.Prompt != "" {
		c.DrawText(mapCol+2, Rows-2, cells.Truncate(v.Prompt, panelCol-mapCol-4), ColSel)
	}
	if n := len(c.Missing); n > 0 {
		c.DrawText(mapCol+2, Rows-1, tf("msg.noGlyph", n), ColWarn)
	}
}

// drawPage 用整頁內容蓋掉地圖區——列表型的指令（將軍列表、領土列表）
// 要的空間比訊息列多。
func drawPage(c *Canvas, title string, lines []string) {
	w := panelCol - mapCol
	c.DrawBox(mapCol, 0, w, Rows, ColFrame)
	for y := 1; y < Rows-1; y++ {
		for x := mapCol + 1; x < panelCol-1; x++ {
			c.DrawText(x, y, " ", ColBG)
		}
	}
	c.DrawText(mapCol+2, 0, title, ColSel)
	for i, line := range lines {
		if 1+i >= Rows-1 {
			c.DrawText(mapCol+2, Rows-1, t("msg.more"), ColDim)
			break
		}
		c.DrawText(mapCol+2, 1+i, cells.Truncate(line, w-4), ColFG)
	}
}

// drawLog 畫地圖區下緣的訊息。**失敗的命令也要看得見**——
// 靜靜地沒反應會讓玩家以為是按鍵沒進去。
func drawLog(c *Canvas, log []string) {
	const rows = 5
	top := Rows - rows - 3
	if len(log) > rows {
		log = log[len(log)-rows:]
	}
	for i, line := range log {
		// 失敗與警告用警示色。**不要只看第一個 byte**：`─`（U+2500）
		// 與 `✗`（U+2717）的 UTF-8 首位元組都是 0xE2，分月線會整排變紅。
		fg := ColDim
		if strings.HasPrefix(line, "✗") || strings.HasPrefix(line, "⚠") {
			fg = ColWarn
		}
		c.DrawText(mapCol+2, top+i, cells.Truncate(line, panelCol-mapCol-4), fg)
	}
}

// drawTimeColumn 畫左側直排的年月，與原版同一個形狀（「中平六年元月」）。
//
// 直排是**漢字才成立的版面**：一個字剛好填滿一格，往下排讀得下去。
// 譯成拼音之後同一個年號變成九個字母，四格寬的欄位一列放不下一個字，
// 拆開往下排就成了每列一個字母的長條。所以拉丁字母的語系改成橫排，
// 放在地圖區上緣——版面讓給可讀性。
func drawTimeColumn(c *Canvas, d game.Date, cal game.Calendar) {
	c.DrawBox(timeCol, 0, timeW, Rows, ColFrame)
	s := d.Format(cal)
	if !hasWide(s) {
		c.DrawText(mapCol+2, 0, s, ColFG)
		return
	}
	i := 0
	for _, ch := range s {
		if 2+i >= Rows-1 {
			break
		}
		c.DrawText(timeCol+2, 2+i, string(ch), ColFG)
		i++
	}
}

// hasWide 說一段文字裡有沒有全形字。
//
// 判準是「有沒有」不是「全部是不是」：西曆的中文寫法是 `189年1月`，
// 阿拉伯數字混著漢字，原版也是一位一列往下排。
func hasWide(s string) bool {
	for _, ch := range s {
		if cells.Width(string(ch)) == 2 {
			return true
		}
	}
	return false
}

// drawMap 畫地圖區。
//
// **這不是原版的地圖。** 原版是一張美術圖，remake 不重製也不散布它。
// 這裡用相鄰表（`docs/spec/003` §3.2）把 42 個郡排成格狀，每一格顯示
// 郡號與所屬勢力的代號——版面不同，但**拓樸與原版一致**，
// 而拓樸才是規則層要的東西。
func drawMap(c *Canvas, g *game.State, sel int) {
	c.DrawBox(mapCol, 0, panelCol-mapCol, Rows, ColFrame)
	const cellW, perRow = 7, 6
	for i, p := range g.Prefectures() {
		col := mapCol + 2 + (i%perRow)*cellW
		row := 2 + (i/perRow)*2
		if row >= Rows-2 {
			break
		}
		fg := ColDim
		if p.Owned() {
			fg = factionColour(p.Owner)
		}
		c.DrawText(col, row, fmt.Sprintf("%2d", p.ID), ColDim)
		name := cells.Truncate(p.Name, 4)
		if p.ID == sel {
			// 選取中的郡用反白框標出來，不靠顏色——**顏色會與勢力衝突**。
			c.DrawText(col+2, row, name, ColSel)
			c.DrawBox(col+1, row, 6, 1, ColSel)
		} else {
			c.DrawText(col+2, row, name, fg)
		}
	}
}

// drawInfoPanel 畫右上的訊息欄。
//
// 欄位與原版相同（說明書 p.17）：郡名與編號、所屬諸侯、主事者、
// 金、米、現役將、在野武將、兵士。
func drawInfoPanel(c *Canvas, g *game.State, sel int) {
	c.DrawBox(panelCol, 0, panelW, 13, ColFrame)
	p := g.Prefecture(sel)
	if p == nil {
		c.DrawText(panelCol+2, 2, t("msg.noPrefecture"), ColDim)
		return
	}
	_ = p
	line := func(row int, label, value string) {
		c.DrawText(panelCol+2, row, cells.Pad(label, 10), ColDim)
		c.DrawText(panelCol+12, row, value, ColFG)
	}
	c.DrawText(panelCol+2, 1, fmt.Sprintf("%2d %s", p.ID, p.Name), ColSel)

	if !p.Owned() {
		c.DrawText(panelCol+2, 3, t("msg.blankPref"), ColDim)
	} else {
		lord := g.Lord(p.Owner)
		gov := g.Governor(p.ID)
		if lord != nil {
			line(3, t("fld.lord"), lord.Name)
		}
		if gov != nil {
			line(4, t("fld.governor"), gov.Name)
		}
	}
	line(6, t("fld.gold"), fmt.Sprintf("%d", p.Gold))
	line(7, t("fld.rice"), fmt.Sprintf("%d", p.Rice))
	line(8, t("fld.population"), fmt.Sprintf("%d", p.Population))
	line(9, t("fld.soldiers"), fmt.Sprintf("%d", g.Soldiers(p.ID)))

	line(10, t("fld.active"), tf("fld.activeFree",
		g.ActiveGenerals(p.ID), g.FreeGenerals(p.ID)))
	line(11, t("fld.landFlood"), fmt.Sprintf("%d / %d", p.LandValue, p.FloodRate))
}

// drawCommandPanel 畫右下的指令欄。原版是兩欄五列。
func drawCommandPanel(c *Canvas, title string, cmds []Command) {
	c.DrawBox(panelCol, 13, panelW, Rows-13, ColFrame)
	c.DrawText(panelCol+2, 13, title, ColSel)
	per := 5
	if len(cmds) <= 5 {
		per = len(cmds)
	}
	for i, cmd := range cmds {
		col := panelCol + 2 + (i/per)*13
		row := 15 + i%per
		if row >= Rows-1 {
			break
		}
		c.DrawText(col, row, fmt.Sprintf("%c.", cmd.Key), ColDim)
		c.DrawText(col+3, row, cmd.Name, ColFG)
	}
	if len(cmds) < len(Commands()) {
		c.DrawText(panelCol+2, Rows-2, t("msg.back"), ColDim)
	}
}

// SubMenu 回傳某一類指令底下的項目，編號與手冊相同
// （`docs/reference/01-manual-10-commands.md`）。
//
// **只列已經實作的。** 列出來卻按不動的項目比沒列更糟——
// 玩家會以為是自己按錯。沒實作的類別回 nil，呼叫端顯示理由。
// SubMenu 回傳一類指令的子選單。
//
// **文字一律照原版執行檔的字串表**（`docs/re/04` §2），不照手冊轉錄：
// 手冊是二手的，而且有出入——「其他」的第一項原版寫 `結束`，手冊寫
// 「＊結束」（`＊` 是手冊自己的記號）。原版的錯字也照原樣留著
// （「洪水防冶」「郡縣自冶」），理由同 `Floopy`（F29）。
//
// 還沒實作的項目**照樣列出來**：選單少一項，玩家看不出是沒做還是
// 原版沒有；列出來按下去會說還沒實作，那是可以理解的狀態。
func SubMenu(key byte) (string, []Command) {
	var title string
	var keys []string
	switch key {
	case '1':
		title, keys = "cmd.view", []string{"view.pick", "view.generals",
			"view.inspect", "view.territory", "view.terrain", "view.treasury"}
	case '2':
		title, keys = "cmd.military", []string{"mil.move", "mil.attack", "mil.transport"}
	case '3':
		title, keys = "cmd.troops", []string{"tro.train", "tro.conscript",
			"tro.arms", "tro.balance"}
	case '4':
		title, keys = "cmd.civil", []string{"civ.reclaim", "civ.flood",
			"civ.fort", "civ.rest"}
	case '5':
		title, keys = "cmd.trade", []string{"trd.buy", "trd.sell", "trd.relief"}
	case '6':
		title, keys = "cmd.people", []string{"ppl.search", "ppl.recruit",
			"ppl.reward", "ppl.dismiss"}
	case '7':
		title, keys = "cmd.lord", []string{"lord.chief", "lord.governor",
			"lord.autonomy", "lord.gift", "lord.headhunt"}
	case '8':
		title, keys = "cmd.plot", []string{"plot.tiger", "plot.distant",
			"plot.forge", "plot.incite", "plot.joint"}
	case '9':
		title, keys = "cmd.other", []string{"oth.quit", "oth.save", "oth.music",
			"oth.sound", "oth.delay", "oth.war", "oth.era", "oth.voice"}
	default:
		return "", nil
	}
	items := make([]Command, 0, len(keys))
	for i, k := range keys {
		items = append(items, Command{Key: byte('1' + i), Name: t(k)})
	}
	return t(title), items
}

// ColSel 是選取中的顏色。
var ColSel = color.RGBA{0xFF, 0xD0, 0x60, 0xFF}

// factionColours 給每個勢力一個顏色。
//
// **這不是原版的色盤。** 原版用 `EGAFILL.PAL`／`HERCFILL.PAL`
// （`docs/formats/01` §3），那還沒解。這裡只求「相鄰的勢力看得出不同」，
// 解出來之後要換掉。
var factionColours = []color.RGBA{
	{0x60, 0xC0, 0x60, 0xFF}, {0x60, 0x90, 0xE0, 0xFF}, {0xE0, 0x80, 0x60, 0xFF},
	{0xE0, 0xD0, 0x60, 0xFF}, {0xC0, 0x70, 0xC0, 0xFF}, {0xA0, 0xA0, 0xA0, 0xFF},
	{0x60, 0xC0, 0xC0, 0xFF}, {0xC0, 0xC0, 0x80, 0xFF}, {0xE0, 0x60, 0x90, 0xFF},
	{0x80, 0xE0, 0xA0, 0xFF}, {0x90, 0x90, 0xE0, 0xFF}, {0xE0, 0xA0, 0x60, 0xFF},
	{0xB0, 0xE0, 0x60, 0xFF}, {0xE0, 0x60, 0x60, 0xFF},
}

func factionColour(f state.FactionID) color.RGBA {
	if int(f) < len(factionColours) {
		return factionColours[f]
	}
	return ColFG
}

// FactionLetter 給勢力一個單字元代號，方便在窄格子裡標示歸屬。
//
// **這是 remake 的顯示手段不是原版行為**——原版用顏色與紋飾區分
// （說明書 p.16）。顏色要等 `.PAL` 解出來才對得上。
func FactionLetter(f state.FactionID) byte {
	if f == state.NoFaction {
		return '.'
	}
	if f < 26 {
		return byte('A' + f)
	}
	return '?'
}
