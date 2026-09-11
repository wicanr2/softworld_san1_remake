package ui

// 接上原版素材的主戰場。
//
// 版面全是原版的（`docs/spec/005` §主戰場、`docs/re/05` §2.5）：底紋、
// 上方花邊、場地圖塊、部隊的旗與兵力牌、兩個軍力面板與指令面板的位置、
// 肖像與肖像框。**字是 remake 自己的字庫**（`CLAUDE.md` §3.3），
// 所以字級與原版的 32×32／16×16 不同；面板裡的排法也是 remake 自己排的。

import (
	"fmt"
	"image"
	"image/draw"
	"strings"

	"github.com/wicanr2/softworld_san1_remake/internal/assets"
	"github.com/wicanr2/softworld_san1_remake/internal/battle"
	"github.com/wicanr2/softworld_san1_remake/internal/cells"
	"github.com/wicanr2/softworld_san1_remake/internal/game"
)

// ArtBattleInfo 是畫面上要寫、而戰場自己不知道的東西。
type ArtBattleInfo struct {
	// Prefecture／Province 是郡名與州名，畫在左欄。
	Prefecture string
	Province   string

	// Field 是州郡記錄 offset 55–174 那 120 個位元組。
	//
	// **要原版的位元組，不能拿 `battle.Field` 換回來**：`battle.Terrain`
	// 是 remake 自己的編號，與原版的地形碼不同，而圖塊的編號就是原版的
	// 地形碼——換錯了不會報錯，只會整張畫成別的地形。
	Field []byte

	// ID 是郡編號，畫在左欄第一個框的最後一列。
	ID int

	// Date 與 Calendar 是下方花邊上那一行年月（`docs/spec/011`）。
	Date     game.Date
	Calendar game.Calendar

	// Commander 是攻方與守方的統帥姓名，Portrait 是他們的肖像編號
	// （−1 ＝ 沒有）。順序是攻方、守方，與 `assets.BattleFrameX` 相同。
	Commander [2]string
	Portrait  [2]int
}

// artBattleSides 是兩個面板各自代表的軍力：攻方看主攻軍、守方看主守軍。
var artBattleSides = [2]battle.Side{battle.MainAttacker, battle.MainDefender}

// DrawArtBattle 畫一整張主戰場。
func DrawArtBattle(c *Canvas, ab *ArtBattle, b *battle.Battle, v BattleView, info ArtBattleInfo) {
	im := ab.compose(b, v, info)
	draw.Draw(c.Img, image.Rect(0, 0, assets.ScreenW, assets.ScreenH),
		im.RGBA(), image.Point{}, draw.Src)
	ab.drawText(c, b, v, info)
}

// compose 把所有圖層拼成一張 640×408 的索引圖。
func (ab *ArtBattle) compose(b *battle.Battle, v BattleView, info ArtBattleInfo) *assets.Image {
	var im *assets.Image
	if ab.bg != nil {
		im = ab.bg.Clone()
	} else {
		im = &assets.Image{W: assets.ScreenW, H: assets.ScreenH,
			Pix: make([]byte, assets.ScreenW*assets.ScreenH)}
	}

	// 場地：先畫邊框再蓋圖塊——**順序照原版**（`0x225c8` 的邊框迴圈在
	// 地形之前）。反過來畫的話下緣那兩列白線會壓在地形上。
	//
	// ⚠ **格子以外的地方不塗黑**：奇數欄往下錯開 16，上下各留一塊
	// 沒有格子的空隙，而原版那裡露出來的是**底紋**不是黑色
	// （基準畫面上 x ≥ 106、y 36–49 那一塊是黃青棋盤）。
	if ab.top != nil {
		im.Blit(ab.top, 0, 0)
	}
	if ab.bottom != nil {
		im.Blit(ab.bottom, 0, assets.MapBorderBottomY)
	}
	// 下方花邊上那一行年月是**文字層**（`drawBattleDate`，`docs/spec/011`），
	// 不在這裡——這一層只拼圖塊。
	im.FieldEdges()
	im.BlitField(ab.tiles, info.Field)
	ab.drawUnits(im, b, v)

	// 左欄與三個面板：先塗底色再畫下凹的外框。
	im.LeftColumn()
	for i, x := range assets.BattlePanelX {
		paper := byte(assets.BattlePanelPaper)
		if i == 2 {
			paper = assets.BattleOrderPaper
		}
		im.FillRect(x, assets.BattlePanelY, assets.BattlePanelW, assets.BattlePanelH, paper)
		im.BevelBox(x, assets.BattlePanelY,
			x+assets.BattlePanelW-1, assets.BattlePanelY+assets.BattlePanelH-1)
	}

	// 天氣圖示與肖像。
	if w := ab.weather[b.Weather.OriginalIndex()%len(ab.weather)]; w != nil {
		im.Blit(w, assets.BattleWeatherX, assets.BattleWeatherY)
	}
	for i := range assets.BattleFrameX {
		if ab.frame[0] != nil {
			im.Blit(ab.frame[0], assets.BattleFrameX[i], assets.BattlePanelY)
		}
		face := ab.face(info.Portrait[i])
		if face == nil {
			continue
		}
		if assets.BattleFaceMirror[i] {
			face = face.Mirror()
		}
		im.Blit(face, assets.BattleFaceX[i], assets.BattleFaceY)
	}
	return im
}

// drawUnits 把部隊的旗與兵力牌畫上去。
func (ab *ArtBattle) drawUnits(im *assets.Image, b *battle.Battle, v BattleView) {
	for _, u := range b.Units {
		if u == nil || !u.Alive() {
			continue
		}
		army, form := u.Side.OriginalIndex(), u.Formation.OriginalIndex()
		if army < 0 || form < 0 || ab.flags[army][form] == nil {
			continue
		}
		flag := ab.flags[army][form]
		paper := byte(0)
		if u == v.Acting {
			flag, paper = flag.Complement(), 0x0F
		}
		col, row := battle.ToOffset(u.At)
		x, y := assets.FlagCell(col, row)
		im.Blit(flag, x, y)
		im.FillRect(x, y+assets.FlagPlateOffsetY,
			assets.FlagPlateW, assets.FlagPlateH, paper)
	}
}

// drawPlates 把兵力牌上的數字寫上去。牌子本身在 `drawUnits` 就填好了。
func (ab *ArtBattle) drawPlates(c *Canvas, b *battle.Battle, v BattleView) {
	for _, u := range b.Units {
		if u == nil || !u.Alive() {
			continue
		}
		army := u.Side.OriginalIndex()
		form := u.Formation.OriginalIndex()
		if army < 0 || form < 0 {
			continue
		}
		ink := assets.FlagPlateColour[army]
		if u == v.Acting {
			ink ^= 0x0F
		}
		col, row := battle.ToOffset(u.At)
		x, y := assets.FlagCell(col, row)
		c.DrawTextPx(x, y+assets.FlagPlateOffsetY,
			assets.FlagPlateText(u.Soldiers()), assets.EGAPalette[ink])
	}
}

// drawText 把字疊上去。**分成兩趟**：圖層都是索引色，字是 RGBA，
// 混在一起畫會讓「哪一層蓋哪一層」變得不好講。
func (ab *ArtBattle) drawText(c *Canvas, b *battle.Battle, v BattleView, info ArtBattleInfo) {
	ab.drawPlates(c, b, v)
	drawBattleDate(c, info.Date, info.Calendar)

	// 左欄：郡名一個字一列（原版 32×32，這裡是 16×16）、州名、日數、天氣。
	ink := assets.EGAPalette[14]
	for i, r := range []rune(info.Prefecture) {
		if i >= 2 {
			break
		}
		c.DrawTextPx(assets.BattleNameX, assets.BattleNameY+i*assets.BattleNameStep,
			string(r), ink)
	}
	c.DrawTextPx(assets.BattleNameX, assets.BattleProvinceY, info.Province,
		assets.EGAPalette[15])
	c.DrawTextPx(assets.BattleNameX, assets.BattleNumberY, fmt.Sprintf("%2d", info.ID),
		assets.EGAPalette[13])
	y0, _ := assets.BattleLeftBox(2)
	c.DrawTextPx(assets.BattleNameX, y0, WeatherName(b.Weather), assets.EGAPalette[15])
	y0, _ = assets.BattleLeftBox(3)
	c.DrawTextPx(assets.BattleNameX, y0, tf("bat.dayShort", b.Day), assets.EGAPalette[15])

	// 兩個軍力面板。
	for i, side := range artBattleSides {
		x := assets.BattlePanelX[i] + assets.BattleFrameW + 4
		if i == 1 {
			x = assets.BattlePanelX[i] + 4
		}
		units, men := 0, 0
		leaders := 0
		for _, u := range b.Units {
			if u.Side == side && u.Alive() {
				units++
				men += u.Soldiers()
				leaders += len(u.Leaders)
			}
		}
		lines := []string{
			tf("bat.armyOf", info.Commander[i]),
			SideName(side),
			tf("bat.forcesLine", units, leaders),
			tf("bat.menLine", men),
			tf("bat.goldLine", b.Gold[side]),
		}
		ink := assets.EGAPalette[assets.BattlePanelInk]
		if rows, small := sidePanelLayout(c, lines); small {
			for k, s := range rows {
				c.DrawSmallTextPx(x, assets.BattlePanelY+4+k*SmallH, s, ink)
			}
		} else {
			for k, s := range rows {
				c.DrawTextPx(x, assets.BattlePanelY+4+k*CellH, s, ink)
			}
		}
	}

	// 指令面板：原版的三行三列 ＋ 提示。**選了用計、交戰、方向或紮營
	// 之後，那三行換成該選的選項**——先前只畫選單標題，六種計謀、交戰
	// 方式都看不到，玩家只能照手冊背編號（`docs/spec/014` §7）。
	ordX := assets.BattlePanelX[2] + 4
	ord := assets.EGAPalette[assets.BattleOrderInk]
	w := (assets.BattlePanelW - 8) / CellW
	opts := BattleCommandLines()
	if len(v.Items) > 0 {
		opts = v.Items
	}
	small := battleSmallLayout(c, opts, v.Menu, v.Prompt)
	switch {
	case len(opts) <= battleOptRows:
		for k, s := range opts {
			c.DrawTextPx(ordX, assets.BattlePanelY+4+k*CellH, cells.Truncate(s, w), ord)
		}
	case small != nil:
		// 英文九個指令原尺寸要五行以上：**整塊改用小字**留在面板裡
		//（使用者裁定「文字允許縮小」，`docs/spec/014` §7）——選項、
		// 標題、提示都用小字，一行 28 字、面板放得下 8 行。
		cols := (assets.BattlePanelW - 8) / SmallW
		for k, s := range small {
			ink := ord
			if k == len(small)-1 && v.Prompt != "" {
				ink = assets.EGAPalette[15]
			}
			c.DrawSmallTextPx(ordX, assets.BattlePanelY+4+k*SmallH, cells.Truncate(s, cols), ink)
		}
		if len(v.Page) > 0 {
			drawOverlay(c, battlePageX0, battlePageY0, battlePageX1, battlePageY1,
				v.PageTitle, v.Page, t("hint.page"), v.PageTop)
		}
		return
	default:
		// 譯文排不進三行（英文九個指令要五行以上）：在面板**正上方**畫
		// 一個同寬的選單框往上長，戰場的其餘部分照樣看得到——選指令的
		// 時候玩家要看得到戰場，所以不能像主畫面那樣整片蓋掉。
		x0, x1 := assets.BattlePanelX[2], assets.BattlePanelX[2]+assets.BattlePanelW
		y1 := assets.BattlePanelY - 2
		y0 := y1 - len(opts)*CellH - 8
		c.FillRect(x0, y0, x1, y1, artInkPageBG)
		for k, s := range opts {
			c.DrawTextPx(ordX, y0+4+k*CellH, cells.Truncate(s, w), ord)
		}
	}
	row := battleOptRows
	if v.Menu != "" {
		c.DrawTextPx(ordX, assets.BattlePanelY+4+row*CellH,
			cells.Truncate(v.Menu, (assets.BattlePanelW-8)/CellW), ord)
		row++
	}
	if v.Prompt != "" {
		c.DrawTextPx(ordX, assets.BattlePanelY+4+row*CellH,
			cells.Truncate(v.Prompt, (assets.BattlePanelW-8)/CellW),
			assets.EGAPalette[15])
	}
	// 查看部隊那一頁蓋在戰場區上（面板上面那一整塊）。
	if len(v.Page) > 0 {
		drawOverlay(c, battlePageX0, battlePageY0, battlePageX1, battlePageY1,
			v.PageTitle, v.Page, t("hint.page"), v.PageTop)
	}
}

// sideTextW 是軍力面板上文字區的寬：面板 176 扣掉肖像框 80 與兩邊的
// 間隙，92 像素。先前照整塊面板截在 21 格，英文的「Main Attackers」
// 就畫進隔壁那一塊面板——中文剛好都短，才沒露出來。
const sideTextW = assets.BattlePanelW - assets.BattleFrameW - 4

// sidePanelLayout 決定軍力面板的五行怎麼畫：原尺寸放得下（11 格）就照畫；
// 放不下而且都是 ASCII 就整塊改小字，長的一行折成兩行（一行 15 字、
// 面板放得下 8 行）；都不行才截。回傳要畫的行與用不用小字。
func sidePanelLayout(c *Canvas, lines []string) ([]string, bool) {
	cols := sideTextW / CellW
	fit := true
	for _, l := range lines {
		if cells.Width(l) > cols {
			fit = false
		}
	}
	if fit {
		return lines, false
	}
	scols, srows := sideTextW/SmallW, (assets.BattlePanelH-8)/SmallH
	var rows []string
	for _, l := range lines {
		rows = append(rows, cells.Wrap(l, scols)...)
	}
	if len(rows) <= srows && c.FitsSmall(strings.Join(rows, "")) {
		return rows, true
	}
	out := make([]string, len(lines))
	for i, l := range lines {
		out[i] = cells.Truncate(l, cols)
	}
	return out, false
}

// battleSmallLayout 回傳用小字排的整塊指令面板（選項、標題、提示），
// 放不下或有小字級沒有的字（中日文）就回 nil。
//
// 選項是先照原尺寸排好的行（`BattleCommandLines` 等），這裡拆回一項
// 一項再照小字的寬度重排：每一項都以「編號.」開頭，名字裡不會有這個樣子。
func battleSmallLayout(c *Canvas, opts []string, menu, prompt string) []string {
	cols := (assets.BattlePanelW - 8) / SmallW
	rows := (assets.BattlePanelH - 8) / SmallH
	var items []string
	for _, line := range opts {
		items = append(items, splitBattleItems(line)...)
	}
	lines := packBattleCols(items, cols)
	if menu != "" {
		lines = append(lines, menu)
	}
	if prompt != "" {
		lines = append(lines, prompt)
	}
	if len(lines) > rows {
		return nil
	}
	for _, l := range lines {
		if !c.FitsSmall(l) {
			return nil
		}
	}
	return lines
}

// splitBattleItems 把一行選項拆回一項一項（在「空白＋數字＋.」前面切）。
func splitBattleItems(line string) []string {
	var out []string
	start := 0
	rs := []rune(line)
	for i := 1; i+2 < len(rs); i++ {
		if rs[i] == ' ' && rs[i+1] >= '0' && rs[i+1] <= '9' && rs[i+2] == '.' {
			out = append(out, strings.TrimSpace(string(rs[start:i])))
			start = i + 1
		}
	}
	return append(out, strings.TrimSpace(string(rs[start:])))
}

// 戰場畫面的選項列數與分頁的範圍（`docs/spec/014` §7）。
const (
	// battleOptRows 是指令面板上給選項的列數：三列選項 ＋ 標題 ＋ 提示，
	// 正好是 96 像素高的面板放得下的五列。
	battleOptRows = 3

	// 分頁蓋在三個面板上方那一整塊：左右對齊面板的外緣（64–624），
	// 上緣留 4 像素，下緣停在面板上面。
	battlePageX0, battlePageY0 = 64, 4
	battlePageX1, battlePageY1 = 624, assets.BattlePanelY - 4
)

// face 取一張肖像；沒有就回 nil。
func (ab *ArtBattle) face(n int) *assets.Image {
	if n < 0 || ab.faces == nil {
		return nil
	}
	i, ok := ab.faces.ByName(fmt.Sprintf("F%03d.FAC", n))
	if !ok {
		return nil
	}
	im, err := assets.DecodeImage(ab.faces.Data(i))
	if err != nil {
		return nil
	}
	return im
}

// drawBattleDate 畫下方花邊上那一行年月（`docs/spec/011`）。
//
// 十個格子的版面（量在原版基準畫面「建安二年九月秋」上）：
//
//	格 0–1  年號　　格 2–3  年數（靠右）　格 4  空
//	格 5    「年」　格 6–7  月份（靠右）　格 8  「月」　格 9  季節
//
// 字用**兩色棋盤**畫（`assets.BattleDateInk*`）。
func drawBattleDate(c *Canvas, d game.Date, cal game.Calendar) {
	cell := battleDateCells(d, cal)
	odd := assets.EGAPalette[assets.BattleDateInkOdd]
	even := assets.EGAPalette[assets.BattleDateInkEven]
	for i, s := range cell {
		if s == "" {
			continue
		}
		for _, r := range s {
			c.DrawRuneBoxDitherPx(
				assets.BattleDateX+i*assets.BattleDateStep, assets.BattleDateY,
				assets.BattleDateW, assets.BattleDateH, r, odd, even)
			break // 一格一個字
		}
	}
}

// battleDateCells 把一個年月排進十個格子。
//
// ⚠ **版面只有一個樣本**（「建安二年九月秋」，`docs/spec/011` §2）。
// 靠右對齊是從那一個樣本推的：個位數的「二」落在格 3 而不是格 2。
// **年數超過兩個中文字時怎麼排沒有樣本**——這裡退回半形數字，
// 因為「二十一」取最後兩個字會變成「十一」，那是一個讀得通而錯的年份。
func battleDateCells(d game.Date, cal game.Calendar) [assets.BattleDateCells]string {
	var out [assets.BattleDateCells]string
	put := func(lo int, s string) {
		rs := []rune(s)
		// 靠右：兩格放不下就從右邊開始塞，塞得下幾個算幾個。
		for i := len(rs) - 1; i >= 0 && lo+1-(len(rs)-1-i) >= lo; i-- {
			out[lo+1-(len(rs)-1-i)] = string(rs[i])
		}
	}
	name, nth, ok := game.EraOf(d.Year)
	if cal == game.Western || !ok {
		// 西曆那條路**沒有樣本**：原版切西曆時這一行寫什麼沒量過。
		// 這裡照同一個版面排阿拉伯數字，年放四格。
		y := fmt.Sprintf("%d", d.Year)
		for i, r := range y {
			if i < 4 {
				out[i] = string(r)
			}
		}
		out[5] = t("date.yearChar")
		put(6, fmt.Sprintf("%d", d.Month))
		out[8] = t("date.monthChar")
		out[9] = d.Season().Name()
		return out
	}
	for i, r := range []rune(game.EraName(name)) {
		if i < 2 {
			out[i] = string(r)
		}
	}
	put(2, battleNumeral(nth))
	out[5] = t("date.yearChar")
	put(6, battleNumeral(d.Month))
	out[8] = t("date.monthChar")
	out[9] = d.Season().Name()
	return out
}

// battleNumeral 是年數／月份在那一行的寫法。
//
// 第一年與正月寫「元」（與主畫面直排同一個規則，`game.Date.Format`）。
// **超過兩個中文字就退回半形數字**——那一格只有兩個位置，見
// `battleDateCells` 的 ⚠。
func battleNumeral(n int) string {
	if n == 1 {
		return t("date.first")
	}
	s := game.Chinese(n)
	if len([]rune(s)) > 2 {
		return fmt.Sprintf("%d", n)
	}
	return s
}
