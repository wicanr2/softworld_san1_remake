package assets

import "fmt"

// 主戰場的地形圖塊。
//
// `EICON.GRP`（27,792 B）**不是一整塊圖，是三十六筆 `.IMG` 首尾相接**：
// 每筆 4 個位元組表頭加四個 192 位元組的平面，共 772 個位元組，
// 27,792 ÷ 772 ＝ 36 整除。每一張 48×32。
//
// **圖塊編號就是地形碼**：拿原版主戰場的截圖抽一格，把四個平面的位元
// 樣式去 `EICON.GRP` 裡 grep，命中的記錄編號與那一格的地形碼相同
// （四格四中：樹林 8 對 #8、平原 7 對 #7）。原版畫的時候也是直接把
// 低四位推進去（`0x22781`，大於 10 就不畫）。
//
// 格子的位置（`0x22745`–`0x22781`）：
//
//	x = 欄 × 48 + 56
//	y = 列 × 32 + 36        奇數欄再 +16
//
// **兩種版面都塞得下**：12×7 是 x ≤ 584、y ≤ 244；8×10 是 x ≤ 392、
// y ≤ 340。把兩個軸讀反的話 8×10 會算出 y=488 超出畫面——那個矛盾
// 就是讀反了的證據。
//
// 尺寸不必猜：每一筆的表頭寫著高 32、寬 48。
//
// 與原版**紮完寨之後**的主戰場逐格比：78 格裡 **73 格逐像素全中**，
// 差的五格正是五支部隊站的位置。
//
// ⚠ 用紮寨那一步的畫面當基準的話只有 4 格全中，而原因不是座標錯——
// 原版在紮寨時把可以下寨的格子照常畫、其餘蓋一層 50% 網點
// （偶數列偶數欄留著、其餘變黑）。**判準要看形狀不要看比例**：
// 把差異畫成圖案一眼看得出是網點，只看相符率會猜成天氣。

const (
	// TileW／TileH 是一個地形圖塊的尺寸。
	TileW = 48
	TileH = 32
	// tileRecord 是 `EICON.GRP` 裡一筆的長度：表頭 4 ＋ 四個平面各 192。
	tileRecord = ImageHeader + TileW/8*TileH*4
	// TileCount 是 `EICON.GRP` 裡有幾張。
	TileCount = 36

	// FieldOriginX／Y 是第 0 欄第 0 列的左上角。
	FieldOriginX = 56
	FieldOriginY = 36
	// FieldStagger 是奇數欄往下錯開多少。
	FieldStagger = 16
	// FieldCols 是戰場地圖每一列在記錄裡佔幾格（州郡記錄 offset 55–174）。
	FieldCols = 12
	// MaxTerrain 是會畫出圖塊的最大地形碼；超過就不畫（`0x22772`）。
	MaxTerrain = 10
)

// BattleTiles 從 `DATA1` 解出三十六張地形圖塊，索引就是地形碼。
func BattleTiles(data1 *Container) ([]*Image, error) {
	i, ok := data1.ByName("EICON.GRP")
	if !ok {
		return nil, fmt.Errorf("assets: DATA1 裡沒有 EICON.GRP")
	}
	b := data1.Data(i)
	if len(b) != TileCount*tileRecord {
		return nil, fmt.Errorf("assets: EICON.GRP 長 %d，想要 %d（%d × %d）",
			len(b), TileCount*tileRecord, TileCount, tileRecord)
	}
	out := make([]*Image, TileCount)
	for k := 0; k < TileCount; k++ {
		im, err := DecodeImage(b[k*tileRecord : (k+1)*tileRecord])
		if err != nil {
			return nil, fmt.Errorf("assets: 解第 %d 張圖塊：%w", k, err)
		}
		if im.W != TileW || im.H != TileH {
			return nil, fmt.Errorf("assets: 第 %d 張圖塊是 %d×%d", k, im.W, im.H)
		}
		out[k] = im
	}
	return out, nil
}

// FieldCell 回傳第 (col, row) 格的左上角。
func FieldCell(col, row int) (x, y int) {
	x = col*TileW + FieldOriginX
	y = row*TileH + FieldOriginY
	if col%2 == 1 {
		y += FieldStagger
	}
	return
}

// BattleField 把一個郡的戰場地圖畫成 640×408。
//
// field 是州郡記錄 offset 55–174 那 120 個位元組；`0xFF` 是圖外。
// 只畫地形，標記（高四位）與部隊由上層疊。
func BattleField(tiles []*Image, field []byte, bg byte) *Image {
	im := &Image{W: ScreenW, H: ScreenH, Pix: make([]byte, ScreenW*ScreenH)}
	for i := range im.Pix {
		im.Pix[i] = bg
	}
	im.BlitField(tiles, field)
	return im
}

// BlitField 把圖塊畫到既有的圖上，格子以外的地方不動。
//
// 場地四周的邊框（`FieldEdges`）要在圖塊**之前**畫：原版就是那個順序，
// 所以邊框在錯開的欄位那裡會被圖塊蓋掉一半。反過來畫的話下緣那兩列
// 白線會壓在地形上，與原版差五百多格。
func (im *Image) BlitField(tiles []*Image, field []byte) {
	// **順序有差**：奇數欄往下錯開 16，而圖塊高 32——後畫的會蓋掉
	// 先畫的一半。原版逐欄畫（`0x22704` 的兩層迴圈外層是欄），
	// 照列畫出來的疊法不一樣，格子邊緣就會對不上。
	rows := len(field) / FieldCols
	for col := 0; col < FieldCols; col++ {
		for row := 0; row < rows; row++ {
			b := field[row*FieldCols+col]
			if b == 0xFF {
				continue
			}
			t := int(b & 0x0F)
			if t > MaxTerrain || t >= len(tiles) || tiles[t] == nil {
				continue
			}
			x, y := FieldCell(col, row)
			im.Blit(tiles[t], x, y)
		}
	}
}

// 部隊的標記是**旗幟**（`DATA1` 的 `WFLAG*`）。
//
// 四組旗幟的組名寫在 `DS:0x5da2`：`D0`、`D1`、`A0`、`A1`。載入端
// （`0x11636`）用 `WFLAG%s%c.IMG` 組出檔名，`%c` 是 `'0'+n`（n 走 0–5），
// 載進去的槽號是 `20 + 組×6 + n`；畫的那一端（`0x21c01`–`0x21c16`）
// 算的是同一個式子，而同一支常式前面用 `(組×10 + n) × 42` 去索引部隊
// 記錄——所以**第一個參數是軍力、第二個是隊伍**。
//
// **`D` 是守、`A` 是攻。** 紮完寨的基準畫面上四面綠旗（`D0`）的兵力牌
// 是 2673／1500／2227／1500，合計 7900，等於右側面板的「周瑜軍　主守軍
// 四軍 4將　兵 7900」；畫面上唯一的另一個標記牌是 3000，等於左側的
// 「陳就軍　主攻軍　一軍 1將　兵 3000」，用的是 `A0`。
//
// 組的顏色：`D0` 淺綠、`D1` 淺青、`A0` 紅／淺紅網點、`A1` 洋紅／淺洋紅
// 網點，與兵力牌的字色表（`DS:0x796a` ＝ 10、11、12、13）一一對上。
//
// 每組六張。0–4 的旗面上各印一個字——**帥、先、左、右、後**，逐像素
// 讀字模讀出來的，即中軍、先鋒、左軍、右軍、後軍；順序與紮營順序相同
// （說明書 p.28）。第六張（n ＝ 5）是 16×15 的城門圖示，不是旗。
//
// 位置：旗畫在格子左上角加 (8, 0)，兵力牌畫在再往下 15 個像素
// （`0x21c67` 的 `[bp-4] + 0xf`）。
const (
	// FlagW／FlagH 是旗幟的尺寸。
	FlagW = 24
	FlagH = 15
	// FlagOffsetX／Y 是旗幟相對於格子左上角的位移。
	FlagOffsetX = 8
	FlagOffsetY = 0
	// FlagPlateOffsetY 是兵力牌相對於旗幟左上角的 y。
	FlagPlateOffsetY = FlagH
	// FlagPlateW／FlagPlateH 是兵力牌那個黑底方塊的大小。
	// 量自基準畫面：右軍的牌佔 (352,83)–(391,99)，下一列的地形從 100 起。
	FlagPlateW = 40
	FlagPlateH = 17
	// FlagPlateCells 是牌上留幾個字格：原版的格式是 `%5d`
	// （`DS:0x7ab4`），一格 8 像素，所以四位數的兵力靠右、左邊空一格。
	FlagPlateCells = 5
	// FlagUnits 是每組有幾支部隊的旗（隊伍 0–4）。
	FlagUnits = 5
	// FlagGate 是每組第六張——城門圖示的編號。
	FlagGate = 5
	// GateW 是城門圖示的寬，與旗不同。
	GateW = 16
)

// FlagPlateColour 是兵力牌的字色，逐軍力一個（`DS:0x796a`）。
var FlagPlateColour = [4]byte{10, 11, 12, 13}

// FlagCell 回傳第 (col, row) 格上旗幟的左上角。
func FlagCell(col, row int) (x, y int) {
	x, y = FieldCell(col, row)
	return x + FlagOffsetX, y + FlagOffsetY
}

// flagSets 是四組旗幟的組名，順序就是原版部隊陣列的軍力順序：
// 主守軍、助守軍、主攻軍、助攻軍。
var flagSets = [4]string{"D0", "D1", "A0", "A1"}

// FlagName 回傳某一支部隊的旗幟項目名。
//
// army 是軍力 0–3（主守、助守、主攻、助攻），unit 是原版的隊伍編號：
// 0–4 依序是中軍、先鋒、左軍、右軍、後軍，5 是城門圖示。
//
// ⚠ 這個編號**不是** remake `battle.Formation` 的編號，兩者要換算
// （`battle.Formation.OriginalIndex`）。
func FlagName(army, unit int) string {
	if army < 0 || army >= len(flagSets) || unit < 0 || unit > FlagGate {
		return ""
	}
	return fmt.Sprintf("WFLAG%s%d.IMG", flagSets[army], unit)
}

// UnitFlag 取一面旗幟，name 像 `WFLAGD00.IMG`。城門圖示（`*5.IMG`）
// 窄一些，所以只檢查高度。
func UnitFlag(data1 *Container, name string) (*Image, error) {
	i, ok := data1.ByName(name)
	if !ok {
		return nil, fmt.Errorf("assets: DATA1 裡沒有 %s", name)
	}
	im, err := DecodeImage(data1.Data(i))
	if err != nil {
		return nil, err
	}
	if im.H != FlagH || (im.W != FlagW && im.W != GateW) {
		return nil, fmt.Errorf("assets: %s 是 %d×%d，旗幟應該是 %d×%d 或 %d×%d",
			name, im.W, im.H, FlagW, FlagH, GateW, FlagH)
	}
	return im, nil
}

// UnitFlags 載入四組共 24 張。
func UnitFlags(data1 *Container) ([4][6]*Image, error) {
	var out [4][6]*Image
	for army := range out {
		for unit := range out[army] {
			im, err := UnitFlag(data1, FlagName(army, unit))
			if err != nil {
				return out, err
			}
			out[army][unit] = im
		}
	}
	return out, nil
}

// FlagPlateText 是兵力牌上的字：原版用 `%5d` 靠右對齊（`DS:0x7ab4`）。
func FlagPlateText(soldiers int) string {
	return fmt.Sprintf("%*d", FlagPlateCells, soldiers)
}

// 遮罩：`8x8AND0`–`3`，四張 8×8。
//
// 用法是 **AND**：`畫面 &= 遮罩`，所以遮罩是 15 的地方留著、是 0 的地方
// 變黑。四張的密度不同——
//
//	AND0  偶數列的偶數欄留著        1/4
//	AND1  棋盤                      1/2
//	AND2  斜線，每四格留一格        1/8
//	AND3  偶數列整列留著            1/2
//
// **紮寨那一步用的是 `AND0`**：原版把可以下寨的格子照常畫、其餘蓋一層
// 遮罩。判準是原版紮寨那一格的畫面（`sweep-04`）——78 格裡 73 格與
// 「圖塊 AND `8x8AND0`」逐像素相同，剩下四格是照常畫的可下寨格。
const MaskCount = 4

// Masks 載入四張遮罩。
func Masks(data1 *Container) ([MaskCount]*Image, error) {
	var out [MaskCount]*Image
	for i := range out {
		name := fmt.Sprintf("8x8AND%d.IMG", i)
		j, ok := data1.ByName(name)
		if !ok {
			return out, fmt.Errorf("assets: DATA1 裡沒有 %s", name)
		}
		im, err := DecodeImage(data1.Data(j))
		if err != nil {
			return out, err
		}
		out[i] = im
	}
	return out, nil
}

// MaskCamp 是紮寨那一步用的遮罩編號。
const MaskCamp = 0

// ApplyMask 把一塊矩形套上遮罩（`畫面 &= 遮罩`）。
//
// 遮罩對齊的是**畫面座標**，與底紋同一個規矩，跨格子接得起來。
func (im *Image) ApplyMask(x, y, w, h int, mask *Image) {
	for dy := 0; dy < h; dy++ {
		yy := y + dy
		if yy < 0 || yy >= im.H {
			continue
		}
		for dx := 0; dx < w; dx++ {
			xx := x + dx
			if xx < 0 || xx >= im.W {
				continue
			}
			im.Pix[yy*im.W+xx] &= mask.Pix[(yy%mask.H)*mask.W+xx%mask.W]
		}
	}
}
