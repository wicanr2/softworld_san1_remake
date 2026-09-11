package ui

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/wicanr2/softworld_san1_remake/internal/assets"
	"github.com/wicanr2/softworld_san1_remake/internal/cells"
	"github.com/wicanr2/softworld_san1_remake/internal/game"
	"github.com/wicanr2/softworld_san1_remake/internal/i18n"
	"github.com/wicanr2/softworld_san1_remake/internal/state"
)

// 原版素材主畫面上的指令表、子選單、挑選清單與分頁（`docs/spec/014`）。

// artSessionFixture 開一局劇本 001 與接上原版素材的主畫面；沒有素材就 skip。
func artSessionFixture(t *testing.T) (*ArtScreen, *game.State) {
	t.Helper()
	root := os.Getenv("SAN1_ORIG")
	if root == "" {
		t.Skip("沒設 SAN1_ORIG，跳過（本儲存庫不含原版檔案）")
	}
	dir := filepath.Join(root, "三國演義")
	open := func(name string) *assets.Container {
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
	c2, c3, c1 := open("DATA2"), open("DATA3"), open("DATA1")
	sc, err := state.LoadScenario(c2, state.Scenario1)
	if err != nil {
		t.Fatal(err)
	}
	g, err := game.New(sc, 0, 5, state.EditionBase)
	if err != nil {
		t.Fatal(err)
	}
	a, err := NewArtScreen(c3, c1)
	if err != nil {
		t.Fatal(err)
	}
	return a, g
}

// TestArtSessionDrawsWhatThePlayerNeeds 釘住玩家下令時要看的每一樣東西
// 都真的畫出來了。
//
// **這一支先前的結果是全部 0**：`DrawArtSession` 只畫提示，打開子選單、
// 挑選清單、數字輸入、分頁與沒打開畫出來逐位元組相同——而文字版一直都
// 有畫，所以兩者的差別在別的測試裡看不出來。判準是「打開與沒打開要不同」，
// 而且不同的地方要落在該落的那一塊。
func TestArtSessionDrawsWhatThePlayerNeeds(t *testing.T) {
	a, g := artSessionFixture(t)
	face := testFace(t)
	draw := func(v View) *Canvas {
		c := NewCanvasPx(assets.ScreenW, assets.ScreenH, face)
		DrawArtSession(c, a, g, nil, v)
		return c
	}
	base := draw(View{})
	title, items := SubMenu('9')
	cases := []struct {
		name string
		v    View
		// 變化要落在這一塊（像素）裡。
		x0, y0, x1, y1 int
	}{
		{"其他子選單（下面板）", View{Menu: title, Items: items}, 408, 292, 632, 372},
		{"挑選清單（上面板）", View{Menu: "挑哪一位", Items: []Command{
			{Key: '1', Name: "陳就"}, {Key: '2', Name: "黃祖"}}}, 408, 36, 632, 292},
		{"數字輸入（上面板）", View{Menu: "設定延遲時間", Items: []Command{
			{Key: '=', Name: "30"}, {Key: ' ', Name: "上限 100"}}}, 408, 36, 632, 292},
		{"郡的資料（上面板）", View{Status: true}, 408, 36, 632, 292},
		{"分頁（內容區）", View{PageTitle: "測試", Page: []string{"第一行", "第二行"}},
			artPageX0, artPageY0, artPageX1, artPageY1},
		{"結局（下面板）", View{Over: true}, 408, 292, 632, 372},
	}
	for _, c := range cases {
		got := draw(c.v)
		n, outside := 0, 0
		w := assets.ScreenW
		for i := 0; i < len(base.Img.Pix); i += 4 {
			if string(base.Img.Pix[i:i+4]) == string(got.Img.Pix[i:i+4]) {
				continue
			}
			n++
			x, y := (i/4)%w, (i/4)/w
			if x < c.x0 || x >= c.x1 || y < c.y0 || y >= c.y1 {
				outside++
			}
		}
		if n == 0 {
			t.Errorf("%s：打開與沒打開畫出來一模一樣——玩家看不到它", c.name)
			continue
		}
		if outside > 0 {
			t.Errorf("%s：%d 個像素變了，其中 %d 個落在該畫的那一塊外面", c.name, n, outside)
		}
	}
}

// TestSubMenuLinesMatchTheOriginal 釘住中文的子選單與原版字串逐行相同。
//
// 字串是 `AA.EXE` 的原文（`docs/re/04` §2），斷行也是——原版畫在下面板
// 上的就是這幾行（`docs/spec/014` §2.2，`sub-1`／`sub-9` 逐群量過）。
func TestSubMenuLinesMatchTheOriginal(t *testing.T) {
	saved := i18n.Current
	defer func() { i18n.Current = saved }()
	i18n.Current = i18n.ZhHant

	orig := map[byte]string{
		'1': "1.選擇州郡 2.將軍列表\n3.檢視將軍 4.領土列表\n5.郡地理誌 6.君主物品\n請選擇:",
		'2': "1.調動軍隊\n2.發動戰役\n3.運送錢糧\n請下命令:",
		'3': "1.訓練兵士 2.徵兵\n3.購買武器 4.調整兵力\n請下命令:",
		'4': "1.土地開發 2.洪水防冶\n3.建築關寨 4.休息\n請下命令:",
		'5': "1.買入米糧 2.賣出米糧\n3.開倉賑民\n請下命令:",
		'6': "1.尋訪人才 2.登用人才\n3.賞賜金帛 4.撤職\n請下命令:",
		'7': "1.指定軍師 2.指定太守\n3.郡縣自冶 4.賞賜物品\n5.登用他國人才\n請下命令:",
		'8': "1.驅虎吞狼 2.遠交近攻\n3.偽書使疑 4.策反人民\n5.聯合出兵\n那一頂:",
	}
	w := (artRightR - artMsgX) / CellW
	for k := byte('1'); k <= '8'; k++ {
		_, items := SubMenu(k)
		got := strings.Join(SubMenuLines(k, items, w), "\n")
		if got != orig[k] {
			t.Errorf("第 %c 類：\n  排出來 %q\n  原版是 %q", k, got, orig[k])
		}
	}
	// 「其他」多兩項（remake 加的），前兩行要與原版逐字相同。
	_, items := SubMenu('9')
	lines := SubMenuLines('9', items, w)
	want := []string{"1.結束 2.儲存 3.音樂", "4.音效 5.延時 6.戰役"}
	for i, l := range want {
		if i >= len(lines) || lines[i] != l {
			t.Errorf("其他第 %d 行是 %q，原版是 %q", i+1, lines[min(i, len(lines)-1)], l)
		}
	}
	if last := lines[len(lines)-1]; !strings.HasSuffix(last, "請選擇:") {
		t.Errorf("其他最後一行是 %q，要以提示字結尾", last)
	}
}

// TestSubMenusShowInFullInEveryLanguage 釘住三個語系、九類子選單都完整
// 畫得出來，而且**中文九類都在下面板**（與原版相同）。
//
// 英文常比中文長（`CLAUDE.md` §3.3），塞不下的時候**不會報錯**：多出來的
// 行被下面板截掉，玩家看到的就是少了幾個選項。所以放不下的要換地方——
// 下面板 → 上面板一行一項 → 整個內容區（`docs/spec/014` §3.2），而且
// 換到哪裡要列得出來，那就是這一格的 remake 差異清單。
func TestSubMenusShowInFullInEveryLanguage(t *testing.T) {
	saved := i18n.Current
	defer func() { i18n.Current = saved }()
	pageCols := (artPageX1-artPageX0)/CellW - 2
	pageRows := (artPageY1-artPageY0)/CellH - 2 // 標題與提示各佔一行
	for _, l := range []i18n.Locale{i18n.ZhHant, i18n.En, i18n.Ja} {
		i18n.Current = l
		var moved []string
		for k := byte('1'); k <= '9'; k++ {
			title, items := SubMenu(k)
			if subMenuFitsLower(k, items) {
				continue
			}
			if l == i18n.ZhHant {
				t.Errorf("中文第 %c 類排不進下面板——原版九類都在下面板", k)
			}
			lines := commandLines(items)
			where := "上面板"
			for _, line := range append([]string{title}, lines...) {
				if cells.Width(line) > artUpperCols {
					where = "內容區"
				}
			}
			if len(lines)+1 > artUpperRows {
				where = "內容區"
			}
			if where == "內容區" {
				for _, line := range lines {
					if cells.Width(line) > pageCols {
						t.Errorf("%s 第 %c 類有一項 %d 格，連內容區（%d 格）都放不下：%q",
							l, k, cells.Width(line), pageCols, line)
					}
				}
				if len(lines) > pageRows {
					t.Errorf("%s 第 %c 類有 %d 項，內容區只放得下 %d 行", l, k, len(lines), pageRows)
				}
			}
			moved = append(moved, fmt.Sprintf("%c→%s", k, where))
		}
		if len(moved) > 0 {
			t.Logf("%s 排不進下面板、改畫別處的子選單：%v", l, moved)
		}
	}
}

// TestCommandMenuLayout 釘住十項指令表的格位與顏色（`docs/spec/014` §2.1）。
//
// 字模是 remake 自己的，所以不逐像素比原版；比的是**墨水落在哪一格**：
// 每一項的名稱在自己那一列（列頂 52、每列 +48、32 像素高）與那一欄
//（左 424–504、右 536–616）裡，而且顏色偶數項黃、奇數項洋紅。
func TestCommandMenuLayout(t *testing.T) {
	face := testFace(t)
	saved := i18n.Current
	defer func() { i18n.Current = saved }()
	for _, l := range []i18n.Locale{i18n.ZhHant, i18n.En, i18n.Ja} {
		i18n.Current = l
		c := NewCanvasPx(assets.ScreenW, assets.ScreenH, face)
		drawArtCommands(c)
		for i := range Commands() {
			x0, y0 := artUpperX, artUpperY+(i%5)*artCmdRowDY
			x1 := x0 + 80
			if i >= 5 {
				x0, x1 = artCmdRightX, artCmdRightX+80
			}
			want := artInkCmdEven
			if i%2 == 1 {
				want = artInkCmdOdd
			}
			nameInk, keyInk := 0, 0
			for y := y0; y < y0+32; y++ {
				for x := x0; x < x1; x++ {
					switch c.Img.RGBAAt(x, y) {
					case want:
						nameInk++
					case artInkCmdKey:
						keyInk++
					}
				}
			}
			if nameInk == 0 {
				t.Errorf("%s 第 %d 項的名稱沒畫在它那一格（%d,%d），或顏色不對", l, i, x0, y0)
			}
			if keyInk == 0 {
				t.Errorf("%s 第 %d 項的編號沒畫在它那一格", l, i)
			}
		}
		// 格子外面（上面板以外）不能有指令表的墨水。
		for y := 0; y < assets.ScreenH; y++ {
			for x := 0; x < assets.ScreenW; x++ {
				if x >= artUpperX && x < 632 && y >= artUpperY && y < 292 {
					continue
				}
				if px := c.Img.RGBAAt(x, y); px == artInkCmdEven || px == artInkCmdOdd {
					t.Fatalf("%s 指令表畫出上面板了：(%d,%d)", l, x, y)
				}
			}
		}
	}
}

// TestPagesFitTheOverlayInEveryLanguage 釘住分頁在三個語系下都不超出
// 內容區（`docs/spec/014` §3.3）。
//
// 超出的部分會被 `cells.Truncate` 截掉而不報錯——玩家看到的是一欄數字
// 少了最後幾位。判準是劇本 001 開局的**每一個**郡、勢力與將軍都畫一次，
// 不挑樣本：最長的那一列才會撞到邊。
func TestPagesFitTheOverlayInEveryLanguage(t *testing.T) {
	_, g := artSessionFixture(t)
	saved := i18n.Current
	defer func() { i18n.Current = saved }()
	cols := (artPageX1-artPageX0)/CellW - 2
	for _, l := range []i18n.Locale{i18n.ZhHant, i18n.En, i18n.Ja} {
		i18n.Current = l
		worst := map[string]int{}
		check := func(kind, title string, lines []string) {
			for _, line := range append([]string{title}, lines...) {
				if w := cells.Width(line); w > worst[kind] {
					worst[kind] = w
				}
				if w := cells.Width(line); w > cols {
					t.Errorf("%s 的%s有一列 %d 格，內容區只有 %d 格：%q", l, kind, w, cols, line)
					return
				}
			}
		}
		for _, p := range g.Prefectures() {
			if p.ID == 0 {
				continue
			}
			title, lines := GeneralList(g, p.ID)
			check("將軍列表", title, lines)
		}
		for _, f := range g.Factions() {
			title, lines := TerritoryList(g, f.ID)
			check("領土列表", title, lines)
			title, lines = TreasuryList(g, f.ID)
			check("君主物品", title, lines)
		}
		for i := 0; g.General(i) != nil; i++ {
			title, lines := GeneralPage(g, i)
			check("檢視將軍", title, lines)
		}
		t.Logf("%s 各種分頁最寬的一列：%v（內容區 %d 格）", l, worst, cols)
	}
}
