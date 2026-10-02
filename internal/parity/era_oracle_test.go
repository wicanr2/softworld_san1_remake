//go:build oracle

package parity

import (
	"fmt"
	"path/filepath"
	"testing"

	"github.com/wicanr2/dosgolem/oracle"

	"github.com/wicanr2/softworld_san1_remake/internal/state"
)

// TestOriginalEraAfterOneYear 驗證原版從中平六年跨年後，主畫面是否改元。
//
// 這條走正常玩家路徑：劇本一選曹操，每個可操作郡都執行
// 「內政 → 休息 → Y」，直到原版自己的月底結算入口出現十二次。
// 亂數在開局完成後固定，讓每月 AI 與郡次序可以重播；不改年月或盤面。
func TestOriginalEraAfterOneYear(t *testing.T) {
	root := origRoot(t)
	c := openContainer(t, filepath.Join(root, "DATA2"))
	sc, err := state.LoadScenario(c, state.Slot("001"))
	if err != nil {
		t.Fatal(err)
	}
	mas, _, _ := sc.Tables()

	o, err := oracle.Load(filepath.Join(root, "AA.EXE"), root)
	if err != nil {
		t.Fatal(err)
	}
	defer o.Close()
	dateCalls := watchMainDate(t, o, openContainer(t, filepath.Join(root, "DATA3")))

	s := observeBoot(o)
	settlements := 0
	o.OnCall(addr(0x1581c), func(*oracle.Oracle) { settlements++ })
	bootToNewGame(t, o, caoCaoPick, mas)
	waitBoot(t, o, "新局第一個主命令輸入", 500_000_000,
		func() bool { return s.mainAsk > 0 })
	waitBootScan(t, o, "新局第一個主命令", 5_000_000)

	gameSeg := o.ES()
	dgroup := o.DSReg()
	if year, month := o.Word(oracle.Addr{Seg: gameSeg, Off: 0x3140}),
		o.Word(oracle.Addr{Seg: gameSeg, Off: 0x3f08}); year != 189 || month != 1 {
		t.Fatalf("原版開局年月=%d/%d，預期 189/1", year, month)
	}
	// `1058:21A0`／`1058:23DC` 掃這 17 筆起始年，再由同索引的
	// `DS:0x603A` 字碼對取年號兩字。190 的 0x96／0x97 對應字圖
	// 「初」「平」；189 使用前一筆 0x92／0x97「中」「平」。
	wantStarts := [...]uint16{
		184, 190, 194, 196, 220, 227, 233, 237, 240,
		249, 254, 256, 260, 264, 265, 275, 280,
	}
	for i, want := range wantStarts {
		if got := o.Word(oracle.Addr{Seg: dgroup, Off: uint16(0x6018 + i*2)}); got != want {
			t.Fatalf("原版年號起始年[%d]=%d，預期 %d", i, got, want)
		}
	}
	for i, want := range []uint16{0x92, 0x97, 0x96, 0x97} {
		if got := o.Word(oracle.Addr{Seg: dgroup, Off: uint16(0x603a + i*2)}); got != want {
			t.Fatalf("原版前兩組年號字碼 word[%d]=%#x，預期 %#x", i, got, want)
		}
	}

	const seed = uint32(0x13579BDF)
	ds := uint32(dgroup) * 16
	o.SetWord(addr(ds+0xa3ae), uint16(seed&0xffff))
	o.SetWord(addr(ds+0xa3b0), uint16(seed>>16))
	if len(*dateCalls) < 10 {
		t.Fatalf("開局日期繪圖僅命中 %d 次，預期至少 10 次", len(*dateCalls))
	}
	startCalls := append([]dateImageCall(nil), (*dateCalls)[len(*dateCalls)-10:]...)
	dumpScreen(t, o, "era-189-01")

	for settlements < 12 {
		beforeAffairs := s.affairsAsk
		o.Drain()
		o.PressScan("4\r")
		waitBoot(t, o, "內政子命令輸入", 100_000_000,
			func() bool { return s.affairsAsk > beforeAffairs })
		waitBootScan(t, o, "內政子命令", 5_000_000)

		beforeRest := s.restYN
		o.Drain()
		o.PressScan("4\r")
		waitBoot(t, o, "休息確認輸入", 100_000_000,
			func() bool { return s.restYN > beforeRest })

		beforeMain := s.mainAsk
		o.Drain()
		o.PressScan("Y")
		waitBoot(t, o, "下一個主命令輸入", 1_000_000_000,
			func() bool { return s.mainAsk > beforeMain })
		waitBootScan(t, o, "下一個主命令", 5_000_000)
		t.Logf("月底結算 %d/12；目前主命令次數 %d", settlements, s.mainAsk)
	}

	dumpScreen(t, o, "era-190-01")
	if settlements != 12 {
		t.Fatalf("月底結算次數=%d，預期 12", settlements)
	}
	if year, month := o.Word(oracle.Addr{Seg: gameSeg, Off: 0x3140}),
		o.Word(oracle.Addr{Seg: gameSeg, Off: 0x3f08}); year != 190 || month != 1 {
		t.Fatalf("跨過十二次月底後原版年月=%d/%d，預期 190/1", year, month)
	}

	// 最終畫面的日期區不能以黑色墨點讀值；比原版自己載入的 CP 圖號。
	// 十槽的實際圖片落點為 (24,65+28i)，24×24（docs/spec/005 §2.2）。
	if len(*dateCalls) < 20 {
		t.Fatalf("跨年日期繪圖僅命中 %d 次，沒有第二組十槽", len(*dateCalls))
	}
	endCalls := (*dateCalls)[len(*dateCalls)-10:]
	for i := 0; i < 10; i++ {
		a, b := startCalls[i], endCalls[i]
		wantSame := i != 0 && i != 3
		if same := a.name == b.name; same != wantSame {
			t.Errorf("日期字格 %d 跨年前後相同=%v，預期 %v", i, same, wantSame)
		}
		if a.x != 24 || a.y != 65+28*i || a.w != 24 || a.h != 24 ||
			b.x != a.x || b.y != a.y || b.w != a.w || b.h != a.h {
			t.Errorf("日期字格 %d 座標／尺寸不符：開局 %+v，跨年 %+v", i, a, b)
		}
	}
	if endCalls[0].name != "CP150.IMG" || endCalls[3].name != "CP169.IMG" {
		t.Errorf("跨年沒有改成初平元年：%+v", endCalls)
	}
	t.Logf("固定亂數 seed=%#08x；已由正常玩家路徑跨過十二次月底結算", seed)
}

func screenRect(pix []byte, x, y, w, h int) []byte {
	out := make([]byte, 0, w*h)
	for row := 0; row < h; row++ {
		lo := (y+row)*scrW + x
		if lo < 0 || lo+w > len(pix) {
			panic(fmt.Sprintf("畫面裁切越界：(%d,%d) %dx%d", x, y, w, h))
		}
		out = append(out, pix[lo:lo+w]...)
	}
	return out
}
