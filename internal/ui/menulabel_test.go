package ui

import (
	"bytes"
	"image"
	"image/color"
	"os"
	"strings"
	"testing"

	"github.com/wicanr2/softworld_san1_remake/internal/assets"
	"github.com/wicanr2/softworld_san1_remake/internal/font"
	"github.com/wicanr2/softworld_san1_remake/internal/i18n"
)

func menuLabelSmallFace(t *testing.T) *font.Face {
	t.Helper()
	f, err := os.Open("../../fonts/ascii6x10.hex.gz")
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	face, err := font.ParseHexGz(f, 10)
	if err != nil {
		t.Fatal(err)
	}
	return face
}

func TestMenuLabelEveryCurrentTranslationFits(t *testing.T) {
	for _, name := range []string{"unifont.hex.gz", "kai.hex.gz", "li.hex.gz"} {
		face := namedFace(t, name)
		for _, locale := range i18n.Locales() {
			for _, key := range []string{"title.mainPlate", "title.pickScenario", "title.loadPlate", "title.musicPlate"} {
				t.Run(name+"/"+string(locale)+"/"+key, func(t *testing.T) {
					c := NewCanvasPx(640, 408, face)
					c.SetSmallFace(menuLabelSmallFace(t))
					s := i18n.T(locale, key)
					l := menuLabelGeometry(c, s)
					if l.text != s {
						t.Fatalf("current title truncated: %q -> %q", s, l.text)
					}
					box := image.Rect(l.x, l.y, l.x+l.width, l.y+l.height)
					if !box.In(menuLabelSafe) {
						t.Fatalf("layout %v outside %v", box, menuLabelSafe)
					}
					if d := (box.Min.X + box.Max.X) - (menuLabelSafe.Min.X + menuLabelSafe.Max.X); d < -1 || d > 1 {
						t.Fatalf("horizontal centre differs by %d half pixels", d)
					}
					if d := (box.Min.Y + box.Max.Y) - (menuLabelSafe.Min.Y + menuLabelSafe.Max.Y); d < -1 || d > 1 {
						t.Fatalf("vertical centre differs by %d half pixels", d)
					}
					DrawMenuLabel(c, s, fg)
					x0, y0, x1, y1, n := inkBox(c, fg)
					if n == 0 || !image.Rect(x0, y0, x1+1, y1+1).In(menuLabelSafe) {
						t.Fatalf("ink outside safe body: %d %d %d %d n=%d", x0, y0, x1, y1, n)
					}
					if len(c.Missing) > 0 || c.Clipped != 0 {
						t.Fatalf("missing=%v clipped=%d", c.Missing, c.Clipped)
					}
				})
			}
		}
	}
}

func TestMenuLabelChineseOriginalCellsUnchanged(t *testing.T) {
	for _, name := range []string{"unifont.hex.gz", "kai.hex.gz", "li.hex.gz"} {
		face := namedFace(t, name)
		for _, s := range []string{"主選擇單", "選擇年代", "載入進度", "音樂欣賞"} {
			got, want := NewCanvasPx(640, 408, face), NewCanvasPx(640, 408, face)
			DrawMenuLabel(got, s, fg)
			for i, r := range []rune(s) {
				want.DrawRuneWidePx(80, 242+24*i, r, fg, 2)
			}
			if !bytes.Equal(got.Img.Pix, want.Img.Pix) {
				t.Fatalf("%s %q changed original four cells", name, s)
			}
		}
	}
}

func TestMenuLabelJapaneseFifthCharacterStaysAboveDecoration(t *testing.T) {
	c := NewCanvasPx(640, 408, testFace(t))
	l := menuLabelGeometry(c, "年代を選ぶ")
	if l.text != "年代を選ぶ" || l.step != 18 || l.y != 242 || l.height != 88 {
		t.Fatalf("five-character layout: %+v", l)
	}
	got, want := NewCanvasPx(640, 408, testFace(t)), NewCanvasPx(640, 408, testFace(t))
	DrawMenuLabel(got, l.text, fg)
	for i, r := range []rune(l.text) {
		want.DrawRuneWidePx(80, 242+18*i, r, fg, 2)
	}
	if !bytes.Equal(got.Img.Pix, want.Img.Pix) {
		t.Fatal("fifth character was not fully drawn in the fifth safe cell")
	}
}

func TestMenuLabelEnglishEraUsesCompleteExistingSmallFont(t *testing.T) {
	face := testFace(t)
	small := menuLabelSmallFace(t)
	c := NewCanvasPx(640, 408, face)
	c.SetSmallFace(small)
	l := menuLabelGeometry(c, "Choose an era")
	if !l.rotated || !l.small || l.text != "Choose an era" || l.width != 10 || l.height != 78 || l.x != 91 || l.y != 247 {
		t.Fatalf("era layout: %+v", l)
	}
	horizontal := NewCanvasPx(78, 10, face)
	horizontal.SetSmallFace(small)
	horizontal.DrawSmallTextPx(0, 0, "Choose an era", fg)
	want := image.NewRGBA(c.Img.Bounds())
	for y := 0; y < 10; y++ {
		for x := 0; x < 78; x++ {
			if horizontal.Img.RGBAAt(x, y) == fg {
				want.SetRGBA(91+9-y, 247+x, fg)
			}
		}
	}
	DrawMenuLabel(c, "Choose an era", fg)
	if !bytes.Equal(c.Img.Pix, want.Pix) {
		t.Fatal("complete small text rotation differs from the existing horizontal font")
	}
}

func TestMenuLabelLongTitlesElideBeforePainting(t *testing.T) {
	for _, s := range []string{"Choose a very long scenario description", "選擇非常長的年代標題"} {
		c := NewCanvasPx(640, 408, testFace(t))
		c.SetSmallFace(menuLabelSmallFace(t))
		l := menuLabelGeometry(c, s)
		if l.text == s || !(strings.HasSuffix(l.text, "...") || strings.HasSuffix(l.text, "…")) {
			t.Fatalf("long title has no explicit ellipsis: %q", l.text)
		}
		DrawMenuLabel(c, s, fg)
		x0, y0, x1, y1, n := inkBox(c, fg)
		if n == 0 || !image.Rect(x0, y0, x1+1, y1+1).In(menuLabelSafe) || c.Clipped != 0 {
			t.Fatalf("long title escaped safe area: %q", s)
		}
	}
}

func TestMenuLabelEnglishSecondaryLabelsAreDrawn(t *testing.T) {
	c1, c3 := artContainers(t)
	ts, err := NewTitleScreen(c3, c1)
	if err != nil {
		t.Fatal(err)
	}
	for _, s := range []string{"Choose an era", "Load", "Music"} {
		got, plain := NewCanvasPx(640, 408, testFace(t)), NewCanvasPx(640, 408, testFace(t))
		got.SetSmallFace(menuLabelSmallFace(t))
		DrawTitleLayer(got, ts, 0, s, ScenarioLabelInk, nil, -1)
		DrawTitleLayer(plain, ts, 0, "", ScenarioLabelInk, nil, -1)
		changed := 0
		for y := 0; y < 408; y++ {
			for x := 0; x < 640; x++ {
				if got.Img.RGBAAt(x, y) != plain.Img.RGBAAt(x, y) {
					changed++
					if !image.Pt(x, y).In(menuLabelSafe) {
						t.Fatalf("%q changed non-label pixel (%d,%d)", s, x, y)
					}
				}
			}
		}
		if changed == 0 {
			t.Fatalf("secondary label %q suppressed", s)
		}
	}
}

func TestMenuLabelUnavailableSmallFontStillBoundsText(t *testing.T) {
	c := NewCanvasPx(640, 408, testFace(t))
	s := "Choose an era"
	l := menuLabelGeometry(c, s)
	if l.small || !strings.HasSuffix(l.text, "...") {
		t.Fatalf("unavailable small-font fallback: %+v", l)
	}
	DrawMenuLabel(c, s, fg)
	x0, y0, x1, y1, n := inkBox(c, fg)
	if n == 0 || !image.Rect(x0, y0, x1+1, y1+1).In(menuLabelSafe) {
		t.Fatal("fallback escaped safe area")
	}
}

func TestMenuLabelOversizeFontDoesNotPaintOutsideBody(t *testing.T) {
	face, err := font.ParseHex(strings.NewReader("004D:"+strings.Repeat("FF", 33)+"\n"), 33)
	if err != nil {
		t.Fatal(err)
	}
	c := NewCanvasPx(assets.ScreenW, assets.ScreenH, face)
	DrawMenuLabel(c, "M", color.RGBA{255, 127, 63, 255})
	if c.Clipped != 1 {
		t.Fatalf("unsupported face diagnostic = %d", c.Clipped)
	}
	if !bytes.Equal(c.Img.Pix, make([]byte, len(c.Img.Pix))) {
		t.Fatal("oversize rotated glyph painted outside the title body")
	}
}
