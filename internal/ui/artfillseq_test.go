package ui

import (
	"image/color"
	imgpng "image/png"
	"os"
	"path/filepath"
	"testing"

	"github.com/wicanr2/softworld_san1_remake/internal/assets"
	"github.com/wicanr2/softworld_san1_remake/internal/save"
	"github.com/wicanr2/softworld_san1_remake/internal/state"
)

// TestPrefectureFillsInOrderMatchTheOriginal 把 42 個郡**依序灌進同一張圖**
// 再與原版比。
//
// 與 `TestPrefectureFillsMatchTheOriginal` 的差別只有一處：那一支給每個郡
// 一份底圖的副本，這一支共用一張。**兩者只有在地圖邊界有缺口時才會分岔**
// ——區域相連的話，後灌的郡會蓋掉先灌的，而「一郡一張」看不到這件事。
func TestPrefectureFillsInOrderMatchTheOriginal(t *testing.T) {
	f, err := os.Open("../../workplace/shots/orig-loaded.png")
	if err != nil {
		t.Skipf("沒有主畫面的基準畫面：%v", err)
	}
	shot, err := imgpng.Decode(f)
	f.Close()
	if err != nil {
		t.Fatal(err)
	}
	root := os.Getenv("SAN1_ORIG")
	if root == "" {
		t.Skip("沒設 SAN1_ORIG，跳過（本儲存庫不含原版檔案）")
	}
	dir := filepath.Join(root, "三國演義")
	open := func(name string) *assets.Container {
		read := func(ext string) []byte {
			b, err := os.ReadFile(filepath.Join(dir, name+"."+ext))
			if err != nil {
				t.Skipf("讀不到 %s.%s：%v", name, ext, err)
			}
			return b
		}
		c, err := assets.OpenContainer(read("NAM"), read("IDX"), read("GRP"))
		if err != nil {
			t.Fatal(err)
		}
		return c
	}
	c2, c3, c1 := open("DATA2"), open("DATA3"), open("DATA1")
	g, err := save.ReadOriginal(c2, 1, state.EditionBase)
	if err != nil {
		t.Fatalf("讀原版第一個進度：%v", err)
	}
	a, err := NewArtScreen(c3, c1)
	if err != nil {
		t.Fatal(err)
	}
	if a.fills == nil {
		t.Fatal("EGAFILL.PAL 沒讀進來")
	}

	all := a.base.Clone()
	for _, p := range g.Prefectures() {
		if p.Owner == state.NoFaction {
			continue
		}
		all.FloodFillPattern(int(p.MapX)+assets.MapOriginX,
			int(p.MapY)+assets.MapOriginY, &a.fills[int(p.Owner)%len(a.fills)])
	}

	// 只比被灌到的格子——沒灌到的地方有郡名、圖示與其他東西疊著。
	bad, n := 0, 0
	for y := 0; y < assets.ScreenH; y++ {
		for x := 0; x < assets.ScreenW; x++ {
			if all.At(x, y) == a.base.At(x, y) {
				continue
			}
			n++
			want := color.RGBAModel.Convert(shot.At(x, y)).(color.RGBA)
			if assets.EGAPalette[all.At(x, y)&15] != want {
				bad++
			}
		}
	}
	t.Logf("依序灌：填了 %d 格，其中 %d 格與原版不同（%.2f%%）",
		n, bad, float64(bad)*100/float64(n))
	// 郡的編號寫在填色上面，所以留 5% 的餘裕。
	if bad*20 > n {
		t.Errorf("依序灌之後仍有 %d/%d 格不同", bad, n)
	}
}
