package music

import (
	"fmt"
	"math"
	"testing"
)

// keyOns 走一遍曲子，收下每個頻道的 key-on（block 與 F-number）。
func keyOns(s *Song, b *Bank) map[int][]string {
	p := NewPlayer(s, b)
	c := p.Chip()
	var fnum [9]int
	got := map[int][]string{}
	c.Trace = func(reg, val byte) {
		ch := int(reg & 0x0F)
		if ch >= 9 {
			return
		}
		switch {
		case reg >= 0xA0 && reg < 0xB0:
			fnum[ch] = fnum[ch]&0x300 | int(val)
		case reg >= 0xB0 && reg < 0xC0:
			fnum[ch] = fnum[ch]&0xFF | int(val&3)<<8
			if val&0x20 != 0 {
				got[ch] = append(got[ch], fmt.Sprintf("%d/%03X", val>>2&7, fnum[ch]))
			}
		}
	}
	for !p.Done() {
		p.Sample()
	}
	return got
}

// TestNotesMatchTheOriginal 拿原版自己彈出來的音高對拍。
//
// 對照組是原版在 DOS 裡播 `思古` 時送到 0x388／0x389 的埠寫入，
// 用 dosgolem 的 `-dump-ports` 錄下來再拆成 block 與 F-number
// （`docs/formats/06` §8）。這裡比的是每個聲部前十四次按鍵。
//
// 這一項同時釘住四件事：曲子解得對、聲部怎麼對到頻道、音符表的值、
// 以及滑音怎麼算。任何一件錯了音高就對不上。
func TestNotesMatchTheOriginal(t *testing.T) {
	want := map[int]string{
		0: "4/222 4/266 4/222 4/1E6 4/222 4/198 4/16C 3/28A 4/198 4/1B1 4/198 4/16C 4/1E6 4/198",
		1: "4/21F 4/262 4/21F 4/1E3 4/21F 4/196 4/169 3/286 4/196 4/1AE 4/196 4/169 4/1E3 4/196",
		2: "3/198 3/198 3/16C 3/198 3/198 3/16C 3/198 3/198 3/1B1 3/1B1 3/198 3/198 3/1B1 3/1B1",
		3: "3/222 3/222 3/1E6 3/222 3/222 3/1E6 3/222 3/222 3/222 3/222 3/1E6 3/1E6 3/222 3/222",
		4: "3/28A 3/28A 3/266 3/28A 3/28A 3/266 3/28A 3/28A 3/28A 3/28A 3/28A 3/28A 3/28A 3/28A",
		5: "3/222 3/222 3/1E6 3/222 3/222 3/1E6 3/222 3/222 3/222 3/222 3/1E6 3/1E6 3/222 3/222",
		6: "4/286 4/262 4/286 4/286 4/262 4/286 4/286 4/286 4/286 4/262 4/286 4/286 4/286 4/286",
		7: "5/196 5/169 5/196 5/196 5/169 5/196 5/1AE 5/196 5/1AE 5/169 5/196 5/196 5/1AE 5/196",
		8: "5/21F 5/1E3 5/21F 5/21F 5/1E3 5/21F 5/21F 5/1E3 5/21F 5/1E3 5/1E3 5/21F 5/21F 5/1E3",
	}
	d := data1(t)
	tracks, err := ParseAll(d[SongIndex], d[SongData])
	if err != nil {
		t.Fatal(err)
	}
	got := keyOns(tracks[0].Song, tracks[0].Bank)
	// 沒有滑音的聲部要逐項相同；有滑音的四個（1、6、7、8）容許
	// 一個 F-number 的差——原版的滑音算法還沒解出來，見 bendCents。
	bent := map[int]bool{1: true, 6: true, 7: true, 8: true}
	for ch := 0; ch < 9; ch++ {
		var g []string
		for i, v := range got[ch] {
			if i == 14 {
				break
			}
			g = append(g, v)
		}
		line := join(g)
		if line == want[ch] {
			continue
		}
		if bent[ch] && nearlySame(g, want[ch]) {
			continue
		}
		t.Errorf("第 %d 個聲部\n  得到 %s\n  原版 %s", ch, line, want[ch])
	}
}

func join(s []string) string {
	out := ""
	for i, v := range s {
		if i > 0 {
			out += " "
		}
		out += v
	}
	return out
}

// nearlySame 允許每一項的 F-number 差一。
func nearlySame(got []string, want string) bool {
	w := split2(want)
	if len(got) != len(w) {
		return false
	}
	for i := range got {
		var gb, gf, wb, wf int
		if _, err := fmt.Sscanf(got[i], "%d/%X", &gb, &gf); err != nil {
			return false
		}
		if _, err := fmt.Sscanf(w[i], "%d/%X", &wb, &wf); err != nil {
			return false
		}
		if gb != wb || gf-wf > 1 || wf-gf > 1 {
			return false
		}
	}
	return true
}

func split2(s string) []string {
	var out []string
	cur := ""
	for _, r := range s {
		if r == ' ' {
			if cur != "" {
				out = append(out, cur)
			}
			cur = ""
			continue
		}
		cur += string(r)
	}
	if cur != "" {
		out = append(out, cur)
	}
	return out
}

// TestRenderMakesSound 釘住五首都合出聲音，而且長度與曲長吻合。
func TestRenderMakesSound(t *testing.T) {
	if testing.Short() {
		t.Skip("合成整首要幾十秒")
	}
	d := data1(t)
	tracks, err := ParseAll(d[SongIndex], d[SongData])
	if err != nil {
		t.Fatal(err)
	}
	for i, tr := range tracks {
		pcm := Render(tr.Song, tr.Bank)
		if len(pcm) == 0 {
			t.Fatalf("第 %d 首合出來是空的", i)
		}
		secs := float64(len(pcm)) / OPLRate
		if d := secs - tr.Song.Duration(); d < 0 || d > 3.5 {
			t.Errorf("第 %d 首合出 %.1f 秒，曲長 %.1f 秒", i, secs, tr.Song.Duration())
		}
		var sum float64
		peak := 0
		for _, v := range pcm {
			sum += float64(v) * float64(v)
			if int(v) > peak {
				peak = int(v)
			} else if -int(v) > peak {
				peak = -int(v)
			}
		}
		rms := math.Sqrt(sum / float64(len(pcm)))
		t.Logf("第 %d 首（%s）：%.1f 秒、峰值 %d、均方根 %.0f",
			i, TrackNames()[i], secs, peak, rms)
		if rms < 200 {
			t.Errorf("第 %d 首幾乎沒有聲音（均方根 %.0f）", i, rms)
		}
		if peak >= 32767 {
			t.Errorf("第 %d 首削頂了", i)
		}
	}
}

// goertzel 回某個頻率在一段取樣裡的振幅。
func goertzel(x []float64, f float64) float64 {
	w := 2 * math.Pi * f / OPLRate
	cw, sw := math.Cos(w), math.Sin(w)
	coeff := 2 * cw
	var s0, s1, s2 float64
	for _, v := range x {
		s0 = v + coeff*s1 - s2
		s2, s1 = s1, s0
	}
	return 2 * math.Hypot(s1-s2*cw, s2*sw) / float64(len(x))
}

// oneNote 用某件音色彈一個音一秒，回傳取樣與基頻。
func oneNote(in Instrument, fnum, block int) ([]float64, float64) {
	c := NewOPL2()
	c.Write(0x01, 0x20)
	writeOp(c, chOffset[0], in.Modulator())
	writeOp(c, chOffset[0]+3, in.Carrier())
	c.Write(0xC0, in.Connection())
	c.Write(0xA0, byte(fnum&0xFF))
	c.Write(0xB0, byte(fnum>>8&3|block<<2|0x20))
	buf := make([]float64, OPLRate)
	for i := range buf {
		buf[i] = c.Sample()
	}
	return buf, float64(fnum) * math.Exp2(float64(block)) / (1 << 20) * OPLRate
}

// TestInstrumentsSoundLikeTheirNames 釘住合成出來的音色與名字相符。
//
// 這是合成器唯一便宜的正確性檢查：包絡或連接方式接錯，波形照樣出得來，
// **聽起來也還是某種樂器**——但長笛不會變成有八個泛音的簧片聲。
// 音色庫用的是 AdLib 的標準名，所以名字本身就是預期值。
//
//	oboe1   雙簧管：撐著不放，泛音多
//	flute2  長笛：撐著不放，幾乎只有基音
//	piano1  鋼琴：一觸即衰
func TestInstrumentsSoundLikeTheirNames(t *testing.T) {
	d := data1(t)
	tracks, err := ParseAll(d[SongIndex], d[SongData])
	if err != nil {
		t.Fatal(err)
	}
	byName := map[string]Instrument{}
	for _, tr := range tracks {
		for _, in := range tr.Bank.Instruments {
			byName[in.Name] = in
		}
	}
	for _, name := range []string{"oboe1", "flute2", "piano1"} {
		in, ok := byName[name]
		if !ok {
			t.Fatalf("音色庫裡沒有 %s", name)
		}
		buf, base := oneNote(in, 0x198, 4)
		h1 := goertzel(buf, base)
		h2 := goertzel(buf, base*2)
		h4 := goertzel(buf, base*4)
		// 前四分之一與最後四分之一的能量比，看它撐不撐得住。
		head, tail := rms(buf[:len(buf)/4]), rms(buf[len(buf)*3/4:])
		t.Logf("%-7s 基頻 %.0f Hz　二次諧波 %.3f、四次 %.3f（基音 %.3f）　尾／頭 %.2f",
			name, base, h2, h4, h1, tail/head)
		if h1 <= 0 {
			t.Errorf("%s 彈不出基音", name)
		}
		switch name {
		case "flute2":
			if h2 > h1/4 {
				t.Errorf("%s 該幾乎只有基音，二次諧波卻有基音的 %.0f%%",
					name, h2/h1*100)
			}
			if tail < head/2 {
				t.Errorf("%s 該撐得住，尾巴只剩頭的 %.0f%%", name, tail/head*100)
			}
		case "oboe1":
			if h4 < h1/20 {
				t.Errorf("%s 該有明顯的高次泛音，四次諧波只有基音的 %.1f%%",
					name, h4/h1*100)
			}
			if tail < head/2 {
				t.Errorf("%s 該撐得住，尾巴只剩頭的 %.0f%%", name, tail/head*100)
			}
		case "piano1":
			if tail > head/2 {
				t.Errorf("%s 該一觸即衰，尾巴還有頭的 %.0f%%", name, tail/head*100)
			}
		}
	}
}

func rms(x []float64) float64 {
	sum := 0.0
	for _, v := range x {
		sum += v * v
	}
	return math.Sqrt(sum / float64(len(x)))
}
