package assets

import (
	"os"
	"path/filepath"
	"testing"
)

// container 開一組原版容器；沒有素材就 skip。**本儲存庫不含原版檔案。**
func container(t *testing.T, name string) *Container {
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
	c, err := OpenContainer(read("NAM"), read("IDX"), read("GRP"))
	if err != nil {
		t.Fatal(err)
	}
	return c
}

// TestDecodeEveryImage 釘住容器裡每一個 `.IMG`／`.FAC` 都解得開。
//
// 判準是**尺寸與長度必須吻合**：(寬+7)/8 × 高 × 4 個平面。
// 這個等式對不上的東西不硬解——一張硬解出來的圖看起來就像雜訊，
// 而雜訊與「這張圖本來就很花」分不出來。
func TestDecodeEveryImage(t *testing.T) {
	total, bad := 0, 0
	for _, name := range []string{"DATA1", "DATA2", "DATA3"} {
		c := container(t, name)
		for i := 0; i < c.Len(); i++ {
			n := c.Entry(i).Name
			if !hasSuffix(n, ".IMG") && !hasSuffix(n, ".FAC") {
				continue
			}
			total++
			im, err := DecodeImage(c.Data(i))
			if err != nil {
				bad++
				t.Errorf("%s／%s：%v", name, n, err)
				continue
			}
			if len(im.Pix) != im.W*im.H {
				t.Errorf("%s／%s：%d×%d 卻有 %d 個像素", name, n, im.W, im.H, len(im.Pix))
			}
		}
	}
	if total == 0 {
		t.Fatal("一張圖都沒找到")
	}
	t.Logf("%d 張圖，%d 張解不開", total, bad)
}

// TestPortraitsAreSameSize 釘住肖像全部一樣大。
//
// 306 張肖像（`DATA1` 50 張、`DATA3` 256 張）都是 2,564 個位元組，
// 解出來要是同一個尺寸——不一樣就表示表頭讀錯了。
func TestPortraitsAreSameSize(t *testing.T) {
	var w, h int
	n := 0
	for _, name := range []string{"DATA1", "DATA3"} {
		c := container(t, name)
		for i := 0; i < c.Len(); i++ {
			if !hasSuffix(c.Entry(i).Name, ".FAC") {
				continue
			}
			im, err := DecodeImage(c.Data(i))
			if err != nil {
				t.Fatalf("%s：%v", c.Entry(i).Name, err)
			}
			if n == 0 {
				w, h = im.W, im.H
			} else if im.W != w || im.H != h {
				t.Fatalf("%s 是 %d×%d，前面的是 %d×%d",
					c.Entry(i).Name, im.W, im.H, w, h)
			}
			n++
		}
	}
	if n == 0 {
		t.Skip("沒有肖像")
	}
	if w != 64 || h != 80 {
		t.Errorf("肖像是 %d×%d，量到的是 64×80", w, h)
	}
	t.Logf("%d 張肖像，全部 %d×%d", n, w, h)
}

// TestImagesAreNotNoise 釘住解出來的圖有結構，不是雜訊。
//
// 判準是「隔一格同色」的比例。⚠ **不要用相鄰同色**：這個年代的 EGA 圖
// 大量使用抖動（棋盤式交錯兩色換取更多視覺色階），相鄰同色反而少，
// 隔一格才是該相同的那一對。十六色的雜訊約 6%，實際的圖遠高於此。
func TestImagesAreNotNoise(t *testing.T) {
	c := container(t, "DATA3")
	checked := 0
	for i := 0; i < c.Len() && checked < 20; i++ {
		if !hasSuffix(c.Entry(i).Name, ".FAC") {
			continue
		}
		im, err := DecodeImage(c.Data(i))
		if err != nil {
			t.Fatal(err)
		}
		same, pairs := 0, 0
		for y := 0; y < im.H; y++ {
			for x := 0; x < im.W; x++ {
				v := im.Pix[y*im.W+x]
				if x+2 < im.W {
					pairs++
					if im.Pix[y*im.W+x+2] == v {
						same++
					}
				}
				if y+2 < im.H {
					pairs++
					if im.Pix[(y+2)*im.W+x] == v {
						same++
					}
				}
			}
		}
		if pct := same * 100 / pairs; pct < 30 {
			t.Errorf("%s 只有 %d%% 隔一格同色，看起來像雜訊", c.Entry(i).Name, pct)
		}
		checked++
	}
	if checked == 0 {
		t.Skip("沒有肖像可以檢查")
	}
}

// TestRejectsGarbage 釘住尺寸對不上的東西不硬解。
func TestRejectsGarbage(t *testing.T) {
	for _, b := range [][]byte{
		nil, {1, 2}, {0, 0, 0, 0},
		append([]byte{80, 0, 64, 0}, make([]byte, 100)...), // 長度不對
	} {
		if _, err := DecodeImage(b); err == nil {
			t.Errorf("% X 竟然解得出圖", b[:min(len(b), 8)])
		}
		if IsImage(b) {
			t.Error("IsImage 說這是圖")
		}
	}
}

func hasSuffix(s, suf string) bool {
	return len(s) >= len(suf) && s[len(s)-len(suf):] == suf
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
