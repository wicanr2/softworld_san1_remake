package ui

import (
	"image"

	"github.com/wicanr2/softworld_san1_remake/internal/assets"
)

// 場景圖的特效（`0x32dfa(x, y)`，`docs/spec/010`）：呼叫端先把一張
// 176×96 的 `SCG##.IMG` 載進來，這一支把它畫在顯示記憶體的第二頁
// （`es:0x2e78(1)`、`0x36c9:0x426(x, y)`），再由 `0x32e40` 擲 `RND(4)` 挑
// 四種拉幕之一，逐步從第二頁搬到第一頁（`es:0x3efc`，每一步一聲音效）。
// 走完之後那一塊就是整張場景圖。四十八個呼叫端傳的 (x, y) 只有三種：
// 主畫面右側面板 (432,80)、計略的 (432,120)、主戰場第三塊面板 (448,268)。

// sceneAllSteps 傳給 DrawScene 就是「走完」。
const sceneAllSteps = 1 << 20

// SceneRect 是場景圖落在 (x, y) 時蓋到的那一塊。
func SceneRect(x, y int) image.Rectangle {
	return image.Rect(x, y, x+assets.SceneW, y+assets.SceneH)
}

// NewSceneWipe 造「把場景圖拉進 (x, y)」的那一段拉幕：To 是一張與畫布
// 同尺寸、只在那一塊放著場景圖的圖，其餘的像素不會被搬到（`Wipe.Advance`
// 只搬 Rect 裡的）。scene 為 nil 回 nil。
func NewSceneWipe(c *Canvas, scene *assets.Image, kind WipeKind, x, y int) *Wipe {
	if scene == nil || c == nil {
		return nil
	}
	to := image.NewRGBA(c.Img.Bounds())
	for sy := 0; sy < scene.H; sy++ {
		for sx := 0; sx < scene.W; sx++ {
			if image.Pt(x+sx, y+sy).In(to.Bounds()) {
				to.SetRGBA(x+sx, y+sy, assets.EGAPalette[scene.Pix[sy*scene.W+sx]&15])
			}
		}
	}
	return &Wipe{Kind: kind, Rect: SceneRect(x, y).Intersect(to.Bounds()), From: c.Img, To: to}
}

// DrawScene 把場景圖拉進 (x, y) 走到第 step 步（1 起算）的樣子畫在畫布上；
// step 大於總步數就是走完（整張場景圖）。回傳總步數。
func DrawScene(c *Canvas, a *ArtScreen, n int, kind WipeKind, x, y, step int) int {
	if a == nil {
		return 0
	}
	w := NewSceneWipe(c, a.Scene(n), kind, x, y)
	if w == nil {
		return 0
	}
	for i := 0; i < step && w.Advance(c.Img); i++ {
	}
	return w.Steps()
}
