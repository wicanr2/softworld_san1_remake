//go:build oracle

package parity

import (
	"fmt"
	"image"
	"image/png"
	"os"
	"path/filepath"
	"sort"
	"testing"

	"github.com/wicanr2/dosgolem/oracle"

	"github.com/wicanr2/softworld_san1_remake/internal/assets"
	"github.com/wicanr2/softworld_san1_remake/internal/state"
)

// 觀測原版走一個月做了什麼。
//
// 盤面在記憶體裡的位置已知（`tables_oracle_test.go`），所以**每送一次鍵
// 就把三張表讀下來，逐欄位比**，就看得到它改了什麼——不必反組譯。
//
// 這一條是探索用的：它不斷言原版怎麼決定，只把「哪些欄位會動、什麼時候動」
// 記錄下來。斷言要等看得懂那些變化再寫。

// 畫面尺寸**跟著 `assets` 走，不在這裡寫死**。兩邊各記一份的時候，
// 改了一邊、另一邊照樣跑，而症狀是「基準畫面比 remake 矮一截」——
// 看起來像 remake 多畫了東西，不像兩個常數分家（`docs/spec/006`）。
const scrW, scrH = assets.ScreenW, assets.ScreenH

// dumpScreen 把畫面存成 PNG。**每一步都存**：看得到停在哪個提示，
// 就不必猜下一個鍵要送什麼。
func dumpScreen(t *testing.T, o *oracle.Oracle, name string) {
	t.Helper()
	pix := o.IndexedEGASize(scrW, scrH)
	if len(pix) < scrW*scrH {
		t.Logf("畫面只有 %d 個像素，跳過存圖", len(pix))
		return
	}
	img := image.NewRGBA(image.Rect(0, 0, scrW, scrH))
	for y := 0; y < scrH; y++ {
		for x := 0; x < scrW; x++ {
			// **用 EGA 的十六色，不要用 DAC 的色盤**：EGA 的顏色在
			// 屬性控制器不在 VGA 的 DAC，`Palette()` 在這個模式下是全黑
			// ——存出來的圖會整張黑，看起來像畫面沒東西而不像色盤取錯。
			img.Set(x, y, assets.EGAPalette[pix[y*scrW+x]&15])
		}
	}
	dir := os.Getenv("SAN1_SHOTS")
	if dir == "" {
		return
	}
	_ = os.MkdirAll(dir, 0o755)
	f, err := os.Create(filepath.Join(dir, name+".png"))
	if err != nil {
		t.Log(err)
		return
	}
	defer f.Close()
	if err := png.Encode(f, img); err != nil {
		t.Log(err)
	}
}

// bootToMain 把原版開到遊戲主畫面，回傳三張表的基底與長度。
//
// 走的是「載入舊進度」那條（`docs/re/02` §3.3）：開新遊戲會問防拷密碼，
// 而密碼表在說明書掃描裡判讀不出來。
func bootToMain(t *testing.T, o *oracle.Oracle, mas []byte) uint32 {
	t.Helper()
	o.TypeBoth("122")
	send := map[int]string{4: "\r", 17: "2", 23: "1"}
	var base uint32
	for i := 0; i < 27; i++ {
		if err := o.Run(50_000_000); err != nil {
			t.Fatalf("停止：%v", err)
		}
		if k, ok := send[i]; ok {
			o.TypeBoth(k)
		}
		if base == 0 {
			if h := o.Search(mas[:48]); len(h) == 1 {
				base = h[0]
			}
		}
	}
	if base == 0 {
		// 走「載入舊進度」時盤面的內容與劇本檔不同，搜不到——那不代表
		// 沒載入。位址是量出來的固定值，退回去用它。
		base = 0x399b0
	}

	// **這個階段的提示讀掃描碼**，而且字元佇列的殘留要先清掉
	//（`docs/re/02` §3）。送完等畫面動，動了才走下一步。
	// 難度可以換：`SAN1_DIFFICULTY`，預設 5。**換難度是個實驗手段**——
	// 有些數值看起來像常數，其實是難度選出來的。
	diff := "5"
	if v := os.Getenv("SAN1_DIFFICULTY"); v != "" {
		diff = v
	}
	for i, keys := range []string{"1\r", "\r", diff + "\r", "\r", "\r"} {
		if err := o.Run(150_000_000); err != nil {
			t.Fatalf("沉澱時停止：%v", err)
		}
		before := screenOf(o)
		o.Drain()
		o.PressScan(keys)
		moved := false
		for k := 0; k < 12; k++ {
			if err := o.Run(50_000_000); err != nil {
				t.Fatalf("停止：%v", err)
			}
			if pixelDiff(before, screenOf(o), nil) > 200 {
				moved = true
				break
			}
		}
		if !moved {
			dumpScreen(t, o, fmt.Sprintf("卡在第%d步", i+1))
			t.Fatalf("載入序列第 %d 步（送 %q）畫面沒動", i+1, keys)
		}
	}
	return base
}

// bootToGame 一路開到**過了防拷密碼**的主畫面。
//
// `bootToMain` 只走到主畫面，那時密碼關還沒跳出來——第一道帶 ＊ 的
// 指令用掉之後才會（`docs/re/02` §3.3）。所以要真的讓月份走下去，
// 得先把那一關過掉。
//
// 代價是**開頭那一個月已經走過去了**：觸發密碼關的那道「內政 → 休息」
// 就是玩家那一個月的指令。回傳時遊戲停在下一個月的主畫面。
func bootToGame(t *testing.T, o *oracle.Oracle, mas []byte) uint32 {
	t.Helper()
	base := bootToMain(t, o, mas)
	const settle = 40_000_000
	for _, k := range []string{"4\r", "4\r", "Y"} {
		o.Drain()
		o.PressScan(k)
		if err := o.Run(settle * 3); err != nil {
			t.Fatalf("觸發密碼關時停止：%v", err)
		}
	}
	// 密碼那個提示**不吃掃描碼**（實測畫面差 240，與什麼都不送同級），
	// 所以兩條路一起餵。
	o.Drain()
	o.TypeBoth(passwordAnswer + "\r")
	if err := o.Run(settle); err != nil {
		t.Fatalf("作答時停止：%v", err)
	}
	o.TypeBoth("Y\r")
	// ⚠ **這一步的停點是「預算用完」，不是判準**（`CONTEXT.md` R49）。
	//
	// 原版在主畫面不是靜止的：沒有玩家輸入時它會**自動跑完電腦的郡**，
	// 一路跑到玩家的郡才停下來等。所以「停在哪」由預算決定——2026-09-10
	// 換 base 到 dosgolem `main` 之後，同樣的 1.2 億道指令讓原版多跑了
	// 29 格，順序表（每月洗牌的結果）因此整份不同，月度對拍從差 0 個
	// 位元組變成差 339。
	//
	// **新的停點比較正確**：那是原版真的在等輸入的地方，舊的是半路
	// （游標 0 ＝ 一格都還沒跑，而原版本來會自己跑完電腦那些）。
	// 換掉這一段要連同「對拍視窗怎麼取」一起重想
	//（worklist `boot-recipe-behavior-triggered`），不是單獨改預算。
	if err := o.Run(settle * 3); err != nil {
		t.Fatalf("確認時停止：%v", err)
	}
	t.Logf("bootToGame 停在游標 %d", monthCursor(o))
	return base
}

// driveToMonthStart 把原版驅動到**月底結算的入口**，並在那一刻把盤面取走。
//
// 名字說的是目的地：結算之後緊接著就是開月，所以站在結算入口＝站在
// 「這個月的最後一步、下個月還沒開始」。對拍的視窗要跨過這個交界。
//
// 為什麼要這一步：`bootToGame` 停在**玩家的郡**（原版自動跑完電腦那些，
// 輪到玩家才停等輸入）。從那裡取對拍視窗，視窗只剩「玩家後面那幾格」
// ——2026-09-10 換 base 之後是 14 格，而月初出發有 37 格以上
//（`CONTEXT.md` R49）。視窗小不只是樣本少：起點落在月中，開月那一整段
//（物價、洗牌、四季、進貢）根本比不到，而那是 440 → 0 那條路上佔比最大的
// 一段。
//
// 停的地方**由原版自己的路標決定，不是指令預算**：
//
//	0x1581c  月底結算的入口（`docs/re/06` §9.5）
//	0x1746e  郡回合的入口
//
// ⚠ 三個踩過的坑：
//
//  1. **游標 0 是「經過」不是「停下來」。** 原版不會在月初停——它會一路跑到
//     玩家的郡。用「跑到游標歸零」當條件會跑滿預算（實測 2.4 億道指令
//     游標還是 28）。要用 hook 在那一刻把狀態取走，不是把執行停在那裡。
//  2. **玩家可能有好幾個郡。** 送一組鍵只過得了一個，原版接著停在下一個
//     玩家的郡等輸入。所以要**反覆送到月底結算真的發生**，
//     不是送一次就跑。
//  3. `oracle.At()` 這裡用不上：它比的是執行期 CS:IP 的線性位址，而
//     `0x1581c` 這種是**映像位移**（`docs/re/03`）。`OnCall` 走的是同一套
//     位址而測試一直用得好，所以攔截點沿用它。
func driveToMonthStart(t *testing.T, o *oracle.Oracle, seq []string,
	base uint32, total int, seed uint32) []byte {
	t.Helper()
	const settle = 40_000_000
	var at *oracle.State
	// **停在月底結算的入口，不是結算之後。**
	//
	// 對拍的視窗要**跨過月底**：月 N 的尾巴 → 月底結算 ＋ 冬季事件（含進貢）
	// → 開月洗牌 → 月 N+1 的郡回合。測試就是拿「月底結算那一刻的亂數狀態」
	// 把兩邊接上的，停在結算之後等於把對齊點跳過去了——症狀是
	// 「沒攔到月底結算——亂數對不起來」。
	o.OnCall(addr(0x1581c), func(o *oracle.Oracle) {
		if at == nil {
			at = o.Save()
		}
	})
	for i := 0; i < 40 && at == nil; i++ {
		o.Drain()
		for _, keys := range seq {
			o.PressScan(keys)
			if err := o.Run(settle); err != nil {
				t.Fatalf("驅動到月初（第 %d 輪送鍵）：%v", i+1, err)
			}
			if at != nil {
				break
			}
		}
		if at != nil {
			break
		}
		// 沒停在玩家的郡的話就是還在跑電腦那些，續跑一段。
		if err := o.Run(settle); err != nil {
			t.Fatalf("驅動到月初（第 %d 輪續跑）：%v", i+1, err)
		}
	}
	if at == nil {
		t.Fatalf("送了 40 輪還沒走到月底結算（游標 %d）——配方要重看",
			monthCursor(o))
	}
	// **回到月底結算那一刻。** hook 只能「看」不能「停」：拍完之後那一次
	// `o.Run` 會繼續跑到預算用完，原版早就走過好幾個郡了（實測跑到第 9 格，
	// 視窗因此少掉前 10 格）。拍的是**整台機器**（`o.Save`）而不是三張表，
	// 所以還原之後原版就真的站在結算的入口，後面的流程從那裡重新走。
	o.Restore(at)

	// **固定亂數，讓每月洗牌的結果可重現。**
	//
	// 對拍視窗是 `[游標, 玩家的郡)`，而玩家排到第幾格是**開月洗牌抽出來
	// 的**——舊 base 上是第 37 格、換 base 之後是第 9 格。視窗大小因此
	// 隨機，「差 N 個位元組」在不同輪之間不能比大小，樣本數也跟著浮動。
	//
	// 原版的亂數是 MSC 的 LCG，狀態就在 `DS:0xa3ae`／`0xa3b0` 兩格
	//（`docs/re/03` §1.45）。**這一刻正好在洗牌之前**：月底結算本身不抽
	// （實測 0 次），結算之後才是開月洗牌。所以在這裡寫進去，洗牌的結果
	// 就固定了。
	//
	// remake 那一邊不必另外設：測試拿「結算那一刻的狀態」餵 `SeedRand`，
	// 而那個狀態就是這裡寫進去的值——**兩邊自動同源**。
	if seed != 0 {
		ds := uint32(o.DSReg()) * 16
		o.SetWord(addr(ds+0xa3ae), uint16(seed))
		o.SetWord(addr(ds+0xa3b0), uint16(seed>>16))
		t.Logf("亂數固定成 0x%08x（洗牌之前）", seed)
	}
	t.Logf("驅動到月底結算：游標 %d", monthCursor(o))
	return o.Bytes(addr(base), total)
}

// monthCursor 讀「這個月處理到第幾格」（`es:[0x20f4]`，`docs/re/08` §2）。
//
// 0 ＝ 開月了、一格都還沒跑。它是**存檔欄位**，所以跨執行器可比——
// 診斷「原版停在哪」時比指令數有意義：指令數會隨執行器改動而變，
// 這一格不會。
func monthCursor(o *oracle.Oracle) int {
	ds := uint32(o.DSReg()) * 16
	return int(int16(o.Word(addr(uint32(o.Word(addr(ds+0xa726)))*16 + 0x20f4))))
}

func screenOf(o *oracle.Oracle) []uint8 {
	return append([]uint8(nil), o.IndexedEGASize(scrW, scrH)...)
}

// pixelDiff 數兩張畫面差幾個像素，`mask` 為真的位置不算。
func pixelDiff(a, b []uint8, mask []bool) int {
	n := 0
	for i := range a {
		if i >= len(b) {
			break
		}
		if mask != nil && i < len(mask) && mask[i] {
			continue
		}
		if a[i] != b[i] {
			n++
		}
	}
	return n
}

// blinkMask 找出「什麼都不按也會變」的像素。
//
// **游標閃爍會蓋掉一個字的回顯。** 主畫面上輸入一個數字只改動兩百多個
// 像素，與游標閃爍同一個量級——不扣掉雜訊的話，「按了有反應」與
// 「按了沒反應」量出來的數字一樣，於是每一種送法都會被判成沒反應。
//
// 執行器是決定性的：從同一個快照跑同樣的指令數，游標的相位一模一樣，
// 所以扣掉這一份之後對照組的差應該是零。
func blinkMask(o *oracle.Oracle, snap *oracle.State, steps uint64, rounds int) ([]uint8, []bool) {
	o.Restore(snap)
	base := screenOf(o)
	mask := make([]bool, len(base))
	for r := 0; r < rounds; r++ {
		o.Restore(snap)
		_ = o.Run(steps * uint64(r+1))
		cur := screenOf(o)
		for i := range base {
			if i < len(cur) && base[i] != cur[i] {
				mask[i] = true
			}
		}
	}
	o.Restore(snap)
	return base, mask
}

func TestZZWatchTurn(t *testing.T) {
	root := origRoot(t)
	c := openContainer(t, filepath.Join(root, "DATA2"))
	sc, err := state.LoadScenario(c, state.Slot("001"))
	if err != nil {
		t.Fatal(err)
	}
	mas, sta, gen := sc.Tables()
	total := len(mas) + len(sta) + len(gen)

	o, err := oracle.Load(filepath.Join(root, "AA.EXE"), root)
	if err != nil {
		t.Fatal(err)
	}
	defer o.Close()

	base := bootToMain(t, o, mas)
	dumpScreen(t, o, "10-主畫面")

	polls, pressed := o.MouseActivity()
	t.Logf("到主畫面為止：滑鼠被輪詢 %d 次（其中 %d 次有按鍵）、讀過 %d 個鍵、等鍵盤 %d 次",
		polls, pressed, len(o.KeyReads()), o.KeyWaits())

	// **一個數字要跟著 Enter 才算輸入完。** 說明書 p.5／p.17：
	// 「下令用數字鍵 0–9」「先鍵入數字再按 Enter」「不輸入數字直接按
	// Enter 結束該指令」。先前只送裸數字，回顯的那一個字被游標閃爍的
	// 雜訊蓋過去，量起來就是「沒反應」。
	snap := o.Save()
	const settle = 50_000_000
	beforeScr, mask := blinkMask(o, snap, settle, 3)
	masked := 0
	for _, b := range mask {
		if b {
			masked++
		}
	}
	t.Logf("游標雜訊：%d 個像素會自己變（總共 %d）", masked, len(mask))

	beforeTab := o.Bytes(addr(base), total)
	for _, k := range []struct{ name, keys string }{
		{"對照組（什麼都不送）", ""},
		{"Enter", "\r"},
		{"9 ＋ Enter", "9\r"},
	} {
		o.Restore(snap)
		o.Drain()
		if k.keys != "" {
			o.PressScan(k.keys)
		}
		if err := o.Run(settle * 3); err != nil {
			t.Logf("%s：停止 %v", k.name, err)
			continue
		}
		after := screenOf(o)
		tab := o.Bytes(addr(base), total)
		t.Logf("%-22s → 扣掉雜訊還差 %5d 像素、盤面 %4d 位元組%s",
			k.name, pixelDiff(beforeScr, after, mask), differs8(beforeTab, tab),
			where(beforeTab, tab, len(mas), len(sta)))
		dumpScreen(t, o, "11-"+k.name)
	}
	o.Restore(snap)
}

// 已知的欄位名（`internal/state` 的解碼器，`docs/formats/03`）。
//
// **位移印成名字才讀得下去。** 一輪走完會列出幾十筆變化，
// 「29(×43)」與「物價(×43)」的差別是要不要回去翻文件。
var (
	prefField = map[int]string{
		0: "郡名", 1: "郡名", 2: "郡名", 3: "郡名",
		14: "人口", 15: "人口", 16: "兵士", 17: "兵士",
		18: "金", 19: "金", 20: "米", 21: "米",
		22: "在職將", 23: "在野將", 26: "民忠", 27: "地力",
		28: "水利", 29: "物價", 30: "所屬",
	}
	genField = map[int]string{
		7: "年齡", 8: "體力", 9: "智", 10: "武", 11: "魅",
		12: "職位", 13: "出身郡", 16: "忠誠", 17: "身分", 18: "勢力",
		19: "所在", 21: "兵種", 22: "兵力", 23: "兵力",
		24: "訓練", 25: "武裝",
	}
	masField = map[int]string{2: "君主"}
)

// fieldName 把位移換成名字；還沒解出來的就印位移。
func fieldName(m map[int]string, off int) string {
	if n, ok := m[off]; ok {
		return n
	}
	return fmt.Sprintf("+%d", off)
}

// where 說變化落在哪一張表、哪些欄位。
func where(a, b []byte, nmas, nsta int) string {
	tabs := []struct {
		name  string
		lo    int
		n     int
		rec   int
		field map[int]string
	}{
		{"諸侯", 0, nmas, state.MasterRecordSize, masField},
		{"州郡", nmas, nsta, state.PrefectureRecordSize, prefField},
		{"人物", nmas + nsta, len(a) - nmas - nsta, state.GeneralRecordSize, genField},
	}
	out := ""
	for _, x := range tabs {
		offs := map[int]int{}
		recs := map[int]bool{}
		for i := 0; i < x.n && x.lo+i < len(b); i++ {
			if a[x.lo+i] != b[x.lo+i] {
				offs[i%x.rec]++
				recs[i/x.rec] = true
			}
		}
		if len(offs) == 0 {
			continue
		}
		keys := make([]int, 0, len(offs))
		for k := range offs {
			keys = append(keys, k)
		}
		sort.Ints(keys)
		// 同一個欄位的高低位元組要併成一筆，否則一個 16 位元的值會
		// 看起來像兩個欄位在動。
		seen := map[string]int{}
		order := []string{}
		for _, k := range keys {
			n := fieldName(x.field, k)
			if _, ok := seen[n]; !ok {
				order = append(order, n)
			}
			seen[n] += offs[k]
		}
		out += "\n    " + x.name + "："
		for _, n := range order {
			out += fmt.Sprintf(" %s(×%d)", n, seen[n])
		}
		out += fmt.Sprintf("　%d 筆", len(recs))
	}
	return out
}

func differs8(a, b []byte) int {
	n := 0
	for i := range a {
		if a[i] != b[i] {
			n++
		}
	}
	return n
}
