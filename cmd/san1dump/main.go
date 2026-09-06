// Command san1dump 把原版的劇本資料印出來。
//
// 這支的用途是**把整條管線接起來驗一次**：容器 → 劇本表 → 排版。
// 它不是遊戲，是「讀得對不對」的最短路徑；出現任何怪東西，
// 在這裡就會看到，不必等引擎畫出來。
//
// ⚠ **本儲存庫不含任何原版檔案**，`-root` 由玩家自備。
//
//	go run ./cmd/san1dump -root path/to/三國演義 -slot 001
package main

import (
	"flag"
	"fmt"
	imgpng "image/png"
	"os"
	"path/filepath"

	"github.com/wicanr2/softworld_san1_remake/internal/ai"
	"github.com/wicanr2/softworld_san1_remake/internal/assets"
	"github.com/wicanr2/softworld_san1_remake/internal/cells"
	"github.com/wicanr2/softworld_san1_remake/internal/font"
	"github.com/wicanr2/softworld_san1_remake/internal/game"
	"github.com/wicanr2/softworld_san1_remake/internal/session"
	"github.com/wicanr2/softworld_san1_remake/internal/state"
	"github.com/wicanr2/softworld_san1_remake/internal/ui"
)

func main() {
	root := flag.String("root", "", "原版遊戲目錄（必填，玩家自備）")
	slot := flag.String("slot", "001", "劇本或存檔槽：001..006、SV1..SV6")
	what := flag.String("what", "all", "印什麼：pref／gen／master／all")
	cols := flag.Int("cols", 6, "郡名槽位寬度（半形格），用來檢查裝不裝得下")
	png := flag.String("png", "", "把畫面存成 PNG（無頭環境驗版面用）")
	screen := flag.String("screen", "list", "畫哪一張：list（州郡一覽）／main（遊戲主畫面）")
	faction := flag.Int("faction", -1, "main 畫面的玩家勢力；−1 ＝ 用第一個在用的勢力")
	sel := flag.Int("sel", 0, "main 畫面訊息欄要顯示哪一個郡；0 ＝ 玩家的第一個郡")
	months := flag.Int("months", 0, "main 畫面先讓電腦跑幾個月再畫")
	aiMode := flag.String("ai", "enhanced", "電腦 AI：base／plus／enhanced")
	fontPath := flag.String("font", "fonts/unifont.hex.gz", "點陣字型（-png 時才需要）")
	flag.Parse()

	if *root == "" {
		flag.Usage()
		os.Exit(2)
	}

	c, err := openData2(*root)
	if err != nil {
		die(err)
	}
	sc, err := state.LoadScenario(c, state.Slot(*slot))
	if err != nil {
		die(err)
	}
	fmt.Printf("劇本 %s（來源 %s）\n\n", *slot, *root)

	if *png != "" {
		if err := writePNG(*png, *fontPath, sc, *slot, *screen, *aiMode, *faction, *sel, *months); err != nil {
			die(err)
		}
		fmt.Printf("畫面存到 %s\n\n", *png)
	}

	if *what == "pref" || *what == "all" {
		fmt.Printf("州郡（%d 個，編號 1-based）\n", len(sc.Prefectures()))
		for i, p := range sc.Prefectures() {
			fmt.Printf("  %2d %s", p.ID, cells.Pad(p.Name, *cols))
			if (i+1)%6 == 0 {
				fmt.Println()
			}
		}
		fmt.Println()
		// 槽位檢查：多語系時英文譯名塞不回來的，在這裡就看得到。
		over := 0
		for _, p := range sc.Prefectures() {
			if !cells.Fits(p.Name, *cols) {
				over++
			}
		}
		fmt.Printf("  槽寬 %d 格：%d 個郡名放不下\n\n", *cols, over)
	}

	if *what == "gen" || *what == "all" {
		people := sc.People()
		fmt.Printf("人物（%d 個槽，其中 %d 個是人）\n", len(sc.Generals()), len(people))
		for i, g := range people {
			fmt.Printf("  %s", cells.Pad(g.Name, 8))
			if (i+1)%8 == 0 {
				fmt.Println()
			}
		}
		fmt.Println()
		byLen := map[int]int{}
		for _, g := range people {
			byLen[len([]rune(g.Name))]++
		}
		fmt.Printf("  字數分佈 %v\n\n", byLen)
	}

	if *what == "master" || *what == "all" {
		fmt.Printf("諸侯（%d 位）——欄位版面未解，只印原始 bytes 前 16 個\n",
			len(sc.Masters()))
		for _, m := range sc.Masters() {
			fmt.Printf("  [%2d] % X\n", m.Index, m.Raw[:16])
		}
	}
}

// writePNG 把引擎會畫的那一張畫面存成圖。
//
// **和 cmd/san1 畫的是同一張**（都走 ui.DrawPrefectureList）。
// 畫面 bug 測試看不到，但存成圖就看得到，而且無頭環境也產得出來。
func writePNG(out, fontPath string, sc *state.Scenario, slot, screen, aiMode string, faction, sel, months int) error {
	fh, err := os.Open(fontPath)
	if err != nil {
		return err
	}
	face, err := font.ParseHexGz(fh, 16)
	fh.Close()
	if err != nil {
		return err
	}
	c := ui.NewCanvas(ui.Cols, ui.Rows, face)
	switch screen {
	case "main":
		f := faction
		if f < 0 {
			act := sc.ActiveFactions()
			if len(act) == 0 {
				return fmt.Errorf("劇本 %s 裡沒有在用的勢力", slot)
			}
			f = act[0]
		}
		g, err := game.New(sc, state.FactionID(f), 5)
		if err != nil {
			return err
		}
		brain, err := ai.New(ai.Mode(aiMode))
		if err != nil {
			return err
		}
		s := session.New(g, brain, state.FactionID(f))
		for i := 0; i < months; i++ {
			s.EndMonth()
		}
		if sel == 0 {
			if t := g.Territory(state.FactionID(f)); len(t) > 0 {
				sel = t[0]
			}
		}
		ui.DrawSession(c, g, s.Log, ui.View{Sel: sel, Over: s.Over})
	case "list":
		ui.DrawPrefectureList(c, sc, slot)
	default:
		return fmt.Errorf("不認識的畫面 %q（收 list 或 main）", screen)
	}
	if n := len(c.Missing); n > 0 {
		fmt.Fprintf(os.Stderr, "⚠ %d 個字沒有字模：%q\n", n, string(missingRunes(c)))
	}
	f, err := os.Create(out)
	if err != nil {
		return err
	}
	defer f.Close()
	return imgpng.Encode(f, c.Img)
}

func missingRunes(c *ui.Canvas) []rune {
	var rs []rune
	for r := range c.Missing {
		rs = append(rs, r)
	}
	return rs
}

// openData2 開 DATA2 容器。三個檔缺一不可。
func openData2(root string) (*assets.Container, error) {
	base := filepath.Join(root, "DATA2")
	var parts [3][]byte
	for i, ext := range []string{".NAM", ".IDX", ".GRP"} {
		b, err := os.ReadFile(base + ext)
		if err != nil {
			return nil, fmt.Errorf("讀 DATA2%s：%w", ext, err)
		}
		parts[i] = b
	}
	return assets.OpenContainer(parts[0], parts[1], parts[2])
}

func die(err error) {
	fmt.Fprintln(os.Stderr, "san1dump:", err)
	os.Exit(1)
}
