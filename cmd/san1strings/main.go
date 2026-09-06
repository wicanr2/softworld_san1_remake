// san1strings 從原版的執行檔映像抽出 Big5／ASCII 字串。
//
// `docs/re/04-program-strings.md` 是它的產物。**工具要進版控**：
// 一份沒有辦法重跑的抽取結果，別人只能選擇相信或不信。
//
// ⚠ **本儲存庫不含任何原版檔案**，路徑由玩家自備。
// 輸出是位移 ＋ 字串，不是原版素材的複本——要完整的資料請自己跑。
//
//	tools/go.sh run ./cmd/san1strings -f /orig/三國演義/AA.EXE
//	tools/go.sh run ./cmd/san1strings -f /orig/三國演義/AA.EXE -at 0x46c02,0x485b0
package main

import (
	"flag"
	"fmt"
	"os"
	"strconv"
	"strings"

	"golang.org/x/text/encoding/traditionalchinese"
)

func main() {
	path := flag.String("f", "", "執行檔映像（必填，玩家自備）")
	from := flag.Int64("from", 0, "從哪個檔案位移開始掃")
	to := flag.Int64("to", 0, "掃到哪裡；0 ＝ 到檔尾")
	min := flag.Int("min", 3, "最短幾個字元才印")
	at := flag.String("at", "", "改成只印這幾個位移起的字串（逗號分隔，可用 0x 開頭）")
	n := flag.Int("n", 4, "-at 模式每個位移印幾條")
	flag.Parse()
	if *path == "" {
		fmt.Fprintln(os.Stderr, "san1strings: 要用 -f 指到執行檔（本儲存庫不含原版檔案）")
		flag.Usage()
		os.Exit(2)
	}
	data, err := os.ReadFile(*path)
	if err != nil {
		fmt.Fprintln(os.Stderr, "san1strings:", err)
		os.Exit(1)
	}
	if *at != "" {
		dumpAt(data, *at, *n)
		return
	}
	end := int(*to)
	if end <= 0 || end > len(data) {
		end = len(data)
	}
	for _, s := range scan(data, int(*from), end, *min) {
		fmt.Printf("%#08x\t%s\n", s.off, s.text)
	}
}

type found struct {
	off  int
	text string
}

// scan 掃出 NUL 結尾、整串解得開的字串。
//
// 判準是「每個位元組要嘛是可列印 ASCII，要嘛是合法的 Big5 雙位元組對」。
// **寬一點會撈到大量點陣資料**——圖形的位元組序列偶爾也是合法 Big5，
// 所以再要求要嘛含漢字、要嘛長得像檔名。
func scan(data []byte, from, to, min int) []found {
	dec := traditionalchinese.Big5.NewDecoder()
	var out []found
	for i := from; i < to; {
		j := i
		for j < to && data[j] != 0 {
			j++
		}
		if j-i >= 2 && plausible(data[i:j]) {
			if s, err := dec.Bytes(data[i:j]); err == nil {
				txt := string(s)
				if len([]rune(strings.TrimSpace(txt))) >= min {
					out = append(out, found{i, txt})
				}
			}
		}
		i = j + 1
	}
	return out
}

func big5Lead(b byte) bool { return b >= 0xA1 && b <= 0xF9 }
func big5Tail(b byte) bool {
	return (b >= 0x40 && b <= 0x7E) || (b >= 0xA1 && b <= 0xFE)
}

func plausible(s []byte) bool {
	cjk := 0
	for i := 0; i < len(s); {
		c := s[i]
		switch {
		case c == 0x0A || c == 0x09:
			i++
		case c >= 0x20 && c < 0x7F:
			i++
		case big5Lead(c) && i+1 < len(s) && big5Tail(s[i+1]):
			cjk++
			i += 2
		default:
			return false
		}
	}
	return cjk > 0 || len(s) >= 8
}

// dumpAt 從指定位移印出接下來幾條字串。
//
// 選單是一整條含 `\n` 的字串，掃描結果只給第一行看不出全貌；
// 要引用成規格證據就得看完整條。
func dumpAt(data []byte, list string, n int) {
	dec := traditionalchinese.Big5.NewDecoder()
	for _, a := range strings.Split(list, ",") {
		a = strings.TrimSpace(a)
		off, err := strconv.ParseInt(a, 0, 64)
		if err != nil {
			fmt.Fprintf(os.Stderr, "san1strings: 位移 %q 看不懂\n", a)
			continue
		}
		i := int(off)
		for k := 0; k < n && i < len(data); k++ {
			j := i
			for j < len(data) && data[j] != 0 {
				j++
			}
			if j > i {
				txt := ""
				if s, err := dec.Bytes(data[i:j]); err == nil {
					txt = string(s)
				} else {
					txt = fmt.Sprintf("(解不出來 % X)", data[i:j])
				}
				fmt.Printf("%#08x\t%q\n", i, txt)
			}
			i = j + 1
		}
		fmt.Println()
	}
}
