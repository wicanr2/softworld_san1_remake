package ui

import (
	"fmt"
	"image/color"

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

// DrawMainScreen 畫遊戲主畫面。
//
// sel 是訊息欄要顯示哪一個郡；0 或越界時只畫地圖與指令。
func DrawMainScreen(c *Canvas, g *game.State, sel int) {
	c.Fill(ColBG)
	drawTimeColumn(c, g)
	drawMap(c, g, sel)
	drawInfoPanel(c, g, sel)
	drawCommandPanel(c)

	if n := len(c.Missing); n > 0 {
		c.DrawText(mapCol+1, Rows-1, fmt.Sprintf("⚠ %d 個字沒有字模", n), ColWarn)
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
	line(9, "兵士", fmt.Sprintf("%d", p.Soldiers))

	active := 0
	for _, x := range g.Garrison(p.ID) {
		if x.Faction == p.Owner {
			active++
		}
	}
	line(10, "現役將", fmt.Sprintf("%d", active))
	line(11, "土地/洪水", fmt.Sprintf("%d / %d", p.LandValue, p.FloodRate))
}

// drawCommandPanel 畫右下的指令欄。原版是兩欄五列。
func drawCommandPanel(c *Canvas) {
	c.DrawBox(panelCol, 13, panelW, Rows-13, ColFrame)
	cmds := Commands()
	for i, cmd := range cmds {
		col := panelCol + 2 + (i/5)*13
		row := 15 + i%5
		c.DrawText(col, row, fmt.Sprintf("%c.", cmd.Key), ColDim)
		c.DrawText(col+3, row, cmd.Name, ColFG)
	}
}

// ColSel 是選取中的顏色。
var ColSel = color.RGBA{0xFF, 0xD0, 0x60, 0xFF}

// factionColours 給每個勢力一個顏色。
//
// **這不是原版的色盤。** 原版用 `EGAFILL.PAL`／`HERCFILL.PAL`
//（`docs/formats/01` §3），那還沒解。這裡只求「相鄰的勢力看得出不同」，
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
