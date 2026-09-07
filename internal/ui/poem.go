package ui

import (
	"image"
	"image/draw"

	"github.com/wicanr2/softworld_san1_remake/internal/assets"
)

// 開場的詞。
//
// 底圖是原版的兩張 `SANTL`／`SANTR`（`assets.PoemScreen`）；字是 remake
// 自己的字庫畫的。
//
// **字本身讀自原版的畫面**：開場那一段的第一格還沒寫字，第二格寫完，
// 兩張相減就得到每一欄的位置——欄距 42 像素、列距 24 像素、字身 26×22，
// 最右邊那一欄是「詞曰」。原版把這些字畫成點陣，`DATA0`–`DATA3` 裡
// 找不到對應的 Big5 字串（`DATA0.GRP` 是打包過的執行檔），所以這裡是
// 照畫面謄的（`L1`）。
//
// 內容是楊慎的〈臨江仙〉，《三國演義》毛宗崗本的卷首詞。
//
// ⚠ **沒有英日譯文**：這是作品的引文不是介面用語，翻譯要另外處理，
// 所以不進 `internal/i18n` 的目錄。
var poemColumns = []string{
	"詞曰",
	"滾滾長江東逝水",
	"浪花淘盡英雄",
	"是非成敗轉頭空",
	"青山依舊在",
	"幾度夕陽紅",
	"白髮漁樵江渚上",
	"慣看秋月春風",
	"一壺濁酒喜相逢",
	"古今多少事",
	"都付笑談中",
}

// PoemColumns 由右到左回傳開場詞的每一欄。
func PoemColumns() []string {
	out := make([]string, len(poemColumns))
	copy(out, poemColumns)
	return out
}

// 版面：最右邊那一欄的左緣、欄距、第一個字的上緣、列距。都量自原版的
// 畫面（把寫字前後兩格相減，得到每一欄的字塊邊界）。
//
// 「詞曰」那一欄比詩低一列。字是**淺青（11）配黑影**——相減出來的
// 像素只有這兩個顏色。
const (
	poemRightX = 510
	poemStepX  = 42
	poemTopY   = 108
	poemStepY  = 24
	poemInk    = 11
)

// DrawImage 把一整張 640×350 的圖貼滿畫布。開場的三英圖
// （`assets.TitleArt`）就只是一張圖，沒有疊字。
func DrawImage(c *Canvas, im *assets.Image) {
	draw.Draw(c.Img, image.Rect(0, 0, assets.ScreenW, assets.ScreenH),
		im.RGBA(), image.Point{}, draw.Src)
}

// DrawPoem 畫開場詞：底圖用原版的，字由右到左直排。
//
// 原版的字身 26×22，remake 的字庫是 16×16，所以字比原版小一圈；
// 欄與列的位置照原版。
func DrawPoem(c *Canvas, im *assets.Image) {
	draw.Draw(c.Img, image.Rect(0, 0, assets.ScreenW, assets.ScreenH),
		im.RGBA(), image.Point{}, draw.Src)
	ink, shadow := assets.EGAPalette[poemInk], assets.EGAPalette[0]
	for i, col := range poemColumns {
		x := poemRightX - i*poemStepX
		top := poemTopY
		if i == 0 {
			top += poemStepY // 「詞曰」比詩低一列
		}
		for k, r := range []rune(col) {
			y := top + k*poemStepY
			c.DrawTextPx(x+1, y+1, string(r), shadow)
			c.DrawTextPx(x, y, string(r), ink)
		}
	}
}
