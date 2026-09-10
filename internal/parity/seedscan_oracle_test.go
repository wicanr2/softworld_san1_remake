//go:build oracle

package parity

import (
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"github.com/wicanr2/dosgolem/oracle"

	"github.com/wicanr2/softworld_san1_remake/internal/state"
)

// 掃亂數種子，看每一個讓玩家排到順序表的第幾格。
//
// **為什麼要挑**：月度對拍的視窗是 `[游標, 玩家的郡)`——玩家排得越後面，
// 一次比得到的郡越多。而玩家排第幾格是開月洗牌抽出來的，所以種子決定
// 視窗大小（`CONTEXT.md` R49）。
//
// ⚠ **挑的判準是視窗大小，不是「哪個種子讓測試變綠」。** 後者是拿樣本數
// 換綠：視窗小到只剩兩三個郡時什麼都比不到，而測試照樣印 PASS。
// 所以這一支只報「玩家排第幾格」，不跑對拍、不看差異。
//
// 一次開機掃全部：`bootToGame` 要三分鐘，而換種子只要
// `Restore` ＋ 兩個 `SetWord`，所以把機器狀態存下來重複用。
func TestZZSeedScan(t *testing.T) {
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
	seq := strings.Split(envOr("SAN1_TURNKEY", "4\r|4\r|Y"), "|")
	total := state.MasterTableSize + state.PrefectureTableSize + state.GeneralTableSize
	// 走到月底結算的入口並停在那裡（種子 0 ＝ 先不動亂數）。
	driveToMonthStart(t, o, seq, base, total, 0)
	atSettle := o.Save()

	seeds := []uint32{
		0x5A17C0DE, 0x11111111, 0x22222222, 0xDEADBEEF, 0x00C0FFEE,
		0x13579BDF, 0x2468ACE0, 0xA5A5A5A5, 0x0F0F0F0F, 0x7FFFFFFF,
	}
	if v := os.Getenv("SAN1_SEEDS"); v != "" {
		seeds = nil
		for _, s := range strings.Split(v, ",") {
			if n, err := strconv.ParseUint(strings.TrimSpace(s), 0, 32); err == nil {
				seeds = append(seeds, uint32(n))
			}
		}
	}

	// **玩家是誰要問原版的記憶體，不是問劇本檔。** 諸侯記錄 offset 0
	// （操縱方：1 ＝ 玩家、2 ＝ 電腦、`0xFFFF` ＝ 沒在用）**劇本檔裡還沒
	// 填**，存檔才有（`docs/formats/03`）。拿劇本檔問會得到 −1。
	player := -1
	for i := 0; i < 16; i++ {
		if o.Word(addr(base+uint32(i*72))) == 1 {
			player = i
			break
		}
	}
	if player < 0 {
		t.Fatal("盤面上找不到玩家操縱的勢力——沒有玩家就沒有視窗可言")
	}
	t.Logf("玩家勢力槽號 %d", player)

	type row struct {
		seed uint32
		at   int
	}
	var rows []row
	for _, sd := range seeds {
		o.Restore(atSettle)
		ds := uint32(o.DSReg()) * 16
		o.SetWord(addr(ds+0xa3ae), uint16(sd))
		o.SetWord(addr(ds+0xa3b0), uint16(sd>>16))

		// 跑到第一個郡的回合，讀順序表與旗標。
		var seqTab []int
		got := false
		o.OnCall(addr(0x1746e), func(o *oracle.Oracle) {
			if got {
				return
			}
			got = true
			d := uint32(o.DSReg()) * 16
			es := uint32(o.Word(addr(d+0xa726))) * 16
			for i := 0; i < 43; i++ {
				seqTab = append(seqTab, int(int16(o.Word(addr(es+0x0e+uint32(i)*2)))))
			}
		})
		for i := 0; i < 12 && !got; i++ {
			if err := o.Run(40_000_000); err != nil {
				t.Fatalf("種子 %#08x：%v", sd, err)
			}
		}
		if !got {
			t.Logf("種子 %#08x：沒走到第一個郡", sd)
			continue
		}
		// 玩家的第一個郡排在第幾格。
		staBase := base + uint32(state.MasterTableSize)
		at := len(seqTab)
		for i, p := range seqTab {
			if p <= 0 || p >= 43 {
				continue
			}
			if int(o.Byte(addr(staBase+uint32(p*176+30)))) == player {
				at = i
				break
			}
		}
		// ⚠ **「找不到」不能與「排在最後」共用一個值。** 第一版把找不到
		// 寫成 `at = len(seqTab)` ＝ 43，於是十個種子全報「視窗 43 個郡」
		// ——看起來像大豐收，其實是玩家槽號解錯（−1）而一次都沒比中。
		// 全部一樣就是沒在測東西。
		if at >= len(seqTab) {
			t.Fatalf("種子 %#08x：順序表裡找不到玩家（勢力 %d）的郡——"+
				"是槽號解錯還是玩家一個郡都不剩？", sd, player)
		}
		rows = append(rows, row{sd, at})
		t.Logf("種子 %#08x → 玩家排第 %2d 格（視窗 %2d 個郡）", sd, at, at)
	}

	best := row{}
	for _, r := range rows {
		if r.at > best.at {
			best = r
		}
	}
	t.Logf("視窗最大的是種子 %#08x：%d 個郡", best.seed, best.at)
}
