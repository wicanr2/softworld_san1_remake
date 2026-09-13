package save_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/wicanr2/softworld_san1_remake/internal/assets"
	"github.com/wicanr2/softworld_san1_remake/internal/save"
	"github.com/wicanr2/softworld_san1_remake/internal/state"
)

// data2 開原版的 DATA2 容器；沒素材就 skip。
func data2(t *testing.T) *assets.Container {
	t.Helper()
	root := os.Getenv("SAN1_ORIG")
	if root == "" {
		t.Skip("沒設 SAN1_ORIG，跳過（本儲存庫不含原版檔案）")
	}
	dir := filepath.Join(root, "三國演義")
	read := func(ext string) []byte {
		b, err := os.ReadFile(filepath.Join(dir, "DATA2."+ext))
		if err != nil {
			t.Skipf("讀不到 DATA2.%s：%v", ext, err)
		}
		return b
	}
	c, err := assets.OpenContainer(read("NAM"), read("IDX"), read("GRP"))
	if err != nil {
		t.Fatal(err)
	}
	return c
}

// TestListOriginal 列原版出貨的六個進度。年份要對得上名稱裡寫的年份。
func TestListOriginal(t *testing.T) {
	list := save.ListOriginal(data2(t))
	if len(list) != save.OriginalSlots {
		t.Fatalf("列出 %d 個槽", len(list))
	}
	years := []int{197, 198, 201, 208, 215, 220}
	for i, info := range list {
		if !info.Exists {
			t.Errorf("第 %d 個進度讀不出來", i+1)
			continue
		}
		if info.Year != years[i] {
			t.Errorf("第 %d 個進度是 %d 年，想要 %d 年", i+1, info.Year, years[i])
		}
		if info.Name == "" {
			t.Errorf("第 %d 個進度沒有名稱", i+1)
		}
	}
	// 名稱裡寫的年份與 BASEPRO 的年份是**兩份獨立資料**，對得上才算讀對。
	for _, i := range []int{2, 3, 4, 5} {
		if !strings.Contains(list[i].Name, "Y") {
			continue
		}
		if !strings.Contains(list[i].Describe(), "年") {
			t.Errorf("第 %d 個進度的說明是 %q", i+1, list[i].Describe())
		}
	}
}

// TestReadOriginal 把原版出貨的進度讀成一局。
//
// **第 2 個進度讀不進來是預期的**：它存的難度是 15，而原版的輸入畫面
// 只給到 10、係數表也只有 11 格（`docs/re/08` §5）。這種情形要報錯，
// 不能夾成 10——夾過的難度會安靜地把整局的行為換掉。
func TestReadOriginal(t *testing.T) {
	c := data2(t)
	for slot := 1; slot <= save.OriginalSlots; slot++ {
		g, err := save.ReadOriginal(c, slot, state.EditionBase)
		if slot == 2 {
			if err == nil {
				t.Errorf("第 2 個進度的難度是 15，應該被擋下來")
			}
			continue
		}
		if err != nil {
			t.Errorf("第 %d 個進度：%v", slot, err)
			continue
		}
		if g.Date.Month < 1 || g.Date.Month > 12 {
			t.Errorf("第 %d 個進度的月份是 %d", slot, g.Date.Month)
		}
		if g.Player == state.NoFaction {
			t.Errorf("第 %d 個進度沒有玩家", slot)
		}
		// 讀進來的盤面要像一局在玩的遊戲：有人有領地、有人活著。
		owned := 0
		for _, p := range g.Prefectures() {
			if p.Owner != state.NoFaction {
				owned++
			}
		}
		if owned == 0 {
			t.Errorf("第 %d 個進度一個有主的郡都沒有", slot)
		}
		// **玩家沒有領地不是錯誤**：出貨的第 1 個進度就是這樣——
		// 玩家是自創君主（諸侯槽 14、人物槽 346），一個郡都不剩，
		// 那是輸掉的局面，不是讀壞。
		if slot != 1 && len(g.Territory(g.Player)) == 0 {
			t.Errorf("第 %d 個進度的玩家沒有領地", slot)
		}
	}
}

// TestReadOriginalKeepsCommanded 釘住「這個月哪些郡還沒下令」跟著進來。
//
// 這是 `BASEPRO` 那 43 格旗標唯一看得見的用途，而出貨的六個進度剛好
// 涵蓋從月初（第 6 個，42 個郡還沒下令）到月尾（第 3 個，剩 5 個）。
func TestReadOriginalKeepsCommanded(t *testing.T) {
	c := data2(t)
	for _, tc := range []struct{ slot, pending int }{{3, 5}, {6, 42}} {
		g, err := save.ReadOriginal(c, tc.slot, state.EditionBase)
		if err != nil {
			t.Fatalf("第 %d 個進度：%v", tc.slot, err)
		}
		n := 0
		for _, p := range g.Prefectures() {
			if !p.Commanded {
				n++
			}
		}
		if n != tc.pending {
			t.Errorf("第 %d 個進度有 %d 個郡還沒下令，想要 %d",
				tc.slot, n, tc.pending)
		}
	}
}

// TestOriginalCustomLord 釘住自創君主：出貨的第 1 個進度由諸侯槽 14
// 控制，它的君主是人物槽 346，姓名是三個造字（Big5 `A141`–`A143`，
// 在標準對照表裡是三個全形標點）。
//
// 那三個碼位的字模在 `BASEPRE.SV1` 裡（`docs/re/08` §3），所以名字
// 讀起來像標點是正常的——原版把標點的碼位挪用成玩家自己畫的字。
func TestOriginalCustomLord(t *testing.T) {
	c := data2(t)
	g, err := save.ReadOriginal(c, 1, state.EditionBase)
	if err != nil {
		t.Fatal(err)
	}
	if g.Player != 14 {
		t.Errorf("第 1 個進度的玩家是勢力 %d，想要 14", g.Player)
	}
	lord := g.Lord(g.Player)
	if lord == nil {
		t.Fatal("第 1 個進度的玩家沒有君主")
	}
	if lord.Index != 346 {
		t.Errorf("君主是人物槽 %d，想要 346", lord.Index)
	}
	if lord.Name != "，、。" {
		t.Errorf("君主的姓名是 %q，想要三個造字碼位 A141–A143", lord.Name)
	}
	// 第 1 格的 BASEPRO 游標指到玩家郡 41。原版接回月內迴圈時先重整
	// 守將清單再顯示主命令，所以檔案裡的 0／0 到畫面時已成為 5／1。
	if got := g.Troops(41); got != 5 {
		t.Errorf("玩家郡 41 的兵士快照是 %d，原版載入後是 5", got)
	}
	if got := g.StoredActiveGenerals(41); got != 1 {
		t.Errorf("玩家郡 41 的現役將快照是 %d，原版載入後是 1", got)
	}
}
