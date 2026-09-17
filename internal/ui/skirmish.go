package ui

import (
	"fmt"

	"github.com/wicanr2/softworld_san1_remake/internal/assets"
	"github.com/wicanr2/softworld_san1_remake/internal/battle"
)

// 對戰子畫面（`0x2deb0`，`docs/spec/005` §8「對戰子畫面」）：主戰場的外框、
// 左欄與面板照舊，場地換成 12×10 的版型子地圖（`0x2e700`），部隊的旗幟
// 換成每一位將領一個標記（`0x2e796`）。
//
// 標記在格子左上角 (x, y)（與地形圖塊同一套座標，奇數欄往下 16）：
//
//	名字（人物表的 6 個位元組）  (x, y)       字色 軍力色、底 0
//	「攻」／「守」             (x, y+15)    字色 0、底 軍力色
//	兵 `%4d`                  (x+16, y+15) 字色 軍力色 & 7、底 軍力色
//	右緣一條黑直線            x+47，y..y+31
//
// 軍力色是 `DS:0x796a[軍力]`（10／11／12／13）。

// skirmishFieldBytes 把子地圖換成 `BlitField` 吃的 120 個位元組（列 × 12 ＋ 欄）。
func skirmishFieldBytes(s *battle.Skirmish) []byte {
	out := make([]byte, battle.SkirmishRows*battle.SkirmishCols)
	for r := range s.Map {
		for c, v := range s.Map[r] {
			b := byte(v)
			if b&0x0F == 0x0F {
				b = 0xFF // 地形碼 15 不畫（`0x2e769`）
			}
			out[r*battle.SkirmishCols+c] = b
		}
	}
	return out
}

// skirmishMarkers 是要畫標記的將領：還在子地圖上的每一位，照原版畫的
// 順序（守方槽 0–9、攻方槽 0–9，`0x2e3be`–`0x2e473`）。
func skirmishMarkers(s *battle.Skirmish) []*battle.SkirmishGeneral {
	var out []*battle.SkirmishGeneral
	for side := range s.Gens {
		for _, g := range s.Gens[side] {
			if g != nil && !g.Gone {
				out = append(out, g)
			}
		}
	}
	return out
}

// skirmishColour 是那一位所屬軍力的顏色（`DS:0x796a`）。
func skirmishColour(g *battle.SkirmishGeneral) byte {
	army := g.Unit.Side.OriginalIndex()
	if army < 0 || army >= len(assets.FlagPlateColour) {
		return 15
	}
	return assets.FlagPlateColour[army]
}

// SkirmishMarkerW／H 是一個標記蓋住的範圍：名字一行加「攻 兵」一行，
// 右緣直線在 x+47。
const (
	SkirmishMarkerW = 48
	SkirmishMarkerH = 31
)

// drawSkirmishMarkers 畫標記的底色與右緣直線；字在 drawSkirmishMarkerText。
func drawSkirmishMarkers(im *assets.Image, s *battle.Skirmish, v BattleView) {
	for _, g := range skirmishMarkers(s) {
		x, y := assets.FieldCell(g.Col, g.Row)
		ink := skirmishColour(g)
		// 照原版印的順序蓋：「攻／守」、名字、兵、直線（`0x2e88d`–`0x2e944`）。
		// 名字那一行 16 高，會蓋掉「攻」那一格的第一列。
		im.FillRect(x, y+15, CellW*2, CellH, ink)
		im.FillRect(x, y, SkirmishMarkerW, CellH, 0)
		im.FillRect(x+CellW*2, y+15, CellW*4, CellH, ink)
		im.FillRect(x+SkirmishMarkerW-1, y, 1, SkirmishMarkerH+1, 0)
	}
	if g := v.SkirmishActing; g != nil && v.Blink {
		x, y := assets.FieldCell(g.Col, g.Row)
		im.XorRect(x, y, assets.TileW, assets.TileH, 0x0F)
	}
}

// drawSkirmishMarkerText 寫標記上的字。反白那一格的字色跟著取補數。
func drawSkirmishMarkerText(c *Canvas, s *battle.Skirmish, v BattleView) {
	for _, g := range skirmishMarkers(s) {
		x, y := assets.FieldCell(g.Col, g.Row)
		ink := skirmishColour(g)
		name, side, num := ink, byte(0), ink&7
		if g == v.SkirmishActing && v.Blink {
			name, side, num = name^0x0F, side^0x0F, num^0x0F
		}
		c.DrawTextPx(x, y, battlePaddedName(g.Leader.Name), assets.EGAPalette[name])
		mark := t("skm.defender")
		if g.Side == battle.SkirmishAttacker {
			mark = t("skm.attacker")
		}
		c.DrawTextPx(x, y+15, mark, assets.EGAPalette[side])
		c.DrawTextPx(x+CellW*2, y+15, fmt.Sprintf("%4d", g.Leader.Soldiers), assets.EGAPalette[num])
	}
}
