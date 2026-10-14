package main

import (
	"bytes"
	"image"
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/wicanr2/softworld_san1_remake/internal/ai"
	"github.com/wicanr2/softworld_san1_remake/internal/assets"
	"github.com/wicanr2/softworld_san1_remake/internal/font"
	"github.com/wicanr2/softworld_san1_remake/internal/game"
	"github.com/wicanr2/softworld_san1_remake/internal/i18n"
	"github.com/wicanr2/softworld_san1_remake/internal/state"
	"github.com/wicanr2/softworld_san1_remake/internal/ui"
)

func controlCanvas(t *testing.T, height int) *ui.Canvas {
	t.Helper()
	load := func(name string, height int) *font.Face {
		f, err := os.Open(filepath.Join("../../fonts", name))
		if err != nil {
			t.Fatal(err)
		}
		defer f.Close()
		face, err := font.ParseHexGz(f, height)
		if err != nil {
			t.Fatal(err)
		}
		return face
	}
	c := ui.NewCanvasPx(640, height, load("unifont.hex.gz", 16))
	c.SetSmallFace(load("ascii6x10.hex.gz", 10))
	return c
}

func controlApp(t *testing.T, edition state.Edition) *app {
	t.Helper()
	s, _ := bubbleVoiceFixture(t, edition)
	s.Bubbles = nil
	at := s.AdvanceToHuman(0)
	s.Bubbles = nil
	if at == 0 {
		t.Fatal("未到玩家郡")
	}
	folder := "三國演義"
	if edition == state.EditionPlus {
		folder = "三國演義1加強版"
	}
	dir := filepath.Join(os.Getenv("SAN1_ORIG"), folder)
	c1, err := openContainer(dir, "DATA1")
	if err != nil {
		t.Fatal(err)
	}
	c3, err := openContainer(dir, "DATA3")
	if err != nil {
		t.Fatal(err)
	}
	art, err := ui.NewArtScreen(c3, c1)
	if err != nil {
		t.Fatal(err)
	}
	a := &app{s: s, art: art, canvas: controlCanvas(t, 408), view: ui.View{Sel: at, Status: true}}
	a.mainAsk(at)
	return a
}

func pressControlKey(t *testing.T, a *app, key ebiten.Key) {
	t.Helper()
	a.pointer.owner, a.pointer.keys = a.inputOwner(), []ebiten.Key{key}
	if err := a.Update(); err != nil {
		t.Fatal(err)
	}
}

func TestMainEnterPreservesTurnAndSave(t *testing.T) {
	for _, edition := range []state.Edition{state.EditionBase, state.EditionPlus} {
		for _, bare := range []bool{false, true} {
			t.Run(string(edition)+map[bool]string{false: "/prompt", true: "/bare"}[bare], func(t *testing.T) {
				a := controlApp(t, edition)
				if bare {
					a.num = nil
					a.view.Prompt = "原有提示"
				}
				at, cursor := a.s.Waiting(), a.s.MonthCursor
				before, err := a.s.G.CaptureSnapshot()
				if err != nil {
					t.Fatal(err)
				}
				dir := t.TempDir()
				name := a.s.SaveName(1, at, "")
				if err := a.s.Save(dir, 1, name); err != nil {
					t.Fatal(err)
				}
				files := mainBubbleSaveFiles(t, dir)
				for _, key := range []ebiten.Key{ebiten.KeyEnter, ebiten.KeyNumpadEnter} {
					status := a.view.Status
					pressControlKey(t, a, key)
					if a.view.Status == status || a.s.Waiting() != at || a.s.MonthCursor != cursor || a.view.Sel != at {
						t.Fatal("Enter 未切換面板或消耗回合")
					}
					after, err := a.s.G.CaptureSnapshot()
					if err != nil || !reflect.DeepEqual(before, after) {
						t.Fatalf("Enter 改變遊戲快照：%v", err)
					}
				}
				if err := a.s.Save(dir, 1, name); err != nil {
					t.Fatal(err)
				}
				if !reflect.DeepEqual(files, mainBubbleSaveFiles(t, dir)) {
					t.Fatal("狀態往返改變完整存檔")
				}
				a.num.value, a.num.typed, a.num.digits = 4, true, "4"
				pressControlKey(t, a, ebiten.KeyEnter)
				if a.menu != '4' || a.view.Status || a.s.Waiting() != at {
					t.Fatal("已輸入的主命令未正常確認")
				}
			})
		}
	}
}

func TestSceneClearsBeforeWipeAndRestoresStatus(t *testing.T) {
	old := i18n.Current
	i18n.Current = i18n.ZhHant
	defer func() { i18n.Current = old }()
	for _, edition := range []state.Edition{state.EditionBase, state.EditionPlus} {
		t.Run(string(edition), func(t *testing.T) {
			a := controlApp(t, edition)
			a.paint()
			before := append([]byte(nil), a.canvas.Img.Pix...)
			b := &game.Bubble{Scene: 1, X1: assets.SceneMainX, Y1: assets.SceneMainY}
			a.s.Queue([]game.Event{{Bubble: b}})
			a.paint()
			panel := image.Rect(408, 36, 632, 292)
			checkBlue := func(except image.Rectangle) {
				t.Helper()
				for y := panel.Min.Y; y < panel.Max.Y; y++ {
					for x := panel.Min.X; x < panel.Max.X; x++ {
						pt := image.Pt(x, y)
						// ClearPanel 保留外框，只清兩塊相交的內部矩形。
						if !pt.In(image.Rect(424, 44, 616, 284)) && !pt.In(image.Rect(416, 52, 624, 276)) {
							continue
						}
						if !image.Pt(x, y).In(except) && a.canvas.Img.RGBAAt(x, y) != assets.EGAPalette[1] {
							t.Fatalf("事件留下郡狀態像素：%d,%d", x, y)
						}
					}
				}
			}
			checkBlue(image.Rectangle{})
			a.startWipe()
			if a.wipe == nil {
				t.Fatal("場景沒有拉幕")
			}
			for a.wipe != nil {
				a.stepWipe()
				checkBlue(image.Rect(b.X1, b.Y1, b.X1+176, b.Y1+96))
			}
			a.paint()
			checkBlue(image.Rect(b.X1, b.Y1, b.X1+176, b.Y1+96))
			a.s.PopBubble()
			a.paint()
			if !bytes.Equal(before, a.canvas.Img.Pix) {
				t.Fatal("事件結束後未恢復完整主畫面")
			}
		})
	}
}

func TestWindowBarFourOptionsFitLocales(t *testing.T) {
	old := i18n.Current
	defer func() { i18n.Current = old }()
	for _, locale := range i18n.Locales() {
		i18n.Current = locale
		a := &app{canvas: &ui.Canvas{}, windowBar: &windowBar{state: ui.NewWindowBarState(), mode: ai.ModeEnhanced}}
		for level := 1; level <= game.AIOrdersMax; level++ {
			a.windowBar.orders = level
			for field := 0; field < ui.WindowBarOptionCount; field++ {
				a.windowBar.state.Menu = field
				c := controlCanvas(t, ui.WindowBarCanvasHeight)
				ui.DrawWindowBar(c, a.barView())
				if len(c.Missing) != 0 || c.Clipped != 0 {
					t.Fatalf("%s 強化 %d 欄位 %d 缺字或裁切：%v %d", locale, level, field, c.Missing, c.Clipped)
				}
			}
		}
	}
}

func TestKeyboardOnlyClearsPointerWithoutChangingInput(t *testing.T) {
	a := &app{canvas: &ui.Canvas{}, windowBar: &windowBar{state: ui.NewWindowBarState()}, num: &numEntry{value: 23, typed: true, digits: "23"}}
	a.pointer.keys = []ebiten.Key{ebiten.KeyEnter}
	a.pointer.used = true
	a.chooseWindowOption(3, 0)
	a.updatePointerInput()
	if a.gameMouseEnabled() || len(a.pointer.keys) != 0 || a.pointer.pressed || a.num.value != 23 || a.num.digits != "23" || !a.num.typed {
		t.Fatal("純鍵盤未停用滑鼠，或切換遺失已輸入的數字")
	}
	a.chooseWindowOption(3, 1)
	if !a.gameMouseEnabled() || a.barView().Selected[3] != 1 {
		t.Fatal("未恢復鍵盤＋滑鼠")
	}
}

// 把目前等待的郡放在當月最後一格，隔離「一旦耗令就換月」的邊界。
// 這是控制器 fixture，不當成正常玩家路徑或原版 oracle。
func controlLastMonthCell(t *testing.T, a *app) int {
	t.Helper()
	at := a.s.Waiting()
	for i, id := range a.s.MonthOrder {
		if id == at {
			last := len(a.s.MonthOrder) - 1
			a.s.MonthOrder[i], a.s.MonthOrder[last] = a.s.MonthOrder[last], id
			a.s.MonthCursor = last
			return at
		}
	}
	t.Fatal("等待郡不在當月順序中")
	return 0
}

func TestLastCellEnterAndNavigationKeepMonth(t *testing.T) {
	for _, edition := range []state.Edition{state.EditionBase, state.EditionPlus} {
		t.Run(string(edition), func(t *testing.T) {
			a := controlApp(t, edition)
			at := controlLastMonthCell(t, a)
			cursor := a.s.MonthCursor
			before, err := a.s.G.CaptureSnapshot()
			if err != nil {
				t.Fatal(err)
			}
			dir := t.TempDir()
			name := a.s.SaveName(1, at, "")
			if err := a.s.Save(dir, 1, name); err != nil {
				t.Fatal(err)
			}
			files := mainBubbleSaveFiles(t, dir)
			for i := 0; i < 12; i++ {
				pressControlKey(t, a, ebiten.KeyEnter)
				pressControlKey(t, a, ebiten.KeyNumpadEnter)
				for frame := 0; frame < 8; frame++ {
					if err := a.Update(); err != nil {
						t.Fatal(err)
					}
				}
			}
			a.pointerHit(ui.PointerTarget{Kind: ui.PointerKey, Value: ui.PointerCommands})
			a.pointerMap(image.Pt(300, 380)) // 地圖外空白處。
			a.pointerBack()
			if err := a.Update(); err != nil {
				t.Fatal(err)
			}
			after, err := a.s.G.CaptureSnapshot()
			if err != nil || !reflect.DeepEqual(before, after) || a.s.Waiting() != at || a.s.MonthCursor != cursor {
				t.Fatal("Enter、顯示命令、空白點擊或返回推進了末郡")
			}
			if err := a.s.Save(dir, 1, name); err != nil {
				t.Fatal(err)
			}
			if !reflect.DeepEqual(files, mainBubbleSaveFiles(t, dir)) {
				t.Fatal("末郡的一般操作改變完整存檔")
			}
		})
	}
}

func TestMainMouseRequiresNamedCommand(t *testing.T) {
	for _, edition := range []state.Edition{state.EditionBase, state.EditionPlus} {
		t.Run(string(edition), func(t *testing.T) {
			a := controlApp(t, edition)
			at := controlLastMonthCell(t, a)
			date := a.s.G.Date
			a.pointer.used = true
			for _, status := range []bool{true, false} {
				a.view.Status = status
				a.paint()
				if h, ok := a.canvas.PointerAt(image.Pt(352, 390)); ok && h.Kind == ui.PointerKey && h.Value == 13 {
					t.Fatal("主提示仍提供通用確定按鈕")
				}
			}
			a.num.value, a.num.typed, a.num.digits = 4, true, "4"
			a.pointerHit(ui.PointerTarget{Kind: ui.PointerKey, Value: 13})
			if len(a.pointer.keys) != 0 || a.s.Waiting() != at || a.s.G.Date != date {
				t.Fatal("主提示的一般滑鼠確定接受了待送指令")
			}
			// 明確點內政只開子選單，點休息才執行耗令命令。
			a.pointerHit(ui.PointerTarget{Kind: ui.PointerCommand, Value: '4'})
			if err := a.Update(); err != nil || a.menu != '4' || a.s.Waiting() != at {
				t.Fatal("具名主命令沒有開子選單，或提前耗令")
			}
			a.pointerHit(ui.PointerTarget{Kind: ui.PointerNumber, Value: 4})
			if err := a.Update(); err != nil {
				t.Fatal(err)
			}
			// fixture 的劉備未任軍師，休息直接成功。
			if !a.s.G.Prefecture(at).Commanded || a.s.G.Date != date {
				t.Fatal("具名休息未耗令，或在命令完成前換月")
			}
			if err := a.Update(); err != nil || a.s.G.Date == date {
				t.Fatal("末郡完成休息後沒有推進月份")
			}
		})
	}
}

func TestLastCellCancelledAndInvalidCommandKeepMonth(t *testing.T) {
	for _, edition := range []state.Edition{state.EditionBase, state.EditionPlus} {
		t.Run(string(edition), func(t *testing.T) {
			a := controlApp(t, edition)
			at := controlLastMonthCell(t, a)
			date, cursor := a.s.G.Date, a.s.MonthCursor
			pressControlKey(t, a, ebiten.Key4)
			pressControlKey(t, a, ebiten.KeyEnter)
			pressControlKey(t, a, ebiten.KeyEscape)
			a.apply(game.ReliefOrder{At: at, Gold: -1})
			for frame := 0; frame < 8; frame++ {
				if err := a.Update(); err != nil {
					t.Fatal(err)
				}
			}
			if a.s.Waiting() != at || a.s.MonthCursor != cursor || a.s.G.Date != date || a.s.G.Prefecture(at).Commanded {
				t.Fatal("取消或無效命令推進了末郡")
			}
		})
	}
}
