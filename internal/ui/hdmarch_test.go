package ui

import (
	"bytes"
	"fmt"
	"image"
	"image/color"
	"os"
	"path/filepath"
	"testing"

	"github.com/wicanr2/softworld_san1_remake/internal/assets"
)

func TestHDMarchSourceValidation(t *testing.T) {
	for slot := 0; slot < 16; slot++ {
		t.Run(fmt.Sprintf("slot%02d", slot), func(t *testing.T) {
			for _, check := range []string{"valid", "shape", "alpha", "container", "source_hash", "png_hash", "missing"} {
				t.Run(check, func(t *testing.T) {
					im := &assets.Image{W: 32, H: 32, Pix: make([]byte, 1024)}
					if check == "shape" {
						im.H = 31
						im.Pix = make([]byte, 32*31)
					}
					dir, container, entry, _ := hdMenuFixture(t, fmt.Sprintf("CVSC%02d.IMG", slot), im, check == "alpha")
					entry.Container = "DATA3"
					switch check {
					case "container":
						entry.Container = "DATA1"
					case "source_hash":
						entry.SourceSHA256 = "wrong"
					case "png_hash":
						entry.SHA256 = "wrong"
					case "missing":
						if e := os.Remove(filepath.Join(dir, entry.File)); e != nil {
							t.Fatal(e)
						}
					}
					hdManifest(t, dir, []HDEntry{entry})
					p, e := LoadHDPack(dir, "base", map[string]*assets.Container{"DATA1": container, "DATA3": container})
					if e != nil {
						t.Fatal(e)
					}
					if check == "valid" {
						if p.Count != 1 || len(p.Warnings) != 0 {
							t.Fatal(p.Warnings)
						}
					} else if p.Count != 0 || len(p.Warnings) != 1 {
						t.Fatalf("invalid accepted: %s count%d warnings%v", check, p.Count, p.Warnings)
					}
				})
			}
		})
	}
	for slot := 16; slot < 24; slot++ {
		if hdResource.MatchString(fmt.Sprintf("CVSC%02d.IMG", slot)) {
			t.Fatal("mask accepted as HD art", slot)
		}
	}
	for _, n := range []int{854, 855} {
		dir := t.TempDir()
		many := make([]HDEntry, n)
		for i := range many {
			many[i].Edition = "plus"
		}
		hdManifest(t, dir, many)
		_, e := LoadHDPack(dir, "base", nil)
		if (e == nil) != (n == 854) {
			t.Fatal("manifest bound", n, e)
		}
	}
}

func TestHDMarchAllFramesAndFallback(t *testing.T) {
	art, e := MarchArt(artContainer(t, "DATA3"))
	if e != nil {
		t.Fatal(e)
	}
	layouts := [][4]int{{52, 92, 98, 107}, {98, 107, 52, 92}, {161, 160, 161, 188}, {161, 188, 161, 160}, {10, 10, 30, 15}}
	for li, coords := range layouts {
		for _, missing := range []string{"none", "attacker", "defender", "all"} {
			t.Run(fmt.Sprintf("layout%d-%s", li, missing), func(t *testing.T) {
				l := NewMarchLayout(coords[0], coords[1], coords[2], coords[3])
				base := &assets.Image{W: 640, H: 408, Pix: make([]byte, 640*408)}
				for i := range base.Pix {
					base.Pix[i] = byte((i/640 + i%640) % 16)
				}
				m := NewMarch(l, art, 34, base.Clone(), nil)
				p := &HDPack{images: map[[32]byte]*image.RGBA{}}
				for slot := 0; slot < 16; slot++ {
					if missing == "all" || missing == "attacker" && (slot == l.Att || slot == l.Att+1) || missing == "defender" && (slot == l.Def || slot == l.Def+1) {
						continue
					}
					p.images[hdImageKey(art[slot])] = marchNativeFixture(slot)
				}
				for frame := 0; frame <= m.Frames(); frame++ {
					active := m.Step()
					c := testCanvasPx(t, 640, 408)
					c.HD = p
					c.drawRGBA(c.Img.Bounds(), base.RGBA(), image.Point{})
					DrawMarch(c, m)
					cpu := append([]byte(nil), c.Img.Pix...)
					got := c.Output(true)
					want := image.NewRGBA(got.Bounds())
					scaleRGBA4(want, c.Img)
					if active {
						moved := max(0, frame-MarchPreroll)
						phase := frame & 1
						poses := [][4]int{{int(float64(l.AX-8) + float64(moved*(l.DX-l.AX))/32 + 80), int(float64(l.AY-24) + float64(moved*(l.DY-l.AY))/32 + 44), l.Att + phase, l.AttMask + phase}, {l.DX + 72, l.DY + 20, l.Def + phase, l.DefMask + phase}}
						if poses[1][1] < poses[0][1] {
							poses[0], poses[1] = poses[1], poses[0]
						}
						// 獨立逐原生像素合成，後畫原圖即使缺高清也取得覆蓋權。
						clip := image.Rect(l.X, l.Y, l.X+l.W, l.Y+l.H).Intersect(c.Img.Bounds())
						for _, pose := range poses {
							sprite, mask := art[pose[2]], art[pose[3]]
							r := image.Rect(pose[0], pose[1], pose[0]+32, pose[1]+32).Intersect(clip)
							high := p.images[hdImageKey(sprite)]
							for y := r.Min.Y; y < r.Max.Y; y++ {
								for x := r.Min.X; x < r.Max.X; x++ {
									sx, sy := x-pose[0], y-pose[1]
									if mask.At(sx, sy)&8 != 0 && sprite.At(sx, sy)&15 == 0 {
										continue
									}
									for dy := 0; dy < 4; dy++ {
										for dx := 0; dx < 4; dx++ {
											col := c.Img.RGBAAt(x, y)
											if high != nil {
												col = high.RGBAAt(sx*4+dx, sy*4+dy)
											}
											want.SetRGBA(x*4+dx, y*4+dy, col)
										}
									}
								}
							}
						}
					}
					if !bytes.Equal(got.Pix, want.Pix) {
						for y := 0; y < got.Bounds().Dy(); y++ {
							for x := 0; x < got.Bounds().Dx(); x++ {
								if got.RGBAAt(x, y) != want.RGBAAt(x, y) {
									t.Fatalf("frame%d active%v mismatch %d,%d got%v want%v", frame, active, x, y, got.RGBAAt(x, y), want.RGBAAt(x, y))
								}
							}
						}
					}
					if !bytes.Equal(cpu, c.Img.Pix) || c.Output(false) != c.Img {
						t.Fatal("CPU changed")
					}
					if !active && !bytes.Equal(m.Screen.Pix, base.Pix) {
						t.Fatal("restore differs")
					}
				}
			})
		}
	}
}

func marchNativeFixture(slot int) *image.RGBA {
	out := image.NewRGBA(image.Rect(0, 0, 128, 128))
	for y := 0; y < 128; y++ {
		for x := 0; x < 128; x++ {
			out.SetRGBA(x, y, color.RGBA{byte(x + 17), byte(y + 21), byte(slot + 63), 255})
		}
	}
	return out
}

func TestHDMarchCacheAndLaterUI(t *testing.T) {
	art, e := MarchArt(artContainer(t, "DATA3"))
	if e != nil {
		t.Fatal(e)
	}
	p := &HDPack{images: map[[32]byte]*image.RGBA{}}
	for slot := 0; slot < 16; slot++ {
		p.images[hdImageKey(art[slot])] = marchNativeFixture(slot)
		high := p.maskedMarch(art[slot], art[16+slot%8])
		if high == nil || p.maskedMarch(art[slot], art[16+slot%8]) != high {
			t.Fatal("cache identity")
		}
		for y := 0; y < 128; y++ {
			for x := 0; x < 128; x++ {
				own := art[16+slot%8].At(x/4, y/4)&8 == 0 || art[slot].At(x/4, y/4)&15 != 0
				col := high.RGBAAt(x, y)
				if own {
					if col != p.images[hdImageKey(art[slot])].RGBAAt(x, y) {
						t.Fatal("native detail lost")
					}
				} else if col != (color.RGBA{}) {
					t.Fatal("hole not transparent premultiplied zero")
				}
			}
		}
	}
	if len(p.marchCache) != 16 {
		t.Fatal("cache entries", len(p.marchCache))
	}
	n := 0
	for _, im := range p.marchCache {
		n += len(im.Pix)
	}
	if n != 1<<20 {
		t.Fatal("cache bytes", n)
	}
	changed := art[0].Clone()
	changed.Pix[0] ^= 1
	p.images[hdImageKey(changed)] = marchNativeFixture(20)
	if p.maskedMarch(changed, art[16]) != nil || len(p.marchCache) != 16 {
		t.Fatal("cache exceeds cap")
	}
	if p.maskedMarch(nil, art[16]) != nil || p.maskedMarch(art[0], nil) != nil {
		t.Fatal("invalid source accepted")
	}
	l := NewMarchLayout(52, 92, 98, 107)
	base := &assets.Image{W: 640, H: 408, Pix: make([]byte, 640*408)}
	m := NewMarch(l, art, 4, base, nil)
	m.Step()
	c := testCanvasPx(t, 640, 408)
	c.HD = p
	DrawMarch(c, m)
	r := image.Rect(l.X, l.Y, l.X+l.W, l.Y+l.H)
	c.FillRect(r.Min.X, r.Min.Y, r.Max.X, r.Max.Y, fg)
	got := c.Output(true)
	for y := r.Min.Y * 4; y < r.Max.Y*4; y++ {
		for x := r.Min.X * 4; x < r.Max.X*4; x++ {
			if got.RGBAAt(x, y) != fg {
				t.Fatal("later UI hidden")
			}
		}
	}
}
