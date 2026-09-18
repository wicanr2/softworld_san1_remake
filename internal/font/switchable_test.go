package font

import (
	"os"
	"testing"
)

// TestSwitchableFontsCoverTheSameRunes 釘住主選單那兩套字型（Issue #71）
// **與 `unifont` 涵蓋一模一樣的碼位、一模一樣的字寬**。
//
// 少一個碼位在畫面上是空白，而空白看起來像排版問題不像缺字
// （`fonts/README.md`）；字寬變了整個版面跟著跑掉——版面是按格算的
// （`internal/cells`），而兩者在畫面上都不會報錯。
func TestSwitchableFontsCoverTheSameRunes(t *testing.T) {
	load := func(name string) *Face {
		t.Helper()
		fh, err := os.Open("../../fonts/" + name)
		if err != nil {
			t.Fatalf("%s 開不起來：%v", name, err)
		}
		defer fh.Close()
		f, err := ParseHexGz(fh, 16)
		if err != nil {
			t.Fatalf("%s 讀不進來：%v", name, err)
		}
		return f
	}
	base := load("unifont.hex.gz")
	for _, name := range []string{"kai.hex.gz", "li.hex.gz"} {
		f := load(name)
		if f.Len() != base.Len() {
			t.Errorf("%s 有 %d 個字，unifont 有 %d 個", name, f.Len(), base.Len())
		}
		missing, wide := 0, 0
		var firstMissing rune
		for _, r := range base.Runes() {
			g0, _ := base.Glyph(r)
			g, ok := f.Glyph(r)
			if !ok {
				if missing == 0 {
					firstMissing = r
				}
				missing++
				continue
			}
			if g.W != g0.W || g.H != g0.H {
				wide++
			}
		}
		if missing > 0 {
			t.Errorf("%s 少了 %d 個碼位，第一個是 U+%04X", name, missing, firstMissing)
		}
		if wide > 0 {
			t.Errorf("%s 有 %d 個字的尺寸與 unifont 不同", name, wide)
		}
	}
}

// TestSwitchableFontsAreNotUnifont 是反向對照：兩套字型如果只是把 unifont
// 複製一份，上面那支照樣全綠——那就等於「選項沒作用」。
func TestSwitchableFontsAreNotUnifont(t *testing.T) {
	load := func(name string) *Face {
		fh, err := os.Open("../../fonts/" + name)
		if err != nil {
			t.Fatal(err)
		}
		defer fh.Close()
		f, err := ParseHexGz(fh, 16)
		if err != nil {
			t.Fatal(err)
		}
		return f
	}
	base := load("unifont.hex.gz")
	// 這幾個字三套都有，而且是這一款畫面上實際會出現的（郡名、指令）。
	probe := []rune("三國演義劉備曹操內政軍事人事君主謀略其他")
	for _, name := range []string{"kai.hex.gz", "li.hex.gz"} {
		f := load(name)
		same := 0
		for _, r := range probe {
			g0, ok0 := base.Glyph(r)
			g, ok := f.Glyph(r)
			if !ok0 || !ok {
				t.Fatalf("%q 在 unifont 或 %s 裡沒有", r, name)
			}
			if sameRows(g0, g) {
				same++
			}
		}
		if same == len(probe) {
			t.Errorf("%s 的 %d 個取樣字與 unifont 逐點相同——這一套根本沒換到字模",
				name, same)
		}
	}
}

func sameRows(a, b Glyph) bool {
	if a.W != b.W || a.H != b.H {
		return false
	}
	for y := range a.Rows {
		for x := range a.Rows[y] {
			if a.Rows[y][x] != b.Rows[y][x] {
				return false
			}
		}
	}
	return true
}
