package game

import (
	"encoding/binary"
	"fmt"

	"github.com/wicanr2/softworld_san1_remake/internal/state"
)

// 把進行中的局面寫回原版的三張表。
//
// 存檔就是這三張表 ＋ 一份 remake 自己的補充（`internal/save`）。
// 版面與偏移量與 `state.DecodeTables` 讀的是同一份（`docs/spec/003`）：
// **解碼與編碼要對著同一張版面表寫**，各寫各的就會慢慢分家。
//
// ⚠ **只蓋已知欄位。** 州郡表 176 個位元組解出 24 個、人物表 30 個解出
// 20 個、諸侯表 72 個解出 2 個。其餘原封不動帶著走——未解的東西不該
// 因為存了一次檔就消失，那會讓將來解出它的人拿到一份被我們洗過的資料。

// 三張表的欄位偏移。與 `internal/state` 的解碼一致。
const (
	staName       = 0  // 郡名，4 byte Big5
	staPopulation = 14 // uint16，實際值 ÷ 100
	staSoldiers   = 16 // uint16，實際值 ÷ 100
	staGold       = 18 // uint16
	staRice       = 20 // uint16
	staActive     = 22 // 現役武將數
	staFree       = 23 // 在野武將數
	staLoyalty    = 26
	staLandValue  = 27
	staFloodRate  = 28
	staPrice      = 29
	staAutonomy   = 12
	staForts      = 25
	staOwner      = 30
	// 戰場地圖：offset 55–174，12 欄 × 10 列（`docs/spec/003` §3.3）。
	staField, staFieldLen = 55, 120
	staGovernor   = 32

	genAge      = 7
	genStamina  = 8
	genIntel    = 9
	genWar      = 10
	genCharm    = 11
	genRank     = 12
	genOrigin   = 13
	genLoyalty  = 16
	genStatus   = 17
	genFaction  = 18
	genLocation = 19
	genTroop    = 21
	genSoldiers = 22 // uint16
	genTraining = 24
	genArms     = 25
)

// masLord 是諸侯表裡君主的人物槽號（uint16）。
//
// **它會變。** 君主老死時由麾下接位（`events.go`），不寫回去的話
// 讀檔會拿回開局那位——而且那個人已經不在了。
const masLord = 2

// masChief 是諸侯表裡軍師的人物槽號（uint16，`0xFFFF` ＝ 沒有）。
// 「指定軍師」讀它當換人的門檻（`docs/mechanics/70-ai` §2.7）。
const masChief = 6

// masTreasury 是寶庫的第一格（玉璽），後面依序是兵書、寶刀、美女、駿馬
// （`state.TreasuryOf`，`L0`）。進貢寫的是 `es:0xf` 起算的四格
// （`0x172f8`），也就是 offset 15–18。
const masTreasury = 14

const (
	staRecord = 176
	genRecord = 30
	masRecord = 72
)

// Tables 把目前的局面寫回三張表，回傳可以直接存成檔案的位元組。
//
// 郡表的第 0 筆是原版的啞元，原封不動帶過去。
func (g *State) Tables() (mas, sta, gen []byte, err error) {
	if len(g.rawSta) != state.PrefectureTableSize ||
		len(g.rawGen) != state.GeneralTableSize ||
		len(g.rawMas) != state.MasterTableSize {
		return nil, nil, nil, fmt.Errorf(
			"game: 這一局沒有原版的表可以寫回去（是不是繞過 New 造出來的？）")
	}
	mas = append([]byte(nil), g.rawMas...)
	sta = append([]byte(nil), g.rawSta...)
	gen = append([]byte(nil), g.rawGen...)

	for _, f := range g.factions {
		if int(f.ID)*masRecord+masLord+2 > len(mas) {
			continue
		}
		// **絕嗣的勢力君主欄要寫哨兵**（原版 `0xFFFF`）：`f.Lord` 是 −1
		// 時直接 put 會寫成 `0xFFFF` 的補數，讀回來就變成一個真的槽號。
		lord := state.NoValue16
		if f.Lord >= 0 {
			lord = f.Lord
		}
		put16(mas[int(f.ID)*masRecord+masLord:], lord)
		// 軍師（offset 6）。**不寫的話換軍師存不下來**——`f.Chief` 改了，
		// 存檔還是舊的那一位，讀回來門檻又變回他的智。
		chief := state.NoValue16
		if f.Chief >= 0 {
			chief = f.Chief
		}
		put16(mas[int(f.ID)*masRecord+masChief:], chief)
		// 寶庫（offset 14–18，玉璽在最前面）。**不寫的話進貢存不下來**
		// ——每年冬天發下去的東西讀回來就沒了，而賞賜物品那四支是拿
		// 存量當閘門的（`0xd9b4`：庫存 <= r 就不送）。
		for t := TreasureSeal; t < treasureCount; t++ {
			at := int(f.ID)*masRecord + masTreasury + int(t)
			if at >= len(mas) {
				break
			}
			mas[at] = byte(clampTo(f.Treasury[t], 255))
		}
	}

	for i := range g.prefectures {
		p := &g.prefectures[i]
		rec := sta[(p.ID)*staRecord:]
		// 人口與兵士存的是實際值 ÷ 100（原版的格式字串把 `00` 直接接在
		// 數字後面）。**除回去要無條件捨去**，四捨五入會讓存讀一輪之後
		// 人口自己長大。
		put16(rec[staPopulation:], p.Population/100)
		// **兵士寫的是存值，不是當下重算**（`State.troops`，`0x1949e`）。
		put16(rec[staSoldiers:], g.Troops(p.ID))
		put16(rec[staGold:], p.Gold)
		put16(rec[staRice:], p.Rice)
		rec[staActive] = clampByte(g.ActiveGenerals(p.ID))
		rec[staFree] = clampByte(g.FreeGenerals(p.ID))
		put16(rec[staAutonomy:], int(p.Autonomy))
		rec[staForts] = clampByte(p.Forts)
		rec[staLoyalty] = p.PublicLoyalty
		rec[staLandValue] = p.LandValue
		rec[staFloodRate] = p.FloodRate
		rec[staPrice] = p.PriceLevel
		rec[staOwner] = byte(p.Owner)
		// **戰場地圖要寫回去。** 漏掉這一段，建築關寨改的那一格
		//（低四位變 6）在 `Tables()` 就消失了——郡表的金與關寨數對得上，
		// 只有地圖那一格還是舊值，而存讀一輪之後關寨也跟著不見。
		if len(p.BattleField) == staFieldLen {
			copy(rec[staField:staField+staFieldLen], p.BattleField)
		}
		// 主事者（offset 32）。**先問一次 Governor** 讓它把失聯的那一位
		// 重新指派好，否則存檔帶著一個已經不在的人，讀回來又要重推——
		// 而重推在君主與太守同郡時給不出唯一解。
		var slot uint16 = state.NoValue16
		if x := g.Governor(p.ID); x != nil {
			slot = uint16(x.Index)
		}
		binary.LittleEndian.PutUint16(rec[staGovernor:], slot)
	}

	for i := range g.generals {
		x := &g.generals[i]
		rec := gen[i*genRecord:]
		rec[genAge] = x.Age
		rec[genStamina] = x.Stamina
		rec[genIntel] = x.Intel
		rec[genWar] = x.War
		rec[genCharm] = x.Charm
		rec[genRank] = byte(x.Rank)
		rec[genOrigin] = clampByte(x.Origin)
		rec[genLoyalty] = x.Loyalty
		rec[genStatus] = byte(x.Status)
		rec[genFaction] = byte(x.Faction)
		rec[genLocation] = clampByte(x.Location)
		rec[genTroop] = byte(x.Troop)
		put16(rec[genSoldiers:], x.Soldiers)
		rec[genTraining] = x.Training
		rec[genArms] = x.Arms
	}
	return mas, sta, gen, nil
}

// put16 寫一個 little-endian uint16，並且**夾住上限**。
//
// 原版的欄位是 16 位元；溢位在 Go 裡是靜靜地捲回去，
// 一個三萬人的郡會變成三百人而且沒有任何錯誤訊息。
func put16(b []byte, v int) {
	if v < 0 {
		v = 0
	}
	if v > 0xFFFF {
		v = 0xFFFF
	}
	binary.LittleEndian.PutUint16(b, uint16(v))
}

func clampByte(v int) byte {
	if v < 0 {
		return 0
	}
	if v > 0xFF {
		return 0xFF
	}
	return byte(v)
}
