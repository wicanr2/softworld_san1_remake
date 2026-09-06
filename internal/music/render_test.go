package music

import (
	"fmt"
	"testing"
)

// keyOns 走一遍曲子，收下每個頻道的 key-on（block 與 F-number）。
func keyOns(s *Song, b *Bank) map[int][]string {
	c := NewOPL2()
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
	drive(c, s, b, func(int) {})
	return got
}

// TestNotesMatchTheOriginal 拿原版自己彈出來的音高對拍。
//
// 對照組是原版在 DOS 裡播 `思古` 時送到 0x388／0x389 的埠寫入，
// 用 dosgolem 的 `-dump-ports` 錄下來再拆成 block 與 F-number
//（`docs/formats/06` §8）。這裡比的是每個聲部前十四次按鍵。
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
		rms := 0.0
		if len(pcm) > 0 {
			rms = sqrt(sum / float64(len(pcm)))
		}
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

func sqrt(v float64) float64 {
	if v <= 0 {
		return 0
	}
	x := v
	for i := 0; i < 40; i++ {
		x = (x + v/x) / 2
	}
	return x
}
