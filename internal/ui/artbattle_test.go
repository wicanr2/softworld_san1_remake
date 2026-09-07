package ui

import (
	"image/color"
	imgpng "image/png"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/wicanr2/softworld_san1_remake/internal/assets"
	"github.com/wicanr2/softworld_san1_remake/internal/battle"
)

// artShotPath 是原版紮完寨之後的主戰場，由 `internal/parity` 的
// `TestZZBattleKeySweep` 產。
const artShotPath = "../../workplace/shots/bf/orig-battle.png"

// artContainers 開 DATA1／DATA3；沒有原版素材就 skip。
func artContainers(t *testing.T) (*assets.Container, *assets.Container) {
	t.Helper()
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
	return open("DATA1"), open("DATA3")
}

// TestArtBattleFurnitureMatchesTheOriginal 把畫面上**與局面無關**的部分
// 逐像素對回原版：底紋、左欄的黑邊、三個面板的外框。
//
// 部隊、文字、肖像會隨局面變，這裡不比——比得動的就要 100%。
func TestArtBattleFurnitureMatchesTheOriginal(t *testing.T) {
	f, err := os.Open(artShotPath)
	if err != nil {
		t.Skipf("沒有主戰場基準畫面 %s", artShotPath)
	}
	shot, err := imgpng.Decode(f)
	f.Close()
	if err != nil {
		t.Fatal(err)
	}
	c1, c3 := artContainers(t)
	ab, err := NewArtBattle(c1, c3)
	if err != nil {
		t.Fatal(err)
	}
	fld := battle.Generate(battle.Params{Prefecture: 25})
	b := battle.New(battle.Setup{Field: fld, Seed: 1})
	im := ab.compose(b, BattleView{}, ArtBattleInfo{
		Field:    make([]byte, battle.FieldBytes),
		Portrait: [2]int{-1, -1},
	})

	same := func(name string, x0, y0, x1, y1 int) {
		t.Helper()
		bad, n := 0, 0
		for y := y0; y <= y1; y++ {
			for x := x0; x <= x1; x++ {
				n++
				a := assets.EGAPalette[im.At(x, y)&15]
				o := color.RGBAModel.Convert(shot.At(x, y)).(color.RGBA)
				if a != o {
					bad++
				}
			}
		}
		if bad != 0 && testing.Verbose() {
			for y := y0; y <= y1; y++ {
				line := ""
				for x := x0; x <= x1; x++ {
					a := assets.EGAPalette[im.At(x, y)&15]
					o := color.RGBAModel.Convert(shot.At(x, y)).(color.RGBA)
					if a != o {
						line += "x"
					} else {
						line += "."
					}
				}
				if strings.Contains(line, "x") {
					t.Logf("  y=%d %s", y, line)
				}
			}
		}
		if bad != 0 {
			t.Errorf("%s（%d,%d）–（%d,%d）：%d／%d 格對不上", name, x0, y0, x1, y1, bad, n)
		}
	}
	// 底紋：畫面左緣那一條沒有別的東西蓋。
	same("底紋", 0, 180, 5, 260)
	// 左欄：四個小框的左右邊與它們之間露出來的底紋，還有場地的左緣。
	// 框內是郡名、天氣圖示這些會變的東西，不比。
	//
	// 右側只比到 y ＝ 258：**場地下緣那兩列是白色的立體邊**
	// （原版 `0x220f0` 每 96 像素一組畫出來的階梯狀邊框），
	// remake 目前把場地周圍一律留黑，那一段還沒接。
	same("左欄左側", 0, 34, 7, 262)
	same("左欄右側", 40, 34, 55, 258)
	// 三個面板的外框：上面兩條黑線。
	for i, x := range assets.BattlePanelX {
		same("面板外框", x-2, assets.BattlePanelY-2, x+assets.BattlePanelW, assets.BattlePanelY-1)
		if i == 2 {
			break
		}
	}
	// 肖像框的上片。
	for _, x := range assets.BattleFrameX {
		same("肖像框", x, assets.BattlePanelY, x+assets.BattleFrameW-1, assets.BattlePanelY+7)
	}
}
