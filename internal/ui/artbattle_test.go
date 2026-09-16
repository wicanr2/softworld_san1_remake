package ui

import (
	"fmt"
	"image/color"
	imgpng "image/png"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/wicanr2/softworld_san1_remake/internal/assets"
	"github.com/wicanr2/softworld_san1_remake/internal/battle"
	"github.com/wicanr2/softworld_san1_remake/internal/cells"
	"github.com/wicanr2/softworld_san1_remake/internal/i18n"
	"github.com/wicanr2/softworld_san1_remake/internal/state"
)

// artShotPath 是原版紮完寨之後的主戰場，由 `internal/parity` 的
// `TestZZBattleKeySweep` 配合 `SAN1_BATTLESHOT=orig-battle` 產。
const artShotPath = "../../workplace/shots/bf/orig-battle.png"

// artContainer 開一個容器；沒有原版素材就 skip。
func artContainer(t *testing.T, name string) *assets.Container {
	t.Helper()
	root := os.Getenv("SAN1_ORIG")
	if root == "" {
		t.Skip("沒設 SAN1_ORIG，跳過（本儲存庫不含原版檔案）")
	}
	dir := filepath.Join(root, "三國演義")
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

// artContainers 開 DATA1／DATA3。
func artContainers(t *testing.T) (*assets.Container, *assets.Container) {
	t.Helper()
	return artContainer(t, "DATA1"), artContainer(t, "DATA3")
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
	if b := shot.Bounds(); b.Dx() != assets.ScreenW || b.Dy() != assets.ScreenH {
		t.Fatalf("主戰場基準 %s 是 %d×%d，應為 %d×%d",
			artShotPath, b.Dx(), b.Dy(), assets.ScreenW, assets.ScreenH)
	}
	c1, c3 := artContainers(t)
	// 場地要用**基準畫面那個郡**的地形（廬陵，25）：邊框畫在圖塊底下，
	// 拿別的地圖比會在錯開的欄位那裡整片對不上。
	c2 := artContainer(t, "DATA2")
	sc, err := state.LoadScenario(c2, state.Slot("001"))
	if err != nil {
		t.Fatal(err)
	}
	pref, err := sc.Prefecture(25)
	if err != nil {
		t.Fatal(err)
	}
	ab, err := NewArtBattle(c1, c3)
	if err != nil {
		t.Fatal(err)
	}
	fld := battle.Generate(battle.Params{Prefecture: 25})
	b := battle.New(battle.Setup{Field: fld, Seed: 1})
	im := ab.compose(b, BattleView{}, ArtBattleInfo{
		Field:    pref.BattleField,
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
	same("左欄右側", 40, 34, 55, 262)
	// 整片場地：地形、階梯邊框、欄與欄之間露出來的底紋，一次比完。
	//
	// 五支部隊站的格子整格挖掉：那一張是**紮完寨**的畫面，旗與兵力牌
	// 蓋在上面，而且紮過寨的格子連地形都變了（守軍那一格在原版是
	// 洋紅網點，靜態地圖上寫的是平原）。
	{
		marks := [][2]int{{5, 2}, {5, 3}, {6, 3}, {6, 1}, {6, 6}}
		skip := func(x, y int) bool {
			for _, m := range marks {
				cx, cy := assets.FieldCell(m[0], m[1])
				if x >= cx && x < cx+assets.TileW &&
					y >= cy && y < cy+assets.TileH+assets.FlagPlateH {
					return true
				}
			}
			return false
		}
		bad, n := 0, 0
		for y := 34; y <= 261; y++ {
			for x := 54; x <= 631; x++ {
				if skip(x, y) {
					continue
				}
				n++
				a := assets.EGAPalette[im.At(x, y)&15]
				o := color.RGBAModel.Convert(shot.At(x, y)).(color.RGBA)
				if a != o {
					bad++
				}
			}
		}
		if bad != 0 {
			minx, miny, maxx, maxy := 9999, 9999, -1, -1
			for y := 34; y <= 261; y++ {
				for x := 54; x <= 631; x++ {
					if skip(x, y) {
						continue
					}
					a := assets.EGAPalette[im.At(x, y)&15]
					o := color.RGBAModel.Convert(shot.At(x, y)).(color.RGBA)
					if a != o {
						if x < minx {
							minx = x
						}
						if y < miny {
							miny = y
						}
						if x > maxx {
							maxx = x
						}
						if y > maxy {
							maxy = y
						}
					}
				}
			}
			t.Logf("對不上的範圍：(%d,%d)–(%d,%d)", minx, miny, maxx, maxy)
			t.Errorf("整片場地：%d／%d 格對不上", bad, n)
		} else {
			t.Logf("整片場地 %d 格逐格相同", n)
		}
	}

	// 場地的上緣與階梯：地圖的圖塊從 y ＝ 36 起，34–35 兩列只有邊框。
	same("場地上緣", 54, 34, 631, 35)
	// 第一個階梯：偶數欄的右緣白線與奇數欄還沒開始的那一段。
	same("階梯", 104, 36, 105, 51)

	// 場地的下緣：白線那兩列。
	same("場地下緣", 54, 260, 631, 261)

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

// TestArtBattlePortraitsMatchTheOriginal 把基準畫面裡兩個面板的肖像對回
// 原版：原版主戰場的兩個面板各有一張統帥的肖像（`orig-battle.png` 是
// 陳就對周瑜），攻方那張左右翻、守方那張直放（`assets.BattleFaceMirror`）。
//
// 基準畫面是誰打誰不寫死——在 `DATA1` 的所有 `F%03d.FAC` 裡找**逐像素
// 相同**的那一張（照那一側該不該翻），找得到就代表位置、翻不翻與解碼
// 都對；再拿找到的編號叫 `compose`，那一塊要與基準畫面逐點相同——
// 這才是 remake 真正畫肖像的那條路。
func TestArtBattlePortraitsMatchTheOriginal(t *testing.T) {
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
	var found [2]int
	for side := range assets.BattleFaceX {
		found[side] = -1
		x0, y0 := assets.BattleFaceX[side], assets.BattleFaceY
		for n := 0; n < 400; n++ {
			face := ab.face(n)
			if face == nil {
				continue
			}
			if assets.BattleFaceMirror[side] {
				face = face.Mirror()
			}
			ok := true
			for y := 0; y < face.H && ok; y++ {
				for x := 0; x < face.W; x++ {
					a := assets.EGAPalette[face.At(x, y)&15]
					o := color.RGBAModel.Convert(shot.At(x0+x, y0+y)).(color.RGBA)
					if a != o {
						ok = false
						break
					}
				}
			}
			if ok {
				if found[side] >= 0 {
					t.Errorf("第 %d 側的肖像同時對上 F%03d 與 F%03d", side, found[side], n)
				}
				found[side] = n
			}
		}
		if found[side] < 0 {
			t.Errorf("第 %d 側 (%d,%d) 起的那一塊在 DATA1 的肖像裡找不到逐像素相同的（翻 %v）",
				side, x0, y0, assets.BattleFaceMirror[side])
		}
	}
	t.Logf("基準畫面的肖像：攻方 F%03d、守方 F%03d", found[0], found[1])
	if found[0] < 0 || found[1] < 0 {
		return
	}
	// 反過來走 remake 的路：拿找到的編號畫，肖像那兩塊要與基準逐點相同。
	fld := battle.Generate(battle.Params{Prefecture: 25})
	b := battle.New(battle.Setup{Field: fld, Seed: 1})
	im := ab.compose(b, BattleView{}, ArtBattleInfo{Portrait: found})
	for side := range assets.BattleFaceX {
		face := ab.face(found[side])
		bad := 0
		for y := 0; y < face.H; y++ {
			for x := 0; x < face.W; x++ {
				px, py := assets.BattleFaceX[side]+x, assets.BattleFaceY+y
				a := assets.EGAPalette[im.At(px, py)&15]
				o := color.RGBAModel.Convert(shot.At(px, py)).(color.RGBA)
				if a != o {
					bad++
				}
			}
		}
		if bad > 0 {
			t.Errorf("第 %d 側畫出來的肖像有 %d 點與基準不同", side, bad)
		}
	}
}

// TestArtBattleTextCellsMatchTheOriginal 把主戰場面板與左欄的**每一格字**
// 對回基準畫面（廬陵，陳就一軍三千兵五千金打蒯越軍的周瑜四軍七千八百兵
// 五百金）：郡名兩格 32×32、兩邊統帥名各兩格 32×32、五行資料的每一格
// 16×16——**格內有墨、格外那一圈沒有墨**，兩邊要同時成立。字模不比
// （remake 自建字庫），比的是落點與字級（Issue #48）。
func TestArtBattleTextCellsMatchTheOriginal(t *testing.T) {
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
	saved := i18n.Current
	defer func() { i18n.Current = saved }()
	i18n.Current = i18n.ZhHant

	att := []battle.Leader{{Index: 228, Name: "陳就", War: 60, Intel: 50, Stamina: 100,
		Charm: 50, Soldiers: 3000, Training: 60, Arms: 60, Troop: battle.TroopLand}}
	var def []battle.Leader
	for i, n := range []int{2000, 2000, 2000, 1800} {
		def = append(def, battle.Leader{Index: 14 + i, Name: "周瑜", War: 70, Intel: 90,
			Stamina: 100, Charm: 80, Soldiers: n, Training: 60, Arms: 60, Troop: battle.TroopLand})
	}
	b := battle.New(battle.Setup{
		Field: battle.Generate(battle.Params{Prefecture: 25}), Seed: 1,
		Attackers: att, Defenders: def,
		AttackerGold: 5000, DefenderGold: 500,
	})
	c := testCanvasPx(t, assets.ScreenW, assets.ScreenH)
	c.SetSmallFace(testSmallFace(t))
	DrawArtBattle(c, ab, b, BattleView{}, ArtBattleInfo{
		Prefecture: "廬陵", Province: "揚州", ID: 25, Portrait: [2]int{-1, -1},
		Commander: [2]string{"陳就", "周瑜"}, Lord: [2]string{"陳就", "蒯越"},
	})

	// 一格：(x, y, 寬, 高, 底色, 所在的文字區)。墨 ＝ 不是底色的像素；
	// 文字區之外（面板的框、肖像、左欄的框線）不算。
	type cell struct {
		name       string
		x, y, w, h int
		paper      int
		region     [4]int // x0, y0, x1, y1（含）
	}
	var cellsToCheck []cell
	nameRegion := [4]int{assets.BattleNameX, assets.BattleNameY,
		assets.BattleNameX + 31, assets.BattleNameY + 2*assets.BattleNameStep - 1}
	for i, r := range []rune("廬陵") {
		cellsToCheck = append(cellsToCheck, cell{"郡名" + string(r),
			assets.BattleNameX, assets.BattleNameY + i*assets.BattleNameStep, 32, 32,
			assets.BattleOrderPaper, nameRegion})
	}
	for side, name := range []string{"陳就", "周瑜"} {
		nx := assets.BattlePanelNameX[side]
		region := [4]int{nx, assets.BattlePanelY, nx + 31, assets.BattlePanelY + assets.BattlePanelH - 1}
		for k, r := range []rune(name) {
			cellsToCheck = append(cellsToCheck, cell{"統帥" + string(r),
				nx, assets.BattlePanelY + 16 + k*32, 32, 32, assets.BattlePanelPaper, region})
		}
	}
	lines := [2][]string{
		{" 陳就 軍", " 主攻軍 ", "一軍 1將", "兵  3000", "金  5000"},
		{" 蒯越 軍", " 主守軍 ", "四軍 4將", "兵  7800", "金   500"},
	}
	for side := range lines {
		tx := assets.BattlePanelTextX[side]
		region := [4]int{tx, assets.BattlePanelY, tx + 63, assets.BattlePanelLineY(5) - 1}
		for k, line := range lines[side] {
			x := tx
			for _, r := range line {
				w := cells.RuneWidth(r) * CellW
				if r != ' ' {
					cellsToCheck = append(cellsToCheck, cell{fmt.Sprintf("面板%d行%d「%c」", side, k+1, r),
						x, assets.BattlePanelLineY(k), w, CellH, assets.BattlePanelPaper, region})
				}
				x += w
			}
		}
	}
	ink := func(im interface {
		At(x, y int) color.Color
	}, x, y, paper int) bool {
		if x < 0 || y < 0 || x >= assets.ScreenW || y >= assets.ScreenH {
			return false
		}
		return color.RGBAModel.Convert(im.At(x, y)).(color.RGBA) != assets.EGAPalette[paper]
	}
	inAnyCell := func(x, y int) bool {
		for _, k := range cellsToCheck {
			if x >= k.x && x < k.x+k.w && y >= k.y && y < k.y+k.h {
				return true
			}
		}
		return false
	}
	// inside：格內的墨；stray：格外那一圈上、而且不屬於任何一格的墨
	// （相鄰的字貼著是正常的，字模不同才會差在格的邊上）。
	count := func(im interface {
		At(x, y int) color.Color
	}, k cell) (inside, stray int) {
		for y := k.y - 1; y <= k.y+k.h; y++ {
			for x := k.x - 1; x <= k.x+k.w; x++ {
				if y >= assets.ScreenH || !ink(im, x, y, k.paper) {
					continue
				}
				if x < k.region[0] || x > k.region[2] || y < k.region[1] || y > k.region[3] {
					continue
				}
				switch {
				case x >= k.x && x < k.x+k.w && y >= k.y && y < k.y+k.h:
					inside++
				case !inAnyCell(x, y):
					stray++
				}
			}
		}
		return
	}
	for _, k := range cellsToCheck {
		oi, os := count(shot, k)
		mi, ms := count(c.Img, k)
		if oi == 0 {
			t.Errorf("%s：基準畫面那一格 (%d,%d) 沒有墨——格子讀錯了", k.name, k.x, k.y)
		}
		if mi == 0 {
			t.Errorf("%s：remake 那一格 (%d,%d) 沒有墨", k.name, k.x, k.y)
		}
		if os != 0 {
			t.Errorf("%s：基準畫面格 (%d,%d) 外那一圈有 %d 點不屬於任何一格——格子擺錯了", k.name, k.x, k.y, os)
		}
		if ms != 0 {
			t.Errorf("%s：remake 格 (%d,%d) 外那一圈有 %d 點不屬於任何一格", k.name, k.x, k.y, ms)
		}
	}
	t.Logf("比了 %d 格", len(cellsToCheck))
}
