package main

import (
	"bytes"
	"image/color"
	"os"
	"path/filepath"
	"testing"

	"github.com/wicanr2/softworld_san1_remake/internal/ai"
	"github.com/wicanr2/softworld_san1_remake/internal/assets"
	"github.com/wicanr2/softworld_san1_remake/internal/font"
	"github.com/wicanr2/softworld_san1_remake/internal/game"
	"github.com/wicanr2/softworld_san1_remake/internal/menu"
	"github.com/wicanr2/softworld_san1_remake/internal/state"
	"github.com/wicanr2/softworld_san1_remake/internal/ui"
)

func TestMenuFontChoiceRendersKaiAndLi(t *testing.T) {
	read := func(name string) *font.Face {
		t.Helper()
		f, err := os.Open("../../fonts/" + name)
		if err != nil {
			t.Fatal(err)
		}
		defer f.Close()
		face, err := font.ParseHexGz(f, 16)
		if err != nil {
			t.Fatal(err)
		}
		return face
	}
	initial := read("unifont.hex.gz")
	a := &app{canvas: ui.NewCanvasPx(256, 16, initial), fontDir: "../../fonts"}
	m := menu.New(nil, state.EditionBase, ai.ModeBase, t.TempDir(), 5)
	m.OnFont = a.setMenuFont
	ink := color.RGBA{255, 255, 255, 255}
	background := color.RGBA{0, 0, 0, 255}
	for _, test := range []struct {
		choice, kind int
		file         string
	}{{2, game.FontKai, "kai.hex.gz"}, {3, game.FontLi, "li.hex.gz"}, {2, game.FontKai, "kai.hex.gz"}} {
		m.Confirm(test.choice)
		if a.fontKind != test.kind || m.Stage() != menu.Menu {
			t.Fatalf("選項 %d 的字型或畫面錯誤: %d", test.choice+1, a.fontKind)
		}
		a.canvas.Fill(background)
		a.canvas.DrawTextPx(0, 0, "曹操劉備三國演義", ink)
		expected := ui.NewCanvasPx(256, 16, read(test.file))
		expected.Fill(background)
		expected.DrawTextPx(0, 0, "曹操劉備三國演義", ink)
		if !bytes.Equal(a.canvas.Img.Pix, expected.Img.Pix) {
			t.Fatalf("選項 %d 未顯示完整 %s 字模", test.choice+1, test.file)
		}
	}
}

func TestTitleNewGamePreservesChosenFont(t *testing.T) {
	root := os.Getenv("SAN1_ORIG")
	if root == "" {
		root = "../../org_game"
	}
	for _, ed := range []struct {
		folder  string
		edition state.Edition
		mode    ai.Mode
	}{
		{"三國演義", state.EditionBase, ai.ModeBase},
		{"三國演義1加強版", state.EditionPlus, ai.ModePlus},
	} {
		t.Run(ed.folder, func(t *testing.T) {
			read := func(ext string) []byte {
				b, err := os.ReadFile(filepath.Join(root, ed.folder, "DATA2."+ext))
				if os.IsNotExist(err) {
					t.Skip("未掛原版素材")
				}
				if err != nil {
					t.Fatal(err)
				}
				return b
			}
			container, err := assets.OpenContainer(read("NAM"), read("IDX"), read("GRP"))
			if err != nil {
				t.Fatal(err)
			}
			for _, choice := range []int{2, 3} {
				fh, err := os.Open("../../fonts/unifont.hex.gz")
				if err != nil {
					t.Fatal(err)
				}
				initial, err := font.ParseHexGz(fh, 16)
				fh.Close()
				if err != nil {
					t.Fatal(err)
				}
				a := &app{canvas: ui.NewCanvasPx(256, 16, initial), fontDir: "../../fonts", fontCache: map[int]*font.Face{game.FontDefault: initial}}
				a.menuScreen = menu.New(container, ed.edition, ed.mode, t.TempDir(), 5)
				a.menuScreen.OnFont = a.setMenuFont
				for _, i := range []int{choice, 0, 0, 1, 0, 4} {
					a.titleConfirm(i)
				}
				if a.s == nil || a.menuScreen != nil || a.s.G.Options.Font != choice-1 {
					t.Fatal("新局遺失主選單字體")
				}
				a.setFont(game.FontDefault)
				if a.fontKind != game.FontDefault || a.s.G.Options.Font != game.FontDefault {
					t.Fatal("無法回到啟動字型")
				}
				a.canvas.Fill(color.RGBA{0, 0, 0, 255})
				a.canvas.DrawTextPx(0, 0, "劉備", color.RGBA{255, 255, 255, 255})
				want := ui.NewCanvasPx(256, 16, initial)
				want.Fill(color.RGBA{0, 0, 0, 255})
				want.DrawTextPx(0, 0, "劉備", color.RGBA{255, 255, 255, 255})
				if !bytes.Equal(a.canvas.Img.Pix, want.Img.Pix) {
					t.Fatal("預設字型未恢復")
				}
			}
		})
	}
}
