package music

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/wicanr2/softworld_san1_remake/internal/assets"
)

// data1 開原版的 DATA1 容器；沒有素材就 skip。
// **本儲存庫不含原版檔案。**
func data1(t *testing.T) map[string][]byte {
	t.Helper()
	root := os.Getenv("SAN1_ORIG")
	if root == "" {
		t.Skip("沒設 SAN1_ORIG，跳過（本儲存庫不含原版檔案）")
	}
	dir := filepath.Join(root, "三國演義")
	read := func(ext string) []byte {
		b, err := os.ReadFile(filepath.Join(dir, "DATA1."+ext))
		if err != nil {
			t.Skipf("讀不到 DATA1.%s：%v", ext, err)
		}
		return b
	}
	c, err := assets.OpenContainer(read("NAM"), read("IDX"), read("GRP"))
	if err != nil {
		t.Fatal(err)
	}
	out := map[string][]byte{}
	for i := 0; i < c.Len(); i++ {
		out[c.Entry(i).Name] = c.Data(i)
	}
	return out
}

// TestParseAllSongs 釘住六首曲子全部解得開。
//
// 解析器的自我檢查是**事件數要與表頭吻合**：走完事件流卻數不對，
// 表示版面讀錯了——而一個讀錯版面的解析器照樣會吐出一串看似合理的音符。
func TestParseAllSongs(t *testing.T) {
	d := data1(t)
	for _, c := range []struct {
		idx, grp string
		want     int
	}{
		{SongIndex, SongData, 5},
	} {
		tracks, err := ParseAll(d[c.idx], d[c.grp])
		if err != nil {
			t.Fatalf("%s：%v", c.grp, err)
		}
		if len(tracks) != c.want {
			t.Errorf("%s 解出 %d 首，應該是 %d 首", c.grp, len(tracks), c.want)
		}
		for i, tr := range tracks {
			if len(tr.Song.Events) == 0 {
				t.Errorf("%s 第 %d 首沒有事件", c.grp, i)
			}
			if tr.Song.Tempo < 40 || tr.Song.Tempo > 240 {
				t.Errorf("%s 第 %d 首的速度是 %d BPM", c.grp, i, tr.Song.Tempo)
			}
			if d := tr.Song.Duration(); d < 5 || d > 600 {
				t.Errorf("%s 第 %d 首長 %.1f 秒", c.grp, i, d)
			}
			if len(tr.Bank.Instruments) == 0 {
				t.Errorf("%s 第 %d 首沒有音色", c.grp, i)
			}
			t.Logf("%s #%d：%d 事件、%d BPM、%.1f 秒、%d 個音色",
				c.grp, i, len(tr.Song.Events), tr.Song.Tempo,
				tr.Song.Duration(), len(tr.Bank.Instruments))
		}
	}
}

// TestSongsUseOPL2Channels 釘住頻道編號落在 OPL2 放得下的範圍。
//
// OPL2 有兩種模式：旋律模式九個聲部（0..8），節奏模式六個旋律聲部
// （0..5）加五個打擊樂器。資料裡的用法與後者吻合——用到 6 以上的曲子
// 一定跳過 6，而且音色庫的後五個一定是 `bdrum1 snare1 tom1 cymbal1 hihat1`
// 這一組（`docs/formats/06`）。所以頻道上限是 10，不是 8。
func TestSongsUseOPL2Channels(t *testing.T) {
	d := data1(t)
	tracks, err := ParseAll(d[SongIndex], d[SongData])
	if err != nil {
		t.Fatal(err)
	}
	for i, tr := range tracks {
		used := map[int]bool{}
		for _, e := range tr.Song.Events {
			if ch := e.Channel(); ch >= 0 {
				used[ch] = true
			}
		}
		for ch := range used {
			if ch > 10 {
				t.Errorf("第 %d 首用到頻道 %d，OPL2 最多到 10（0..5 旋律 ＋ 6..10 打擊）", i, ch)
			}
		}
		// 用到 6 以上就一定是節奏模式，那時 6 本身不會出現。
		if used[7] || used[8] || used[9] || used[10] {
			if used[6] && (used[9] || used[10]) {
				t.Errorf("第 %d 首同時用到頻道 6 與 9/10——兩種模式混在一起", i)
			}
		}
		if len(used) == 0 {
			t.Errorf("第 %d 首一個頻道都沒用到", i)
		}
	}
}

// TestProgramChangesFitTheBank 釘住每個換音色事件指得到音色庫裡的一個。
//
// **這一條把兩個項目綁在一起**：曲子的 program number 與相鄰那個音色庫的
// 音色數要對得起來，才證明「偶數是曲子、奇數是它的音色庫」這個配對成立。
func TestProgramChangesFitTheBank(t *testing.T) {
	d := data1(t)
	for _, c := range [][2]string{{SongIndex, SongData}} {
		tracks, err := ParseAll(d[c[0]], d[c[1]])
		if err != nil {
			t.Fatal(err)
		}
		for i, tr := range tracks {
			n := len(tr.Bank.Instruments)
			for _, e := range tr.Song.Events {
				if e.Kind() != ProgramChange {
					continue
				}
				if p := int(e.Data[0]); p >= n {
					t.Errorf("%s 第 %d 首換到音色 %d，音色庫只有 %d 個",
						c[1], i, p, n)
				}
			}
		}
	}
}

// TestInstrumentNamesAreAdlib 釘住音色名稱是 AdLib 的標準音色名。
//
// 名稱欄位讀錯的話會是一堆亂碼；讀對的話是 `piano1`、`oboe1`、
// `bdrum1` 這種一眼認得出來的東西。
func TestInstrumentNamesAreAdlib(t *testing.T) {
	d := data1(t)
	tracks, err := ParseAll(d[SongIndex], d[SongData])
	if err != nil {
		t.Fatal(err)
	}
	seen := map[string]bool{}
	for _, tr := range tracks {
		for _, in := range tr.Bank.Instruments {
			if in.Name == "" {
				t.Error("有音色沒有名字")
				continue
			}
			for _, r := range in.Name {
				if r < 0x20 || r > 0x7E {
					t.Errorf("音色名 %q 有不可列印的字元", in.Name)
					break
				}
			}
			seen[in.Name] = true
		}
	}
	// AdLib 的標準音色庫裡本來就有這幾個。
	for _, want := range []string{"piano1", "oboe1", "bdrum1", "snare1"} {
		if !seen[want] {
			t.Errorf("五首曲子裡沒有用到 %q——名稱欄位可能讀錯了", want)
		}
	}
}

// TestBadDataIsRejected 釘住讀錯的東西要報錯，不要硬解。
func TestBadDataIsRejected(t *testing.T) {
	if _, err := ParseSong(make([]byte, 8)); err == nil {
		t.Error("八個位元組也解得出曲子")
	}
	junk := make([]byte, 200)
	if _, err := ParseSong(junk); err == nil {
		t.Error("全零的資料解得出曲子——固定值沒有擋住")
	}
	if _, err := ParseBank(junk); err == nil {
		t.Error("全零的資料解得出音色庫")
	}
	if _, err := split([]byte{1, 2, 3}, nil); err == nil {
		t.Error("長度不是 4 的倍數的索引也收")
	}
}

// TestLongSongParses 釘住 `MUSV` 那一首長曲解得乾淨。
//
// 它是唯一用得到兩件事的曲子：**時間差超過 127**，以及**即時訊息插在
// 事件中間**。五首短曲兩件都沒有，所以短曲全部吻合不代表解析器對——
// 這一條才擋得住那兩個坑（`docs/formats/06` §3）。
func TestLongSongParses(t *testing.T) {
	d := data1(t)
	tracks, err := ParseAll(d[LongIndex], d[LongData])
	if err != nil {
		t.Fatal(err)
	}
	if len(tracks) != 1 {
		t.Fatalf("MUSV 解出 %d 首，應該是 1 首", len(tracks))
	}
	s := tracks[0].Song
	t.Logf("MUSV：%d 事件、%d BPM、%.1f 秒、%d 個音色",
		len(s.Events), s.Tempo, s.Duration(), len(tracks[0].Bank.Instruments))

	// 時間差要有超過 127 的，否則這一條擋不住「照可變長度讀」那個錯法。
	big, prev := 0, 0
	for _, e := range s.Events {
		if e.At-prev > 127 {
			big++
		}
		prev = e.At
	}
	if big == 0 {
		t.Error("沒有超過 127 的時間差——這一條就擋不住可變長度那個讀法了")
	}
	// 即時訊息要有，而且要有插在事件中間的（不在開頭也不在結尾）。
	rt := 0
	for i, e := range s.Events {
		if e.Status >= 0xF8 && i > 0 && i < len(s.Events)-1 {
			rt++
		}
	}
	if rt == 0 {
		t.Error("沒有插在中間的即時訊息——這一條就擋不住「每個事件都有時間差」那個讀法了")
	}
	t.Logf("  超過 127 的時間差 %d 個、插在中間的即時訊息 %d 個", big, rt)

	// 最後一個事件不能超過表頭說的全曲長度。
	if last := s.Events[len(s.Events)-1].At; last > s.Ticks {
		t.Errorf("最後一個事件在 %d tick，表頭說全曲 %d tick", last, s.Ticks)
	}
}

// TestMIDIExportRoundTrips 釘住匯出的 MIDI 檔還原得回同一批音符。
//
// 匯出是**解析對不對的驗收方式**：解錯的資料播出來不會是音樂。
// 這裡不播，改成把檔案再讀一遍，比對音符事件。
func TestMIDIExportRoundTrips(t *testing.T) {
	d := data1(t)
	tracks, err := ParseAll(d[SongIndex], d[SongData])
	if err != nil {
		t.Fatal(err)
	}
	for i, tr := range tracks {
		blob := tr.Song.MIDI()
		if string(blob[:4]) != "MThd" {
			t.Fatalf("第 %d 首不是以 MThd 開頭", i)
		}
		// MThd 是 4（標記）＋ 4（長度）＋ 6（內容）＝ 14 個位元組。
		if string(blob[14:18]) != "MTrk" {
			t.Fatalf("第 %d 首在位移 14 不是 MTrk，而是 %q", i, blob[14:18])
		}
		if div := int(blob[12])<<8 | int(blob[13]); div != TicksPerBeat {
			t.Errorf("第 %d 首的 division 是 %d，應該是 %d", i, div, TicksPerBeat)
		}
		// MThd 之後：14 bytes 表頭 ＋ MTrk 的 4 bytes 標記 ＋ 4 bytes 長度
		n := int(blob[18])<<24 | int(blob[19])<<16 | int(blob[20])<<8 | int(blob[21])
		if 22+n != len(blob) {
			t.Fatalf("第 %d 首的 MTrk 說有 %d 個位元組，檔案有 %d", i, n, len(blob)-22)
		}
		body := blob[22:]
		// 最後三個位元組是 end of track。
		if string(body[len(body)-3:]) != "\xff\x2f\x00" {
			t.Errorf("第 %d 首沒有 end of track", i)
		}
		// 音符數要一樣。
		want := 0
		for _, e := range tr.Song.Events {
			if e.Kind() == NoteOn || e.Kind() == NoteOff {
				want++
			}
		}
		got := countNotes(t, body)
		if got != want {
			t.Errorf("第 %d 首匯出後有 %d 個音符事件，原本 %d 個", i, got, want)
		}
		if want == 0 {
			t.Errorf("第 %d 首一個音符都沒有", i)
		}
	}
}

// countNotes 走一遍 MIDI track，數音符事件。
func countNotes(t *testing.T, b []byte) int {
	t.Helper()
	n, i := 0, 0
	var running byte
	for i < len(b) {
		_, sz, err := varLen(b[i:])
		if err != nil {
			t.Fatalf("時間差解不開：%v", err)
		}
		i += sz
		if i >= len(b) {
			break
		}
		st := b[i]
		if st&0x80 != 0 {
			i++
			if st < 0xF0 {
				running = st
			}
		} else {
			st = running
		}
		switch {
		case st == 0xFF: // meta
			i++ // type
			ln, sz, _ := varLen(b[i:])
			i += sz + ln
		case st == 0xF0:
			ln, sz, _ := varLen(b[i:])
			i += sz + ln
		default:
			size, err := eventSize(st)
			if err != nil {
				t.Fatalf("位移 %d 的狀態 %#02x：%v", i, st, err)
			}
			if st&0xF0 == NoteOn || st&0xF0 == NoteOff {
				n++
			}
			i += size
		}
	}
	return n
}

// TestVarBytes 釘住可變長度數值寫得回去也讀得回來。
func TestVarBytes(t *testing.T) {
	for _, v := range []int{0, 1, 127, 128, 255, 8192, 16383, 100000} {
		b := varBytes(v)
		got, n, err := varLen(b)
		if err != nil {
			t.Fatalf("%d 寫成 % X 之後讀不回來：%v", v, b, err)
		}
		if got != v || n != len(b) {
			t.Errorf("%d 寫成 % X，讀回來是 %d（吃了 %d 個位元組）", v, b, got, n)
		}
	}
}

// TestInstrumentFieldsAreInRange 釘住解出來的音色參數落在 OPL2 的值域。
//
// 版面（`docs/formats/06` §5）是拿原版填進 OPL2 的暫存器值比對出來的。
// 值域檢查擋的是另一件事：**欄位挪一格仍然解得出「參數」**，
// 只是那些數字會爆掉值域——十六種音色裡只要有一個 MULT 大於 15
// 就表示版面錯了。
func TestInstrumentFieldsAreInRange(t *testing.T) {
	d := data1(t)
	tracks, err := ParseAll(d[SongIndex], d[SongData])
	if err != nil {
		t.Fatal(err)
	}
	n, perc := 0, 0
	for _, tr := range tracks {
		for _, in := range tr.Bank.Instruments {
			// 單運算子的節奏音色，載波那一半是別的資料，不必落在值域裡。
			if in.Percussive() {
				perc++
				continue
			}
			for _, c := range []struct {
				tag string
				op  Operator
			}{{"調變", in.Modulator()}, {"載波", in.Carrier()}} {
				op := c.op
				for _, f := range []struct {
					name  string
					v, hi int
				}{
					{"KSL", op.KSL, 3}, {"MULT", op.Mult, 15}, {"FB", op.FB, 7},
					{"AR", op.AR, 15}, {"SL", op.SL, 15}, {"EG", op.EG, 1},
					{"DR", op.DR, 15}, {"RR", op.RR, 15}, {"TL", op.TL, 63},
					{"AM", op.AM, 1}, {"VIB", op.VIB, 1}, {"KSR", op.KSR, 1},
					{"Wave", op.Wave, 3},
				} {
					if f.v < 0 || f.v > f.hi {
						t.Errorf("%s 的%s %s ＝ %d，值域是 0..%d",
							in.Name, c.tag, f.name, f.v, f.hi)
					}
				}
			}
			n++
		}
	}
	if n == 0 {
		t.Fatal("一件音色都沒有")
	}
	t.Logf("%d 件音色的參數全部落在值域裡，另有 %d 件是單運算子", n, perc)
}

// TestRegistersRoundTrip 釘住參數組回暫存器再拆開是同一組數字。
func TestRegistersRoundTrip(t *testing.T) {
	op := Operator{KSL: 2, Mult: 9, FB: 5, AR: 14, SL: 7, EG: 1,
		DR: 3, RR: 11, TL: 42, AM: 1, VIB: 0, KSR: 1, Wave: 2}
	r20, r40, r60, r80, rE0 := op.Registers()
	got := Operator{
		AM: int(r20 >> 7 & 1), VIB: int(r20 >> 6 & 1), EG: int(r20 >> 5 & 1),
		KSR: int(r20 >> 4 & 1), Mult: int(r20 & 15),
		KSL: int(r40 >> 6), TL: int(r40 & 63),
		AR: int(r60 >> 4), DR: int(r60 & 15),
		SL: int(r80 >> 4), RR: int(r80 & 15),
		Wave: int(rE0 & 3), FB: op.FB,
	}
	if got != op {
		t.Errorf("組回暫存器再拆開變成 %+v，原本是 %+v", got, op)
	}
}

// TestCarrierHasNoFeedback 釘住載波的回授固定是 0。
//
// OPL2 的回授只作用在調變器上；音色庫在載波那一格留的是垃圾值
// （`piano1` 是 0x40F6、`bdrum1` 是 0x102F），照抄會讓合成出來的聲音
// 完全不對。
func TestCarrierHasNoFeedback(t *testing.T) {
	d := data1(t)
	tracks, err := ParseAll(d[SongIndex], d[SongData])
	if err != nil {
		t.Fatal(err)
	}
	junk := 0
	for _, tr := range tracks {
		for _, in := range tr.Bank.Instruments {
			if in.Carrier().FB != 0 {
				t.Errorf("%s 的載波回授不是 0", in.Name)
			}
			if in.Param(carrierBase+fieldFB) > 7 {
				junk++
			}
		}
	}
	if junk == 0 {
		t.Error("沒有任何音色在載波的 FB 那一格留下超出值域的值——" +
			"那一格本來就該是垃圾，一個都沒有反而可疑")
	}
}

// TestConnectionIsInverted 釘住 CON 的反相。
//
// 音色庫寫 1 代表調頻，OPL2 的 `0xC0` 位元 0 寫 1 代表相加。照抄會把
// 兩個運算子從串接變成並聯，聽起來完全是另一件樂器。
func TestConnectionIsInverted(t *testing.T) {
	d := data1(t)
	tracks, err := ParseAll(d[SongIndex], d[SongData])
	if err != nil {
		t.Fatal(err)
	}
	fm, additive := 0, 0
	for _, tr := range tracks {
		for _, in := range tr.Bank.Instruments {
			c := in.Connection()
			if int(c)>>1&7 != in.Modulator().FB&7 {
				t.Errorf("%s 的回授沒有進 0xC0", in.Name)
			}
			if in.Param(fieldCON)&1 == 1 {
				if c&1 != 0 {
					t.Errorf("%s 的 CON 是 1（調變），0xC0 位元 0 該是 0", in.Name)
				}
				fm++
			} else {
				if c&1 != 1 {
					t.Errorf("%s 的 CON 是 0（相加），0xC0 位元 0 該是 1", in.Name)
				}
				additive++
			}
		}
	}
	if fm == 0 || additive == 0 {
		t.Errorf("兩種連接方式要各有樣本，得到調頻 %d、相加 %d", fm, additive)
	}
}
