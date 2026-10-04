package main

import (
	"bytes"
	"image/color"
	"os"
	"testing"

	"github.com/wicanr2/softworld_san1_remake/internal/ai"
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
