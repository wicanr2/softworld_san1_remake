package ui

import (
	"fmt"
	"image"
	"image/color"
	"image/draw"

	"github.com/wicanr2/softworld_san1_remake/internal/cells"

	"github.com/wicanr2/softworld_san1_remake/internal/assets"
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
}

// NewArtScreen 從 `DATA3` 容器拼出底圖，順便留著容器好取肖像。
func NewArtScreen(data3 *assets.Container) (*ArtScreen, error) {
	bg, err := assets.MainScreen(data3)
	if err != nil {
		return nil, err
	}
	return &ArtScreen{base: bg, faces: data3}, nil
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

// 原版畫面上的幾個位置（像素），量自 `workplace/shots/orig-main.png`。
const (
	artPanelX = 408 // 右面板左緣
	artPanelW = 224
	artMapX   = 72  // 地圖區左緣（左邊那 72 像素是花邊直條）
	artMsgY   = 302 // 底部訊息列

	// artPortraitX／Y 是肖像的位置。**量出來的**：拿原版的主畫面截圖
	// 與 256 張 `F###.FAC` 逐一比對，`F228.FAC` 在 (536,116) 逐像素
	// 100% 相符（`cmd/san1imgcheck`）。
	artPortraitX = 536
	artPortraitY = 116

	// artDateCol 是左側直條上年月的位置（原版直排在那裡）。
	artDateCol = 1
	artDateRow = 4
)

// artFactionColour 是州郡填色用的 EGA 色號。
//
// **這不是原版的表。** 原版的填色走 `EGAFILL.PAL`（1024 個位元組，
// 還沒解），這裡先用一組看得出區別的色號，登記為 remake 差異
// （`docs/design/02`）。無主的郡不填，留底圖的白色。
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
	// 主事者的肖像疊在底圖上，位置與原版相同。
	if who := g.Governor(sel); who != nil {
		if face := a.Portrait(int(who.Portrait)); face != nil {
			im.Blit(face, artPortraitX, artPortraitY)
		}
	}
	draw.Draw(c.Img, image.Rect(0, 0, assets.ScreenW, assets.ScreenH),
		im.RGBA(), image.Point{}, draw.Src)

	col := artPanelX / CellW // 右面板從第 51 格開始
	row := 3
	ink := color.RGBA{0x00, 0x00, 0x00, 0xFF} // 面板底色是亮的，字用黑

	// 年月直排在左側直條上，與原版一樣。
	for i, r := range []rune(g.Date.Format(v.Calendar)) {
		c.DrawText(artDateCol, artDateRow+i, string(r), ink)
	}
	// 第一列照原版：郡名、州名、編號。
	c.DrawText(col, row, p.Name, ink)
	c.DrawText(col+5, row, state.ProvinceName(int(p.Province)),
		color.RGBA{0xAA, 0x00, 0x00, 0xFF})
	c.DrawText(col+10, row, fmt.Sprintf("%2d", p.ID), ink)
	row++
	if lord := g.Lord(p.Owner); lord != nil {
		c.DrawText(col, row, "君主 "+lord.Name, ink)
		if f := g.Faction(p.Owner); f != nil {
			c.DrawText(col+13, row, fmt.Sprintf("人望%3d", f.Prestige), ink)
		}
	} else {
		c.DrawText(col, row, "無　主", ink)
	}
	row++
	c.DrawText(col+2, row, game.AutonomyName(p.Autonomy), ink)
	row++
	for _, line := range []string{
		fmt.Sprintf("土地價值 %3d", p.LandValue),
		fmt.Sprintf("洪水率   %3d", p.FloodRate),
		fmt.Sprintf("物價     %3d", p.PriceLevel),
		fmt.Sprintf("民眾忠誠 %3d", p.PublicLoyalty),
		fmt.Sprintf("人口   %5d", p.Population),
		"",
		fmt.Sprintf("金     %5d", p.Gold),
		fmt.Sprintf("米     %5d", p.Rice),
	} {
		c.DrawText(col, row, line, ink)
		row++
	}
	row++
	c.DrawText(col, row, fmt.Sprintf("現役武將 %2d", len(g.Garrison(p.ID))), ink)
	row++
	c.DrawText(col, row, fmt.Sprintf("兵士   %5d", g.Soldiers(p.ID)), ink)

	// 主事者的姓名寫在肖像下面。
	if who := g.Governor(sel); who != nil {
		c.DrawText(artPortraitX/CellW, (artPortraitY+80)/CellH+1, who.Name, ink)
	}

	// 底部訊息列：原版在地圖區底下壓一條，字用亮色。
	msg := v.Prompt
	if msg == "" && len(log) > 0 {
		msg = log[len(log)-1]
	}
	if msg != "" {
		c.FillRect(artMapX, artMsgY, artPanelX-8, assets.ScreenH-4,
			color.RGBA{0x00, 0x00, 0xAA, 0xFF})
		c.DrawText(artMapX/CellW+1, (artMsgY+6)/CellH,
			cells.Truncate(msg, (artPanelX-artMapX)/CellW-3),
			color.RGBA{0xFF, 0xFF, 0x55, 0xFF})
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
func DrawTitle(c *Canvas, ts *TitleScreen, sel int) {
	draw.Draw(c.Img, image.Rect(0, 0, assets.ScreenW, assets.ScreenH),
		ts.bg.RGBA(), image.Point{}, draw.Src)
	ink := color.RGBA{0x55, 0xFF, 0xFF, 0xFF}
	hot := color.RGBA{0xFF, 0xFF, 0x55, 0xFF}
	// 左邊那面牌子：原版有一張 `MENU1.IMG`，但位置還沒定得下來
	// （`internal/assets` 的說明），所以先由 remake 自己畫一個框。
	c.DrawBox(7, 13, 12, 8, ink)
	for i, r := range []rune("主選擇單") {
		c.DrawText(12, 15+i, string(r), ink)
	}
	for i, s := range TitleItems() {
		b := assets.MenuButtons()[i]
		col := hot
		if i != sel {
			col = ink
		}
		// 按鈕是 200×46；字往內縮 16 像素、直向置中。
		c.DrawText((b[0]+16)/CellW, (b[1]+15)/CellH, s, col)
	}
}
