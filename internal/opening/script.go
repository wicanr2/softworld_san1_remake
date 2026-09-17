package opening

import (
	"fmt"
	"iter"
	"math/big"

	"github.com/wicanr2/softworld_san1_remake/internal/assets"
)

// Kind 是一拍要怎麼過。
type Kind int

const (
	// HoldSeconds 是 `0ad0:0f9c(n)`：等 DOS 時鐘的秒數跳 n 次，按鍵提早結束並清鍵。
	HoldSeconds Kind = iota
	// HoldTicks 是 `0ad0:1022(n)`：等百分之一秒那一欄跳 n 次——DOS 時鐘
	// 一個計時器中斷（每秒 18.2 次）跳一次，所以是 n 個 tick。按鍵同上。
	HoldTicks
	// Step 是動畫沒有延遲的一步：原版照 CPU 速度跑，每一步問一次鍵盤，
	// 有鍵就放棄片頭剩下的部分、直接到「程式載入中」。
	Step
	// WaitKey 是 `0ad0:00d0`：先清鍵，再一直等到有鍵，讀掉。
	WaitKey
	// End 是「程式載入中」那一張畫好了，片頭結束。
	End
)

// Site 標出這一拍在原版的哪一個等待點（對拍用它對齊兩邊的序列）。
type Site int

const (
	SiteTrademark   Site = iota // f9c(4)：商標
	SiteTitleFont               // f9c(3)：三國演義標題字
	SiteCredits                 // f9c(5)：製作人員
	SiteCreditsHold             // f9c(8)：製作人員（換到另一頁），背後正在畫海景
	SiteSea                     // 1022(20)：海景與船隊的第一格
	SiteBoat                    // 1022(RND(6)+5)：船隊每一格之前
	SiteBoatsHold               // 1022(200)：船隊最後一格
	SitePoemRow                 // 1022(1)：寫詞每六列
	SitePoemColumn              // 1022(3)：寫詞每一欄寫完
	SiteFade                    // 1022(n)：調色盤淡出
	SiteRibbon                  // 頭像橫幅每一步
	SiteScroll                  // 三英圖捲入每一步
	SiteTitleHold               // f9c(2)：三英圖停住
	SiteTitleKey                // 三英圖等鍵
	SiteLoading                 // 程式載入中
)

var siteNames = [...]string{"商標", "標題字", "製作人員", "製作人員（第二頁）", "海景",
	"船隊", "船隊停住", "寫詞六列", "寫詞一欄", "淡出", "頭像橫幅", "三英圖捲入",
	"三英圖停住", "三英圖等鍵", "程式載入中"}

func (s Site) String() string {
	if int(s) < len(siteNames) {
		return siteNames[s]
	}
	return fmt.Sprintf("Site(%d)", int(s))
}

// Beat 是片頭的一拍：原版在這一刻等待或問鍵盤，畫面是 Pages 顯示中的那一頁。
type Beat struct {
	Kind  Kind
	N     int // HoldSeconds／HoldTicks 的次數
	Site  Site
	Pages *Pages
}

// Art 是片頭用到的圖，全部來自 `DATA1`（`0ad0:011a` 讀的那一串）。
type Art struct {
	CMarkL, CMarkR, TitleFont *assets.Image
	Credits                   [3]*assets.Image // PRV0–PRV2
	SantL, SantR              *assets.Image
	Boats                     [4]*assets.Image // SANTBB／SANTBM1／SANTBM2／SANTBS（原版槽 2–5）
	// PoemInk 是詞的字（淺青 11、底 0），PoemMask 是它的反相（字 0、底 15）。
	// 原版是 `TZUE`／`TZUE1`，464×166、貼在 (88,106)；remake 用自己的字庫
	// 畫同樣大小的兩張（`FontPoem`）。
	PoemInk, PoemMask *assets.Image
	Faces             [50]*assets.Image // FaceIDs 那 50 張肖像
	Titl              [4]*assets.Image  // TITL0–TITL3
	Loads             *assets.Image
}

// FaceIDs 是頭像橫幅用的肖像編號，照原版的順序（`DS:1172`，50 個）。
var FaceIDs = [50]int{0, 5, 4, 2, 12, 13, 255, 62, 7, 47, 77, 63, 64, 52, 8, 10, 6, 14,
	19, 20, 247, 186, 249, 233, 229, 230, 101, 215, 184, 155, 54, 159, 143, 146, 138,
	120, 144, 81, 102, 214, 53, 156, 51, 149, 132, 160, 192, 172, 163, 248}

// LoadArt 從 `DATA1` 讀片頭的圖。詞的那兩張不在這裡——由呼叫端給
// （`FontPoem` 或 `OriginalPoem`）。
func LoadArt(c1 *assets.Container) (*Art, error) {
	get := func(name string) (*assets.Image, error) {
		i, ok := c1.ByName(name)
		if !ok {
			return nil, fmt.Errorf("opening: DATA1 裡沒有 %s", name)
		}
		return assets.DecodeImage(c1.Data(i))
	}
	a := &Art{}
	var err error
	for _, f := range []struct {
		dst  **assets.Image
		name string
	}{
		{&a.CMarkL, "CMARKL.IMG"}, {&a.CMarkR, "CMARKR.IMG"}, {&a.TitleFont, "TITFONT.IMG"},
		{&a.Credits[0], "PRV0.IMG"}, {&a.Credits[1], "PRV1.IMG"}, {&a.Credits[2], "PRV2.IMG"},
		{&a.SantL, "SANTL.IMG"}, {&a.SantR, "SANTR.IMG"},
		{&a.Boats[0], "SANTBB.IMG"}, {&a.Boats[1], "SANTBM1.IMG"},
		{&a.Boats[2], "SANTBM2.IMG"}, {&a.Boats[3], "SANTBS.IMG"},
		{&a.Titl[0], "TITL0.IMG"}, {&a.Titl[1], "TITL1.IMG"},
		{&a.Titl[2], "TITL2.IMG"}, {&a.Titl[3], "TITL3.IMG"},
		{&a.Loads, "LOADS.IMG"},
	} {
		if *f.dst, err = get(f.name); err != nil {
			return nil, err
		}
	}
	for i, id := range FaceIDs {
		if a.Faces[i], err = get(fmt.Sprintf("F%03d.FAC", id)); err != nil {
			return nil, err
		}
	}
	return a, nil
}

// OriginalPoem 回原版的 `TZUE`（字）與 `TZUE1`（遮罩）。對拍用。
func OriginalPoem(c1 *assets.Container) (ink, mask *assets.Image, err error) {
	for _, f := range []struct {
		dst  **assets.Image
		name string
	}{{&ink, "TZUE.IMG"}, {&mask, "TZUE1.IMG"}} {
		i, ok := c1.ByName(f.name)
		if !ok {
			return nil, nil, fmt.Errorf("opening: DATA1 裡沒有 %s", f.name)
		}
		if *f.dst, err = assets.DecodeImage(c1.Data(i)); err != nil {
			return nil, nil, err
		}
	}
	return ink, mask, nil
}

// 船隊（`DS:1060`–`DS:1130`）：用第幾張船、起點 x、y、每格位移的 19 倍。
var (
	boatArt = [8]int{0, 0, 1, 2, 3, 3, 1, 3}
	boatX0  = [8]int{111, 465, 162, 404, 15, 389, 300, 600}
	boatY   = [8]int{272, 276, 250, 248, 242, 238, 252, 235}
	boatDX  = [8]int{22, 22, 16, 11, 6, 6, 14, 8}
)

// BoatFrames 是船隊動畫的格數（`0ad0:0b96` 的迴圈）。
const BoatFrames = 22

// 寫詞（`DS:1130`–`DS:1172`）：每一欄揭露的 x 與列的範圍 [from, to)，由右到左。
var (
	poemRevealX    = [11]int{520, 464, 416, 376, 336, 296, 256, 212, 168, 128, 80}
	poemRevealFrom = [11]int{130, 104, 104, 104, 104, 104, 104, 104, 104, 104, 104}
	poemRevealTo   = [11]int{174, 274, 250, 274, 227, 227, 274, 250, 274, 227, 227}
)

// 詞貼在哪裡：遮罩兩次（先 +2,+2 再原位）做出黑影，字再 OR 上去。
const (
	PoemX = 88
	PoemY = 106
)

// fadeSteps 是 `0ad0:0d26` 的淡出：先等 wait 個 tick，再把 sets 裡的屬性暫存器改掉。
var fadeSteps = []struct {
	wait int
	sets [][2]byte // {暫存器, 值}
}{
	{10, [][2]byte{{4, 0x08}}}, {5, [][2]byte{{4, 0x00}}}, {5, [][2]byte{{6, 0x01}}},
	{5, [][2]byte{{6, 0x08}}}, {5, [][2]byte{{6, 0x00}}}, {4, [][2]byte{{12, 0x04}}},
	{4, [][2]byte{{12, 0x08}}}, {4, [][2]byte{{12, 0x00}}}, {3, [][2]byte{{13, 0x05}}},
	{3, [][2]byte{{13, 0x08}}}, {3, [][2]byte{{13, 0x00}}},
	{3, [][2]byte{{14, 0x07}, {15, 0x3e}}}, {2, [][2]byte{{14, 0x08}, {15, 0x07}}},
	{2, [][2]byte{{14, 0x00}, {15, 0x08}}}, {2, [][2]byte{{14, 0x00}, {15, 0x00}}},
}

// 頭像橫幅（`0ad0:096e`）：肖像從第 42 張起，換到第 134 張為止。
const (
	ribbonFirst = 42
	ribbonLast  = 134
	ribbonY     = 240 // 顯示的那一條 y 240..319
)

// Script 是一次片頭。Rand(n) 回 [0, n)，給船隊每一格前的等待用
// （原版是 MSC 的 `rand() % n`）。
type Script struct {
	Art  *Art
	Rand func(n int) int
	// Aborted 由播放端在 Step 那一拍收到按鍵時設起來：片頭剩下的部分跳過，
	// 直接到「程式載入中」（原版 `0ad0:055e` 回 −1）。
	Aborted bool
	Pages   *Pages
}

// Beats 從頭播一次片頭。
func (s *Script) Beats() iter.Seq[Beat] {
	return func(yield func(Beat) bool) {
		s.Pages = NewPages()
		p, a := s.Pages, s.Art
		stopped := false
		beat := func(k Kind, n int, site Site) bool {
			if stopped {
				return false
			}
			if !yield(Beat{Kind: k, N: n, Site: site, Pages: p}) {
				stopped = true
			}
			return !stopped
		}
		// step 回 false 表示這一段要放棄（按鍵或播放端停了）。
		step := func(site Site) bool {
			return beat(Step, 0, site) && !s.Aborted
		}

		// 0ad0:011a — 商標、標題字、製作人員。
		p.Draw = 1
		p.Clear(0)
		p.Show = 1
		p.Draw = 0
		p.Clear(0)
		p.Put(a.CMarkL, 0, 64, Copy)
		p.Put(a.CMarkR, 320, 64, Copy)
		p.Show = 0
		p.Draw = 1
		p.Clear(0)
		p.Put(a.TitleFont, 88, 122, Copy)
		if !beat(HoldSeconds, 4, SiteTrademark) {
			return
		}
		p.Show = 1
		p.Draw = 0
		p.Clear(0)
		for i, im := range a.Credits {
			p.Put(im, 64, 84+80*i, Copy)
		}
		if !beat(HoldSeconds, 3, SiteTitleFont) {
			return
		}
		p.Show = 0
		if !beat(HoldSeconds, 5, SiteCredits) {
			return
		}
		p.Draw = 1
		p.CopyPage(0, 1)
		p.Show = 1

		// 海景與船隊的第一格畫在第 0 頁，畫完才切過去。
		p.Draw = 0
		p.Put(a.SantL, 0, 49, Copy)
		p.Put(a.SantR, 320, 49, Copy)
		if !beat(HoldSeconds, 8, SiteCreditsHold) {
			return
		}
		x := [8]float64{}
		for i := range x {
			x[i] = float64(boatX0[i])
			p.Put(a.Boats[boatArt[i]], boatX0[i], boatY[i], And)
		}
		p.Show = 0
		p.CopyPage(0, 1)
		p.Show = 1
		p.Draw = 0

		// 0ad0:055e 播完或中途放棄都接「程式載入中」。
		s.sequence(beat, step, x)
		if stopped {
			return
		}
		p.Draw = 0
		p.Clear(0)
		p.Show = 0
		p.Draw = 1
		p.Clear(0)
		p.Put(a.Loads, 200, 180, Copy)
		p.Show = 1
		p.CopyPage(1, 0)
		beat(End, 0, SiteLoading)
	}
}

// sequence 是 `0ad0:055e`：船隊、寫詞、淡出、頭像橫幅、三英圖。回 false
// 表示中途放棄（按鍵）或播放端停了。
func (s *Script) sequence(beat func(Kind, int, Site) bool, step func(Site) bool, x [8]float64) bool {
	p, a := s.Pages, s.Art

	// 0ad0:0b34 — 船隊。
	p.Draw = 0
	p.Show = 1
	if !beat(HoldTicks, 20, SiteSea) {
		return false
	}
	for f := 0; f < BoatFrames; f++ {
		if !beat(HoldTicks, s.Rand(6)+5, SiteBoat) {
			return false
		}
		p.Put(a.SantL, 0, 49, Copy)
		p.Put(a.SantR, 320, 49, Copy)
		for i := range x {
			p.Put(a.Boats[boatArt[i]], int(x[i]), boatY[i], And)
			x[i] = boatStep(x[i], boatDX[i])
		}
		p.CopyPage(0, 1)
	}
	if !beat(HoldTicks, 200, SiteBoatsHold) {
		return false
	}

	// 0ad0:0c60 — 寫詞：字畫在第 0 頁，一列一列搬到顯示中的第 1 頁。
	p.Put(a.PoemMask, PoemX+2, PoemY+2, And)
	p.Put(a.PoemMask, PoemX, PoemY, And)
	p.Put(a.PoemInk, PoemX, PoemY, Or)
	for col := range poemRevealX {
		cx := poemRevealX[col]
		for row := poemRevealFrom[col]; row < poemRevealTo[col]; row++ {
			p.CopyRect(0, 1, cx, row, cx+39, row, cx, row)
			if row%6 == 0 && !beat(HoldTicks, 1, SitePoemRow) {
				return false
			}
		}
		if !beat(HoldTicks, 3, SitePoemColumn) {
			return false
		}
	}

	// 0ad0:0d26 — 調色盤淡出，換成黑底的詞（EGA 那一支；另一種繪圖卡不淡出）。
	for _, f := range fadeSteps {
		if !beat(HoldTicks, f.wait, SiteFade) {
			return false
		}
		for _, set := range f.sets {
			p.Pal[set[0]] = set[1]
		}
	}
	if !beat(HoldTicks, 2, SiteFade) {
		return false
	}
	p.Draw = 0
	p.Clear(0)
	p.Put(a.PoemInk, PoemX, PoemY, Or)
	p.CopyPage(0, 1)
	p.Pal = DefaultPal

	// 0ad0:057e — 三英圖先拼在第 0 頁，切成 80 條 8 像素寬的直條收起來。
	p.Draw = 0
	for i, im := range a.Titl {
		p.Put(im, 160*i, 0, Copy)
	}
	var strips [80]*assets.Image
	for k := range strips {
		strips[k] = p.Capture(8*k, 0, 8*k+7, assets.ScreenH-1)
	}
	p.CopyPage(1, 0)

	if !s.ribbon(step) {
		return false
	}
	return s.scroll(beat, step, strips)
}

// boatStep 是 `x += dx × (1/19)`，照 MSC 浮點模擬器的算法：乘與加在 80 位元
// 的暫存器裡做（64 位元尾數），存回 double 時才四捨五入到偶數。
func boatStep(x float64, dx int) float64 {
	const prec = 64
	c := new(big.Float).SetPrec(prec).SetFloat64(1.0 / 19)
	v := new(big.Float).SetPrec(prec).SetInt64(int64(dx))
	v.Mul(v, c)
	v.Add(v, new(big.Float).SetPrec(prec).SetFloat64(x))
	f, _ := v.Float64()
	return f
}

// ribbon 是 `0ad0:096e` 的頭像橫幅：第 0 頁的 y 0..79 排八張肖像、y 80..159
// 放下一張、y 160..239 存著橫幅那一條的底，每一步把底還原、肖像往左錯 8
// 像素拼進 y 240..319，再把那一條（從右邊一路加寬）搬到顯示中的第 1 頁。
func (s *Script) ribbon(step func(Site) bool) bool {
	p, a := s.Pages, s.Art
	p.CopyRect(1, 0, 0, ribbonY, 639, ribbonY+79, 0, 160)
	reveal := 1
	for base := ribbonFirst; base < ribbonLast; base++ {
		next := a.Faces[(base+8)%len(a.Faces)]
		off := 0
		for left := 80; left > 0; left -= 8 {
			p.CopyRect(0, 0, 0, 160, 639, 239, 0, ribbonY)
			for k := 0; k < 8; k++ {
				p.Put(a.Faces[(base+k)%len(a.Faces)], 80*k, 0, Copy)
			}
			p.Put(next, 0, 80, Copy)
			if left > 16 {
				p.CopyRect(0, 0, off, 0, 63, 79, 0, ribbonY)
			}
			for sx2, dx := 143, left; sx2 < 703; sx2, dx = sx2+80, dx+80 {
				p.CopyRect(0, 0, sx2-63, 0, sx2, 79, dx, ribbonY)
			}
			if left != 80 {
				p.CopyRect(0, 0, 0, 80, off-1, 159, left+560, ribbonY)
			}
			p.CopyRect(0, 1, (80-reveal)*8, ribbonY, 639, ribbonY+79, (80-reveal)*8, ribbonY)
			if !step(SiteRibbon) {
				return false
			}
			if reveal < 80 {
				reveal++
			}
			off += 8
		}
	}
	return true
}

// scroll 是 `0ad0:10aa`：三英圖從右邊捲進來。兩頁交替——先切顯示頁，
// 再把另一頁左移 8 像素、右緣補一條直條。捲完停住。
func (s *Script) scroll(beat func(Kind, int, Site) bool, step func(Site) bool, strips [80]*assets.Image) bool {
	p := s.Pages
	show := 1
	for k := range strips {
		p.Draw = show ^ 1
		p.Show = show
		if show == 1 {
			p.CopyRect(1, 0, 8, 0, 639, assets.ScreenH-1, 0, 0)
		} else {
			p.CopyRect(0, 1, 8, 0, 639, assets.ScreenH-1, 0, 0)
		}
		p.Put(strips[k], 632, 0, Copy)
		if !step(SiteScroll) {
			return false
		}
		show ^= 1
	}
	if show == 0 {
		p.Show = 0
		p.CopyPage(0, 1)
	} else {
		p.Show = 1
		p.CopyPage(1, 0)
	}
	for i := 0; i < 4; i++ {
		if !beat(HoldSeconds, 2, SiteTitleHold) {
			return false
		}
	}
	return beat(WaitKey, 0, SiteTitleKey)
}
