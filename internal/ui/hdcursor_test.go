package ui

import (
	"bytes"
	"crypto/sha256"
	"encoding/binary"
	"fmt"
	"image"
	"image/color"
	"image/png"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/wicanr2/softworld_san1_remake/internal/assets"
)

func cursorTestContainer(t *testing.T, name string, sprite, mask *assets.Image) *assets.Container {
	t.Helper()
	nam, idx, raw := []byte{}, []byte{}, []byte{}
	for n, im := range []*assets.Image{sprite, mask} {
		if im == nil {
			continue
		}
		key := name
		if n == 1 {
			key = strings.TrimSuffix(name, ".IMG") + "M.IMG"
		}
		_, c, _, _ := hdMenuFixture(t, key, im, false)
		parts := strings.Split(key, ".")
		entry := make([]byte, 16)
		copy(entry, parts[0])
		copy(entry[9:], parts[1])
		nam = append(nam, entry...)
		raw = append(raw, c.Data(0)...)
		var end [4]byte
		binary.LittleEndian.PutUint32(end[:], uint32(len(raw)))
		idx = append(idx, end[:]...)
	}
	c, err := assets.OpenContainer(nam, idx, raw)
	if err != nil {
		t.Fatal(err)
	}
	return c
}

func cursorTestImages() (*assets.Image, *assets.Image, *image.RGBA) {
	s := &assets.Image{W: 8, H: 16, Pix: make([]byte, 128)}
	m := &assets.Image{W: 8, H: 16, Pix: bytes.Repeat([]byte{15}, 128)}
	m.Set(1, 4, 0)
	m.Set(2, 4, 0)
	m.Set(3, 4, 0)
	s.Set(2, 4, 12)
	s.Set(3, 4, 4)
	high := image.NewRGBA(image.Rect(0, 0, 32, 64))
	for y := 0; y < 64; y++ {
		for x := 0; x < 32; x++ {
			if m.At(x/4, y/4) == 0 {
				high.SetRGBA(x, y, color.RGBA{byte(100 + x), byte(50 + y), 31, 255})
			}
		}
	}
	return s, m, high
}

func TestHDCursorSourceAndAlphaValidation(t *testing.T) {
	for _, check := range []string{"valid", "mask-missing", "source-shape", "mask-shape", "mask-code", "OR-identity", "pair-hash", "container", "identity-paint", "owned-transparent", "png-hash", "png-missing"} {
		t.Run(check, func(t *testing.T) {
			s, m, high := cursorTestImages()
			if check == "mask-missing" {
				m = nil
			}
			if check == "source-shape" {
				s.W = 16
				s.Pix = make([]byte, 256)
			}
			if check == "mask-shape" {
				m.H = 15
				m.Pix = m.Pix[:120]
			}
			if check == "mask-code" {
				m.Set(7, 15, 3)
			}
			if check == "OR-identity" {
				s.Set(7, 15, 12)
			}
			if check == "identity-paint" {
				high.SetRGBA(31, 63, color.RGBA{10, 20, 30, 255})
			}
			if check == "owned-transparent" {
				high.SetRGBA(4, 16, color.RGBA{})
			}
			c := cursorTestContainer(t, "CURB0.IMG", s, m)
			dir := t.TempDir()
			f, recipe, sourceErr := hdCursorSource(c, "CURB0.IMG")
			if check == "mask-missing" || check == "source-shape" || check == "mask-shape" || check == "mask-code" || check == "OR-identity" {
				if sourceErr == nil {
					t.Fatal("未知來源獲准")
				}
				return
			}
			if sourceErr != nil {
				t.Fatal(sourceErr)
			}
			var buf bytes.Buffer
			if err := png.Encode(&buf, high); err != nil {
				t.Fatal(err)
			}
			file := filepath.Join(dir, "cursor.png")
			if err := os.WriteFile(file, buf.Bytes(), 0644); err != nil {
				t.Fatal(err)
			}
			entry := HDEntry{Edition: "base", Container: "DATA1", Name: f.Name, SourceSHA256: recipe, File: "cursor.png", SHA256: fmt.Sprintf("%x", sha256.Sum256(buf.Bytes())), Width: 32, Height: 64}
			if check == "pair-hash" {
				i, _ := c.ByName("CURB0M.IMG")
				entry.SourceSHA256 = fmt.Sprintf("%x", sha256.Sum256(c.Data(i)))
			}
			if check == "container" {
				entry.Container = "DATA3"
			}
			if check == "png-hash" {
				entry.SHA256 = "wrong"
			}
			if check == "png-missing" {
				if err := os.Remove(file); err != nil {
					t.Fatal(err)
				}
			}
			hdManifest(t, dir, []HDEntry{entry})
			pack, err := LoadHDPack(dir, "base", map[string]*assets.Container{"DATA1": c, "DATA3": c})
			if err != nil {
				t.Fatal(err)
			}
			if check == "valid" {
				if pack.Count != 1 || len(pack.Warnings) != 0 || pack.cursors[hdCursorKey(f)] == nil {
					t.Fatal("合法游標拒收", pack.Warnings)
				}
			} else if pack.Count != 0 || len(pack.Warnings) != 1 {
				t.Fatal("錯項未回退", pack.Count, pack.Warnings)
			}
		})
	}
	for _, name := range []string{"CURE0.IMG", "CURB6.IMG", "CURB0M.IMG"} {
		if hdResource.MatchString(name) {
			t.Fatal("游標鍵越界", name)
		}
	}
}

func TestHDCursorFullCPUAndNativeComposition(t *testing.T) {
	s, m, high := cursorTestImages()
	f := assets.CursorFrame{Name: "CURB0.IMG", Sprite: s, Mask: m}
	var frames [6]assets.CursorFrame
	for i := range frames {
		frames[i] = f
	}
	for paper := 0; paper < 16; paper++ {
		for _, pt := range []image.Point{{4, 3}, {-3, -5}, {29, 19}} {
			t.Run(fmt.Sprintf("paper%d/at%v", paper, pt), func(t *testing.T) {
				control, c := testCanvasPx(t, 32, 24), testCanvasPx(t, 32, 24)
				control.Fill(assets.EGAPalette[paper])
				c.Fill(assets.EGAPalette[paper])
				bg := image.NewRGBA(image.Rect(0, 0, 128, 96))
				for y := 0; y < 96; y++ {
					for x := 0; x < 128; x++ {
						bg.SetRGBA(x, y, color.RGBA{byte(x), byte(y), 89, 255})
					}
				}
				c.HD = &HDPack{cursors: map[[32]byte]*image.RGBA{hdCursorKey(f): high}}
				c.addHigh(bg, c.Img.Bounds(), image.Point{})
				// identity 格前已畫的白字，不能因透出材質而消失。
				control.FillRect(5, 6, 6, 7, assets.EGAPalette[15])
				c.FillRect(5, 6, 6, 7, assets.EGAPalette[15])
				drawInputCursor(control, &frames, InputCursor{On: true, Frame: -1}, pt.X, pt.Y)
				drawInputCursor(c, &frames, InputCursor{On: true, Frame: -1}, pt.X, pt.Y)
				if !bytes.Equal(c.Img.Pix, control.Img.Pix) || c.Output(false) != c.Img {
					t.Fatal("原貌 CPU 改變")
				}
				out := c.Output(true)
				for y := 0; y < 96; y++ {
					for x := 0; x < 128; x++ {
						lx, ly := x/4-pt.X, y/4-pt.Y
						inside := lx >= 0 && lx < 8 && ly >= 0 && ly < 16
						want := bg.RGBAAt(x, y)
						if x/4 == 5 && y/4 == 6 {
							want = assets.EGAPalette[15]
						}
						if inside && m.At(lx, ly) == 0 {
							want = high.RGBAAt(x-pt.X*4, y-pt.Y*4)
						}
						if out.RGBAAt(x, y) != want {
							t.Fatalf("透明／裁切／前景 %d,%d", x, y)
						}
					}
				}
				col := color.RGBA{241, 101, 17, 255}
				c.FillRect(0, 0, 32, 24, col)
				out = c.Output(true)
				for y := 0; y < 96; y++ {
					for x := 0; x < 128; x++ {
						if out.RGBAAt(x, y) != col {
							t.Fatal("後畫前景失去覆蓋權")
						}
					}
				}
			})
		}
	}
	// 名稱及遮罩都參與身份，即使兩幀圖相同仍能分開回退。
	other := f
	other.Name = "CURB5.IMG"
	if hdCursorKey(other) == hdCursorKey(f) {
		t.Fatal("名稱別名碰撞")
	}
	other = f
	other.Mask = m.Clone()
	other.Mask.Set(0, 0, 0)
	if hdCursorKey(other) == hdCursorKey(f) {
		t.Fatal("遮罩身份遺漏")
	}
	for _, on := range []bool{false, true} {
		c := testCanvasPx(t, 32, 24)
		c.Fill(assets.EGAPalette[3])
		c.HD = &HDPack{}
		drawInputCursor(c, &frames, InputCursor{On: on}, 4, 3)
		if len(c.highOps) != 0 {
			t.Fatal("舊包或未等輸入新增游標")
		}
	}
}

func TestHDCursorActualFramesAndMissingAlias(t *testing.T) {
	root := os.Getenv("SAN1_ORIG")
	if root == "" {
		t.Skip("沒設 SAN1_ORIG，本儲存庫不含原版資料")
	}
	for _, edition := range []string{"base", "plus"} {
		t.Run(edition, func(t *testing.T) {
			folder := "三國演義"
			if edition == "plus" {
				folder = "三國演義1加強版"
			}
			var raw [3][]byte
			for n, ext := range []string{"NAM", "IDX", "GRP"} {
				var err error
				raw[n], err = os.ReadFile(filepath.Join(root, folder, "DATA1."+ext))
				if err != nil {
					t.Skipf("沒有 %s DATA1.%s：%v", edition, ext, err)
				}
			}
			d1, err := assets.OpenContainer(raw[0], raw[1], raw[2])
			if err != nil {
				t.Fatal(err)
			}
			dir := t.TempDir()
			entries := []HDEntry{}
			var families [4][6]assets.CursorFrame
			for style := range families {
				families[style], err = assets.CursorFrames(d1, assets.CursorStyle(style))
				if err != nil {
					t.Fatal(err)
				}
				for frame, f := range families[style] {
					wantName := fmt.Sprintf("CUR%c%d.IMG", 'A'+style, frame)
					if f.Name != wantName {
						t.Fatalf("來源名稱 %q，應為 %q", f.Name, wantName)
					}
					_, recipe, err := hdCursorSource(d1, f.Name)
					if err != nil {
						t.Fatal(err)
					}
					high := image.NewRGBA(image.Rect(0, 0, 32, 64))
					for y := 0; y < 64; y++ {
						for x := 0; x < 32; x++ {
							if f.Mask.At(x/4, y/4) == 0 {
								high.SetRGBA(x, y, color.RGBA{byte(20 + frame), byte(x + 30), byte(y + 50), 255})
							}
						}
					}
					var buf bytes.Buffer
					if err := png.Encode(&buf, high); err != nil {
						t.Fatal(err)
					}
					file := f.Name + ".png"
					if err := os.WriteFile(filepath.Join(dir, file), buf.Bytes(), 0644); err != nil {
						t.Fatal(err)
					}
					entries = append(entries, HDEntry{Edition: edition, Container: "DATA1", Name: f.Name, SourceSHA256: recipe, File: file, SHA256: fmt.Sprintf("%x", sha256.Sum256(buf.Bytes())), Width: 32, Height: 64})
				}
			}
			hdManifest(t, dir, entries)
			pack, err := LoadHDPack(dir, edition, map[string]*assets.Container{"DATA1": d1})
			if err != nil || pack.Count != 24 || len(pack.Warnings) != 0 {
				t.Fatal("完整游標包", err, pack)
			}
			for style, frames := range families {
				for frame, f := range frames {
					t.Run(f.Name, func(t *testing.T) {
						plain, c := testCanvasPx(t, 32, 24), testCanvasPx(t, 32, 24)
						plain.Fill(assets.EGAPalette[3])
						c.Fill(assets.EGAPalette[3])
						c.HD = pack
						in := InputCursor{On: true, Frame: frame}
						drawInputCursor(plain, &families[style], in, 4, 3)
						drawInputCursor(c, &families[style], in, 4, 3)
						if !bytes.Equal(c.Img.Pix, plain.Img.Pix) {
							t.Fatal("完整原貌 CPU 改變")
						}
						out := c.Output(true)
						for y := 0; y < 96; y++ {
							for x := 0; x < 128; x++ {
								want := plain.Img.RGBAAt(x/4, y/4)
								lx, ly := x/4-4, y/4-3
								if lx >= 0 && lx < 8 && ly >= 0 && ly < 16 && f.Mask.At(lx, ly) == 0 {
									want = color.RGBA{byte(20 + frame), byte(x - 16 + 30), byte(y - 12 + 50), 255}
								}
								if out.RGBAAt(x, y) != want {
									t.Fatalf("完整原生畫布 %d,%d", x, y)
								}
							}
						}
					})
				}
			}
			// CURC1／5 的來源圖與遮罩相同；缺一項只能讓該幀回退。
			if !bytes.Equal(families[2][1].Sprite.Pix, families[2][5].Sprite.Pix) || !bytes.Equal(families[2][1].Mask.Pix, families[2][5].Mask.Pix) {
				t.Fatal("同圖別名的來源前提改變")
			}
			for _, missing := range []int{1, 5} {
				filtered := []HDEntry{}
				for _, e := range entries {
					if e.Name != families[2][missing].Name {
						filtered = append(filtered, e)
					}
				}
				hdManifest(t, dir, filtered)
				p, err := LoadHDPack(dir, edition, map[string]*assets.Container{"DATA1": d1})
				if err != nil || p.Count != 23 || len(p.Warnings) != 0 {
					t.Fatal("缺幀包載入", err, p)
				}
				c := testCanvasPx(t, 32, 24)
				c.HD = p
				for frame, f := range families[2] {
					if (c.highCursor(f) == nil) != (frame == missing) {
						t.Fatal("缺幀由同圖別名吞掉", missing, frame)
					}
				}
			}
		})
	}
}

func TestHDCursorTitleFramesAndMissingFallback(t *testing.T) {
	d1, d3 := artContainers(t)
	title, err := NewTitleScreen(d3, d1)
	if err != nil {
		t.Fatal(err)
	}
	pack := &HDPack{cursors: map[[32]byte]*image.RGBA{}}
	for i, f := range *title.cursors {
		high := image.NewRGBA(image.Rect(0, 0, 32, 64))
		for y := 0; y < 64; y++ {
			for x := 0; x < 32; x++ {
				if f.Mask.At(x/4, y/4) == 0 {
					high.SetRGBA(x, y, color.RGBA{byte(21 + i), 71, 149, 255})
				}
			}
		}
		pack.cursors[hdCursorKey(f)] = high
	}
	for frame := 0; frame < 6; frame++ {
		for _, missing := range []bool{false, true} {
			c, plain := testCanvasPx(t, 640, 408), testCanvasPx(t, 640, 408)
			c.HD = pack
			f := title.cursors[frame]
			key := hdCursorKey(f)
			high := pack.cursors[key]
			if missing {
				delete(pack.cursors, key)
			}
			DrawTitleFrame(c, title, 0, frame)
			DrawTitleFrame(plain, title, 0, frame)
			out := c.Output(true)
			if !bytes.Equal(c.Img.Pix, plain.Img.Pix) {
				t.Fatal("主選單 CPU 改變")
			}
			for y := 0; y < 64; y++ {
				for x := 0; x < 32; x++ {
					want := plain.Img.RGBAAt(assets.MenuOrnamentX+x/4, assets.MenuOrnamentY+y/4)
					if !missing && f.Mask.At(x/4, y/4) == 0 {
						want = high.RGBAAt(x, y)
					}
					if out.RGBAAt(assets.MenuOrnamentX*4+x, assets.MenuOrnamentY*4+y) != want {
						t.Fatal("主選單游標或缺幀回退", frame, missing)
					}
				}
			}
			pack.cursors[key] = high
		}
	}
}
