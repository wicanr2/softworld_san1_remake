package ui

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"unicode"

	"github.com/wicanr2/softworld_san1_remake/internal/ai"
	"github.com/wicanr2/softworld_san1_remake/internal/assets"
	"github.com/wicanr2/softworld_san1_remake/internal/battle"
	"github.com/wicanr2/softworld_san1_remake/internal/cells"
	"github.com/wicanr2/softworld_san1_remake/internal/game"
	"github.com/wicanr2/softworld_san1_remake/internal/i18n"
	"github.com/wicanr2/softworld_san1_remake/internal/session"
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
		got := strings.Join(SubMenuLines(k, items, w, artLowerRows), "\n")
		if got != orig[k] {
			t.Errorf("第 %c 類：\n  排出來 %q\n  原版是 %q", k, got, orig[k])
		}
	}
	// 「其他」多兩項（remake 加的），前兩行要與原版逐字相同。
	_, items := SubMenu('9')
	lines := SubMenuLines('9', items, w, artLowerRows)
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

// TestSubMenusShowInFullInEveryLanguage 釘住三個語系、九類子選單都畫在
// **原版的下面板**：中文與日文用原尺寸，英文放不下就用小字（6×10）。
//
// 使用者裁定（2026-09-11）：英文「文字允許縮小」，不縮短名稱、也不改畫到
// 別的面板。所以判準是「每一類都在下面板」，而且**中文一律原尺寸**
//（與原版相同）。英文常比中文長（`CLAUDE.md` §3.3），塞不下的時候不會
// 報錯——多出來的行被截掉，玩家看到的就是少了幾個選項。
func TestSubMenusShowInFullInEveryLanguage(t *testing.T) {
	c := testCanvasPx(t, assets.ScreenW, assets.ScreenH)
	saved := i18n.Current
	defer func() { i18n.Current = saved }()
	for _, l := range []i18n.Locale{i18n.ZhHant, i18n.En, i18n.Ja} {
		i18n.Current = l
		var smallOnes []string
		for k := byte('1'); k <= '9'; k++ {
			_, items := SubMenu(k)
			lay := subMenuLayout(c, k, items)
			if !lay.ok {
				t.Errorf("%s 第 %c 類連小字都排不進下面板", l, k)
				continue
			}
			if lay.small {
				if l != i18n.En {
					t.Errorf("%s 第 %c 類用了小字——只有英文放不下時才用", l, k)
				}
				smallOnes = append(smallOnes, string(k))
			}
		}
		if len(smallOnes) > 0 {
			t.Logf("%s 用小字的子選單：%v", l, smallOnes)
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

// TestArtBattleDrawsOptionsAndPages 釘住戰場上的選項與查看部隊那一頁
// 都畫得出來（`docs/spec/014` §7）。
//
// 先前指令面板只畫三行指令與選單標題：選了「用計」之後六種計謀看不到，
// 「查看」的那一頁也看不到——打開與沒打開差 0 個位元組，與主畫面同一個洞。
func TestArtBattleDrawsOptionsAndPages(t *testing.T) {
	c1, c3 := artContainers(t)
	ab, err := NewArtBattle(c1, c3)
	if err != nil {
		t.Fatal(err)
	}
	fld := battle.Generate(battle.Params{Prefecture: 25})
	b := battle.New(battle.Setup{Field: fld, Seed: 1})
	face := testFace(t)
	draw := func(v BattleView) *Canvas {
		c := NewCanvasPx(assets.ScreenW, assets.ScreenH, face)
		DrawArtBattle(c, ab, b, v, ArtBattleInfo{Portrait: [2]int{-1, -1}})
		return c
	}
	base := draw(BattleView{Menu: i18n.S("page.command"), Items: BattleCommandLines()})
	px, py := assets.BattlePanelX[2], assets.BattlePanelY
	for _, c := range []struct {
		name           string
		v              BattleView
		x0, y0, x1, y1 int
	}{
		{"用計的六種計謀", BattleView{Menu: CommandName(battle.CmdPlot), Items: BattleStratagemLines()},
			px, py, px + assets.BattlePanelW, py + assets.BattlePanelH},
		{"交戰方式", BattleView{Menu: CommandName(battle.CmdEngage), Items: BattleEngageLines()},
			px, py, px + assets.BattlePanelW, py + assets.BattlePanelH},
		{"查看部隊", BattleView{Menu: i18n.S("page.command"), Items: BattleCommandLines(),
			PageTitle: "查看", Page: []string{"甲", "乙"}},
			battlePageX0, battlePageY0, battlePageX1, battlePageY1},
	} {
		got := draw(c.v)
		n, outside := 0, 0
		for i := 0; i < len(base.Img.Pix); i += 4 {
			if string(base.Img.Pix[i:i+4]) == string(got.Img.Pix[i:i+4]) {
				continue
			}
			n++
			x, y := (i/4)%assets.ScreenW, (i/4)/assets.ScreenW
			if x < c.x0 || x >= c.x1 || y < c.y0 || y >= c.y1 {
				outside++
			}
		}
		if n == 0 {
			t.Errorf("%s：畫出來與只有指令面板一模一樣——玩家看不到它", c.name)
		} else if outside > 0 {
			t.Errorf("%s：%d 個像素變了，其中 %d 個落在該畫的那一塊外面", c.name, n, outside)
		}
	}
}

// TestBattleOptionsFitThePanelInEveryLanguage 釘住戰場上的每一組選項：
// 中文原尺寸三行以內（與原版相同）；英文原尺寸放不下就用小字、**留在面板
// 裡**（使用者裁定「文字允許縮小」）；日文的長計謀名小字級沒有字，才改畫
// 在面板上方的選單框。
//
// 放不下的部分會被截掉而不報錯：玩家看到「3.Tr」卻不知道那是陷阱。
func TestBattleOptionsFitThePanelInEveryLanguage(t *testing.T) {
	c := testCanvasPx(t, assets.ScreenW, assets.ScreenH)
	saved := i18n.Current
	defer func() { i18n.Current = saved }()
	w := (assets.BattlePanelW - 8) / CellW
	for _, l := range []i18n.Locale{i18n.ZhHant, i18n.En, i18n.Ja} {
		i18n.Current = l
		for name, lines := range map[string][]string{
			"指令":   BattleCommandLines(),
			"交戰方式": BattleEngageLines(),
			"計謀":   BattleStratagemLines(),
			"紮營":   {i18n.S("bat.arrowKeys"), i18n.S("bat.place"), i18n.S("bat.autoAll")},
		} {
			for _, s := range lines {
				if cells.Width(strings.TrimRight(s, " ")) > w {
					t.Errorf("%s 的%s有一行 %d 格，面板只有 %d 格：%q",
						l, name, cells.Width(strings.TrimRight(s, " ")), w, s)
				}
			}
			if len(lines) <= battleOptRows {
				continue
			}
			if l == i18n.ZhHant {
				t.Errorf("中文的%s有 %d 行，原版是三行以內：%q", name, len(lines), lines)
			}
			small := battleSmallLayout(c, lines, i18n.S("page.command"), "Att Vanguard (Guan Yu)  Moves 5")
			switch {
			case small != nil:
				t.Logf("%s 的%s原尺寸 %d 行，改用小字留在面板裡（%d 行）", l, name, len(lines), len(small))
			case l == i18n.En:
				t.Errorf("英文的%s連小字都排不進面板：%q", name, lines)
			default:
				t.Logf("%s 的%s排成 %d 行，改畫在面板上方", l, name, len(lines))
			}
		}
	}
}

// TestArtBattleTallOptionsGoAboveThePanel 釘住小字級也排不下的選項（日文的
// 計謀四行、有漢字）畫在指令面板正上方的選單框裡，而且不越過那一欄。
func TestArtBattleTallOptionsGoAboveThePanel(t *testing.T) {
	c1, c3 := artContainers(t)
	ab, err := NewArtBattle(c1, c3)
	if err != nil {
		t.Fatal(err)
	}
	saved := i18n.Current
	defer func() { i18n.Current = saved }()
	i18n.Current = i18n.Ja
	fld := battle.Generate(battle.Params{Prefecture: 25})
	b := battle.New(battle.Setup{Field: fld, Seed: 1})
	c := testCanvasPx(t, assets.ScreenW, assets.ScreenH)
	lines := BattleStratagemLines()
	if len(lines) <= battleOptRows {
		t.Fatalf("日文的計謀只有 %d 行——這支測試的前提（排不進三行）不成立了", len(lines))
	}
	DrawArtBattle(c, ab, b, BattleView{Menu: CommandName(battle.CmdPlot), Items: lines},
		ArtBattleInfo{Portrait: [2]int{-1, -1}})
	x0, x1 := assets.BattlePanelX[2], assets.BattlePanelX[2]+assets.BattlePanelW
	y1 := assets.BattlePanelY - 2
	y0 := y1 - len(lines)*CellH - 8
	bg, ink := 0, 0
	ord := assets.EGAPalette[assets.BattleOrderInk]
	for y := y0; y < y1; y++ {
		for x := x0; x < x1; x++ {
			switch c.Img.RGBAAt(x, y) {
			case artInkPageBG:
				bg++
			case ord:
				ink++
			}
		}
	}
	if bg == 0 || ink == 0 {
		t.Fatalf("面板上方的選單框沒畫出來（底色 %d 點、字 %d 點）", bg, ink)
	}
	// 框的左右兩邊外面不能有選單的底色。
	for y := y0; y < y1; y++ {
		for _, x := range []int{x0 - 1, x1} {
			if c.Img.RGBAAt(x, y) == artInkPageBG {
				t.Fatalf("選單框越過那一欄了：(%d,%d)", x, y)
			}
		}
	}
}

// TestArtBattleEnglishStaysInThePanel 釘住英文的九個指令用小字畫在指令
// 面板**裡面**：面板上方沒有選單框，面板裡有小字的墨水。
func TestArtBattleEnglishStaysInThePanel(t *testing.T) {
	c1, c3 := artContainers(t)
	ab, err := NewArtBattle(c1, c3)
	if err != nil {
		t.Fatal(err)
	}
	saved := i18n.Current
	defer func() { i18n.Current = saved }()
	i18n.Current = i18n.En
	fld := battle.Generate(battle.Params{Prefecture: 25})
	b := battle.New(battle.Setup{Field: fld, Seed: 1})
	c := testCanvasPx(t, assets.ScreenW, assets.ScreenH)
	DrawArtBattle(c, ab, b, BattleView{Menu: i18n.S("page.command"), Items: BattleCommandLines()},
		ArtBattleInfo{Portrait: [2]int{-1, -1}})
	x0, x1 := assets.BattlePanelX[2], assets.BattlePanelX[2]+assets.BattlePanelW
	ord := assets.EGAPalette[assets.BattleOrderInk]
	above, inside := 0, 0
	for y := 0; y < assets.ScreenH; y++ {
		for x := x0; x < x1; x++ {
			px := c.Img.RGBAAt(x, y)
			if y < assets.BattlePanelY && px == artInkPageBG {
				above++
			}
			if y >= assets.BattlePanelY && y < assets.BattlePanelY+assets.BattlePanelH && px == ord {
				inside++
			}
		}
	}
	if above > 0 {
		t.Errorf("英文的指令還是畫到面板上方的選單框（%d 點）——應該用小字留在面板裡", above)
	}
	if inside == 0 {
		t.Error("指令面板裡沒有字")
	}
}

// TestArtStatusFitsEveryLanguage 釘住郡的資料面板在三個語系下每一格都
// 塞得進原版的槽位（`docs/spec/014` §6）。
//
// 槽位是原版版面決定的：左欄標籤 ＋ 靠右的數值共 12 格、右欄 10 格、
// 州名 6 格。英文的「Land Value 100」十四格就壓到右欄——不會報錯，
// 只會兩個數字疊在一起。判準是劇本 001 開局**每一個有主的郡**都畫一次。
func TestArtStatusFitsEveryLanguage(t *testing.T) {
	tf := i18n.Sf
	_, g := artSessionFixture(t)
	saved := i18n.Current
	defer func() { i18n.Current = saved }()
	rightCols := artLordCols
	for _, l := range []i18n.Locale{i18n.ZhHant, i18n.En, i18n.Ja} {
		i18n.Current = l
		for _, pv := range g.Prefectures() {
			if !pv.Owned() {
				continue
			}
			p := g.Prefecture(pv.ID)
			for _, f := range artStatusFields(g, p) {
				if w := cells.Width(f.label) + 1 + cells.Width(f.value); w > (f.r-f.x)/CellW {
					t.Errorf("%s 郡 %d 的「%s %s」%d 格，槽位只有 %d 格",
						l, p.ID, f.label, f.value, w, (f.r-f.x)/CellW)
				}
			}
			if lord := g.Lord(p.Owner); lord != nil {
				if s := tf("stat.lord", PersonName(lord.Name)); cells.Width(s) > rightCols {
					t.Errorf("%s 郡 %d 的君主欄 %q 超過 %d 格", l, p.ID, s, rightCols)
				}
				if s := tf("stat.fame", 100); cells.Width(s) > rightCols {
					t.Errorf("%s 的人望欄 %q 超過 %d 格", l, s, rightCols)
				}
			}
			name := PlaceName(p.Name)
			prov := PlaceName(state.ProvinceName(int(p.Province)))
			if artAllWide(name) {
				// 中文與日文：郡名雙倍字停在州名前（488）、州名 6 格。
				if w := cells.Width(name) * 2; w > (artProvX-artNameX)/CellW {
					t.Errorf("%s 郡 %d 的郡名 %q 雙倍字 %d 格，槽位只有 %d 格",
						l, p.ID, name, w, (artProvX-artNameX)/CellW)
				}
				if s := artProvince(int(p.Province)); cells.Width(s) > artProvCols {
					t.Errorf("%s 郡 %d 的州名 %q 超過 %d 格", l, p.ID, s, artProvCols)
				}
			} else {
				if cells.Width(name) > artNameCols {
					t.Errorf("%s 郡 %d 的郡名 %q 超過 %d 格", l, p.ID, name, artNameCols)
				}
				if s := artProvinceIn(int(p.Province), artProvWideCols); cells.Width(s) > artProvWideCols ||
					strings.HasSuffix(s, "zh") {
					t.Errorf("%s 郡 %d 的州名 %q 放不下 %d 格（被截了）", l, p.ID, s, artProvWideCols)
				}
				if prov == state.ProvinceName(int(p.Province)) {
					t.Errorf("%s 郡 %d 的州名 %q 沒有轉寫", l, p.ID, prov)
				}
			}
		}
	}
}

// TestDrawTextRotated 釘住轉 90° 的畫字：一個半形字佔 16 像素寬、8 像素高，
// 由上往下排。
func TestDrawTextRotated(t *testing.T) {
	c := NewCanvasPx(64, 64, testFace(t))
	h := c.DrawTextRotatedPx(8, 4, "HH", artInkName)
	if h != 2*CellW {
		t.Errorf("兩個半形字轉過來高 %d，想要 %d", h, 2*CellW)
	}
	x0, y0, x1, y1 := 99, 99, -1, -1
	for y := 0; y < 64; y++ {
		for x := 0; x < 64; x++ {
			if c.Img.RGBAAt(x, y) == artInkName {
				x0, y0, x1, y1 = min(x0, x), min(y0, y), max(x1, x), max(y1, y)
			}
		}
	}
	if x1 < 0 {
		t.Fatal("什麼都沒畫")
	}
	if x0 < 8 || x1 >= 8+CellH {
		t.Errorf("墨水的 x 在 %d–%d，應該在列高那一條 8–%d 裡", x0, x1, 8+CellH-1)
	}
	if y0 < 4 || y1 >= 4+2*CellW {
		t.Errorf("墨水的 y 在 %d–%d，應該在 4–%d 裡", y0, y1, 4+2*CellW-1)
	}
}

// TestArtDateStaysInTheStrip 釘住左側直條上的年月，三個語系、兩種曆法
// 都留在直條裡（內側的底是 y 356，再下去是下方花邊）。
//
// 英文先前是一個字母一列直排：「Zhongping 6, month 1, Spring」二十八個
// 字元要 448 像素高，直條只有三百像素——下半截畫到花邊上。判準是幾何：
// 直排每字 16 像素、轉 90° 排每個半形字 8 像素，加上起點不能過底。
func TestArtDateStaysInTheStrip(t *testing.T) {
	_, g := artSessionFixture(t)
	saved := i18n.Current
	defer func() { i18n.Current = saved }()
	const stripBottom = 356
	for _, l := range []i18n.Locale{i18n.ZhHant, i18n.En, i18n.Ja} {
		i18n.Current = l
		for _, cal := range []game.Calendar{game.ChineseEra, game.Western} {
			// 最長的月份與年份：十二月、四位數的年。
			date := game.Date{Year: 1999, Month: 12}.FormatWithSeason(cal)
			if cal == game.ChineseEra {
				date = g.Date.FormatWithSeason(cal)
			}
			h := len([]rune(date)) * CellH
			if artHasLatin(date) {
				h = cells.Width(date) * CellW
			}
			if bottom := artDateRow*CellH + h; bottom > stripBottom {
				t.Errorf("%s 的年月 %q 畫到 y %d，直條的底是 %d", l, date, bottom, stripBottom)
			}
		}
	}
}

// TestBattleSidePanelsFitEveryLanguage 釘住戰場左右兩塊軍力面板的五行字：
// 中日文原尺寸放得進文字區（64 像素＝8 格：肖像 80、統帥名 32 之外的
// 部分，`assets.BattlePanelTextX`），英文放不下就整塊改小字、長的一行折
// 兩行，一個字都不截（`docs/spec/014` §7）。數值取上限：兵與金五位數。
// 兩塊面板只畫主攻軍與主守軍（`artBattleSides`）。
func TestBattleSidePanelsFitEveryLanguage(t *testing.T) {
	c := testCanvasPx(t, assets.ScreenW, assets.ScreenH)
	saved := i18n.Current
	defer func() { i18n.Current = saved }()
	for _, l := range []i18n.Locale{i18n.ZhHant, i18n.En, i18n.Ja} {
		i18n.Current = l
		for _, s := range artBattleSides {
			lines := []string{
				i18n.Sf("bat.armyOf", battlePaddedName(i18n.PersonName("諸葛亮"))),
				i18n.Sf("bat.sideLine", SideName(s)),
				i18n.Sf("bat.forcesLine", battleUnitsNumeral(10), 10),
				i18n.Sf("bat.menLine", 99900),
				i18n.Sf("bat.goldLine", 30000),
			}
			width := sideTextW
			if l == i18n.En {
				width = sideTextW + 32 // 沒畫統帥名，那 32 像素讓給資料
			}
			rows, small := sidePanelLayout(c, lines, width)
			if small && l != i18n.En {
				t.Errorf("%s 的軍力面板用了小字——只有英文放不下時才用：%q", l, lines)
			}
			joined := strings.ReplaceAll(strings.Join(rows, ""), " ", "")
			if joined != strings.ReplaceAll(strings.Join(lines, ""), " ", "") {
				t.Errorf("%s 的軍力面板截了字：\n  原本 %q\n  畫的 %q", l, lines, rows)
			}
			if small {
				t.Logf("%s 的軍力面板用小字：%q", l, rows)
			}
		}
	}
}

// TestTextPageCoversAndClears 釘住文字版的分頁：框延伸到畫面右緣、
// 三個語系的領土列表都不必截，而且框裡除了字以外都是底色。
//
// 先前清底色是畫空白字元——空白沒有墨水，等於沒清，地圖從底下透出來；
// 框也只到地圖區的 44 格，領土列表中文就要 60 格，最後幾欄被截掉。
func TestTextPageCoversAndClears(t *testing.T) {
	_, g := artSessionFixture(t)
	face := testFace(t)
	saved := i18n.Current
	defer func() { i18n.Current = saved }()
	for _, l := range []i18n.Locale{i18n.ZhHant, i18n.En, i18n.Ja} {
		i18n.Current = l
		title, lines := TerritoryList(g, g.Player)
		for _, line := range lines {
			if w := cells.Width(line); w > Cols-mapCol-4 {
				t.Errorf("%s 的領土列表有一列 %d 格，文字版的分頁只有 %d 格", l, w, Cols-mapCol-4)
			}
		}
		c := NewCanvas(Cols, Rows, face)
		DrawSession(c, g, nil, View{PageTitle: title, Page: lines[:1]})
		// 第三列以下（只有第一列有字）整片都要是底色。
		for y := 3 * CellH; y < (Rows-1)*CellH; y++ {
			for x := (mapCol + 1) * CellW; x < (Cols-1)*CellW; x++ {
				if px := c.Img.RGBAAt(x, y); px != ColBG {
					t.Fatalf("%s 分頁框裡 (%d,%d) 不是底色——底下的東西透出來了", l, x, y)
				}
			}
		}
	}
}

// TestTextScreenClipsNothing 釘住文字版（沒有原版素材時的畫面）在三個
// 語系、每一種狀態下都沒有字在畫布右緣被截掉（`Canvas.Clipped`）。
//
// 右側的資料欄與指令欄貼著右緣，溢出的字會被 `DrawText` 安靜地丟掉——
// 畫面上只是少了幾個字，看起來像譯文本來就這樣。
func TestTextScreenClipsNothing(t *testing.T) {
	_, g := artSessionFixture(t)
	face := testFace(t)
	saved := i18n.Current
	defer func() { i18n.Current = saved }()
	for _, l := range []i18n.Locale{i18n.ZhHant, i18n.En, i18n.Ja} {
		i18n.Current = l
		views := map[string]View{"主提示": {}}
		for k := byte('1'); k <= '9'; k++ {
			title, items := SubMenu(k)
			views["子選單 "+string(k)] = View{Menu: title, Items: items}
		}
		for _, p := range g.Prefectures() {
			if p.Owned() {
				views["郡 "+p.Name] = View{Sel: p.ID}
			}
		}
		for name, v := range views {
			c := NewCanvas(Cols, Rows, face)
			c.SetSmallFace(testSmallFace(t))
			DrawSession(c, g, nil, v)
			if c.Clipped > 0 {
				t.Errorf("%s 的文字版在「%s」截掉了 %d 格", l, name, c.Clipped)
			}
		}
	}
}

// TestBattleReportsFitEveryLanguage 釘住戰報頁三個語系都在分頁的 68 格內。
//
// 逐日紀錄帶人名與計謀名，英文最容易撞邊。判準是**真的打出來的戰役**：
// 強化 AI 全電腦跑三十六個月，每一場都排一次——挑幾場樣本會漏掉最長的
// 那一行。
func TestBattleReportsFitEveryLanguage(t *testing.T) {
	_, g := artSessionFixture(t)
	s := session.New(g, ai.NewEnhanced(0), state.NoFaction)
	for m := 0; m < 36 && !s.Over; m++ {
		s.EndMonth()
	}
	reports := s.Battles()
	if len(reports) == 0 {
		t.Fatal("三十六個月一場戰役都沒打——量不到戰報")
	}
	saved := i18n.Current
	defer func() { i18n.Current = saved }()
	cols := (artPageX1-artPageX0)/CellW - 2
	for _, l := range []i18n.Locale{i18n.ZhHant, i18n.En, i18n.Ja} {
		i18n.Current = l
		widest := ""
		for _, r := range reports {
			title, lines := BattleReport(g, r)
			for _, line := range append([]string{title}, lines...) {
				if cells.Width(line) > cells.Width(widest) {
					widest = line
				}
				if w := cells.Width(line); w > cols {
					t.Errorf("%s 的戰報有一列 %d 格，分頁只有 %d 格：%q", l, w, cols, line)
				}
			}
		}
		t.Logf("%s：%d 場戰報，最寬一列 %d 格 %q", l, len(reports), cells.Width(widest), widest)
	}
}

// TestTacticalReportsAreTranslatedAndFit 釘住戰術層的逐日戰報有譯文，
// 而且三個語系的長句折完都在分頁的 68 格內、一個字都沒少（`docs/spec/014` §8）。
//
// 先前三十三則紀錄全部寫死中文、部隊名用中文的 `String()` 拼，英日文玩家
// 看到的戰報內文是中文。判準是**真的打出來的戰役**：照
// `battle.TestAutoUsesTheWholeRepertoire` 的配方掃一批（地形、天候、
// 兵力比都換），單挑、計謀、弓箭、退兵、全滅都會出現——挑一兩場樣本
// 會漏掉最長的那一行。將領用真的武將名字，拼音表才轉得到。
func TestTacticalReportsAreTranslatedAndFit(t *testing.T) {
	att := []string{"關羽", "張飛", "趙雲", "馬超", "黃忠", "諸葛亮", "魏延", "姜維"}
	def := []string{"曹操", "夏侯惇", "許褚", "典韋", "張遼", "司馬懿"}
	leaders := func(names []string, war, intel uint8, soldiers, base int) []battle.Leader {
		out := make([]battle.Leader, len(names))
		for i, n := range names {
			out[i] = battle.Leader{Name: n, War: war - uint8(i), Intel: intel, Stamina: 100,
				Charm: 50, Soldiers: soldiers, Training: 50, Arms: 50,
				Troop: battle.TroopLand, Index: base + i}
		}
		return out
	}
	saved := i18n.Current
	defer func() { i18n.Current = saved }()
	cols := (artPageX1-artPageX0)/CellW - 2
	for _, l := range []i18n.Locale{i18n.ZhHant, i18n.En, i18n.Ja} {
		i18n.Current = l
		lines, widest := 0, ""
		for id := 1; id <= 42; id += 5 {
			for _, w := range []battle.Weather{battle.Clear, battle.Windy, battle.Rainy} {
				for _, c := range []struct {
					ratio float64
					war   uint8
				}{{0.8, 90}, {1.4, 90}, {0.1, 99}} {
					p := battle.Params{Prefecture: id,
						Neighbours: []int{(id % 42) + 1, ((id + 7) % 42) + 1}, Forts: id % 6}
					b := battle.New(battle.Setup{
						Field: battle.Generate(p), Weather: w, Seed: uint32(id*31 + int(w)),
						Attackers: leaders(att, c.war, 85, int(3000*c.ratio), 0),
						Defenders: leaders(def, 60, 60, 2500, 100),
						FromGate:  p.Neighbours[0], AttackerGold: 3000, AttackerRice: 8000,
						DefenderGold: 2000, DefenderRice: 6000,
					})
					b.Auto()
					for _, line := range b.Log {
						lines++
						if cells.Width(line) > cells.Width(widest) {
							widest = line
						}
						// 長句折行（`PageLines`）：折完每一行都要在分頁寬度內，
						// 而且**一個字都沒少**——先前截掉的正好是句尾的傷亡數字。
						rows := PageLines([]string{line}, cols)
						joined := ""
						for _, r := range rows {
							if w := cells.Width(r); w > cols {
								t.Errorf("%s 的戰報折完還有一行 %d 格：%q", l, w, r)
							}
							joined += strings.ReplaceAll(r, " ", "")
						}
						if joined != strings.ReplaceAll(line, " ", "") {
							t.Errorf("%s 的戰報折行掉了字：\n  原句 %q\n  折完 %q", l, line, rows)
						}
						if l == i18n.En {
							for _, r := range line {
								if unicode.Is(unicode.Han, r) {
									t.Fatalf("英文戰報裡有漢字 %q：%q", string(r), line)
								}
							}
						}
					}
				}
			}
		}
		if lines == 0 {
			t.Fatalf("%s 打了一批戰役卻一行戰報都沒有", l)
		}
		t.Logf("%s：%d 行戰報，最寬 %d 格 %q", l, lines, cells.Width(widest), widest)
	}
}

// TestPageScrollsToTheEnd 釘住一頁放不下的分頁捲得到最後一行。
//
// 先前沒有捲動：一場三十天的戰報動輒數十行，分頁只放得下十九行，
// 其餘的只顯示「還有更多」，玩家讀不到。
func TestPageScrollsToTheEnd(t *testing.T) {
	var body []string
	for i := 1; i <= 60; i++ {
		body = append(body, fmt.Sprintf("第 %d 行", i))
	}
	for _, art := range []bool{true, false} {
		var v View
		v.SetPage("戰報", body)
		_, rows := PageSize(art)
		v.ScrollPage(1000, art)
		if want := len(body) - rows; v.PageTop != want {
			t.Errorf("art=%v 捲到底停在第 %d 行，想要 %d", art, v.PageTop, want)
		}
		v.ScrollPage(-1000, art)
		if v.PageTop != 0 {
			t.Errorf("art=%v 捲回頂停在第 %d 行", art, v.PageTop)
		}
		// 換一頁要捲回最上面。
		v.ScrollPage(5, art)
		v.SetPage("別頁", body)
		if v.PageTop != 0 {
			t.Errorf("art=%v 換頁之後停在第 %d 行，應該回到最上面", art, v.PageTop)
		}
	}
	// 捲得動時標題帶位置、提示換成怎麼捲；捲到底時畫得出最後一行。
	cols, rows := PageSize(true)
	lines, head, hint, top := pageWindow("戰報", body, 1000, cols, rows)
	if !strings.Contains(head, "／60") || hint != i18n.S("hint.pageScroll") {
		t.Errorf("標題 %q、提示 %q：捲得動時要帶位置與捲動的說明", head, hint)
	}
	if lines[top+rows-1] != "第 60 行" {
		t.Errorf("捲到底時最後一行是 %q", lines[top+rows-1])
	}
}
