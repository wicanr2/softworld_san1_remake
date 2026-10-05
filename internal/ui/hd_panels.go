package ui

import (
	"crypto/sha256"
	"encoding/binary"
	"fmt"
	"image"
	"image/draw"
	"strings"

	"github.com/wicanr2/softworld_san1_remake/internal/assets"
	xdraw "golang.org/x/image/draw"
)

type hdSideKey struct{ corner, edge [32]byte }
type hdPanelSize struct {
	name  string
	w, h  int
	paper bool
}
type hdPanel struct {
	image  *image.RGBA
	corner int
}

// hdPanelSource 的來源鍵是固定顯示配方，並非原版儲存的完整面板。
func hdPanelSource(c *assets.Container, name string) (*assets.Image, []byte, error) {
	if c == nil {
		return nil, nil, fmt.Errorf("缺少來源容器")
	}
	raw := func(n string) ([]byte, error) {
		i, ok := c.ByName(n)
		if !ok {
			return nil, fmt.Errorf("來源資源不存在: %s", n)
		}
		return c.Data(i), nil
	}
	if strings.HasPrefix(name, "PANEL.SIDE#") {
		k := strings.TrimPrefix(name, "PANEL.SIDE#")
		allowed := map[string]bool{"A1": true, "A3": true, "B3": true, "C1": true, "C5": true, "C7": true, "D1": true, "D2": true, "E1": true}
		if !allowed[k] {
			return nil, nil, fmt.Errorf("未知面板來源鍵")
		}
		letter, paper := k[0], k[1]-'0'
		f, e := assets.LoadSideFrame(c, letter)
		if e != nil {
			return nil, nil, e
		}
		if f.Corner.W != 16 || f.Corner.H != 16 || f.Edge.W != 8 || f.Edge.H != 8 {
			return nil, nil, fmt.Errorf("面板拼件尺寸不符")
		}
		w, h := 224, 256
		if k == "D2" {
			h = 80
		}
		im := &assets.Image{W: w, H: h, Pix: make([]byte, w*h)}
		im.FillRect(0, 0, w, h, paper)
		im.DrawPanel(assets.MainPanel{W: w, H: h, Fill: paper}, f)
		a, e := raw(fmt.Sprintf("SIDE%c16.IMG", letter))
		if e != nil {
			return nil, nil, e
		}
		b, e := raw(fmt.Sprintf("SIDE%c8.IMG", letter))
		if e != nil {
			return nil, nil, e
		}
		recipe := append([]byte("SAN1-PANEL-SIDE-v1\x00"), a...)
		recipe = append(recipe, b...)
		recipe = append(recipe, letter, paper)
		var size [4]byte
		binary.LittleEndian.PutUint16(size[:2], uint16(w))
		binary.LittleEndian.PutUint16(size[2:], uint16(h))
		recipe = append(recipe, size[:]...)
		return im, recipe, nil
	}
	if name == "PANEL.BEVEL#1" || name == "PANEL.BEVEL#3" {
		a, e := raw(assets.BattleBGTile)
		if e != nil {
			return nil, nil, e
		}
		tile, e := assets.DecodeImage(a)
		if e != nil {
			return nil, nil, e
		}
		if tile.W != 8 || tile.H != 8 {
			return nil, nil, fmt.Errorf("戰場底紋尺寸不符")
		}
		paper := name[len(name)-1] - '0'
		im := &assets.Image{W: 180, H: 100, Pix: make([]byte, 18000)}
		im.FillRect(2, 2, 176, 96, paper)
		im.BevelBox(2, 2, 177, 97)
		recipe := append([]byte("SAN1-PANEL-BEVEL-v1\x00"), a...)
		recipe = append(recipe, paper, 180, 100, 2)
		return im, recipe, nil
	}
	return nil, nil, fmt.Errorf("未知面板來源鍵")
}

func hdSideFingerprint(f assets.SideFrame) hdSideKey {
	return hdSideKey{hdImageKey(f.Corner), hdImageKey(f.Edge)}
}

func (p *HDPack) registerPanel(c *assets.Container, e HDEntry, high *image.RGBA) {
	corner := 2
	if strings.HasPrefix(e.Name, "PANEL.SIDE#") {
		corner = 16
		k := strings.TrimPrefix(e.Name, "PANEL.SIDE#")
		f, _ := assets.LoadSideFrame(c, k[0])
		finger := hdSideFingerprint(f)
		if p.sidePanels[finger] == nil {
			p.sidePanels[finger] = map[byte]string{}
		}
		p.sidePanels[finger][k[1]-'0'] = e.Name
		preferred := map[byte]string{'A': "A3", 'B': "B3", 'C': "C7", 'D': "D2", 'E': "E1"}
		if preferred[k[0]] == k {
			p.sideBorders[finger] = e.Name
		}
	}
	p.panels[e.Name] = hdPanel{high, corner}
}

// hdSlicePanel 只縮放邊和中央，保留四角的 16 邏輯像素。
func hdSlicePanel(src *image.RGBA, w, h, corner int) *image.RGBA {
	out := image.NewRGBA(image.Rect(0, 0, w*4, h*4))
	b := src.Bounds()
	k := corner * 4
	sx, sy := [4]int{b.Min.X, b.Min.X + k, b.Max.X - k, b.Max.X}, [4]int{b.Min.Y, b.Min.Y + k, b.Max.Y - k, b.Max.Y}
	dx, dy := [4]int{0, k, w*4 - k, w * 4}, [4]int{0, k, h*4 - k, h * 4}
	for y := 0; y < 3; y++ {
		for x := 0; x < 3; x++ {
			a, r := image.Rect(sx[x], sy[y], sx[x+1], sy[y+1]), image.Rect(dx[x], dy[y], dx[x+1], dy[y+1])
			if !r.Empty() {
				xdraw.CatmullRom.Scale(out, r, src, a, draw.Src, nil)
			}
		}
	}
	return out
}

func (c *Canvas) highPanel(name string, r image.Rectangle, paper bool) *image.RGBA {
	if c.HD == nil {
		return nil
	}
	p, ok := c.HD.panels[name]
	if !ok {
		return nil
	}
	if r.Dx() < p.corner*2 || r.Dy() < p.corner*2 || r.Dx() > assets.ScreenW || r.Dy() > assets.ScreenH {
		return nil
	}
	key := hdPanelSize{name, r.Dx(), r.Dy(), paper}
	if im := c.HD.panelCache[key]; im != nil {
		return im
	}
	var out *image.RGBA
	if paper {
		out = image.NewRGBA(image.Rect(0, 0, r.Dx()*4, r.Dy()*4))
		b := p.image.Bounds()
		xdraw.CatmullRom.Scale(out, out.Bounds(), p.image, b.Inset(p.corner*4), draw.Src, nil)
	} else {
		out = hdSlicePanel(p.image, r.Dx(), r.Dy(), p.corner)
	}
	// 正式畫面使用有限尺寸；變動視窗不改變這份邏輯尺寸快取。
	if len(c.HD.panelCache) < 64 && c.HD.panelCacheBytes+len(out.Pix) <= 64<<20 {
		c.HD.panelCache[key] = out
		c.HD.panelCacheBytes += len(out.Pix)
	}
	return out
}

func (c *Canvas) drawHighSidePanel(f assets.SideFrame, x, y, w, h int, paper ...byte) {
	if c.HD == nil || f.Corner == nil || f.Edge == nil {
		return
	}
	finger := hdSideFingerprint(f)
	name := c.HD.sideBorders[finger]
	borderOnly := len(paper) == 0
	if !borderOnly {
		name = c.HD.sidePanels[finger][paper[0]]
	}
	r := image.Rect(x, y, x+w, y+h)
	high := c.highPanel(name, r, false)
	if high == nil {
		return
	}
	c.addHigh(high, r, image.Point{})
	if borderOnly {
		// 十字以外恰是原版的四角與 8 像素邊。原有底紋保留覆蓋權。
		c.trackRect(image.Rect(x+16, y+8, x+w-16, y+h-8))
		c.trackRect(image.Rect(x+8, y+16, x+w-8, y+h-16))
	}
}

func (c *Canvas) drawHighBevel(x, y, w, h int, paper byte) {
	r := image.Rect(x-2, y-2, x+w+2, y+h+2)
	high := c.highPanel(fmt.Sprintf("PANEL.BEVEL#%d", paper), r, false)
	if high != nil {
		c.addHigh(high, r, image.Point{})
	}
}

// drawHighBattleLeftColumn 沿用原版四框的位置；天候與文字由呼叫端最後疊畫。
func (c *Canvas) drawHighBattleLeftColumn() {
	for i := range assets.BattleLeftBoxCount {
		y0, y1 := assets.BattleLeftBox(i)
		c.drawHighBevel(assets.BattleLeftBoxX0, y0,
			assets.BattleLeftBoxX1-assets.BattleLeftBoxX0+1, y1-y0+1,
			assets.BattleOrderPaper)
	}
}

func (c *Canvas) drawHighPaper(r image.Rectangle, paper byte) {
	high := c.highPanel(fmt.Sprintf("PANEL.BEVEL#%d", paper), r, true)
	if high != nil {
		c.addHigh(high, r, image.Point{})
	}
}

func panelRecipeHash(raw []byte) string { return fmt.Sprintf("%x", sha256.Sum256(raw)) }

func (c *Canvas) drawHighPortraitFrame(fr [4]*assets.Image, x, y int) {
	at := [4]image.Point{{x - 8, y - 8}, {x - 8, y + 80}, {x - 8, y}, {x + 64, y}}
	for i, im := range fr {
		if im != nil {
			c.trackRect(image.Rect(at[i].X, at[i].Y, at[i].X+im.W, at[i].Y+im.H))
			c.drawHigh(im, at[i].X, at[i].Y)
		}
	}
}
