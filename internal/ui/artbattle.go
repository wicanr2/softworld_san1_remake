package ui

// 接上原版素材的主戰場。
//
// 版面全是原版的（`docs/spec/005` §主戰場、`docs/re/05` §2.5）：底紋、
// 上方花邊、場地圖塊、部隊的旗與兵力牌、兩個軍力面板與指令面板的位置、
// 肖像與肖像框。**字是 remake 自己的字庫**（`CLAUDE.md` §3.3），
// 所以字級與原版的 32×32／16×16 不同；面板裡的排法也是 remake 自己排的。
//
// 戰場上的對白不在這裡：`DrawBattleSpeech`（`bubble.go`）把那一塊面板
// 填藍再走 `DrawBubble` 疊在這一張上，主程式畫完 `DrawArtBattle` 才叫它
// （`docs/spec/005` §9.7）。

import (
	"fmt"
	"image"
	"image/draw"
	"strconv"
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
	// （−1 ＝ 沒有）。順序是攻方、守方，與 `assets.BattleLayout.Face` 的側別相同。
	Commander [2]string
	Portrait  [2]int

	// Lord 是兩邊**君主**的姓名：面板第一行「X軍」寫的是勢力的君主
	// （`0x22dbb` 從統帥的勢力查諸侯表），不是統帥。空的就用統帥。
	Lord [2]string

	// Units 不是 nil 時，兩塊軍力面板改畫**部隊面板**（對戰子畫面裡的
	// 兩支：攻方陣營那一支、守方陣營那一支，`0x320a6`），見 UnitPanel。
	Units *[2]UnitPanel

	// Inspect 不是 nil 時，第三塊面板改畫**查看**那一位將領（`0x284a2`）。
	Inspect *InspectPanel

	// Skirmish 不是 nil 時場地改畫對戰子畫面的子地圖與將領標記
	// （`0x2e700`／`0x2e796`），左欄的時刻照子畫面的時刻。
	Skirmish *battle.Skirmish
}

// UnitPanel 是對戰子畫面裡一塊部隊面板要的東西（`0x320a6(軍力, 隊伍)`，
// `L0`；進子畫面、攻擊結算與單挑收尾都畫它）：第 0 槽那一位的肖像與名字
// （`Unit.Head`），君主名，軍力名，「隊伍名 N將」，「兵 N」。**單挑時畫的
// 也是部隊第 0 槽那一位，不是單挑的當事人。** Unit 為 nil、或第 0 槽空了，
// 面板只剩藍底。
type UnitPanel struct {
	Unit *battle.Unit
	// Portrait 是第 0 槽那一位的肖像編號（−1 ＝ 沒有）。
	Portrait int
	// Lord 是這支部隊勢力的君主名。
	Lord string
}

// InspectPanel 是查看那一塊要的東西（`0x284a2(軍力, 人物)`，`L0`）：
// 那一位的肖像（不翻面）與名字（該側的字色）、六行「體能／謀略／戰力／
// 訓練／武裝／兵」。
type InspectPanel struct {
	Leader *battle.Leader
	Side   battle.Side
	// Portrait 是肖像編號（−1 ＝ 沒有）。
	Portrait int
}

// 查看那一塊的位置（`0x284a2`，兩種版面相同——座標是寫死的）：肖像
// 64×80 在 (552,276)、框 `FBRB` 在肖像外圍 8、名字 32×32 直排在 x 512、
// 六行字從 (448,268) 起一行 16。
const (
	inspectFaceX, inspectFaceY = 552, 276
	inspectNameX               = 512
)

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
	//
	// 版面照州郡的形狀挑：12 欄的寬圖面板在下、8 欄的窄圖面板在右
	// （`docs/spec/005` §8「主戰場整張畫面的版面」）。
	l := assets.BattleLayoutFor(b.Field.Narrow())
	im.FieldEdges(l)
	im.BlitField(ab.tiles, info.Field)
	if s := info.Skirmish; s != nil {
		// 子地圖蓋在主戰場上：地形碼 15 的格不畫，露出底下原來那一張
		// （`0x2e76c`）；旗幟不畫，換成將領標記。
		im.BlitFieldUpTo(ab.tiles, skirmishFieldBytes(s), assets.SkirmishMaxTerrain)
		drawSkirmishMarkers(im, s, v)
	} else {
		ab.drawUnits(im, b, v)
	}

	// 左欄、場地左右緣的線與三個面板：先塗底色再畫下凹的外框。
	// 查看那一塊（`0x284a2`）把第三塊面板清成藍再畫，與軍力面板同色。
	im.LeftColumn()
	im.FieldLines(l)
	for i := range l.PanelX {
		paper := byte(assets.BattlePanelPaper)
		if i == 2 && info.Inspect == nil {
			paper = assets.BattleOrderPaper
		}
		x0, y0, x1, y1 := l.Panel(i)
		im.FillRect(x0, y0, assets.BattlePanelW, assets.BattlePanelH, paper)
		im.BevelBox(x0, y0, x1, y1)
	}

	// 天氣圖示與肖像。
	if w := ab.weather[b.Weather.OriginalIndex()%len(ab.weather)]; w != nil {
		im.Blit(w, assets.BattleWeatherX, assets.BattleWeatherY)
	}
	// 兩塊軍力面板的肖像：統帥（框 `FBRC`），或對戰子畫面裡那兩支部隊
	// 的第 0 槽（框 `FBRA`；第 0 槽空了那一塊只剩藍底，`0x320a6` 的
	// `0xFFFF` 分支）。
	for i := range assets.BattleFaceMirror {
		portrait, frame := info.Portrait[i], ab.frame
		if info.Units != nil {
			portrait, frame = -1, ab.frameA
			if u := info.Units[i]; u.Unit != nil && u.Unit.Head() != nil {
				portrait = u.Portrait
			}
			if portrait < 0 {
				continue
			}
		}
		fx, fy := l.Face(i)
		ab.blitFrame(im, frame, fx, fy)
		face := ab.face(portrait)
		if face == nil {
			continue
		}
		if assets.BattleFaceMirror[i] {
			face = face.Mirror()
		}
		im.Blit(face, fx, fy)
	}
	// 查看：那一位的肖像在 (552,276)，不翻面，框是 `FBRB`。
	if info.Inspect != nil {
		ab.blitFrame(im, ab.frameB, inspectFaceX, inspectFaceY)
		if face := ab.face(info.Inspect.Portrait); face != nil {
			im.Blit(face, inspectFaceX, inspectFaceY)
		}
	}
	return im
}

// blitFrame 把一組肖像框的四片畫在肖像 (x, y) 的外圍：上 (x−8, y−8)、
// 下 (x−8, y+80)、左 (x−8, y)、右 (x+64, y)（`0x1058:0x276c`，樣式
// 0–3 ＝ `FBRA`–`FBRD`；軍力面板的 `FBRC`、部隊面板的 `FBRA`、查看的
// `FBRB` 四片在基準畫面上各 0 像素差）。
func (ab *ArtBattle) blitFrame(im *assets.Image, fr [4]*assets.Image, x, y int) {
	at := [4][2]int{{x - 8, y - 8}, {x - 8, y + 80}, {x - 8, y}, {x + 64, y}}
	for k, piece := range fr {
		if piece != nil {
			im.Blit(piece, at[k][0], at[k][1])
		}
	}
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
	if info.Skirmish != nil {
		drawSkirmishMarkerText(c, info.Skirmish, v)
	} else {
		ab.drawPlates(c, b, v)
	}
	drawBattleDate(c, info.Date, info.Calendar)

	// 左欄：郡名一個字一列（原版 32×32 的雙倍字，`0x2181d`）、州名、日數、天氣。
	// 拉丁字母的郡名放不進兩格 32×32，退成一倍字直排（`docs/spec/014` 的
	// 規則，remake 差異）。
	ink := assets.EGAPalette[14]
	nameScale := assets.BattleNameScale
	if artHasLatin(info.Prefecture) {
		nameScale = 1
	}
	for i, r := range []rune(info.Prefecture) {
		if i >= 2 {
			break
		}
		c.DrawRuneScaledPx(assets.BattleNameX, assets.BattleNameY+i*assets.BattleNameStep,
			r, ink, nameScale, nameScale)
	}
	c.DrawTextPx(assets.BattleNameX, assets.BattleProvinceY, info.Province,
		assets.EGAPalette[15])
	c.DrawTextPx(assets.BattleNameX, assets.BattleNumberY, fmt.Sprintf("%2d", info.ID),
		assets.EGAPalette[13])
	y0, _ := assets.BattleLeftBox(2)
	c.DrawTextPx(assets.BattleNameX, y0, WeatherName(b.Weather), assets.EGAPalette[15])
	hour := battle.SkirmishFirstHour
	if info.Skirmish != nil {
		hour = info.Skirmish.Hour
	}
	drawBattleDayBox(c, b.Day, hour)

	// 兩個軍力面板（`0x22c94`，位置在 `assets.BattleLayout.NameX`／`TextX`）；
	// 對戰子畫面裡換成兩支部隊的面板（`0x320a6`）。
	l := assets.BattleLayoutFor(b.Field.Narrow())
	for i, side := range artBattleSides {
		units, men := 0, 0
		leaders := 0
		for _, u := range b.Units {
			if u.Side == side && u.Alive() {
				units++
				men += u.Soldiers()
				leaders += len(u.Leaders)
			}
		}
		ink := assets.EGAPalette[assets.BattlePanelInks[i]]
		commander, lord := info.Commander[i], info.Lord[i]
		var lines []string
		if info.Units != nil {
			// 部隊面板：第 0 槽那一位的名字，字色照那支部隊的軍力
			// （`DS:0x796a`）；六行是君主、軍力名、（空）、隊伍名與將數、
			// 兵數、（空）（`DS:0x889c`–`0x88be`）。第 0 槽空了整塊只剩藍底。
			u := info.Units[i]
			if u.Unit == nil || u.Unit.Head() == nil {
				continue
			}
			side = u.Unit.Side
			ink = assets.EGAPalette[assets.FlagPlateColour[side.OriginalIndex()]]
			commander, lord = u.Unit.Head().Name, u.Lord
			lines = []string{
				tf("bat.armyOf", battlePaddedName(lord)),
				tf("bat.sideLine", SideName(side)),
				"",
				tf("bat.panelUnit", u.Unit.Formation.Label(), u.Unit.LeaderCount()),
				tf("bat.panelMen", u.Unit.Soldiers()),
			}
		}
		// 統帥名：32×32 直排在肖像旁邊（`0x22fd4`），兩字名從 y+16 起、
		// 三字名從 y+0 起。拉丁字母的名字直排讀不下去，不畫（第一行的
		// 「X's army」已經有名字；remake 差異）。
		bigName := false
		if name := []rune(commander); artAllWide(commander) && len(name) <= 3 {
			bigName = true
			y := l.PanelY[i]
			if len(name) == 2 {
				y += 16
			}
			for k, r := range name {
				c.DrawRuneScaledPx(l.NameX(i), y+k*32, r, ink, 2, 2)
			}
		}
		// 五行資料：原版的第一行是 6 個位元組的姓名欄接「軍」（兩字名前後
		// 各一個空白），第三行的軍數是國字（`DS:0x7927`），兵是存的百位
		// 接「00」。
		if lord == "" {
			lord = commander
		}
		if lines == nil {
			lines = []string{
				tf("bat.armyOf", battlePaddedName(lord)),
				tf("bat.sideLine", SideName(side)),
				tf("bat.forcesLine", battleUnitsNumeral(units), leaders),
				tf("bat.menLine", men/100*100),
				tf("bat.goldLine", b.Gold[side]),
				tf("bat.riceLine", b.Rice[side]),
			}
		}
		// 沒畫統帥名（拉丁字母）的時候，那 32 像素讓給資料，文字區從
		// 統帥名的位置起算、寬 96。
		x, width := l.TextX(i), sideTextW
		if !bigName {
			x = min(l.TextX(i), l.NameX(i))
			width = sideTextW + 32
		}
		if rows, small := sidePanelLayout(c, lines, width); small {
			for k, s := range rows {
				c.DrawSmallTextPx(x, l.LineY(i, 0)+k*SmallH, s, ink)
			}
		} else {
			for k, s := range rows {
				c.DrawTextPx(x, l.LineY(i, k), s, ink)
			}
		}
	}

	// 查看（`0x284a2`）：第三塊面板換成那一位的肖像、名字與六行資料，
	// 字黃（14）從面板左上角起一行 16；名字 32×32 直排在 x 512，字色照
	// 那一側（`DS:0x796a`）。分頁照樣蓋在場地上（remake 差異：原版是
	// 逐位翻看，remake 一頁列出整支部隊）。
	if ins := info.Inspect; ins != nil && ins.Leader != nil {
		x0, y0, _, _ := l.Panel(2)
		yellow := assets.EGAPalette[assets.BattleOrderInk]
		for k, s := range []string{
			tf("bat.insp.stamina", ins.Leader.Stamina),
			tf("bat.insp.intel", ins.Leader.Intel),
			tf("bat.insp.war", ins.Leader.War),
			tf("bat.insp.training", ins.Leader.Training),
			tf("bat.insp.arms", ins.Leader.Arms),
			tf("bat.insp.men", ins.Leader.Soldiers),
		} {
			c.DrawTextPx(x0, y0+k*CellH, cells.Truncate(s, (inspectNameX-x0)/CellW), yellow)
		}
		ink := assets.EGAPalette[assets.FlagPlateColour[ins.Side.OriginalIndex()]]
		if name := []rune(ins.Leader.Name); artAllWide(ins.Leader.Name) && len(name) <= 3 {
			y := y0
			if len(name) == 2 {
				y += 16
			}
			for k, r := range name {
				c.DrawRuneScaledPx(inspectNameX, y+k*32, r, ink, 2, 2)
			}
		} else {
			// 拉丁字母的名字 32×32 直排讀不下去：小字折進那 32 像素寬的
			// 一欄，一行 5 個字母、最多 6 行（remake 差異）。
			cols := (inspectFaceX - 8 - inspectNameX) / SmallW
			for k, line := range cells.Wrap(ins.Leader.Name, cols) {
				if k*SmallH >= assets.BattlePanelH {
					break
				}
				c.DrawSmallTextPx(inspectNameX, y0+k*SmallH, cells.Truncate(line, cols), ink)
			}
		}
		if len(v.Page) > 0 {
			drawOverlay(c, battlePageX0, battlePageY0, battlePageX1, battlePageY1,
				v.PageTitle, v.Page, t("hint.page"), v.PageTop)
		}
		return
	}

	// 指令面板：原版的三行三列 ＋ 提示，字從面板的左上角 (448,268) 起
	// （文字視窗的原點；基準畫面上「1.移動」的墨從 (449,269) 起）。
	// **選了用計、交戰、方向或紮營之後，那三行換成該選的選項**——先前
	// 只畫選單標題，六種計謀、交戰方式都看不到，玩家只能照手冊背編號
	// （`docs/spec/014` §7）。
	ordX, ordY := l.PanelX[2], l.PanelY[2]
	if v.Window != "" {
		// 原版的文字視窗：面板清成青 3，逐列寫（`BattleWindowLines`）。英日文排不進
		// 6 列就改小字（一行 28 字、8 列，remake 差異同下面的選單）。
		c.FillRect(ordX, ordY, ordX+assets.BattlePanelW, ordY+assets.BattlePanelH, assets.EGAPalette[battleWindowPaper])
		ink := assets.EGAPalette[battleWindowInk]
		lines := BattleWindowLines(v.Window, BattleWindowCols, 1<<16)
		if len(lines) <= BattleWindowRows {
			for k, s := range lines {
				c.DrawTextPx(ordX, ordY+k*CellH, s, ink)
			}
			// 游標接在最後一行字後面（休息確認之後是 (568,268)）。
			if x, y, ok := cursorAfterLines(ordX, ordY, CellH, lines); ok {
				drawInputCursor(c, ab.cursor, v.Input, x, y)
			}
		} else if small := BattleWindowLines(v.Window, assets.BattlePanelW/SmallW, 1<<16); len(small) <= assets.BattlePanelH/SmallH {
			for k, s := range small {
				c.DrawSmallTextPx(ordX, ordY+k*SmallH, s, ink)
			}
		} else {
			rows := BattleWindowLines(v.Window, BattleWindowCols, BattleWindowRows)
			for k, s := range rows {
				c.DrawTextPx(ordX, ordY+k*CellH, s, ink)
			}
			if x, y, ok := cursorAfterLines(ordX, ordY, CellH, rows); ok {
				drawInputCursor(c, ab.cursor, v.Input, x, y)
			}
		}
		if len(v.Page) > 0 {
			drawOverlay(c, battlePageX0, battlePageY0, battlePageX1, battlePageY1,
				v.PageTitle, v.Page, t("hint.page"), v.PageTop)
		}
		return
	}
	ord := assets.EGAPalette[assets.BattleOrderInk]
	w := assets.BattlePanelW / CellW
	opts := BattleCommandLines()
	if len(v.Items) > 0 {
		opts = v.Items
	}
	small := battleSmallLayout(c, opts, v.Menu, v.Prompt)
	switch {
	case len(opts) <= battleOptRows:
		for k, s := range opts {
			c.DrawTextPx(ordX, ordY+k*CellH, cells.Truncate(s, w), ord)
		}
	case small != nil:
		// 英文九個指令原尺寸要五行以上：**整塊改用小字**留在面板裡
		//（使用者裁定「文字允許縮小」，`docs/spec/014` §7）——選項、
		// 標題、提示都用小字，一行 28 字、面板放得下 8 行。
		cols := assets.BattlePanelW / SmallW
		for k, s := range small {
			ink := ord
			if k == len(small)-1 && v.Prompt != "" {
				ink = assets.EGAPalette[15]
			}
			c.DrawSmallTextPx(ordX, ordY+k*SmallH, cells.Truncate(s, cols), ink)
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
		x0, x1 := l.PanelX[2], l.PanelX[2]+assets.BattlePanelW
		y1 := l.PanelY[2] - 2
		y0 := y1 - len(opts)*CellH - 8
		c.FillRect(x0, y0, x1, y1, artInkPageBG)
		for k, s := range opts {
			c.DrawTextPx(ordX+4, y0+4+k*CellH, cells.Truncate(s, w-1), ord)
		}
	}
	row := battleOptRows
	if v.SkirmishActing != nil && len(opts) < battleOptRows {
		// 子畫面的選單（`0x2fb14`）：兩行選項之後緊接著「名字(餘步/移動力)」
		// 那一行，沒有標題。
		row = len(opts)
	}
	if v.Menu != "" && v.SkirmishActing == nil {
		c.DrawTextPx(ordX, ordY+row*CellH,
			cells.Truncate(v.Menu, assets.BattlePanelW/CellW), ord)
		row++
	}
	if v.Prompt != "" {
		c.DrawTextPx(ordX, ordY+row*CellH,
			cells.Truncate(v.Prompt, assets.BattlePanelW/CellW),
			assets.EGAPalette[15])
	}
	// 查看部隊那一頁蓋在戰場區上（面板上面那一整塊）。
	if len(v.Page) > 0 {
		drawOverlay(c, battlePageX0, battlePageY0, battlePageX1, battlePageY1,
			v.PageTitle, v.Page, t("hint.page"), v.PageTop)
	}
}

// sideTextW 是軍力面板上五行資料的寬：面板 176 扣掉肖像 80 與統帥名
// 32，64 像素（攻方 176–239、守方 256–319）。先前照整塊面板截在 21 格，
// 英文的「Main Attackers」就畫進隔壁那一塊面板——中文剛好都短，才沒露出來。
const sideTextW = assets.BattlePanelW - assets.BattleFrameW - 32

// battlePaddedName 照原版人物表的 6 位元組姓名欄排名字：兩字名前後各
// 一個空白（原版第一行因此是「 陳就 軍」），三字名剛好填滿；拉丁字母的
// 名字照原樣。
func battlePaddedName(name string) string {
	if artAllWide(name) && cells.Width(name) == 4 {
		return " " + name + " "
	}
	return name
}

// 左欄第四框（y 228–323，`0x219b5`–`0x21b38`）：**國字日數直排**——三格
// 各 32×16（`0x1d4e` 的 sx＝2、sy＝1，白 15）在 (8,228)／(8,244)／(8,260)，
// 位數表 `DS:0x78a8＋4×天` 指到 `DS:0x7924` 起的「　十一二…九」；「日」
// (8,276) 也是 32×16；時辰名 `DS:0x7946[((時＋1) mod 24) ÷ 2]` 16×16
// 淺青 11 在 (8,292)、「時」16×32 在 (24,292)、時數 `%2d` 在 (8,308)。
// 時刻 `es:0x31a6` 進戰場設 6（`0x2055c`），對戰子畫面裡每時刻 +1、結束
// 又設回 6（`0x2e67a`），所以主戰場上永遠是「卯時 6」。
//
// 英文沒有國字與地支：日數用阿拉伯數字一格 16×16、「Day」「Hour」照原
// 尺寸（remake 差異）。
const (
	battleDayBoxX        = assets.BattleLeftBoxX0
	battleDayNumeralY    = 228
	battleDaySignY       = 276
	battleHourY          = 292
	battleHourNumberY    = 308
	battleDayNumeralStep = 16
)

// battleDayNumerals 照原版的位數表把第幾天拆成三格國字（空的是 ""）：
// 1–9 在中格、10 是「十」、11–19「十」＋個位、20「二十」、21–29 三格全用、
// 30「三十」；超過 30 原版讀到表外，remake 夾在 30。
func battleDayNumerals(day int) [3]string {
	numerals := []string{"十", "一", "二", "三", "四", "五", "六", "七", "八", "九"}
	n := func(d int) string { return numerals[d%10] }
	switch {
	case day <= 0:
		return [3]string{}
	case day < 10:
		return [3]string{"", n(day), ""}
	case day == 10:
		return [3]string{"", "十", ""}
	case day < 20:
		return [3]string{"", "十", n(day)}
	case day == 20:
		return [3]string{"", "二", "十"}
	case day < 30:
		return [3]string{"二", "十", n(day)}
	default:
		return [3]string{"", "三", "十"}
	}
}

// drawBattleDayBox 畫左欄第四框：日數、「日」、時辰、「時」、時數。
func drawBattleDayBox(c *Canvas, day, hour int) {
	white, cyan := assets.EGAPalette[15], assets.EGAPalette[11]
	if sign := t("bat.daySign"); artAllWide(sign) {
		for k, r := range battleDayNumerals(day) {
			if r != "" {
				c.DrawRuneScaledPx(battleDayBoxX, battleDayNumeralY+k*battleDayNumeralStep,
					[]rune(r)[0], white, 2, 1)
			}
		}
		c.DrawRuneScaledPx(battleDayBoxX, battleDaySignY, []rune(sign)[0], white, 2, 1)
		branches := []rune(t("bat.hourBranches"))
		if i := ((hour + 1) % 24) / 2; i < len(branches) {
			c.DrawRuneWidePx(battleDayBoxX, battleHourY, branches[i], cyan, 1)
		}
		c.DrawRuneScaledPx(battleDayBoxX+CellW*2, battleHourY, []rune(t("bat.hourSign"))[0], cyan, 1, 2)
	} else {
		c.DrawTextPx(battleDayBoxX, battleDayNumeralY+battleDayNumeralStep, fmt.Sprintf("%2d", day), white)
		c.DrawTextPx(battleDayBoxX, battleDaySignY, cells.Truncate(sign, 4), white)
		c.DrawTextPx(battleDayBoxX, battleHourY, cells.Truncate(t("bat.hourSign"), 4), cyan)
	}
	c.DrawTextPx(battleDayBoxX, battleHourNumberY, fmt.Sprintf("%2d", hour), cyan)
}

// battleUnitsNumeral 是第三行的軍數：原版查 `DS:0x7927` 的國字表
// （十、一…九），譯文的格式裡沒有「軍」字就用阿拉伯數字。
func battleUnitsNumeral(n int) string {
	if !strings.Contains(t("bat.forcesLine"), "軍") {
		return strconv.Itoa(n)
	}
	numerals := []string{"十", "一", "二", "三", "四", "五", "六", "七", "八", "九"}
	if n >= 0 && n < len(numerals) {
		return numerals[n]
	}
	return strconv.Itoa(n)
}

// sidePanelLayout 決定軍力面板的五行怎麼畫：原尺寸放得下（11 格）就照畫；
// 放不下而且都是 ASCII 就整塊改小字，長的一行折成兩行（一行 15 字、
// 面板放得下 8 行）；都不行才截。回傳要畫的行與用不用小字。
func sidePanelLayout(c *Canvas, lines []string, width int) ([]string, bool) {
	cols := width / CellW
	fit := true
	for _, l := range lines {
		if cells.Width(l) > cols {
			fit = false
		}
	}
	if fit {
		return lines, false
	}
	scols, srows := width/SmallW, (assets.BattlePanelH-8)/SmallH
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

	// 分頁蓋在三個面板上方那一整塊：左右對齊寬版面面板的外緣（64–624），
	// 上緣留 4 像素，下緣停在指令面板上面（兩種版面的指令面板都在 y 268）。
	battlePageX0, battlePageY0 = 64, 4
	battlePageX1, battlePageY1 = 624, 268 - 4
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

// 郡地理誌（查看 5，`0x185a6`，`L0`＋`L1`、Issue #60）：原版把主戰場的
// 合成常式（`0x2020:0x2244(郡, −1)`——第二個參數非零只畫第三塊面板）畫在
// 顯示記憶體的**第二頁**再切過去：底紋、上下花邊、場地邊框與圖塊、第三塊
// 面板填藍；沒有左欄、沒有軍力面板、年月那一行也不寫。通道那幾格
// （標記高四位 0–9）由 `0x18742(郡, 0)` 在格子左上角加 (16,15) 印鄰郡的
// 編號 `%d`（黃 14、底 7，`0x104e`）；最後主事者在第三塊面板說 356
// 「此乃本郡之地理圖誌」，等鍵回主畫面。
const (
	atlasLabelDX, atlasLabelDY = 16, 15
	// 編號的字色與底色。
	atlasLabelInk, atlasLabelBG = 14, 7
)

// DrawArtAtlas 畫郡地理誌那一張（不含對白，對白由呼叫端用 `DrawBubble`
// 疊在第三塊面板上）。field 是州郡記錄的 120 個位元組，fld 是解出來的戰場
// （通道的格子與鄰郡編號從它取）。
func DrawArtAtlas(c *Canvas, ab *ArtBattle, field []byte, fld *battle.Field) {
	var im *assets.Image
	if ab.bg != nil {
		im = ab.bg.Clone()
	} else {
		im = &assets.Image{W: assets.ScreenW, H: assets.ScreenH,
			Pix: make([]byte, assets.ScreenW*assets.ScreenH)}
	}
	if ab.top != nil {
		im.Blit(ab.top, 0, 0)
	}
	if ab.bottom != nil {
		im.Blit(ab.bottom, 0, assets.MapBorderBottomY)
	}
	l := assets.BattleLayoutFor(fld.Narrow())
	im.FieldEdges(l)
	im.BlitField(ab.tiles, field)
	im.FieldLines(l)
	x0, y0, x1, y1 := l.Panel(2)
	im.FillRect(x0, y0, assets.BattlePanelW, assets.BattlePanelH, assets.BattlePanelPaper)
	im.BevelBox(x0, y0, x1, y1)
	draw.Draw(c.Img, image.Rect(0, 0, assets.ScreenW, assets.ScreenH),
		im.RGBA(), image.Point{}, draw.Src)
	// 通道格子上的鄰郡編號。
	ink, bg := assets.EGAPalette[atlasLabelInk], assets.EGAPalette[atlasLabelBG]
	for n, hs := range fld.Gates {
		label := strconv.Itoa(n)
		for _, h := range hs {
			col, row := battle.ToOffset(h)
			x, y := assets.FieldCell(col, row)
			x, y = x+atlasLabelDX, y+atlasLabelDY
			c.FillRect(x, y, x+len(label)*CellW, y+CellH, bg)
			c.DrawTextPx(x, y, label, ink)
		}
	}
}
