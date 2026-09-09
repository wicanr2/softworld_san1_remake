//go:build oracle

package parity

import (
	"path/filepath"
	"testing"

	"github.com/wicanr2/dosgolem/oracle"

	"github.com/wicanr2/softworld_san1_remake/internal/state"
)

// 訓練兵士**玩家那條**的幅度，對智與武掃一遍。
//
// 一個點分不出式子：曹操智 95 武 82、訓練 26 → 40（＋14），
// `(智÷3 ＋ 武÷2) ÷ 5` 與 `(智 ＋ 武) ÷ 12` 都算得出 14。
// 而且那一格是唯一在測公式的樣本——開局只有君主帶兵，其餘在職將兵力
// 是 0，兩邊都把訓練度歸零，看起來「對上了」其實什麼都沒驗到。
//
// 曹操那一局開機約九十秒，之後每一格還原重試。
func TestZZPlayerTrainSweep(t *testing.T) {
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

	nMas, nSta := state.MasterTableSize, state.PrefectureTableSize
	work := o.ES()
	at := int(o.Word(oracle.Addr{Seg: work, Off: curPrefOff}))
	const lord = 13 // 曹操；郡 11 裡唯一帶兵的
	gen := base + uint32(nMas+nSta+lord*state.GeneralRecordSize)
	t.Logf("郡 %d、將領槽號 %d", at, lord)

	snap := o.Save()
	run := func(intel, war int) int {
		o.Restore(snap)
		o.SetByte(addr(gen+9), uint8(intel))  // 智
		o.SetByte(addr(gen+10), uint8(war))   // 武
		o.SetByte(addr(gen+24), 0)            // 訓練度歸零，量的就是幅度
		for _, k := range []string{"3\r", "1\r"} {
			o.PressScan(k)
			if err := o.Run(playerSettle); err != nil {
				t.Fatalf("智 %d 武 %d 送 %q 時停止：%v", intel, war, k, err)
			}
		}
		return int(o.Byte(addr(gen + 24)))
	}

	t.Log("訓練兵士（訓練度從 0 起）：")
	for _, c := range []struct{ intel, war int }{
		{95, 82}, {0, 0}, {30, 0}, {60, 0}, {90, 0}, {99, 0},
		{0, 30}, {0, 60}, {0, 90}, {0, 99},
		{30, 30}, {60, 60}, {99, 99}, {12, 88}, {88, 12},
	} {
		t.Logf("  智 %2d 武 %2d → 訓練度 %d", c.intel, c.war, run(c.intel, c.war))
	}
}
