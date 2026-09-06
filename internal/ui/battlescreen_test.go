package ui

import (
	"strings"
	"testing"

	"github.com/wicanr2/softworld_san1_remake/internal/battle"
)

func testBattle(t *testing.T) *battle.Battle {
	t.Helper()
	var att, def []battle.Leader
	for i := 0; i < 8; i++ {
		att = append(att, battle.Leader{Index: i, Name: "攻將", War: uint8(90 - i),
			Intel: 70, Stamina: 100, Charm: 50, Soldiers: 3000, Training: 60,
			Arms: 60, Troop: battle.TroopLand})
	}
	for i := 0; i < 6; i++ {
		def = append(def, battle.Leader{Index: 100 + i, Name: "守將", War: uint8(80 - i),
			Intel: 80, Stamina: 100, Charm: 50, Soldiers: 2500, Training: 60,
			Arms: 60, Troop: battle.TroopLand})
	}
	return battle.New(battle.Setup{
		Field:     battle.Generate(battle.Params{Prefecture: 15, Neighbours: []int{14, 16}, LandValue: 60}),
		Weather:   battle.Windy,
		Seed:      7,
		Attackers: att, Defenders: def, FromGate: 14,
		AttackerGold: 3000, AttackerRice: 9000,
		DefenderGold: 2000, DefenderRice: 8000,
	})
}

// TestDrawBattleFillsTheField 釘住戰場畫得出來、每一格都有東西。
//
// **空白的格子在畫面上與「還沒畫」長得一模一樣。** 逐格數墨才問得到
// 「這一張真的畫完了嗎」。
func TestDrawBattleFillsTheField(t *testing.T) {
	face := testFace(t)
	c := NewCanvas(Cols, Rows, face)
	b := testBattle(t)
	DrawBattle(c, b, BattleView{})
	blank := 0
	for y := 0; y < b.Field.H; y++ {
		row := fieldRow + y
		if row >= Rows-1 {
			break
		}
		for x := 0; x < b.Field.W; x++ {
			col := fieldCol + x*fieldStep + y%2
			if col+fieldStep > sideCol {
				break
			}
			if c.InkAt(col, row, ColBG) == 0 {
				blank++
			}
		}
	}
	if blank > 0 {
		t.Errorf("戰場上有 %d 格是空白的", blank)
	}
	if len(c.Missing) > 0 {
		t.Errorf("有畫不出來的字：%v", c.Missing)
	}
}

// TestBattleFieldFitsTheCanvas 釘住 21×15 的戰場放得進 80×25 的畫布。
func TestBattleFieldFitsTheCanvas(t *testing.T) {
	// 最寬的一列：奇數列右移一格。
	right := fieldCol + (battle.FieldW-1)*fieldStep + 1 + fieldStep
	if right > sideCol {
		t.Errorf("戰場右緣在第 %d 格，會壓到第 %d 格的狀態欄", right, sideCol)
	}
	if fieldRow+battle.FieldH >= Rows {
		t.Errorf("戰場下緣在第 %d 列，畫布只有 %d 列", fieldRow+battle.FieldH, Rows)
	}
}

// TestBattleSidePanelShowsBothSides 釘住狀態欄看得到雙方的兵與錢糧。
func TestBattleSidePanelShowsBothSides(t *testing.T) {
	face := testFace(t)
	c := NewCanvas(Cols, Rows, face)
	b := testBattle(t)
	DrawBattle(c, b, BattleView{})
	ink := 0
	for row := 1; row < Rows-1; row++ {
		ink += c.InkAt(sideCol+2, row, ColBG)
	}
	if ink == 0 {
		t.Error("右側狀態欄整欄空白")
	}
}

// TestBattleUnitPageListsLeaders 釘住「查看」列得出隊伍裡的每一位將領。
func TestBattleUnitPageListsLeaders(t *testing.T) {
	b := testBattle(t)
	var u *battle.Unit
	for _, x := range b.Units {
		if x.Alive() {
			u = x
			break
		}
	}
	if u == nil {
		t.Fatal("開場就沒有部隊")
	}
	title, lines := BattleUnitPage(u)
	if title != "查看" {
		t.Errorf("標題是 %q", title)
	}
	body := strings.Join(lines, "\n")
	for _, l := range u.Leaders {
		if !strings.Contains(body, l.Name) {
			t.Errorf("查看頁沒有列出 %s", l.Name)
		}
	}
	if !strings.Contains(body, "餘步") {
		t.Error("查看頁沒有顯示餘步（原版的用詞）")
	}
	if _, empty := BattleUnitPage(nil); len(empty) == 0 {
		t.Error("查看空的部隊應該也有一行說明")
	}
}

// TestCursorIsVisible 釘住游標所在的格與旁邊看得出不同。
func TestCursorIsVisible(t *testing.T) {
	face := testFace(t)
	b := testBattle(t)
	at := battle.FromOffset(3, 3)
	col := fieldCol + 3*fieldStep + 3%2
	row := fieldRow + 3

	plain := NewCanvas(Cols, Rows, face)
	DrawBattle(plain, b, BattleView{})
	marked := NewCanvas(Cols, Rows, face)
	DrawBattle(marked, b, BattleView{Cursor: Hexer{At: at, Shown: true}})

	same := true
	for y := row * CellH; y < (row+1)*CellH && same; y++ {
		for x := col * CellW; x < (col+fieldStep)*CellW; x++ {
			if plain.Img.RGBAAt(x, y) != marked.Img.RGBAAt(x, y) {
				same = false
				break
			}
		}
	}
	if same {
		t.Error("游標所在的格與沒有游標時長得一樣")
	}
}

// TestTerrainPageDrawsTheWholeField 釘住郡地理誌畫得出整張戰場，
// 而且列得出通往鄰郡的通道（說明書 p.19）。
func TestTerrainPageDrawsTheWholeField(t *testing.T) {
	f := battle.Generate(battle.Params{
		Prefecture: 15, Neighbours: []int{14, 16, 20}, LandValue: 60, FloodRate: 50,
	})
	title, lines := TerrainPage("洛陽", f, f.Gates)
	if !strings.Contains(title, "洛陽") || !strings.Contains(title, "郡地理誌") {
		t.Errorf("標題是 %q", title)
	}
	if len(lines) < f.H {
		t.Fatalf("只畫了 %d 列，戰場有 %d 列", len(lines), f.H)
	}
	for y := 0; y < f.H; y++ {
		got := len([]rune(strings.TrimLeft(lines[y], " ")))
		if got != f.W {
			t.Errorf("第 %d 列有 %d 格，戰場寬 %d", y, got, f.W)
		}
	}
	body := strings.Join(lines, "\n")
	for _, n := range []string{"14 郡", "16 郡", "20 郡"} {
		if !strings.Contains(body, n) {
			t.Errorf("沒有列出通往 %s 的通道", n)
		}
	}
	if !strings.Contains(body, "城") {
		t.Error("戰場上看不到城池")
	}
	if _, empty := TerrainPage("x", nil, nil); len(empty) == 0 {
		t.Error("沒有戰場時也該有一行說明")
	}
}

// TestTerrainPageIsStable 釘住同一個郡看兩次得到同一張圖。
//
// 郡地理誌與真的打起來用的必須是同一張——不然「先看地形再決定怎麼打」
// 這件事就沒有意義。
func TestTerrainPageIsStable(t *testing.T) {
	p := battle.Params{Prefecture: 22, Neighbours: []int{21, 23}, LandValue: 40}
	_, a := TerrainPage("x", battle.Generate(p), nil)
	_, b := TerrainPage("x", battle.Generate(p), nil)
	if strings.Join(a, "\n") != strings.Join(b, "\n") {
		t.Error("同一個郡看兩次得到不同的地形")
	}
}
