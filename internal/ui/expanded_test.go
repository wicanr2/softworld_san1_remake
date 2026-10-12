package ui

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/wicanr2/softworld_san1_remake/internal/assets"
	"github.com/wicanr2/softworld_san1_remake/internal/game"
)

func TestExpandedPortraitValidationAndMirror(t *testing.T) {
	dir, container, e, _, high := hdFixture(t)
	hdManifest(t, dir, []HDEntry{e})
	entry := expandedArtEntry{Portrait: 492, File: e.File, SHA256: e.SHA256, Width: e.Width, Height: e.Height}
	write := func(entries []expandedArtEntry) {
		t.Helper()
		b, err := json.Marshal(expandedArtManifest{Schema: 1, Entries: entries})
		if err != nil {
			t.Fatal(err)
		}
		if err = os.WriteFile(filepath.Join(dir, "expanded-portraits.json"), b, 0644); err != nil {
			t.Fatal(err)
		}
	}
	write([]expandedArtEntry{entry})
	p, err := LoadHDPack(dir, "base", map[string]*assets.Container{"DATA3": container})
	if err != nil || p.ExpandedCount != 1 || len(p.Warnings) != 0 {
		t.Fatalf("新增頭像未載入：%+v %v", p, err)
	}
	native := assets.ExpandedPortrait(492)
	if got := p.images[hdImageKey(native.Mirror())].RGBAAt(0, 0); got != high.RGBAAt(255, 0) {
		t.Fatal("新增頭像的戰役鏡像未載入")
	}
	for _, kind := range []string{"hash", "path", "size", "portrait", "duplicate"} {
		t.Run(kind, func(t *testing.T) {
			bad := entry
			switch kind {
			case "hash":
				bad.SHA256 = "wrong"
			case "path":
				bad.File = "../face.png"
			case "size":
				bad.Width++
			case "portrait":
				bad.Portrait = 1000
			}
			entries := []expandedArtEntry{bad}
			if kind == "duplicate" {
				entries = append(entries, bad)
			}
			write(entries)
			p, err := LoadHDPack(dir, "base", map[string]*assets.Container{"DATA3": container})
			if err != nil || p.ExpandedCount != 0 || len(p.Warnings) != len(entries) || p.images[hdImageKey(native)] != nil {
				t.Fatalf("破損頭像未回退：%+v %v", p, err)
			}
		})
	}
}

func TestExpandedPortraitIdentitiesAndFactionFills(t *testing.T) {
	seen := map[[32]byte]bool{}
	for i := 0; i < 500; i++ {
		im := assets.ExpandedPortrait(i)
		key := hdImageKey(im)
		if im == nil || im.W != 64 || im.H != 80 || seen[key] {
			t.Fatalf("新增肖像 %d 缺圖或來源鍵碰撞", i)
		}
		seen[key] = true
	}
	a := &ArtScreen{fills: &[16]assets.FillPattern{}}
	patterns := map[assets.FillPattern]bool{}
	for i := 0; i < 24; i++ {
		p := a.factionFill(i)
		if i < 16 {
			if p != &a.fills[i] {
				t.Fatal("原勢力圖樣被替換")
			}
			continue
		}
		if p == nil || patterns[*p] {
			t.Fatal("新增勢力圖樣重複")
		}
		patterns[*p] = true
	}
}

func TestExpandedFourCharacterBubbleName(t *testing.T) {
	for _, left := range []bool{true, false} {
		c := NewCanvasPx(assets.ScreenW, assets.ScreenH, testFace(t))
		b := &game.Bubble{X1: 424, Y1: 80, X2: 615, Y2: 175, Left: left, Text: "久仰"}
		DrawBubbleAs(c, nil, b, "南華老仙", 1499)
		x, ink := b.X1, bubbleNameLeft
		if !left {
			x, ink = b.X2-63, bubbleNameRight
		}
		want := NewCanvasPx(16, 16, testFace(t))
		want.FillRect(0, 0, 16, 16, bubbleBlack)
		want.DrawTextPx(0, 0, "仙", ink)
		for py := 0; py < 16; py++ {
			for px := 0; px < 16; px++ {
				if c.Img.RGBAAt(x+48+px, 160+py) != want.Img.RGBAAt(px, py) {
					t.Fatal("四字姓名末字被截斷")
				}
			}
		}
	}
}
