package state

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/wicanr2/softworld_san1_remake/internal/assets"
)

// loadData2 開原版的 DATA2 容器；沒素材就 skip。
//
// **本儲存庫不含原版檔案。** 缺素材要 skip 不要用自製代用品——
// 安靜的替代品會讓「還沒做完」看起來像做完了。
func loadData2(t *testing.T, ver string) *assets.Container {
	t.Helper()
	root := os.Getenv("SAN1_ORIG")
	if root == "" {
		t.Skip("沒設 SAN1_ORIG，跳過需要原版素材的測試")
	}
	base := filepath.Join(root, ver, "DATA2")
	rd := func(ext string) []byte {
		b, err := os.ReadFile(base + ext)
		if err != nil {
			t.Fatalf("讀 %s%s：%v", base, ext, err)
		}
		return b
	}
	c, err := assets.OpenContainer(rd(".NAM"), rd(".IDX"), rd(".GRP"))
	if err != nil {
		t.Fatalf("OpenContainer：%v", err)
	}
	return c
}

var versions = []string{"三國演義", "三國演義1加強版"}

// TestPrefectures 釘住 docs/formats/02：42 個郡、1-based、名稱是兩個漢字。
func TestPrefectures(t *testing.T) {
	for _, ver := range versions {
		t.Run(ver, func(t *testing.T) {
			c := loadData2(t, ver)
			s, err := LoadScenario(c, Scenario1)
			if err != nil {
				t.Fatalf("LoadScenario：%v", err)
			}
			ps := s.Prefectures()
			if len(ps) != 42 {
				t.Fatalf("郡數 ＝ %d，想要 42", len(ps))
			}
			// 頭尾兩個是 docs/formats/02 量到的，拿來當定位樁。
			if ps[0].Name != "遼東" || ps[0].ID != 1 {
				t.Errorf("第一個郡 ＝ %d/%q，想要 1/遼東", ps[0].ID, ps[0].Name)
			}
			if ps[41].Name != "鬱林" || ps[41].ID != 42 {
				t.Errorf("最後一個郡 ＝ %d/%q，想要 42/鬱林", ps[41].ID, ps[41].Name)
			}
			for _, p := range ps {
				if len([]rune(p.Name)) != 2 {
					t.Errorf("郡 %d 的名稱 %q 不是兩個字", p.ID, p.Name)
				}
			}
			// 1-based：0 與 43 要回錯誤，不能回啞元。
			if _, err := s.Prefecture(0); err == nil {
				t.Error("Prefecture(0) 應該回錯誤，不是回筆 0 的啞元")
			}
			if _, err := s.Prefecture(43); err == nil {
				t.Error("Prefecture(43) 應該回錯誤")
			}
			if p, err := s.Prefecture(1); err != nil || p.Name != "遼東" {
				t.Errorf("Prefecture(1) ＝ %q, %v", p.Name, err)
			}
		})
	}
}

// TestGenerals 釘住 docs/formats/03 §4：350 槽、346 個是人、字數分佈 326/20，
// 末尾 4 槽是填充（姓名欄是連續 Big5 碼位的標點）。
func TestGenerals(t *testing.T) {
	for _, ver := range versions {
		t.Run(ver, func(t *testing.T) {
			c := loadData2(t, ver)
			s, err := LoadScenario(c, Scenario1)
			if err != nil {
				t.Fatalf("LoadScenario：%v", err)
			}
			gs := s.Generals()
			if len(gs) != 350 {
				t.Fatalf("槽數 ＝ %d，想要 350（含空槽）", len(gs))
			}
			byLen := map[int]int{}
			named := 0
			for i, g := range gs {
				if g.Index != i {
					t.Fatalf("第 %d 筆的 Index ＝ %d——索引必須與原版槽號一致", i, g.Index)
				}
				if !g.IsPerson {
					continue
				}
				named++
				byLen[len([]rune(g.Name))]++
			}
			if named != 346 {
				t.Errorf("是人物的槽 ＝ %d，想要 346", named)
			}
			if got := len(s.People()); got != 346 {
				t.Errorf("People() ＝ %d 筆，想要 346", got)
			}
			// 6 byte 裝 2–3 個漢字，這個分佈是格式對不對的直接證據。
			if byLen[2] != 326 || byLen[3] != 20 {
				t.Errorf("字數分佈 ＝ %v，想要 2 字 326、3 字 20", byLen)
			}
			// 末尾 4 槽是填充：有文字但不是人。
			// 它們的姓名欄是連續 Big5 碼位 A141–A14C，不是資料。
			for i := 346; i < 350; i++ {
				if gs[i].IsPerson {
					t.Errorf("第 %d 槽是填充槽，不該判成人物（%q）", i, gs[i].Name)
				}
				if gs[i].Name == "" {
					t.Errorf("第 %d 槽應該有標點文字，Name 卻是空的", i)
				}
			}
		})
	}
}

// TestAllSlots 六個劇本都要讀得起來，而且名冊相同（docs/formats/03 §4）。
func TestAllSlots(t *testing.T) {
	c := loadData2(t, "三國演義")
	var first []string
	for _, slot := range []Slot{Scenario1, Scenario2, Scenario3, Scenario4, Scenario5, Scenario6} {
		s, err := LoadScenario(c, slot)
		if err != nil {
			t.Fatalf("劇本 %s：%v", slot, err)
		}
		if len(s.Masters()) != 16 {
			t.Errorf("劇本 %s 的諸侯數 ＝ %d，想要 16", slot, len(s.Masters()))
		}
		names := make([]string, 0, 350)
		for _, g := range s.Generals() {
			names = append(names, g.Name)
		}
		if first == nil {
			first = names
			continue
		}
		for i := range names {
			if names[i] != first[i] {
				t.Fatalf("劇本 %s 第 %d 槽的姓名與劇本 001 不同（%q vs %q）——"+
					"名冊應該是共用的", slot, i, names[i], first[i])
			}
		}
	}
}

func TestMissingSlot(t *testing.T) {
	c := loadData2(t, "三國演義")
	if _, err := LoadScenario(c, Slot("999")); err == nil {
		t.Fatal("不存在的槽應該回錯誤")
	}
}
