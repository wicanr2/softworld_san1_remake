package ui

// 主戰場的畫面。
//
// 與主畫面同一個原則：**畫面不決定任何規則**，只讀 `internal/battle`。
// 這一層畫到 `image.RGBA`，所以無頭測得到——戰場的 bug 用眼睛看
// 很難分辨「畫錯了」與「規則錯了」。

import (
	"fmt"
	"image/color"

	"github.com/wicanr2/softworld_san1_remake/internal/battle"
	"github.com/wicanr2/softworld_san1_remake/internal/cells"
)

// 戰場版面（格）。戰場 21×15 格，每格畫兩個半形位置，奇數列右移一格
// ——六方向的格子在方形字格上就是這個排法。
const (
	fieldCol  = 1
	fieldRow  = 2
	fieldStep = 2 // 一個戰場格佔幾個半形位置
	sideCol   = 48
)

// terrainGlyph 是各地形在畫面上的字。
//
// 用單一漢字而不是符號：漢字在點陣字型裡是 16×16，剛好填滿兩個半形格，
// **不會出現半格的縫**。符號多半是半形的，排起來會歪。
var terrainGlyph = map[battle.Terrain]string{
	battle.Plain:    "・",
	battle.Desert:   "﹕",
	battle.Hill:     "山",
	battle.Forest:   "林",
	battle.Shallow:  "水",
	battle.Deep:     "淵",
	battle.City:     "城",
	battle.Fort:     "寨",
	battle.Mountain: "巖",
}

var terrainColour = map[battle.Terrain]color.RGBA{
	battle.Plain:    {0x50, 0x60, 0x50, 0xFF},
	battle.Desert:   {0x70, 0x68, 0x48, 0xFF},
	battle.Hill:     {0x80, 0x70, 0x50, 0xFF},
	battle.Forest:   {0x40, 0x80, 0x48, 0xFF},
	battle.Shallow:  {0x50, 0x80, 0xA0, 0xFF},
	battle.Deep:     {0x38, 0x58, 0xA0, 0xFF},
	battle.City:     {0xD0, 0xC0, 0x70, 0xFF},
	battle.Fort:     {0xB0, 0x90, 0x60, 0xFF},
	battle.Mountain: {0x60, 0x60, 0x68, 0xFF},
}

// BattleView 是戰場畫面要顯示的東西。
type BattleView struct {
	// Cursor 是游標所在的格；Acting 是輪到誰下令（可以是 nil）。
	Cursor Hexer
	Acting *battle.Unit

	// Menu 是展開中的指令選單標題；空字串表示還沒選指令。
	Menu  string
	Items []string

	Prompt string

	// Page 覆蓋整個戰場區（查看部隊、戰報）。
	PageTitle string
	Page      []string
}

// Hexer 讓畫面不必直接依賴座標型別的零值語意。
type Hexer struct {
	At    battle.Hex
	Shown bool
}

// DrawBattle 畫一場進行中的戰役。
func DrawBattle(c *Canvas, b *battle.Battle, v BattleView) {
	c.Fill(ColBG)
	drawField(c, b, v)
	drawBattleSide(c, b, v)
	if len(v.Page) > 0 {
		drawPage(c, v.PageTitle, v.Page)
	}
	if v.Prompt != "" {
		c.DrawText(fieldCol, Rows-1, cells.Truncate(v.Prompt, Cols-fieldCol-1), ColSel)
	}
}

// drawField 畫格子與上面的部隊。
func drawField(c *Canvas, b *battle.Battle, v BattleView) {
	f := b.Field
	for y := 0; y < f.H; y++ {
		row := fieldRow + y
		if row >= Rows-1 {
			break
		}
		for x := 0; x < f.W; x++ {
			h := battle.FromOffset(x, y)
			col := fieldCol + x*fieldStep + y%2
			if col+fieldStep > sideCol {
				break
			}
			t := f.At(h)
			glyph, fg := terrainGlyph[t], terrainColour[t]
			if u := b.UnitAt(h); u != nil {
				glyph, fg = unitGlyph(u), unitColour(u)
			}
			if v.Cursor.Shown && v.Cursor.At == h {
				fg = ColSel
			}
			c.DrawText(col, row, glyph, fg)
		}
	}
}

// unitGlyph 是一支部隊在格子上的字：隊伍的第一個字。
//
// 先鋒／左軍／右軍／中軍／後軍 → 先／左／右／中／後。
func unitGlyph(u *battle.Unit) string {
	name := u.Formation.String()
	for _, r := range name {
		return string(r)
	}
	return "？"
}

// sideColour 分攻守兩色。**同一方的五個隊伍不再分色**：
// 戰場上要一眼看出的是敵我，不是番號。
func sideColour(s battle.Side) color.RGBA {
	if s.Attacking() {
		return color.RGBA{0xE0, 0x80, 0x70, 0xFF}
	}
	return color.RGBA{0x70, 0xB0, 0xE0, 0xFF}
}

func unitColour(u *battle.Unit) color.RGBA { return sideColour(u.Side) }

// drawBattleSide 畫右側的狀態欄。
func drawBattleSide(c *Canvas, b *battle.Battle, v BattleView) {
	c.DrawBox(sideCol, 0, Cols-sideCol, Rows, ColFrame)
	col, row := sideCol+2, 1
	put := func(s string, fg color.RGBA) {
		if row >= Rows-1 {
			return
		}
		c.DrawText(col, row, cells.Truncate(s, Cols-col-2), fg)
		row++
	}
	put(fmt.Sprintf("第 %d 日／%d", b.Day, battle.BattleDays), ColSel)
	put(b.Weather.String(), ColFG)
	row++

	for _, s := range []battle.Side{battle.MainAttacker, battle.AidAttacker,
		battle.MainDefender, battle.AidDefender} {
		n, men := 0, 0
		for _, u := range b.Units {
			if u.Side == s && u.Alive() {
				n++
				men += u.Soldiers()
			}
		}
		if n == 0 {
			continue
		}
		put(fmt.Sprintf("%s %d隊", s, n), sideColour(s))
		put(fmt.Sprintf("  兵%d 金%d 米%d", men, b.Gold[s], b.Rice[s]), ColFG)
	}
	row++

	if u := v.Acting; u != nil && u.Alive() {
		put("── 輪到 ──", ColSel)
		put(u.Name(), unitColour(u))
		put(fmt.Sprintf("兵 %d　餘步 %d", u.Soldiers(), u.Move), ColFG)
		if ch := u.Chief(); ch != nil {
			put(fmt.Sprintf("體能 %d　戰力 %d", ch.Stamina, ch.War), ColFG)
		}
		put(fmt.Sprintf("箭 %d 次", u.Arrows()), ColFG)
	}
	if v.Menu != "" {
		row++
		put("── "+v.Menu+" ──", ColSel)
		for _, it := range v.Items {
			put(it, ColFG)
		}
	}
}

// BattleCommandLines 是部隊層的指令，排成原版的三行三列。
//
// 原版的選單是（`AA.EXE` `0x46c02`，`docs/re/04` §4）：
//
//	1.移動 2.對戰 3.快戰
//	4.死戰 5.弓箭 6.策略
//	7.查看 8.退兵 0.休息
//
// **照它排三行**：一行一個指令會佔掉九列，右側欄放不下，
// 而放不下的那一個就是 `0.休息`——一個看起來像「這個版本沒有休息」的畫面。
func BattleCommandLines() []string {
	cmds := []battle.Command{
		battle.CmdMove, battle.CmdEngage, battle.CmdQuick,
		battle.CmdDeath, battle.CmdArchery, battle.CmdPlot,
		battle.CmdInspect, battle.CmdRetreat, battle.CmdRest,
	}
	var out []string
	for i := 0; i < len(cmds); i += 3 {
		line := ""
		for _, c := range cmds[i:min3(i+3, len(cmds))] {
			line += fmt.Sprintf("%d.%s ", int(c), c)
		}
		out = append(out, line)
	}
	return out
}

// BattleEngageLines 是單位層的指令（原版 `0x46fa0`）：
//
//	1.行軍 2.單挑 3.攻擊
//	7.查看 0.休息
func BattleEngageLines() []string {
	return []string{"1.行軍 2.單挑 3.攻擊", "7.查看 0.休息"}
}

func min3(a, b int) int {
	if a < b {
		return a
	}
	return b
}

// BattleUnitPage 是「查看」一支部隊的內容。
func BattleUnitPage(u *battle.Unit) (string, []string) {
	if u == nil {
		return "查看", []string{"（沒有這一支）"}
	}
	out := []string{
		fmt.Sprintf("%s　兵 %d　餘步 %d", u.Name(), u.Soldiers(), u.Move),
		fmt.Sprintf("訓練 %d　武裝 %d　兵種 %s　箭 %d 次",
			u.AvgTraining(), u.AvgArms(), u.Troop(), u.Arrows()),
		"",
		cells.Pad("姓名", 8) + cells.Pad("戰", 4) + cells.Pad("謀", 4) +
			cells.Pad("體", 4) + cells.Pad("兵士", 7) + cells.Pad("訓", 4) + "武",
	}
	for i := range u.Leaders {
		l := &u.Leaders[i]
		state := ""
		switch {
		case l.Dead:
			state = "（歿）"
		case l.Captured:
			state = "（被擒）"
		}
		out = append(out, cells.Pad(l.Name+state, 8)+
			cells.Pad(fmt.Sprintf("%d", l.War), 4)+
			cells.Pad(fmt.Sprintf("%d", l.Intel), 4)+
			cells.Pad(fmt.Sprintf("%d", l.Stamina), 4)+
			cells.Pad(fmt.Sprintf("%d", l.Soldiers), 7)+
			cells.Pad(fmt.Sprintf("%d", l.Training), 4)+
			fmt.Sprintf("%d", l.Arms))
	}
	return "查看", out
}
