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

	// 兩個開關在資料段的位移（`docs/re/08` §1）。0 ＝ 開啟、1 ＝ 關閉。
	sfxStateOff   = 0x31aa // 音效狀態
	voiceStateOff = 0x17c2 // 語音狀態
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

	var plays, isr int
	o.OnCall(addr(speechPlayAddr), func(*oracle.Oracle) { plays++ })
	o.OnCall(addr(speechISRAddr), func(*oracle.Oracle) { isr++ })

	bootToMain(t, o, seedMas)
	dumpScreen(t, o, "speech-00-main")

	// **開關直接寫記憶體，不走選單。** 「其他」那一層的子選單讀的是
	// 另一條輸入路徑（`docs/re/02` §3）——送 `8\r` 進去畫面不動，
	// 而畫面不動看起來像選項沒反應。兩個旗標的位置與語意在
	// `docs/re/08` §1：**0 ＝ 開啟、1 ＝ 關閉**。
	es := o.ES()
	o.SetWord(oracle.Addr{Seg: es, Off: sfxStateOff}, 0)
	o.SetWord(oracle.Addr{Seg: es, Off: voiceStateOff}, 0)
	t.Logf("音效狀態 ← 0（開啟）、語音狀態 ← 0（開啟），段 %04X", es)

	// 開關打開之後跑一段，讓可能的語音播出來。
	const settle = 40_000_000
	if err := o.Run(settle * 12); err != nil {
		t.Fatalf("等語音時停止：%v", err)
	}
	dumpScreen(t, o, "speech-01-after")

	sp := o.Speaker()
	t.Logf("結果：喇叭切換 %d 次、8253 通道 0 分頻值 %d（%.0f Hz）、"+
		"播放器進去 %d 次、ISR 跑 %d 次、中斷間隔被夾 %d 次",
		len(sp), o.PITDivisor(), o.PITHz(), plays, isr, o.IRQ0Clamped())

	if plays == 0 {
		t.Skipf("播放器一次都沒被呼叫——這一段沒有語音。" +
			"**這不是擷取沒接上**：整支播放器與 `speak(編號, 速度)` 的" +
			"介面解在 `docs/re/09`，缺的是「什麼情況下才會講話」，" +
			"不是攔截點。兩個開關已經直接寫成開啟了")
	}
	if len(sp) == 0 {
		t.Fatal("播放器被呼叫了，但喇叭一次都沒動——擷取沒接上")
	}
	if d := o.PITDivisor(); d < 1 || d > 30000 {
		t.Errorf("分頻值 %d 不在說明書講的 1–30000", d)
	}
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
