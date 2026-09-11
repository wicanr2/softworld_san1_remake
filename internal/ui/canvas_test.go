package ui

import (
	"image/color"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/wicanr2/softworld_san1_remake/internal/assets"
	"github.com/wicanr2/softworld_san1_remake/internal/cells"
	"github.com/wicanr2/softworld_san1_remake/internal/font"
	"github.com/wicanr2/softworld_san1_remake/internal/game"
	"github.com/wicanr2/softworld_san1_remake/internal/i18n"
	"github.com/wicanr2/softworld_san1_remake/internal/state"
)

var (
	bg = color.RGBA{0, 0, 0, 255}
	fg = color.RGBA{255, 255, 255, 255}
)

func testFace(t *testing.T) *font.Face {
	t.Helper()
	fh, err := os.Open("../../fonts/unifont.hex.gz")
	if err != nil {
		t.Skipf("沒有字型檔：%v", err)
	}
	defer fh.Close()
	f, err := font.ParseHexGz(fh, 16)
	if err != nil {
		t.Fatalf("字型載入失敗：%v", err)
	}
	return f
}

func TestDrawTextInk(t *testing.T) {
	f := testFace(t)
	c := NewCanvas(20, 3, f)
	c.Fill(bg)

	// 沒畫之前每一格都是空的——先確認基準，否則「有墨水」證明不了什麼。
	if n := c.InkAt(0, 0, bg); n != 0 {
		t.Fatalf("還沒畫，(0,0) 就有 %d 個像素", n)
	}

	w := c.DrawText(0, 0, "遼東", fg)
	if w != 4 {
		t.Errorf("畫了 %d 格，想要 4（兩個全形字）", w)
	}
	for col := 0; col < 4; col++ {
		if c.InkAt(col, 0, bg) == 0 {
			t.Errorf("第 %d 格是空的——字沒畫出來", col)
		}
	}
	// 第 4 格之後不該有東西。
	if n := c.InkAt(4, 0, bg); n != 0 {
		t.Errorf("第 4 格有 %d 個像素，畫超出去了", n)
	}
	if len(c.Missing) != 0 {
		t.Errorf("有缺字：%v", c.Missing)
	}
}

// TestDrawTextClips 釘住「超出右緣不繞行」。
//
// 原版的版面是固定格的，繞行會蓋掉下一列的東西。
func TestDrawTextClips(t *testing.T) {
	f := testFace(t)
	c := NewCanvas(4, 2, f)
	c.Fill(bg)
	w := c.DrawText(0, 0, "遼東涿郡", fg) // 8 格，畫布只有 4 格
	if w != 4 {
		t.Errorf("畫了 %d 格，想要 4（其餘裁掉）", w)
	}
	// 第 1 列必須完全乾淨——繞行的話這裡會有東西。
	for col := 0; col < 4; col++ {
		if n := c.InkAt(col, 1, bg); n != 0 {
			t.Errorf("第 1 列第 %d 格有 %d 個像素——文字繞行了", col, n)
		}
	}
}

// TestMissingIsRecorded 釘住「缺字要記下來，不要靜靜畫成空白」。
//
// 用一份自造的小字型，不用真字型去找「它沒有的字」——
// 第一版那樣寫，挑的碼位 unifont 居然有（它給未指派區也附了字模），
// 測試就變成在賭字型的涵蓋範圍。自造的字型知道自己有什麼。
func TestMissingIsRecorded(t *testing.T) {
	f, err := font.ParseHex(strings.NewReader("0041:FF00\n"), 2) // 只有 'A'
	if err != nil {
		t.Fatal(err)
	}
	c := NewCanvas(8, 1, f)
	c.Fill(bg)
	c.DrawText(0, 0, "AB", fg)
	if c.Missing['B'] != 1 {
		t.Errorf("缺字沒被記下來：%v", c.Missing)
	}
	if c.Missing['A'] != 0 {
		t.Errorf("'A' 有字模，不該記成缺字：%v", c.Missing)
	}
	// 'A' 畫在第 0 格，'B' 缺字所以第 1 格必須是空的——
	// 而且游標**有前進**（缺字不會讓後面的字擠上來）。
	if c.InkAt(0, 0, bg) == 0 {
		t.Error("'A' 沒畫出來")
	}
	if n := c.InkAt(1, 0, bg); n != 0 {
		t.Error("缺字時不該畫東西（畫豆腐塊會讓缺字看起來像設計）")
	}
}

// TestPrefectureRowFits 是版面測試：42 個郡名排成六欄，
// 每個槽 6 格寬時所有名字都要畫得出來、而且不溢出。
func TestPrefectureRowFits(t *testing.T) {
	f := testFace(t)
	const slot = 6
	names := []string{"遼東", "涿郡", "渤海", "鄴郡", "太原", "上黨"}
	c := NewCanvas(slot*len(names), 1, f)
	c.Fill(bg)
	for i, n := range names {
		if !cells.Fits(n, slot) {
			t.Errorf("%q 放不進 %d 格", n, slot)
		}
		if got := c.DrawText(i*slot, 0, cells.Center(n, slot), fg); got != slot {
			t.Errorf("%q 畫了 %d 格，想要 %d", n, got, slot)
		}
	}
	if len(c.Missing) != 0 {
		t.Errorf("有缺字：%v", c.Missing)
	}
	// 每個槽的中間兩格要有墨水（Center 之後名字在中間）。
	for i := range names {
		mid := i*slot + slot/2 - 1
		if c.InkAt(mid, 0, bg) == 0 {
			t.Errorf("第 %d 個槽的中間是空的", i)
		}
	}
}

func TestDrawBox(t *testing.T) {
	f := testFace(t)
	c := NewCanvas(6, 3, f)
	c.Fill(bg)
	c.DrawBox(0, 0, 6, 3, fg)
	// 框內部（第 1 列中間的格）不該被框線碰到。
	if n := c.InkAt(2, 1, bg); n != 0 {
		t.Errorf("框內部有 %d 個像素，框線畫進去了", n)
	}
	// 角落要有。
	if c.InkAt(0, 0, bg) == 0 {
		t.Error("左上角沒有框線")
	}
}

// TestSubMenusMatchTheOriginal 釘住子選單的文字與原版執行檔的字串表相同
// （`docs/re/04` §2）。
//
// ⚠ **不要照手冊改。** 手冊是二手轉錄，與程式有出入：「其他」的第一項
// 原版寫 `結束`，手冊寫「＊結束」。原版自己的錯字（洪水防**冶**、
// 郡縣自**冶**）也照原樣留著——這是保存專案，改正等於在還原品上留下
// 一處與原版不同而沒人記得的地方（同 `Floopy`，F29）。
func TestSubMenusMatchTheOriginal(t *testing.T) {
	cases := []struct {
		key   byte
		title string
		items []string
		// remake 是 remake 自己加在後面的項目。**分開列是為了讓
		// 「原版有哪幾項」這個斷言不被稀釋**：混在同一個清單裡，
		// 之後就沒有人分得出哪幾項是原版的。
		remake []string
	}{
		{'1', "查看", []string{"選擇州郡", "將軍列表", "檢視將軍", "領土列表", "郡地理誌", "君主物品"}, nil},
		{'2', "軍事", []string{"調動軍隊", "發動戰役", "運送錢糧"}, nil},
		{'3', "兵士", []string{"訓練兵士", "徵兵", "購買武器", "調整兵力"}, nil},
		{'4', "內政", []string{"土地開發", "洪水防冶", "建築關寨", "休息"}, nil},
		{'5', "商業", []string{"買入米糧", "賣出米糧", "開倉賑民"}, nil},
		{'6', "人事", []string{"尋訪人才", "登用人才", "賞賜金帛", "撤職"}, nil},
		{'7', "君主", []string{"指定軍師", "指定太守", "郡縣自冶", "賞賜物品", "登用他國人才"}, nil},
		{'8', "謀略", []string{"驅虎吞狼", "遠交近攻", "偽書使疑", "策反人民", "聯合出兵"}, nil},
		{'9', "其他", []string{"結束", "儲存", "音樂", "音效", "延時", "戰役", "年號", "語音"},
			[]string{"電腦AI", "電腦指令"}},
	}
	for _, c := range cases {
		title, items := SubMenu(c.key)
		if title != c.title {
			t.Errorf("第 %q 類叫 %q，原版寫 %q", c.key, title, c.title)
		}
		want := append(append([]string(nil), c.items...), c.remake...)
		if len(items) != len(want) {
			t.Errorf("%s 有 %d 項，原版 %d 項 ＋ remake %d 項",
				c.title, len(items), len(c.items), len(c.remake))
			continue
		}
		for i, w := range want {
			if items[i].Name != w {
				t.Errorf("%s 第 %d 項是 %q，應該是 %q", c.title, i+1, items[i].Name, w)
			}
			// **remake 加的項目不能擠動原版那幾項的編號**：玩家的手指
			// 記得「9-3 是音樂」，換一個位置等於把原版的操作改掉。
			if items[i].Key != MenuKey(i) {
				t.Errorf("%s 的 %q 按鍵是 %q，應該是 %q", c.title, w, items[i].Key, MenuKey(i))
			}
		}
	}
}

// TestMenuKeyTenthIsZero 釘住第十項按 `0`。
//
// `byte('1'+i)` 到第十項會算出 `:`，而鍵盤只送得進 `0`–`9`
//（`cmd/san1` 的 `press`）——那一項會永遠按不動，畫面上卻完全正常。
func TestMenuKeyTenthIsZero(t *testing.T) {
	seen := map[byte]bool{}
	for i := 0; i < 10; i++ {
		k := MenuKey(i)
		if k < '0' || k > '9' {
			t.Errorf("第 %d 項的鍵是 %q，鍵盤送不進來", i+1, k)
		}
		if seen[k] {
			t.Errorf("第 %d 項的鍵 %q 與前面某一項重複", i+1, k)
		}
		seen[k] = true
	}
	if MenuKey(9) != '0' {
		t.Errorf("第十項的鍵是 %q，應該是 '0'", MenuKey(9))
	}
}

// TestOtherSubMenuKeepsTheOriginalEight 釘住「其他」的前八項還是原版那八項。
//
// remake 在後面加了「電腦AI」與「電腦指令」（`docs/design/02` §5）。
// **加在後面是條件**：原版的八項要留在原來的編號上。
func TestOtherSubMenuKeepsTheOriginalEight(t *testing.T) {
	title, items := SubMenu('9')
	if title != "其他" {
		t.Errorf("第 9 類叫 %q，應該是「其他」", title)
	}
	want := []string{"結束", "儲存", "音樂", "音效", "延時", "戰役", "年號", "語音"}
	if len(items) < len(want) {
		t.Fatalf("其他只有 %d 項，手冊列八項", len(items))
	}
	for i, w := range want {
		if items[i].Name != w {
			t.Errorf("第 %d 項是 %q，應該是 %q", i+1, items[i].Name, w)
		}
		if items[i].Key != byte('1'+i) {
			t.Errorf("%q 的按鍵是 %q，應該是 %q", w, items[i].Key, byte('1'+i))
		}
	}
}

// TestEveryMainCommandHasASubMenu 釘住十個指令都有東西可按。
//
// 0 是「狀態」，原版直接顯示在畫面上不必展開。
func TestEveryMainCommandHasASubMenu(t *testing.T) {
	for _, c := range Commands() {
		if c.Key == '0' {
			continue
		}
		title, items := SubMenu(c.Key)
		if title == "" || len(items) == 0 {
			t.Errorf("「%s」（%q）沒有子選單", c.Name, c.Key)
		}
	}
}

// TestTimeColumnShowsTheEra 釘住左側直排顯示的是年號，而且切得掉。
//
// 原版主畫面左側直排寫「中平六年元月春」（F41 ＋ 季節那一格）。
// 這一條盯的是**畫面上真的畫出那七個字**——`Date.FormatWithSeason` 對
// 不代表有人叫它。
func TestTimeColumnShowsTheEra(t *testing.T) {
	face := testFace(t)
	for _, c := range []struct {
		cal  game.Calendar
		text string
	}{
		{game.ChineseEra, "中平六年元月春"},
		{game.Western, "189年1月春"},
	} {
		canvas := NewCanvas(Cols, Rows, face)
		canvas.Fill(ColBG)
		drawTimeColumn(canvas, game.Date{Year: 189, Month: 1}, c.cal)
		// 逐字比：每個字畫在 timeCol+2 那一欄，從第 2 列往下。
		for i, r := range []rune(c.text) {
			row := 2 + i
			if row >= Rows-1 {
				break
			}
			if canvas.InkAt(timeCol+2, row, ColBG) == 0 {
				t.Errorf("%s：第 %d 列（應該是 %q）沒有畫東西", c.cal.Name(), row, r)
			}
		}
		if len(canvas.Missing) > 0 {
			t.Errorf("%s：有畫不出來的字 %v", c.cal.Name(), canvas.Missing)
		}
	}
}

// TestGeneralPageShowsTheOriginalFields 釘住檢視將軍的欄位與原版對齊。
//
// 原版的檢視畫面是 `体能 %3d`／`謀略 %3d 兵士數 %4d`／
// `戰力 %3d 訓練度 %3d`／`魅力 %3d 武裝度 %3d`（`docs/re/04` §7）。
// **原版寫的是「体能」不是「體能」**，照它。
func TestGeneralPageShowsTheOriginalFields(t *testing.T) {
	g := loadGame(t)
	title, lines := GeneralPage(g, 0)
	if title != "檢視將軍" {
		t.Errorf("標題是 %q", title)
	}
	body := strings.Join(lines, "\n")
	for _, w := range []string{"体能", "謀略", "戰力", "魅力",
		"兵士數", "訓練度", "武裝度", "忠心度", "人氏", "帶兵上限"} {
		if !strings.Contains(body, w) {
			t.Errorf("檢視頁沒有 %q", w)
		}
	}
	if _, empty := GeneralPage(g, 99999); len(empty) == 0 {
		t.Error("越界的槽號也該有一行說明")
	}
}

// TestBattleReportPage 釘住戰報頁看得到勝負、天數、折損與逐日紀錄。
func TestBattleReportPage(t *testing.T) {
	g := loadGame(t)
	r := &game.BattleResult{From: 15, To: 14, AttackerWon: true, Days: 12,
		AttackerLost: 1200, DefenderLost: 3400,
		Captives: []game.Captive{{General: 1, Name: "某將"}},
		Log:      []string{"第 1 日　戰役開始（刮風）", "第 2 日　主攻軍先鋒 攻 主守軍中軍"}}
	title, lines := BattleReport(g, r)
	if title != "戰報" {
		t.Errorf("標題是 %q", title)
	}
	body := strings.Join(lines, "\n")
	for _, w := range []string{"攻方獲勝", "12 日", "1200", "3400", "某將", "戰役開始"} {
		if !strings.Contains(body, w) {
			t.Errorf("戰報頁沒有 %q", w)
		}
	}
	if _, empty := BattleReport(g, nil); len(empty) == 0 {
		t.Error("沒有戰役時也該有一行說明")
	}
}

// TestBattleListNewestFirst 釘住戰役紀錄新的排前面。
func TestBattleListNewestFirst(t *testing.T) {
	g := loadGame(t)
	rs := []*game.BattleResult{
		{From: 1, To: 2, Days: 3},
		{From: 15, To: 14, Days: 30, AttackerWon: true},
	}
	_, lines := BattleList(g, rs)
	if len(lines) != 2 {
		t.Fatalf("列出 %d 行", len(lines))
	}
	if !strings.Contains(lines[0], "30 日") {
		t.Errorf("第一行是 %q，應該是最近的那一場", lines[0])
	}
	if _, empty := BattleList(g, nil); len(empty) == 0 {
		t.Error("沒有紀錄時也該有一行說明")
	}
}

// loadGame 開一局；沒有原版素材就 skip。**本儲存庫不含原版檔案。**
func loadGame(t *testing.T) *game.State {
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
	sc, err := state.LoadScenario(c, state.Scenario1)
	if err != nil {
		t.Fatal(err)
	}
	g, err := game.New(sc, 0, 5, state.EditionBase)
	if err != nil {
		t.Fatal(err)
	}
	return g
}

// TestFontCoversEveryLocale 釘住每個語系的每一個字都畫得出來。
//
// **缺字在畫面上是空白，而空白看起來像排版問題。** 譯文加一個字型
// 沒有的字，跑起來只會少一格，不會有任何錯誤。
func TestFontCoversEveryLocale(t *testing.T) {
	face := testFace(t)
	for _, l := range i18n.Locales() {
		bad := map[rune]bool{}
		for _, s := range i18n.All(l) {
			for _, r := range face.Covers(s) {
				// 格式動詞與標點不必畫。
				if r == '%' || r == 'd' || r == 's' {
					continue
				}
				bad[r] = true
			}
		}
		if len(bad) > 0 {
			var list []string
			for r := range bad {
				list = append(list, string(r))
			}
			t.Errorf("%s 有字型畫不出來的字：%v", l, list)
		}
	}
}
