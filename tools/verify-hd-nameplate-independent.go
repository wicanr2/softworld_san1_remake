//go:build ignore

// 在 Docker 獨立解碼正常單挑收據的全部 PNG，核對完整英文姓名牌。
package main

import (
	"bufio"
	"compress/gzip"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"image"
	"image/png"
	"os"
	"path/filepath"
	"strings"
)

func must(err error) {
	if err != nil {
		panic(err)
	}
}

func digest(path string) string {
	b, err := os.ReadFile(path)
	must(err)
	return fmt.Sprintf("%x", sha256.Sum256(b))
}

func main() {
	if len(os.Args) != 3 {
		panic("用法：go run tools/verify-hd-nameplate-independent.go <GUI 根目錄> <新收據.json>")
	}
	if _, err := os.Stat(os.Args[2]); !os.IsNotExist(err) {
		panic("輸出已存在或無法核對")
	}
	f, err := os.Open("fonts/ascii6x10.hex.gz")
	must(err)
	z, err := gzip.NewReader(f)
	must(err)
	glyphs := map[byte][]byte{}
	s := bufio.NewScanner(z)
	for s.Scan() {
		line := strings.TrimSpace(s.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		parts := strings.SplitN(line, ":", 2)
		var cp int
		_, err = fmt.Sscanf(parts[0], "%x", &cp)
		must(err)
		b, err := hex.DecodeString(parts[1])
		must(err)
		if cp < 128 {
			glyphs[byte(cp)] = b
		}
	}
	must(s.Err())
	must(z.Close())
	must(f.Close())

	proof := map[string]any{"passed": true, "scope": "正常 GUI 檔案完整性與完整英文姓名字模；非原版 oracle", "editions": map[string]any{}, "font_sha256": digest("fonts/ascii6x10.hex.gz")}
	for _, ed := range []string{"base", "plus"} {
		dir := filepath.Join(os.Args[1], ed)
		var receipt struct {
			Passed        bool
			BinarySHA256  string            `json:"binary_sha256"`
			SourcesSHA256 map[string]string `json:"sources_sha256"`
			Captures      []struct{ File, SHA256 string }
		}
		b, err := os.ReadFile(filepath.Join(dir, "receipt.json"))
		must(err)
		must(json.Unmarshal(b, &receipt))
		if !receipt.Passed || digest(filepath.Join(dir, "san1-window-check")) != receipt.BinarySHA256 {
			panic("GUI 未通過或程式雜湊不符：" + ed)
		}
		for source, want := range receipt.SourcesSHA256 {
			if digest(filepath.Join(dir, filepath.Base(source))) != want {
				panic("工具快照不符：" + source)
			}
		}
		names := 0
		for _, c := range receipt.Captures {
			path := filepath.Join(dir, c.File)
			if digest(path) != c.SHA256 {
				panic("PNG 雜湊不符：" + path)
			}
			f, err := os.Open(path)
			must(err)
			im, err := png.Decode(f)
			must(err)
			must(f.Close())
			if !strings.Contains(c.File, "defender-speech-1-en-") {
				continue
			}
			scale := 1
			if strings.Contains(c.File, "-hd-") {
				scale = 4
			}
			if im.Bounds() != image.Rect(0, 0, 640*scale, 408*scale) {
				panic("姓名牌畫布尺寸不符")
			}
			// 固定姓名、自由字模及原 48×16 字區，各自獨立於正式渲染器。
			name := "Chen Gong"
			w, h := 192, 40*192/(len(name)*24)
			left, top := 0, (64-h)/2
			for y := 0; y < 16*scale; y++ {
				for x := 0; x < 48*scale; x++ {
					hx, hy := x*4/scale, y*4/scale
					var wr, wg, wb uint32
					if hx >= left && hx < left+w && hy >= top && hy < top+h {
						sx := (2*(hx-left) + 1) * len(name) * 24 / (2 * w)
						sy := (2*(hy-top) + 1) * 40 / (2 * h)
						g := glyphs[name[sx/24]]
						if len(g) != 10 {
							panic("姓名缺自由字模")
						}
						if g[sy/4]&(1<<uint(7-sx%24/4)) != 0 {
							wr, wg, wb = 85*257, 255*257, 85*257
						}
					}
					r, g, b, a := im.At(568*scale+x, 236*scale+y).RGBA()
					if r != wr || g != wg || b != wb || a != 65535 {
						panic(fmt.Sprintf("%s 姓名牌完整像素不符 (%d,%d)", path, x, y))
					}
				}
			}
			names++
		}
		if names != 3 {
			panic(fmt.Sprintf("%s 原貌／高清／恢復姓名牌不是三張：%d", ed, names))
		}
		proof["editions"].(map[string]any)[ed] = map[string]any{"png_count": len(receipt.Captures), "nameplate_count": names, "receipt_sha256": digest(filepath.Join(dir, "receipt.json")), "binary_sha256": receipt.BinarySHA256}
	}
	b, err := json.MarshalIndent(proof, "", "  ")
	must(err)
	must(os.WriteFile(os.Args[2], append(b, '\n'), 0644))
	fmt.Println(string(b))
}
