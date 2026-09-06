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
	staOwner      = 30

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
		put16(mas[int(f.ID)*masRecord+masLord:], f.Lord)
	}

	for i := range g.prefectures {
		p := &g.prefectures[i]
		rec := sta[(p.ID)*staRecord:]
		// 人口與兵士存的是實際值 ÷ 100（原版的格式字串把 `00` 直接接在
		// 數字後面）。**除回去要無條件捨去**，四捨五入會讓存讀一輪之後
		// 人口自己長大。
		put16(rec[staPopulation:], p.Population/100)
		put16(rec[staSoldiers:], g.Soldiers(p.ID)/100)
		put16(rec[staGold:], p.Gold)
		put16(rec[staRice:], p.Rice)
		rec[staActive] = clampByte(g.ActiveGenerals(p.ID))
		rec[staFree] = clampByte(g.FreeGenerals(p.ID))
		rec[staLoyalty] = p.PublicLoyalty
		rec[staLandValue] = p.LandValue
		rec[staFloodRate] = p.FloodRate
		rec[staPrice] = p.PriceLevel
		rec[staOwner] = byte(p.Owner)
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
