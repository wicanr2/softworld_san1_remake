package ui

import (
	"fmt"
	"image/color"
	"strings"

	"github.com/wicanr2/softworld_san1_remake/internal/cells"
	"github.com/wicanr2/softworld_san1_remake/internal/game"
	"github.com/wicanr2/softworld_san1_remake/internal/state"
)

// Command 是主畫面的一類指令。編號與原版相同（`docs/re/02`）。
type Command struct {
	Key  byte
	Name string
}

// Commands 是主畫面右下的十類指令，順序與編號與原版相同。
func Commands() []Command {
	return []Command{
		{'0', "狀態"}, {'1', "查看"}, {'2', "軍事"}, {'3', "兵士"}, {'4', "內政"},
		{'5', "商業"}, {'6', "人事"}, {'7', "君主"}, {'8', "謀略"}, {'9', "其他"},
	}
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
	drawTimeColumn(c, g)
	drawMap(c, g, v.Sel)
	drawInfoPanel(c, g, v.Sel)
	if v.Menu == "" {
		drawCommandPanel(c, "指令", Commands())
	} else {
		drawCommandPanel(c, v.Menu, v.Items)
	}
	if len(v.Page) > 0 {
		drawPage(c, v.PageTitle, v.Page)
	} else {
		drawLog(c, log)
	}
	if v.Over {
		c.DrawText(mapCol+2, Rows-3, "這一局結束了。訊息在下方。", ColWarn)
	}
	if v.Prompt != "" {
		c.DrawText(mapCol+2, Rows-2, cells.Truncate(v.Prompt, panelCol-mapCol-4), ColSel)
	}
	if n := len(c.Missing); n > 0 {
		c.DrawText(mapCol+2, Rows-1, fmt.Sprintf("⚠ %d 個字沒有字模", n), ColWarn)
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
			c.DrawText(mapCol+2, Rows-1, "…（還有更多）", ColDim)
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

// drawTimeColumn 畫左側直排的年月。
//
// 原版顯示年號（「中平六年元月」）。**年號表還沒解**，所以這裡先用西元；
// 換成年號是顯示層的事，不影響規則。
func drawTimeColumn(c *Canvas, g *game.State) {
	c.DrawBox(timeCol, 0, timeW, Rows, ColFrame)
	// 直排。數字逐位往下排，與原版左側的直排年號同一個形狀。
	rows := []string{}
	for _, ch := range fmt.Sprintf("%d", g.Date.Year) {
		rows = append(rows, string(ch))
	}
	rows = append(rows, "年", "")
	for _, ch := range fmt.Sprintf("%d", g.Date.Month) {
		rows = append(rows, string(ch))
	}
	rows = append(rows, "月")
	for i, s := range rows {
		if 2+i >= Rows-1 {
			break
		}
		c.DrawText(timeCol+2, 2+i, s, ColFG)
	}
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
		c.DrawText(panelCol+2, 2, "（未選擇州郡）", ColDim)
		return
	}
	_ = p
	line := func(row int, label, value string) {
		c.DrawText(panelCol+2, row, cells.Pad(label, 10), ColDim)
		c.DrawText(panelCol+12, row, value, ColFG)
	}
	c.DrawText(panelCol+2, 1, fmt.Sprintf("%2d %s", p.ID, p.Name), ColSel)

	if !p.Owned() {
		c.DrawText(panelCol+2, 3, "空白郡", ColDim)
	} else {
		lord := g.Lord(p.Owner)
		gov := g.Governor(p.ID)
		if lord != nil {
			line(3, "諸侯", lord.Name)
		}
		if gov != nil {
			line(4, "太守", gov.Name)
		}
	}
	line(6, "金", fmt.Sprintf("%d", p.Gold))
	line(7, "米", fmt.Sprintf("%d", p.Rice))
	line(8, "人口", fmt.Sprintf("%d", p.Population))
	line(9, "兵士", fmt.Sprintf("%d", g.Soldiers(p.ID)))

	line(10, "現役將", fmt.Sprintf("%d / 在野 %d",
		g.ActiveGenerals(p.ID), g.FreeGenerals(p.ID)))
	line(11, "土地/洪水", fmt.Sprintf("%d / %d", p.LandValue, p.FloodRate))
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
		c.DrawText(panelCol+2, Rows-2, "ESC 返回", ColDim)
	}
}

// SubMenu 回傳某一類指令底下的項目，編號與手冊相同
// （`docs/reference/01-manual-10-commands.md`）。
//
// **只列已經實作的。** 列出來卻按不動的項目比沒列更糟——
// 玩家會以為是自己按錯。沒實作的類別回 nil，呼叫端顯示理由。
func SubMenu(key byte) (string, []Command) {
	switch key {
	case '1':
		return "查看", []Command{{'2', "將軍列表"}, {'4', "領土列表"}, {'6', "君主物品"}}
	case '2':
		return "軍事", []Command{{'1', "調動軍隊"}, {'2', "發動戰役"}, {'3', "運送錢糧"}}
	case '3':
		return "兵士", []Command{{'1', "訓練兵士"}, {'2', "徵兵"},
			{'3', "購買武器"}, {'4', "調整兵力"}}
	case '4':
		return "內政", []Command{{'1', "土地開發"}, {'2', "洪水防治"},
			{'3', "建築關寨"}, {'4', "休息"}}
	case '5':
		return "商業", []Command{{'1', "買入米糧"}, {'2', "賣出米糧"}, {'3', "開倉賑民"}}
	case '6':
		return "人事", []Command{{'1', "尋訪人才"}, {'2', "登用人才"},
			{'3', "賞賜金帛"}, {'4', "撤職"}}
	case '7':
		return "君主", []Command{{'1', "指定軍師"}, {'2', "指定太守"},
			{'3', "郡縣自治"}, {'4', "賞賜物品"}, {'5', "登用他國人才"}}
	case '8':
		return "謀略", []Command{{'1', "驅虎吞狼"}, {'2', "遠交近攻"},
			{'3', "偽書使疑"}, {'4', "策反人民"}, {'5', "聯合出兵"}}
	case '9':
		// 手冊 p.25 的八項；還沒做的照樣列出來——**選單少一項，
		// 玩家看不出是沒做還是不存在**。按下去會說還沒實作。
		return "其他", []Command{{'1', "＊結束"}, {'2', "儲存"}, {'3', "音樂"},
			{'4', "音效"}, {'5', "延時"}, {'6', "戰役"}, {'7', "年號"}, {'8', "語音"}}
	}
	return "", nil
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
