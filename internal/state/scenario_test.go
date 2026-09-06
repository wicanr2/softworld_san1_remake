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

// TestScenario1Factions 釘住 docs/spec/003 §2 那四個欄位。
//
// 十四行領地分佈是三件事同時成立才有的：`BASEMAS` off2 指到對的人、
// `BASESTA` off30 指到對的勢力、而且兩者與原版畫面（「劉備 主公，請對(8)」，
// 8 ＝ 齊郡）一致。**任何一個欄位偏移錯掉，這張表就會變成另一個
// 自洽但錯的樣子**——所以整張比，不是抽一格比。
func TestScenario1Factions(t *testing.T) {
	c := loadData2(t, "三國演義")
	sc, err := LoadScenario(c, Scenario1)
	if err != nil {
		t.Fatal(err)
	}

	want := map[string][]string{
		"劉備":  {"齊郡"},
		"曹操":  {"陳留"},
		"孫堅":  {"長沙"},
		"袁紹":  {"渤海", "鄴郡"},
		"袁術":  {"潁川", "南陽"},
		"董卓":  {"上黨", "弘農", "洛陽", "京兆"},
		"劉焉":  {"成都", "巴郡", "永昌"},
		"馬騰":  {"武威"},
		"劉表":  {"襄陽", "南郡", "宜都"},
		"陶謙":  {"琅邪", "下邳"},
		"公孫瓚": {"涿郡"},
		"劉繇":  {"建業"},
		"王朗":  {"吳郡"},
		"孔融":  {"北海"},
	}

	active := sc.ActiveFactions()
	if len(active) != len(want) {
		t.Fatalf("實際在用的勢力有 %d 個，應該是 %d 個", len(active), len(want))
	}
	for _, f := range active {
		lord, err := sc.Lord(f)
		if err != nil {
			t.Fatalf("勢力 %d 的君主取不到：%v", f, err)
		}
		exp, ok := want[lord.Name]
		if !ok {
			t.Errorf("勢力 %d 的君主是 %q，不在預期名單裡", f, lord.Name)
			continue
		}
		var got []string
		for _, p := range sc.Territory(f) {
			got = append(got, p.Name)
		}
		if len(got) != len(exp) {
			t.Errorf("%s 有 %v，應該是 %v", lord.Name, got, exp)
			continue
		}
		for i := range got {
			if got[i] != exp[i] {
				t.Errorf("%s 有 %v，應該是 %v", lord.Name, got, exp)
				break
			}
		}
	}

	// 無主的郡：42 − 24 ＝ 18。
	free := 0
	for _, p := range sc.Prefectures() {
		if !p.Owned() {
			free++
		}
	}
	if free != 18 {
		t.Errorf("無主的郡有 %d 個，應該是 18 個", free)
	}
}

// TestRetinueLivesInOwnTerritory 釘住兩張表的一致性。
//
// 這一條**不看任何預期值**，只問「人物的所在郡歸不歸自己的勢力」。
// off18／off19／off30 三個偏移只要有一個錯，一致率就會塌掉——
// 而預期值型的測試看不出這件事，因為它只檢查自己列出來的那幾筆。
func TestRetinueLivesInOwnTerritory(t *testing.T) {
	for _, ver := range versions {
		c := loadData2(t, ver)
		for _, slot := range []Slot{Scenario1, Scenario2, Scenario3, Scenario4, Scenario5, Scenario6} {
			sc, err := LoadScenario(c, slot)
			if err != nil {
				t.Fatalf("%s %s：%v", ver, slot, err)
			}
			owner := map[int]uint8{}
			for _, p := range sc.Prefectures() {
				owner[p.ID] = p.Owner
			}
			employed, bad := 0, 0
			for _, g := range sc.People() {
				if !g.Employed() {
					continue
				}
				employed++
				if owner[int(g.Location)] != g.Faction {
					bad++
					if bad <= 3 {
						t.Errorf("%s %s：%s 效力於勢力 %d，卻在郡 %d（那個郡屬於 %d）",
							ver, slot, g.Name, g.Faction, g.Location, owner[int(g.Location)])
					}
				}
			}
			if employed == 0 {
				t.Errorf("%s %s：一位有勢力的人物都沒有——欄位大概讀錯了", ver, slot)
			}
			if bad > 0 {
				t.Errorf("%s %s：%d/%d 位人物不在自己勢力的郡裡", ver, slot, bad, employed)
			}
		}
	}
}

// TestPrefectureGeneralCounts 用兩張表互相印證。
//
// 郡記著「現役武將數」與「在野武將數」，而那兩個數字**在人物表裡數得出來**。
// 三個欄位（人物的勢力、所在郡、身分）與兩個欄位（郡的兩個計數）
// 只要有一個偏移錯掉，數字就對不上——而預期值型的測試看不出這件事。
//
// 在野那一欄要加「身分 ＝ StatusAvailable」才數得對：不加條件時
// 42 個郡只對得上 26 個（`docs/spec/003` §5）。
func TestPrefectureGeneralCounts(t *testing.T) {
	for _, ver := range versions {
		c := loadData2(t, ver)
		for _, slot := range []Slot{Scenario1, Scenario2, Scenario3, Scenario4, Scenario5, Scenario6} {
			sc, err := LoadScenario(c, slot)
			if err != nil {
				t.Fatalf("%s %s：%v", ver, slot, err)
			}
			active := map[uint8]int{}
			free := map[uint8]int{}
			for _, g := range sc.People() {
				if g.Location < 1 || g.Location > PrefectureCount {
					continue
				}
				switch {
				case g.Employed():
					active[g.Location]++
				case g.Status == StatusAvailable:
					free[g.Location]++
				}
			}
			for _, p := range sc.Prefectures() {
				id := uint8(p.ID)
				if int(p.ActiveGenerals) != active[id] {
					t.Errorf("%s %s 郡 %d %s：現役武將表記 %d，數人物得到 %d",
						ver, slot, p.ID, p.Name, p.ActiveGenerals, active[id])
				}
				if int(p.FreeGenerals) != free[id] {
					t.Errorf("%s %s 郡 %d %s：在野武將表記 %d，數人物得到 %d",
						ver, slot, p.ID, p.Name, p.FreeGenerals, free[id])
				}
			}
		}
	}
}

// TestOneRulerPerPrefecture 釘住身分欄的意義。
//
// 每個有主的郡剛好一位主事者：身分為君主或太守，兩者都不在時由軍師代理。
// 六個劇本 × 兩版共 193 個有主的郡全部成立（代理出現兩次）。
//
// 這條同時檢查了身分、勢力、所在郡與郡的歸屬四個欄位——只要有一個
// 偏移錯掉，「剛好一位」就會塌掉。
func TestOneRulerPerPrefecture(t *testing.T) {
	for _, ver := range versions {
		c := loadData2(t, ver)
		for _, slot := range []Slot{Scenario1, Scenario2, Scenario3, Scenario4, Scenario5, Scenario6} {
			sc, err := LoadScenario(c, slot)
			if err != nil {
				t.Fatalf("%s %s：%v", ver, slot, err)
			}
			owned := 0
			for _, p := range sc.Prefectures() {
				if !p.Owned() {
					continue
				}
				owned++
				g, ok := sc.Governor(p.ID)
				if !ok {
					t.Errorf("%s %s：郡 %d %s 有主（勢力 %d）卻找不到唯一的主事者",
						ver, slot, p.ID, p.Name, p.Owner)
					continue
				}
				if g.Faction != p.Owner {
					t.Errorf("%s %s：郡 %d %s 的主事者 %s 屬於勢力 %d，不是 %d",
						ver, slot, p.ID, p.Name, g.Name, g.Faction, p.Owner)
				}
			}
			if owned == 0 {
				t.Errorf("%s %s：一個有主的郡都沒有——歸屬欄大概讀錯了", ver, slot)
			}
		}
	}
}

// TestGeneralAttributeRanges 釘住四個屬性都在 0..100。
//
// **越界不會讓程式壞掉，只會讓數值悄悄失真。** 偏移錯一格時，
// 讀到的多半仍是「一個看起來像數字的東西」。
func TestGeneralAttributeRanges(t *testing.T) {
	c := loadData2(t, "三國演義")
	sc, err := LoadScenario(c, Scenario1)
	if err != nil {
		t.Fatal(err)
	}
	for _, g := range sc.People() {
		fields := []struct {
			name string
			v    uint8
		}{{"體能", g.Stamina}, {"謀略", g.Intel}, {"戰力", g.War}, {"魅力", g.Charm}}
		// 忠誠只在有主子時有意義；在野者是 0xFF 哨兵。
		if g.HasLoyalty() {
			fields = append(fields, struct {
				name string
				v    uint8
			}{"忠誠", g.Loyalty})
		} else if g.Employed() {
			t.Errorf("%s 有勢力卻沒有忠誠值", g.Name)
		}
		for _, f := range fields {
			if f.v > 100 {
				t.Errorf("%s 的%s是 %d，超過 100", g.Name, f.name, f.v)
			}
		}
		if g.Rank > RankJuniorGeneral {
			t.Errorf("%s 的職位是 %d，字串表只有 9 格", g.Name, g.Rank)
		}
		if g.Troop > 6 {
			t.Errorf("%s 的兵種是 %d，字串表只有 7 格", g.Name, g.Troop)
		}
	}
}

// TestAdjacencyIsSymmetricAndConnected 釘住地圖圖形。
//
// 相鄰表是**規則層的地基**（行軍、開戰、外交都靠它），而它壞掉的方式
// 很安靜：單向的邊會讓部隊走得過去回不來，斷開的分量會讓一整塊地圖
// 永遠打不到——兩者都不會報錯。
func TestAdjacencyIsSymmetricAndConnected(t *testing.T) {
	for _, ver := range versions {
		c := loadData2(t, ver)
		sc, err := LoadScenario(c, Scenario1)
		if err != nil {
			t.Fatal(err)
		}
		edges := 0
		for _, p := range sc.Prefectures() {
			if len(p.Neighbours) == 0 {
				t.Errorf("%s：郡 %d %s 沒有鄰居——它會變成孤島", ver, p.ID, p.Name)
			}
			for _, n := range p.Neighbours {
				edges++
				if !sc.Adjacent(n, p.ID) {
					t.Errorf("%s：%d %s 說 %d 是鄰居，但反過來不成立", ver, p.ID, p.Name, n)
				}
				if n == p.ID {
					t.Errorf("%s：郡 %d %s 把自己列為鄰居", ver, p.ID, p.Name)
				}
			}
		}
		if edges%2 != 0 {
			t.Errorf("%s：邊的端點總數是奇數 %d——一定有單向的邊", ver, edges)
		}

		// 全連通：從 1 號郡走得到全部 42 個。
		seen := map[int]bool{1: true}
		stack := []int{1}
		for len(stack) > 0 {
			x := stack[len(stack)-1]
			stack = stack[:len(stack)-1]
			p, err := sc.Prefecture(x)
			if err != nil {
				t.Fatal(err)
			}
			for _, n := range p.Neighbours {
				if !seen[n] {
					seen[n] = true
					stack = append(stack, n)
				}
			}
		}
		if len(seen) != PrefectureCount {
			t.Errorf("%s：從郡 1 只走得到 %d 個郡，應該是 %d 個",
				ver, len(seen), PrefectureCount)
		}
	}
}

// TestAdjacencyIsMapNotState 釘住「相鄰表是地圖不是劇本狀態」。
//
// 六個劇本共用同一張圖。哪天不一樣了，代表這個欄位其實不是相鄰表，
// 或者版本之間動過地圖——兩種都要知道。
func TestAdjacencyIsMapNotState(t *testing.T) {
	for _, ver := range versions {
		c := loadData2(t, ver)
		var base [][]int
		for _, slot := range []Slot{Scenario1, Scenario2, Scenario3, Scenario4, Scenario5, Scenario6} {
			sc, err := LoadScenario(c, slot)
			if err != nil {
				t.Fatalf("%s %s：%v", ver, slot, err)
			}
			var got [][]int
			for _, p := range sc.Prefectures() {
				got = append(got, p.Neighbours)
			}
			if base == nil {
				base = got
				continue
			}
			for i := range got {
				if len(got[i]) != len(base[i]) {
					t.Errorf("%s %s：郡 %d 的鄰居數與劇本 001 不同", ver, slot, i+1)
					continue
				}
				for j := range got[i] {
					if got[i][j] != base[i][j] {
						t.Errorf("%s %s：郡 %d 的鄰居與劇本 001 不同", ver, slot, i+1)
						break
					}
				}
			}
		}
	}
}

// TestAICostFactor 釘住電腦諸侯的花費係數。
//
// 表是從原版記憶體讀出來的（`0x048074` 起的六個 double：
// 1、1、1、1、0.9、0.75）。**判準是截斷取整的實際數字**——
// 等級 5 那一列的十九個樣本都與 `×3/4` 相符。
func TestAICostFactor(t *testing.T) {
	for _, c := range []struct{ base, level, want int }{
		{100, 0, 100}, {100, 3, 100}, {100, 4, 90}, {100, 5, 75},
		// 原版量到的樣本（等級 5）
		{14, 5, 10}, {17, 5, 12}, {18, 5, 13}, {2, 5, 1},
		{21, 5, 15}, {323, 5, 242}, {33, 5, 24}, {5, 5, 3},
		{6, 5, 4}, {81, 5, 60}, {9, 5, 6}, {92, 5, 69},
	} {
		if got := AICost(c.base, c.level); got != c.want {
			t.Errorf("等級 %d 付 %d：算出 %d，原版是 %d",
				c.level, c.base, got, c.want)
		}
	}
}
