//go:build oracle

package parity

import (
	"encoding/binary"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"

	"github.com/wicanr2/dosgolem/oracle"

	"github.com/wicanr2/softworld_san1_remake/internal/state"
)

// 誰寫了這個欄位。
//
// **這是「決策程式碼在哪」最直接的答案。** 靜態的交叉參考只涵蓋直接
// 定址；`mov es:[si+0x2228], al` 這種以結構基底加位移的寫法掃不出來，
// 而三張表全部是這樣存取的（`docs/re/03` §1）。
//
// 做法是讓執行器在寫入落進表的範圍時記下當時的 `CS:IP`，跑一個月，
// 再按 IP 分組。**訓練度是誰改的、金是誰扣的**，答案就是那幾個位址。
//
// ⚠ **位址只到「這一次執行的線性位址」為止。** 現成的
// `workplace/ida/OVL.BIN`（從 `0110:0000` 取的）裡連一次 `28 22` 都沒有
// ——那是人物表訓練度欄的位移，所以含這些程式碼的那一層不在那份 dump
// 裡。要對到映像位移，得從**同一次執行**把碼段取出來（`dumpImage`）。

// dumpImage 把一段線性記憶體寫成檔，給 objdump 用。
//
// 寫進 `workplace/`（gitignore）。那是原版載入後的碼段，與原版執行檔
// 一樣不散布。
func dumpImage(t *testing.T, o *oracle.Oracle, lo, hi uint32, name string) {
	t.Helper()
	dir := os.Getenv("SAN1_DUMP")
	if dir == "" {
		return
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Log(err)
		return
	}
	b := o.Bytes(addr(lo), int(hi-lo))
	path := filepath.Join(dir, fmt.Sprintf("%s-%06x.bin", name, lo))
	if err := os.WriteFile(path, b, 0o644); err != nil {
		t.Log(err)
		return
	}
	t.Logf("碼段 %#x–%#x（%d 個位元組）寫到 %s；objdump 的 --adjust-vma ＝ %#x",
		lo, hi, len(b), path, lo)
}

// TestZZDumpCode 只做一件事：開機到遊戲裡，把碼段寫成檔。
//
// 拆開來是因為**取碼段不必跑一個月**，而跑一個月會讓整條測試超過十分鐘
// ——那個長度在這台機器上常常被記憶體守衛砍掉，砍掉就什麼都沒留下。
func TestZZDumpCode(t *testing.T) {
	root := origRoot(t)
	c := openContainer(t, filepath.Join(root, "DATA2"))
	sc0, err := state.LoadScenario(c, state.Slot("001"))
	if err != nil {
		t.Fatal(err)
	}
	seedMas, _, _ := sc0.Tables()

	o, err := oracle.Load(filepath.Join(root, "AA.EXE"), root)
	if err != nil {
		t.Fatal(err)
	}
	defer o.Close()

	bootToGame(t, o, seedMas)
	// 範圍要蓋到主程式全部的碼段。`0x01f000` 那個上界是最初隨手取的，
	// 而外交（`2c21:1b56` ＝ 線性 `0x02dd66`）與交戰都落在它外面——
	// 讀不到的原因是 dump 太短，不是那段碼不在記憶體裡。
	// 戰場的字串在 `0x46c02` 一帶（`docs/re/04` §4），用它們的碼也在
	// 那個範圍附近，所以上界要拉到字串區的後面。
	dumpImage(t, o, 0x00b000, 0x050000, "code")
	// **低位址那一段也是程式自己的碼。** `lcall` 打到的段裡有
	// `0x0110`、`0x0374`、`0x03eb`、`0x04fb`、`0x05c4`、`0x0fe4`，
	// 解算成線性全在 `0xb000` 以下——原本的窗蓋不到，於是對那一區的
	// 靜態掃描一律回零筆，而零筆與「沒有人用」分不開（`CONTEXT.md` R42）。
	dumpImage(t, o, 0x000400, 0x00b000, "code")
}

// TestZZDumpBattleCode 把原版帶進一場真的打得起來的戰役，然後倒記憶體。
//
// 戰術層本身**不是 overlay**（`docs/re/03` §1.5），開機後的碼段裡就有；
// 這一支要的是**資料段**：地形代價表之類的常數都寫成 `[bx+0x7c42]` 這種
// DS 相對位址，而 DS 是 MSC 的 DGROUP，不在碼段的 dump 裡。所以進到
// 戰鬥裡攔一次，把 DS 記下來，連著整個資料段倒出來。
//
// 盤面**直接寫記憶體擺出來**，不靠存檔剛好是什麼樣子（`stageABattle`）。
// 送的鍵用 `SAN1_BATTLEKEY` 換，`|` 分段，`enterMark` 代表 Enter。
func TestZZDumpBattleCode(t *testing.T) {
	root := origRoot(t)
	c := openContainer(t, filepath.Join(root, "DATA2"))
	sc0, err := state.LoadScenario(c, state.Slot("001"))
	if err != nil {
		t.Fatal(err)
	}
	seedMas, _, _ := sc0.Tables()

	o, err := oracle.Load(filepath.Join(root, "AA.EXE"), root)
	if err != nil {
		t.Fatal(err)
	}
	defer o.Close()

	base := bootToGame(t, o, seedMas)
	at, to := stageABattle(t, o, base)

	// `0x2053c` 是主戰場。攔它進去的那一刻取 DS——資料段的位置只有
	// 執行期知道，而戰術層的常數表全是 DS 相對的。
	var dgroup uint16
	var steps uint64
	o.OnCall(addr(0x2053c), func(o *oracle.Oracle) {
		if dgroup == 0 {
			dgroup, steps = o.DSReg(), o.Steps()
		}
	})
	// `0x22704` 是畫戰場那一支——它跑到了才代表整編走完。
	fielded := 0
	o.OnCall(addr(0x22704), func(o *oracle.Oracle) { fielded++ })

	const settle = 40_000_000
	spell := func(n int) string {
		out := ""
		for _, c := range fmt.Sprintf("%d", n) {
			out += string(c) + "|"
		}
		return out + enterMark
	}
	// **每一個提示都要 Enter，選單也一樣**（`docs/re/03` §1.5 的按鍵表）。
	menu := "2|" + enterMark
	// 整編：每個軍團一輪，「請按任一鍵」→ 分配將軍 → 分到那一軍 →
	// 分配完畢(Y/N)；主守軍不問錢糧，主攻軍要問（`docs/re/05` §6）。
	// 守方郡有兩位守將、攻方郡一位——盤面是 stageABattle 擺的。
	one := func(n int) string {
		return fmt.Sprintf("%d|%s|1|%s", n, enterMark, enterMark)
	}
	org := enterMark + "|" + one(1) + "|N|" + one(2) + "|Y" +
		"|" + enterMark + "|" + one(1) + "|Y|5000|" + enterMark +
		"|9000|" + enterMark + "|Y"
	keys := envOr("SAN1_BATTLEKEY",
		menu+"|"+menu+"|"+spell(at)+"|"+spell(to)+"|"+org)
	for i, seg := range strings.Split(keys, "|") {
		o.Drain()
		o.PressScan(strings.ReplaceAll(seg, enterMark, "\r"))
		if err := o.Run(settle); err != nil {
			t.Fatalf("送第 %d 段（%q）時停止：%v", i+1, seg, err)
		}
		dumpScreen(t, o, fmt.Sprintf("battle-%d", i+1))
	}
	if dgroup == 0 {
		t.Fatal("沒有進到主戰場 0x2053c——按鍵序列或盤面不對")
	}
	if fielded == 0 {
		t.Fatal("戰場沒有畫出來——整編的按鍵序列不對")
	}
	t.Logf("主戰場在第 %d 步進去，DS ＝ %#06x（線性 %#07x）",
		steps, dgroup, uint32(dgroup)<<4)
	// 再跑一段讓戰場畫完，畫面與資料一起留下來。
	if err := o.Run(settle); err != nil {
		t.Fatalf("戰場畫面停止：%v", err)
	}
	dumpScreen(t, o, "battle-field")
	lo := uint32(dgroup) << 4
	dumpImage(t, o, lo, lo+0x10000, "dgroup")
	// 戰場的工作區在**另一個段**：`ds:0xa872` 這一格存著它的段值。
	// 地圖緩衝區 `0x163a`、部隊記錄 `0x3502`、軍團記錄 `0x175e`、
	// 將領戰力值 `0x548` 全在那裡（`docs/re/05`）。
	work := o.Word(oracle.Addr{Seg: dgroup, Off: 0xa872})
	t.Logf("戰場工作區的段 ＝ %#06x（線性 %#07x）", work, uint32(work)<<4)
	wlo := uint32(work) << 4
	dumpImage(t, o, wlo, wlo+0x10000, "work")
}

// enterMark 是環境變數裡代表 Enter 的兩個字元。**不寫真的 CR**：
// 經過 shell 與 `docker -e` 會被吃掉或轉掉。
const enterMark = `\r`

// stageABattle 直接改記憶體，把盤面擺成打得起來，回傳要打的郡。
//
// **不靠原版的 `RND()`，也不靠存檔剛好是什麼樣子**：那份存檔的玩家在
// 南海，兵 500、金 3、現役將 1，戰役指令按下去就退回來。這裡把玩家的
// 守軍與錢糧墊高，再把一個鄰郡放上敵將——郡的歸屬是從人物表導出來的
// （`docs/re/03` §1.5），所以放人就等於換旗。
func stageABattle(t *testing.T, o *oracle.Oracle, base uint32) (int, int) {
	t.Helper()
	nMas, nSta, nGen := state.MasterTableSize, state.PrefectureTableSize, state.GeneralTableSize
	raw := o.Bytes(addr(base), nMas+nSta+nGen)
	mas, sta, gen := raw[:nMas], raw[nMas:nMas+nSta], raw[nMas+nSta:]

	sc, err := state.DecodeTables(state.Slot("001"), mas, sta, gen)
	if err != nil {
		t.Fatal(err)
	}
	players := sc.Players()
	if len(players) == 0 {
		t.Fatal("盤面上沒有玩家")
	}
	me := players[0]
	// **不要拿君主的所在郡當起點**：自創君主指向填充槽，`Location`
	// 是哨兵值（實測 255），拿去索引州郡表會越界。從盤面找他的郡。
	at := 0
	for id := 1; id <= 42; id++ {
		if int(sta[id*176+30]) == me {
			at = id
			break
		}
	}
	if at == 0 {
		t.Fatalf("勢力 %d 在盤面上沒有郡", me)
	}

	// 鄰郡：州郡 offset 45–54，`0xFF` 補齊。
	to := 0
	for k := 45; k <= 54; k++ {
		if n := int(sta[at*176+k]); n != 0xFF && n != 0 {
			to = n
			break
		}
	}
	if to == 0 {
		t.Fatalf("郡 %d 沒有鄰郡", at)
	}
	enemy := -1
	for _, f := range sc.ActiveFactions() {
		if f != me {
			enemy = f
			break
		}
	}
	if enemy < 0 {
		t.Fatal("盤面上只有玩家一個勢力")
	}

	put16 := func(b []byte, i, v int) { b[i], b[i+1] = byte(v), byte(v>>8) }

	// 玩家這一邊：守軍每人三千兵、訓練與武裝八成，郡裡錢糧管夠。
	mine := 0
	for i := 0; i < 350; i++ {
		r := gen[i*30:]
		if int(r[18]) != me || int(r[19]) != at {
			continue
		}
		put16(r, 22, 3000)
		r[24], r[25] = 80, 80
		// **智也要墊高**：計謀的門檻表 `DS:0x7f6e` 最低 60（陷阱、誘敵）、
		// 最高 80（火攻），領隊的智不夠的話那六支一律被擋在第二道門，
		// 實跑就走不到計謀（`docs/re/05` §4）。
		r[9] = 99
		mine++
	}
	put16(sta[at*176:], 16, mine*30)
	put16(sta[at*176:], 18, 9000)
	put16(sta[at*176:], 20, 20000)

	// **「發動戰役」要君主本人在這個郡**（`0x1891f` 檢查州郡 offset 32
	// 指到的人身分是不是 0）。這份存檔的玩家是自創君主，諸侯 offset 2
	// 指向填充槽，所以怎麼按都會被擋回來。把本郡第一位守將改成君主。
	for i := 0; i < 350; i++ {
		r := gen[i*30:]
		if int(r[18]) != me || int(r[19]) != at {
			continue
		}
		r[17] = 0 // 身分 ← 君主
		put16(sta[at*176:], 32, i)
		put16(mas[me*72:], 2, i)
		t.Logf("把人物 %d 設成勢力 %d 的君主，坐鎮郡 %d", i, me, at)
		break
	}

	// 敵方這一邊：挑兩位在野的人放進目標郡，郡就跟著換旗。
	placed := 0
	for i := 0; i < 350 && placed < 2; i++ {
		r := gen[i*30:]
		if r[18] != 0xFF {
			continue
		}
		r[18], r[19], r[17], r[12] = byte(enemy), byte(to), 3, 3
		put16(r, 22, 1500)
		r[24], r[25] = 50, 50
		placed++
	}
	sta[to*176+30] = byte(enemy)
	put16(sta[to*176:], 16, placed*15)
	put16(sta[to*176:], 18, 500)
	put16(sta[to*176:], 20, 3000)
	sta[to*176+22] = byte(placed)

	o.SetBytes(addr(base), raw)
	// **寫完要讀回來**：`base` 找錯或表在別處的話，寫進去什麼事都不會
	// 發生，而畫面看起來完全正常——那是最難發現的那種錯。
	back := o.Bytes(addr(base), nMas+nSta+nGen)
	bs := back[nMas : nMas+nSta]
	get16 := func(b []byte, i int) int { return int(b[i]) | int(b[i+1])<<8 }
	t.Logf("盤面擺好：玩家勢力 %d 在郡 %d（%d 位守將 × 3000 兵），"+
		"目標郡 %d 換成勢力 %d（%d 位 × 1500 兵）", me, at, mine, to, enemy, placed)
	t.Logf("讀回來：郡 %d 金 %d 米 %d 兵士 %d；郡 %d 所屬 %d 兵士 %d 現役將 %d",
		at, get16(bs, at*176+18), get16(bs, at*176+20), get16(bs, at*176+16),
		to, bs[to*176+30], get16(bs, to*176+16), bs[to*176+22])
	return at, to
}

// TestZZBattleKeySweep 從**同一個快照**試多組按鍵，找出打得起來的那一組。
//
// **開機要三分鐘，一組按鍵要十秒**——所以開機一次、存快照，每個候選
// 還原之後再送。同一個問題問到第三次還在等好幾分鐘，就該把迴圈變快
// （`~/.claude/CLAUDE.md` 的長工作紀律）。
//
// 判準是**戰鬥真的被叫起來了**：攔 `0x18bb9`，也就是選完目標郡之後
// 那個 `lcall 2020:0000(來源郡, 目標郡)`。畫面看起來對不對是觀感，
// 那一支被呼叫是事實。
//
// 途中的路標一起攔，卡在哪一段一眼就看得出來：
//
//	0x188ea  發動戰役的入口
//	0x1892a  過了「主事者是君主」那道門
//	0x189c5  選完出兵郡
//	0x18aad  軍師勸諫的分支（`RND(5)+80 < es:[0x16f2]` 才走）
//	0x18b4e  兩條路會合
//	0x18bb9  呼叫戰鬥
func TestZZBattleKeySweep(t *testing.T) {
	root := origRoot(t)
	c := openContainer(t, filepath.Join(root, "DATA2"))
	sc0, err := state.LoadScenario(c, state.Slot("001"))
	if err != nil {
		t.Fatal(err)
	}
	seedMas, _, _ := sc0.Tables()

	o, err := oracle.Load(filepath.Join(root, "AA.EXE"), root)
	if err != nil {
		t.Fatal(err)
	}
	defer o.Close()

	base := bootToGame(t, o, seedMas)
	at, to := stageABattle(t, o, base)
	snap := o.Save()

	marks := []struct {
		at   uint32
		name string
	}{
		{0x188ea, "入口"}, {0x1892a, "君主門"}, {0x189c5, "選完出兵郡"},
		{0x18aad, "軍師勸諫"}, {0x18b4e, "會合"}, {0x18bb9, "呼叫戰鬥"},
		{0x2053c, "主戰場"}, {0x20a75, "整編:任一鍵"}, {0x20d26, "整編:分配"},
		{0x20b37, "整編:金"}, {0x20bf1, "整編:米"}, {0x20c4e, "整編:確認"},
		{0x22704, "畫戰場"}, {0x21816, "戰場迴圈"},
	}
	hit := map[string]int{}
	for _, m := range marks {
		name := m.name
		o.OnCall(addr(m.at), func(o *oracle.Oracle) { hit[name]++ })
	}
	asked := map[uint32]int{}
	o.OnCall(addr(0x34ede), func(o *oracle.Oracle) { asked[o.Caller().Linear()]++ })
	// `0x1d613` 是選郡那支數字輸入回來的那一刻，AX 就是它的回傳值：
	// `0xFFFF` ＝ 取消、`0xFFFE` ＝ 不合法要重問、其餘 ＝ 選中的郡。
	var got []string
	o.OnCall(addr(0x1d613), func(o *oracle.Oracle) {
		got = append(got, fmt.Sprintf("%#06x", o.AX()))
	})
	// **誰在讀鍵**：`1058:0e24`（線性 `0x113a4`）是「讀一個鍵」的共用常式。
	// 攔它並記呼叫端，就知道卡在哪一個提示——不必再猜。
	readers := map[uint32]int{}
	o.OnCall(addr(0x113a4), func(o *oracle.Oracle) { readers[o.Caller().Linear()]++ })

	// `1058:0e24` 也是數字欄位真正讀鍵的那一支，回傳 ASCII。
	// 攔它回來的那一刻（`0x34f91`）就知道哪些鍵進得去。
	var keys []string
	o.OnCall(addr(0x34f91), func(o *oracle.Oracle) {
		k := o.AX() & 0xff
		if k >= 0x20 && k < 0x7f {
			keys = append(keys, fmt.Sprintf("%q", rune(k)))
			return
		}
		keys = append(keys, fmt.Sprintf("%#02x", k))
	})

	// 候選：目標的數字有沒有 Enter、要不要補選將領、幾個 Y。
	E := enterMark
	// **順序是先問「從那一郡移出」再問「攻打那一郡」**（`0x18992` 那個
	// 選郡的互動在建完「自己的郡」清單之後，`0x189e8` 才建可打的目標）。
	// **一段一鍵**：鍵盤是逐鍵中斷送進去的，一段裡塞三個鍵的話
	// `Run(settle)` 的預算用光時輸入常式還沒讀完，它就一直卡在那裡
	// ——量到的現象是「進得去、回不來」（`0x1d613` 從來沒被執行到）。
	spell := func(n int) string {
		out := ""
		for _, c := range fmt.Sprintf("%d", n) {
			out += string(c) + "|"
		}
		return out + E
	}
	src, dst := spell(at), spell(to)
	// `|||` 之間的空段只是多等一輪：**選完子選單之後畫面要重畫**，
	// 太快送出的第一個數字會被吃掉（量到的現象是 `4|1|\r` 回傳 1）。
	// **每一個提示都要 Enter，選單也不例外**：欄寬由上限決定
	// （`0x34eea`），滿了之後多打的數字被直接丟掉，只有 `0x0d` 才收工
	// （`0x35017`）。所以 `2` 沒送 Enter 時，後面的數字全被選單吃掉——
	// 量到的鍵序列 `'2' '2' '4' '1' 0x0d …` 正好是這個形狀。
	menu := "2|" + E
	head := menu + "|" + menu + "|" + src + "|" + dst
	// 進到主戰場之後是**整編**（`0x20a30`）：先「請按任一鍵」，
	// 再分配將軍，再問攜帶多少金、多少米，最後確認。四個軍團依序來，
	// 主守軍（陣營碼 0）不問，錢糧全帶（`0x20ac3`）。
	gold, rice := "5000|"+E, "9000|"+E
	// 分配：「分配那一位將軍(1-%d)」→「將%s分到那一軍(1-5)」→
	// 「分配完畢(Y/N)」。守方郡 25 有兩位、攻方郡 41 有一位。
	one := func(n int) string { return fmt.Sprintf("%d|%s|1|%s", n, E, E) }
	cands := []string{
		head + "|" + E + "|" + one(1) + "|N|" + one(2) + "|Y",
		head + "|" + E + "|" + one(1) + "|" + one(2) + "|Y",
		head + "|" + E + "|" + one(1) + "|Y|" + one(2) + "|Y",
		head + "|" + E + "|" + one(1) + "|N|" + one(2) + "|Y" +
			"|" + E + "|" + one(1) + "|Y|" + gold + "|" + rice + "|Y",
	}
	if v := os.Getenv("SAN1_BATTLEKEY"); v != "" {
		cands = []string{v}
	}
	alias := os.Getenv("SAN1_BATTLESHOT")
	if alias != "" {
		if len(cands) != 1 {
			t.Fatal("SAN1_BATTLESHOT 只能與 SAN1_BATTLEKEY 一起使用，避免四個候選互相覆寫")
		}
		if filepath.Base(alias) != alias || alias == "." || alias == ".." {
			t.Fatalf("SAN1_BATTLESHOT 只能是單一檔名，不可含路徑：%q", alias)
		}
	}
	const settle = 30_000_000
	for ci, cand := range cands {
		o.Restore(snap)
		o.Drain()
		for k := range asked {
			delete(asked, k)
		}
		for k := range hit {
			delete(hit, k)
		}
		for k := range readers {
			delete(readers, k)
		}
		for _, seg := range strings.Split(cand, "|") {
			k := strings.ReplaceAll(seg, enterMark, "\r")
			// 候選字串開頭的 `P:` 表示這一組走 `int 21h` 的字元佇列，
			// 其餘走硬體掃描碼。**兩條一起餵會產生重複的字元**，
			// 所以要分開試而不是都送。
			if strings.HasPrefix(cand, "P:") {
				o.TypeBoth(strings.TrimPrefix(k, "P:"))
			} else {
				o.PressScan(k)
			}
			if err := o.Run(settle); err != nil {
				t.Fatalf("候選 %q 執行停止：%v", cand, err)
			}
		}
		var route []string
		for _, m := range marks {
			if n := hit[m.name]; n > 0 {
				route = append(route, fmt.Sprintf("%s×%d", m.name, n))
			}
		}
		var who []string
		for a, n := range readers {
			who = append(who, fmt.Sprintf("%#07x×%d", a, n))
		}
		sort.Strings(who)
		var fields []string
		for a, n := range asked {
			fields = append(fields, fmt.Sprintf("%#07x×%d", a, n))
		}
		sort.Strings(fields)
		t.Logf("候選 %d %-30q → 走到 %v；讀鍵 %v；數字欄位 %v；鍵 %v",
			ci+1, cand, route, who, fields, keys)
		got, keys = got[:0], keys[:0]
		_ = got
		if alias != "" {
			dumpScreen(t, o, alias)
		} else {
			dumpScreen(t, o, fmt.Sprintf("sweep-%02d", ci+1))
		}
	}
}

// TestZZHookTraining 攔「訓練兵士」那支常式，讀它的參數與呼叫端。
//
// 常式在線性 `0xbd70`（`docs/re/03` §1.3）。它對郡裡每一位守將算
//
//	訓練度 = min(100, 訓練度 + (智/3 + 武/2) / 參數)
//
// **參數是唯一還不知道的東西**，而呼叫端就是電腦諸侯的決策位址——
// 一次攔截兩件事都拿得到。
func TestZZHookTraining(t *testing.T) {
	root := origRoot(t)
	c := openContainer(t, filepath.Join(root, "DATA2"))
	sc0, err := state.LoadScenario(c, state.Slot("001"))
	if err != nil {
		t.Fatal(err)
	}
	seedMas, _, _ := sc0.Tables()

	o, err := oracle.Load(filepath.Join(root, "AA.EXE"), root)
	if err != nil {
		t.Fatal(err)
	}
	defer o.Close()

	bootToGame(t, o, seedMas)

	seen := map[string]int{}
	// `0xbd70` 是共用常式，`0xbe80`／`0xbe94` 是兩個 thunk——各自推一個
	// 常數（4 與 3）再呼叫它。**決策端是呼叫 thunk 的人**，所以三個都攔。
	for _, site := range []struct {
		name string
		lin  uint32
	}{{"共用常式 0xbd70", 0xbd70}, {"thunk÷4 0xbe80", 0xbe80}, {"thunk÷3 0xbe94", 0xbe94}} {
		name := site.name
		o.OnCall(addr(site.lin), func(o *oracle.Oracle) {
			c := o.Caller()
			seen[fmt.Sprintf("%-18s ← 呼叫端 %04x:%04x（線性 %#06x）",
				name, c.Seg, c.Off, uint32(c.Seg)*16+uint32(c.Off))]++
		})
	}
	for _, keys := range []string{"4\r", "4\r", "Y"} {
		o.Drain()
		o.PressScan(keys)
		if err := o.Run(40_000_000 * 3); err != nil {
			t.Fatalf("原版停止：%v", err)
		}
	}
	if len(seen) == 0 {
		t.Log("一個月裡沒有人呼叫 0xbd70——位址可能不是常式的進入點")
		return
	}
	keys := make([]string, 0, len(seen))
	for k := range seen {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for _, k := range keys {
		t.Logf("%s ×%d", k, seen[k])
	}
}

// TestZZWhoWritesTheTables 跑一個月，列出寫三張表的程式位址。
func TestZZWhoWritesTheTables(t *testing.T) {
	root := origRoot(t)
	c := openContainer(t, filepath.Join(root, "DATA2"))
	sc0, err := state.LoadScenario(c, state.Slot("001"))
	if err != nil {
		t.Fatal(err)
	}
	seedMas, _, _ := sc0.Tables()

	o, err := oracle.Load(filepath.Join(root, "AA.EXE"), root)
	if err != nil {
		t.Fatal(err)
	}
	defer o.Close()

	base := bootToGame(t, o, seedMas)
	// **碼段和量到的位址要出自同一次執行**，否則對不上。
	// 範圍要蓋到主程式全部的碼段。`0x01f000` 那個上界是最初隨手取的，
	// 而外交（`2c21:1b56` ＝ 線性 `0x02dd66`）與交戰都落在它外面——
	// 讀不到的原因是 dump 太短，不是那段碼不在記憶體裡。
	// 戰場的字串在 `0x46c02` 一帶（`docs/re/04` §4），用它們的碼也在
	// 那個範圍附近，所以上界要拉到字串區的後面。
	dumpImage(t, o, 0x00b000, 0x050000, "code")
	nMas, nSta := state.MasterTableSize, state.PrefectureTableSize
	nGen := state.GeneralTableSize
	_ = state.MasterRecordSize

	// 分三段看，否則同一個 IP 寫哪一張表分不出來。
	for _, seg := range []struct {
		name string
		lo   uint32
		n    int
		rec  int
		fld  map[int]string
	}{
		{"州郡", base + uint32(nMas), nSta, state.PrefectureRecordSize, prefField},
		{"人物", base + uint32(nMas+nSta), nGen, state.GeneralRecordSize, genField},
	} {
		log := o.WatchWritesAt(seg.lo, seg.lo+uint32(seg.n)-1)
		for _, keys := range []string{"4\r", "4\r", "Y"} {
			o.Drain()
			o.PressScan(keys)
			if err := o.Run(40_000_000 * 3); err != nil {
				t.Fatalf("%s：原版停止 %v", seg.name, err)
			}
		}
		o.StopWatchingWrites()

		type site struct {
			n      int
			fields map[string]bool
		}
		by := map[uint32]*site{}
		for _, w := range *log {
			lin := uint32(w.IP.Seg)*16 + uint32(w.IP.Off)
			s := by[lin]
			if s == nil {
				s = &site{fields: map[string]bool{}}
				by[lin] = s
			}
			s.n++
			s.fields[fieldName(seg.fld, int(w.Off)%seg.rec)] = true
		}
		ips := make([]uint32, 0, len(by))
		for a := range by {
			ips = append(ips, a)
		}
		sort.Slice(ips, func(i, j int) bool { return by[ips[i]].n > by[ips[j]].n })

		t.Logf("%s表：一個月裡有 %d 次寫入，來自 %d 個位址",
			seg.name, len(*log), len(ips))
		for i, a := range ips {
			if i >= 20 {
				t.Logf("    …（還有 %d 個位址）", len(ips)-20)
				break
			}
			var fs []string
			for f := range by[a].fields {
				fs = append(fs, f)
			}
			sort.Strings(fs)
			t.Logf("    線性 %#06x 寫了 %4d 次：%v", a, by[a].n, fs)
		}
	}
}

// dispatchSites 是分派器裡**全部十八個**分派點，以及各自的表位址。
//
// **清單是拿位元組樣式掃出來的，不是順著讀出來的**：`ff 9f` ＝
// `lcall far [bx+disp16]`。前八個間隔固定 `0x11`，第九個之後隔著一整段
// 預算計算的碼——順著讀會在那裡停下來，那正是原本只數到九張的原因
// （`CONTEXT.md` R10）。
var dispatchSites = map[uint32]uint16{
	0xe926: 0x54d4, 0xe937: 0x5694, 0xe948: 0x5674, 0xe959: 0x5614,
	0xe96a: 0x5634, 0xe97b: 0x5554, 0xe98c: 0x5534, 0xe99d: 0x56b4,
	0xe9fc: 0x5594, 0xea5b: 0x5574, 0xeaba: 0x5514, 0xeb19: 0x55f4,
	0xeb78: 0x5654, 0xebd7: 0x56d4, 0xebe8: 0x56f4, 0xebf9: 0x55b4,
	0xec0a: 0x55d4, 0xec1b: 0x54f4,
}

// budgetSites 是六個「呼叫之前先算本回合預算」的表，配上它在係數表
// 一筆 12 byte 裡的位移（`docs/re/03` §1.4）。
var budgetSites = map[uint16]int{
	0x5594: 0, 0x5574: 2, 0x5514: 4, 0x55f4: 6, 0x5654: 8, 0x56d4: 10,
}

// TestZZDispatch 讀電腦諸侯的指令分派表。
//
// 十八個分派點形狀都一樣：
//
//	mov es, [0xa63a]
//	mov bx, es:[0x20f6]      ; 索引
//	shl bx; shl bx           ; ×4（far pointer）
//	lcall far ptr [bx+0x55NN]
//
// 表的位址彼此相差 `0x20` ＝ 8 個 far pointer，所以**索引是 0–7**，
// 也就是每種行為有八個版本。這一條把索引與解出來的目標位址讀下來
// ——那份對應就是「哪一種電腦諸侯做哪一件事」。
func TestZZDispatch(t *testing.T) {
	root := origRoot(t)
	c := openContainer(t, filepath.Join(root, "DATA2"))
	sc0, err := state.LoadScenario(c, state.Slot("001"))
	if err != nil {
		t.Fatal(err)
	}
	seedMas, _, _ := sc0.Tables()

	o, err := oracle.Load(filepath.Join(root, "AA.EXE"), root)
	if err != nil {
		t.Fatal(err)
	}
	defer o.Close()

	base := bootToGame(t, o, seedMas)
	t.Logf("三張表的基底 %#x，難度 %q，索引來源 %#06x 現在是 %d",
		base, envOr("SAN1_DIFFICULTY", "5"), 0x040736, o.Byte(addr(0x040736)))

	sites := dispatchSites
	// **表走 DS 不是 ES。** 那道 `ff 9f 54 55` 沒有 `26` 前綴，
	// 預設段就是 DS；讀成 ES 會拿到看起來像位址的垃圾。
	word := func(o *oracle.Oracle, lin uint32) uint32 {
		return uint32(o.Byte(addr(lin))) | uint32(o.Byte(addr(lin+1)))<<8
	}
	seen := map[string]int{}
	dumped := map[uint16]bool{}
	for lin, tbl := range sites {
		table := tbl
		at := lin
		o.OnCall(addr(lin), func(o *oracle.Oracle) {
			ds := uint32(o.DSReg())
			// 索引來自 `es:[0x20f6]`，而那個 ES 是前一道指令從變數載的。
			// 把它印出來才知道那一格落在哪張表的哪個欄位。
			ix := uint32(o.ES())*16 + 0x20f6
			seen[fmt.Sprintf("分派點 %#06x 表 %#04x 索引 %d（來源線性 %#06x，距基底 %+d）",
				at, table, o.BX()/4, ix, int(ix)-int(base))]++
			if dumped[table] {
				return
			}
			dumped[table] = true
			// 順便把「行動者排序」的加權表讀出來：鍵是
			// 智 ＋ 武 ＋ 表[身分]，表在 DS:0x5986，以身分×2 索引
			// （`0xf1d7` 的 `add ax, [bx+0x5986]`，`docs/re/03` §1.4）。
			if table == 0x5594 {
				// 武裝度重算用的兩個浮點常數（`0xc24a` 的
				// `fmul qword ds:[0xa5c8]`、`0xc257` 的 `ds:[0xa5a8]`）。
				for _, off := range []uint32{0xa5a8, 0xa5c8} {
					b := o.Bytes(addr(ds*16+off), 8)
					seen[fmt.Sprintf("  武裝度的浮點常數 DS:%#04x ＝ %g", off,
						math.Float64frombits(binary.LittleEndian.Uint64(b)))] = 0
				}
			}
			// 本回合的預算：六張表在呼叫前各寫一次 `es:[0x3d16]`，
			// 值 ＝ 係數表[12×(4×等級 + es:[0x3f08] mod 4) + 位移]
			// × 郡的金 × ds:[0xa632]（`docs/re/03` §1.4）。
			// 這裡把係數表整張、常數、以及那個取 mod 4 的量一起讀出來。
			if off, ok := budgetSites[table]; ok {
				seen[fmt.Sprintf("  預算：表 %#04x 取係數位移 %+d，此刻 es:[0x3d16] ＝ %d",
					table, off, int16(word(o, uint32(o.ES())*16+0x3d16)))]++
			}
			if table == 0x5594 {
				c := o.Bytes(addr(ds*16+0xa632), 8)
				seen[fmt.Sprintf("  預算的浮點常數 DS:0xa632 ＝ %g",
					math.Float64frombits(binary.LittleEndian.Uint64(c)))] = 0
				seen[fmt.Sprintf("  取 mod 4 的那個量 es:[0x3f08] ＝ %d（mod 4 ＝ %d）",
					int16(word(o, uint32(o.ES())*16+0x3f08)),
					int16(word(o, uint32(o.ES())*16+0x3f08))%4)]++
				// 係數表 DS:0x5714：24 筆 × 6 個 word。
				for lv := 0; lv < 6; lv++ {
					var row []string
					for ph := 0; ph < 4; ph++ {
						k := lv*4 + ph
						var six []string
						for j := 0; j < 6; j++ {
							six = append(six, fmt.Sprintf("%d",
								int16(word(o, ds*16+0x5714+uint32(k*12+j*2)))))
						}
						row = append(row, "相位"+fmt.Sprint(ph)+":"+strings.Join(six, ","))
					}
					seen[fmt.Sprintf("  預算係數 等級%d %s", lv, strings.Join(row, "  "))] = 0
				}
			}
			// 徵兵（`0xbeb8`）與調整兵力（`0xc2c4`）用到的常數與表。
			if table == 0x5574 {
				// 帶兵上限表：`es:[bx+0x666e]`，職位×2 索引，
				// 段來自 `ds:[0xa5c6]`（`0xbf4a`／`0xc4bd`）。
				capSeg := word(o, ds*16+0xa5c6)
				var caps []string
				for r := 0; r < 12; r++ {
					caps = append(caps, fmt.Sprintf("職位%d=%d", r,
						int16(word(o, capSeg*16+0x666e+uint32(r)*2))))
				}
				seen["  帶兵上限表 es:0x666e："+strings.Join(caps, " ")] = 0
				// 徵兵的兩個 qword（`0xbee2` 的 fsub、`0xbf02` 的下限）
				// 與兩個 dword（`0xbff7`／`0xc034` 的 fmul）。
				for _, c := range []struct {
					off  uint32
					wide bool
					what string
				}{
					{0xa5b0, true, "徵兵：人口減去的下限"},
					{0xa5b8, true, "徵兵：夾住用的常數"},
					{0xa5d0, false, "徵兵：訓練/武裝換算 A"},
					{0xa5d4, false, "徵兵：訓練/武裝換算 B"},
					{0xa5de, true, "調整兵力：份額的加項"},
					{0xa5e6, true, "買米：存糧目標的夾值"},
					{0xa5f2, true, "買米：買完之後留下的金的下限"},
					{0xa9ea, true, "計略：燒米燒錢的係數"},
					{0xa604, true, "賞賜金帛：忠誠增幅的係數"},
					{0xa60c, true, "賞賜金帛：反算花費的係數"},
				} {
					if c.wide {
						b := o.Bytes(addr(ds*16+c.off), 8)
						seen[fmt.Sprintf("  %s DS:%#04x ＝ %g（qword）", c.what, c.off,
							math.Float64frombits(binary.LittleEndian.Uint64(b)))] = 0
						continue
					}
					b := o.Bytes(addr(ds*16+c.off), 4)
					seen[fmt.Sprintf("  %s DS:%#04x ＝ %g（dword）", c.what, c.off,
						math.Float32frombits(binary.LittleEndian.Uint32(b)))] = 0
				}
			}
			// 出兵（表 `0x54f4`）：進攻那一條分支多一道兵力比較
			// （`0xb5f9`–`0xb61e`）：`表[ds:0x5430 + 8×es:[0x30fe]] ×
			// es:[0x3c96]` 小於目標郡的兵士就不打。把表與那兩個量讀出來。
			if table == 0x5594 {
				var w []string
				for i := 0; i < 12; i++ {
					b := o.Bytes(addr(ds*16+0x5430+uint32(i)*8), 8)
					w = append(w, fmt.Sprintf("[%d]=%g", i,
						math.Float64frombits(binary.LittleEndian.Uint64(b))))
				}
				seen["  出兵的兵力係數表 DS:0x5430："+strings.Join(w, " ")] = 0
				seg1 := word(o, ds*16+0xa590)
				seg2 := word(o, ds*16+0xa57e)
				seen[fmt.Sprintf("  出兵：索引 es:[0x30fe] ＝ %d、乘數 es:[0x3c96] ＝ %d",
					int16(word(o, seg1*16+0x30fe)), int16(word(o, seg2*16+0x3c96)))]++
			}
			if table == 0x54d4 {
				var w []string
				for st := 0; st < 12; st++ {
					w = append(w, fmt.Sprintf("身分%d=%d", st,
						int16(word(o, ds*16+0x5986+uint32(st)*2))))
				}
				seen["  行動者排序的加權表 DS:0x5986："+strings.Join(w, " ")] = 0
			}
			// 八個項目一次讀完：表彼此相差 0x20 ＝ 8 個 far pointer。
			for i := 0; i < 8; i++ {
				ent := ds*16 + uint32(table) + uint32(i)*4
				off := word(o, ent)
				seg := word(o, ent+2)
				tgt := seg*16 + off
				// thunk 的形狀是 `33 c0 9a .. .. .. .. b8 K K 50 0e e8`
				// ——推的常數在 `b8` 後面。
				extra := ""
				if o.Byte(addr(tgt)) == 0x33 && o.Byte(addr(tgt+7)) == 0xb8 {
					extra = fmt.Sprintf("　推的常數 %d", word(o, tgt+8))
				}
				seen[fmt.Sprintf("  表 %#04x[%d] → %04x:%04x（線性 %#06x）%s",
					table, i, seg, off, tgt, extra)] = 0
			}
		})
	}
	for _, keys := range []string{"4\r", "4\r", "Y"} {
		o.Drain()
		o.PressScan(keys)
		if err := o.Run(40_000_000 * 3); err != nil {
			t.Fatalf("原版停止：%v", err)
		}
	}
	keys := make([]string, 0, len(seen))
	for k := range seen {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for _, k := range keys {
		if n := seen[k]; n > 0 {
			t.Logf("%s ×%d", k, n)
		} else {
			t.Log(k)
		}
	}
}

// TestZZIndexSource 找出分派索引是誰寫的、寫的是什麼。
//
// 索引在線性 `0x040736`，十八個分派點共用它，取值只見過 4 與 5。
// **難度不是它**（難度 5 與 8 得到相同的索引值）。所以直接看寫入端。
func TestZZIndexSource(t *testing.T) {
	root := origRoot(t)
	c := openContainer(t, filepath.Join(root, "DATA2"))
	sc0, err := state.LoadScenario(c, state.Slot("001"))
	if err != nil {
		t.Fatal(err)
	}
	seedMas, _, _ := sc0.Tables()

	o, err := oracle.Load(filepath.Join(root, "AA.EXE"), root)
	if err != nil {
		t.Fatal(err)
	}
	defer o.Close()

	bootToGame(t, o, seedMas)

	// 分派常式的進入點在 0xe8d2（`push bp; mov bp,sp`）。
	// 參數就是等級，進去之後被夾在 0–5。
	lv := map[string]int{}
	o.OnCall(addr(0xe8d2), func(o *oracle.Oracle) {
		c := o.Caller()
		lv[fmt.Sprintf("等級 %d ← 呼叫端 %04x:%04x（線性 %#06x）",
			int16(o.Arg(0)), c.Seg, c.Off, uint32(c.Seg)*16+uint32(c.Off))]++
	})

	const ix = 0x040736
	log := o.WatchWritesAt(ix, ix+1)
	for _, keys := range []string{"4\r", "4\r", "Y"} {
		o.Drain()
		o.PressScan(keys)
		if err := o.Run(40_000_000 * 3); err != nil {
			t.Fatalf("原版停止：%v", err)
		}
	}
	o.StopWatchingWrites()

	seen := map[string]int{}
	for _, w := range *log {
		lin := uint32(w.IP.Seg)*16 + uint32(w.IP.Off)
		seen[fmt.Sprintf("線性 %#06x 把 %#04x 位移的值寫成 %d（原本 %d）",
			lin, w.Off, w.New, w.Old)]++
	}
	keys := make([]string, 0, len(seen))
	for k := range seen {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	lk := make([]string, 0, len(lv))
	for k := range lv {
		lk = append(lk, k)
	}
	sort.Strings(lk)
	for _, k := range lk {
		t.Logf("分派常式 0xe8d2：%s ×%d", k, lv[k])
	}
	t.Logf("一個月裡 %#x 被寫了 %d 次，來自 %d 種寫法", ix, len(*log), len(seen))
	for i, k := range keys {
		if i >= 20 {
			t.Logf("    …（還有 %d 種）", len(keys)-20)
			break
		}
		t.Logf("    %s ×%d", k, seen[k])
	}
}

// TestZZTableMeaning 把分派表各自對應到哪些欄位。
//
// 做法是**時間軸歸屬**：分派點與盤面寫入都帶著執行到第幾道指令
// （`MemWrite.Step`／`Oracle.Steps`），所以每一次寫入都可以歸給它前面
// 最近的那一次分派。不必逐支反組譯。
//
// ⚠ 歸屬只在「分派之間不重疊」時成立。十八個分派點是**順序**執行的
// （`0xe926` 到 `0xec1b` 一路往下，中間沒有分支），所以前提成立；
// 但被呼叫的常式如果自己又轉呼叫別的東西，寫入還是算在它頭上——
// 那正是我們要的。
func TestZZTableMeaning(t *testing.T) {
	root := origRoot(t)
	c := openContainer(t, filepath.Join(root, "DATA2"))
	sc0, err := state.LoadScenario(c, state.Slot("001"))
	if err != nil {
		t.Fatal(err)
	}
	seedMas, _, _ := sc0.Tables()

	o, err := oracle.Load(filepath.Join(root, "AA.EXE"), root)
	if err != nil {
		t.Fatal(err)
	}
	defer o.Close()

	base := bootToGame(t, o, seedMas)
	nMas, nSta := state.MasterTableSize, state.PrefectureTableSize
	total := nMas + nSta + state.GeneralTableSize

	type ev struct {
		step  uint64
		table uint16
	}
	var evs []ev
	for lin, tbl := range dispatchSites {
		table := tbl
		o.OnCall(addr(lin), func(o *oracle.Oracle) {
			evs = append(evs, ev{o.Steps(), table})
		})
	}
	log := o.WatchWritesAt(base, base+uint32(total)-1)
	for _, keys := range []string{"4\r", "4\r", "Y"} {
		o.Drain()
		o.PressScan(keys)
		if err := o.Run(40_000_000 * 3); err != nil {
			t.Fatalf("原版停止：%v", err)
		}
	}
	o.StopWatchingWrites()
	sort.Slice(evs, func(i, j int) bool { return evs[i].step < evs[j].step })

	fields := map[uint16]map[string]int{}
	orphan := 0
	for _, w := range *log {
		i := sort.Search(len(evs), func(k int) bool { return evs[k].step > w.Step }) - 1
		if i < 0 {
			orphan++
			continue
		}
		tbl := evs[i].table
		if fields[tbl] == nil {
			fields[tbl] = map[string]int{}
		}
		off := int(w.Off)
		var name string
		switch {
		case off < nMas:
			name = "諸侯." + fieldName(masField, off%state.MasterRecordSize)
		case off < nMas+nSta:
			name = "州郡." + fieldName(prefField, (off-nMas)%state.PrefectureRecordSize)
		default:
			name = "人物." + fieldName(genField, (off-nMas-nSta)%state.GeneralRecordSize)
		}
		fields[tbl][name]++
	}
	t.Logf("一個月：分派 %d 次、盤面寫入 %d 次（%d 次落在第一次分派之前）",
		len(evs), len(*log), orphan)
	tbls := make([]int, 0, len(fields))
	for tb := range fields {
		tbls = append(tbls, int(tb))
	}
	sort.Ints(tbls)
	for _, tb := range tbls {
		m := fields[uint16(tb)]
		ks := make([]string, 0, len(m))
		for k := range m {
			ks = append(ks, k)
		}
		sort.Slice(ks, func(i, j int) bool { return m[ks[i]] > m[ks[j]] })
		var parts []string
		for i, k := range ks {
			if i >= 8 {
				parts = append(parts, "…")
				break
			}
			parts = append(parts, fmt.Sprintf("%s×%d", k, m[k]))
		}
		t.Logf("表 %#04x → %s", tb, strings.Join(parts, " "))
	}
}

// TestZZRandom 確認 `1058:058c` 是不是原版的亂數。
//
// 內政那支常式（`0xba9c`）的形狀是
//
//	r1 = f(K)     K ＝ 等級常數
//	r1 == 0 → 土地開發
//	否則 r2 = f(K)；r2 == 1 → 洪水防治，否則不做
//
// 如果 `f(n)` 回 0..n−1 而且分布平坦，那就是 `RND`——**這一支定位出來
// 之後，其他常式裡的每一個「機率」都跟著讀得出來**。
//
// 判準是**回傳值的分布**，不是名字：只看「有被呼叫」證明不了什麼。
func TestZZRandom(t *testing.T) {
	root := origRoot(t)
	c := openContainer(t, filepath.Join(root, "DATA2"))
	sc0, err := state.LoadScenario(c, state.Slot("001"))
	if err != nil {
		t.Fatal(err)
	}
	seedMas, _, _ := sc0.Tables()

	o, err := oracle.Load(filepath.Join(root, "AA.EXE"), root)
	if err != nil {
		t.Fatal(err)
	}
	defer o.Close()

	bootToGame(t, o, seedMas)

	const rnd = 0x1058*16 + 0x058c
	args := map[uint16]int{}
	o.OnCall(addr(rnd), func(o *oracle.Oracle) { args[o.Arg(0)]++ })

	// 回傳值要在呼叫端的下一道指令讀（`add sp,2` 之前 AX 還是回傳值）。
	ret := map[string]int{}
	for _, site := range []struct {
		name string
		lin  uint32
	}{{"內政 r=f(K)", 0xbaac}, {"內政 第二次", 0xbae5}} {
		name := site.name
		o.OnCall(addr(site.lin), func(o *oracle.Oracle) {
			ret[fmt.Sprintf("%s 回 %d", name, o.AX())]++
		})
	}
	for _, keys := range []string{"4\r", "4\r", "Y"} {
		o.Drain()
		o.PressScan(keys)
		if err := o.Run(40_000_000 * 3); err != nil {
			t.Fatalf("原版停止：%v", err)
		}
	}
	ak := make([]int, 0, len(args))
	for a := range args {
		ak = append(ak, int(a))
	}
	sort.Ints(ak)
	total := 0
	for _, a := range ak {
		total += args[uint16(a)]
	}
	t.Logf("%#06x 一個月被呼叫 %d 次，參數：", rnd, total)
	for _, a := range ak {
		t.Logf("    參數 %d ×%d", a, args[uint16(a)])
	}
	rk := make([]string, 0, len(ret))
	for k := range ret {
		rk = append(rk, k)
	}
	sort.Strings(rk)
	for _, k := range rk {
		t.Logf("    %s ×%d", k, ret[k])
	}
}

// TestZZDispatchScope 問一件事：分派器對哪些郡跑。
//
// `0x5674`（指定太守）開頭檢查「諸侯 offset 0 == 1 就跳過」——玩家的
// 勢力不做；但 `0x5594`（武裝度）沒有那個檢查。**所以這些表裡可能有
// 一部分是每月結算而不是 AI 決策**，而那決定 `internal/ai` 與
// `internal/game` 的分工。
//
// 判準很直接：分派器的呼叫端（`0x017504`）第一個參數就是郡編號，
// 看**玩家的郡有沒有出現在名單裡**。
func TestZZDispatchScope(t *testing.T) {
	root := origRoot(t)
	c := openContainer(t, filepath.Join(root, "DATA2"))
	sc0, err := state.LoadScenario(c, state.Slot("001"))
	if err != nil {
		t.Fatal(err)
	}
	seedMas, _, _ := sc0.Tables()

	o, err := oracle.Load(filepath.Join(root, "AA.EXE"), root)
	if err != nil {
		t.Fatal(err)
	}
	defer o.Close()

	base := bootToGame(t, o, seedMas)
	nMas := state.MasterTableSize
	board := o.Bytes(addr(base), nMas+state.PrefectureTableSize)

	// 誰是玩家、玩家有哪些郡。
	player := -1
	for i := 0; i < nMas/state.MasterRecordSize; i++ {
		off := i * state.MasterRecordSize
		if int(board[off])|int(board[off+1])<<8 == 1 {
			player = i
		}
	}
	mine := map[int]bool{}
	rec := state.PrefectureRecordSize
	for i := 1; i*rec < state.PrefectureTableSize; i++ {
		if int(board[nMas+i*rec+30]) == player {
			mine[i] = true
		}
	}
	t.Logf("玩家是勢力 %d，擁有 %d 個郡：%v", player, len(mine), keysOf(mine))

	seen := map[int]int{}
	o.OnCall(addr(0x017504), func(o *oracle.Oracle) { seen[int(o.Arg(0))]++ })
	for _, keys := range []string{"4\r", "4\r", "Y"} {
		o.Drain()
		o.PressScan(keys)
		if err := o.Run(40_000_000 * 3); err != nil {
			t.Fatalf("原版停止：%v", err)
		}
	}
	hitMine, hitOther := 0, 0
	for p, n := range seen {
		if mine[p] {
			hitMine += n
		} else {
			hitOther += n
		}
	}
	t.Logf("分派器一個月跑了 %d 個相異的郡、共 %d 次；其中玩家的郡 %d 次、別人的 %d 次",
		len(seen), hitMine+hitOther, hitMine, hitOther)
	if hitMine > 0 {
		t.Log("→ **分派器對玩家的郡也跑**，所以這些表裡有一部分是每月結算")
	} else {
		t.Log("→ 分派器只對電腦諸侯的郡跑，這些表全部是 AI 決策")
	}
}

func keysOf(m map[int]bool) []int {
	out := make([]int, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	sort.Ints(out)
	return out
}

// TestZZSpend 量「扣錢」那支常式的等級係數。
//
// `0e8d:0354`（線性 `0xec24`）把呼叫端給的金額乘上一個**以 AI 等級
// 索引的浮點係數**，再從 AI 的本回合預算（`es:[0x3d16]`）與郡的金
// （州郡 offset 18）各扣一次，兩邊都夾下限 0。
//
// 係數在浮點表裡，`objdump` 讀不到（MSC 的浮點模擬器指令流）——
// **但量得到**：進場時的參數與轉回整數之後的 AX 配成一對就是係數。
func TestZZSpend(t *testing.T) {
	root := origRoot(t)
	c := openContainer(t, filepath.Join(root, "DATA2"))
	sc0, err := state.LoadScenario(c, state.Slot("001"))
	if err != nil {
		t.Fatal(err)
	}
	seedMas, _, _ := sc0.Tables()

	o, err := oracle.Load(filepath.Join(root, "AA.EXE"), root)
	if err != nil {
		t.Fatal(err)
	}
	defer o.Close()

	bootToGame(t, o, seedMas)

	var pending []int
	pairs := map[string]int{}
	o.OnCall(addr(0xec24), func(o *oracle.Oracle) {
		pending = append(pending, int(int16(o.Arg(0))))
	})
	o.OnCall(addr(0xec49), func(o *oracle.Oracle) {
		if len(pending) == 0 {
			return
		}
		base := pending[len(pending)-1]
		pending = pending[:len(pending)-1]
		lv := o.Byte(addr(0x040736))
		pairs[fmt.Sprintf("等級 %d：base %d → 扣 %d", lv, base, int(int16(o.AX())))]++
	})
	for _, keys := range []string{"4\r", "4\r", "Y"} {
		o.Drain()
		o.PressScan(keys)
		if err := o.Run(40_000_000 * 3); err != nil {
			t.Fatalf("原版停止：%v", err)
		}
	}
	// 係數表是 double 陣列（索引 ＝ 等級 × 8）。等級 5 量到 0.75，
	// 所以直接在記憶體裡搜那個 double 的位元組，回頭讀整張表。
	const f75 = "\x00\x00\x00\x00\x00\x00\xe8\x3f" // IEEE 754 的 0.75
	for _, at := range o.Search([]byte(f75)) {
		lo := at
		if lo >= 40 {
			lo -= 40
		}
		var vals []string
		for k := 0; k < 8; k++ {
			b := o.Bytes(addr(lo+uint32(k)*8), 8)
			vals = append(vals, fmt.Sprintf("%.4g", math.Float64frombits(
				binary.LittleEndian.Uint64(b))))
		}
		t.Logf("0.75 出現在 %#06x；%#06x 起的八個 double：%v", at, lo, vals)
	}

	ks := make([]string, 0, len(pairs))
	for k := range pairs {
		ks = append(ks, k)
	}
	sort.Strings(ks)
	t.Logf("一個月扣錢 %d 種組合：", len(ks))
	for i, k := range ks {
		if i >= 24 {
			t.Logf("    …（還有 %d 種）", len(ks)-24)
			break
		}
		t.Logf("    %s ×%d", k, pairs[k])
	}
}
