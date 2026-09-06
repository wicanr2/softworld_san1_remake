package assets

import (
	"encoding/binary"
	"os"
	"path/filepath"
	"testing"
)

// build 造一組合法的容器 bytes，供不需要原版素材的測試使用。
func build(names []string, sizes []uint32) (nam, idx, grp []byte) {
	var end uint32
	for i, n := range names {
		var raw [12]byte
		for j := range raw {
			raw[j] = ' '
		}
		base, ext := n, ""
		if k := len(n) - 4; k > 0 && n[k] == '.' {
			base, ext = n[:k], n[k+1:]
		}
		copy(raw[0:8], base)
		raw[8] = '.'
		copy(raw[9:12], ext)
		nam = append(nam, raw[:]...)
		nam = append(nam, 0, 0, 0, 0)

		end += sizes[i]
		var b [4]byte
		binary.LittleEndian.PutUint32(b[:], end)
		idx = append(idx, b[:]...)
	}
	grp = make([]byte, end)
	for i := range grp {
		grp[i] = byte(i)
	}
	return
}

func TestOpenContainerRoundTrip(t *testing.T) {
	names := []string{"ZHONG.COD", "YING.PAT", "EGAFILL.PAL"}
	sizes := []uint32{556, 2048, 1024}
	c, err := OpenContainer(build(names, sizes))
	if err != nil {
		t.Fatalf("OpenContainer: %v", err)
	}
	if c.Len() != 3 {
		t.Fatalf("Len ＝ %d，想要 3", c.Len())
	}
	var want uint32
	for i, n := range names {
		e := c.Entry(i)
		if e.Name != n {
			t.Errorf("第 %d 項名稱 ＝ %q，想要 %q", i, e.Name, n)
		}
		if e.Start != want {
			t.Errorf("第 %d 項 Start ＝ %d，想要 %d", i, e.Start, want)
		}
		if e.Size() != sizes[i] {
			t.Errorf("第 %d 項 Size ＝ %d，想要 %d", i, e.Size(), sizes[i])
		}
		if got := uint32(len(c.Data(i))); got != sizes[i] {
			t.Errorf("第 %d 項 Data 長度 ＝ %d，想要 %d", i, got, sizes[i])
		}
		want = e.End
	}
	if i, ok := c.ByName("YING.PAT"); !ok || i != 1 {
		t.Errorf("ByName(YING.PAT) ＝ (%d, %v)，想要 (1, true)", i, ok)
	}
	// 大小寫敏感：查不到比安靜拿到別的東西好。
	if _, ok := c.ByName("ying.pat"); ok {
		t.Error("ByName 應該大小寫敏感")
	}
}

// TestOpenContainerRejects 釘住 spec 001 §3 的四項驗證條件。
//
// **每一項都是「不報錯就會安靜地錯下去」的形狀**，所以這裡要求的是
// 回錯誤，不是盡力而為。
func TestOpenContainerRejects(t *testing.T) {
	good := func() (nam, idx, grp []byte) {
		return build([]string{"A.PAT", "B.PAT"}, []uint32{16, 32})
	}

	t.Run("GRP 是 MZ 執行檔", func(t *testing.T) {
		nam, idx, grp := good()
		grp[0], grp[1] = 'M', 'Z'
		if _, err := OpenContainer(nam, idx, grp); err == nil {
			t.Fatal("想要錯誤：DATA0／4／5 的 .GRP 是執行檔，硬切會切出垃圾")
		}
	})

	t.Run("項數對不上", func(t *testing.T) {
		nam, idx, grp := good()
		if _, err := OpenContainer(nam[:namEntrySize], idx, grp); err == nil {
			t.Fatal("想要錯誤：DATA0 就是靠項數不符露餡的（43 vs 604）")
		}
	})

	t.Run("IDX 逆序", func(t *testing.T) {
		nam, idx, grp := good()
		binary.LittleEndian.PutUint32(idx[0:], 999) // 第 0 項比第 1 項大
		if _, err := OpenContainer(nam, idx, grp); err == nil {
			t.Fatal("想要錯誤：逆序代表它不是位移表")
		}
	})

	t.Run("末值與 GRP 長度不符", func(t *testing.T) {
		nam, idx, grp := good()
		if _, err := OpenContainer(nam, idx, grp[:len(grp)-1]); err == nil {
			t.Fatal("想要錯誤：末值 ≠ 檔長代表這三個檔不是一組")
		}
	})
}

func TestNormalizeName(t *testing.T) {
	mk := func(s string) [12]byte {
		var r [12]byte
		copy(r[:], s)
		return r
	}
	for _, tc := range []struct{ in, want string }{
		{"ZHONG   .COD", "ZHONG.COD"},
		{"HERCFILL.PAL", "HERCFILL.PAL"},
		{"F000    .FAC", "F000.FAC"},
		{"SANTBM1 .IMG", "SANTBM1.IMG"},
		{"NOEXT   .   ", "NOEXT"},
	} {
		if got := normalizeName(mk(tc.in)); got != tc.want {
			t.Errorf("normalizeName(%q) ＝ %q，想要 %q", tc.in, got, tc.want)
		}
	}
}

// origDir 回傳原版素材目錄；沒設就 skip。
//
// **本儲存庫不含原版檔案。** 缺素材要 skip 不要用自製代用品——
// 安靜的替代品會讓「還沒做完」看起來像做完了。
func origDir(t *testing.T) string {
	t.Helper()
	d := os.Getenv("SAN1_ORIG")
	if d == "" {
		t.Skip("沒設 SAN1_ORIG，跳過需要原版素材的測試")
	}
	return d
}

// TestRealContainers 對原版的六個槽跑一次，釘住 docs/formats/01 量到的事實。
func TestRealContainers(t *testing.T) {
	root := origDir(t)
	for _, ver := range []string{"三國演義", "三國演義1加強版"} {
		for _, tc := range []struct {
			n       int
			wantLen int // 期望項數；0 表示預期開不起來
			first   string
		}{
			{0, 0, ""}, // .GRP 是 MZ，且項數 43 vs 604
			{1, 217, "VZHONG.COD"},
			{2, 506, "SCG30.IMG"},
			{3, 422, "F000.FAC"},
			{4, 0, ""}, // .GRP 是 MZ（帶 LZ91 簽章）
			{5, 0, ""}, // .GRP 是 MZ
		} {
			name := ver + "/DATA" + string(rune('0'+tc.n))
			t.Run(name, func(t *testing.T) {
				base := filepath.Join(root, ver, "DATA"+string(rune('0'+tc.n)))
				rd := func(ext string) []byte {
					b, err := os.ReadFile(base + ext)
					if err != nil {
						t.Fatalf("讀 %s%s：%v", base, ext, err)
					}
					return b
				}
				c, err := OpenContainer(rd(".NAM"), rd(".IDX"), rd(".GRP"))
				if tc.wantLen == 0 {
					if err == nil {
						t.Fatalf("想要錯誤（這個槽不是容器），卻開起來了：%d 項", c.Len())
					}
					return
				}
				if err != nil {
					t.Fatalf("OpenContainer：%v", err)
				}
				if c.Len() != tc.wantLen {
					t.Errorf("項數 ＝ %d，想要 %d", c.Len(), tc.wantLen)
				}
				if got := c.Entry(0).Name; got != tc.first {
					t.Errorf("第 0 項 ＝ %q，想要 %q", got, tc.first)
				}
				// 無縫覆蓋：每一項的起點就是前一項的終點，末項終點 ＝ 檔長。
				var pos uint32
				for i := 0; i < c.Len(); i++ {
					if e := c.Entry(i); e.Start != pos {
						t.Fatalf("第 %d 項有斷層：Start ＝ %d，前一項終點 ＝ %d", i, e.Start, pos)
					} else {
						pos = e.End
					}
				}
			})
		}
	}
}
