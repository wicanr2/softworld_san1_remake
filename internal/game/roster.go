package game

// 行動者那一張表的排序（`0xf170`，`L0`、`[both]`）。分派器的十八張表與
// 戰役入口的電腦整編（`0x23630`／加強版 `0x21138`）都拿這一份順序。

// ActorWeight 是「這回合誰行動」的身分加權（`DS:0x5986`，`L0`）：
//
//	身分 0 君主 2000、1 軍師 1600、2 太守 1200、3 一般武將 800
//	身分 4–7 重複 2000／1600／1200／800
//	身分 8、9（在野）與 11（未登場）0、身分 10 是 400
//
// **權重完全壓過能力值**（智 ＋ 武 最多 200，而權重差是 400 的倍數），
// 所以排序實際上是「先看身分，同身分再比智 ＋ 武」。
var ActorWeight = [12]int{2000, 1600, 1200, 800, 2000, 1600, 1200, 800, 0, 0, 400, 0}

// ActorKey 是行動者那一張表的排序鍵：`智 + 武 + 加權表[身分]`
// （`0xf1d7` 的 `add ax, [bx+0x5986]`）。
func ActorKey(x *General) int {
	w := 0
	if int(x.Status) < len(ActorWeight) {
		w = ActorWeight[x.Status]
	}
	return int(x.Intel) + int(x.War) + w
}

// ActorRoster 是分派器手上那一份名單（`es:[0x58c]`），**照原版的順序**。
//
// 建表的是 `buildRoster(郡, 模式 2)`：所在郡相同、身分 ≤ 3，**不比對
// 勢力**（`docs/re/07` §6，28 次量過集合相等）。接著按 ActorKey 由大到小
// **交換排序**（`0xf170`）：內層一比到更大的就**當場對調**，不是記下
// 最大值再換一次。位置 0 因此落在「第一個最大」上，而同鍵的其餘元素會被
// 交換打亂——換成穩定排序會有差（`docs/playtest/02` 量到 25 次裡有 3 次
// 差在相鄰一對）。
func (g *State) ActorRoster(prefecture int) []*General {
	out := append([]*General(nil), g.Garrison(prefecture)...)
	for i := range out {
		for j := i + 1; j < len(out); j++ {
			if ActorKey(out[j]) > ActorKey(out[i]) {
				out[i], out[j] = out[j], out[i]
			}
		}
	}
	return out
}
