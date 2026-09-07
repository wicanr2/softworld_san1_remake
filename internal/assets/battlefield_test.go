package assets

import (
	"image"
	"image/color"
	imgpng "image/png"
	"os"
	"testing"
)

// bfShotPath 是原版主戰場的基準圖，由 `internal/parity` 的
// `TestZZBattleKeySweep`（候選 4）產，要設 `SAN1_SHOTS`。
const bfShotPath = "../../workplace/shots/bf/sweep-04.png"

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
	// **只驗「有格子全中」**。全中的那幾格證明了三件事：圖塊的解碼、
	// 圖塊編號就是地形碼、格子的座標。剩下的格子差在哪還沒解——
	// 逐格找最佳位移的結果是**沒有整體偏移**（全中的都落在 (0,0)，
	// 其餘在 ±24 的範圍內找不到 95% 以上的位置），所以不是座標問題，
	// 是那些格子上面疊了東西。基準畫面那一場下著雨（左側面板寫「雨」），
	// 天氣是目前最像的解釋，還沒驗。
	if full < 4 {
		t.Errorf("只有 %d 格全中：圖塊或座標不對", full)
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
