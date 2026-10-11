package main

import (
	"image"
	"testing"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/wicanr2/softworld_san1_remake/internal/assets"
	"github.com/wicanr2/softworld_san1_remake/internal/battle"
	"github.com/wicanr2/softworld_san1_remake/internal/ui"
)

func TestPointerCanvasScaleAndWindowBar(t *testing.T) {
	for _, hd := range []bool{false, true} {
		for _, bar := range []bool{false, true} {
			a := &app{hdTheme: hd, windowBar: &windowBar{}}
			a.windowBar.state.Visible = bar
			scale, y := 1, 100
			if hd {
				scale = 4
			}
			if bar {
				y += ui.WindowBarHeight
			}
			if p := a.gamePointer(520*scale, y*scale); p != image.Pt(520, 100) {
				t.Fatalf("hd=%v bar=%v: %v", hd, bar, p)
			}
		}
	}
	for col := 0; col < assets.FieldCols; col++ {
		for row := 0; row < 10; row++ {
			x, y := assets.FieldCell(col, row)
			c, r, ok := pointerField(image.Pt(x+assets.TileW/2, y+assets.TileH/2))
			if !ok || c != col || r != row {
				t.Fatalf("場地 (%d,%d) → (%d,%d) %v", col, row, c, r, ok)
			}
		}
	}
}

func TestPointerKeepsAmountEntryAndRejectsUnavailablePref(t *testing.T) {
	a := &app{num: &numEntry{max: 9}}
	a.pointerHit(ui.PointerTarget{Kind: ui.PointerKey, Value: '7'})
	if a.num.typed || len(a.pointer.keys) != 1 || a.pointer.keys[0] != ebiten.Key7 {
		t.Fatal("金額九元以內也須先輸入，再確認")
	}
	a.pointer.keys = nil
	a.pointerHit(ui.PointerTarget{Kind: ui.PointerCommand, Value: '4'})
	if len(a.pointer.keys) != 0 {
		t.Fatal("金額輸入穿透到主命令")
	}
	a.view.PrefPick = &ui.PrefPick{}
	a.view.PrefPick.Valid[11] = true
	a.pointerHit(ui.PointerTarget{Kind: ui.PointerPref, Value: 12})
	if a.num.typed || len(a.pointer.keys) != 0 {
		t.Fatal("無效郡被選中")
	}
	a.pointerHit(ui.PointerTarget{Kind: ui.PointerPref, Value: 11})
	if !a.num.typed || a.num.value != 11 || len(a.pointer.keys) != 1 || a.pointer.keys[0] != ebiten.KeyEnter {
		t.Fatal("有效郡未交給原有輸入驗證")
	}
}

func TestPointerSkirmishAdjacentDirection(t *testing.T) {
	g := &battle.SkirmishGeneral{Col: 3, Row: 4}
	a := &app{fight: &fight{asking: &engineAsk{skirmish: g}, skm: skmMarch}}
	for _, d := range battle.Dirs() {
		h := battle.FromOffset(g.Col, g.Row).Step(d)
		col, row := battle.ToOffset(h)
		x, y := assets.FieldCell(col, row)
		a.pointerMap(image.Pt(x+assets.TileW/2, y+assets.TileH/2))
		want := ebiten.Key0 + ebiten.Key(d)
		if len(a.pointer.keys) != 1 || a.pointer.keys[0] != want {
			t.Fatalf("方向 %d: %v", d, a.pointer.keys)
		}
	}
	a.pointer.keys = nil
	x, y := assets.FieldCell(8, 8)
	a.pointerMap(image.Pt(x+assets.TileW/2, y+assets.TileH/2))
	if len(a.pointer.keys) != 0 {
		t.Fatal("不相鄰的格子產生了移動")
	}
}

func TestPointerBattleBackAndFortConfirmation(t *testing.T) {
	a := &app{fight: &fight{waiting: waitRestYN}}
	a.pointerBack()
	if len(a.pointer.keys) != 1 || a.pointer.keys[0] != ebiten.KeyN {
		t.Fatal("戰場右鍵未取消確認")
	}
	a.fight.waiting = waitPlotDir
	a.pointerBack()
	if a.pointer.keys[0] != ebiten.KeyEnter {
		t.Fatal("用計右鍵未送原有取消輸入")
	}
	a.fight.view.Page = []string{"資料"}
	a.pointerBack()
	if a.pointer.keys[0] != ebiten.KeyEscape {
		t.Fatal("資料頁未先攔截返回")
	}
	a = &app{fortSpot: &fortSpotState{col: 2, row: 3, confirm: true}}
	x, y := assets.FieldCell(4, 4)
	a.pointerMap(image.Pt(x+assets.TileW/2, y+assets.TileH/2))
	if a.fortSpot.col != 2 || a.fortSpot.row != 3 || len(a.pointer.keys) != 0 {
		t.Fatal("確認時點擊穿透並改掉築城位置")
	}
}

func TestPointerNarrowBattlePanelDoesNotSelectHiddenField(t *testing.T) {
	a := &app{fight: &fight{waiting: waitCamp,
		runner: &battle.Runner{B: &battle.Battle{Field: &battle.Field{W: 8, H: 10}}}}}
	x, y := assets.FieldCell(9, 2)
	a.pointerMap(image.Pt(x+assets.TileW/2, y+assets.TileH/2))
	if len(a.pointer.keys) != 0 || a.fight.view.Cursor.At != (battle.Hex{}) {
		t.Fatal("點擊窄圖面板穿透到隱藏場地")
	}
	x, y = assets.FieldCell(5, 8)
	a.pointerMap(image.Pt(x+assets.TileW/2, y+assets.TileH/2))
	if len(a.pointer.keys) != 1 || a.fight.view.Cursor.At != battle.FromOffset(5, 8) {
		t.Fatal("可見場地未送出紮寨位置")
	}
}
