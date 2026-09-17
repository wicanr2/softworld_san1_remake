package opening

import (
	"image"
	"image/color"
	imgpng "image/png"
	"os"
	"path/filepath"
	"testing"

	"github.com/wicanr2/softworld_san1_remake/internal/assets"
	"github.com/wicanr2/softworld_san1_remake/internal/font"
)

func data1(t *testing.T) *assets.Container {
	t.Helper()
	root := os.Getenv("SAN1_ORIG")
	if root == "" {
		t.Skip("沒設 SAN1_ORIG，跳過（本儲存庫不含原版檔案）")
	}
	dir := filepath.Join(root, "三國演義")
	read := func(ext string) []byte {
		b, err := os.ReadFile(filepath.Join(dir, "DATA1."+ext))
		if err != nil {
			t.Skipf("讀不到 DATA1.%s：%v", ext, err)
		}
		return b
	}
	c, err := assets.OpenContainer(read("NAM"), read("IDX"), read("GRP"))
	if err != nil {
		t.Fatal(err)
	}
	return c
}

func face(t *testing.T) *font.Face {
	t.Helper()
	fh, err := os.Open("../../fonts/unifont.hex.gz")
	if err != nil {
		t.Skipf("沒有字型檔：%v", err)
	}
	defer fh.Close()
	f, err := font.ParseHexGz(fh, 16)
	if err != nil {
		t.Fatal(err)
	}
	return f
}

// blankArt 是尺寸對、內容全黑的一份圖，給不需要原版素材的測試。
func blankArt() *Art {
	im := func(w, h int) *assets.Image {
		return &assets.Image{W: w, H: h, Pix: make([]byte, w*h)}
	}
	a := &Art{
		CMarkL: im(320, 290), CMarkR: im(320, 290), TitleFont: im(464, 164),
		SantL: im(320, 295), SantR: im(320, 295),
		PoemInk: im(PoemW, PoemH), PoemMask: im(PoemW, PoemH), Loads: im(240, 20),
	}
	for i := range a.Credits {
		a.Credits[i] = im(512, 80)
	}
	for i, wh := range [][2]int{{40, 64}, {24, 32}, {16, 20}, {16, 16}} {
		a.Boats[i] = im(wh[0], wh[1])
	}
	for i := range a.Faces {
		a.Faces[i] = im(64, 80)
	}
	for i := range a.Titl {
		a.Titl[i] = im(160, 400)
	}
	return a
}

// TestBeatsFollowTheOriginalSequence 釘住片頭的拍數與順序：
// 原版 `TestZZOpeningMatchesTheOriginal` 量到的 1,308 拍。
func TestBeatsFollowTheOriginalSequence(t *testing.T) {
	s := &Script{Art: blankArt(), Rand: func(int) int { return 0 }}
	counts := map[Site]int{}
	last := Site(-1)
	n := 0
	for b := range s.Beats() {
		// 寫詞的「六列」與「一欄」兩種拍本來就交錯。
		if b.Site < last && !(b.Site == SitePoemRow && last == SitePoemColumn) {
			t.Fatalf("第 %d 拍［%s］排在［%s］後面", n, b.Site, last)
		}
		if b.Site > last {
			last = b.Site
		}
		counts[b.Site]++
		n++
	}
	want := map[Site]int{
		SiteTrademark: 1, SiteTitleFont: 1, SiteCredits: 1, SiteCreditsHold: 1, SiteSea: 1,
		SiteBoat: 22, SiteBoatsHold: 1, SitePoemRow: 247, SitePoemColumn: 11, SiteFade: 16,
		SiteRibbon: 920, SiteScroll: 80, SiteTitleHold: 4, SiteTitleKey: 1, SiteLoading: 1,
	}
	for site, w := range want {
		if counts[site] != w {
			t.Errorf("［%s］%d 拍，原版 %d 拍", site, counts[site], w)
		}
	}
	if n != 1308 {
		t.Errorf("共 %d 拍，原版 1308 拍", n)
	}
	if last != SiteLoading {
		t.Errorf("最後一拍是［%s］", last)
	}
}

// TestAbortSkipsToLoading：動畫那一步收到按鍵，片頭剩下的跳過、直接到載入中。
func TestAbortSkipsToLoading(t *testing.T) {
	s := &Script{Art: blankArt(), Rand: func(int) int { return 0 }}
	var sites []Site
	for b := range s.Beats() {
		sites = append(sites, b.Site)
		if b.Site == SiteRibbon {
			s.Aborted = true
		}
	}
	if got := sites[len(sites)-2]; got != SiteRibbon {
		t.Errorf("放棄前一拍是［%s］，應該是頭像橫幅", got)
	}
	if got := sites[len(sites)-1]; got != SiteLoading {
		t.Errorf("最後一拍是［%s］", got)
	}
}

// TestFontPoemInkMatchesTheOriginal：remake 字庫畫的詞與原版 `TZUE` 比字格的墨
// ——每個字框兩邊都有墨，字框外兩邊都沒有；而且 remake 的字與黑影全部
// 落在原版逐列揭露的範圍裡（落在外面的格子寫詞時就不會出現）。
func TestFontPoemInkMatchesTheOriginal(t *testing.T) {
	c1 := data1(t)
	orig, _, err := OriginalPoem(c1)
	if err != nil {
		t.Fatal(err)
	}
	ink, mask := FontPoem(face(t))
	gw, gh := PoemGlyphSize()
	inBox := make([]bool, PoemW*PoemH)
	boxes := 0
	for col, text := range PoemColumns() {
		for row := range []rune(text) {
			bx, by := PoemGlyphBox(col, row)
			bx, by = bx-PoemX, by-PoemY
			o, r := 0, 0
			for y := by; y < by+gh; y++ {
				for x := bx; x < bx+gw; x++ {
					if y >= PoemH {
						continue // 第七個字的字框比圖低一列
					}
					inBox[y*PoemW+x] = true
					if orig.At(x, y) != 0 {
						o++
					}
					if ink.At(x, y) != 0 {
						r++
					}
				}
			}
			if o == 0 || r == 0 {
				t.Errorf("第 %d 欄第 %d 字：原版 %d 格墨，remake %d 格", col, row, o, r)
			}
			boxes++
		}
	}
	for i := range inBox {
		if inBox[i] {
			continue
		}
		if orig.Pix[i] != 0 || ink.Pix[i] != 0 {
			t.Fatalf("字框外 (%d,%d) 有墨：原版 %d remake %d", i%PoemW, i/PoemW, orig.Pix[i], ink.Pix[i])
		}
		if mask.Pix[i] != 15 {
			t.Fatalf("字框外 (%d,%d) 的遮罩是 %d", i%PoemW, i/PoemW, mask.Pix[i])
		}
	}
	// 揭露範圍：第 col 欄搬 x 的位元組起 5 個位元組、列 [from, to)。
	revealed := func(sx, sy int) bool {
		for c := range poemRevealX {
			x0 := poemRevealX[c] >> 3 * 8
			x1 := x0 + ((39>>3)+1)*8
			if sx >= x0 && sx < x1 && sy >= poemRevealFrom[c] && sy < poemRevealTo[c] {
				return true
			}
		}
		return false
	}
	for y := 0; y < PoemH; y++ {
		for x := 0; x < PoemW; x++ {
			if mask.At(x, y) != 0 {
				continue
			}
			for _, d := range []int{0, 2} {
				if sx, sy := PoemX+x+d, PoemY+y+d; !revealed(sx, sy) {
					t.Fatalf("remake 的字（位移 %d）落在 (%d,%d)，原版不會揭露那一格", d, sx, sy)
				}
			}
		}
	}
	t.Logf("%d 個字框，兩邊每框都有墨、框外都沒有；remake 的字與黑影都在揭露範圍裡", boxes)
}

// DOSBox-X 的片頭連拍（`SAN1_DOSBOX_MODE=opening SAN1_DOSBOX_OUT=workplace/shots/dosboxx-opening
// tools/dosboxx.sh`）：製作人員停住的那一段、海景第一格、船隊最後一格。
var dosboxxOpening = []struct {
	file string
	site Site
}{
	{"opening-a-030.png", SiteCredits},
	{"opening-a-064.png", SiteSea},
	{"opening-a-110.png", SiteBoatsHold},
}

// TestOpeningMatchesDosboxX 拿 DOSBox-X 驗片頭三張（Issue #61）：dosgolem 那一邊是
// `TestZZOpeningMatchesTheOriginal`，這一支是獨立的第二個實作，
// 整張 640×408 照調色盤換成 RGB 後逐點相同才算數。
func TestOpeningMatchesDosboxX(t *testing.T) {
	shots := map[Site]image.Image{}
	for _, d := range dosboxxOpening {
		path := filepath.Join("../../workplace/shots/dosboxx-opening", d.file)
		f, err := os.Open(path)
		if err != nil {
			t.Skipf("沒有 %s（跑 SAN1_DOSBOX_MODE=opening tools/dosboxx.sh 產）", path)
		}
		im, err := imgpng.Decode(f)
		f.Close()
		if err != nil {
			t.Fatal(err)
		}
		shots[d.site] = im
	}
	art, err := LoadArt(data1(t))
	if err != nil {
		t.Fatal(err)
	}
	art.PoemInk, art.PoemMask = FontPoem(face(t))
	s := &Script{Art: art, Rand: func(int) int { return 0 }}
	for b := range s.Beats() {
		shot, ok := shots[b.Site]
		if !ok {
			continue
		}
		delete(shots, b.Site)
		pal := b.Pages.Palette()
		vis := b.Pages.Visible()
		bad := 0
		for y := 0; y < assets.ScreenH; y++ {
			for x := 0; x < assets.ScreenW; x++ {
				a := color.RGBAModel.Convert(shot.At(x, y)).(color.RGBA)
				if a != pal[vis.At(x, y)&15] {
					bad++
				}
			}
		}
		if bad != 0 {
			t.Errorf("［%s］與 DOSBox-X 差 %d 點", b.Site, bad)
		} else {
			t.Logf("［%s］與 DOSBox-X 整張 %d×%d 逐點相同", b.Site, assets.ScreenW, assets.ScreenH)
		}
		if len(shots) == 0 {
			break
		}
	}
}

// TestDefaultPaletteIsTheEGAPalette：屬性暫存器的預設值換出來就是 16 色表。
func TestDefaultPaletteIsTheEGAPalette(t *testing.T) {
	p := NewPages()
	if got := p.Palette(); got != assets.EGAPalette {
		t.Errorf("預設調色盤 %v，EGAPalette %v", got, assets.EGAPalette)
	}
}
