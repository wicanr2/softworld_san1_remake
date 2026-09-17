package game

import (
	"fmt"

	"github.com/wicanr2/softworld_san1_remake/internal/state"
)

// ErrAlreadyGifted 是「%s已賞賜過了」（`DS:0x75b4`）：同一道命令裡
// 再賞同一位。
var ErrAlreadyGifted = fmt.Errorf("這一位這道命令裡已經賞賜過了")

// GiftRound 是玩家的一道「君主→4.賞賜物品」（`0x1cfd6`，`L0`、`[base]`）。
//
// 原版是兩層迴圈：「賞賜那一郡的將軍」挑郡 →「賞賜那一位」挑人 → 物品 →
// 回到「賞賜那一位」；空 Enter 回到挑郡，再空 Enter 才結束這道命令。
//
//   - 已賞名單 `es:0x31c2[人]` **每次進這道命令重設**（`0x1cfe1`），所以
//     「已賞賜過了」只擋同一道命令裡的同一位，跨命令、跨月都不擋。
//   - 可挑的郡是「主人等於目前這一郡的主人」的每一郡（`0x1d028`），
//     不看君主在不在那裡。
//   - 回傳值 `-0x4(%bp)` 進來是 `0xFFFF`，賞出一件就改成 0；主命令迴圈
//     看到 `0xFFFF` 回主選單再問（`0x17791`），否則這個郡的回合結束。
type GiftRound struct {
	// At 是下這道命令的郡。
	At    int
	by    state.FactionID
	given map[int]bool
	// Gave 為真表示這道命令至少賞出一件。
	Gave bool
}

// OpenGift 開一道賞賜物品。君主那一類的閘門（主事者是君主本人，
// `0x1c7d2`）與「這個郡這個月下過令沒」在這裡擋。
func (g *State) OpenGift(at int, by state.FactionID) (*GiftRound, error) {
	if err := g.requireLordAt(at, by); err != nil {
		return nil, err
	}
	if _, err := g.canOrder(at, by); err != nil {
		return nil, err
	}
	return &GiftRound{At: at, by: by, given: map[int]bool{}}, nil
}

// OpenReward 開一道人事→3.賞賜金帛（`0x1c1d2`，`L0`、`[base]`）。與賞賜物品共用「賞過了」
// 那一張表（`es:0x31c2`，進命令時重設），回傳值的規則也相同：賞出一位就結束這個郡的回合，
// 一位都沒賞回主選單。這一類不必君主在場。
func (g *State) OpenReward(at int, by state.FactionID) (*GiftRound, error) {
	if _, err := g.canOrder(at, by); err != nil {
		return nil, err
	}
	return &GiftRound{At: at, by: by, given: map[int]bool{}}, nil
}

// GiftPrefectureOK 是挑郡那一問的清單（`es:0x2102`）：主人與下令的郡相同。
func (g *State) GiftPrefectureOK(r *GiftRound, pref int) bool {
	p, home := g.Prefecture(pref), g.Prefecture(r.At)
	return p != nil && home != nil && p.Owned() && p.Owner == home.Owner
}

// GiftCandidates 是「賞賜那一位」的名單：`buildRoster(郡, 模式 2)`，
// 所在郡相同、身分 0–3，**不比對勢力**。
func (g *State) GiftCandidates(pref int) []*General {
	return g.Garrison(pref)
}

// Gifted 回這一位在這道命令裡賞過沒。
func (r *GiftRound) Gifted(target int) bool { return r.given[target] }

// GiftCheck 是送之前的檢查：挑人那一問之後「已賞賜過了」（`0x1d132`）、
// 挑物品那一問「沒有這一件就重問」（`0x1d21a`）。
func (g *State) GiftCheck(r *GiftRound, target int, t Treasure) error {
	if r == nil {
		return fmt.Errorf("game: 沒有開賞賜物品")
	}
	if t == TreasureSeal {
		return ErrCantGift
	}
	if t < 0 || t >= treasureCount {
		return fmt.Errorf("game: 沒有這件寶物 %d", t)
	}
	x := g.General(target)
	if x == nil || !x.Employed() || !g.GiftPrefectureOK(r, x.Location) {
		return ErrUnknownUnit
	}
	if r.given[target] {
		return ErrAlreadyGifted
	}
	if f := g.Faction(r.by); f == nil || f.Treasury[t] <= 0 {
		return ErrNoTreasure
	}
	return nil
}

// Gift 是 `0x1d118(人, 勢力)` 的一件：扣寶庫、記已賞、加能力與忠誠。
//
// 效果（`0x1d2c6`–`0x1d46b`，`L0`）：
//
//	兵書  謀略 += 2；忠誠 += RND(30) + 謀略 ÷ 2
//	寶刀  戰力 += 3；忠誠 += RND(30) + 戰力 ÷ 2
//	美女  魅力 += 5；忠誠 += RND(50) + 50
//	駿馬  魅力 += 3、戰力 += 2；忠誠 += RND(30) + 戰力 ÷ 2
//
// **沒有電腦那四支的 `RND(2)`**，而且忠誠看的是**截到 90 之前**的能力：
// 加完能力馬上擲忠誠，之後才把每一項截在 90、又不低於賞賜前的值。
// 全部是位元組運算：忠誠加完以有號位元組看，大於 100 或小於 0 都寫 100。
func (g *State) Gift(r *GiftRound, target int, t Treasure) error {
	if err := g.GiftCheck(r, target, t); err != nil {
		return err
	}
	x, f := g.General(target), g.Faction(r.by)
	r.given[target] = true
	f.Treasury[t]--
	oldIntel, oldWar, oldCharm := x.Intel, x.War, x.Charm
	var gain uint8
	switch t {
	case TreasureBook:
		x.Intel += 2
		gain = uint8(g.Roll(TreasureLoyaltySpread, target, 0x1d2eb)) + uint8(int8(x.Intel)/2)
	case TreasureBlade:
		x.War += 3
		gain = uint8(g.Roll(TreasureLoyaltySpread, target, 0x1d333)) + uint8(int8(x.War)/2)
	case TreasureBeauty:
		x.Charm += 5
		gain = uint8(g.Roll(TreasureBeautyLoyaltySpread, target, 0x1d36d)) + TreasureBeautyLoyaltyFloor
	case TreasureHorse:
		x.Charm += 3
		x.War += 2
		gain = uint8(g.Roll(TreasureLoyaltySpread, target, 0x1d333)) + uint8(int8(x.War)/2)
	}
	x.Loyalty += gain
	if v := int8(x.Loyalty); v > 100 || v < 0 {
		x.Loyalty = 100
	}
	x.Intel = giftCap(x.Intel, oldIntel)
	x.War = giftCap(x.War, oldWar)
	x.Charm = giftCap(x.Charm, oldCharm)
	r.Gave = true
	return nil
}

// giftCap 是 `0x1d3d8` 起的截斷：以有號位元組看，大於 90 寫 90；
// 比賞賜前小就寫回賞賜前的值（原本就高過 90 的人不會被拉低）。
func giftCap(v, old uint8) uint8 {
	if int8(v) > TreasureCap {
		v = TreasureCap
	}
	if int8(v) < int8(old) {
		v = old
	}
	return v
}

// CloseGift 收掉這道命令；賞出過東西就是這個郡這個月下過令了。
func (g *State) CloseGift(r *GiftRound) bool {
	if r == nil || !r.Gave {
		return false
	}
	if p := g.Prefecture(r.At); p != nil {
		g.endTurn(p)
	}
	return true
}
