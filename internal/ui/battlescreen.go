package ui

// 主戰場的畫面。
//
// 與主畫面同一個原則：**畫面不決定任何規則**，只讀 `internal/battle`。
// 這一層畫到 `image.RGBA`，所以無頭測得到——戰場的 bug 用眼睛看
// 很難分辨「畫錯了」與「規則錯了」。

import (
	"fmt"
	"image/color"
	"sort"
	"strings"

	"github.com/wicanr2/softworld_san1_remake/internal/assets"
	"github.com/wicanr2/softworld_san1_remake/internal/battle"
	"github.com/wicanr2/softworld_san1_remake/internal/cells"
)

// 戰場版面（格）。戰場 12×10 格，每格畫兩個半形位置
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

	// Window 不是空字串時第三塊面板是原版的**文字視窗**：清成青 3，照原版訊息
	// 常式（`0x33d8:0xcc0`）逐字排——換行字元換行、滿 22 格折行、超過 6 列
	// 往上捲——字色一律黃 14（`docs/spec/014` §7.1）。這時 Menu／Items／Prompt 不畫。
	Window string
	// Input 是文字視窗最後一行後面的輸入游標（`docs/spec/014` §4.1）。
	Input InputCursor

	// Menu 是展開中的指令選單標題；空字串表示還沒選指令。
	Menu  string
	Items []string

	Prompt string

	// Page 覆蓋整個戰場區（查看部隊、戰報）。
	PageTitle string
	Page      []string
	// PageTop 是分頁捲到第幾行（見 `View.PageTop`）。
	PageTop int

	// Inspecting 是查看中的那支部隊；原版素材的畫面把第三塊面板換成
	// 它第 0 槽那一位的肖像與資料（`ArtBattleInfo.Inspect`）。
	Inspecting *battle.Unit

	// SkirmishActing 是對戰子畫面裡輪到的那一位；Blink 為真時他那一格
	// 反白（原版每 512 個時脈切換一次，`0x1538:0x58ac`）。
	SkirmishActing *battle.SkirmishGeneral
	Blink          bool
}

// SetPage 打開一頁，捲回最上面。
func (v *BattleView) SetPage(title string, lines []string) {
	v.PageTitle, v.Page, v.PageTop = title, lines, 0
}

// ClosePage 收起分頁，查看中的部隊一起清掉。
func (v *BattleView) ClosePage() {
	v.PageTitle, v.Page, v.Inspecting = "", nil, nil
}

// ScrollPage 捲動分頁；art 為真表示原版素材的戰場畫面。
func (v *BattleView) ScrollPage(delta int, art bool) {
	cols, rows := BattlePageSize(art)
	v.PageTop = clampTop(len(PageLines(v.Page, cols)), v.PageTop+delta, rows)
}

// BattlePageSize 是戰場上分頁一頁放得下幾格寬、幾行。
func BattlePageSize(art bool) (cols, rows int) {
	if art {
		return (battlePageX1-battlePageX0)/CellW - 2, (battlePageY1-battlePageY0)/CellH - 2
	}
	return PageSize(false)
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
		drawPage(c, v.PageTitle, v.Page, v.PageTop)
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
	put(tf("msg.day", b.Day, battle.BattleDays), ColSel)
	put(WeatherName(b.Weather), ColFG)
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
		put(tf("bat.units", SideName(s), n), sideColour(s))
		put(tf("bat.supply", men, b.Gold[s], b.Rice[s]), ColFG)
	}
	row++

	if u := v.Acting; u != nil && u.Alive() {
		put("── "+t("msg.turnOf")+" ──", ColSel)
		put(u.Name(), unitColour(u))
		put(tf("bat.unitLine", u.Soldiers(), u.Move), ColFG)
		if ch := u.Chief(); ch != nil {
			put(tf("bat.chiefLine", ch.Stamina, ch.War), ColFG)
		}
		put(tf("bat.arrowsLine", u.Arrows), ColFG)
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
	items := make([]string, 0, len(cmds))
	for _, c := range cmds {
		items = append(items, fmt.Sprintf("%d.%s", int(c), CommandName(c)))
	}
	return packBattle(items)
}

// BattleOptionCols 是戰場指令面板一行幾格（176 像素的面板扣掉兩邊各 4）。
const BattleOptionCols = (assets.BattlePanelW - 8) / CellW

// packBattle 把選項排成原版的樣子：**一行最多三項**、塞不下 21 格就換行。
//
// 中文三項一行正好 20 格，排出來就是原版的三行三列；英日文的名字長，
// 一行只放得下一兩項，行數會超過面板的三行——那時原版素材的戰場畫面
// 改在面板正上方畫一個選單框（`docs/spec/014` §7）。**不截字**：
// 截掉的選項（「3.Tr」）比換個地方畫更糟。
func packBattle(items []string) []string { return packBattleCols(items, BattleOptionCols) }

// packBattleCols 是 `packBattle` 指定寬度的版本（小字級一行 28 字）。
func packBattleCols(items []string, cols int) []string {
	var out []string
	cur, n := "", 0
	for _, it := range items {
		if cur != "" && (n >= 3 || cells.Width(cur)+1+cells.Width(it) > cols) {
			out = append(out, cur)
			cur, n = "", 0
		}
		if cur != "" {
			cur += " "
		}
		cur += it
		n++
	}
	if cur != "" {
		out = append(out, cur)
	}
	return out
}

// BattleEngageLines 是單位層的指令（原版 `0x46fa0`）：
// 對戰模式下只剩移動、對戰、快戰，查看與休息另起一行。
func BattleEngageLines() []string {
	fmtCmd := func(c battle.Command) string { return fmt.Sprintf("%d.%s", int(c), CommandName(c)) }
	return append(
		packBattle([]string{fmtCmd(battle.CmdMove), fmtCmd(battle.CmdEngage), fmtCmd(battle.CmdQuick)}),
		packBattle([]string{fmtCmd(battle.CmdInspect), fmtCmd(battle.CmdRest)})...)
}

// BattleStratagemLines 是六種計謀，排成兩行。編號與原版相同。
func BattleStratagemLines() []string {
	all := []battle.Stratagem{battle.Fire, battle.Flood, battle.Trap,
		battle.Lure, battle.Burn, battle.Siege}
	items := make([]string, 0, len(all))
	for _, s := range all {
		items = append(items, fmt.Sprintf("%d.%s", int(s), StratagemName(s)))
	}
	return packBattle(items)
}


// BattleUnitPage 是「查看」一支部隊的內容。
func BattleUnitPage(u *battle.Unit) (string, []string) {
	if u == nil {
		return t("bat.inspect"), []string{t("msg.none")}
	}
	out := []string{
		tf("bat.unitHead", u.Name(), u.Soldiers(), u.Move),
		tf("bat.unitStats",
			u.AvgTraining(), u.AvgArms(), TroopKindName(u.Troop()), u.Arrows),
		"",
	}
	// 欄寬依內容決定（`cells.Columns`）：名字後面接「陣亡」「被俘」時，
	// 寫死的八格會把狀態截掉——那正是這一頁最要緊的一個字。
	rows := [][]string{{t("fld.name"), t("fld.war"), t("fld.intel"),
		t("fld.stamina"), t("fld.soldiers"), t("fld.training"), t("fld.arms")}}
	for i := range u.Leaders {
		l := &u.Leaders[i]
		state := ""
		switch {
		case l.Dead:
			state = t("bat.dead")
		case l.Captured:
			state = t("bat.captured")
		}
		rows = append(rows, []string{PersonName(l.Name) + state,
			fmt.Sprintf("%d", l.War), fmt.Sprintf("%d", l.Intel),
			fmt.Sprintf("%d", l.Stamina), fmt.Sprintf("%d", l.Soldiers),
			fmt.Sprintf("%d", l.Training), fmt.Sprintf("%d", l.Arms)})
	}
	return t("bat.inspect"), append(out, cells.Columns(rows)...)
}

// TerrainPage 是「郡地理誌」：某個郡的主戰場地形（說明書 p.19，
// 查看選單的第 5 項）。
//
// 地圖是**原版的資料**：州郡記錄 offset 55–174 的 120 個位元組
// （`internal/battle`.Load）。
//
// 文字版一格一個字，圖外留白。**原版是奇數欄往下移半格的六角格**，
// 那個半格在字元格子裡表現不出來，所以這裡只排成矩陣；相鄰關係
// 仍然照六角走（`Field.Step`），畫面版才按真的座標畫。
func TerrainPage(name string, f *battle.Field, gates map[int][]battle.Hex) (string, []string) {
	if f == nil {
		return t("page.terrain"), []string{t("msg.none")}
	}
	out := make([]string, 0, f.H+6)
	for y := 0; y < f.H; y++ {
		line := ""
		for x := 0; x < f.W; x++ {
			h := battle.FromOffset(x, y)
			if f.Outside(h) {
				line += "　"
				continue
			}
			line += terrainGlyph[f.At(h)]
		}
		out = append(out, strings.TrimRight(line, "　"))
	}
	out = append(out, "")
	// 「圖中的數字位置代表前往鄰近州郡的通道」（說明書 p.19）。
	var ns []int
	for n := range gates {
		ns = append(ns, n)
	}
	sort.Ints(ns)
	line := t("msg.gates")
	for _, n := range ns {
		line += tf("msg.gateN", n)
	}
	out = append(out, line)
	out = append(out, t("msg.legend")+terrainLegend())
	return name + "　" + t("page.terrain"), out
}

// terrainLegend 是地形圖例：一個字加上它的名字。
func terrainLegend() string {
	keys := []struct {
		t   battle.Terrain
		key string
	}{
		{battle.Plain, "ter.plain"}, {battle.Desert, "ter.desert"},
		{battle.Hill, "ter.hill"}, {battle.Forest, "ter.forest"},
		{battle.Shallow, "ter.shallow"}, {battle.Deep, "ter.deep"},
		{battle.City, "ter.city"}, {battle.Fort, "ter.fort"},
		{battle.Mountain, "ter.mountain"},
	}
	s := ""
	for _, k := range keys {
		s += terrainGlyph[k.t] + t(k.key) + " "
	}
	return s
}
