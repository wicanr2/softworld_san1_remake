// san1imgcheck 拿 dosgolem 產的 EGA 畫面當基準，驗證圖檔的解讀。
//
// 圖的**版面**（表頭是高與寬、四個完整位元平面、最高位在左）換一種讀法
// 就不是圖，肉眼分得出來；但**平面與顏色的對應只換顏色不換結構**，
// 二十四種排列在結構上完全一樣。要分出哪一種對，只能拿原版自己算出來的
// 色號比對。
//
// 用法：先用 dosgolem 把原版跑到某個畫面並存成 PNG，
//
//	cd ../dosgolem-san
//	DOSGOLEM_ORIG=.../org_game tools/go.sh run ./cmd/probe \
//	  -exe "/orig/三國演義/AA.EXE" -root "/orig/三國演義" \
//	  -steps 3000000000 -keys '122' \
//	  -keys-at '250000000:\r,900000000:1,1500000000:1' \
//	  -ega-every '100000000:640x350=/src/workplace/shots/s'
//
// 再把某幾個圖檔拿去那張畫面裡找：
//
//	tools/go.sh run ./cmd/san1imgcheck -shot shots/s-022.png \
//	  -root /path/to/三國演義 -container DATA1 -items F000.FAC,F005.FAC
//
// ⚠ **本儲存庫不含任何原版檔案**，畫面與圖檔都出自玩家自己那一份。
package main

import (
	"flag"
	"fmt"
	"image/color"
	"image/png"
	"os"
	"path/filepath"
	"strings"

	"github.com/wicanr2/softworld_san1_remake/internal/assets"
)

// indexOf 把 RGB 換回 EGA 色號。
func indexOf(c color.RGBA) int {
	for i, p := range assets.EGAPalette {
		if p.R == c.R && p.G == c.G && p.B == c.B {
			return i
		}
	}
	return -1
}

func loadScreen(path string) ([]byte, int, int, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, 0, 0, err
	}
	defer f.Close()
	img, err := png.Decode(f)
	if err != nil {
		return nil, 0, 0, err
	}
	b := img.Bounds()
	w, h := b.Dx(), b.Dy()
	out := make([]byte, w*h)
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			r, g, bl, _ := img.At(b.Min.X+x, b.Min.Y+y).RGBA()
			i := indexOf(color.RGBA{uint8(r >> 8), uint8(g >> 8), uint8(bl >> 8), 255})
			if i < 0 {
				return nil, 0, 0, fmt.Errorf("畫面裡有不在 EGA 十六色裡的顏色")
			}
			out[y*w+x] = byte(i)
		}
	}
	return out, w, h, nil
}

// decode 用指定的參數解一張圖，回傳色號。
func decode(body []byte, w, h int, bits [4]uint, wholePlanes, msb bool) []byte {
	stride := (w + 7) / 8
	if stride*h*4 != len(body) {
		return nil
	}
	pix := make([]byte, w*h)
	plane := stride * h
	for p := 0; p < 4; p++ {
		for y := 0; y < h; y++ {
			var row int
			if wholePlanes {
				row = p*plane + y*stride
			} else {
				row = y*stride*4 + p*stride
			}
			for x := 0; x < w; x++ {
				var mask byte
				if msb {
					mask = 0x80 >> (uint(x) % 8)
				} else {
					mask = 1 << (uint(x) % 8)
				}
				if body[row+x/8]&mask != 0 {
					pix[y*w+x] |= 1 << bits[p]
				}
			}
		}
	}
	return pix
}

// find 在畫面裡找一張圖，回傳位置與相符的像素比例。
func find(screen []byte, sw, sh int, pix []byte, w, h int) (int, int, float64) {
	if w > sw || h > sh {
		return -1, -1, 0
	}
	bestX, bestY, best := -1, -1, 0.0
	for oy := 0; oy+h <= sh; oy++ {
		for ox := 0; ox+w <= sw; ox++ {
			// 先用第一列快篩。
			ok := true
			for x := 0; x < w && ok; x++ {
				if screen[oy*sw+ox+x] != pix[x] {
					ok = false
				}
			}
			if !ok {
				continue
			}
			same := 0
			for y := 0; y < h; y++ {
				for x := 0; x < w; x++ {
					if screen[(oy+y)*sw+ox+x] == pix[y*w+x] {
						same++
					}
				}
			}
			if r := float64(same) / float64(w*h); r > best {
				best, bestX, bestY = r, ox, oy
			}
			if best == 1 {
				return bestX, bestY, best
			}
		}
	}
	return bestX, bestY, best
}

func perms() [][4]uint {
	var out [][4]uint
	var rec func(cur []uint, left []uint)
	rec = func(cur, left []uint) {
		if len(left) == 0 {
			var a [4]uint
			copy(a[:], cur)
			out = append(out, a)
			return
		}
		for i := range left {
			nl := append(append([]uint{}, left[:i]...), left[i+1:]...)
			rec(append(append([]uint{}, cur...), left[i]), nl)
		}
	}
	rec(nil, []uint{0, 1, 2, 3})
	return out
}

func main() {
	shot := flag.String("shot", "", "dosgolem 產的 EGA 畫面（PNG，必填）")
	root := flag.String("root", "", "原版遊戲目錄（必填，玩家自備）")
	cname := flag.String("container", "DATA1", "容器：DATA1／DATA2／DATA3")
	list := flag.String("items", "", "要驗的項目，逗號分隔（例 F000.FAC,F005.FAC）")
	flag.Parse()
	if *shot == "" || *root == "" || *list == "" {
		fmt.Fprintln(os.Stderr, "san1imgcheck: -shot、-root、-items 都要給")
		flag.Usage()
		os.Exit(2)
	}
	screen, sw, sh, err := loadScreen(*shot)
	if err != nil {
		fmt.Fprintln(os.Stderr, "san1imgcheck: 讀畫面：", err)
		os.Exit(1)
	}
	fmt.Printf("畫面 %dx%d\n", sw, sh)

	base := filepath.Join(*root, *cname)
	var p [3][]byte
	for i, ext := range []string{".NAM", ".IDX", ".GRP"} {
		b, err := os.ReadFile(base + ext)
		if err != nil {
			fmt.Fprintln(os.Stderr, "san1imgcheck:", err)
			os.Exit(1)
		}
		p[i] = b
	}
	c, err := assets.OpenContainer(p[0], p[1], p[2])
	if err != nil {
		fmt.Fprintln(os.Stderr, "san1imgcheck:", err)
		os.Exit(1)
	}

	hits := 0
	for _, name := range strings.Split(*list, ",") {
		name = strings.TrimSpace(name)
		i, ok := c.ByName(name)
		if !ok {
			fmt.Printf("%-14s 容器裡沒有這一項\n", name)
			continue
		}
		d := c.Data(i)
		a := int(d[0]) | int(d[1])<<8
		b := int(d[2]) | int(d[3])<<8
		body := d[4:]
		bestDesc, bestR := "", 0.0
		for _, dims := range [][2]int{{b, a}, {a, b}} {
			w, h := dims[0], dims[1]
			for _, whole := range []bool{true, false} {
				for _, msb := range []bool{true, false} {
					for _, pm := range perms() {
						pix := decode(body, w, h, pm, whole, msb)
						if pix == nil {
							continue
						}
						x, y, r := find(screen, sw, sh, pix, w, h)
						if r > bestR {
							bestR = r
							bestDesc = fmt.Sprintf("%dx%d 完整平面=%v 最高位在左=%v 平面→位元=%v　在 (%d,%d) 相符 %.1f%%",
								w, h, whole, msb, pm, x, y, r*100)
						}
						if r == 1 {
							goto done
						}
					}
				}
			}
		}
	done:
		if bestR == 0 {
			fmt.Printf("%-14s 這張畫面裡找不到\n", name)
			continue
		}
		hits++
		fmt.Printf("%-14s %s\n", name, bestDesc)
	}
	if hits == 0 {
		fmt.Println("\n這張畫面裡一張都沒對上——換一張有那些圖的畫面再試。")
		os.Exit(1)
	}
	fmt.Printf("\n%d 項對上了。100%% 相符表示尺寸、平面順序、顏色對應三件事都對。\n", hits)
}
