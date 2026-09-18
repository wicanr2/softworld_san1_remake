// mkhexfont 把 TrueType 字型點陣化成 GNU Unifont 的 HEX 格式
// （`fonts/README.md` 的「格式」那一節），給主選單的「使用楷書字／
// 使用隸書字」用（Issue #71）。
//
//	go run ./tools/mkhexfont -ttf <字型> -like fonts/unifont.hex.gz -out fonts/kai.hex.gz
//
// **碼位清單取自 `-like` 那一份**：要換的是字模，不是涵蓋率——
// 少一個碼位在畫面上是空白，而空白看起來像排版問題不像缺字
// （`fonts/README.md`）。所以兩套的碼位集合必須與 unifont 逐個相同，
// 由 `-like` 決定，不自己挑。
//
// 全形字畫成 16×16、半形畫成 8×16，寬度看 `-like` 那一份同一個碼位的寬度
// ——版面是按格算的（`internal/cells`），寬度換了整個畫面就跑掉。
package main

import (
	"bufio"
	"compress/gzip"
	"flag"
	"fmt"
	"image"
	"image/draw"
	"os"
	"sort"

	"golang.org/x/image/font"
	"golang.org/x/image/font/opentype"
	"golang.org/x/image/math/fixed"

	gamefont "github.com/wicanr2/softworld_san1_remake/internal/font"
)

func main() {
	ttf := flag.String("ttf", "", "TrueType 字型檔")
	like := flag.String("like", "fonts/unifont.hex.gz", "碼位與寬度照這一份")
	out := flag.String("out", "", "輸出的 .hex.gz")
	size := flag.Float64("size", 16, "字級（點）")
	baseline := flag.Int("baseline", 13, "基線在第幾列")
	show := flag.String("show", "", "只把這幾個字印成 ASCII 圖（除錯用，不寫檔）")
	flag.Parse()
	if *show != "" {
		if err := debugShow(*ttf, *show, *size, *baseline); err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
		return
	}
	if *ttf == "" || *out == "" {
		fmt.Fprintln(os.Stderr, "用法：mkhexfont -ttf <字型> -out <輸出>")
		os.Exit(2)
	}
	if err := run(*ttf, *like, *out, *size, *baseline); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func run(ttfPath, likePath, outPath string, size float64, baseline int) error {
	lf, err := os.Open(likePath)
	if err != nil {
		return err
	}
	defer lf.Close()
	base, err := gamefont.ParseHexGz(lf, 16)
	if err != nil {
		return err
	}
	blob, err := os.ReadFile(ttfPath)
	if err != nil {
		return err
	}
	sfnt, err := opentype.Parse(blob)
	if err != nil {
		return err
	}
	face, err := opentype.NewFace(sfnt, &opentype.FaceOptions{
		Size: size * super, DPI: 72, Hinting: font.HintingNone,
	})
	if err != nil {
		return err
	}
	defer face.Close()

	cps := base.Runes()
	sort.Slice(cps, func(i, j int) bool { return cps[i] < cps[j] })

	fh, err := os.Create(outPath)
	if err != nil {
		return err
	}
	defer fh.Close()
	zw := gzip.NewWriter(fh)
	bw := bufio.NewWriter(zw)
	drawn, blank := 0, 0
	for _, r := range cps {
		g, ok := base.Glyph(r)
		if !ok {
			continue
		}
		rows, filled := raster(face, r, g.W, baseline)
		if !filled {
			// **畫不出來就照抄 `-like` 那一份**：字型沒有這個字時
			// 留白等於缺字，而缺字在畫面上看起來像排版問題。
			rows = g.Rows
			blank++
		} else {
			drawn++
		}
		fmt.Fprintf(bw, "%04X:", r)
		for _, row := range rows {
			for _, b := range row {
				fmt.Fprintf(bw, "%02X", b)
			}
		}
		fmt.Fprintln(bw)
	}
	if err := bw.Flush(); err != nil {
		return err
	}
	if err := zw.Close(); err != nil {
		return err
	}
	fmt.Printf("%s：%d 個碼位，%d 個由 %s 畫出來，%d 個沿用 %s\n",
		outPath, len(cps), drawn, ttfPath, blank, likePath)
	return nil
}

// raster 把一個字畫進 w×16 的點陣；字型裡沒有這個字（整片空白）回 false。
//
// **不能直接用 16 點渲染。** 楷書與隸書是毛筆字，一筆在 16 像素高的框裡
// 只有一個像素寬；直接畫出來反鋸齒會把同一筆切成一段一段，看起來像雜訊
// 不像字（`三` 的三橫會變成三坨）。所以先用 `super` 倍的字級畫，再按
// 覆蓋率降取樣——一格裡有 `coverage` 以上的子像素是實心就填實。
const (
	super    = 3    // 超取樣倍率
	coverage = 0.35 // 一格的平均覆蓋率要多少才算實心
)

func raster(face font.Face, r rune, w, baseline int) ([][]byte, bool) {
	const h = 16
	bw, bh := w*super, h*super
	img := image.NewGray(image.Rect(0, 0, bw, bh))
	d := &font.Drawer{Dst: img, Src: image.NewUniform(image.White.C), Face: face}
	adv, ok := face.GlyphAdvance(r)
	if !ok {
		return nil, false
	}
	x := (fixed.I(bw) - adv) / 2
	if x < 0 {
		x = 0
	}
	d.Dot = fixed.Point26_6{X: x, Y: fixed.I(baseline * super)}
	d.DrawString(string(r))
	bpr := (w + 7) / 8
	rows := make([][]byte, h)
	any := false
	cells := float64(super * super * 255)
	need := int(cells * coverage)
	for y := 0; y < h; y++ {
		row := make([]byte, bpr)
		for px := 0; px < w; px++ {
			sum := 0
			for sy := 0; sy < super; sy++ {
				for sx := 0; sx < super; sx++ {
					sum += int(img.GrayAt(px*super+sx, y*super+sy).Y)
				}
			}
			if sum >= need {
				row[px/8] |= 0x80 >> uint(px%8)
				any = true
			}
		}
		rows[y] = row
	}
	return rows, any
}

// debugShow 把幾個字印成 ASCII 圖，用來看字級與基線擺對了沒有。
func debugShow(ttfPath, text string, size float64, baseline int) error {
	blob, err := os.ReadFile(ttfPath)
	if err != nil {
		return err
	}
	sf, err := opentype.Parse(blob)
	if err != nil {
		return err
	}
	face, err := opentype.NewFace(sf, &opentype.FaceOptions{Size: size * super, DPI: 72, Hinting: font.HintingNone})
	if err != nil {
		return err
	}
	defer face.Close()
	m := face.Metrics()
	fmt.Printf("字級 %.1f 基線 %d：ascent %v descent %v height %v\n",
		size, baseline, m.Ascent, m.Descent, m.Height)
	for _, r := range text {
		adv, ok := face.GlyphAdvance(r)
		b, _, _ := face.GlyphBounds(r)
		fmt.Printf("=== %c U+%04X advance %v bounds %v ok=%v\n", r, r, adv, b, ok)
		rows, filled := raster(face, r, 16, baseline)
		for _, row := range rows {
			line := ""
			for x := 0; x < 16; x++ {
				if row[x/8]&(0x80>>uint(x%8)) != 0 {
					line += "#"
				} else {
					line += "."
				}
			}
			fmt.Println(line)
		}
		fmt.Println("filled:", filled)
	}
	return nil
}

var _ = draw.Draw
