package game

// 玩家「挑一位將軍」的共用清單（`0x18024(提示, 鍵, 郡, 模式)`，`L0`、`[base]`，
// `docs/spec/014` §4.2）：`buildRoster(郡, 模式)`（`0xfc1e`）照槽號收人、
// 先按行動者鍵交換排序（`0xf170`），鍵 1–4 再按那一項交換排序一次。

// PickMode 是 `buildRoster` 的模式（`docs/re/07` §6）。
type PickMode int

const (
	PickAny         PickMode = 0 // 身分不是 9、11、12（查看→檢視將軍）
	PickFree        PickMode = 1 // 身分 8 或 10（在野；登用）
	PickServing     PickMode = 2 // 身分 0–3
	PickWise        PickMode = 3 // 身分 0–3 且謀略 ≥ 80（建關寨）
	PickFreeOrSub   PickMode = 4 // 身分 1、2、3、8、10
	PickSubject     PickMode = 5 // 身分 1–3（賞賜金帛、挖角）
	PickWiseSub     PickMode = 6 // 身分 1–3 且謀略 ≥ 80（指定軍師）
	PickOfficerOnly PickMode = 7 // 身分 3（撤職）
)

// PickKey 是清單的第三欄與排序（`DS:0x6ac6` 的欄名）。
type PickKey int

const (
	PickByStatus   PickKey = 0 // 現任：身分名，行動者鍵排序
	PickByIntel    PickKey = 1 // 謀略（`0xf360`：謀略 ＋ 加權）
	PickByWar      PickKey = 2 // 戰力（`0xf440`：戰力 ＋ 加權）
	PickByCharm    PickKey = 3 // 魅力（`0xf520`：魅力 ＋ 加權）
	PickByLoyalty  PickKey = 4 // 忠誠（`0xf6b6`：忠誠；君主印 --）
	PickByStamina  PickKey = 5 // 體能
	PickByAge      PickKey = 6 // 年齡
	PickBySoldiers PickKey = 7 // 兵士
	PickByArms     PickKey = 8 // 武裝
	PickByBoth     PickKey = 9 // 兵士 武裝
)

// PickRoster 是 `0x18024` 那一份清單，照原版的順序。
func (g *State) PickRoster(pref int, mode PickMode, key PickKey) []*General {
	var out []*General
	for i := range g.generals {
		x := &g.generals[i]
		if x.Location != pref || !pickModeTakes(mode, x) {
			continue
		}
		out = append(out, x)
	}
	exchangeSort(out, ActorKey)
	weight := func(x *General) int {
		if int(x.Status) < len(ActorWeight) {
			return ActorWeight[x.Status]
		}
		return 0
	}
	switch key {
	case PickByIntel:
		exchangeSort(out, func(x *General) int { return int(int8(x.Intel)) + weight(x) })
	case PickByWar:
		exchangeSort(out, func(x *General) int { return int(int8(x.War)) + weight(x) })
	case PickByCharm:
		exchangeSort(out, func(x *General) int { return int(int8(x.Charm)) + weight(x) })
	case PickByLoyalty:
		exchangeSort(out, func(x *General) int { return int(int8(x.Loyalty)) })
	}
	return out
}

// pickModeTakes 是 `0xfc3a`–`0xfdb0` 七個模式與預設的身分條件。
func pickModeTakes(mode PickMode, x *General) bool {
	s := int(x.Status)
	in := func(v ...int) bool {
		for _, w := range v {
			if s == w {
				return true
			}
		}
		return false
	}
	switch mode {
	case PickFree:
		return in(8, 10)
	case PickServing:
		return in(0, 1, 2, 3)
	case PickWise:
		return in(0, 1, 2, 3) && int(int8(x.Intel)) >= 0x50
	case PickFreeOrSub:
		return in(1, 2, 3, 8, 10)
	case PickSubject:
		return in(1, 2, 3)
	case PickWiseSub:
		return in(1, 2, 3) && int(int8(x.Intel)) >= 0x50
	case PickOfficerOnly:
		return in(3)
	}
	return !in(9, 11, 12)
}

// exchangeSort 是原版那幾支的交換排序：內層一比到更大的就當場對調。
func exchangeSort(out []*General, key func(*General) int) {
	for i := range out {
		for j := i + 1; j < len(out); j++ {
			if key(out[j]) > key(out[i]) {
				out[i], out[j] = out[j], out[i]
			}
		}
	}
}
