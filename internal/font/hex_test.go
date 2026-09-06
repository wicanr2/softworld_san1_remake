package font

import (
	"os"
	"strings"
	"testing"
)

func TestParseHexMinimal(t *testing.T) {
	// 兩個 8×2 的字：一個全實心、一個全空。
	src := "# 註解\n\n0041:FF00\n0042:0000\n"
	f, err := ParseHex(strings.NewReader(src), 2)
	if err != nil {
		t.Fatalf("ParseHex：%v", err)
	}
	if f.Len() != 2 {
		t.Fatalf("字數 ＝ %d，想要 2", f.Len())
	}
	g, ok := f.Glyph('A')
	if !ok {
		t.Fatal("找不到 A")
	}
	if g.W != 8 || g.H != 2 {
		t.Errorf("尺寸 ＝ %d×%d，想要 8×2", g.W, g.H)
	}
	// 第 0 列全實心，第 1 列全空。
	for x := 0; x < 8; x++ {
		if !g.At(x, 0) {
			t.Errorf("(%d,0) 應該是實心", x)
		}
		if g.At(x, 1) {
			t.Errorf("(%d,1) 應該是空的", x)
		}
	}
	// 越界回 false，不 panic。
	for _, p := range [][2]int{{-1, 0}, {0, -1}, {8, 0}, {0, 2}} {
		if g.At(p[0], p[1]) {
			t.Errorf("越界 (%d,%d) 應該回 false", p[0], p[1])
		}
	}
}

// TestParseHexRejects 釘住「不報錯就會安靜地錯下去」的幾種輸入。
func TestParseHexRejects(t *testing.T) {
	for _, tc := range []struct{ name, src string; h int }{
		{"高度非正", "0041:FF00", 0},
		{"沒有冒號", "0041FF00", 2},
		{"碼位不是十六進位", "XYZ:FF00", 2},
		{"點陣字元數是奇數", "0041:FF0", 2},
		{"除不盡高度", "0041:FF0000", 4},
		{"一個字都沒有", "# 只有註解\n\n", 2},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if _, err := ParseHex(strings.NewReader(tc.src), tc.h); err == nil {
				t.Fatal("想要錯誤")
			}
		})
	}
}

func TestCovers(t *testing.T) {
	f, err := ParseHex(strings.NewReader("0041:FF00\n"), 2)
	if err != nil {
		t.Fatal(err)
	}
	if m := f.Covers("AAA A"); len(m) != 0 {
		t.Errorf("不該有缺字，卻回 %q", m)
	}
	m := f.Covers("AB")
	if len(m) != 1 || m[0] != 'B' {
		t.Errorf("缺字 ＝ %q，想要 [B]", m)
	}
}

// TestRealFont 對真的字型檔跑一次，並確認 42 個郡名都有字模。
//
// 這是 CJK 畫布的第一道擋牆：譯文用了字型沒有的字，在畫面上是空白，
// 而空白看起來像排版問題不像缺字。
func TestRealFont(t *testing.T) {
	fh, err := os.Open("../../fonts/unifont.hex.gz")
	if err != nil {
		t.Skipf("沒有字型檔：%v", err)
	}
	defer fh.Close()
	f, err := ParseHexGz(fh, 16)
	if err != nil {
		t.Fatalf("ParseHexGz：%v", err)
	}
	if f.Len() < 1000 {
		t.Errorf("只讀到 %d 個字，太少", f.Len())
	}
	g, ok := f.Glyph('遼')
	if !ok {
		t.Fatal("字型裡沒有「遼」")
	}
	if g.W != 16 || g.H != 16 {
		t.Errorf("漢字尺寸 ＝ %d×%d，想要 16×16", g.W, g.H)
	}
	// 全空的字模等於沒有——會畫出一片空白而不報錯。
	filled := 0
	for y := 0; y < g.H; y++ {
		for x := 0; x < g.W; x++ {
			if g.At(x, y) {
				filled++
			}
		}
	}
	if filled == 0 {
		t.Error("「遼」的字模全空")
	}

	// 42 個郡名（docs/formats/02）用到的字全部要有。
	const prefectures = "遼東涿郡渤海鄴太原上黨北齊琅邪下邳陳留譙潁川弘農洛陽京兆安定天水武威酒泉建業淮南吳柴桑廬陵夷襄江宜都長沙陵桂零漢中成巴永昌牂柯寧鬱林"
	if missing := f.Covers(prefectures); len(missing) > 0 {
		t.Errorf("郡名用到的字有 %d 個沒有字模：%q", len(missing), string(missing))
	}
}
