package main

import (
	_ "embed"
	"fmt"
	"image/color"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/wicanr2/softworld_san1_remake/internal/cells"
	"github.com/wicanr2/softworld_san1_remake/internal/font"
	"github.com/wicanr2/softworld_san1_remake/internal/ui"
)

// GNU Unifont 子集，來源為 fonts/unifont.hex.gz。
// 授權見 fonts/LICENSE-unifont.txt；不含原版字模。
//
//go:embed startup.hex
var startupHex string

type startupPaths struct{ root, font, saves, hd string }

func defaultStartupPaths(executable, edition string) startupPaths {
	dir := filepath.Dir(executable)
	return startupPaths{
		root:  filepath.Join(dir, "game", edition),
		font:  filepath.Join(dir, "fonts", "unifont.hex.gz"),
		saves: filepath.Join(dir, "saves"),
		hd:    filepath.Join(dir, "hd-assets"),
	}
}

type preparedApp struct {
	app      *app
	activate func()
}

type startupResult struct {
	prepared *preparedApp
	err      error
}

// startupWindow 的畫布及圖像只由遊戲迴圈持有。背景只送階段與完成結果。
type startupWindow struct {
	load           func(func(string)) (*preparedApp, error)
	result         chan startupResult
	phases         chan string
	canvas         *ui.Canvas
	image          *ebiten.Image
	game           *app
	err            error
	phase, root    string
	scale, ticks   int
	drawn, started bool
}

func newStartupWindow(root string, scale int, load func(func(string)) (*preparedApp, error)) *startupWindow {
	face, err := font.ParseHex(strings.NewReader(startupHex), 16)
	if err != nil {
		panic(err)
	} // 內嵌且經測試的字型，與玩家檔案無關。
	return &startupWindow{root: root, scale: max(1, scale), load: load,
		result: make(chan startupResult, 1), phases: make(chan string, 16),
		canvas: ui.NewCanvasPx(640, 400, face), phase: "準備載入"}
}

func (w *startupWindow) Update() error {
	if w.game != nil {
		return w.game.Update()
	}
	w.ticks++
	if w.err != nil {
		if ebiten.IsKeyPressed(ebiten.KeyEscape) {
			return ebiten.Termination
		}
		return nil
	}
	// 第一張載入畫面繪出後才讀外部檔案，避免視窗出現前就卡住。
	if w.drawn && !w.started {
		w.started = true
		go func() {
			previous := time.Now()
			phase := "準備載入"
			prepared, err := w.load(func(next string) {
				fmt.Fprintf(os.Stderr, "san1 startup: %s %.3fs\n", phase, time.Since(previous).Seconds())
				previous, phase = time.Now(), next
				select {
				case w.phases <- next:
				default:
				}
			})
			fmt.Fprintf(os.Stderr, "san1 startup: %s %.3fs\n", phase, time.Since(previous).Seconds())
			w.result <- startupResult{prepared, err}
		}()
	}
	for {
		select {
		case phase := <-w.phases:
			w.phase = phase
		default:
			goto drained
		}
	}
drained:
	select {
	case result := <-w.result:
		if result.err != nil {
			w.err = result.err
			fmt.Fprintln(os.Stderr, "san1:", w.err)
			ebiten.SetWindowTitle("三國演義：無法啟動")
			return nil
		}
		result.prepared.activate()
		w.game = result.prepared.app
		b := w.game.canvas.Img.Bounds()
		ebiten.SetWindowSize(b.Dx()*w.scale, b.Dy()*w.scale)
		title := "三國演義 remake"
		if lord := w.game.s.G.Lord(w.game.s.Player); lord != nil {
			title = fmt.Sprintf("三國演義 remake ── %s（AI：%s）", lord.Name, w.game.s.Brain.Name())
		}
		ebiten.SetWindowTitle(title)
	default:
	}
	return nil
}

func (w *startupWindow) Draw(dst *ebiten.Image) {
	if w.game != nil {
		w.game.Draw(dst)
		return
	}
	w.drawn = true
	w.drawStatus()
	if w.image == nil {
		w.image = ebiten.NewImage(640, 400)
	}
	w.image.WritePixels(w.canvas.Img.Pix)
	dst.DrawImage(w.image, nil)
}

func (w *startupWindow) Layout(width, height int) (int, int) {
	if w.game != nil {
		return w.game.Layout(width, height)
	}
	return 640, 400
}

func (w *startupWindow) drawStatus() {
	c := w.canvas
	ink := color.RGBA{232, 214, 160, 255}
	c.Fill(color.RGBA{22, 38, 40, 255})
	c.DrawText(5, 4, "三國演義 Remastered", ink)
	c.DrawText(5, 6, releaseVersion, ink)
	text := "正在載入，請稍候"
	if w.err != nil {
		text = "無法啟動遊戲"
	}
	c.DrawText(5, 9, text, ink)
	if w.err == nil {
		c.DrawText(5, 11, w.phase, ink)
		c.FillRect(40, 215, 600, 219, color.RGBA{55, 70, 70, 255})
		x := 40 + (w.ticks*4)%480
		c.FillRect(x, 215, x+80, 219, ink)
	} else {
		// 限制行數及寬度，長錯誤與路徑不能蓋住離開提示。
		for i, line := range startupLines(w.err.Error(), 70, 5) {
			c.DrawText(5, 11+i, line, ink)
		}
		c.DrawText(5, 17, "請確認遊戲資料與 fonts 目錄完整。", ink)
		c.DrawText(5, 21, "按 Esc 或關閉視窗離開", ink)
		c.DrawText(5, 18, "資料目錄：", ink)
		for i, line := range startupLines(w.root, 70, 2) {
			c.DrawText(5, 19+i, line, ink)
		}
	}
}

func startupLines(text string, width, limit int) []string {
	var lines []string
	line, used := "", 0
	for _, r := range text {
		n := cells.RuneWidth(r)
		if r == '\n' || used+n > width {
			lines = append(lines, line)
			line, used = "", 0
			if len(lines) == limit {
				last := []rune(lines[limit-1])
				for cells.Width(string(last))+3 > width && len(last) > 0 {
					last = last[:len(last)-1]
				}
				lines[limit-1] = string(last) + "..."
				return lines
			}
			if r == '\n' {
				continue
			}
		}
		line += string(r)
		used += n
	}
	if line != "" {
		lines = append(lines, line)
	}
	return lines
}
