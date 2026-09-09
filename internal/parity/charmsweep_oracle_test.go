//go:build oracle

package parity

import (
	"fmt"
	"path/filepath"
	"testing"

	"github.com/wicanr2/dosgolem/oracle"

	"github.com/wicanr2/softworld_san1_remake/internal/state"
)

// 賞賜金帛與開倉賑民**玩家那條**的效果，對主事者的魅力掃一遍。
//
// 兩點框不出式子：賞賜在魅力 43 給 +27、魅力 60 給 +38，兩點差 11，
// 而 `魅力÷3` 只差 6、`÷2` 差 9，都不對。與其在兩點上湊係數，
// 不如把魅力擺成一串值各量一次——**開機一次、存快照，每一格還原重試**，
// 一格五秒。
//
// 賑民那一邊順便驗上限：送 300 米、每格 5，原始增幅 60，
// 所有候選魅力的上限都咬得到，所以量到的就是上限本身。
func TestZZPlayerCharmSweep(t *testing.T) {
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

	nMas, nSta, nGen := state.MasterTableSize, state.PrefectureTableSize,
		state.GeneralTableSize
	work := o.ES()
	at := int(o.Word(oracle.Addr{Seg: work, Off: curPrefOff}))
	raw := o.Bytes(addr(base), nMas+nSta+nGen)
	gi := -1
	for i := 0; i < nGen/state.GeneralRecordSize; i++ {
		r := raw[nMas+nSta+i*state.GeneralRecordSize:]
		if r[17] <= 3 && int(r[19]) == at {
			gi = i
			break
		}
	}
	if gi < 0 {
		t.Fatalf("郡 %d 沒有在職武將", at)
	}
	pref := base + uint32(nMas+at*state.PrefectureRecordSize)
	gen := base + uint32(nMas+nSta+gi*state.GeneralRecordSize)
	t.Logf("郡 %d、將領槽號 %d、人口 %d00、物價 %d",
		at, gi, o.Word(addr(pref+14)), o.Byte(addr(pref+29)))

	snap := o.Save()
	// **14／28／42／89 是拿來分辨係數的。** 掃出來的範圍是
	// 0.64 ≤ k < 0.6444，而 0.64 剛好貼在下界；這四個魅力值上，
	// `0.64` 與任何略大的係數會差一格（例如 14 × 0.64 ＝ 8.96 → 8，
	// 14 × 0.6429 ＝ 9.0 → 9）。貼著邊界的常數要挑會分家的點驗。
	charms := []int{6, 12, 14, 24, 28, 30, 42, 43, 51, 60, 75, 89, 90, 99}

	// 賞賜：忠誠擺 10 留出空間（上限 100 會把大的增幅截掉，
	// 截掉之後量到的是 90 不是效果）。
	t.Log("賞賜金帛（賞 100 金，忠誠起點 10）：")
	for _, cm := range charms {
		o.Restore(snap)
		o.SetWord(addr(pref+18), 9000)
		o.SetWord(addr(pref+20), 9000)
		o.SetByte(addr(gen+generalCharmOff), uint8(cm))
		o.SetByte(addr(gen+generalLoyaltyOff), 10)
		gold0 := o.Word(addr(pref + 18))
		for _, k := range []string{"6\r", "3\r", "1\r", "100\r"} {
			o.PressScan(k)
			if err := o.Run(playerSettle); err != nil {
				t.Fatalf("魅力 %d 送 %q 時停止：%v", cm, k, err)
			}
		}
		t.Logf("  魅力 %3d → 忠誠 %3d（增幅 %3d）、金 %d → %d",
			cm, o.Byte(addr(gen+generalLoyaltyOff)),
			int(o.Byte(addr(gen+generalLoyaltyOff)))-10,
			gold0, o.Word(addr(pref+18)))
	}

	// 賑民：送 300 米，原始增幅 300 ÷ 每格；量到的是上限。
	t.Log("開倉賑民（給 300 米）：")
	for _, cm := range charms {
		o.Restore(snap)
		o.SetWord(addr(pref+18), 9000)
		o.SetWord(addr(pref+20), 9000)
		o.SetByte(addr(gen+generalCharmOff), uint8(cm))
		o.SetByte(addr(pref+26), 10) // 民心起點
		for _, k := range []string{"5\r", "3\r", "300\r"} {
			o.PressScan(k)
			if err := o.Run(playerSettle); err != nil {
				t.Fatalf("魅力 %d 送 %q 時停止：%v", cm, k, err)
			}
		}
		t.Logf("  魅力 %3d → 民心 %3d（增幅 %3d）、米 %d",
			cm, o.Byte(addr(pref+26)), int(o.Byte(addr(pref+26)))-10,
			o.Word(addr(pref+20)))
	}
	// **金與米那一維也要量。** 上面兩串都固定在金 100、米 300 一個點上，
	// 所以係數落在「魅力那一邊」還是「金那一邊」分不出來。魅力釘在 99
	// （賑民的上限 33 夠高，原始增幅看得見），掃金與米。
	t.Log("賞賜金帛（魅力 99、忠誠起點 5，掃金）：")
	for _, g := range []int{10, 20, 30, 40, 50, 60, 70, 80, 90, 100} {
		o.Restore(snap)
		o.SetWord(addr(pref+18), 9000)
		o.SetWord(addr(pref+20), 9000)
		o.SetByte(addr(gen+generalCharmOff), 99)
		o.SetByte(addr(gen+generalLoyaltyOff), 5)
		for _, k := range []string{"6\r", "3\r", "1\r", fmt.Sprintf("%d\r", g)} {
			o.PressScan(k)
			if err := o.Run(playerSettle); err != nil {
				t.Fatalf("金 %d 送 %q 時停止：%v", g, k, err)
			}
		}
		t.Logf("  金 %3d → 忠誠 %3d（增幅 %3d）、郡庫 %d",
			g, o.Byte(addr(gen+generalLoyaltyOff)),
			int(o.Byte(addr(gen+generalLoyaltyOff)))-5,
			o.Word(addr(pref+18)))
	}

	// 人口那一維：每格的除數是 10 還是 12，在人口 7000 這一個點上
	// 分不開（70 ÷ 10 ＝ 7、70 ÷ 12 ＝ 5，量到的是 7，但「人口 ÷ 1000」
	// 也是 7）。換幾個人口再量才定得下來。
	t.Log("開倉賑民（魅力 99、米 100、民心起點 5，掃人口）：")
	for _, pop := range []int{30, 50, 84, 100, 120, 144, 200} {
		o.Restore(snap)
		o.SetWord(addr(pref+18), 9000)
		o.SetWord(addr(pref+20), 9000)
		o.SetWord(addr(pref+14), uint16(pop)) // 人口 ÷ 100
		o.SetByte(addr(gen+generalCharmOff), 99)
		o.SetByte(addr(pref+26), 5)
		for _, k := range []string{"5\r", "3\r", "100\r"} {
			o.PressScan(k)
			if err := o.Run(playerSettle); err != nil {
				t.Fatalf("人口 %d00 送 %q 時停止：%v", pop, k, err)
			}
		}
		t.Logf("  人口 %d00 → 民心 %3d（增幅 %3d）",
			pop, o.Byte(addr(pref+26)), int(o.Byte(addr(pref+26)))-5)
	}

	t.Log("開倉賑民（魅力 99、民心起點 5，掃米）：")
	for _, r := range []int{10, 25, 50, 75, 100, 125, 150, 175, 200, 300} {
		o.Restore(snap)
		o.SetWord(addr(pref+18), 9000)
		o.SetWord(addr(pref+20), 9000)
		o.SetByte(addr(gen+generalCharmOff), 99)
		o.SetByte(addr(pref+26), 5)
		for _, k := range []string{"5\r", "3\r", fmt.Sprintf("%d\r", r)} {
			o.PressScan(k)
			if err := o.Run(playerSettle); err != nil {
				t.Fatalf("米 %d 送 %q 時停止：%v", r, k, err)
			}
		}
		t.Logf("  米 %3d → 民心 %3d（增幅 %3d）、郡庫 %d",
			r, o.Byte(addr(pref+26)), int(o.Byte(addr(pref+26)))-5,
			o.Word(addr(pref+20)))
	}
}
