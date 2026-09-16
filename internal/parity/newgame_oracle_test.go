//go:build oracle

package parity

import (
	"fmt"
	"path/filepath"
	"testing"

	"github.com/wicanr2/dosgolem/oracle"

	"github.com/wicanr2/softworld_san1_remake/internal/state"
)

// 開一局新的，選指定的君主。
//
// `bootToGame` 走的是「載入舊進度」，落在建安二年、南海、一個只有一位
// 將領的郡——那個郡出兵會被原版擋掉（移出之後沒有人治理），編隊那一段
// 因此驗不到。開新局才選得到有兵有將、旁邊還有敵人的局面。
//
// **開新遊戲是一等驗收路徑**（`CLAUDE.md` §7 第 12 條）：從存檔載入
// 看不到的缺口，開局才會露出來。
//
// 劇本一的諸侯依槽號排，選君主那一格是 1 起算：
//
//	1 劉備（郡 8、3 人）    2 曹操（郡 11、7 人）   3 孫堅（郡 31、6 人）
//	4 袁紹（郡 3,4、19 人） 5 袁術（郡 13,27）      6 董卓（郡 6,14,15,16）
//	…
func bootToNewGame(t *testing.T, o *oracle.Oracle, lord int, mas []byte) uint32 {
	t.Helper()
	return bootToNewGameAt(t, o, lord, 5, mas)
}

// newGameDrive 是開新局那幾步的行為停點：主選單 → 年代 → 玩家數 → 君主
// → 難度 → 防拷／就任對白 → 主命令。拆成兩段，讓選君主那一格的畫面
// 也拿得到（`TestZZLordPickScreenMatchesTheOriginal`）。
type newGameDrive struct {
	t          *testing.T
	o          *oracle.Oracle
	s          *bootSignals
	ascCallers []uint32
	keyCalls   int
	nums       []numAsk
}

type numAsk struct {
	lo, hi int
}

// 主選單選「開始新遊戲」後，年代選擇的低階輸入 caller 是
// runtime 1058:17D7；保留原始定位，不把它改名成產品語意。
const eraCaller = 0x1058*16 + 0x17d7

// 年代確認後先走 runtime 33D8:1FAB 的字元輸入，再進入君主欄位。
const playerCountCaller = 0x33d8*16 + 0x1fab

func (d *newGameDrive) waitCaller(name string, want uint32) {
	before := len(d.ascCallers)
	waitBoot(d.t, d.o, name, 500_000_000, func() bool {
		for _, c := range d.ascCallers[before:] {
			if c == want {
				return true
			}
		}
		return false
	})
}

func (d *newGameDrive) waitNum(name string, lo, hi int) {
	before := len(d.nums)
	cond := oracle.NewCond(name, func(*oracle.Oracle) bool {
		for _, n := range d.nums[before:] {
			if n.lo == lo && n.hi == hi {
				return true
			}
		}
		return false
	})
	if err := d.o.RunUntil(cond, oracle.Budget(1_000_000_000)); err != nil {
		d.t.Fatalf("等待%s失敗（nums=%v、asc=%v）：%v", name, d.nums, d.ascCallers, err)
	}
	// 玩家數、君主與難度都走 runtime `1058:0E57` 的掃描碼等待。
	waitBootScan(d.t, d.o, name+"掃描碼", 5_000_000)
}

// bootToLordPick 把原版開到**選君主**那一格（劇本一、一位玩家），停在
// 君主編號的數字輸入（1..6）——右側面板這時畫著第一頁六位君主的肖像
// （`0x124ca`）。
func bootToLordPick(t *testing.T, o *oracle.Oracle) *newGameDrive {
	t.Helper()
	// 以行為停點驅動，不再依賴已作廢的 50M 指令分段配方。
	// `bootToMenu` 已完成裝置題、開場與標題，並停在主選單掃描碼迴圈。
	d := &newGameDrive{t: t, o: o, s: bootToMenu(t, o)}
	o.OnCall(addr(bootASCInputFn), func(o *oracle.Oracle) {
		c := o.Caller()
		d.ascCallers = append(d.ascCallers, uint32(c.Seg)*16+uint32(c.Off))
	})
	o.OnCall(addr(bootKeyInputFn), func(*oracle.Oracle) { d.keyCalls++ })
	o.OnCall(addr(bootNumInputFn), func(o *oracle.Oracle) {
		d.nums = append(d.nums, numAsk{int(o.Arg(0)), int(o.Arg(1))})
	})

	o.Drain()
	o.PressScan("1")
	d.waitCaller("新局年代選擇輸入", eraCaller)
	// callback 只證明年代輸入常式被呼叫；先等它回到 runtime 掃描碼
	// 迴圈，才送下一個字元，避免鍵在畫面切換前被消費。
	waitBootScan(t, o, "新局年代畫面", 500_000_000)
	o.Drain()
	o.TypeBoth("1")
	d.waitCaller("新局玩家數輸入", playerCountCaller)
	waitBootScan(t, o, "新局玩家數畫面", 500_000_000)
	o.Drain()
	o.TypeBoth("1\r")

	// 玩家數已在上面的字元 caller 停點送出 1；這個數字欄位是選君主。
	d.waitNum("新局君主輸入", 1, 6)
	return d
}

// bootToNewGameAt 是 bootToNewGame 加上難度（1..10）。
func bootToNewGameAt(t *testing.T, o *oracle.Oracle, lord, difficulty int, mas []byte) uint32 {
	t.Helper()
	d := bootToLordPick(t, o)
	s := d.s
	o.Drain()
	o.PressScan(fmt.Sprintf("%d\r", lord))
	// 密碼提示同樣有既有 observer 的行為路標（03EB:02C2、0..9999）。
	// 在等待難度欄位的掃描碼沉澱前先記基準，避免密碼 callback 恰好
	// 出現在那段沉澱期間時被漏算。
	beforePassword := s.passwordAsk
	beforeMain := s.mainAsk
	d.waitNum("新局難度輸入", 1, 10)
	o.Drain()
	// 不把後續主命令的 0..9 callback 誤當成密碼欄位；那個 callback
	// 可能在掃描碼等待期間先出現。
	o.TypeBoth(fmt.Sprintf("%d\r", difficulty))
	// 開局後的第一個月是否抽中防拷盤問由原版自己的 RND(12) 決定；
	// 不把「有盤問」硬編成必要條件，否則另一個合法亂數狀態會被誤判
	// 成開機失敗。兩個 callback 都是原始輸入路標，先等其中一個。
	waitBoot(t, o, "新局防拷或主命令輸入", 500_000_000,
		func() bool {
			return s.passwordAsk > beforePassword || s.mainAsk > beforeMain
		})
	if s.passwordAsk > beforePassword {
		o.Drain()
		// 密碼欄位在 `03EB:02C2` 走字元路徑；只餵字元佇列，避免掃描碼
		// 副本污染後面的 Y/N 確認。
		o.Type(passwordAnswer + "\r")
		beforeYN := s.passwordYN
		waitBoot(t, o, "新局密碼確認", 500_000_000,
			func() bool { return s.passwordYN > beforeYN })
		o.Drain()
		o.PressScan("Y")
	}

	// 就任對白的句數由盤面決定，這裡只用輸入 callsite 作「下一句可送」
	// 的行為停點；主命令 caller 出現即停止。最多 32 句是失敗即關閉的
	// 安全上限，不是成功條件。
	for i := 0; i < 32 && s.mainAsk == beforeMain; i++ {
		beforeKey := d.keyCalls
		o.Drain()
		o.PressScan("\r")
		waitBoot(t, o, fmt.Sprintf("新局就任對白第 %d 句", i+1),
			100_000_000, func() bool {
				return s.mainAsk > beforeMain || d.keyCalls > beforeKey
			})
	}
	if s.mainAsk == beforeMain {
		t.Fatalf("新局就任對白超過 32 句仍未到主命令輸入")
	}
	waitBootScan(t, o, "新局主命令", 5_000_000)

	// base 版的執行期資料表基底由 `docs/re/02` §3.3 固定為 0x399B0。
	// 開局選君主會更新諸侯表的動態欄位，因此不能再拿劇本前 48 bytes
	// 做 exact Search；固定基底後仍由下游 DecodeTables 驗證三張表形狀。
	const base = uint32(0x399b0)
	if len(mas) < 48 || len(o.Bytes(addr(base), len(mas))) != len(mas) {
		t.Fatalf("新局三張表基底 %#x 無法讀取完整諸侯表", base)
	}
	return base
}

// TestPlayerCommandsAsCaoCao 拿劇本一的曹操再走一次逐道對拍。
//
// 曹操在郡 11、在職七人，鄰郡有別的勢力——**出兵那一道真的打得起來**，
// 而 `bootToGame` 那個局面（南海、一位將領）只驗得到「兩邊都拒絕」。
func TestPlayerCommandsAsCaoCao(t *testing.T) {
	runPlayerCommands(t, func(t *testing.T, o *oracle.Oracle, mas []byte) uint32 {
		return bootToNewGame(t, o, caoCaoPick, mas)
	})
}

// caoCaoPick 是選君主那一格要送的號碼（劇本一，1 起算）。
const caoCaoPick = 2

// TestZZNewGameBoardIsCaoCao 正對照：開局之後玩家真的是曹操。
//
// 選錯號碼不會報錯，只會讓後面每一道命令都對著別人的郡下——
// 而那看起來與「命令算錯了」一模一樣。
func TestZZNewGameBoardIsCaoCao(t *testing.T) {
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
	base := bootToNewGame(t, o, caoCaoPick, seedMas)

	nMas, nSta, nGen := state.MasterTableSize, state.PrefectureTableSize,
		state.GeneralTableSize
	raw := o.Bytes(addr(base), nMas+nSta+nGen)
	sc, err := state.DecodeTables(state.Slot("001"),
		raw[:nMas], raw[nMas:nMas+nSta], raw[nMas+nSta:])
	if err != nil {
		t.Fatal(err)
	}
	players := sc.Players()
	if len(players) != 1 {
		t.Fatalf("玩家有 %v 個，開局選的是一個人", players)
	}
	lord, err := sc.Lord(players[0])
	if err != nil {
		t.Fatal(err)
	}
	work := o.ES()
	at := int(o.Word(oracle.Addr{Seg: work, Off: curPrefOff}))
	t.Logf("玩家勢力 %d，君主 %s，主畫面停在郡 %d", players[0], lord.Name, at)
	if lord.Name != "曹操" {
		t.Errorf("選君主送 %d 拿到的是 %s，不是曹操", caoCaoPick, lord.Name)
	}
}

// TestZZNewGameBoardBase 是 `TestZZNewGameBoardPlus` 的原版對照：同一個
// 停點（第一個郡回合入口 `0x1746e`）、同一份遮罩 `newGameRuntimeFields`。
func TestZZNewGameBoardBase(t *testing.T) {
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
	var board []byte
	o.OnCall(addr(0x1746e), func(o *oracle.Oracle) {
		if board == nil {
			board = o.Bytes(addr(0x399b0), state.MasterTableSize+state.PrefectureTableSize+state.GeneralTableSize)
		}
	})
	bootToNewGame(t, o, caoCaoPick, seedMas)
	if board == nil {
		t.Fatal("走到主命令了，郡回合入口卻一次都沒攔到")
	}
	checkNewGameBoard(t, state.EditionBase, sc0, board)
}
