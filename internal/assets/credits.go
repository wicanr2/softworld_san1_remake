package assets

import "fmt"

// 製作群畫面的素材（`docs/spec/012`）。
//
// `DATA2` 裡有 31 個項目屬於這一組（`docs/formats/01` §4.1）：
//
//	UPR00–UPR21   二十二條 24 高的字幕，寬度不一（64–584）
//	REC10L/R      山城風景，兩塊 320×336 並排成 640×336
//	REC11L/R      另一張山景，同尺寸
//	ENDO0–ENDO3   四塊 160×336 並排成 640×336 的朝堂圖
//	ENDO4.MSK     640×151 的單平面遮罩
//
// 字幕解出來是製作群名單（企劃／程式／語音 陳則孝、圖形／人物造型
// 張崑耀、背景音樂製作 林宏佳…），所以這一組是**製作群**不是「結局畫面」
// ——`docs/formats/04` 原本那一句已經更正（`CONTEXT.md` R57）。
//
// 遮罩對得上 `REC10`：位移 y=54 時**逐點 99.39% 吻合**（把遮罩的 1
// 當天空、0 當山，與圖裡的天空色比）。它是天空與山的分界。
//
// ⚠ **本套件不含任何原版資料**；讀的是玩家自己那一份。

// 這一組素材的尺寸（`L0`，從表頭讀出來的）。
const (
	CreditBackdropW = 640
	CreditBackdropH = 336
	CreditLineH     = 24
	CreditLines     = 22

	// CreditMaskY 是遮罩對齊到背景的位移（`L1`，逐點掃出來的最佳值）。
	CreditMaskY = 54
)

// Credits 是製作群畫面要的東西。
type Credits struct {
	// Backdrop 是兩張山景（`REC10`／`REC11`），各 640×336。
	Backdrop [2]*Image
	// Mask 是 `REC10` 的天空遮罩，640×151，1 ＝ 天空、0 ＝ 山。
	Mask *Image
	// Hall 是朝堂圖，640×336。
	Hall *Image
	// Lines 是二十二條字幕，由上往下就是名單的順序。
	Lines []*Image
}

// LoadCredits 從 `DATA2` 讀這一組素材。
//
// **缺一個就回錯誤**：這是一整組，少一條字幕不會讓畫面壞掉到看得出來，
// 只會讓名單少一行——而那正是最不該安靜發生的事（這是保存專案）。
func LoadCredits(c *Container) (*Credits, error) {
	if c == nil {
		return nil, fmt.Errorf("assets: 沒有 DATA2，讀不到製作群")
	}
	get := func(name string) ([]byte, error) {
		i, ok := c.ByName(name)
		if !ok {
			return nil, fmt.Errorf("assets: DATA2 裡沒有 %s", name)
		}
		return c.Data(i), nil
	}
	pair := func(l, r string) (*Image, error) {
		lb, err := get(l)
		if err != nil {
			return nil, err
		}
		rb, err := get(r)
		if err != nil {
			return nil, err
		}
		li, err := DecodeImage(lb)
		if err != nil {
			return nil, fmt.Errorf("%s：%w", l, err)
		}
		ri, err := DecodeImage(rb)
		if err != nil {
			return nil, fmt.Errorf("%s：%w", r, err)
		}
		if li.H != ri.H {
			return nil, fmt.Errorf("assets: %s 與 %s 高度不同（%d／%d）",
				l, r, li.H, ri.H)
		}
		out := &Image{W: li.W + ri.W, H: li.H,
			Pix: make([]byte, (li.W+ri.W)*li.H)}
		out.Blit(li, 0, 0)
		out.Blit(ri, li.W, 0)
		return out, nil
	}

	cr := &Credits{}
	var err error
	if cr.Backdrop[0], err = pair("REC10L.IMG", "REC10R.IMG"); err != nil {
		return nil, err
	}
	if cr.Backdrop[1], err = pair("REC11L.IMG", "REC11R.IMG"); err != nil {
		return nil, err
	}
	for _, b := range cr.Backdrop {
		if b.W != CreditBackdropW || b.H != CreditBackdropH {
			return nil, fmt.Errorf("assets: 山景是 %d×%d，想要 %d×%d",
				b.W, b.H, CreditBackdropW, CreditBackdropH)
		}
	}

	// 朝堂圖是**四塊**並排，不是兩塊。
	hall := &Image{W: CreditBackdropW, H: CreditBackdropH,
		Pix: make([]byte, CreditBackdropW*CreditBackdropH)}
	x := 0
	for i := 0; i < 4; i++ {
		name := fmt.Sprintf("ENDO%d.IMG", i)
		b, err := get(name)
		if err != nil {
			return nil, err
		}
		im, err := DecodeImage(b)
		if err != nil {
			return nil, fmt.Errorf("%s：%w", name, err)
		}
		hall.Blit(im, x, 0)
		x += im.W
	}
	if x != CreditBackdropW {
		return nil, fmt.Errorf("assets: 四塊朝堂圖並排是 %d 寬，想要 %d",
			x, CreditBackdropW)
	}
	cr.Hall = hall

	mb, err := get("ENDO4.MSK")
	if err != nil {
		return nil, err
	}
	if cr.Mask, err = DecodeMask(mb); err != nil {
		return nil, fmt.Errorf("ENDO4.MSK：%w", err)
	}

	for i := 0; i < CreditLines; i++ {
		name := fmt.Sprintf("UPR%02d.IMG", i)
		b, err := get(name)
		if err != nil {
			return nil, err
		}
		im, err := DecodeImage(b)
		if err != nil {
			return nil, fmt.Errorf("%s：%w", name, err)
		}
		if im.H != CreditLineH {
			return nil, fmt.Errorf("assets: %s 高 %d，想要 %d",
				name, im.H, CreditLineH)
		}
		cr.Lines = append(cr.Lines, im)
	}
	return cr, nil
}

// SkyAt 回「背景的這一格是不是天空」。
//
// 遮罩只涵蓋山脊那一帶（`CreditMaskY` 起的 151 列）：那之上全是天空、
// 之下全不是。**界外要明確回答**，別讓呼叫端各自猜一套。
func (cr *Credits) SkyAt(x, y int) bool {
	if cr == nil || cr.Mask == nil {
		return false
	}
	if y < CreditMaskY {
		return true
	}
	my := y - CreditMaskY
	if my >= cr.Mask.H || x < 0 || x >= cr.Mask.W {
		return false
	}
	return cr.Mask.Pix[my*cr.Mask.W+x] != 0
}
