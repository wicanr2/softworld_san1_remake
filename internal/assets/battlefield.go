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
// 與原版畫面比，只有四格逐像素全中——**而那正是應該的**。
// 基準畫面停在**紮寨**那一步（提示是「0:紮寨」），原版那時把可以下寨的
// 格子照常畫、其餘的蓋上一層 50% 網點：逐像素攤開來看，不相符的格子
// 是「偶數列偶數欄留著、其餘變黑」的規則圖案，而**留下來的像素與圖塊
// 完全相同**。全中的四格是可以下寨的那一小片。
//
// 所以圖塊的解碼、編號與座標三件事都成立；要驗整張圖得換一張
// **紮完寨之後**的基準畫面。

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
