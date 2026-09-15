//go:build oracle

package parity

import (
	"fmt"
	"path/filepath"
	"sort"
	"strings"
	"testing"

	"github.com/wicanr2/dosgolem/oracle"

	"github.com/wicanr2/softworld_san1_remake/internal/state"
)

// 城門圖示 `WFLAG?5.IMG`（每組旗的第六張）原版什麼時候畫（Issue #32）。
//
// 靜態讀出來的是：畫圖常式 `0x36c9:0x426`（線性 `0x370b6`，參數 x、y、
// 槽號）在主程式裡算 `20 + 軍力×6 + 隊伍` 的只有 `0x21c06`（部隊標記
// `0x21b40`），而隊伍永遠是 0–4：整編、日循環、重算（`0x2514c`）與招降
// 加入（`0x25cd2`，`0x25d8c` 的 `cmp $5`）的迴圈全部到 5 為止，佔位圖存的
// 也是這些記錄的編號。槽 25／31／37／43 因此沒有任何呼叫端算得到。
//
// 這一支拿實跑當正對照：攔畫圖常式記下每一個槽號，打一場（含對戰
// 子畫面）之後，旗的槽 20–24 要出現過，25／31／37／43 一次都不能；載圖
// 常式那一邊確認四張 `WFLAG?5` 真的載進了那四個槽——**載了而沒有畫**
// 才是「死素材」的證據，只看畫沒看載會分不出「沒載」與「沒畫」。
func TestZZCityGateIconNeverDrawn(t *testing.T) {
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

	const drawFn = 0x36c9*16 + 0x426
	loaded := map[int]string{}
	o.OnCall(addr(imgLoadFn), func(o *oracle.Oracle) {
		lin := uint32(o.Arg(1))*16 + uint32(o.Arg(0))
		var b strings.Builder
		for i := uint32(0); i < 16; i++ {
			ch := o.Byte(addr(lin + i))
			if ch == 0 {
				break
			}
			b.WriteByte(ch)
		}
		if name := b.String(); strings.HasPrefix(name, "WFLAG") {
			loaded[int(int16(o.Arg(2)))] = name
		}
	})
	drawn := map[int]int{}
	o.OnCall(addr(drawFn), func(o *oracle.Oracle) {
		drawn[int(int16(o.Arg(2)))]++
	})

	base := bootToGame(t, o, seedMas)
	// 盤面丁：玩家一千三對五支各六千、敵將 5——第二天就進對戰子畫面，
	// 子畫面裡畫的東西也一起攔到。
	at, to := stageABattleWith(t, o, base, 1300, 5, 6000, 5)
	driveIntoBattle(t, o, at, to)
	for i := 0; i < 60; i++ {
		for _, k := range []string{"0", "Y"} {
			o.Drain()
			o.PressScan(k)
			if err := o.Run(60_000_000); err != nil {
				t.Fatalf("第 %d 輪送 %q 停止：%v", i+1, k, err)
			}
		}
	}

	var slots []int
	for s := range loaded {
		slots = append(slots, s)
	}
	sort.Ints(slots)
	var ls []string
	for _, s := range slots {
		ls = append(ls, fmt.Sprintf("%d=%s", s, loaded[s]))
	}
	t.Logf("載進來的旗：%s", strings.Join(ls, " "))
	for _, s := range []int{25, 31, 37, 43} {
		if !strings.HasSuffix(loaded[s], "5.IMG") {
			t.Errorf("槽 %d 沒載到 WFLAG?5（載到的是 %q）——載入端的式子與 docs/spec/005 不符", s, loaded[s])
		}
	}
	var ds []int
	for s := range drawn {
		ds = append(ds, s)
	}
	sort.Ints(ds)
	t.Logf("畫過的槽（槽=次數）：%v", func() []string {
		var out []string
		for _, s := range ds {
			out = append(out, fmt.Sprintf("%d=%d", s, drawn[s]))
		}
		return out
	}())
	flags := 0
	for s := 20; s <= 24; s++ {
		flags += drawn[s]
	}
	if flags == 0 {
		t.Fatal("旗的槽 20–24 一次都沒畫——沒進到主戰場，後面的「沒畫」不算數")
	}
	for _, s := range []int{25, 31, 37, 43} {
		if drawn[s] > 0 {
			t.Errorf("城門圖示的槽 %d 被畫了 %d 次——原版會畫它，docs/re/05 §2.5 要改", s, drawn[s])
		}
	}
}
