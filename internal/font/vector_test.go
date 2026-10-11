package font

import (
	"bytes"
	"image"
	"os"
	"strings"
	"testing"
)

func vectorFixture(t *testing.T, kind string) *Face {
	t.Helper()
	h, err := os.Open("../../fonts/" + kind + ".hex.gz")
	if err != nil {
		t.Fatal(err)
	}
	defer h.Close()
	f, err := ParseHexGz(h, 16)
	if err != nil {
		t.Fatal(err)
	}
	v, err := os.Open("../../fonts/" + kind + ".ttf")
	if err != nil {
		t.Fatal(err)
	}
	defer v.Close()
	if err := f.LoadVector(v); err != nil {
		t.Fatal(err)
	}
	return f
}

func TestVectorGlyphCoverageAndBitmapPreserved(t *testing.T) {
	for _, kind := range []string{"kai", "li"} {
		t.Run(kind, func(t *testing.T) {
			f := vectorFixture(t, kind)
			bitmap, _ := f.Glyph('劉')
			before := make([][]byte, len(bitmap.Rows))
			for i := range bitmap.Rows {
				before[i] = append([]byte(nil), bitmap.Rows[i]...)
			}
			for _, tc := range []struct {
				r    rune
				w, h int
			}{{'劉', 64, 64}, {'A', 32, 64}, {'選', 128, 64}, {'備', 48, 48}} {
				m, ok := f.VectorGlyph(tc.r, tc.w, tc.h)
				if !ok || m.Bounds() != image.Rect(0, 0, tc.w, tc.h) {
					t.Fatalf("字格 %c %dx%d：%v", tc.r, tc.w, tc.h, ok)
				}
				ink, partial := 0, 0
				for _, a := range m.Pix {
					if a > 0 {
						ink++
					}
					if a > 0 && a < 255 {
						partial++
					}
				}
				if ink == 0 || partial == 0 {
					t.Fatalf("%c 缺筆畫或向量覆蓋率：%d/%d", tc.r, ink, partial)
				}
				cached, _ := f.VectorGlyph(tc.r, tc.w, tc.h)
				if cached != m {
					t.Fatal("同一字格未使用快取")
				}
			}
			after, _ := f.Glyph('劉')
			if len(before) != len(after.Rows) {
				t.Fatal("原點陣高度改變")
			}
			for i := range before {
				if !bytes.Equal(before[i], after.Rows[i]) {
					t.Fatal("原點陣筆畫被改寫")
				}
			}
		})
	}
}

func TestVectorFallbackAndCacheLimit(t *testing.T) {
	f := vectorFixture(t, "kai")
	identity := f.VectorIdentity()
	if err := f.LoadVector(strings.NewReader("invalid")); err == nil {
		t.Fatal("錯誤字型未拒絕")
	}
	if f.VectorIdentity() != identity {
		t.Fatal("解析失敗破壞原字型")
	}
	for _, tc := range []struct {
		r    rune
		w, h int
	}{{'劉', 0, 64}, {'劉', 64, 513}, {'\U0010ffff', 64, 64}} {
		if _, ok := f.VectorGlyph(tc.r, tc.w, tc.h); ok {
			t.Fatal("無效字格沒有回退")
		}
	}
	old, _ := f.VectorGlyph('劉', 64, 64)
	f.vector.bytes = vectorCacheBytes
	_, _ = f.VectorGlyph('備', 64, 64)
	if f.vector.bytes > vectorCacheBytes {
		t.Fatal("字模快取超過限制")
	}
	next, _ := f.VectorGlyph('劉', 64, 64)
	if old == next || !bytes.Equal(old.Pix, next.Pix) {
		t.Fatal("清快取後字模不一致")
	}
	bitmap, err := ParseHex(strings.NewReader("0041:00000000000000000000000000000000\n"), 16)
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := bitmap.VectorGlyph('A', 32, 64); ok {
		t.Fatal("無向量來源卻成功")
	}
}
