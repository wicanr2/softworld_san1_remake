// Command san1 是《三國演義》remake 的引擎進入點。
//
// ⚠ **本儲存庫不含任何原版檔案。** 用 -root 指到你自己的原版目錄。
//
//	go run ./cmd/san1 -root path/to/三國演義 -faction 0 -ai enhanced
//
// 操作：
//
//	← → ↑ ↓ ／ Tab   在自己的郡之間移動
//	0–9              指令類別（目前只有 3 兵士、4 內政 有實作）
//	1–9              子選單裡選項目
//	Enter            結束這個月
//	Esc              返回上一層
package main

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"sort"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/inpututil"

	"github.com/wicanr2/softworld_san1_remake/internal/ai"
	"github.com/wicanr2/softworld_san1_remake/internal/assets"
	"github.com/wicanr2/softworld_san1_remake/internal/font"
	"github.com/wicanr2/softworld_san1_remake/internal/game"
	"github.com/wicanr2/softworld_san1_remake/internal/session"
	"github.com/wicanr2/softworld_san1_remake/internal/state"
	"github.com/wicanr2/softworld_san1_remake/internal/ui"
)

type app struct {
	canvas *ui.Canvas
	screen *ebiten.Image
	s      *session.Session

	view  ui.View
	menu  byte // 展開中的類別；0 表示在主選單
	dirty bool
}

// conscriptStep 是按一次「徵兵」募多少人。
//
// **原版是輸入數字，這裡先用固定量**。要問數字得有文字輸入框，
// 那是另一件事；固定量讓迴圈先跑得起來，而且不會假裝自己是原版行為。
const conscriptStep = 100

func (a *app) Update() error {
	if inpututil.IsKeyJustPressed(ebiten.KeyEscape) {
		a.menu, a.view.Menu, a.view.Items = 0, "", nil
		a.view.Prompt = ""
		a.dirty = true
		return nil
	}
	if inpututil.IsKeyJustPressed(ebiten.KeyEnter) ||
		inpututil.IsKeyJustPressed(ebiten.KeyNumpadEnter) {
		a.s.EndMonth()
		a.menu, a.view.Menu, a.view.Items = 0, "", nil
		a.view.Prompt = ""
		a.dirty = true
		return nil
	}
	if inpututil.IsKeyJustPressed(ebiten.KeyTab) ||
		inpututil.IsKeyJustPressed(ebiten.KeyRight) ||
		inpututil.IsKeyJustPressed(ebiten.KeyDown) {
		a.cycle(+1)
		return nil
	}
	if inpututil.IsKeyJustPressed(ebiten.KeyLeft) ||
		inpututil.IsKeyJustPressed(ebiten.KeyUp) {
		a.cycle(-1)
		return nil
	}
	for k := ebiten.Key0; k <= ebiten.Key9; k++ {
		if inpututil.IsKeyJustPressed(k) {
			a.press(byte('0' + (k - ebiten.Key0)))
			return nil
		}
	}
	return nil
}

// cycle 在玩家自己的郡之間移動選取。
func (a *app) cycle(d int) {
	own := a.s.PlayerTerritory()
	if len(own) == 0 {
		return
	}
	sort.Ints(own)
	i := 0
	for j, id := range own {
		if id == a.view.Sel {
			i = j
		}
	}
	i = (i + d + len(own)) % len(own)
	a.view.Sel = own[i]
	a.dirty = true
}

func (a *app) press(k byte) {
	defer func() { a.dirty = true }()
	if a.menu == 0 {
		title, items := ui.SubMenu(k)
		if items == nil {
			a.view.Prompt = fmt.Sprintf("「%s」還沒實作", commandName(k))
			return
		}
		a.menu, a.view.Menu, a.view.Items = k, title, items
		a.view.Prompt = ""
		return
	}
	a.do(a.menu, k)
	a.menu, a.view.Menu, a.view.Items = 0, "", nil
}

func (a *app) do(cat, item byte) {
	sel := a.view.Sel
	var o game.Order
	switch {
	case cat == '4' && item == '1':
		o = game.ReclaimOrder{At: sel}
	case cat == '4' && item == '2':
		o = game.FloodControlOrder{At: sel}
	case cat == '3' && item == '1', cat == '3' && item == '2':
		// 徵兵與武器都要指定將領：先挑這個郡裡離上限最遠的那一位。
		// **原版是玩家自己選**，這裡的自動挑選是暫代，要換掉。
		gen := a.pickGeneral(sel)
		if gen == nil {
			a.view.Prompt = "這個郡沒有你的將領"
			return
		}
		if item == '1' {
			o = game.ConscriptOrder{At: sel, General: gen.Index, Count: conscriptStep}
		} else {
			o = game.ArmsOrder{At: sel, General: gen.Index, Units: 100}
		}
	default:
		a.view.Prompt = "沒有這個項目"
		return
	}
	if err := a.s.Do(o); err != nil {
		a.view.Prompt = err.Error()
		return
	}
	a.view.Prompt = ""
}

func (a *app) pickGeneral(prefectureID int) *game.General {
	var best *game.General
	room := 0
	for _, x := range a.s.G.Garrison(prefectureID) {
		if x.Faction != a.s.Player {
			continue
		}
		if r := x.TroopCap() - x.Soldiers; r > room || best == nil {
			best, room = x, r
		}
	}
	return best
}

func commandName(k byte) string {
	for _, c := range ui.Commands() {
		if c.Key == k {
			return c.Name
		}
	}
	return string(k)
}

func (a *app) Draw(dst *ebiten.Image) {
	if a.dirty {
		// 畫面內容在 internal/ui，Ebiten 這一層只負責貼上去——
		// 同一張圖無頭環境也產得出來（cmd/san1dump -png）。
		ui.DrawSession(a.canvas, a.s.G, a.s.Log, a.view)
		a.screen.WritePixels(a.canvas.Img.Pix)
		a.dirty = false
	}
	dst.DrawImage(a.screen, nil)
}

func (a *app) Layout(int, int) (int, int) {
	return ui.Cols * ui.CellW, ui.Rows * ui.CellH
}

func main() {
	root := flag.String("root", "", "原版遊戲目錄（必填，玩家自備）")
	fontPath := flag.String("font", "fonts/unifont.hex.gz", "點陣字型")
	slot := flag.String("slot", "001", "劇本：001..006")
	faction := flag.Int("faction", -1, "玩家的勢力槽號；−1 ＝ 第一個在用的")
	aiMode := flag.String("ai", string(ai.ModeEnhanced),
		"電腦 AI：base（原版還原）／plus（加強版還原）／enhanced（remake 強化）")
	difficulty := flag.Int("difficulty", 5, "難度 1..10")
	scale := flag.Int("scale", 2, "視窗放大倍率（整數倍，不做非整數縮放）")
	flag.Parse()
	if *root == "" {
		fmt.Fprintln(os.Stderr, "san1: 要用 -root 指到原版目錄（本儲存庫不含原版檔案）")
		flag.Usage()
		os.Exit(2)
	}

	fh, err := os.Open(*fontPath)
	if err != nil {
		die(err)
	}
	face, err := font.ParseHexGz(fh, 16)
	fh.Close()
	if err != nil {
		die(err)
	}

	c, err := openData2(*root)
	if err != nil {
		die(err)
	}
	sc, err := state.LoadScenario(c, state.Slot(*slot))
	if err != nil {
		die(err)
	}
	f := *faction
	if f < 0 {
		act := sc.ActiveFactions()
		if len(act) == 0 {
			die(fmt.Errorf("劇本 %s 裡沒有在用的勢力", *slot))
		}
		f = act[0]
	}
	g, err := game.New(sc, state.FactionID(f), *difficulty)
	if err != nil {
		die(err)
	}
	brain, err := ai.New(ai.Mode(*aiMode))
	if err != nil {
		die(err)
	}
	s := session.New(g, brain, state.FactionID(f))

	a := &app{
		canvas: ui.NewCanvas(ui.Cols, ui.Rows, face),
		screen: ebiten.NewImage(ui.Cols*ui.CellW, ui.Rows*ui.CellH),
		s:      s,
		dirty:  true,
	}
	if own := s.PlayerTerritory(); len(own) > 0 {
		sort.Ints(own)
		a.view.Sel = own[0]
	}

	// 整數倍放大：CJK 點陣字非整數縮放會糊掉（`CLAUDE.md` §3.3）。
	if *scale < 1 {
		*scale = 1
	}
	ebiten.SetWindowSize(ui.Cols*ui.CellW**scale, ui.Rows*ui.CellH**scale)
	lord := g.Lord(state.FactionID(f))
	title := "三國演義 remake"
	if lord != nil {
		title = fmt.Sprintf("三國演義 remake ── %s（AI：%s）", lord.Name, brain.Name())
	}
	ebiten.SetWindowTitle(title)
	if err := ebiten.RunGame(a); err != nil {
		die(err)
	}
}

func openData2(root string) (*assets.Container, error) {
	base := filepath.Join(root, "DATA2")
	var parts [3][]byte
	for i, ext := range []string{".NAM", ".IDX", ".GRP"} {
		b, err := os.ReadFile(base + ext)
		if err != nil {
			return nil, fmt.Errorf("讀 DATA2%s：%w", ext, err)
		}
		parts[i] = b
	}
	return assets.OpenContainer(parts[0], parts[1], parts[2])
}

func die(err error) {
	fmt.Fprintln(os.Stderr, "san1:", err)
	os.Exit(1)
}
