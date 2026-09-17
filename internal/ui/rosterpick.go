package ui

import (
	"fmt"

	"github.com/wicanr2/softworld_san1_remake/internal/assets"
	"github.com/wicanr2/softworld_san1_remake/internal/cells"
	"github.com/wicanr2/softworld_san1_remake/internal/game"
)

// RosterPick 是「挑一位將軍」那一格（`0x18024`，`docs/spec/014` §4.2）：右側面板清藍、
// 外框 `SIDEA`，表頭「號  姓名 .」與第三欄欄名，一頁十二列；提示與數字在下面板。
type RosterPick struct {
	// List 是整份名單（`game.PickRoster` 的順序，人物槽號）；Page 是這一頁第一列的索引。
	List []int
	Key  game.PickKey
	Page int

	// Multi 為真是多選清單（`0x18286`，調動軍隊）：外框 `SIDEE`，Marked[i] 為真的那一列
	// 在 (424, 84＋16i) 印白 15 的「*」（`0x18453`）。
	Multi  bool
	Marked []bool
}

// RosterPageRows 是一頁幾列（`0x181d3`：`cmp $0xc`）。
const RosterPageRows = 12

const (
	rosterX0, rosterY0 = 408, 36  // 清藍、拼外框的範圍 (408,36)–(631,291)
	rosterX1, rosterY1 = 631, 291 //
	rosterHeadX        = 440      // 「號  姓名 .」（`0x1810e`）
	rosterHeadY        = 62
	rosterColX         = 520 // 第三欄欄名（`0x18138`）與「現任」的身分名（`0x17ff9`）
	rosterRowY         = 84  // 第 i 列 y ＝ 84 ＋ 16i（`0x17d75`）
	rosterEmptyX       = 424 // 「沒有任何將軍」（`0x18094`）
	rosterEmptyY       = 52
	rosterBG           = 1
	rosterHeadInk      = 15
	rosterColInk       = 14
	rosterRowInk       = 13
	rosterNameInk      = 14
)

// RosterRange 是這一頁數字輸入的上下限（`0x1822c`：頁首＋1 到 min(頁首＋12, 筆數)）。
func RosterRange(p *RosterPick) (lo, hi int) {
	lo = p.Page + 1
	hi = min(p.Page+RosterPageRows, len(p.List))
	return lo, hi
}

// DrawRosterPick 畫右側面板的名單。
func DrawRosterPick(c *Canvas, a *ArtScreen, g *game.State, p *RosterPick) {
	bg := assets.EGAPalette[rosterBG]
	c.FillRect(rosterX0, rosterY0, rosterX1+1, rosterY1+1, bg)
	if a != nil && a.havePanel {
		box := a.pickBox
		if p.Multi {
			box = a.multiBox
		}
		drawSideFrame(c, box, rosterX0, rosterY0, rosterX1-rosterX0+1, rosterY1-rosterY0+1)
	}
	if len(p.List) == 0 {
		c.DrawTextPx(rosterEmptyX, rosterEmptyY, t("pick.none"), assets.EGAPalette[rosterHeadInk])
		return
	}
	c.DrawTextPx(rosterHeadX, rosterHeadY, t("pick.head"), assets.EGAPalette[rosterHeadInk])
	c.DrawTextPx(rosterColX, rosterHeadY, t(fmt.Sprintf("pick.col%d", int(p.Key))), assets.EGAPalette[rosterColInk])
	for i := 0; i < RosterPageRows; i++ {
		n := p.Page + i
		if n >= len(p.List) {
			break
		}
		x := g.General(p.List[n])
		if x == nil {
			continue
		}
		y := rosterRowY + i*CellH
		if p.Multi && n < len(p.Marked) && p.Marked[n] {
			c.DrawTextPx(rosterEmptyX, y, "*", assets.EGAPalette[rosterHeadInk])
		}
		lead, name, tail := RosterRow(x, n, p.Key)
		px := rosterHeadX + c.DrawTextPx(rosterHeadX, y, lead, assets.EGAPalette[rosterRowInk])
		px += c.DrawTextPx(px, y, name, assets.EGAPalette[rosterNameInk])
		c.DrawTextPx(px, y, tail, assets.EGAPalette[rosterRowInk])
		if p.Key == game.PickByStatus || p.Key > game.PickByBoth {
			ink := 15
			if int(x.Status) < len(cardStatusInk) {
				ink = cardStatusInk[x.Status]
			}
			c.DrawTextPx(rosterColX, y, cardStatusName(x.Status), assets.EGAPalette[ink])
		}
	}
}

// RosterRow 是一列的三段字（`0x17d0e` 的格式 `%2d.\x01\x0e%s\x03.%4d`）：
// 序號與點字色 13、姓名欄（6 格）字色 14、其後字色 13。
func RosterRow(x *game.General, n int, key game.PickKey) (lead, name, tail string) {
	lead = fmt.Sprintf("%2d.", n+1)
	name = NameField(PersonName(x.Name))
	if cells.Width(name) > 6 {
		name = cells.Truncate(name, 6)
	}
	switch key {
	case game.PickByIntel:
		tail = fmt.Sprintf(".%4d", int8(x.Intel))
	case game.PickByWar:
		tail = fmt.Sprintf(".%4d", int8(x.War))
	case game.PickByCharm:
		tail = fmt.Sprintf(".%4d", int8(x.Charm))
	case game.PickByLoyalty:
		if x.Status == 0 {
			tail = ". --"
		} else {
			tail = fmt.Sprintf(".%4d", int8(x.Loyalty))
		}
	case game.PickByStamina:
		tail = fmt.Sprintf(".%4d", int8(x.Stamina))
	case game.PickByAge:
		tail = fmt.Sprintf(".%4d", int8(x.Age))
	case game.PickBySoldiers:
		tail = fmt.Sprintf(".%4d", int16(x.Soldiers))
	case game.PickByArms:
		tail = fmt.Sprintf(".%4d", int8(x.Arms))
	case game.PickByBoth:
		tail = fmt.Sprintf(".%4d.%3d", int16(x.Soldiers), int8(x.Arms))
	default:
		tail = "."
	}
	return lead, name, tail
}
