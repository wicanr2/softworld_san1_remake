package ui

import (
	"bytes"
	"crypto/sha256"
	"encoding/binary"
	"fmt"
	"image"
	"image/color"
	"os"
	"path/filepath"
	"testing"

	"github.com/wicanr2/softworld_san1_remake/internal/assets"
	"github.com/wicanr2/softworld_san1_remake/internal/game"
	"github.com/wicanr2/softworld_san1_remake/internal/state"
)

func hdMapContainers(t *testing.T, edition string) map[string]*assets.Container {
	t.Helper()
	root := os.Getenv("SAN1_ORIG")
	if root == "" {
		t.Skip("沒設 SAN1_ORIG，跳過原版來源驗證")
	}
	folder := "三國演義"
	if edition == "plus" {
		folder = "三國演義1加強版"
	}
	out := map[string]*assets.Container{}
	for _, name := range []string{"DATA2", "DATA3"} {
		var data [3][]byte
		for i, ext := range []string{"NAM", "IDX", "GRP"} {
			b, err := os.ReadFile(filepath.Join(root, folder, name+"."+ext))
			if err != nil {
				t.Fatal(err)
			}
			data[i] = b
		}
		c, err := assets.OpenContainer(data[0], data[1], data[2])
		if err != nil {
			t.Fatal(err)
		}
		out[name] = c
	}
	return out
}

func TestHDMapEntryValidation(t *testing.T) {
	for _, name := range []string{"MAINMAP4.IMG", "MAINMAP5.IMG"} {
		for _, check := range []string{"valid", "shape", "alpha", "container", "source", "png-hash", "missing"} {
			t.Run(name+"/"+check, func(t *testing.T) {
				im := &assets.Image{W: 168, H: 336, Pix: make([]byte, 168*336)}
				if check == "shape" {
					im.W = 160
					im.Pix = make([]byte, im.W*im.H)
				}
				dir, cont, entry, _ := hdMenuFixture(t, name, im, check == "alpha")
				switch check {
				case "container":
					entry.Container = "DATA1"
				case "source":
					entry.SourceSHA256 = "wrong"
				case "png-hash":
					entry.SHA256 = "wrong"
				case "missing":
					if err := os.Remove(filepath.Join(dir, entry.File)); err != nil {
						t.Fatal(err)
					}
				}
				_, _, err := loadHDEntry(dir, entry, map[string]*assets.Container{"DATA3": cont, "DATA1": cont})
				if (err == nil) != (check == "valid") {
					t.Fatalf("%s: %v", check, err)
				}
			})
		}
	}
}

func TestHDWorldMapSourceAndFallback(t *testing.T) {
	for _, edition := range []string{"base", "plus"} {
		for _, check := range []string{"valid", "missing-data2", "missing-data3", "unknown-map", "cross-scenario", "outside-seed", "duplicate-seed"} {
			t.Run(edition+"/"+check, func(t *testing.T) {
				cont := hdMapContainers(t, edition)
				switch check {
				case "missing-data2":
					delete(cont, "DATA2")
				case "missing-data3":
					delete(cont, "DATA3")
				case "unknown-map":
					i, _ := cont["DATA3"].ByName("MAINMAP4.IMG")
					cont["DATA3"].Data(i)[4] ^= 1
				case "cross-scenario", "outside-seed":
					i, _ := cont["DATA2"].ByName("BASESTA.006")
					raw := cont["DATA2"].Data(i)
					if check == "outside-seed" {
						binary.LittleEndian.PutUint16(raw[176+6:], 65535)
					} else {
						binary.LittleEndian.PutUint16(raw[176+6:], binary.LittleEndian.Uint16(raw[176+6:])+1)
					}
				case "duplicate-seed":
					for _, slot := range []string{"001", "002", "003", "004", "005", "006"} {
						i, _ := cont["DATA2"].ByName("BASESTA." + slot)
						raw := cont["DATA2"].Data(i)
						copy(raw[352+6:352+10], raw[176+6:176+10])
					}
				}
				mask, err := hdWorldMapCoverage(cont)
				if check != "valid" {
					if err == nil || mask != nil {
						t.Fatal("未知來源未回退", check)
					}
					return
				}
				if err != nil {
					t.Fatal(err)
				}
				count := 0
				for _, v := range mask {
					if v {
						count++
					}
				}
				if len(mask) != 336*336 || count != 61874 {
					t.Fatal("保護面積", count)
				}
				// 獨立走全部白色郡，確認每個來源格與標題墨點皆有保護。
				bg, err := assets.MainScreen(cont["DATA3"])
				if err != nil {
					t.Fatal(err)
				}
				sc, err := state.LoadScenario(cont["DATA2"], state.Scenario1)
				if err != nil {
					t.Fatal(err)
				}
				for _, p := range sc.Prefectures() {
					bg.FloodFill(int(p.MapX)+80, int(p.MapY)+44, 254)
				}
				for y := 0; y < 336; y++ {
					for x := 0; x < 336; x++ {
						code := bg.At(x+72, y+36)
						if (code == 254 || (x >= 78 && x < 178 && y >= 31 && y < 65 && code != 1)) && !mask[y*336+x] {
							t.Fatalf("來源格漏保護 %d,%d", x, y)
						}
					}
				}
			})
		}
	}
}

func TestHDWorldMapLoadAndPartialPack(t *testing.T) {
	for _, edition := range []string{"base", "plus"} {
		cont := hdMapContainers(t, edition)
		i, _ := cont["DATA3"].ByName("MAINMAP4.IMG")
		im, err := assets.DecodeImage(cont["DATA3"].Data(i))
		if err != nil {
			t.Fatal(err)
		}
		dir, _, entry, _ := hdMenuFixture(t, "MAINMAP4.IMG", im, false)
		entry.Edition = edition
		entry.SourceSHA256 = fmt.Sprintf("%x", sha256.Sum256(cont["DATA3"].Data(i)))
		for _, check := range []string{"one-half", "missing-data2", "old-pack"} {
			t.Run(edition+"/"+check, func(t *testing.T) {
				entries := []HDEntry{entry}
				sources := map[string]*assets.Container{"DATA2": cont["DATA2"], "DATA3": cont["DATA3"]}
				if check == "missing-data2" {
					delete(sources, "DATA2")
				}
				if check == "old-pack" {
					entries = nil
				}
				hdManifest(t, dir, entries)
				pack, err := LoadHDPack(dir, edition, sources)
				if err != nil {
					t.Fatal(err)
				}
				count, warnings := 0, 0
				if check == "one-half" {
					count = 1
				}
				if check == "missing-data2" {
					warnings = 1
				}
				if pack.Count != count || len(pack.Warnings) != warnings {
					t.Fatal("逐項回退", pack.Count, pack.Warnings)
				}
			})
		}
	}
}

func TestHDWorldMapFullPixelsAndLaterForeground(t *testing.T) {
	for _, edition := range []string{"base", "plus"} {
		cont := hdMapContainers(t, edition)
		mask, err := hdWorldMapCoverage(cont)
		if err != nil {
			t.Fatal(err)
		}
		a, err := NewArtScreen(cont["DATA3"], nil, cont["DATA2"])
		if err != nil {
			t.Fatal(err)
		}
		pack := &HDPack{images: map[[32]byte]*image.RGBA{}, worldMapCoverage: mask}
		for _, p := range a.layers {
			if p.Name != "MAINMAP4.IMG" && p.Name != "MAINMAP5.IMG" {
				continue
			}
			high := image.NewRGBA(image.Rect(0, 0, 672, 1344))
			for y := 0; y < 1344; y++ {
				for x := 0; x < 672; x++ {
					high.SetRGBA(x, y, color.RGBA{byte(x + p.X*4), byte(y), 73, 255})
				}
			}
			pack.images[hdImageKey(p.Image)] = high
		}
		for _, slot := range []state.Slot{state.Scenario1, state.Scenario2, state.Scenario3, state.Scenario4, state.Scenario5, state.Scenario6} {
			sc, err := state.LoadScenario(cont["DATA2"], slot)
			if err != nil {
				t.Fatal(err)
			}
			g, err := game.New(sc, 1, 5, state.Edition(edition))
			if err != nil {
				t.Fatal(err)
			}
			for _, half := range []int{0, 1, 2} { // 完整、缺左、缺右。
				t.Run(fmt.Sprintf("%s/%s/missing%d", edition, slot, half), func(t *testing.T) {
					c := testCanvasPx(t, 640, 408)
					c.HD = pack
					c.drawRGBA(c.Img.Bounds(), a.Compose(g, 1).RGBA(), image.Point{})
					before := append([]byte(nil), c.Img.Pix...)
					var removed *image.RGBA
					var key [32]byte
					if half > 0 {
						key = hdImageKey(a.layers[half+2].Image)
						removed = pack.images[key]
						delete(pack.images, key)
						defer func() { pack.images[key] = removed }()
					}
					a.drawHighFrame(c)
					out := c.Output(true)
					if !bytes.Equal(before, c.Img.Pix) || c.Output(false) != c.Img {
						t.Fatal("原貌 CPU 變更")
					}
					for y := 0; y < 408; y++ {
						for x := 0; x < 640; x++ {
							mapPoint := x >= 72 && x < 408 && y >= 36 && y < 372
							protected := !mapPoint
							if mapPoint {
								protected = mask[(y-36)*336+x-72] || (half == 1 && x < 240) || (half == 2 && x >= 240)
							}
							for dy := 0; dy < 4; dy++ {
								for dx := 0; dx < 4; dx++ {
									want := c.Img.RGBAAt(x, y)
									if !protected {
										want = color.RGBA{byte(x*4 + dx), byte((y-36)*4 + dy), 73, 255}
									}
									if out.RGBAAt(x*4+dx, y*4+dy) != want {
										t.Fatalf("完整像素 %d,%d/%d,%d", x, y, dx, dy)
									}
								}
							}
						}
					}
					// 後畫前景必須保留四倍完整格，不能被地圖背景蓋回。
					col := color.RGBA{117, 39, 221, 255}
					c.FillRect(72, 36, 408, 37, col)
					out = c.Output(true)
					for y := 144; y < 148; y++ {
						for x := 288; x < 1632; x++ {
							if out.RGBAAt(x, y) != col {
								t.Fatal("後畫前景被覆蓋")
							}
						}
					}
				})
			}
		}
	}
}
