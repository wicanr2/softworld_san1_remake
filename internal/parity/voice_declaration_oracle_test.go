//go:build oracle

package parity

import (
	"fmt"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/wicanr2/dosgolem/oracle"
)

// TestVoiceDeclarationMappingPlus 只驗 ASV.EXE 的兩則宣戰對白。
// 位址均為執行期線性位址，bytes、版本及原始收據見 docs/re/09 §9。
// 原版由 TestZZDosgolemMatchesDosbox 的 rec10 分支另驗，不共用位址。
func TestVoiceDeclarationMappingPlus(t *testing.T) {
	root := plusRoot(t)
	o, err := oracle.Load(filepath.Join(root, "ASV.EXE"), root)
	if err != nil {
		t.Fatal(err)
	}
	defer o.Close()
	const seed = uint32(0x13579bdf)
	setSeed := func() {
		ds := uint32(o.DSReg()) * 16
		o.SetWord(addr(ds+0xa566), uint16(seed&0xffff))
		o.SetWord(addr(ds+0xa568), uint16(seed>>16))
	}
	var seededAt uint64
	o.OnCall(addr(plusRndFn), func(o *oracle.Oracle) {
		if seededAt == 0 {
			setSeed()
			seededAt = o.Steps()
		}
	})
	bootToNewGamePlus(t, o, 1, 5)
	setSeed()
	const load, speak = uint32(0x59d8), uint32(0x5ad6)
	if got := fmt.Sprintf("%x", o.Bytes(addr(load), 24)); got != "558becb802009a1e05b905837e0a007c06837e0a037e0fff" {
		t.Fatalf("加強版載入器 bytes 不符：%s", got)
	}
	type clipLoad struct {
		Slot int
		Name string
	}
	var loads []clipLoad
	var inputs, shown int
	o.OnCall(addr(plusNumInputFn), func(*oracle.Oracle) { inputs++ })
	o.OnCall(addr(plusKeyInputFn), func(*oracle.Oracle) { inputs++ })
	o.OnCall(addr(load), func(o *oracle.Oracle) {
		name := cstr(o, uint32(o.Arg(1))*16+uint32(o.Arg(0)))
		if len(name) > 0 && name[0] == 'R' {
			loads = append(loads, clipLoad{int(o.Arg(2)), name})
			t.Logf("load 線性 %#x step=%d slot=%d name=%s", load, o.Steps(), o.Arg(2), name)
		}
	})
	o.OnCall(addr(speak), func(o *oracle.Oracle) {
		if o.Arg(0) == 1 {
			shown++
			dumpScreen(t, o, fmt.Sprintf("voice-declare-plus-%d", shown))
		}
	})
	ds := uint32(o.DSReg()) * 16
	ss, vs := o.Word(addr(ds+0xa71a)), o.Word(addr(ds+0xac82))
	if ss == 0 || vs == 0 {
		t.Fatalf("聲音開關段未初始化：%x/%x", ss, vs)
	}
	o.SetWord(oracle.Addr{Seg: ss, Off: 0x31b6}, 0)
	o.SetWord(oracle.Addr{Seg: vs, Off: 0x3154}, 0)
	for _, k := range []string{"2\r", "2\r", "8\r"} {
		before := inputs
		o.Drain()
		o.PressScan(k)
		waitBoot(t, o, fmt.Sprintf("宣戰指令 %q", k), 500_000_000, func() bool { return inputs > before })
		waitPlusScan(t, o, "宣戰輸入", 5_000_000)
	}
	o.Drain()
	o.PressScan("7\r")
	waitBoot(t, o, "兩則宣戰的播放入口", 500_000_000, func() bool { return shown == 2 })
	want := []clipLoad{{1, "R032.OKR"}, {2, "R456.OKR"}, {3, "R499.OKR"}, {1, "R000.OKR"}, {2, "R457.OKR"}, {3, "R499.OKR"}}
	if !reflect.DeepEqual(loads, want) {
		t.Fatalf("語音載入 %v，預期 %v", loads, want)
	}
	if seededAt == 0 {
		t.Fatal("固定種子的 RND hook 未命中")
	}
	t.Logf("ASV.EXE SHA-256=%s seed=%#x 首次RND step=%d shown=%d；只注入聲音開關與明示seed", sha256File(t, filepath.Join(root, "ASV.EXE")), seed, seededAt, shown)
}
