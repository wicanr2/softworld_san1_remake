package assets

import (
	"image"
	"image/color"
	imgpng "image/png"
	"os"
	"testing"
)

// bfShotPath 是**紮完寨之後**的原版主戰場，由 `internal/parity` 的
// `TestZZBattleKeySweep` 產：`SAN1_BATTLEKEY` 在候選 4 後面接八個 `0`
// （五支部隊各紮一次），`SAN1_SHOTS` 指到輸出目錄。
const bfShotPath = "../../workplace/shots/bf/orig-battle.png"

// TestBattleTilesDecode 釘住 `EICON.GRP` 的版面：36 筆 × 772 B，每張 48×32。
func TestBattleTilesDecode(t *testing.T) {
	tiles, err := BattleTiles(container(t, "DATA1"))
	if err != nil {
		t.Fatal(err)
	}
	if len(tiles) != TileCount {
		t.Fatalf("解出 %d 張", len(tiles))
	}
	// 圖塊不能是空白的：地形碼 1–10 那幾張各要有兩種以上的顏色。
	for k := 1; k <= MaxTerrain; k++ {
		seen := map[byte]bool{}
		for _, v := range tiles[k].Pix {
			seen[v] = true
		}
		if len(seen) < 2 {
			t.Errorf("圖塊 %d 只有 %d 種顏色", k, len(seen))
		}
	}
}

// TestBattleFieldMatchesTheOriginal 把拼出來的戰場與原版的畫面逐格比。
//
// **逐格比不比整張**：部隊、游標、右下的提示都疊在上面，整張比會被
// 那些拉低。一格一格比才看得出「圖塊對不對、位置對不對」——
// 沒有部隊站的格子要 100%。
func TestBattleFieldMatchesTheOriginal(t *testing.T) {
	f, err := os.Open(bfShotPath)
	if err != nil {
		t.Skipf("沒有主戰場基準畫面 %s", bfShotPath)
	}
	shot, err := imgpng.Decode(f)
	f.Close()
	if err != nil {
		t.Fatal(err)
	}
	tiles, err := BattleTiles(container(t, "DATA1"))
	if err != nil {
		t.Fatal(err)
	}
	// 基準畫面打的是廬陵（郡 25，`TestZZBattleKeySweep` 的目標）。
	field := prefectureField(t, 25)
	im := BattleField(tiles, field, 0)
	rgba := im.RGBA()

	full, part, skipped := 0, 0, 0
	for k, b := range field {
		if b == 0xFF || int(b&0x0F) > MaxTerrain {
			skipped++
			continue
		}
		x, y := FieldCell(k%FieldCols, k/FieldCols)
		if x+TileW > 640 || y+TileH > 350 {
			skipped++
			continue
		}
		same := 0
		for dy := 0; dy < TileH; dy++ {
			for dx := 0; dx < TileW; dx++ {
				a := color.RGBAModel.Convert(rgba.At(x+dx, y+dy)).(color.RGBA)
				c := color.RGBAModel.Convert(shot.At(x+dx, y+dy)).(color.RGBA)
				if a == c {
					same++
				}
			}
		}
		if same == TileW*TileH {
			full++
		} else {
			part++
		}
	}
	t.Logf("%d 格逐像素全中、%d 格不同、%d 格跳過", full, part, skipped)
	// 基準畫面是**紮完寨之後**的主戰場，所以除了五支部隊站的格子之外
	// 都應該逐像素全中。
	//
	// ⚠ 換基準畫面之前這個數字是 4／78，而原因不是座標錯：那一張停在
	// 紮寨那一步，原版把可以下寨的格子照常畫、其餘蓋一層 50% 網點
	// （偶數列偶數欄留著、其餘變黑）。**判準要看形狀不要看比例**——
	// 把差異畫成圖案一眼看得出是網點，只看相符率會猜成天氣。
	if full < 70 {
		t.Errorf("只有 %d 格全中（%d 格不同）：圖塊或座標不對", full, part)
	}
	if part > 8 {
		t.Errorf("%d 格不同，比五支部隊佔的格子多", part)
	}
}

// prefectureField 讀某個郡的戰場地圖。
func prefectureField(t *testing.T, pref int) []byte {
	t.Helper()
	root := os.Getenv("SAN1_ORIG")
	rd := func(ext string) []byte {
		b, err := os.ReadFile(root + "/三國演義/DATA2." + ext)
		if err != nil {
			t.Skip(err)
		}
		return b
	}
	c, err := OpenContainer(rd("NAM"), rd("IDX"), rd("GRP"))
	if err != nil {
		t.Fatal(err)
	}
	i, ok := c.ByName("BASESTA.001")
	if !ok {
		t.Fatal("沒有 BASESTA.001")
	}
	rec := c.Data(i)[pref*176 : (pref+1)*176]
	return append([]byte(nil), rec[55:175]...)
}

var _ = image.Rect

// TestUnitFlagsSitOnCells 釘住部隊旗幟的位置：格子左上角加 (8, 0)。
//
// 四面旗在基準畫面上各有一處逐像素 100% 相符，換算回格子都落在
// 同一個位移上——**四面一致才算數**，一面可能是巧合。
func TestUnitFlagsSitOnCells(t *testing.T) {
	f, err := os.Open(bfShotPath)
	if err != nil {
		t.Skipf("沒有主戰場基準畫面 %s", bfShotPath)
	}
	shot, err := imgpng.Decode(f)
	f.Close()
	if err != nil {
		t.Fatal(err)
	}
	c := container(t, "DATA1")
	for _, tc := range []struct {
		name     string
		col, row int
	}{
		{"WFLAGD00.IMG", 5, 2},
		{"WFLAGD01.IMG", 5, 3},
		{"WFLAGD02.IMG", 6, 3},
		{"WFLAGD03.IMG", 6, 1},
	} {
		flag, err := UnitFlag(c, tc.name)
		if err != nil {
			t.Fatal(err)
		}
		x, y := FlagCell(tc.col, tc.row)
		same := 0
		for fy := 0; fy < FlagH; fy++ {
			for fx := 0; fx < FlagW; fx++ {
				a := EGAPalette[flag.Pix[fy*FlagW+fx]]
				b := color.RGBAModel.Convert(shot.At(x+fx, y+fy)).(color.RGBA)
				if a == b {
					same++
				}
			}
		}
		if same != FlagW*FlagH {
			t.Errorf("%s 在格 (%d,%d) → (%d,%d) 只有 %d／%d 個像素相符",
				tc.name, tc.col, tc.row, x, y, same, FlagW*FlagH)
		}
	}
}

// TestFlagNames 釘住旗幟的命名：組照軍力（守 D、攻 A），數字照原版的
// 隊伍編號（中軍、先鋒、左軍、右軍、後軍，第六張是城門圖示）。
func TestFlagNames(t *testing.T) {
	for _, tc := range []struct {
		army, unit int
		want       string
	}{
		{0, 0, "WFLAGD00.IMG"}, // 主守軍中軍——基準畫面上驗過
		{0, 3, "WFLAGD03.IMG"}, // 主守軍右軍
		{1, 0, "WFLAGD10.IMG"}, // 助守軍
		{2, 0, "WFLAGA00.IMG"}, // 主攻軍——基準畫面上驗過
		{3, 5, "WFLAGA15.IMG"}, // 助攻軍的城門圖示
	} {
		if got := FlagName(tc.army, tc.unit); got != tc.want {
			t.Errorf("FlagName(%d,%d) ＝ %q，想要 %q", tc.army, tc.unit, got, tc.want)
		}
	}
	if FlagName(4, 0) != "" || FlagName(0, 6) != "" {
		t.Error("越界沒有回空字串")
	}
	flags, err := UnitFlags(container(t, "DATA1"))
	if err != nil {
		t.Fatal(err)
	}
	for army := range flags {
		for unit, im := range flags[army] {
			w := FlagW
			if unit == FlagGate {
				w = GateW
			}
			if im == nil || im.W != w || im.H != FlagH {
				t.Errorf("軍力 %d 第 %d 張不是 %d×%d", army, unit, w, FlagH)
			}
		}
	}
}

// TestFlagPlateText 釘住兵力牌的格式：`%5d`，靠右。
func TestFlagPlateText(t *testing.T) {
	for _, tc := range []struct {
		n    int
		want string
	}{{2673, " 2673"}, {1500, " 1500"}, {30000, "30000"}, {7, "    7"}} {
		if got := FlagPlateText(tc.n); got != tc.want {
			t.Errorf("FlagPlateText(%d) ＝ %q，想要 %q", tc.n, got, tc.want)
		}
	}
}

// TestFlagComplement 釘住反白：守方那面旗在基準畫面上是
// `WFLAGA00` 每格取補數，360 格逐格相同。
func TestFlagComplement(t *testing.T) {
	f, err := os.Open(bfShotPath)
	if err != nil {
		t.Skipf("沒有主戰場基準畫面 %s", bfShotPath)
	}
	shot, err := imgpng.Decode(f)
	f.Close()
	if err != nil {
		t.Fatal(err)
	}
	flag, err := UnitFlag(container(t, "DATA1"), "WFLAGA00.IMG")
	if err != nil {
		t.Fatal(err)
	}
	inv := flag.Complement()
	const bx, by = 352, 228
	bad := 0
	for y := 0; y < inv.H; y++ {
		for x := 0; x < inv.W; x++ {
			a := EGAPalette[inv.Pix[y*inv.W+x]&15]
			b := color.RGBAModel.Convert(shot.At(bx+x, by+y)).(color.RGBA)
			if a != b {
				bad++
			}
		}
	}
	if bad != 0 {
		t.Errorf("陳就的中軍在 (%d,%d) 有 %d 格對不上取補數後的 WFLAGA00", bx, by, bad)
	}
}
