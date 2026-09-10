package assets

import "fmt"

// 主畫面右側兩塊面板的外框（`docs/spec/005` §6.2）。
//
// 外框不是畫出來的，是**拼出來的**：`DATA1` 裡有一組 32 塊拼件
// （`SIDEA8`–`SIDEP8` 是 8×8、`SIDEA16`–`SIDEP16` 是 16×16），
// 四個角用 16×16、四條邊用 8×8 平鋪。
//
// **這一組拼件掃不到就會想成「框是程式畫的」**——量到的第一版就是那樣，
// 而拿一個 8×8 的圖樣去全部原版檔案裡搜位元平面，一次就落在
// `DATA1` 的 `SIDEB8.IMG` 上。

// SideFrame 是一組外框拼件。
type SideFrame struct {
	// Corner 是 16×16 的角，Edge 是 8×8 的邊。
	Corner, Edge *Image
}

// LoadSideFrame 取 `SIDE<字母>16` 與 `SIDE<字母>8`。
func LoadSideFrame(data1 *Container, letter byte) (SideFrame, error) {
	var f SideFrame
	for _, p := range []struct {
		name string
		dst  **Image
	}{
		{fmt.Sprintf("SIDE%c16.IMG", letter), &f.Corner},
		{fmt.Sprintf("SIDE%c8.IMG", letter), &f.Edge},
	} {
		i, ok := data1.ByName(p.name)
		if !ok {
			return f, fmt.Errorf("assets: DATA1 裡沒有 %s", p.name)
		}
		im, err := DecodeImage(data1.Data(i))
		if err != nil {
			return f, fmt.Errorf("assets: 解 %s：%w", p.name, err)
		}
		*p.dst = im
	}
	return f, nil
}

// MainPanel 是一塊面板：外框位置、用哪一組拼件、內部底色。
type MainPanel struct {
	X, Y, W, H int
	Letter     byte
	Fill       byte
}

// MainPanels 是主畫面右側的兩塊面板，位置量自原版畫面。
//
// 上面板放郡的資料與主事者肖像，下面板放指令提示。
//
// 三個高度接得剛剛好：上面板 36–291（256）、下面板 292–371（80）、
// 下方花邊 `MAINMAP2` 372–407（36），合起來就是畫面的 408 列。
//
// **下面板是 80 不是 256。** 256 是在 640×350 的畫布上量出來的——
// 那時下緣落在畫面外，量不到，於是照上面板填了同一個數字。畫面改回
// 408 之後量得到了：外框的角在 292–307 與 **356–371** 各出現一次
//（`TestZMeasureLowerPanel` 的方法：拿 `SIDED16` 的每一列去比），
// 高 ＝ 16 ＋ 8×6 ＋ 16 ＝ 80，而 292 ＋ 80 ＝ 372 正好接上花邊。
func MainPanels() [2]MainPanel {
	return [2]MainPanel{
		{X: MainPanelX, Y: 36, W: MainPanelW, H: 256, Letter: 'B', Fill: 3},
		{X: MainPanelX, Y: 292, W: MainPanelW, H: 80, Letter: 'D', Fill: 2},
	}
}

const (
	// MainPanelX／MainPanelW 是兩塊面板共用的左緣與寬度。
	MainPanelX = 408
	MainPanelW = 224
)

// Inner 是面板扣掉外框之後的內部範圍（含頭尾）。
func (p MainPanel) Inner() (x0, y0, x1, y1 int) {
	return p.X + 8, p.Y + 8, p.X + p.W - 9, p.Y + p.H - 9
}

// DrawPanel 把一塊面板畫上去：先填底色，再拼外框。
//
// 順序不能反——外框的拼件裡有透空的地方（底色會從那裡露出來），
// 先拼框再填底色會把框蓋掉一半。
func (im *Image) DrawPanel(p MainPanel, f SideFrame) {
	x0, y0, x1, y1 := p.Inner()
	im.FillRect(x0, y0, x1-x0+1, y1-y0+1, p.Fill)

	const c, e = 16, 8
	// 四個角。
	for _, pt := range [][2]int{
		{p.X, p.Y}, {p.X + p.W - c, p.Y},
		{p.X, p.Y + p.H - c}, {p.X + p.W - c, p.Y + p.H - c},
	} {
		im.Blit(f.Corner, pt[0], pt[1])
	}
	// 上下兩條邊：角與角之間平鋪。
	for x := p.X + c; x+e <= p.X+p.W-c; x += e {
		im.Blit(f.Edge, x, p.Y)
		im.Blit(f.Edge, x, p.Y+p.H-e)
	}
	// 左右兩條邊。
	for y := p.Y + c; y+e <= p.Y+p.H-c; y += e {
		im.Blit(f.Edge, p.X, y)
		im.Blit(f.Edge, p.X+p.W-e, y)
	}
}
