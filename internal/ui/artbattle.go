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

	"github.com/wicanr2/softworld_san1_remake/internal/assets"
	"github.com/wicanr2/softworld_san1_remake/internal/battle"
	"github.com/wicanr2/softworld_san1_remake/internal/cells"
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
	// **remake 還沒畫下方花邊上的年月。** 原版在這條花邊上寫一行灰色的
	// 年月（基準畫面是「建安 二 年 九月 秋」，色號 7，約 35 點／列）。
	// 花邊本身兩邊逐列相同，缺的只有那一行字——它是畫面高度從 350 改回
	// 408 之後才看得到的（`docs/spec/006`）。
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
		for k, s := range lines {
			c.DrawTextPx(x, assets.BattlePanelY+4+k*CellH,
				cells.Truncate(s, (assets.BattlePanelW-8)/CellW),
				assets.EGAPalette[assets.BattlePanelInk])
		}
	}

	// 指令面板：原版的三行三列 ＋ 提示。
	ordX := assets.BattlePanelX[2] + 4
	ord := assets.EGAPalette[assets.BattleOrderInk]
	for k, s := range BattleCommandLines() {
		c.DrawTextPx(ordX, assets.BattlePanelY+4+k*CellH, s, ord)
	}
	row := 3
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
}

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
