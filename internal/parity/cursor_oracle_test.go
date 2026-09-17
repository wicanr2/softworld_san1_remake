//go:build oracle

package parity

import (
	"testing"

	"github.com/wicanr2/dosgolem/oracle"

	"github.com/wicanr2/softworld_san1_remake/internal/ui"
)

// 輸入游標（回呼槽 0 `0x1058:0x7fa`，Issue #78，`docs/spec/014` §4.1）。
const (
	cursorCallbackFn = 0x10d7a // 回呼本體：n＝0 存底、0x63 換格、−1 收掉
	cursorPutFn      = 0x37158 // `0x36c9:0x4c8(x, y, 圖, 模式)`
	cursorShownAt    = 0x110e9 // 換格那一支把那一格搬回顯示頁之後
	cursorFrameVar   = 0x5c02  // `DS:0x5c02` 目前的畫格
	cursorImageBase  = 0x50    // `CURA0` 在圖表裡的號碼
	cursorImageCount = 4 * 12  // 四組 × （六格圖 ＋ 六格遮罩）
)

// cursorTrack 是原版游標此刻在畫面上的樣子。
type cursorTrack struct {
	Shown                 bool
	X, Y                  int
	Style, Kind           int // 組（0＝A…3＝D）與畫格
	Draws                 int
	pendX, pendY, pendImg int
}

// trackCursor 掛上三個路標：換格時記下貼的圖與位置，搬回顯示頁才算看得到；
// `n＝−1` 收掉。**看得到的是最後一次搬完的那一格**——遮罩與圖貼在另一頁，
// 中途取樣的畫面還是上一格。
func trackCursor(o *oracle.Oracle) *cursorTrack {
	tr := &cursorTrack{pendImg: -1}
	o.OnCall(addr(cursorCallbackFn), func(o *oracle.Oracle) {
		if int16(o.Arg(0)) == -1 {
			tr.Shown = false
		}
	})
	o.OnCall(addr(cursorPutFn), func(o *oracle.Oracle) {
		img := int(o.Arg(2))
		if o.Arg(3) == 2 && img >= cursorImageBase && img < cursorImageBase+cursorImageCount {
			tr.pendX, tr.pendY, tr.pendImg = int(o.Arg(0)), int(o.Arg(1)), img
		}
	})
	o.OnCall(addr(cursorShownAt), func(o *oracle.Oracle) {
		if tr.pendImg < 0 {
			return
		}
		k := tr.pendImg - cursorImageBase
		tr.Shown, tr.X, tr.Y = true, tr.pendX, tr.pendY
		tr.Style, tr.Kind = k/12, k%12
		tr.Draws++
		tr.pendImg = -1
	})
	return tr
}

// waitCursorShown 讓原版跑到游標再搬完一格，取樣的相位因此固定在「剛畫好」。
func waitCursorShown(t *testing.T, o *oracle.Oracle, tr *cursorTrack, name string) {
	t.Helper()
	before := tr.Draws
	if err := o.RunUntil(oracle.NewCond(name+"的游標畫好", func(*oracle.Oracle) bool {
		return tr.Draws > before
	}), oracle.Budget(20_000_000)); err != nil {
		t.Fatalf("%s：游標一直沒畫：%v", name, err)
	}
}

// input 是 remake 這一邊要畫的游標：原版看得到才畫，畫格照原版。
func (tr cursorTrack) input() ui.InputCursor {
	return ui.InputCursor{On: tr.Shown, Frame: tr.Kind}
}

// compareCursorCell 比原版游標那 8×16 格：組別要是 style，與 remake 逐像素相同；
// 反對照是 remake 不畫游標的同一張（plain）——那一格必須和原版不同，
// 否則位置或相位錯了也會「相同」。
func compareCursorCell(t *testing.T, name string, tr cursorTrack, style int,
	org, mine, plain func(x, y int) int) {
	t.Helper()
	if !tr.Shown {
		t.Logf("%s：原版這一刻沒有游標（讀鍵還沒開始或剛收掉）", name)
		return
	}
	if tr.Style != style {
		t.Errorf("%s：原版游標是第 %d 組，該是第 %d 組", name, tr.Style, style)
	}
	bad, ink := 0, 0
	for y := tr.Y; y < tr.Y+16; y++ {
		for x := tr.X; x < tr.X+8; x++ {
			if org(x, y) != plain(x, y) {
				ink++
			}
			if org(x, y) != mine(x, y) {
				bad++
			}
		}
	}
	if ink == 0 {
		t.Errorf("%s：原版 (%d,%d) 那一格和不畫游標的 remake 一樣，游標路標沒對上", name, tr.X, tr.Y)
	}
	if bad != 0 {
		t.Errorf("%s：游標 (%d,%d) 第 %d 格 8×16 有 %d 點不同", name, tr.X, tr.Y, tr.Kind, bad)
		return
	}
	t.Logf("%s：游標 (%d,%d) 第 %d 格逐像素相同（%d 點與不畫游標時不同）", name, tr.X, tr.Y, tr.Kind, ink)
}
