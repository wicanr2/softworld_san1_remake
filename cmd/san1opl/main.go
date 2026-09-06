// san1opl 把原版寫進 OPL2 的暫存器值，對回音色庫的 56 個位元組。
//
// 音色庫每件音色 56 個位元組 ＝ 28 個 16 位元欄位（`docs/formats/06` §5）。
// 原版把音色填進 OPL2 的時候，六個暫存器裡就是那些參數的組合：
//
//	0x20  AM<<7 | VIB<<6 | EG<<5 | KSR<<4 | MULT
//	0x40  KSL<<6 | TL
//	0x60  AR<<4 | DR
//	0x80  SL<<4 | RR
//	0xC0  FB<<1 | CNT      （每個頻道一份，不是每個運算子）
//	0xE0  WS
//
// 拆開之後就是十四個 0–15／0–63 的小數字，拿去 28 個欄位裡找就對得上。
// 這個工具做的就是這件事，順便報「哪些段對得上、對不上的差在哪一欄」。
//
// ⚠ **這是「拿原版自己算出來的東西比對」**，與點陣圖那條路同一招
// （`docs/formats/07` §2）。不要靠「哪一種看起來合理」——點陣圖那次
// 先挑了一組看起來完全合理的，與畫面比對出來的並不相同。
//
//	# 先錄（在 dosgolem-san）
//	tools/go.sh run ./cmd/probe -exe ... -adlib -dump-ports '388,389=opl.tsv'
//	# 再對
//	tools/go.sh run ./cmd/san1opl -ports opl.tsv -root /path/to/三國演義
package main

import (
	"bufio"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"

	"github.com/wicanr2/softworld_san1_remake/internal/assets"
	"github.com/wicanr2/softworld_san1_remake/internal/music"
)

// write 是一次 OPL2 暫存器寫入。
type write struct {
	step uint64
	reg  byte
	val  byte
}

func main() {
	ports := flag.String("ports", "", "probe 的 -dump-ports TSV（必填）")
	root := flag.String("root", "", "原版遊戲目錄（音色庫要用；不給就只印暫存器）")
	after := flag.Uint64("after", 0,
		"只看第幾道指令之後的段落。\n"+
			"    驅動程式在初始化時會把一份**自己寫死的**預設音色填進九個頻道，\n"+
			"    那一份不在音色庫裡；要驗版面就得跳過它。")
	flag.Parse()
	if *ports == "" {
		fmt.Fprintln(os.Stderr, "san1opl: 要用 -ports 指到 probe 錄的 TSV")
		flag.Usage()
		os.Exit(2)
	}
	ws, err := readPorts(*ports)
	if err != nil {
		die(err)
	}
	fmt.Printf("OPL2 暫存器寫入 %d 筆\n", len(ws))

	patches := groupPatches(ws)
	if *after > 0 {
		var keep []patch
		for _, p := range patches {
			if p.step >= *after {
				keep = append(keep, p)
			}
		}
		fmt.Printf("只看第 %d 道指令之後：%d 段（原本 %d 段）\n", *after, len(keep), len(patches))
		patches = keep
	}
	fmt.Printf("看起來是「載入一份音色」的段落：%d 個\n\n", len(patches))
	for i, p := range patches {
		if i >= 12 {
			fmt.Printf("（其餘 %d 個略）\n", len(patches)-i)
			break
		}
		fmt.Printf("#%d 第 %d 道指令　頻道 %d\n", i, p.step, p.ch)
		for _, f := range p.fields() {
			fmt.Printf("    %-14s %d\n", f.name, f.val)
		}
	}

	if *root == "" {
		return
	}
	banks, err := loadBanks(*root)
	if err != nil {
		die(err)
	}
	fmt.Printf("\n音色庫：%d 份\n", len(banks))
	if len(patches) == 0 {
		fmt.Println("沒有錄到載入音色的段落——換一段有在放歌的錄音再試。")
		return
	}
	matchBanks(patches, banks)
}

// readPorts 讀 probe 的 TSV，把 0x388／0x389 配成暫存器寫入。
func readPorts(path string) ([]write, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	var out []write
	var reg byte
	sc := bufio.NewScanner(f)
	first := true
	for sc.Scan() {
		if first {
			first = false
			continue // 標題列
		}
		cols := strings.Split(sc.Text(), "\t")
		if len(cols) != 3 {
			continue
		}
		step, _ := strconv.ParseUint(cols[0], 10, 64)
		port, err1 := strconv.ParseUint(cols[1], 16, 16)
		val, err2 := strconv.ParseUint(cols[2], 16, 8)
		if err1 != nil || err2 != nil {
			continue
		}
		switch port {
		case 0x388:
			reg = byte(val)
		case 0x389:
			out = append(out, write{step, reg, byte(val)})
		}
	}
	return out, sc.Err()
}

// patch 是一次「把一份音色填進某個頻道」。
type patch struct {
	step uint64
	ch   int
	// mod／car 是兩個運算子的六個暫存器；索引是 0x20/0x40/0x60/0x80/0xE0。
	mod, car map[byte]byte
	conn     byte // 0xC0
}

// opOffset 是每個頻道兩個運算子在暫存器裡的位移。
var opOffset = [9]byte{0x00, 0x01, 0x02, 0x08, 0x09, 0x0A, 0x10, 0x11, 0x12}

// groupPatches 把寫入序列切成「一份音色」。
//
// 判準是**同一個頻道的一組運算子暫存器**：原版每個運算子固定寫
// `40, C0, 60, 80, 20, E0` 六個（`docs/formats/06` §6）。
func groupPatches(ws []write) []patch {
	cur := map[int]*patch{}
	var out []patch
	flush := func(ch int) {
		p := cur[ch]
		if p == nil {
			return
		}
		if len(p.mod) >= 4 && len(p.car) >= 4 {
			out = append(out, *p)
		}
		delete(cur, ch)
	}
	for _, w := range ws {
		base := w.reg & 0xE0
		off := w.reg & 0x1F
		switch base {
		case 0x20, 0x40, 0x60, 0x80, 0xE0:
			for ch := 0; ch < 9; ch++ {
				m, c := opOffset[ch], opOffset[ch]+3
				if off != m && off != c {
					continue
				}
				p := cur[ch]
				if p == nil {
					p = &patch{step: w.step, ch: ch,
						mod: map[byte]byte{}, car: map[byte]byte{}}
					cur[ch] = p
				}
				if off == m {
					p.mod[base] = w.val
				} else {
					p.car[base] = w.val
				}
				if len(p.mod) == 5 && len(p.car) == 5 {
					out = append(out, *p)
					delete(cur, ch)
				}
			}
		case 0xC0:
			ch := int(off)
			if ch < 9 {
				if p := cur[ch]; p != nil {
					p.conn = w.val
				}
			}
		case 0xA0:
			// 0xA0/0xB0 是音高與 key-on，換音色的段落到此為止。
			flush(int(off))
		}
	}
	return out
}

type field struct {
	name string
	val  int
}

// fields 把六個暫存器拆成十四個參數。
func (p patch) fields() []field {
	get := func(m map[byte]byte, r byte) int { return int(m[r]) }
	var out []field
	for _, op := range []struct {
		tag string
		m   map[byte]byte
	}{{"調變", p.mod}, {"載波", p.car}} {
		r20, r40 := get(op.m, 0x20), get(op.m, 0x40)
		r60, r80 := get(op.m, 0x60), get(op.m, 0x80)
		out = append(out,
			field{op.tag + " AM", r20 >> 7 & 1},
			field{op.tag + " VIB", r20 >> 6 & 1},
			field{op.tag + " EG", r20 >> 5 & 1},
			field{op.tag + " KSR", r20 >> 4 & 1},
			field{op.tag + " MULT", r20 & 15},
			field{op.tag + " KSL", r40 >> 6},
			field{op.tag + " TL", r40 & 63},
			field{op.tag + " AR", r60 >> 4},
			field{op.tag + " DR", r60 & 15},
			field{op.tag + " SL", r80 >> 4},
			field{op.tag + " RR", r80 & 15},
			field{op.tag + " WS", get(op.m, 0xE0) & 3},
		)
	}
	out = append(out,
		field{"回授 FB", int(p.conn) >> 1 & 7},
		field{"連接 CNT", int(p.conn) & 1})
	return out
}

// loadBanks 讀原版的音色庫。
func loadBanks(root string) ([]*music.Bank, error) {
	base := filepath.Join(root, "DATA1")
	var parts [3][]byte
	for i, ext := range []string{".NAM", ".IDX", ".GRP"} {
		b, err := os.ReadFile(base + ext)
		if err != nil {
			return nil, err
		}
		parts[i] = b
	}
	c, err := assets.OpenContainer(parts[0], parts[1], parts[2])
	if err != nil {
		return nil, err
	}
	get := func(name string) []byte {
		i, ok := c.ByName(name)
		if !ok {
			return nil
		}
		return c.Data(i)
	}
	var out []*music.Bank
	for _, pair := range [][2]string{
		{music.SongIndex, music.SongData}, {music.LongIndex, music.LongData},
	} {
		tracks, err := music.ParseAll(get(pair[0]), get(pair[1]))
		if err != nil {
			continue // MUSV 還沒解乾淨，跳過不影響 MUS
		}
		for _, tr := range tracks {
			out = append(out, tr.Bank)
		}
	}
	return out, nil
}

// modLayout 是一個運算子在音色記錄裡的欄位順序。
//
// 出處：驅動程式初始化時填進去的那一份預設音色，與音色庫裡的 `piano1`
// 前十二個欄位**逐項相同而且同順序**。十二個值全中不會是巧合。
var modLayout = []string{
	"KSL", "MULT", "FB", "AR", "SL", "EG", "DR", "RR", "TL", "AM", "VIB", "KSR",
}

const (
	fieldCON = 12 // 調變運算子的連接欄
	carBase  = 13 // 載波那一組的起點
	waveMod  = 26 // 調變運算子的波形
	waveCar  = 27 // 載波的波形
)

// paramsOf 把一個運算子的暫存器拆成具名參數。
func paramsOf(m map[byte]byte, conn byte) map[string]int {
	r20, r40 := int(m[0x20]), int(m[0x40])
	r60, r80 := int(m[0x60]), int(m[0x80])
	return map[string]int{
		"AM": r20 >> 7 & 1, "VIB": r20 >> 6 & 1, "EG": r20 >> 5 & 1,
		"KSR": r20 >> 4 & 1, "MULT": r20 & 15,
		"KSL": r40 >> 6, "TL": r40 & 63,
		"AR": r60 >> 4, "DR": r60 & 15,
		"SL": r80 >> 4, "RR": r80 & 15,
		"WS": int(m[0xE0]) & 3,
		"FB": int(conn) >> 1 & 7, "CNT": int(conn) & 1,
	}
}

// matchBanks 驗證整份音色的欄位版面。
//
// 版面：56 個位元組 ＝ 28 個 16 位元欄位，
//
//	欄位 0–12   調變運算子：KSL MULT FB AR SL EG DR RR TL AM VIB KSR CON
//	欄位 13–25  載波運算子：同樣的十三項
//	欄位 26–27  兩個運算子的波形
//
// 驗法是拿原版填進 OPL2 的暫存器值回頭比對：每一段載入的音色，
// 各項參數要與某一件音色的欄位逐項相同。
//
// 兩件事會讓「載波那一半」對不上，都不是版面錯：
//
//   - 載波沒有回授，OPL2 的 `0xC0` 一份是**整個頻道**共用的，
//     音色庫在載波的 FB 欄留的是未初始化的值。
//   - 節奏模式的小鼓、鈸、鈴鼓、通鼓在 OPL2 裡是**單運算子**，
//     驅動程式只填一半，另一半在檔案裡是什麼都無所謂。
//
// `CON` 與暫存器的位元**相反**：音色庫寫 1 代表調頻，
// 而 `0xC0` 的位元 0 是 1 代表相加。
func matchBanks(patches []patch, banks []*music.Bank) {
	type named struct {
		name string
		in   music.Instrument
	}
	var all []named
	seen := map[string]bool{}
	for _, b := range banks {
		for _, in := range b.Instruments {
			if !seen[in.Name] {
				seen[in.Name] = true
				all = append(all, named{in.Name, in})
			}
		}
	}
	// 比對的欄位：調變十二項（不含 CON，另外驗）＋ 波形，載波同樣，
	// 但扣掉 FB（垃圾）與 TL（被音量調過）。
	modFields := append([]string{}, modLayout...)
	carFields := []string{}
	for _, f := range modLayout {
		if f != "FB" && f != "TL" {
			carFields = append(carFields, f)
		}
	}
	full, half, none := 0, 0, 0
	counts := map[string]int{}
	diffs := map[string]int{}
	conOK, conBad := 0, 0
	for _, p := range patches {
		mp := paramsOf(p.mod, p.conn)
		cp := paramsOf(p.car, p.conn)
		best, bestN, bestMod := named{}, -1, 0
		for _, n := range all {
			ok, om := 0, 0
			for i, f := range modFields {
				if n.in.Param(i) == mp[f] {
					ok++
					om++
				}
			}
			if n.in.Param(waveMod) == mp["WS"] {
				ok++
				om++
			}
			for _, f := range carFields {
				if n.in.Param(carBase+fieldIndex(f)) == cp[f] {
					ok++
				}
			}
			if n.in.Param(waveCar) == cp["WS"] {
				ok++
			}
			if ok > bestN {
				bestN, best, bestMod = ok, n, om
			}
		}
		const wantMod = 12 + 1 // 調變十二項 ＋ 波形
		wantAll := wantMod + len(carFields) + 1
		switch {
		case bestN == wantAll:
			full++
			counts[best.name]++
		case bestMod == wantMod:
			// 調變全中、載波沒中。兩種來源：節奏模式的單運算子音色，
			// 以及驅動程式重設時對九個頻道各填一次的內建預設音色
			//（那一份的調變與 `piano1` 相同，載波的起音不同）。
			half++
			counts[best.name+"（只有調變）"]++
		default:
			none++
			for i, f := range modFields {
				if best.in.Param(i) != mp[f] {
					diffs["調變 "+f]++
				}
			}
			for _, f := range carFields {
				if best.in.Param(carBase+fieldIndex(f)) != cp[f] {
					diffs["載波 "+f]++
				}
			}
		}
		if best.in.Param(fieldCON) == 1-mp["CNT"] {
			conOK++
		} else {
			conBad++
		}
	}
	fmt.Printf("\n版面驗證（調變 0–12、載波 13–25、波形 26–27）：\n")
	fmt.Printf("  兩個運算子全中 %d 段、只有調變全中 %d 段、對不上 %d 段（共 %d 段）\n",
		full, half, none, len(patches))
	var names []string
	for k := range counts {
		names = append(names, k)
	}
	sort.Strings(names)
	for _, k := range names {
		fmt.Printf("    %-22s ×%d\n", k, counts[k])
	}
	if len(diffs) > 0 {
		fmt.Printf("  對不上的那些段，是哪些欄位差的：\n")
		var ks []string
		for k := range diffs {
			ks = append(ks, k)
		}
		sort.Strings(ks)
		for _, k := range ks {
			fmt.Printf("    %-12s ×%d\n", k, diffs[k])
		}
	}
	fmt.Printf("  連接位元（音色庫的 CON 與 0xC0 位元 0 相反）：相符 %d、不符 %d\n",
		conOK, conBad)
}

// fieldIndex 是某個參數在運算子記錄裡的位置。
func fieldIndex(name string) int {
	for i, f := range modLayout {
		if f == name {
			return i
		}
	}
	return -1
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func die(err error) {
	fmt.Fprintln(os.Stderr, "san1opl:", err)
	os.Exit(1)
}
