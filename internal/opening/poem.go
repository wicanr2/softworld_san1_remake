package opening

import (
	"github.com/wicanr2/softworld_san1_remake/internal/assets"
	"github.com/wicanr2/softworld_san1_remake/internal/font"
)

// 開場的詞：楊慎〈臨江仙〉，《三國演義》毛宗崗本的卷首詞。
//
// 原版把它畫成點陣圖（`TZUE`／`TZUE1`），容器裡沒有對應的 Big5 字串，
// 所以這裡是照那張圖謄的（`L1`）。remake 用自己的字庫重畫同樣大小的兩張，
// 留給多語系換字；字的位置照原版每一個字的字框。
//
// ⚠ **沒有英日譯文**：這是作品的引文不是介面用語，不進 `internal/i18n`。
var poemColumns = []string{
	"詞曰",
	"滾滾長江東逝水",
	"浪花淘盡英雄",
	"是非成敗轉頭空",
	"青山依舊在",
	"幾度夕陽紅",
	"白髮漁樵江渚上",
	"慣看秋月春風",
	"一壺濁酒喜相逢",
	"古今多少事",
	"都付笑談中",
}

// PoemColumns 由右到左回傳詞的每一欄。
func PoemColumns() []string {
	out := make([]string, len(poemColumns))
	copy(out, poemColumns)
	return out
}

// 字框量自 `TZUE`（圖內座標）：十欄詩的左緣是 380、338、…、2（間距 42），
// 「詞曰」在 436；每個字框 24×22，第一個字框的上緣 1、列距 24，「詞曰」
// 比詩低一列。字的墨寬剛好 24，高 18–21，最高的從字框第 0 列畫到第 20 列。
const (
	PoemW          = 464
	PoemH          = 166
	poemGlyphW     = 24
	poemGlyphH     = 22
	poemRightColX  = 380
	poemColStep    = 42
	poemHeadX      = 436
	poemTopY       = 1
	poemRowStep    = 24
	PoemInkColor   = 11
	poemMaskColour = 15
)

// PoemGlyphBox 回第 col 欄（由右到左，0 是「詞曰」）第 row 個字的字框
// 左上角，畫面座標。
func PoemGlyphBox(col, row int) (x, y int) {
	if col == 0 {
		return PoemX + poemHeadX, PoemY + poemTopY + (row+1)*poemRowStep
	}
	return PoemX + poemRightColX - (col-1)*poemColStep, PoemY + poemTopY + row*poemRowStep
}

// PoemGlyphSize 是原版一個字框的寬高。
func PoemGlyphSize() (w, h int) { return poemGlyphW, poemGlyphH }

// FontPoem 用 remake 的字庫畫詞的兩張：ink（字 11、底 0）與 mask（字 0、底 15），
// 每個字置中在原版的字框裡。
func FontPoem(face *font.Face) (ink, mask *assets.Image) {
	ink = &assets.Image{W: PoemW, H: PoemH, Pix: make([]byte, PoemW*PoemH)}
	mask = &assets.Image{W: PoemW, H: PoemH, Pix: make([]byte, PoemW*PoemH)}
	for i := range mask.Pix {
		mask.Pix[i] = poemMaskColour
	}
	for col, text := range poemColumns {
		for row, r := range []rune(text) {
			g, ok := face.Glyph(r)
			if !ok {
				continue
			}
			bx, by := PoemGlyphBox(col, row)
			ox := bx - PoemX + (poemGlyphW-g.W)/2
			oy := by - PoemY + (poemGlyphH-g.H)/2
			for gy := 0; gy < g.H; gy++ {
				for gx := 0; gx < g.W; gx++ {
					if g.At(gx, gy) {
						ink.Set(ox+gx, oy+gy, PoemInkColor)
						mask.Set(ox+gx, oy+gy, 0)
					}
				}
			}
		}
	}
	return ink, mask
}
