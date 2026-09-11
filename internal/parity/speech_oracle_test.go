//go:build oracle

package parity

import (
	"crypto/sha256"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"strings"
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

// 語音／音效的緩衝槽：誰載進去、播出去的位元組是不是素材本身。
//
// `docs/re/09` 解出播放器與波形格式之後還剩一個缺口：**播出去的到底是
// 哪一份資料**。這一支把它釘死——攔載入器與播放入口，把播放入口拿到的
// 遠指標指向的位元組倒出來，跟容器裡的素材逐位元組比。
//
// 載入器 `loadClip(char far *name, int slot)`（線性 `0x5a76`，`L0`）：
//
//	slot 不在 0–3      → "Invalid SND"
//	slot 0 且 size>1000 → "Real#0>1000"
//	size > 0x1068(4200) → "Real#1-3>4200"
//	讀進 es:[0x878+slot*4]（遠指標），長度寫 es:[0x3c8c+slot*2]
//
// 所以**槽 0 是音效、槽 1–3 是語音**，兩者共用同一支播放器。
const speechLoadFn = 0x5a76 // loadClip(遠檔名, 槽)

func TestSpeechClipsAreTheAssetBytes(t *testing.T) {
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

	type load struct {
		name string
		slot int
	}
	var loads []load
	o.OnCall(addr(speechLoadFn), func(o *oracle.Oracle) {
		loads = append(loads, load{
			name: cstr(o, oracle.Addr{Seg: o.Arg(1), Off: o.Arg(0)}.Linear()),
			slot: int(int16(o.Arg(2))),
		})
	})

	type play struct {
		n, div int
		sum    string
		first  []byte
	}
	plays := map[string]*play{}
	var order []string
	o.OnCall(addr(speechEntryB), func(o *oracle.Oracle) {
		n := int(o.Arg(2))
		b := o.Bytes(oracle.Addr{Seg: o.Arg(1), Off: o.Arg(0)}, n)
		sum := fmt.Sprintf("%x", sha256.Sum256(b))
		p := plays[sum]
		if p == nil {
			p = &play{div: int(o.Arg(3)), sum: sum, first: append([]byte(nil), b...)}
			plays[sum] = p
			order = append(order, sum)
		}
		p.n++
	})

	bootToMain(t, o, seedMas)
	work := o.ES()
	o.SetWord(oracle.Addr{Seg: work, Off: sfxStateOff}, 0)
	o.SetWord(oracle.Addr{Seg: work, Off: voiceStateOff}, 0)

	const settle = 40_000_000
	for _, k := range []string{"4\r", "4\r", "Y"} {
		o.Drain()
		o.PressScan(k)
		if err := o.Run(settle * 3); err != nil {
			t.Fatalf("送 %q 時停止：%v", k, err)
		}
	}

	t.Logf("載入器進去 %d 次：", len(loads))
	for _, l := range loads {
		t.Logf("  槽 %d ← %s", l.slot, l.name)
	}
	if len(loads) == 0 {
		t.Fatal("載入器一次都沒進去——攔錯地方（`CLAUDE.md` §7 第 21 條）")
	}

	// 素材那一側：把容器裡每一份 `.SND`／`.OKR` 的雜湊算出來，
	// 反查播出去的位元組是哪一份。
	want := map[string]string{}
	for _, base := range []string{"DATA1", "DATA2", "DATA3"} {
		cc := openContainer(t, filepath.Join(root, base))
		for i := 0; i < cc.Len(); i++ {
			name := cc.Entry(i).Name
			u := strings.ToUpper(name)
			if !strings.HasSuffix(u, ".SND") && !strings.HasSuffix(u, ".OKR") {
				continue
			}
			want[fmt.Sprintf("%x", sha256.Sum256(cc.Data(i)))] = base + "／" + name
		}
	}

	t.Logf("播放入口拿到 %d 份不同的資料：", len(order))
	matched := 0
	for _, sum := range order {
		p := plays[sum]
		src, ok := want[sum]
		if ok {
			matched++
		} else {
			src = "（不是任何一份素材）"
		}
		t.Logf("  %d 個位元組、分頻 %d、播 %d 次　%s　%s…",
			len(p.first), p.div, p.n, src, sum[:16])
	}
	if len(order) == 0 {
		t.Fatal("播放入口一次都沒拿到資料")
	}
	if matched != len(order) {
		t.Errorf("%d 份裡只有 %d 份對得上素材——"+
			"播出去的不是容器裡的位元組，格式的結論不成立", len(order), matched)
	}
}

// 播出去的位元序列，逐段對回素材的位元。
//
// 上一支證明了「播放入口拿到的位元組 ＝ `S000.SND` 的 128 個位元組」。
// 這一支再往下一層：**喇叭實際切換的序列**是不是那些位元組的位元。
//
// 做法：喇叭擷取只記變化（`oracle.Speaker`），所以量到的是**每一段
// 同值持續幾個位元**。把相鄰兩次變化的指令差除以「一個位元幾道指令」
// 就換回位元數，再跟素材的位元跑比。
//
// ⚠ **兩個候選一定要一起比**：位元組 0 起跳與位元組 1 起跳的位元跑
// 高度相似（只差開頭），只驗一個會自我實現。
func TestSpeechBitStreamMatchesTheClip(t *testing.T) {
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

	// 播放器模組自己的變數（`docs/re/09` §1），基底是段 `05C4`。
	const mod = 0x5c40
	type snap struct {
		step             uint64
		clip             []byte
		cur28, cur36     uint16
		ptr30, end34     uint16
		div              uint16
	}
	var shots []snap
	o.OnCall(addr(speechEntryB), func(o *oracle.Oracle) {
		shots = append(shots, snap{
			step:  o.Steps(),
			clip:  o.Bytes(oracle.Addr{Seg: o.Arg(1), Off: o.Arg(0)}, int(o.Arg(2))),
			cur28: o.Word(addr(mod + 0x28)),
			cur36: o.Word(addr(mod + 0x36)),
			ptr30: o.Word(addr(mod + 0x30)),
			end34: o.Word(addr(mod + 0x34)),
			div:   o.Arg(3),
		})
	})

	bootToMain(t, o, seedMas)
	work := o.ES()
	o.SetWord(oracle.Addr{Seg: work, Off: sfxStateOff}, 0)
	o.SetWord(oracle.Addr{Seg: work, Off: voiceStateOff}, 0)

	// 與 `TestSpeechClipsAreTheAssetBytes` 同一串鍵：出聲的是第三步。
	for _, k := range []string{"4\r", "4\r", "Y"} {
		o.Drain()
		o.PressScan(k)
		if err := o.Run(120_000_000); err != nil {
			t.Fatalf("送 %q 時停止：%v", k, err)
		}
		if len(shots) >= 3 {
			break
		}
	}
	if len(shots) < 3 {
		t.Fatalf("播放入口只進去 %d 次，湊不出一整段", len(shots))
	}

	// 取第二段：第一段的起始狀態可能還沒穩定。
	s := shots[1]
	t.Logf("第 2 段：%d 個位元組、分頻 %d、進入時 cs:[0x28]=%04X cs:[0x36]=%04X "+
		"cs:[0x30]=%04X cs:[0x34]=%04X",
		len(s.clip), s.div, s.cur28, s.cur36, s.ptr30, s.end34)

	sp := o.Speaker()
	var seg []oracle.SpeakerSample
	for _, x := range sp {
		if x.Step >= s.step && x.Step < shots[2].step {
			seg = append(seg, x)
		}
	}
	t.Logf("這一段喇叭切換 %d 次", len(seg))
	if len(seg) < 8 {
		t.Fatalf("切換太少（%d），量不出位元跑", len(seg))
	}

	// 量到的是「相鄰兩次切換差幾道指令」。要換回位元數就要知道一個位元
	// 幾道指令——**不要拿最小間隔當它**：那會系統性低估，長段就多算一格
	//（實測 36 個位元的一段會量成 37）。用「總指令數 ÷ 候選的總位元數」
	// 現配，兩個候選各配各的，比較才公平。
	deltas := make([]float64, 0, len(seg))
	for i := 1; i < len(seg); i++ {
		deltas = append(deltas, float64(seg[i].Step-seg[i-1].Step))
	}

	// 素材那一側的位元跑。
	//
	// ⚠ **第一段要不要算，看它跟殘留位準一不一樣**：喇叭只記變化，
	// 所以與殘留位準相同的開頭根本不會產生第一次切換，那一段量不到；
	// 不同的話第一個位元就是一次切換，那一段要算。殘留位準 ＝
	// 第一次切換**之後**的反面。一律砍掉第一段的話，剛好把
	// 「跳過首位元組」那個候選砍歪（它的開頭是 1，與殘留的 0 不同）。
	rest := 1 - int(seg[0].Level&1)
	runsFrom := func(bits []int) []float64 {
		var out []float64
		prev, n := -1, 0
		for _, b := range bits {
			if prev < 0 || b == prev {
				prev, n = b, n+1
				continue
			}
			out = append(out, float64(n))
			prev, n = b, 1
		}
		if n > 0 {
			out = append(out, float64(n))
		}
		if len(bits) > 0 && bits[0] == rest && len(out) > 0 {
			out = out[1:]
		}
		return out
	}
	bitsOf := func(b []byte) []int {
		out := make([]int, 0, len(b)*8)
		for _, by := range b {
			for k := 7; k >= 0; k-- {
				out = append(out, int(by>>uint(k))&1)
			}
		}
		return out
	}

	// 候選 A：從位元組 0 起跳，也就是「介面說什麼就播什麼」。
	// 候選 B：**跳過第一個位元組**，前面接上進入時 cs:[0x36] 高位元組
	// 那個殘留位元組——入口 B 把首位元組寫進 cs:[0x28]，而播放迴圈讀的
	// 是 cs:[0x36]（`docs/re/09` §5.1）。
	stale := byte(s.cur36 >> 8)
	cand := []struct {
		name string
		runs []float64
	}{
		{"從位元組 0 起跳", runsFrom(bitsOf(s.clip))},
		{fmt.Sprintf("跳過首位元組（殘留 %02X）", stale),
			runsFrom(append(bitsOf([]byte{stale}), bitsOf(s.clip[1:])...))},
	}

	best, bestScore := -1, -1.0
	for ci, c := range cand {
		if len(c.runs) == 0 {
			continue
		}
		// 一個位元幾道指令：讓兩邊的總長對齊。
		var sumD, sumR float64
		for i := 0; i < len(deltas) && i < len(c.runs); i++ {
			sumD += deltas[i]
			sumR += c.runs[i]
		}
		if sumR == 0 {
			continue
		}
		per := sumD / sumR
		ok := 0
		for i := 0; i < len(deltas) && i < len(c.runs); i++ {
			if math.Abs(deltas[i]-c.runs[i]*per) < per*0.6 {
				ok++
			}
		}
		score := float64(ok) / float64(len(deltas))
		t.Logf("候選「%s」：%d 段、一個位元 %.1f 道指令、對得上 %d／%d（%.1f%%）",
			c.name, len(c.runs), per, ok, len(deltas), 100*score)
		if score > bestScore {
			best, bestScore = ci, score
		}
	}

	if bestScore < 0.95 {
		t.Fatalf("最好的候選只對上 %.1f%%——位元序列不是素材的位元，"+
			"格式的結論不成立", 100*bestScore)
	}
	t.Logf("結論：%s（%.1f%%）", cand[best].name, 100*bestScore)
	if best == 0 {
		return
	}
	// 跳過首位元組是**原版的行為**，不是量測誤差：切換次數要接得上
	// ——量到 N 次切換就有 N−1 個間隔，而最後一段沒有收尾的切換。
	if d := len(cand[1].runs) - len(deltas); d != 1 {
		t.Errorf("跳過首位元組的候選有 %d 段，量到 %d 個間隔（差 %d，想要 1）",
			len(cand[1].runs), len(deltas), d)
	}
}
