package ui

import (
	"fmt"
	"image/color"

	"github.com/wicanr2/softworld_san1_remake/internal/assets"
	"github.com/wicanr2/softworld_san1_remake/internal/cells"
	"github.com/wicanr2/softworld_san1_remake/internal/game"
	"github.com/wicanr2/softworld_san1_remake/internal/i18n"
	"github.com/wicanr2/softworld_san1_remake/internal/state"
)

// 人物資料卡（原版 `0xf874(人物)`，`docs/spec/005` §9.2，`L0`、`[base]`）：
// 右側面板整塊清成灰、肖像加框、名字 32×32、一欄一欄的資料。呼叫端有
// 查看→武將、挖角與計略的目標、戰後處置、示範模式的月底；開新局選君主
// 那一刻也畫。
//
//	清底      (408,36)–(631,291) 灰 7，外框拼 `SIDEC`（樣式 7）
//	肖像      (536,68)，框 `FBRC` 四塊：上 (528,60)、下 (528,148)、左 (528,68)、右 (600,68)
//	名字      (424,52) 32×32 黃 14，畫的是 6 位元組的姓名欄（兩字名前後各一空白）
//	籍貫      (424,84)  `%s%s人氏` 淺青 11：州名 ＋ 出身郡的郡名（4 位元組，不補空白）
//	身分      (424,100) 君主「  現為君主  」（前後各兩個半形空白）；在野／隱居／囚禁／....／死亡（身分
//	          8–12）只畫身分名，而且**從 456 起**（`0xf9bd`）；其餘 `任%s%s`
//	          （君主的姓名欄 ＋ 身分名）；字色查 `DS:0x5924[身分]`
//	忠心度    (424,116) `忠心度   %3d` 淺紅 12——只有走 `任%s%s` 那一支的（身分 1–3、5–7）
//	年齡      (424,132) `現年%2d歲` 淺綠 10
//	体能      (424,164) `体能 %3d    %s  %s軍` 藍 9（職位、兵種）
//	謀略      (424,180) `謀略 %3d    兵士數  %4d` 藍 9（兵士數是存的原值，不除百）
//	戰力      (424,196) `戰力 %3d    訓練度   %3d` 藍 9
//	魅力      (424,212) `魅力 %3d    武裝度   %3d` 藍 9
const (
	cardX0, cardY0, cardX1, cardY1 = 408, 36, 631, 291
	cardPaper                      = 7
	cardNameX, cardNameY           = 424, 52
	cardFaceX, cardFaceY           = 536, 68
	cardTextX                      = 424
)

// cardStatusInk 是身分那一行的字色表（`DS:0x5924`，13 格，索引是身分）。
var cardStatusInk = [13]int{15, 10, 11, 14, 15, 15, 15, 15, 9, 15, 12, 15, 15}

// cardStatusName 是原版身分表（`DS:0x58d4`）在卡片上用到的字。
func cardStatusName(s state.Status) string {
	switch s {
	case state.StatusLord:
		return t("status.lord")
	case state.StatusChief:
		return t("status.chief")
	case state.StatusGovernor:
		return t("status.governor")
	case state.StatusOfficer:
		return t("card.officer")
	case state.StatusAvailable:
		return t("status.free")
	case state.StatusIdle:
		return t("card.hidden")
	case state.StatusStranded:
		return t("card.captive")
	case state.StatusUnborn:
		return t("card.unborn")
	case state.StatusFallen:
		return t("card.dead")
	}
	return fmt.Sprintf("%d", int(s))
}

// paddedName 照原版人物表的 6 位元組姓名欄排名字：兩字名前後各一個空白。
func paddedName(name string) string {
	if artAllWide(name) && cells.Width(name) == 4 {
		return " " + name + " "
	}
	return name
}

// cardLine 是卡片上的一行字：落點、槽寬（半形格）、字色、內容。
type cardLine struct {
	x, y, cols int
	ink        int
	text       string
}

// 文字區：424–623（右邊 8 像素是外框的邊）。肖像框佔 528–607、60–155，
// 所以 y 156 之前的行只到 527。
const (
	cardCols     = (624 - cardTextX) / CellW
	cardNarrow   = (cardFaceX - 8 - cardTextX) / CellW
	cardFaceBotY = cardFaceY + 80 + 8
)

// cardLines 排出一位人物的資料行（名字另外畫）。**沒有一行會超過它的槽**
// （`TestCardFitsEveryLanguage`）。中文照原版一行一行擺；譯文塞不下的
// 三處各有一條退路（`docs/spec/014` 的譯文槽位規則，記為 remake 差異）：
//
//   - 籍貫放不下 → 州名去掉「州」再轉寫（與主畫面的 `artProvinceIn` 同一招）。
//   - `任%s%s` 放不下 → 拆成兩行：身分（`card.servingStatus`）與君主名，
//     忠心度、年齡各往下挪一列（116→132、132→148；原版 148 是空行）。
//   - 身分 8–12 的身分名從 456 起放不下 → 從 424 起。
//   - 体能那一行放不下 → 兵種挪到魅力下面那一列（228，原版是空行）。
func cardLines(g *game.State, x *game.General) []cardLine {
	narrow := func(x int) int { return cardNarrow - (x-cardTextX)/CellW }
	var out []cardLine
	if p := g.Prefecture(x.Origin); p != nil {
		name := PlaceName(p.Name)
		origin := tf("card.origin", PlaceName(state.ProvinceName(int(p.Province))), name)
		if cells.Width(origin) > cardNarrow {
			room := cardNarrow - cells.Width(tf("card.origin", "", name))
			origin = tf("card.origin", artProvinceIn(int(p.Province), room), name)
		}
		out = append(out, cardLine{cardTextX, 84, cardNarrow, 11, origin})
	}
	status := int(x.Status)
	inkStatus := 15
	if status >= 0 && status < len(cardStatusInk) {
		inkStatus = cardStatusInk[status]
	}
	loyaltyY, ageY := 116, 132
	switch {
	case x.Status == state.StatusLord || status == 4:
		out = append(out, cardLine{cardTextX, 100, cardNarrow, inkStatus, t("card.isLord")})
		loyaltyY = 0
	case status >= 8 && status <= 12:
		// 原版從 456 起（兩個全形字的身分名靠中間）；譯名長到放不下就從 424 起。
		sx, name := cardTextX+2*CellW*2, cardStatusName(x.Status)
		if cells.Width(name) > narrow(sx) {
			sx = cardTextX
		}
		out = append(out, cardLine{sx, 100, narrow(sx), inkStatus, name})
		loyaltyY = 0
	default:
		lord := ""
		if l := g.Lord(x.Faction); l != nil {
			lord = PersonName(l.Name)
			// 原版印的是 6 位元組的姓名欄（兩字名前後各一個空白）；
			// 譯文的句型自己帶空白，不再補。
			if i18n.Current == i18n.ZhHant {
				lord = paddedName(lord)
			}
		}
		if line := tf("card.serving", lord, cardStatusName(x.Status)); cells.Width(line) <= cardNarrow {
			out = append(out, cardLine{cardTextX, 100, cardNarrow, inkStatus, line})
		} else {
			out = append(out,
				cardLine{cardTextX, 100, cardNarrow, inkStatus, tf("card.servingStatus", cardStatusName(x.Status))},
				cardLine{cardTextX, 116, cardNarrow, inkStatus, lord})
			loyaltyY, ageY = 132, 148
		}
	}
	if loyaltyY > 0 {
		out = append(out, cardLine{cardTextX, loyaltyY, cardNarrow, 12, tf("card.loyalty", x.Loyalty)})
	}
	out = append(out, cardLine{cardTextX, ageY, cardNarrow, 10, tf("card.age", x.SignedAge())})
	body := tf("card.body", x.Stamina, RankName(x.Rank), TroopName(x.Troop))
	var unit *cardLine
	if cells.Width(body) > cardCols {
		body = tf("card.bodyRank", x.Stamina, RankName(x.Rank))
		unit = &cardLine{cardTextX, 228, cardCols, 9, tf("card.unit", TroopName(x.Troop))}
	}
	out = append(out,
		cardLine{cardTextX, 164, cardCols, 9, body},
		cardLine{cardTextX, 180, cardCols, 9, tf("card.intel", x.Intel, x.Soldiers)},
		cardLine{cardTextX, 196, cardCols, 9, tf("card.war", x.War, x.Training)},
		cardLine{cardTextX, 212, cardCols, 9, tf("card.charm", x.Charm, x.Arms)},
	)
	if unit != nil {
		out = append(out, *unit)
	}
	return out
}

// DrawPersonCard 在畫布上畫一張人物資料卡。肖像與框從 `a` 取；`a` 為 nil
// 時只畫字。
func DrawPersonCard(c *Canvas, a *ArtScreen, g *game.State, index int) {
	x := g.General(index)
	if x == nil {
		return
	}
	c.FillRect(cardX0, cardY0, cardX1+1, cardY1+1, assets.EGAPalette[cardPaper])
	if a != nil {
		// 外框：原版的 `0x1058:0x262c` 在清好的矩形上拼 `SIDEC`（樣式 7）。
		if a.havePanel {
			drawSideFrame(c, a.cardPanel, cardX0, cardY0, cardX1-cardX0+1, cardY1-cardY0+1)
		}
		if face := a.Portrait(int(x.Portrait)); face != nil {
			drawImageAt(c, face, cardFaceX, cardFaceY)
		}
		if a.havePanel {
			fx, fy := cardFaceX-8, cardFaceY-8
			drawImageAt(c, a.cardFrame[0], fx, fy)
			drawImageAt(c, a.cardFrame[1], fx, cardFaceY+80)
			drawImageAt(c, a.cardFrame[2], fx, cardFaceY)
			drawImageAt(c, a.cardFrame[3], fx+72, cardFaceY)
		}
	}
	ink := func(n int) color.RGBA { return assets.EGAPalette[n&15] }
	// 名字：中文照原版 32×32 畫 6 位元組的姓名欄（兩字名前後各一個空白，
	// 所以從 440 起）；譯名（拼音）改一倍字垂直置中，槽位到肖像框前。
	if name := PersonName(x.Name); artAllWide(name) {
		px := cardNameX
		for _, r := range paddedName(name) {
			px += c.DrawRuneScaledPx(px, cardNameY, r, ink(14), 2, 2)
		}
	} else {
		artBigName(c, cardNameX, cardNameY, cardFaceX-8-cardNameX, name, ink(14))
	}
	for _, l := range cardLines(g, x) {
		c.DrawTextPx(l.x, l.y, cells.Truncate(l.text, l.cols), ink(l.ink))
	}
}

// drawSideFrame 照 `assets.Image.DrawPanel` 的拼法把外框拼到畫布上：
// 四個角 16×16、四條邊 8×8 平鋪。
func drawSideFrame(c *Canvas, f assets.SideFrame, x, y, w, h int) {
	const cn, e = 16, 8
	for _, pt := range [][2]int{{x, y}, {x + w - cn, y}, {x, y + h - cn}, {x + w - cn, y + h - cn}} {
		drawImageAt(c, f.Corner, pt[0], pt[1])
	}
	for xx := x + cn; xx+e <= x+w-cn; xx += e {
		drawImageAt(c, f.Edge, xx, y)
		drawImageAt(c, f.Edge, xx, y+h-e)
	}
	for yy := y + cn; yy+e <= y+h-cn; yy += e {
		drawImageAt(c, f.Edge, x, yy)
		drawImageAt(c, f.Edge, x+w-e, yy)
	}
}
