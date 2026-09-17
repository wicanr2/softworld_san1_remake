//go:build oracle

package parity

import (
	"fmt"
	"image"
	"image/color"
	"os"
	"path/filepath"
	"testing"

	"github.com/wicanr2/softworld_san1_remake/internal/game"
	"github.com/wicanr2/softworld_san1_remake/internal/i18n"
	"github.com/wicanr2/softworld_san1_remake/internal/state"
	"github.com/wicanr2/softworld_san1_remake/internal/ui"
)

// TestZZDosboxScreensMatchTheRemake 是 #77–#95 那一串挑選畫面的 **DOSBox-X 交叉驗證**（Issue #96）。
//
// 為什麼不走 `TestZZDosgolemMatchesDosbox`：那一支的驅動配方是為 `rec7`／`rec10` 調的，
// 走到「難度 5」之後這條含防拷密碼的路就岔開（`rec18` 實測第 12 步起只剩 27–63% 相同）。
// 所以照 `rec11` 的做法——**DOSBox 的畫格直接對 remake 的算圖**：錄影那一輪走的是劇本 001、
// 一位玩家、劉備，盤面就是劇本檔本身，remake 從同一份資料重建再畫。
//
// 判準與 dosgolem 那幾支相同：右側面板的底與外框逐像素、字格只比有沒有墨、下面板比墨。
func TestZZDosboxScreensMatchTheRemake(t *testing.T) {
	dir := os.Getenv("SAN1_REC18")
	if dir == "" {
		dir = "../../workplace/rec18/frames"
	}
	dir2 := os.Getenv("SAN1_REC19")
	if dir2 == "" {
		dir2 = "../../workplace/rec19/frames"
	}
	root := origRoot(t)
	sc, err := state.LoadScenario(openContainer(t, filepath.Join(root, "DATA2")), state.Slot("001"))
	if err != nil {
		t.Fatal(err)
	}
	// 錄影那一輪選的是第 1 位（劉備）。
	lords := sc.ActiveFactions()
	if len(lords) == 0 {
		t.Fatal("劇本一沒有勢力")
	}
	me := state.FactionID(lords[0])
	g, err := game.New(sc, me, 5, state.EditionBase)
	if err != nil {
		t.Fatal(err)
	}
	lord := g.Lord(me)
	if lord == nil {
		t.Fatal("沒有君主")
	}
	at := lord.Location
	art, err := ui.NewArtScreen(openContainer(t, filepath.Join(root, "DATA3")), openContainer(t, filepath.Join(root, "DATA1")))
	if err != nil {
		t.Fatal(err)
	}
	face := loadFace(t)
	name := ui.NameField(i18n.PersonName(lord.Name))
	t.Logf("盤面：君主 %s（勢力 %d）在郡 %d %s，金 %d 米 %d", lord.Name, me, at,
		g.Prefecture(at).Name, g.Prefecture(at).Gold, g.Prefecture(at).Rice)

	pp := &ui.PrefPick{}
	for id := 1; id <= 42; id++ {
		pp.Valid[id] = true
	}
	in := ui.InputCursor{On: true}
	var roster ui.RosterPick
	roster.Key = game.PickByIntel
	for _, x := range g.PickRoster(at, game.PickServing, game.PickByIntel) {
		roster.List = append(roster.List, x.Index)
	}
	for _, c := range []struct {
		file string
		v    ui.View
		text []textArea
	}{
		{"022-1Return.png", ui.View{Sel: at, Status: true, Input: in,
			Prompt: i18n.Sf("ask.main", name, at, ui.PlaceName(g.Prefecture(at).Name))},
			[]textArea{{408, 36, 28, 16, -1}, {424, 300, 24, 4, -1}}},
		{"019-1Return.png", ui.View{Sel: at, Input: in, PrefPick: pp,
			Prompt: i18n.S("ask.pref") + i18n.Sf("pick.range", 1, 42)}, prefText},
		{"025-6Return.png", ui.View{Sel: at, Input: in, Prompt: i18n.S("msg.anyKey"),
			Treasury: &ui.TreasuryPanel{Faction: me}},
			[]textArea{{432, 52, 24, 1, 5}, {416, 68, 26, 5, 5}, {424, 300, 24, 4, -1}}},
		{filepath.Join(dir2, "017-1Return.png"), ui.View{Sel: at, Input: in, Roster: &roster,
			Prompt: i18n.S("ask.reclaim") + i18n.Sf("pick.range", 1, len(roster.List))}, rosterText},
	} {
		file := c.file
		if !filepath.IsAbs(file) && filepath.Dir(file) == "." {
			file = filepath.Join(dir, file)
		}
		orig, err := shotPixels(file)
		if err != nil {
			t.Skipf("沒有 DOSBox 參照畫面 %s（跑 tools/dosboxx-record.sh 產）：%v", file, err)
		}
		cv := ui.NewCanvasPx(scrW, scrH, face)
		ui.DrawArtSession(cv, art, g, nil, c.v)
		plain := cv
		comparePanels(t, filepath.Base(file), orig, cv, plain, cursorTrack{}, 1, c.text...)
	}
}

// shotPixels 把 DOSBox 存的 PNG 換成 EGA 十六色的索引，形狀與 `IndexedEGASize` 相同。
func shotPixels(path string) ([]uint8, error) {
	im, err := decodePNG(path)
	if err != nil {
		return nil, err
	}
	b, ok := im.(interface{ Bounds() image.Rectangle })
	if !ok {
		return nil, fmt.Errorf("%s：讀不到尺寸", path)
	}
	if r := b.Bounds(); r.Dx() != scrW || r.Dy() != scrH {
		return nil, fmt.Errorf("%s 是 %dx%d，不是 %dx%d", path, r.Dx(), r.Dy(), scrW, scrH)
	}
	pix := make([]uint8, scrW*scrH)
	for y := 0; y < scrH; y++ {
		for x := 0; x < scrW; x++ {
			q := color.RGBAModel.Convert(im.At(x, y)).(color.RGBA)
			i := inkIndex(q)
			if i < 0 {
				return nil, fmt.Errorf("%s (%d,%d) 的顏色 %v 不在 EGA 十六色裡", path, x, y, q)
			}
			pix[y*scrW+x] = uint8(i)
		}
	}
	return pix, nil
}
