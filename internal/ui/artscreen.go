package ui

import (
	"fmt"
	"image"
	"image/color"
	"image/draw"

	"github.com/wicanr2/softworld_san1_remake/internal/cells"

	"github.com/wicanr2/softworld_san1_remake/internal/assets"
	"github.com/wicanr2/softworld_san1_remake/internal/battle"
	"github.com/wicanr2/softworld_san1_remake/internal/game"
	"github.com/wicanr2/softworld_san1_remake/internal/state"
)

// 接上原版素材的遊戲主畫面。
//
// 底圖是原版 `DATA3` 的七張 `MAINMAP*`（`internal/assets` 的 `MainScreen`，
// 位置讀自 `0x11ebc`–`0x11fc5`）。**州郡依所屬換色**是從州郡座標
// （記錄 offset 6–9 加上 `0x50`／`0x2c`）灌下去的區域填色。
// 上面再疊 remake 自己畫的文字。
//
// ⚠ **原版檔案不隨本專案散布**：底圖來自玩家自己那一份，
// 沒有 `-root` 就畫不出這一張（`cmd/san1dump -screen art`）。
//
// 這一張與 `DrawSession` 的差別只有「底圖」與「文字放哪裡」；
// 規則層的資料是同一份。

// ArtScreen 是一張接上原版底圖的主畫面。
type ArtScreen struct {
	// base 是拼好的底圖（未上色），每回合從它複製一份再填色。
	base  *assets.Image
	faces *assets.Container

	// fills 是州郡的填色圖樣（`EGAFILL.PAL`）；沒有原版的 `DATA1` 就是
	// nil，那時退回 `artFactionColour`。
	fills *[16]assets.FillPattern

	// panels 是右側兩塊面板的外框拼件（`SIDEB`／`SIDED`），frame 是
	// 肖像框（`FBRD` 四塊）。havePanel 為假時兩者都不畫——沒有 `DATA1`
	// 的人看到的仍是底圖，不是一塊空白。
	panels    [2]assets.SideFrame
	frame     [4]*assets.Image
	havePanel bool
}

// NewArtScreen 拼出主畫面的底圖，順便留著容器好取肖像。
//
// data1 給的是州郡的填色圖樣（`EGAFILL.PAL`）；可以是 nil，那就退回
// remake 自己的色號。
func NewArtScreen(data3, data1 *assets.Container) (*ArtScreen, error) {
	bg, err := assets.MainScreen(data3)
	if err != nil {
		return nil, err
	}
	a := &ArtScreen{base: bg, faces: data3}
	if data1 != nil {
		if f, err := assets.FillPatterns(data1); err == nil {
			a.fills = &f
		}
		ok := true
		for i, p := range assets.MainPanels() {
			f, err := assets.LoadSideFrame(data1, p.Letter)
			if err != nil {
				ok = false
				break
			}
			a.panels[i] = f
		}
		if fr, err := assets.PortraitFrame(data1, 'D'); err == nil && ok {
			a.frame = fr
			a.havePanel = true
		}
	}
	return a, nil
}

// Portrait 取一張肖像；沒有就回 nil。
func (a *ArtScreen) Portrait(n int) *assets.Image {
	i, ok := a.faces.ByName(fmt.Sprintf("F%03d.FAC", n))
	if !ok {
		return nil
	}
	im, err := assets.DecodeImage(a.faces.Data(i))
	if err != nil {
		return nil
	}
	return im
}
// 原版畫面上的位置（像素），量自 `workplace/shots/orig-main.png`
// （`docs/spec/005` §6.2）。**版面是按像素排的不是按格**，
// 而且幾列之間留了空行，所以每一列的 y 都是逐列量的不是等距算的。
const (
	artPanelX = 408 // 右面板左緣
	artPanelW = 224
	artMapX   = 72 // 地圖區左緣（左邊那 72 像素是花邊直條）

	// 郡名是 32×32 的雙倍字；州名與編號疊在它右邊，君主與人望再右邊。
	artNameX = 424
	artNameY = 52
	artProvX = 488
	artProvY = 52
	artIDY   = 68
	artLordX = 536
	artLordY = 52
	artFameY = 68

	// 面板上的資料排成**兩欄**：左欄標籤從 424 起、數值靠右對齊到 520；
	// 右欄從 536 起、數值靠右對齊到 616。
	artAutoX  = 432
	artAutoY  = 100
	artFieldX = 424
	artValueR = 520
	artRightX = 536
	artRightR = 616

	// artPortraitX／Y 是肖像，artFrameX／Y 是它的框（`FBRD`，四塊）。
	// **量出來的**：`F228.FAC` 在 (536,116) 逐像素 100% 相符
	// （`cmd/san1imgcheck`），四塊框各自也 100%。
	artPortraitX = 536
	artPortraitY = 116
	artFrameX    = 528
	artFrameY    = 108

	// 下面板兩行提示。
	artMsgX  = 424
	artMsgY  = 300
	artMsgDY = 16

	// artDateCol 是左側直條上年月的位置（原版直排在那裡）。
	artDateCol = 1
	artDateRow = 4
)

// 面板上的字色，量自原版畫面。**每一格不同，是量的不是挑的**：
// 郡名黃、州名與編號洋紅、君主白、人望黃、金米黃、欄位淺青、
// 主事者姓名淺綠、在野武將白。
var (
	artInkName  = color.RGBA{0xFF, 0xFF, 0x55, 0xFF} // 14
	artInkProv  = color.RGBA{0xFF, 0x55, 0xFF, 0xFF} // 13
	artInkLord  = color.RGBA{0xFF, 0xFF, 0xFF, 0xFF} // 15
	artInkFame  = color.RGBA{0xFF, 0xFF, 0x55, 0xFF} // 14
	artInkAuto  = color.RGBA{0xFF, 0xFF, 0xFF, 0xFF} // 15
	artInkField = color.RGBA{0x55, 0xFF, 0xFF, 0xFF} // 11
	artInkGold  = color.RGBA{0xFF, 0xFF, 0x55, 0xFF} // 14
	artInkGov   = color.RGBA{0x55, 0xFF, 0x55, 0xFF} // 10
	artInkMsg   = color.RGBA{0xFF, 0xFF, 0x55, 0xFF} // 14
)


// artFactionColour 是**沒有原版素材時**的退路：一組看得出區別的色號。
//
// 原版的填色走 `EGAFILL.PAL`（十六個勢力各一塊 8×8 的圖樣，
// `assets.FillPatterns`），讀得到就用那一份。無主的郡不填，留底圖的白色。
var artFactionColour = [...]byte{
	9, 12, 10, 14, 13, 11, 6, 2, 1, 5, 4, 3, 8, 7, 9, 12,
}

// Compose 依目前局面畫一張 640×350 的圖。
func (a *ArtScreen) Compose(g *game.State, sel int) *assets.Image {
	im := a.base.Clone()
	for _, p := range g.Prefectures() {
		if p.Owner == state.NoFaction {
			continue
		}
		x := int(p.MapX) + assets.MapOriginX
		y := int(p.MapY) + assets.MapOriginY
		if a.fills != nil {
			im.FloodFillPattern(x, y, &a.fills[int(p.Owner)%len(a.fills)])
			continue
		}
		im.FloodFill(x, y, artFactionColour[int(p.Owner)%len(artFactionColour)])
	}
	return im
}

// DrawArtSession 把接上原版素材的主畫面畫到畫布上。
//
// 畫布要正好 640×350（`assets.ScreenW`／`ScreenH`）——原版的版面是
// 按像素排的，格對不齊時字會壓到花邊上。
func DrawArtSession(c *Canvas, a *ArtScreen, g *game.State, log []string, v View) {
	sel := v.Sel
	if sel <= 0 {
		if ter := g.Territory(g.Player); len(ter) > 0 {
			sel = ter[0]
		} else {
			sel = 1
		}
	}
	p := g.Prefecture(sel)
	if p == nil {
		return
	}

	im := a.Compose(g, sel)
	// 右側兩塊面板：先拼外框與底色，再把肖像與它的框疊上去。
	//
	// **底圖的 `MAINMAPB`／`MAINMAPC` 會被整片蓋掉**——那兩張是黃色與
	// 灰色的雜訊底，原版畫主畫面時同樣覆蓋掉它們。
	if a.havePanel {
		for i, pn := range assets.MainPanels() {
			im.DrawPanel(pn, a.panels[i])
		}
	}
	if who := g.Governor(sel); who != nil {
		if face := a.Portrait(int(who.Portrait)); face != nil {
			im.Blit(face, artPortraitX, artPortraitY)
		}
	}
	if a.havePanel {
		im.Blit(a.frame[0], artFrameX, artFrameY)       // 上
		im.Blit(a.frame[1], artFrameX, artFrameY+88)    // 下
		im.Blit(a.frame[2], artFrameX, artPortraitY)    // 左
		im.Blit(a.frame[3], artFrameX+72, artPortraitY) // 右
	}
	draw.Draw(c.Img, image.Rect(0, 0, assets.ScreenW, assets.ScreenH),
		im.RGBA(), image.Point{}, draw.Src)

	// 年月直排在左側直條上，與原版一樣。
	for i, r := range []rune(g.Date.Format(v.Calendar)) {
		c.DrawText(artDateCol, artDateRow+i, string(r),
			color.RGBA{0x00, 0x00, 0x00, 0xFF})
	}

	// 第一列：郡名是 32×32 的雙倍字，州名與編號在它右邊。
	x := artNameX
	for _, r := range p.Name {
		x += c.DrawRuneScaledPx(x, artNameY, r, artInkName, 2, 2)
	}
	c.DrawTextPx(artProvX, artProvY,
		state.ProvinceName(int(p.Province)), artInkProv)
	x = artProvX
	for _, r := range fmt.Sprintf("%2d", p.ID) {
		x += c.DrawRuneScaledPx(x, artIDY, r, artInkProv, 2, 1)
	}

	if lord := g.Lord(p.Owner); lord != nil {
		c.DrawTextPx(artLordX, artLordY, "君主"+lord.Name, artInkLord)
		if f := g.Faction(p.Owner); f != nil {
			c.DrawTextPx(artLordX, artFameY,
				fmt.Sprintf("人望：%3d", f.Prestige), artInkFame)
		}
	} else {
		c.DrawTextPx(artLordX, artLordY, "無　主", artInkLord)
	}
	c.DrawTextPx(artAutoX, artAutoY, game.AutonomyName(p.Autonomy), artInkAuto)

	// 欄位表：標籤靠左、數值靠右對齊。
	//
	// **原版的數值是靠右不是靠標籤排的**：位數變了整欄還是對齊，
	// 用空白填出來的版面只有在位數剛好時才一樣。
	num := func(v int) string { return fmt.Sprintf("%d", v) }
	right := func(rx, ry int, s string, fg color.RGBA) {
		c.DrawTextPx(rx-len([]rune(s))*CellW, ry, s, fg)
	}
	for _, f := range []struct {
		y     int
		label string
		value string
		fg    color.RGBA
	}{
		{116, "土地價值", num(int(p.LandValue)), artInkField},
		{132, "洪水率", num(int(p.FloodRate)), artInkField},
		{148, "物價", num(int(p.PriceLevel)), artInkField},
		{164, "民眾忠誠", num(int(p.PublicLoyalty)), artInkField},
		{180, "人口", num(int(p.Population)), artInkField},
		{212, "金", num(int(p.Gold)), artInkGold},
		{228, "米", num(int(p.Rice)), artInkGold},
		{260, "在野武將", num(g.FreeGenerals(p.ID)), artInkAuto},
	} {
		c.DrawTextPx(artFieldX, f.y, f.label, f.fg)
		right(artValueR, f.y, f.value, f.fg)
	}
	// 右欄：主事者姓名（32×32 的雙倍字）、現役將、兵士。
	if who := g.Governor(sel); who != nil {
		gx := artRightX
		for _, r := range who.Name {
			gx += c.DrawRuneScaledPx(gx, 212, r, artInkGov, 2, 2)
		}
	}
	c.DrawTextPx(artRightX, 244, "現役將", artInkField)
	right(artRightR, 244, num(len(g.Garrison(p.ID))), artInkField)
	c.DrawTextPx(artRightX, 260, "兵士", artInkField)
	right(artRightR, 260, num(g.Soldiers(p.ID)), artInkField)

	// 下面板兩行提示。**它有自己的底色與外框**，不是壓在地圖上的一條。
	msg := v.Prompt
	if msg == "" && len(log) > 0 {
		msg = log[len(log)-1]
	}
	if msg != "" {
		w := (artRightR - artMsgX) / CellW
		c.DrawTextPx(artMsgX, artMsgY, cells.Truncate(msg, w), artInkMsg)
	}
}


// TitleScreen 是主選單畫面（原版開機後的那一張）。
//
// 圖是原版的（`assets.MenuScreen`），字是 remake 自己的字庫。
// 六個項目的文字照原版的選單抄（`docs/re/02` §3 的開機畫面）。
type TitleScreen struct {
	bg *assets.Image
}

// NewTitleScreen 從 `DATA3` 拼出主選單畫面。
func NewTitleScreen(data3 *assets.Container) (*TitleScreen, error) {
	bg, err := assets.MenuScreen(data3)
	if err != nil {
		return nil, err
	}
	return &TitleScreen{bg: bg}, nil
}

// TitleItems 是六個選項的原文。
func TitleItems() [6]string {
	return [6]string{
		"1. 開始新遊戲", "2. 載入舊進度", "3. 使用楷書字",
		"4. 使用隸書字", "5. 音樂欣賞", "6. 回作業系統",
	}
}

// DrawTitle 畫主選單。sel 是反白的項目（0–5，負數表示沒有）。
//
// 六個項目的顏色照原版是同一個黃（原版沒有選取記號，靠按數字鍵選）。
// **反白是 remake 自己加的**：remake 支援上下鍵移動，沒有記號就看不出
// 停在哪一項，所以選到的那一項改畫白色。
func DrawTitle(c *Canvas, ts *TitleScreen, sel int) {
	draw.Draw(c.Img, image.Rect(0, 0, assets.ScreenW, assets.ScreenH),
		ts.bg.RGBA(), image.Point{}, draw.Src)
	label := color.RGBA{0x55, 0xFF, 0xFF, 0xFF}
	ink := color.RGBA{0xFF, 0xFF, 0x55, 0xFF}
	hot := color.RGBA{0xFF, 0xFF, 0xFF, 0xFF}
	DrawMenuLabel(c, "主選擇單", label)
	for i, s := range TitleItems() {
		b := assets.MenuButtons()[i]
		col := ink
		if i == sel {
			col = hot
		}
		DrawMenuItem(c, b[0], b[1], s, col)
	}
}

// DrawMenuLabel 把左側直牌上的字畫上去（由上往下一個字一列）。
func DrawMenuLabel(c *Canvas, s string, fg color.RGBA) {
	y := assets.MenuLabelY
	for _, r := range s {
		c.DrawRuneWidePx(assets.MenuLabelX, y, r, fg, assets.MenuLabelScaleX)
		y += assets.MenuLabelPitch
	}
}

// DrawMenuItem 照原版的版面把一行字寫在按鈕上，(bx, by) 是按鈕左上角。
//
// 原版**每個全形字前面都有一個空的半形格**，所以中文的字距是 24 不是 16
// （`assets.MenuTextCJKPitch`）。少了那個空格，五個字只佔 80 像素，
// 擠在 200 寬的按鈕左半邊，右半邊空著。
//
// 字串裡本來就有的空白算數（「1. 開始新遊戲」在編號後面已經有一個），
// **不再補第二個**——補了整串中文會往右挪 8 像素。
func DrawMenuItem(c *Canvas, bx, by int, s string, fg color.RGBA) {
	x, y := bx+assets.MenuTextX, by+assets.MenuTextY
	blank := true
	for _, r := range s {
		w := cells.RuneWidth(r)
		if w == 0 {
			continue
		}
		if w > 1 && !blank {
			x += CellW
		}
		c.DrawRuneWidePx(x, y, r, fg, 1)
		x += w * CellW
		blank = r == ' '
	}
}

// ArtBattle 是接上原版素材的主戰場：地形圖塊來自 `DATA1` 的
// `EICON.GRP`，上方花邊與遊戲主畫面共用 `DATA3` 的 `MAINMAP1`
// （`0x2246a` 載入、`0x2247c` 畫在 (0,0)），位置與原版相同
// （`docs/spec/005` §8）。
type ArtBattle struct {
	tiles   []*assets.Image
	flags   [4][6]*assets.Image
	top     *assets.Image
	bg      *assets.Image
	frame   [4]*assets.Image
	weather [3]*assets.Image
	faces   *assets.Container
}

// NewArtBattle 解出三十六張地形圖塊與上方花邊。data3 可以是 nil，
// 那時就不畫花邊。
func NewArtBattle(data1, data3 *assets.Container) (*ArtBattle, error) {
	tiles, err := assets.BattleTiles(data1)
	if err != nil {
		return nil, err
	}
	flags, err := assets.UnitFlags(data1)
	if err != nil {
		return nil, err
	}
	ab := &ArtBattle{tiles: tiles, flags: flags}
	if ab.bg, err = assets.BattleBackground(data1); err != nil {
		return nil, err
	}
	if ab.frame, err = assets.PortraitFrame(data1, 'C'); err != nil {
		return nil, err
	}
	for i := range ab.weather {
		if j, ok := data1.ByName(fmt.Sprintf("WEATHER%d.IMG", i)); ok {
			if im, err := assets.DecodeImage(data1.Data(j)); err == nil {
				ab.weather[i] = im
			}
		}
	}
	if data3 != nil {
		ab.faces = data3
		if i, ok := data3.ByName("MAINMAP1.IMG"); ok {
			if im, err := assets.DecodeImage(data3.Data(i)); err == nil {
				ab.top = im
			}
		}
	}
	return ab, nil
}

// DrawArtField 畫一個郡的戰場：地形與部隊都接原版素材。
//
// field 是州郡記錄 offset 55–174 那 120 個位元組。units 可以是空的，
// 那就只有地形。highlight 是輪到下令的那一支——原版讓它閃，亮的那半
// 是**整面旗取補數**（`assets.Image.Complement`）。
//
// 兵力牌的數字用 remake 自己的字庫畫，位置與字色照原版
// （旗下方 15 像素、`DS:0x796a` 的四個色）。
func DrawArtField(c *Canvas, ab *ArtBattle, name string, field []byte,
	units []*battle.Unit, highlight *battle.Unit) {
	im := assets.BattleField(ab.tiles, field, 0)
	if ab.top != nil {
		im.Blit(ab.top, 0, 0)
	}
	type plate struct {
		x, y int
		text string
		col  byte
	}
	var plates []plate
	for _, u := range units {
		if u == nil || !u.Alive() {
			continue
		}
		army, form := u.Side.OriginalIndex(), u.Formation.OriginalIndex()
		if army < 0 || form < 0 {
			continue
		}
		flag := ab.flags[army][form]
		if flag == nil {
			continue
		}
		ink, paper := assets.FlagPlateColour[army], byte(0)
		if u == highlight {
			flag = flag.Complement()
			ink, paper = ink^0x0F, paper^0x0F
		}
		col, row := battle.ToOffset(u.At)
		x, y := assets.FlagCell(col, row)
		im.Blit(flag, x, y)
		py := y + assets.FlagPlateOffsetY
		im.FillRect(x, py, assets.FlagPlateW, assets.FlagPlateH, paper)
		plates = append(plates, plate{x, py,
			assets.FlagPlateText(u.Soldiers()), ink})
	}
	draw.Draw(c.Img, image.Rect(0, 0, assets.ScreenW, assets.ScreenH),
		im.RGBA(), image.Point{}, draw.Src)
	for _, p := range plates {
		c.DrawTextPx(p.x, p.y, p.text, assets.EGAPalette[p.col])
	}
	c.DrawText(1, 20, name, color.RGBA{0xFF, 0xFF, 0x55, 0xFF})
}

// DrawTitleList 在主選單上疊一張清單（選劇本、選君主、選進度、選曲）。
//
// 原版那幾層跑在開機鏈的第二層（`DATA0.GRP`），主程式的碼段 dump
// 涵蓋不到，所以**版面是 remake 自己排的**：置中的框、一行一項。
func DrawTitleList(c *Canvas, ts *TitleScreen, title string, items []string, sel int) {
	DrawTitle(c, ts, -1)
	// 蓋在六個按鈕那一片上，標題牌留著看得見。
	const col, row, cols = 18, 9, 44
	// 放得下幾項就顯示幾項，其餘捲動——**選君主可以有十六個**，
	// 難度上限也有二十，寫死幾行遲早會有一項被切掉而沒人發現。
	//
	// ⚠ 用的是**這張畫布的列數**（`c.Rows`）不是套件常數 `Rows`：
	// 後者是文字版面的 25 列，接原版素材的畫布只有 21 列，
	// 拿錯的話最底下幾項會被畫到畫面外——而畫面外沒有紅字。
	view := c.Rows - row - 5
	if view < 1 {
		view = 1
	}
	first := 0
	if sel >= view {
		first = sel - view + 1
	}
	if first > len(items)-view {
		first = len(items) - view
	}
	if first < 0 {
		first = 0
	}
	shown := items[first:]
	if len(shown) > view {
		shown = shown[:view]
	}
	rows := len(shown) + 4
	c.FillRect(col*CellW, row*CellH, (col+cols)*CellW, (row+rows)*CellH,
		color.RGBA{0x00, 0x00, 0x2A, 0xFF})
	c.DrawBox(col, row, cols, rows, ColFrame)
	head := title
	if len(items) > view {
		head = fmt.Sprintf("%s（%d／%d）", title, sel+1, len(items))
	}
	c.DrawText(col+2, row+1, cells.Truncate(head, cols-4), ColSel)
	for i, s := range shown {
		ink := ColFG
		if first+i == sel {
			ink = ColSel
		}
		c.DrawText(col+2, row+3+i, cells.Truncate(s, cols-4), ink)
	}
}
