// Command san1 是《三國演義》remake 的引擎進入點。
//
// ⚠ **本儲存庫不含任何原版檔案。** 用 -root 指到你自己的原版目錄。
//
//	go run ./cmd/san1 -root path/to/三國演義
//
// 目前的畫面是劇本的州郡一覽——那是第一個「原版資料真的走完整條管線
// 並畫到螢幕上」的證據。遊戲玩法還沒實作。
package main

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"

	"github.com/hajimehoshi/ebiten/v2"

	"github.com/wicanr2/softworld_san1_remake/internal/assets"
	"github.com/wicanr2/softworld_san1_remake/internal/font"
	"github.com/wicanr2/softworld_san1_remake/internal/state"
	"github.com/wicanr2/softworld_san1_remake/internal/ui"
)

type game struct {
	canvas *ui.Canvas
	screen *ebiten.Image
	sc     *state.Scenario
	slot   string
	dirty  bool
}

func (g *game) Update() error { return nil }

func (g *game) Draw(dst *ebiten.Image) {
	if g.dirty {
		// 畫面內容在 internal/ui，Ebiten 這一層只負責貼上去——
		// 同一張圖無頭環境也產得出來（cmd/san1dump -png）。
		ui.DrawPrefectureList(g.canvas, g.sc, g.slot)
		g.screen.WritePixels(g.canvas.Img.Pix)
		g.dirty = false
	}
	dst.DrawImage(g.screen, nil)
}

func (g *game) Layout(int, int) (int, int) {
	return ui.Cols * ui.CellW, ui.Rows * ui.CellH
}

func main() {
	root := flag.String("root", "", "原版遊戲目錄（必填，玩家自備）")
	fontPath := flag.String("font", "fonts/unifont.hex.gz", "點陣字型")
	slot := flag.String("slot", "001", "劇本：001..006")
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

	g := &game{
		canvas: ui.NewCanvas(ui.Cols, ui.Rows, face),
		screen: ebiten.NewImage(ui.Cols*ui.CellW, ui.Rows*ui.CellH),
		sc:     sc,
		slot:   *slot,
		dirty:  true,
	}
	// 整數倍放大：CJK 點陣字非整數縮放會糊掉（`CLAUDE.md` §3.3）。
	if *scale < 1 {
		*scale = 1
	}
	ebiten.SetWindowSize(ui.Cols*ui.CellW**scale, ui.Rows*ui.CellH**scale)
	ebiten.SetWindowTitle("三國演義 remake")
	if err := ebiten.RunGame(g); err != nil {
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
