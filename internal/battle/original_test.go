package battle

import "testing"

// TestLoadOriginalField 釘住原版戰場地圖的解讀（`L0`、`[base]`）。
//
// 這裡用手排的 120 個位元組，把四件事各驗一次：地形碼的對照、
// 圖外與大山分得開、五個標記格（城池 ＋ 四個軍團起點）、
// 以及出口對回鄰郡編號。
func TestLoadOriginalField(t *testing.T) {
	data := make([]byte, FieldBytes)
	for i := range data {
		data[i] = 0xFF // 先全部圖外
	}
	put := func(x, y int, mark, terrain byte) {
		data[y*FieldW+x] = mark<<4 | terrain
	}
	put(0, 0, 15, 7)  // 平原，沒有標記
	put(1, 0, 15, 1)  // 大山
	put(2, 0, 10, 5)  // 城池
	put(3, 0, 11, 2)  // 第一軍的起點，山丘
	put(4, 0, 12, 8)  // 第二軍，樹林
	put(5, 0, 13, 3)  // 第三軍，淺水
	put(6, 0, 14, 4)  // 第四軍，深水
	put(7, 0, 0, 9)   // 出口 → 鄰郡 33，沙漠
	put(8, 0, 1, 6)   // 出口 → 鄰郡 7，關寨
	put(9, 0, 15, 7)  // 平原

	f, err := Load(data, []int{33, 7})
	if err != nil {
		t.Fatal(err)
	}
	for _, c := range []struct {
		x, y int
		want Terrain
	}{
		{0, 0, Plain}, {1, 0, Mountain}, {2, 0, City}, {3, 0, Hill},
		{4, 0, Forest}, {5, 0, Shallow}, {6, 0, Deep}, {7, 0, Desert},
		{8, 0, Fort},
	} {
		if got := f.At(FromOffset(c.x, c.y)); got != c.want {
			t.Errorf("(%d,%d) 的地形是 %s，應該是 %s", c.x, c.y, got, c.want)
		}
	}
	// 圖外與大山走起來一樣，但分得開。
	if !f.Outside(FromOffset(0, 1)) {
		t.Error("0xFF 的格子應該算圖外")
	}
	if f.Outside(FromOffset(1, 0)) {
		t.Error("大山不是圖外")
	}
	if f.At(FromOffset(0, 1)) != Mountain {
		t.Error("圖外走起來要跟大山一樣")
	}

	if f.CityAt != FromOffset(2, 0) {
		t.Errorf("城池在 %v，應該是 %v", f.CityAt, FromOffset(2, 0))
	}
	for i, want := range []Hex{
		FromOffset(3, 0), FromOffset(4, 0), FromOffset(5, 0), FromOffset(6, 0),
	} {
		if f.Starts[i] != want {
			t.Errorf("第 %d 軍的起點是 %v，應該是 %v", i+1, f.Starts[i], want)
		}
	}
	if len(f.Gates) != 2 {
		t.Fatalf("出口有 %d 個，應該是 2", len(f.Gates))
	}
	if f.Gate(33) != FromOffset(7, 0) || f.Gate(99) != NoHex {
		t.Errorf("出口對錯郡了：%v", f.Gates)
	}
	if f.Gate(7) != FromOffset(8, 0) {
		t.Errorf("出口對錯郡了：%v", f.Gates)
	}

	// 寫回去要逐位元組相同——**這是解讀對不對的判準**，
	// 少讀一個欄位或把圖外當成大山，round-trip 立刻紅。
	back := f.Bytes()
	for i := range data {
		if back[i] != data[i] {
			t.Fatalf("第 %d 格寫回去是 %#02x，原本是 %#02x", i, back[i], data[i])
		}
	}

	// 出口編號超出鄰郡清單時當成沒有出口。原版的資料裡沒有這種格
	// （42 個郡都驗過），但存檔壞掉時不該讓它指到不存在的郡。
	bad := append([]byte(nil), data...)
	bad[9] = 2<<4 | 7
	f2, err := Load(bad, []int{33, 7})
	if err != nil {
		t.Fatal(err)
	}
	if len(f2.Gates) != 2 {
		t.Errorf("出口有 %d 個，超出清單的那個不該算進去", len(f2.Gates))
	}
}

// TestOriginalLayoutMatchesTheDrawing 釘住版面：12 欄 × 10 列、
// **奇數欄往下移半格**（原版 `0x22742`–`0x2276c` 的畫格座標），
// 六個方向換算過來與 dirDelta 逐格相同（`0x24c11` 的 dx／dy 表）。
func TestOriginalLayoutMatchesTheDrawing(t *testing.T) {
	// 原版的 dx／dy 表，索引是 `(欄的奇偶 × 6 + 方向) × 2`。
	dx := [2][6]int{{-1, 0, 1, -1, 0, 1}, {-1, 0, 1, -1, 0, 1}}
	dy := [2][6]int{{0, 1, 0, -1, -1, -1}, {1, 1, 1, 0, -1, 0}}
	dirs := Dirs() // 1 左下、2 下、3 右下、4 左上、5 上、6 右上
	for x := 0; x < FieldW; x++ {
		for y := 0; y < FieldH; y++ {
			h := FromOffset(x, y)
			for i, d := range dirs {
				gx, gy := x+dx[x&1][i], y+dy[x&1][i]
				n := h.Step(d)
				nx, ny := n.Q, n.R+(n.Q-(n.Q&1))/2
				if nx != gx || ny != gy {
					t.Fatalf("(%d,%d) 往方向 %d 走到 (%d,%d)，原版是 (%d,%d)",
						x, y, d, nx, ny, gx, gy)
				}
			}
		}
	}
}
