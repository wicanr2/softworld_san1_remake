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

// BattleField 把一個郡的戰場地圖畫成 640×350。
//
// field 是州郡記錄 offset 55–174 那 120 個位元組；`0xFF` 是圖外。
// 只畫地形，標記（高四位）與部隊由上層疊。
func BattleField(tiles []*Image, field []byte, bg byte) *Image {
	im := &Image{W: ScreenW, H: ScreenH, Pix: make([]byte, ScreenW*ScreenH)}
	for i := range im.Pix {
		im.Pix[i] = bg
	}
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
	return im
}

// 部隊的標記是**旗幟**（`DATA1` 的 `WFLAG*`，24×15）。
//
// 判準是拿原版紮完寨之後的主戰場，把 `DATA1` 裡的圖逐張去畫面上找：
// `WFLAGD00`–`WFLAGD03` 四張各有一處 **100% 相符**，位置分別是
// (304,116)、(304,148)、(352,132)、(352,68)——換算回格子是
// (5,2)、(5,3)、(6,3)、(6,1)，每一張都在**格子的左上角加 (8, 0)**。
//
// 那四張是主攻軍的四支部隊。畫面上的數字牌對回位置是
// D00 ＝ 中軍、D01 ＝ 先鋒、D02 ＝ 左軍、D03 ＝ 右軍——**那正好是紮營
// 的順序**（說明書 p.28：中軍 → 先鋒 → 左軍 → 右軍 → 後軍），
// 而基準畫面就是照那個順序一個一個紮下去的。所以「旗的編號」是照
// 隊伍還是照紮的先後，這一張分不出來，不要當成已知。
//
// **守軍的標記不是 `WFLAG`**：把 `DATA1` 的 190 張圖整張畫面掃過，
// 只有 D00–D03 四處 100%；二十四面旗放到每一格上試也只有那四處。
// 守軍那個洋紅色的牌子要往 `DATA3` 找。
const (
	// FlagW／FlagH 是旗幟的尺寸。
	FlagW = 24
	FlagH = 15
	// FlagOffsetX／Y 是旗幟相對於格子左上角的位移。
	FlagOffsetX = 8
	FlagOffsetY = 0
)

// FlagCell 回傳第 (col, row) 格上旗幟的左上角。
func FlagCell(col, row int) (x, y int) {
	x, y = FieldCell(col, row)
	return x + FlagOffsetX, y + FlagOffsetY
}

// UnitFlag 取一面旗幟，name 像 `WFLAGD00.IMG`。
func UnitFlag(data1 *Container, name string) (*Image, error) {
	i, ok := data1.ByName(name)
	if !ok {
		return nil, fmt.Errorf("assets: DATA1 裡沒有 %s", name)
	}
	im, err := DecodeImage(data1.Data(i))
	if err != nil {
		return nil, err
	}
	if im.W != FlagW || im.H != FlagH {
		return nil, fmt.Errorf("assets: %s 是 %d×%d，旗幟應該是 %d×%d",
			name, im.W, im.H, FlagW, FlagH)
	}
	return im, nil
}
