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

// artShotNarrowPath 是窄版面的基準：同一支測試帶 `SAN1_BATTLETO=26`
// `SAN1_BATTLESHOT=orig-battle-narrow` 產（打郡 26，紮完寨）。
const artShotNarrowPath = "../../workplace/shots/bf/orig-battle-narrow.png"

// artNarrowMarks 是窄版面基準上有部隊站的格子（欄、列）：守方的帥與先
// 在第 5 欄、攻方的帥在第 0 欄。那一張的紅色網點格是郡 26 自己的地形，
// 不是紮的寨。
var artNarrowMarks = [][2]int{{5, 2}, {5, 3}, {0, 5}}

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
// 逐像素對回原版：底紋、左欄的黑邊、場地的邊框、三個面板的外框。
//
// 部隊、文字、肖像會隨局面變，這裡不比——比得動的就要 100%。
func TestArtBattleFurnitureMatchesTheOriginal(t *testing.T) {
	// 五支部隊站的格子整格挖掉：那一張是**紮完寨**的畫面，旗與兵力牌
	// 蓋在上面，而且紮過寨的格子連地形都變了（守軍那一格在原版是
	// 洋紅網點，靜態地圖上寫的是平原）。
	battleFurniture(t, artShotPath, 25, [][2]int{{5, 2}, {5, 3}, {6, 3}, {6, 1}, {6, 6}})
}

// TestArtBattleNarrowFurnitureMatchesTheOriginal 是窄版面（8 欄，面板疊在
// 右邊）的同一套檢查：基準是打郡 26 紮完寨的畫面（`docs/spec/005` §8
// 「窄版面」）。
func TestArtBattleNarrowFurnitureMatchesTheOriginal(t *testing.T) {
	battleFurniture(t, artShotNarrowPath, 26, artNarrowMarks)
}

// battleFurniture 拿 `prefecture` 的地形拼一張沒有部隊的主戰場，與基準
// 畫面 `shotPath` 逐像素比；marks 是基準上有部隊站的格子（欄、列），
// 整格連兵力牌一起挖掉不比。
func battleFurniture(t *testing.T, shotPath string, prefecture int, marks [][2]int) {
	t.Helper()
	f, err := os.Open(shotPath)
	if err != nil {
		t.Skipf("沒有主戰場基準畫面 %s", shotPath)
	}
	shot, err := imgpng.Decode(f)
	f.Close()
	if err != nil {
		t.Fatal(err)
	}
	if b := shot.Bounds(); b.Dx() != assets.ScreenW || b.Dy() != assets.ScreenH {
		t.Fatalf("主戰場基準 %s 是 %d×%d，應為 %d×%d",
			shotPath, b.Dx(), b.Dy(), assets.ScreenW, assets.ScreenH)
	}
	c1, c3 := artContainers(t)
	// 場地要用**基準畫面那個郡**的地形：邊框畫在圖塊底下，
	// 拿別的地圖比會在錯開的欄位那裡整片對不上。
	c2 := artContainer(t, "DATA2")
	sc, err := state.LoadScenario(c2, state.Slot("001"))
	if err != nil {
		t.Fatal(err)
	}
	pref, err := sc.Prefecture(prefecture)
	if err != nil {
		t.Fatal(err)
	}
	ab, err := NewArtBattle(c1, c3)
	if err != nil {
		t.Fatal(err)
	}
	fld, err := battle.Load(pref.BattleField, pref.Neighbours)
	if err != nil {
		t.Fatal(err)
	}
	b := battle.New(battle.Setup{Field: fld, Seed: 1})
	l := assets.BattleLayoutFor(fld.Narrow())
	t.Logf("郡 %d 的版面：窄 %v", prefecture, l.Narrow)
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
	fieldX1, fieldY1 := l.RightX+1, l.FieldY1()
	// 底紋：畫面左緣那一條沒有別的東西蓋。
	same("底紋", 0, 180, 5, 260)
	// 左欄：四個小框的左右邊與它們之間露出來的底紋，還有場地的左緣
	// （兩條黑線畫到 LeftY1）。框內是郡名、天氣圖示這些會變的東西，不比。
	same("左欄左側", 0, 34, 7, 262)
	same("左欄右側", 40, 34, 55, l.LeftY1+1)
	// 整片場地：地形、階梯邊框、欄與欄之間露出來的底紋、左右緣的線，
	// 一次比完。
	{
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
		minx, miny, maxx, maxy := 9999, 9999, -1, -1
		for y := 34; y <= fieldY1; y++ {
			for x := 54; x <= fieldX1; x++ {
				if skip(x, y) {
					continue
				}
				n++
				a := assets.EGAPalette[im.At(x, y)&15]
				o := color.RGBAModel.Convert(shot.At(x, y)).(color.RGBA)
				if a != o {
					bad++
					minx, miny = min(minx, x), min(miny, y)
					maxx, maxy = max(maxx, x), max(maxy, y)
				}
			}
		}
		if bad != 0 {
			t.Logf("對不上的範圍：(%d,%d)–(%d,%d)", minx, miny, maxx, maxy)
			// 逐格數一遍，好知道該挖掉哪些格子。
			for row := 0; row < battle.FieldH; row++ {
				line := ""
				for col := 0; col < battle.FieldW; col++ {
					cx, cy := assets.FieldCell(col, row)
					k := 0
					for y := cy; y < cy+assets.TileH+assets.FlagPlateH; y++ {
						for x := cx; x < cx+assets.TileW; x++ {
							if skip(x, y) || x > fieldX1 || y > fieldY1 {
								continue
							}
							a := assets.EGAPalette[im.At(x, y)&15]
							o := color.RGBAModel.Convert(shot.At(x, y)).(color.RGBA)
							if a != o {
								k++
							}
						}
					}
					line += fmt.Sprintf("%5d", k)
				}
				t.Logf("  列 %d：%s", row, line)
			}
			t.Errorf("整片場地：%d／%d 格對不上", bad, n)
		} else {
			t.Logf("整片場地 %d 格逐格相同", n)
		}
	}

	// 場地的上緣與階梯：地圖的圖塊從 y ＝ 36 起，34–35 兩列只有邊框。
	same("場地上緣", 54, 34, fieldX1, 35)
	// 第一個階梯：偶數欄的右緣白線與奇數欄還沒開始的那一段。
	same("階梯", 104, 36, 105, 51)
	// 場地的下緣：白線那兩列。
	same("場地下緣", 54, fieldY1-1, fieldX1, fieldY1)
	// 場地的右緣：兩條白線（`0x2255d`），(RightX,50) 是黑的。
	same("場地右緣", l.RightX, 34, l.RightX+1, fieldY1)

	// 三個面板的外框：上面兩條黑線。
	for i := range l.PanelX {
		x, y, _, _ := l.Panel(i)
		same("面板外框", x-2, y-2, x+assets.BattlePanelW, y-1)
	}
	// 肖像框的上片。
	for i := range assets.BattleFaceMirror {
		x, y := l.Frame(i)
		same("肖像框", x, y, x+assets.BattleFrameW-1, y+7)
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
	battlePortraits(t, artShotPath, 25)
}

// TestArtBattleNarrowPortraitsMatchTheOriginal 是窄版面的同一套：面板疊在
// 右邊，攻方的肖像在上面那一塊靠左、守方在中間那一塊靠右。
func TestArtBattleNarrowPortraitsMatchTheOriginal(t *testing.T) {
	battlePortraits(t, artShotNarrowPath, 26)
}

func battlePortraits(t *testing.T, shotPath string, prefecture int) {
	t.Helper()
	f, err := os.Open(shotPath)
	if err != nil {
		t.Skipf("沒有主戰場基準畫面 %s", shotPath)
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
	c2 := artContainer(t, "DATA2")
	sc, err := state.LoadScenario(c2, state.Slot("001"))
	if err != nil {
		t.Fatal(err)
	}
	pref, err := sc.Prefecture(prefecture)
	if err != nil {
		t.Fatal(err)
	}
	fld, err := battle.Load(pref.BattleField, pref.Neighbours)
	if err != nil {
		t.Fatal(err)
	}
	l := assets.BattleLayoutFor(fld.Narrow())
	var found [2]int
	for side := range assets.BattleFaceMirror {
		found[side] = -1
		x0, y0 := l.Face(side)
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
	b := battle.New(battle.Setup{Field: fld, Seed: 1})
	im := ab.compose(b, BattleView{}, ArtBattleInfo{Portrait: found})
	for side := range assets.BattleFaceMirror {
		face := ab.face(found[side])
		bad := 0
		fx, fy := l.Face(side)
		for y := 0; y < face.H; y++ {
			for x := 0; x < face.W; x++ {
				px, py := fx+x, fy+y
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
	battleTextCells(t, artShotPath, battleTextCase{
		prefecture: 25, name: "廬陵", province: "揚州",
		defenders: []int{2000, 2000, 2000, 1800},
		commander: [2]string{"陳就", "周瑜"}, lord: [2]string{"陳就", "蒯越"},
		rice: [2]int{8970, 2922},
		lines: [2][]string{
			{" 陳就 軍", " 主攻軍 ", "一軍 1將", "兵  3000", "金  5000", "米  8970"},
			{" 蒯越 軍", " 主守軍 ", "四軍 4將", "兵  7800", "金   500", "米  2922"},
		},
	})
}

// TestArtBattleNarrowTextCellsMatchTheOriginal 是窄版面的同一套（郡 26，
// 陳就一軍三千兵五千金打郭嘉軍的關羽二軍三千兵五百金；郡名在資料裡就叫「夷郡」）：面板疊在右邊，
// 統帥名與五行資料的落點跟著面板的左上角走。
func TestArtBattleNarrowTextCellsMatchTheOriginal(t *testing.T) {
	battleTextCells(t, artShotNarrowPath, battleTextCase{
		prefecture: 26, name: "夷郡", province: "揚州",
		defenders: []int{1500, 1500},
		commander: [2]string{"陳就", "關羽"}, lord: [2]string{"陳就", "郭嘉"},
		rice: [2]int{8970, 2970},
		lines: [2][]string{
			{" 陳就 軍", " 主攻軍 ", "一軍 1將", "兵  3000", "金  5000", "米  8970"},
			{" 郭嘉 軍", " 主守軍 ", "二軍 2將", "兵  3000", "金   500", "米  2970"},
		},
	})
}

// battleTextCase 是一張基準畫面上的局面：郡、兩邊的統帥與君主、守方各
// 將的兵數，以及面板上五行資料應該印出來的字。
type battleTextCase struct {
	prefecture      int
	name, province  string
	defenders       []int
	rice            [2]int
	commander, lord [2]string
	lines           [2][]string
}

func battleTextCells(t *testing.T, shotPath string, tc battleTextCase) {
	t.Helper()
	f, err := os.Open(shotPath)
	if err != nil {
		t.Skipf("沒有主戰場基準畫面 %s", shotPath)
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
	c2 := artContainer(t, "DATA2")
	sc, err := state.LoadScenario(c2, state.Slot("001"))
	if err != nil {
		t.Fatal(err)
	}
	pref, err := sc.Prefecture(tc.prefecture)
	if err != nil {
		t.Fatal(err)
	}
	if pref.Name != tc.name {
		t.Fatalf("郡 %d 叫 %q，測試以為是 %q", tc.prefecture, pref.Name, tc.name)
	}
	fld, err := battle.Load(pref.BattleField, pref.Neighbours)
	if err != nil {
		t.Fatal(err)
	}
	wide := assets.BattleLayoutFor(fld.Narrow())
	saved := i18n.Current
	defer func() { i18n.Current = saved }()
	i18n.Current = i18n.ZhHant

	att := []battle.Leader{{Index: 228, Name: tc.commander[0], War: 60, Intel: 50, Stamina: 100,
		Charm: 50, Soldiers: 3000, Training: 60, Arms: 60, Troop: battle.TroopLand}}
	var def []battle.Leader
	for i, n := range tc.defenders {
		def = append(def, battle.Leader{Index: 14 + i, Name: tc.commander[1], War: 70, Intel: 90,
			Stamina: 100, Charm: 80, Soldiers: n, Training: 60, Arms: 60, Troop: battle.TroopLand})
	}
	b := battle.New(battle.Setup{
		Field: fld, Seed: 1,
		Attackers: att, Defenders: def,
		AttackerGold: 5000, DefenderGold: 500,
		AttackerRice: tc.rice[0], DefenderRice: tc.rice[1],
	})
	// 兩張基準都是紮完寨、休息到第四天的畫面（左欄第四框「四／日／卯時／6」）。
	b.Day = 4
	c := testCanvasPx(t, assets.ScreenW, assets.ScreenH)
	c.SetSmallFace(testSmallFace(t))
	DrawArtBattle(c, ab, b, BattleView{}, ArtBattleInfo{
		Prefecture: tc.name, Province: tc.province, ID: tc.prefecture, Portrait: [2]int{-1, -1},
		Commander: tc.commander, Lord: tc.lord,
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
	for i, r := range []rune(tc.name) {
		cellsToCheck = append(cellsToCheck, cell{"郡名" + string(r),
			assets.BattleNameX, assets.BattleNameY + i*assets.BattleNameStep, 32, 32,
			assets.BattleOrderPaper, nameRegion})
	}
	// 左欄第四框：第四天——「四」在中格 32×16、「日」32×16、「卯」16×16、
	// 「時」16×32、「 6」的 6 在第二格 8×16（Issue #57）。
	dayRegion := [4]int{assets.BattleLeftBoxX0, 228, assets.BattleLeftBoxX1, 323}
	for _, k := range []cell{
		{"日數「四」", 8, 244, 32, 16, assets.BattleOrderPaper, dayRegion},
		{"「日」", 8, 276, 32, 16, assets.BattleOrderPaper, dayRegion},
		{"時辰「卯」", 8, 292, 16, 16, assets.BattleOrderPaper, dayRegion},
		{"「時」", 24, 292, 16, 32, assets.BattleOrderPaper, dayRegion},
		{"時數「6」", 16, 308, 8, 16, assets.BattleOrderPaper, dayRegion},
	} {
		cellsToCheck = append(cellsToCheck, k)
	}
	for side, name := range tc.commander {
		nx, py := wide.NameX(side), wide.PanelY[side]
		region := [4]int{nx, py, nx + 31, py + assets.BattlePanelH - 1}
		for k, r := range []rune(name) {
			cellsToCheck = append(cellsToCheck, cell{"統帥" + string(r),
				nx, py + 16 + k*32, 32, 32, assets.BattlePanelPaper, region})
		}
	}
	lines := tc.lines
	for side := range lines {
		tx := wide.TextX(side)
		region := [4]int{tx, wide.PanelY[side], tx + 63, wide.LineY(side, 6) - 1}
		for k, line := range lines[side] {
			x := tx
			for _, r := range line {
				w := cells.RuneWidth(r) * CellW
				if r != ' ' {
					cellsToCheck = append(cellsToCheck, cell{fmt.Sprintf("面板%d行%d「%c」", side, k+1, r),
						x, wide.LineY(side, k), w, CellH, assets.BattlePanelPaper, region})
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

// TestArtBattleUnitAndInspectPanelsStayInTheirColumns 釘住對戰子畫面的
// 部隊面板與查看那一塊（`docs/spec/005` §8「部隊面板」「查看」）在三個
// 語系下都畫得出來、而且字不壓到肖像：部隊面板的六行只落在資料那一欄
// （攻 176–239、守 256–319）與名字那一欄（144／320 起 32 寬），查看的
// 六行只落在 448–511、名字在 512–543，肖像那一塊 (544,268)–(623,363)
// 只有框與肖像的顏色，沒有字色。
func TestArtBattleUnitAndInspectPanelsStayInTheirColumns(t *testing.T) {
	c1, c3 := artContainers(t)
	ab, err := NewArtBattle(c1, c3)
	if err != nil {
		t.Fatal(err)
	}
	saved := i18n.Current
	defer func() { i18n.Current = saved }()
	fld := battle.Generate(battle.Params{Prefecture: 25})
	b := battle.New(battle.Setup{Field: fld, Seed: 1})
	l := assets.BattleWide
	mk := func(side battle.Side, name string, n int) *battle.Unit {
		u := &battle.Unit{Side: side, Formation: battle.Centre}
		for k := 0; k < n; k++ {
			u.Leaders = append(u.Leaders, battle.Leader{Index: 228 + k, Name: name, War: 78, Intel: 99,
				Stamina: 82, Soldiers: 3000, Training: 80, Arms: 80, Troop: battle.TroopLand})
		}
		return u
	}
	inkAt := func(c *Canvas, x0, y0, x1, y1 int, ink color.RGBA) int {
		n := 0
		for y := y0; y <= y1; y++ {
			for x := x0; x <= x1; x++ {
				if c.Img.RGBAAt(x, y) == ink {
					n++
				}
			}
		}
		return n
	}
	// overdrawn 數一塊裡有幾點與只拼圖層（沒有字）的畫面不同——肖像自己
	// 也有黃與淺紅，只能這樣分出「字壓上去」的點。
	overdrawn := func(c *Canvas, im *assets.Image, x0, y0, x1, y1 int) int {
		n := 0
		for y := y0; y <= y1; y++ {
			for x := x0; x <= x1; x++ {
				if c.Img.RGBAAt(x, y) != assets.EGAPalette[im.At(x, y)&15] {
					n++
				}
			}
		}
		return n
	}
	for _, loc := range []i18n.Locale{i18n.ZhHant, i18n.En, i18n.Ja} {
		i18n.Current = loc
		names := []string{"陳就", "周瑜"}
		if loc == i18n.En {
			names = []string{"Chen Jiu", "Zhou Yu"}
		}
		units := [2]UnitPanel{
			{Unit: mk(battle.MainAttacker, names[0], 1), Portrait: 228, Lord: names[0]},
			{Unit: mk(battle.MainDefender, names[1], 3), Portrait: 14, Lord: names[1]},
		}
		c := testCanvasPx(t, assets.ScreenW, assets.ScreenH)
		c.SetSmallFace(testSmallFace(t))
		info := ArtBattleInfo{Portrait: [2]int{-1, -1}, Units: &units}
		DrawArtBattle(c, ab, b, BattleView{}, info)
		layers := ab.compose(b, BattleView{}, info)
		for i := range units {
			ink := assets.EGAPalette[assets.FlagPlateColour[units[i].Unit.Side.OriginalIndex()]]
			fx, fy := l.Face(i)
			if n := overdrawn(c, layers, fx-8, fy-8, fx+71, fy+87); n > 0 {
				t.Errorf("%s 部隊面板 %d：字壓到肖像與框 %d 點", loc, i, n)
			}
			x0, y0, _, _ := l.Panel(i)
			if n := inkAt(c, x0, y0, x0+assets.BattlePanelW-1, y0+assets.BattlePanelH-1, ink); n == 0 {
				t.Errorf("%s 部隊面板 %d 沒有字", loc, i)
			}
		}
		// 第 0 槽空了的部隊：那一塊只剩藍底。
		gone := mk(battle.MainDefender, names[1], 1)
		gone.Leaders[0].Captured = true
		units[1] = UnitPanel{Unit: gone, Portrait: 14, Lord: names[1]}
		c = testCanvasPx(t, assets.ScreenW, assets.ScreenH)
		DrawArtBattle(c, ab, b, BattleView{}, ArtBattleInfo{Portrait: [2]int{-1, -1}, Units: &units})
		x0, y0, x1, y1 := l.Panel(1)
		if n := inkAt(c, x0, y0, x1, y1, assets.EGAPalette[assets.BattlePanelPaper]); n != assets.BattlePanelW*assets.BattlePanelH {
			t.Errorf("%s 第 0 槽空了的部隊面板該只剩藍底，藍的只有 %d／%d 點", loc, n, assets.BattlePanelW*assets.BattlePanelH)
		}

		// 查看。
		leader := &mk(battle.MainAttacker, names[0], 1).Leaders[0]
		c = testCanvasPx(t, assets.ScreenW, assets.ScreenH)
		info = ArtBattleInfo{Portrait: [2]int{-1, -1},
			Inspect: &InspectPanel{Leader: leader, Side: battle.MainAttacker, Portrait: 228}}
		DrawArtBattle(c, ab, b, BattleView{Page: []string{"x"}, PageTitle: "x"}, info)
		layers = ab.compose(b, BattleView{}, info)
		yellow := assets.EGAPalette[assets.BattleOrderInk]
		x0, y0, x1, y1 = l.Panel(2)
		if n := inkAt(c, x0, y0, x0+63, y1, yellow); n == 0 {
			t.Errorf("%s 查看：六行資料沒有字", loc)
		}
		if n := overdrawn(c, layers, x0+64, y0, inspectFaceX-9, y1); n == 0 {
			t.Errorf("%s 查看：名字那一欄沒有字", loc)
		}
		if n := overdrawn(c, layers, inspectFaceX-8, inspectFaceY-8, x1, y1); n > 0 {
			t.Errorf("%s 查看：字壓到肖像與框 %d 點", loc, n)
		}
	}
}

// TestBattleDayNumeralsFollowTheTable 釘住國字日數的三格（原版 `DS:0x78a8`
// 的位數表：個位在中格、十幾的「十」在中格、二十幾三格全用）。
func TestBattleDayNumeralsFollowTheTable(t *testing.T) {
	for _, k := range []struct {
		day  int
		want [3]string
	}{
		{0, [3]string{}},
		{1, [3]string{"", "一", ""}},
		{4, [3]string{"", "四", ""}},
		{9, [3]string{"", "九", ""}},
		{10, [3]string{"", "十", ""}},
		{11, [3]string{"", "十", "一"}},
		{19, [3]string{"", "十", "九"}},
		{20, [3]string{"", "二", "十"}},
		{21, [3]string{"二", "十", "一"}},
		{29, [3]string{"二", "十", "九"}},
		{30, [3]string{"", "三", "十"}},
		{31, [3]string{"", "三", "十"}},
	} {
		if got := battleDayNumerals(k.day); got != k.want {
			t.Errorf("第 %d 天：%q，該是 %q", k.day, got, k.want)
		}
	}
}

// TestBattleDayBoxStaysInTheColumn 釘住三個語系的第四框都畫在 x 8–39 裡。
func TestBattleDayBoxStaysInTheColumn(t *testing.T) {
	saved := i18n.Current
	defer func() { i18n.Current = saved }()
	for _, loc := range []i18n.Locale{i18n.ZhHant, i18n.En, i18n.Ja} {
		i18n.Current = loc
		for _, day := range []int{1, 24, 30} {
			c := testCanvasPx(t, assets.ScreenW, assets.ScreenH)
			drawBattleDayBox(c, day, battle.SkirmishFirstHour)
			ink, stray := 0, 0
			for y := 0; y < assets.ScreenH; y++ {
				for x := 0; x < assets.ScreenW; x++ {
					if c.Img.RGBAAt(x, y).A == 0 {
						continue
					}
					if x >= assets.BattleLeftBoxX0 && x <= assets.BattleLeftBoxX1 && y >= 228 && y <= 323 {
						ink++
					} else {
						stray++
					}
				}
			}
			if ink == 0 {
				t.Errorf("%s 第 %d 天：第四框裡沒有字", loc, day)
			}
			if stray > 0 {
				t.Errorf("%s 第 %d 天：有 %d 點畫到第四框外", loc, day, stray)
			}
		}
	}
}
