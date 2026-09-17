package ui

import (
	"fmt"
	"image"
	"image/color"
	"image/draw"
	"strings"

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

	// cardPanel／cardFrame 是人物資料卡用的那兩組（`SIDEC`／`FBRC`）：
	// 原版拼框的 `0x1058:0x262c` 用 `(樣式＋500) mod 5` 挑 `SIDEA`–`SIDEE`，
	// 卡片傳 7 → `C`；肖像框的 `0x276c` 傳 2 → `FBRC`（`docs/spec/005` §9.2）。
	cardPanel assets.SideFrame
	cardFrame [4]*assets.Image

	// pickBox 是選君主那一格下方提示框的拼件（`SIDEA`）；multiBox 是多選清單的外框
	// （`0x18286` 傳樣式 9 → `SIDEE`）。
	pickBox, multiBox assets.SideFrame
	// prefBox 是挑郡清單的外框（`0x1d4ec` 傳樣式 8 → `SIDED`）。
	prefBox assets.SideFrame

	// scenes 是 `SCG30`／`SCG31` 所在的容器（`DATA1`）；`SCG01`–`29` 與肖像
	// 同在 `DATA3`（`docs/formats/04`）。
	scenes *assets.Container

	// cursor 是主畫面的輸入游標（`CURB`，`docs/spec/014` §4.1）；setupCursor 是
	// 開新局設定那幾問的（`CURD`，`0x11b90`）。沒有 `DATA1` 就是 nil。
	cursor, setupCursor *[assets.MenuOrnamentFrameCount]assets.CursorFrame
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
	a := &ArtScreen{base: bg, faces: data3, scenes: data1}
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
		if f, err := assets.LoadSideFrame(data1, 'C'); err != nil {
			a.havePanel = false
		} else if fr, err := assets.PortraitFrame(data1, 'C'); err != nil {
			a.havePanel = false
		} else {
			a.cardPanel, a.cardFrame = f, fr
		}
		if f, err := assets.LoadSideFrame(data1, 'A'); err != nil {
			a.havePanel = false
		} else {
			a.pickBox = f
		}
		if f, err := assets.LoadSideFrame(data1, 'E'); err != nil {
			a.havePanel = false
		} else {
			a.multiBox = f
		}
		if f, err := assets.LoadSideFrame(data1, 'D'); err != nil {
			a.havePanel = false
		} else {
			a.prefBox = f
		}
		if f, err := assets.CursorFrames(data1, assets.CursorMain); err == nil {
			a.cursor = &f
		}
		if f, err := assets.CursorFrames(data1, assets.CursorSetup); err == nil {
			a.setupCursor = &f
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

// Scene 取一張場景圖 `SCG%02d.IMG`（176×96，`docs/formats/07` §3）；
// 1–29 在 `DATA3`、30–31 在 `DATA1`。沒有就回 nil。
func (a *ArtScreen) Scene(n int) *assets.Image {
	name := fmt.Sprintf("SCG%02d.IMG", n)
	for _, c := range []*assets.Container{a.faces, a.scenes} {
		if c == nil {
			continue
		}
		i, ok := c.ByName(name)
		if !ok {
			continue
		}
		im, err := assets.DecodeImage(c.Data(i))
		if err != nil {
			return nil
		}
		return im
	}
	return nil
}

// 原版畫面上的位置（像素），量自 `workplace/shots/orig-main.png`
// （`docs/spec/005` §6.2）。**版面是按像素排的不是按格**，
// 而且幾列之間留了空行，所以每一列的 y 都是逐列量的不是等距算的。
const (
	// 面板本身的位置與拼件在 `assets.MainPanels()`；這裡只留文字要用的。
	artMapX = 72 // 地圖區左緣（左邊那 72 像素是花邊直條）

	// 郡名是 32×32 的雙倍字；州名與編號疊在它右邊，君主與人望再右邊。
	artNameX  = 424
	artNameY  = 52
	artProvX  = 488
	artProvY  = 52
	artIDY    = 68
	artLordX  = 536
	artLordY  = 52
	artFameY  = 68
	artChiefY = 84
	// artGovFieldX 是主事者姓名欄（6 byte、兩倍寬）的左緣。
	artGovFieldX = 520

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
	artInkChief = color.RGBA{0xFF, 0x55, 0xFF, 0xFF} // 13
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

// Compose 依目前局面畫一張 640×408 的圖。
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
// 畫布要正好 640×408（`assets.ScreenW`／`ScreenH`）——原版的版面是
// 按像素排的，格對不齊時字會壓到花邊上。
//
// 右側兩塊面板放什麼由 `View` 決定（`docs/spec/014`）：上面板是指令表、
// 郡的資料或挑選清單，下面板是子選單或提示；分頁蓋掉整個內容區。
// **每一樣都要畫出來**——這一支先前只畫提示，打開子選單與沒打開畫出來
// 差 0 個位元組，玩家在預設畫面上看不到任何選項。
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
	upper, subLower := artUpperOf(c, v)

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
	if upper == artUpperStatus {
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
	}
	draw.Draw(c.Img, image.Rect(0, 0, assets.ScreenW, assets.ScreenH),
		im.RGBA(), image.Point{}, draw.Src)

	// 年月直排在左側直條上，與原版一樣——**最後一格是季節**
	// （原版寫「建安二年八月秋」，七個字）。
	//
	//
	// 英文逐字母一列直排讀不下去、按字折行又會把「Zhongping」切斷
	//（直條內側只有 5 格寬），所以**有拉丁字母就整行轉 90° 排**，
	// 像書脊一樣由上往下讀（`docs/spec/014` §3.5）。
	drawArtDate(c, g.Date.FormatWithSeason(v.Calendar))

	// **下面板先畫**：清單寬到要蓋整個內容區時，覆蓋頁要蓋在它上面，
	// 不能讓提示字浮在覆蓋頁上。
	drawArtLower(c, log, v, subLower, a.cursor)
	switch upper {
	case artUpperStatus:
		drawArtStatus(c, g, p, sel)
	case artUpperList:
		if !drawArtList(c, v.Menu, v.Items) {
			// 標籤寬過上面板（存檔槽的描述、英文的計略名）就改蓋整個內容區。
			drawArtOverlay(c, v.Menu, commandLines(v.Items), t("hint.pick"), 0)
		}
	default:
		drawArtCommands(c)
	}
	if v.Roster != nil {
		DrawRosterPick(c, a, g, v.Roster)
	}
	if v.PrefPick != nil {
		DrawPrefPick(c, a, g, v.PrefPick)
	}
	if v.Treasury != nil {
		DrawTreasuryPanel(c, a, g, v.Treasury)
	}
	// 人物資料卡蓋掉右側整塊面板（原版畫卡之前先清 (408,36)–(631,291)）。
	if v.HasCard {
		DrawPersonCard(c, a, g, v.Card)
	}
	if v.Save != nil {
		DrawSaveScreen(c, a, v.Save)
	}
	if len(v.Page) > 0 {
		drawArtOverlay(c, v.PageTitle, v.Page, t("hint.page"), v.PageTop)
	}
}

// drawArtDate 把年月直排在左側直條上（選君主那一格也用）。
func drawArtDate(c *Canvas, date string) {
	ink := color.RGBA{0x00, 0x00, 0x00, 0xFF}
	if artHasLatin(date) {
		c.DrawTextRotatedPx(artDateCol*CellW, artDateRow*CellH, date, ink)
		return
	}
	for i, r := range []rune(date) {
		c.DrawText(artDateCol, artDateRow+i, string(r), ink)
	}
}

// artUpper 是上面板放什麼（`docs/spec/014` §3.1）。
type artUpper int

const (
	artUpperCommands artUpper = iota // 十項指令表（原版主提示時的樣子）
	artUpperStatus                   // 郡的資料（原版的「0.狀態」）
	artUpperList                     // 挑選清單或數字輸入
)

// artUpperOf 決定上面板放什麼；第二個回傳值是「類別子選單畫在下面板」。
func artUpperOf(c *Canvas, v View) (artUpper, bool) {
	if v.Menu != "" {
		if k := subMenuKey(v.Menu); k != 0 && subMenuLayout(c, k, v.Items).ok {
			// 子選單打開時上面板**維持指令表**（原版 `sub-9` 與 `sub-0`
			// 的上面板逐段相同）。
			return artUpperCommands, true
		}
		// 挑選清單、數字輸入，以及**譯文排不進下面板的子選單**
		//（英文的計略名一項就 28 格）改成上面板一行一項。
		return artUpperList, false
	}
	if v.Status {
		return artUpperStatus, false
	}
	return artUpperCommands, false
}

// lowerLayout 是子選單在下面板上的排法：用不用小字、排成哪幾行。
type lowerLayout struct {
	lines []string
	small bool
	ok    bool
}

// subMenuLayout 決定這一類的子選單怎麼放進下面板（`docs/spec/014` §3.2）：
//
//  1. 原尺寸：四行 × 24 格。中文九類全部排得進，與原版一樣。
//  2. 小字（6×10）：六行 × 32 格。**英文排不進原尺寸時用這個**，
//     留在原版的位置——使用者裁定「文字允許縮小」（2026-09-11）。
//     小字級只有 ASCII，所以只有整串都是 ASCII 才走這一條。
//  3. 都不行（日文的長名字）：回 ok＝false，改畫在上面板一行一項。
func subMenuLayout(c *Canvas, k byte, items []Command) lowerLayout {
	fits := func(lines []string, cols, rows int) bool {
		if len(lines) > rows {
			return false
		}
		for _, l := range lines {
			if cells.Width(l) > cols {
				return false
			}
		}
		return true
	}
	cols := (artRightR - artMsgX) / CellW
	if lines := SubMenuLines(k, items, cols, artLowerRows); fits(lines, cols, artLowerRows) {
		return lowerLayout{lines, false, true}
	}
	scols, srows := artLowerSmall()
	lines := SubMenuLines(k, items, scols, srows)
	if fits(lines, scols, srows) && c.FitsSmall(strings.Join(lines, "")) {
		return lowerLayout{lines, true, true}
	}
	return lowerLayout{}
}

// artLowerSmall 是下面板用小字級時放得下幾格、幾行（192 × 64 像素）。
func artLowerSmall() (cols, rows int) {
	return (artRightR - artMsgX) / SmallW, artLowerRows * CellH / SmallH
}

// subMenuFitsLower 回報這一類的子選單排得進下面板（原尺寸或小字）。
func subMenuFitsLower(c *Canvas, k byte, items []Command) bool {
	return subMenuLayout(c, k, items).ok
}

// subMenuKey 是這個標題屬於哪一類的子選單；不是類別子選單回 0。
//
// **用標題認，不另外記狀態**：挑選清單與數字輸入也用 `View.Menu`，
// 多一個欄位就多一處會忘了清掉的狀態。挑選清單的標題是「挑哪一位」
// 這一類問句，與九個類別名不會撞。
func subMenuKey(title string) byte {
	for k := byte('1'); k <= '9'; k++ {
		if name, _ := SubMenu(k); name == title {
			return k
		}
	}
	return 0
}

// 上面板與內容區覆蓋頁的版面（`docs/spec/014` §2.1、§3.3）。
const (
	artUpperX    = 424 // 上面板文字區左緣
	artUpperY    = 52  // 上面板文字區第一列
	artUpperCols = 24  // 424–616
	artUpperRows = 14  // 52–276

	artCmdRightX = 536 // 指令表右欄
	artCmdRowDY  = 48  // 指令表每列 +48
	artCmdMaxW   = 4   // 名稱畫雙倍字最多幾格（64 像素，原版兩個全形字）
	artCmdEdge   = 624 // 上面板內緣（右邊的外框從這裡開始）

	artLowerRows = 4 // 下面板 300／316／332／348

	artPageX0, artPageY0 = 72, 36   // 內容區：地圖 ＋ 右側面板
	artPageX1, artPageY1 = 632, 372 // 70 格 × 21 行
)

// 指令表與覆蓋頁的字色（EGA 色號，量自原版畫面）。
var (
	artInkCmdEven = assets.EGAPalette[14] // 偶數項黃
	artInkCmdOdd  = assets.EGAPalette[13] // 奇數項洋紅
	artInkCmdKey  = assets.EGAPalette[15] // 編號白
	artInkList    = assets.EGAPalette[15]
	artInkPageBG  = assets.EGAPalette[1] // 分頁的藍底（原版將軍資料頁）
	artInkPageDim = assets.EGAPalette[11]
)

// drawArtCommands 畫上面板的十項指令表（`docs/spec/014` §2.1）。
//
// 兩欄五列：編號「`0.`」是 16×16 貼在列的下半，名稱是 32×32 雙倍字；
// 名稱**偶數項黃、奇數項洋紅**。
//
// 雙倍字的寬度是照原版兩個全形字排的（64 像素）：右欄的名稱從 552 起，
// 到面板內緣只剩 72 像素。**十項有一項放不下就整張改畫一倍字**、垂直
// 置中——同一張表大小混雜，看起來像排版壞了。中文十項都是兩個字，
// 所以永遠是雙倍字，與原版相同。
func drawArtCommands(c *Canvas) {
	cmds := Commands()
	double := true
	for _, cmd := range cmds {
		if cells.Width(cmd.Name) > artCmdMaxW {
			double = false
		}
	}
	for i, cmd := range cmds {
		x, y := artUpperX, artUpperY+(i%5)*artCmdRowDY
		limit := artCmdRightX // 左欄的名稱不能壓到右欄的編號
		if i >= 5 {
			x, limit = artCmdRightX, artCmdEdge
		}
		x += c.DrawTextPx(x, y+CellH, string(cmd.Key)+".", artInkCmdKey)
		ink := artInkCmdEven
		if i%2 == 1 {
			ink = artInkCmdOdd
		}
		if double {
			for _, r := range cmd.Name {
				x += c.DrawRuneScaledPx(x, y, r, ink, 2, 2)
			}
			continue
		}
		c.DrawTextPx(x, y+CellH/2, cells.Truncate(cmd.Name, (limit-x)/CellW), ink)
	}
}

// drawArtList 把挑選清單或數字輸入畫在上面板：標題一列、一行一項。
// 標籤寬過上面板時回 false，讓呼叫端改畫在內容區。
func drawArtList(c *Canvas, title string, items []Command) bool {
	lines := commandLines(items)
	if cells.Width(title) > artUpperCols {
		return false
	}
	for _, l := range lines {
		if cells.Width(l) > artUpperCols {
			return false
		}
	}
	c.DrawTextPx(artUpperX, artUpperY, title, artInkName)
	for i, l := range lines {
		if 1+i >= artUpperRows {
			c.DrawTextPx(artUpperX, artUpperY+(artUpperRows-1)*CellH,
				cells.Truncate(t("msg.more"), artUpperCols), artInkPageDim)
			break
		}
		c.DrawTextPx(artUpperX, artUpperY+(1+i)*CellH, l, artInkList)
	}
	return true
}

// commandLines 把清單項目排成一行一項：數字鍵寫成原版的「`1.名稱`」。
//
// 數字輸入借 `Command` 放「= 數值」與「上限」兩行（`cmd/san1` 的
// `showNumber`），那兩個鍵不是數字，照「`= 30`」印；鍵是空白的那一行
// 不印編號。
func commandLines(items []Command) []string {
	out := make([]string, 0, len(items))
	for _, it := range items {
		switch {
		case it.Key == ' ':
			out = append(out, "  "+it.Name)
		case it.Key >= '0' && it.Key <= '9':
			out = append(out, string(it.Key)+"."+it.Name)
		default:
			out = append(out, string(it.Key)+" "+it.Name)
		}
	}
	return out
}

// drawArtLower 畫下面板：類別子選單照原版斷行，其餘時候放提示或最後一則
// 訊息，最多四行（`docs/spec/014` §3.2）。
func drawArtLower(c *Canvas, log []string, v View, subLower bool,
	cursor *[assets.MenuOrnamentFrameCount]assets.CursorFrame) {
	w := (artRightR - artMsgX) / CellW
	var lines []string
	k := subMenuKey(v.Menu)
	switch {
	case v.Menu != "" && k != 0 && subLower:
		if lay := subMenuLayout(c, k, v.Items); lay.small {
			for i, line := range lay.lines {
				c.DrawSmallTextPx(artMsgX, artMsgY+i*SmallH, line, artInkMsg)
			}
			return
		} else {
			lines = lay.lines
		}
	case v.Menu != "" && k != 0:
		// 子選單改畫在上面板了，下面板只留原版那一行提示字。
		lines = []string{t(subMenuPrompt[k])}
	default:
		msg := v.Prompt
		if msg == "" && len(log) > 0 {
			msg = log[len(log)-1]
		}
		if v.Over {
			lines = append(lines, cells.Wrap(t("msg.over"), w)...)
		}
		if msg != "" {
			// 原版是**兩行**（「新君主主公,請到(41)」／「南海下您的命令:」），
			// 一句話折過去，不是兩則訊息。
			lines = append(lines, MessageLines(msg, w)...)
		}
	}
	if len(lines) > artLowerRows {
		lines = lines[:artLowerRows]
	}
	for i, line := range lines {
		c.DrawTextPx(artMsgX, artMsgY+i*artMsgDY, line, artInkMsg)
	}
	// 游標接在最後一行字後面（主命令「南海下您的命令:」之後是 (544,316)）。
	if x, y, ok := cursorAfterLines(artMsgX, artMsgY, artMsgDY, lines); ok {
		drawInputCursor(c, cursor, v.Input, x, y)
	}
}

// drawArtOverlay 把一整頁蓋在內容區上（`docs/spec/014` §3.3）。
func drawArtOverlay(c *Canvas, title string, body []string, hint string, top int) {
	drawOverlay(c, artPageX0, artPageY0, artPageX1, artPageY1, title, body, hint, top)
}

// drawOverlay 把一整頁蓋在一塊矩形上：藍底（原版將軍資料頁的底色）、
// 標題黃、內容白，最後一行是提示。長的一行折下去（`PageLines`），
// 一頁放不下時從 top 那一行開始畫，標題帶位置、提示換成怎麼捲。
func drawOverlay(c *Canvas, x0, y0, x1, y1 int, title string, body []string, hint string, top int) {
	c.FillRect(x0, y0, x1, y1, artInkPageBG)
	cols := (x1-x0)/CellW - 2
	rows := (y1-y0)/CellH - 2 // 標題與提示各佔一行
	x := x0 + CellW
	lines, head, scroll, top := pageWindow(title, body, top, cols, rows)
	if scroll != "" {
		hint = scroll
	}
	c.DrawTextPx(x, y0, cells.Truncate(head, cols), artInkName)
	for i := 0; i < rows && top+i < len(lines); i++ {
		c.DrawTextPx(x, y0+(1+i)*CellH, lines[top+i], artInkList)
	}
	if hint != "" {
		c.DrawTextPx(x, y0+(rows+1)*CellH, cells.Truncate(hint, cols), artInkPageDim)
	}
}

// drawArtStatus 畫上面板的郡的資料（原版的「0.狀態」，`docs/spec/005` §2.1）。
//
// 位置與字色照原版量；**槽位也是原版的**：左欄標籤 ＋ 靠右的數值共 12 格
// （424–520）、右欄 10 格（536–616）。中文的欄名照原版，英日文用一組
// 面板專用的短欄名（`stat.*`）才塞得進（`TestArtStatusFitsEveryLanguage`）。
func drawArtStatus(c *Canvas, g *game.State, p *game.Prefecture, sel int) {
	// 第一列：郡名是 32×32 的雙倍字，州名與編號在它右邊。
	//
	// 譯名（拼音）雙倍字放不下，改成一倍字、可以用到君主那一欄前面
	//（14 格，`Yingchuan` 九格），州名移到第二列、郡編號的左邊（8 格）。
	if name := PlaceName(p.Name); artAllWide(name) {
		artBigName(c, artNameX, artNameY, artProvX-artNameX, name, artInkName)
		c.DrawTextPx(artProvX, artProvY, artProvince(int(p.Province)), artInkProv)
	} else {
		c.DrawTextPx(artNameX, artNameY, cells.Truncate(name, artNameCols), artInkName)
		c.DrawTextPx(artNameX, artIDY, artProvinceIn(int(p.Province), artProvWideCols), artInkProv)
	}
	x := artProvX
	for _, r := range fmt.Sprintf("%2d", p.ID) {
		x += c.DrawRuneScaledPx(x, artIDY, r, artInkProv, 2, 1)
	}

	// 君主與人望那兩列右邊沒有數值欄，可以用到面板內緣（624）——
	// 英文的「Gongsun Zan」十一格，停在 616 就會被截掉最後一個字母。
	rightCols := artLordCols
	if lord := g.Lord(p.Owner); lord != nil {
		c.DrawTextPx(artLordX, artLordY,
			cells.Truncate(tf("stat.lord", PersonName(lord.Name)), rightCols), artInkLord)
		if f := g.Faction(p.Owner); f != nil {
			c.DrawTextPx(artLordX, artFameY,
				cells.Truncate(tf("stat.fame", f.Prestige), rightCols), artInkFame)
		}
	} else {
		c.DrawTextPx(artLordX, artLordY, t("stat.noLord"), artInkLord)
	}
	c.DrawTextPx(artAutoX, artAutoY, game.AutonomyName(p.Autonomy), artInkAuto)
	// 軍師：所屬諸侯有軍師、而且那一位就在這一郡才畫（`0x33411`–`0x3347c`）。
	if f := g.Faction(p.Owner); f != nil && p.Owned() && f.Chief >= 0 {
		if x := g.General(f.Chief); x != nil && x.Location == p.ID {
			c.DrawTextPx(artLordX, artChiefY,
				cells.Truncate(tf("stat.chief", PersonName(x.Name)), rightCols), artInkChief)
		}
	}

	// 欄位表：標籤靠左、數值靠右對齊。
	//
	// **原版的數值是靠右不是靠標籤排的**：位數變了整欄還是對齊，
	// 用空白填出來的版面只有在位數剛好時才一樣。
	right := func(rx, ry int, s string, fg color.RGBA) {
		c.DrawTextPx(rx-cells.Width(s)*CellW, ry, s, fg)
	}
	for _, f := range artStatusFields(g, p) {
		c.DrawTextPx(f.x, f.y, f.label, f.fg)
		right(f.r, f.y, f.value, f.fg)
	}
	// 右欄：主事者姓名（32×32 的雙倍字）。原版把 6 byte 的姓名欄整條從 x 520
	// 起畫（`0x334f4`）：兩字名前後各補一個空白，所以字落在 536；三字名填滿，
	// 從 520 起。
	if who := g.Governor(sel); who != nil {
		name := PersonName(who.Name)
		x := artRightX
		if n := len([]rune(name)); artAllWide(name) && n >= 1 && n <= 3 {
			x = artGovFieldX + 8*(6-2*n)
		}
		artBigName(c, x, 212, artRightR-x, name, artInkGov)
	}
}

// artStatusField 是面板上的一格：標籤從 x 起，數值靠右對齊到 r。
type artStatusField struct {
	x, y, r      int
	label, value string
	fg           color.RGBA
}

// artStatusFields 列出面板上的十格。抽出來是為了**量得到**：
// `TestArtStatusFitsEveryLanguage` 拿同一份清單檢查每一格塞不塞得下。
func artStatusFields(g *game.State, p *game.Prefecture) []artStatusField {
	num := func(v int) string { return fmt.Sprintf("%d", v) }
	L := func(y int, key, value string, fg color.RGBA) artStatusField {
		return artStatusField{artFieldX, y, artValueR, t(key), value, fg}
	}
	R := func(y int, key, value string, fg color.RGBA) artStatusField {
		return artStatusField{artRightX, y, artRightR, t(key), value, fg}
	}
	return []artStatusField{
		L(116, "stat.land", num(int(p.LandValue)), artInkField),
		L(132, "stat.flood", num(int(p.FloodRate)), artInkField),
		L(148, "stat.price", num(int(p.PriceLevel)), artInkField),
		L(164, "stat.loyalty", num(int(p.PublicLoyalty)), artInkField),
		L(180, "stat.population", num(int(p.Population)), artInkField),
		L(212, "stat.gold", num(int(p.Gold)), artInkGold),
		L(228, "stat.rice", num(int(p.Rice)), artInkGold),
		L(260, "stat.free", num(g.FreeGenerals(p.ID)), artInkAuto),
		// 現役將與兵士畫的是州郡記錄的**快照**（offset 22／16），不是即時算的：
		// 兵士以百計，原版印 `"兵士%4d00"`（`0x33500`／`0x33540`）。
		R(244, "stat.officers", num(g.StoredActiveGenerals(p.ID)), artInkField),
		R(260, "stat.soldiers", fmt.Sprintf("%d00", g.Troops(p.ID)), artInkField),
	}
}

// artBigName 畫面板上的大字名字（郡名、主事者）。
//
// 中文照原版畫 32×32 雙倍字。譯名是拼音、不是全形字——雙倍寬一個字母
// 16 像素，「Nanhai」就壓到右邊的州名上了——所以改畫一倍字、垂直置中、
// 截在槽位內。
func artBigName(c *Canvas, x, y, slot int, s string, ink color.RGBA) {
	if artAllWide(s) {
		for _, r := range s {
			x += c.DrawRuneScaledPx(x, y, r, ink, 2, 2)
		}
		return
	}
	c.DrawTextPx(x, y+CellH/2, cells.Truncate(s, slot/CellW), ink)
}

// artHasLatin 回報字串裡有沒有拉丁字母（英文的年號、拼音）。
// 只有數字不算：日文的西曆「189年1月春」照原版直排。
func artHasLatin(s string) bool {
	for _, r := range s {
		if (r >= 'A' && r <= 'Z') || (r >= 'a' && r <= 'z') {
			return true
		}
	}
	return false
}

func artAllWide(s string) bool {
	for _, r := range s {
		if cells.RuneWidth(r) != 2 {
			return false
		}
	}
	return s != ""
}

// artProvince 是州名（原版資料的字）。槽位只有 6 格（488–536）：
// 譯名放不下時拿掉「州」字再轉——「Jiaozhou」變「Jiao」。
//
// 截字比拿掉「州」更糟：「Jiaozh」看不出是哪一州，「Jiao」看得出。
func artProvince(i int) string { return artProvinceIn(i, artProvCols) }

// artProvinceIn 是塞進 cols 格的州名：放不下就拿掉「州」字再轉。
func artProvinceIn(i, cols int) string {
	zh := state.ProvinceName(i)
	s := PlaceName(zh)
	if cells.Width(s) > cols {
		s = PlaceName(strings.TrimSuffix(zh, "州"))
	}
	return cells.Truncate(s, cols)
}

// artProvCols 是州名的槽寬（488 到君主那一欄的 536）；artLordCols 是
// 君主與人望那兩列的寬（536 到面板內緣 624）。
const (
	artProvCols = (artLordX - artProvX) / CellW
	artLordCols = (artCmdEdge - artLordX) / CellW

	// 譯名的排法：郡名一倍字用到君主欄前（424–536），州名在第二列、
	// 郡編號的左邊（424–488）。
	artNameCols     = (artLordX - artNameX) / CellW
	artProvWideCols = (artProvX - artNameX) / CellW
)

// TitleScreen 是主選單畫面（原版開機後的那一張）。
//
// 圖是原版的（`assets.MenuScreen`），字是 remake 自己的字庫。
// 六個項目的文字照原版的選單抄（`docs/re/02` §3 的開機畫面）。
type TitleScreen struct {
	bg     *assets.Image
	frames [assets.MenuOrnamentFrameCount]*assets.Image
}

// TitleOrnamentTicksPerFrame 是 remake 的可攜節拍；60 TPS 時每格約 0.13 秒。
// 原版只量得到每格 25–26 萬道指令，不能跨 DOS 主機換成唯一 wall-clock。
const TitleOrnamentTicksPerFrame = 8

// NewTitleScreen 從 `DATA3` 拼出主選單畫面；有傳 `DATA1` 時再接上
// `CURA0`～`CURA5` 的小飾框動畫。variadic 保留無 DATA1 的靜態退路。
func NewTitleScreen(data3 *assets.Container, data1 ...*assets.Container) (*TitleScreen, error) {
	bg, err := assets.MenuScreen(data3)
	if err != nil {
		return nil, err
	}
	ts := &TitleScreen{bg: bg}
	if len(data1) > 0 && data1[0] != nil {
		if ts.frames, err = assets.MenuScreenFrames(data1[0], data3); err != nil {
			return nil, err
		}
	}
	return ts, nil
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
	DrawTitleFrame(c, ts, sel, 0)
}

// DrawTitleFrame 畫指定的 `CURA0`～`CURA5` 小飾框畫格。
func DrawTitleFrame(c *Canvas, ts *TitleScreen, sel, frame int) {
	bg := ts.bg
	if frame >= 0 && frame < len(ts.frames) && ts.frames[frame] != nil {
		bg = ts.frames[frame]
	}
	draw.Draw(c.Img, image.Rect(0, 0, assets.ScreenW, assets.ScreenH),
		bg.RGBA(), image.Point{}, draw.Src)
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
	frame   [4]*assets.Image // 軍力面板的肖像框 `FBRC`（`0x22c94` 的樣式 2）
	frameA  [4]*assets.Image // 對戰子畫面部隊面板的肖像框 `FBRA`（`0x320a6` 的樣式 0）
	frameB  [4]*assets.Image // 查看那一塊的肖像框 `FBRB`（`0x284a2` 的樣式 1）
	weather [3]*assets.Image
	faces   *assets.Container
	bottom  *assets.Image
	// cursor 是主戰場的輸入游標（`CURC`，`docs/spec/014` §4.1）。
	cursor *[assets.MenuOrnamentFrameCount]assets.CursorFrame
}

// NewArtBattle 解出三十六張地形圖塊與上下兩條花邊。data3 可以是 nil，
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
	if f, err := assets.CursorFrames(data1, assets.CursorBattle); err == nil {
		ab.cursor = &f
	}
	if ab.bg, err = assets.BattleBackground(data1); err != nil {
		return nil, err
	}
	if ab.frame, err = assets.PortraitFrame(data1, 'C'); err != nil {
		return nil, err
	}
	if ab.frameA, err = assets.PortraitFrame(data1, 'A'); err != nil {
		return nil, err
	}
	if ab.frameB, err = assets.PortraitFrame(data1, 'B'); err != nil {
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
		// 下方花邊。原版 `0x2246a` 那一段載完 `MAINMAP1` 接著載
		// `MAINMAP8`，畫在 (0,372)——640×36，下緣正好是畫面的 408
		// （`docs/spec/006`）。先前把它記成「落在畫面外」，
		// 那是畫布設成 350 的結果。
		if i, ok := data3.ByName("MAINMAP8.IMG"); ok {
			if im, err := assets.DecodeImage(data3.Data(i)); err == nil {
				ab.bottom = im
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
// TitleListCols 是開局選單一項最多幾格；再長的會被截掉
// （`internal/menu` 的 `TestTitleListsFitEveryLanguage` 盯著）。
const TitleListCols = 40

func DrawTitleList(c *Canvas, ts *TitleScreen, title string, items []string, sel int) {
	DrawTitle(c, ts, -1)
	// 蓋在六個按鈕那一片上，標題牌留著看得見。
	const col, row, cols = 18, 9, TitleListCols + 4
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

// 主選單以外的幾層（選擇年代…）：原版**不換畫面**，直牌換字、六個按鈕上的
// 字整條重寫（`0x11c7e` 選擇年代，`docs/spec/005` §6.4）。
//
//	直牌  0x33d8:0x1d4e(80, 242+24k, 字, 字色, …)   主選單字色 11、選擇年代 15
//	按鈕  0x33d8:0x104e(x, y, 字串, 字色 14, 底 3)  x ∈ {168, 392}、y ∈ {224, 276, 329}
//
// 按鈕那一條是**照字串逐字排**：半形 8、全形 16，從按鈕左緣 +16 起，字串尾端
// 的空白蓋掉上一層的字（每條 20 個半形格），按鈕圖本身不重貼。
const (
	// TitleLayerTextX／Y 是按鈕上那一條字相對按鈕左上角的位置。
	TitleLayerTextX = 16
	TitleLayerTextY = 9
	// TitleLayerTextCells 是一條有幾個半形格。
	TitleLayerTextCells = 20
	// TitleLayerTextBG／FG 是那一條的底色與字色。
	TitleLayerTextBG = 3
	TitleLayerTextFG = 14
	// ScenarioLabelInk 是選擇年代那一層直牌的字色（`0x11c9a`）。
	ScenarioLabelInk = 15
)

// DrawTitleLayer 畫主選單的其他一層：frame 是小飾框的畫格、labelInk 是直牌字色、
// items 是六個按鈕上的字串（原樣排）、sel 是 remake 反白的那一項（−1 不反白）。
//
// 直牌的字只有全是全形字時才畫（英日版直排放不下，留白，remake 差異）。
func DrawTitleLayer(c *Canvas, ts *TitleScreen, frame int, label string, labelInk byte, items []string, sel int) {
	bg := ts.bg
	if frame >= 0 && frame < len(ts.frames) && ts.frames[frame] != nil {
		bg = ts.frames[frame]
	}
	draw.Draw(c.Img, image.Rect(0, 0, assets.ScreenW, assets.ScreenH),
		bg.RGBA(), image.Point{}, draw.Src)
	if artAllWide(label) {
		DrawMenuLabel(c, label, assets.EGAPalette[labelInk&15])
	}
	for i, b := range assets.MenuButtons() {
		x, y := b[0]+TitleLayerTextX, b[1]+TitleLayerTextY
		c.FillRect(x, y, x+TitleLayerTextCells*CellW, y+CellH, assets.EGAPalette[TitleLayerTextBG])
		if i >= len(items) {
			continue
		}
		ink := assets.EGAPalette[TitleLayerTextFG]
		if i == sel {
			ink = assets.EGAPalette[15]
		}
		for _, r := range items[i] {
			w := cells.RuneWidth(r)
			if w == 0 {
				continue
			}
			if x+w*CellW > b[0]+TitleLayerTextX+TitleLayerTextCells*CellW {
				break
			}
			if r != ' ' {
				c.DrawRuneWidePx(x, y, r, ink, 1)
			}
			x += w * CellW
		}
	}
}

// MessageLines 照原版訊息常式（`0x33d8:0xcc0`）把一段字排成行：換行字元換行、放不下就折；
// **一行剛好寫滿，游標立刻移到下一行**（`0x34c3c`：寫完一個字右移，超過右緣就換行），
// 所以寫滿之後緊接的換行字元會多出一行空白，寫滿結尾的游標也在下一行開頭。
// 「<偽書使疑>派細作到那一郡\n(1-42):」剛好 24 格，原版的「(1-42):」在第三行。
func MessageLines(text string, cols int) []string {
	var out []string
	for _, par := range strings.Split(text, "\n") {
		lines := []string{""}
		if par != "" {
			lines = cells.Wrap(par, cols)
		}
		if n := len(lines); cols > 0 && n > 0 && cells.Width(lines[n-1]) == cols {
			lines = append(lines, "")
		}
		out = append(out, lines...)
	}
	return out
}
