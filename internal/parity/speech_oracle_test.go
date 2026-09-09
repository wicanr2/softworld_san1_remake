//go:build oracle

package parity

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/wicanr2/dosgolem/oracle"

	"github.com/wicanr2/softworld_san1_remake/internal/state"
)

// 原版的 PC 喇叭語音（`docs/reference/02-web-10-gameplay` §10）。
//
// 播放器在主程式的 `0x5C4:0085`（線性 `0x5cc5`）：它把 8253 通道 0 重設成
// `cs:[0x1e]` 那個分頻值——說明書講的「語音速度 1–30000」就是它——
// 再把 `int 08h` 的向量指到 `0x5C4:00D7`，那支 ISR 每次把一個位元送到
// 埠 `0x61` 的 bit1。八個位元一個位元組，走到 `cs:[0x34]` 為止。
//
// dosgolem 這一側要的是 `docs/spec/016`（PIT 通道 0 ＋ 喇叭擷取）。
//
// ⚠ **語音預設關閉**（說明書 p.26），而且關掉音效之後語音也不能用。
// 所以這一支先從「其他」把兩個開關打開，再看喇叭有沒有動。
// 它現在是探索用的：把量到的東西記下來，不斷言原版怎麼決定。
const (
	speechPlayAddr = 0x5cc5 // 05C4:0085 開始播放
	speechISRAddr  = 0x5d17 // 05C4:00D7 每次 IRQ0 送一個位元
	speechSpeakFn  = 0x5b80 // speak(索引 0–3, 速度)，門是音效狀態
	speechEntryB   = 0x5deb // 05C4:01AB 另一個播放入口（軟體延遲迴圈，不動 PIT）
	speechSayFn    = 0x3273e // 顯示訊息 ＋ 講話，111 個呼叫端

	// 兩個開關在工作段的位移（`docs/re/09` §7）。**0 ＝ 開啟、1 ＝ 關閉。**
	sfxStateOff   = 0x31aa // 音效狀態
	voiceStateOff = 0x3148 // 語音狀態
)

func TestSpeechDrivesTheSpeaker(t *testing.T) {
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

	var plays, isr, speaks, says int
	o.OnCall(addr(speechPlayAddr), func(*oracle.Oracle) { plays++ })
	o.OnCall(addr(speechISRAddr), func(*oracle.Oracle) { isr++ })
	o.OnCall(addr(speechSpeakFn), func(*oracle.Oracle) { speaks++ })
	o.OnCall(addr(speechSayFn), func(*oracle.Oracle) { says++ })
	var entryB int
	o.OnCall(addr(speechEntryB), func(*oracle.Oracle) { entryB++ })

	base := bootToMain(t, o, seedMas)
	_ = base
	dumpScreen(t, o, "speech-00-main")

	// **開關直接寫記憶體，不走選單。**「其他」的子選單走的是另一條輸入
	// 路徑（`docs/re/02` §3），送數字進去畫面不動。
	//
	// ⚠ 兩個都要開：`speak` 那一層的門是**音效狀態**，語音那三段的門才是
	// 語音狀態——說明書的「關閉音效，語音將無法使用」就是這個結構。
	work := o.ES()
	o.SetWord(oracle.Addr{Seg: work, Off: sfxStateOff}, 0)
	o.SetWord(oracle.Addr{Seg: work, Off: voiceStateOff}, 0)
	t.Logf("音效狀態 ← 0（開啟）、語音狀態 ← 0（開啟），工作段 %04X", work)

	// 讓遊戲吐訊息：內政（4）→ 休息（4）→ Y 會推一個月，路上會走訊息常式。
	const settle = 40_000_000
	for _, k := range []string{"4\r", "4\r", "Y"} {
		o.Drain()
		o.PressScan(k)
		if err := o.Run(settle * 3); err != nil {
			t.Fatalf("送 %q 時停止：%v", k, err)
		}
	}
	dumpScreen(t, o, "speech-01-after")
	t.Logf("訊息常式進去 %d 次、speak 進去 %d 次", says, speaks)

	sp := o.Speaker()
	t.Logf("結果：喇叭切換 %d 次、8253 通道 0 分頻值 %d（%.0f Hz）、"+
		"訊息 %d 次、speak %d 次、入口 B %d 次、IRQ0 播放器 %d 次、"+
		"ISR %d 次、中斷間隔被夾 %d 次",
		len(sp), o.PITDivisor(), o.PITHz(), says, speaks, entryB, plays, isr,
		o.IRQ0Clamped())

	// **判準是喇叭真的動了**，不是某一支常式進去幾次：這一款有兩條播放路，
	// 一條用 IRQ0（會重設 8253 通道 0），一條是軟體延遲迴圈（不動 PIT）。
	// 只盯 PIT 或只盯 IRQ0 那一支，第二條路會被當成「沒出聲」。
	if speaks == 0 {
		t.Fatal("speak() 一次都沒被呼叫——音效那道門沒開，判準不成立")
	}
	if len(sp) == 0 {
		t.Fatal("speak() 被呼叫了，但埠 0x61 一次都沒動——擷取沒接上")
	}
	// 波形要有兩個位準，否則等於一條直線。
	var lo, hi int
	for _, x := range sp {
		if x.Level != 0 {
			hi++
		} else {
			lo++
		}
	}
	if lo == 0 || hi == 0 {
		t.Errorf("波形只有一個位準（低 %d 高 %d）", lo, hi)
	}
	t.Logf("波形跨 %d 道指令（約 %.2f 秒），高 %d 低 %d",
		sp[len(sp)-1].Step-sp[0].Step,
		float64(sp[len(sp)-1].Step-sp[0].Step)/3_004_073, hi, lo)

	if dir := os.Getenv("SAN1_SHOTS"); dir != "" {
		f, err := os.Create(filepath.Join(dir, "speech.wav"))
		if err != nil {
			t.Fatal(err)
		}
		defer f.Close()
		if err := o.SpeakerWAV(f, 11025); err != nil {
			t.Fatal(err)
		}
		t.Logf("波形 → %s", f.Name())
	}
}
