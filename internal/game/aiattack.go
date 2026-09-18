package game

import (
	"fmt"
	"math/big"

	"github.com/wicanr2/softworld_san1_remake/internal/state"
)

// 電腦諸侯發動的戰役：出兵那一張表的進攻分支（原版 `0xb5c2`、加強版
// `0xb4e0`）直接進戰役入口（`0x20200`／`0x1e252`），**出征的名單在入口的
// 整編才決定**（`0x23630`／`0x21138`，`L0`、`[both]`）。出兵那一張只挑目標、
// 比兵力門檻——第二次編隊（`0xb706`）只在移防那兩條路上。
//
// 整編對主攻軍（陣營碼 2）做的事，兩版同形：
//
//	call 行動者(郡)                       ; 重建清單、重排、重算留守目標
//	                                     ; 加強版在這裡再抽一次策略值（0xe7dd）
//	keep ＝ 留守目標 × 100                                      ; 0x236a9
//	洗牌：for i in 0..n−1: swap(清單[i], 清單[RND(n)])          ; 0x238a1
//	從尾端往前：累計兵力 < keep 就把那一位留在家裡              ; 0x238f8
//	    位置 0 永遠出征；加強版多一條：一個都沒留而 keep <= 0 時
//	    再把最後一位留下（0x213f3）
//	帶走的金 ＝ 郡的金 ÷ 郡的兵士(百) × 出征兵力 × 0.01（米同）  ; 0x239bb
//	    郡的兵士(百) <= 0 時整郡的金米全帶走                     ; 0x23a02
//	名單按「戰力 ＋ 謀略 ÷ 2，君主再加 1000」交換排序（0xf262）    ; 0x23738
//	    加強版的除數是 5（0xedb4）
//	五支部隊依序填：每支 (n−1)÷5 位，前 n − 5×那個數 支多一位   ; 0x23789
//	出征的每一位所在郡 ← 0，然後郡重整一次（0x1d638）             ; 0x23296
//
// 主守軍（陣營碼 0）是整個郡的守將（模式 2 的清單，不比對勢力），原版排過
// 行動者的序（`0x23656`），加強版沒排（`0x21158`）；兩版接著都走同一道
// 整編排序。加強版的戰役入口在整編之前先讓兩邊的主事者各說一句話
// （`0x1e317`／`0x1e34f`），對白框的字色是 `RND(8)`（`0x2f912`）——
// 兩擲不影響規則，少了它整個月的骰序從這裡岔開。

// KeepFunc 給出整編那一刻的留守目標（百）——行動者那一張表的邏輯，由
// AI 那一層提供（`ai.faithful.keepFunc`）。
type KeepFunc func(g *State, prefecture int) int

// WarSpeechColours 是加強版戰役入口兩段對白各擲一次的範圍（`0x2f908`：
// `RND(8)`，字色）。
const WarSpeechColours = 8

// ComputerAttack 是電腦諸侯的進攻：整編照原版的規矩自己決定誰出征。
func (g *State) ComputerAttack(from, to int, by state.FactionID, keep KeepFunc) (*BattleResult, error) {
	src, err := g.canOrder(from, by)
	if err != nil {
		return nil, err
	}
	dst := g.Prefecture(to)
	if dst == nil {
		return nil, fmt.Errorf("game: 郡編號 %d 越界", to)
	}
	if !g.Adjacent(from, to) {
		return nil, ErrNotAdjacent
	}
	if keep == nil {
		return nil, fmt.Errorf("game: 電腦出兵缺留守目標")
	}
	if g.Edition == state.EditionPlus {
		g.Roll(WarSpeechColours, from, to, 0x2f912)
		g.Roll(WarSpeechColours, from, to, 0x2f912)
	}
	// 整編一開始的行動者呼叫重建清單，收尾把 Σ兵力 ÷ 100 寫回州郡的兵士
	// （`0xf072`／加強版 `0xebd5`）。留守目標與帶走的金米讀的都是這一份，
	// 而它與出兵那張表看到的不同——調整兵力重建清單時寫的是攤平**之前**
	// 的總數，攤平用的是入口那一份陳舊的 Σ上限，總數會變（加強版月度
	// 對拍量到郡 15：175 → 195，帶走的金米按 195 算）。
	g.RefreshTroops(from)
	want := keep(g, from)
	att := g.formAttackers(from, want)
	if len(att) == 0 {
		return nil, fmt.Errorf("game: %s 沒有人出征", src.Name)
	}
	g.sortForFormation(att)
	going := 0
	for _, x := range att {
		going += x.Soldiers
	}
	sup := Supply{
		Gold: BattleShare(src.Gold, g.Troops(from), going),
		Rice: BattleShare(src.Rice, g.Troops(from), going),
	}
	// 主守軍：郡裡的每一位（不比對勢力）。玩家守的話原版是逐位問，
	// 這裡不替玩家排。
	def := g.Garrison(to)
	if g.Edition != state.EditionPlus {
		def = g.ActorRoster(to)
	}
	if !g.IsHuman(dst.Owner) {
		g.sortForFormation(def)
	}
	g.endTurn(src)
	p := g.prepare(from, to, att, def, by, sup, Aid{})
	// 出征的人離開之後郡重整一次（`0x1d638`）：主事者走了就換人。
	g.refreshGovernor(from)
	if g.noPlayerIn(from, to) {
		p.autoAI = true
		// 主守軍的整編也把每一位的所在郡清成 0（`0x23296` 對四個軍團
		// 都做），然後 `0x1d638` 把空掉的戰場郡設成無主、主事者沒有——
		// 收尾的重整（`placeAfterAIBattle`）從這個狀態重建。玩家在場的
		// 戰役走另一條收尾，不清。
		for _, x := range def {
			x.Location = 0
		}
		g.refreshGovernor(to)
		day := 0
		p.B.AutoResolveAIWithRoll(func(n int) int {
			day++
			return g.Roll(n, from, to, day, 0x1f5d8)
		})
		// `0x1ecfc` 在大地圖上播這一仗、日迴圈在它裡面跑；分贓（`0x1f6fe`）
		// 在它之後，所以動畫排在分贓的事件前面。加強版的同一段還沒讀，不排。
		if g.Edition != state.EditionPlus {
			g.pending = append(g.pending, Event{Prefecture: to,
				Bubble: &Bubble{MapBattle: &MapBattle{Attacker: from, Defender: to, Days: p.B.Day}}})
		}
		g.ravageBattlefield(to)
	} else if g.Options.PlayerDefends && g.IsHuman(dst.Owner) {
		// **玩家親自守城**（Issue #64）：原版被打的時候一律由玩家指揮
		// 守方（`docs/re/05` §12.4）。規則層不能停著等鍵，所以把整編好的
		// 戰役交出去、回合停在這裡；`cmd/san1` 打完再叫 `FinishDefence`。
		// 攻方那一郡的收尾（`endTurn`、`refreshGovernor`）已經做過了。
		p.Player = true
		g.defence = p
		return nil, nil
	} else {
		p.B.Auto()
	}
	return g.settle(p), nil
}

// PendingDefence 是還沒打的那一場「電腦來攻、玩家自己守」（Issue #64）；
// 沒有就是 nil。月流程看到它就停下來，等 `FinishDefence`。
func (g *State) PendingDefence() *Pending { return g.defence }

// FinishDefence 把玩家守完的那一場搬回局面，回戰報。
func (g *State) FinishDefence() *BattleResult {
	p := g.defence
	if p == nil {
		return nil
	}
	g.defence = nil
	return g.settle(p)
}

// formAttackers 是整編挑出征名單那一段（`0x23888`／加強版 `0x2133e`）。
// 回傳的順序就是部隊要照的順序。
func (g *State) formAttackers(prefecture, want int) []*General {
	pool := g.ActorRoster(prefecture)
	n := len(pool)
	if n == 0 {
		return nil
	}
	for i := range pool {
		j := g.Roll(n, prefecture, i, 0x238a1)
		pool[i], pool[j] = pool[j], pool[i]
	}
	keep := want * 100
	acc, i := 0, n-1
	for i > 0 && acc < keep {
		acc += pool[i].Soldiers
		i--
	}
	if g.Edition == state.EditionPlus && acc == 0 && i == n-1 {
		i--
	}
	return pool[:i+1]
}

// BattleShare 是整編帶走的錢糧（`0x239bb`–`0x239fa`，`L0`）：
//
//	郡的金 ÷ 郡的兵士(百) × 出征兵力 × 0.01
//
// 三步都在 x87 的暫存器裡做完才截斷；出征兵力是**人數不是百**，所以
// 帶著小數。郡存的兵士(百) <= 0 時整郡的金米全帶走（`0x23a02`）。
func BattleShare(amount, units, soldiers int) int {
	if units <= 0 {
		return amount
	}
	q := x87(int64(amount))
	q.Quo(q, x87(int64(units)))
	q.Mul(q, x87(int64(soldiers)))
	q.Mul(q, new(big.Float).SetPrec(64).SetMode(big.ToNearestEven).SetFloat64(0.01))
	n, _ := q.Int64()
	return int(int16(n))
}

// FormationKey 是整編排序的鍵（`0xf262`／加強版 `0xedb4`，`L0`）：
// 戰力 ＋ 謀略 ÷ 除數，君主再加 1000。除數原版 2、加強版 5。
func (g *State) FormationKey(x *General) int {
	div := 2
	if g.Edition == state.EditionPlus {
		div = 5
	}
	k := int(x.War) + int(x.Intel)/div
	if x.Status == state.StatusLord {
		k += 1000
	}
	return k
}

// sortForFormation 是整編那一道**交換排序**（由大到小；內層一比到更大的
// 就當場對調，與 ActorRoster 同一種寫法）。部隊照排完的順序填。
func (g *State) sortForFormation(list []*General) {
	for i := range list {
		for j := i + 1; j < len(list); j++ {
			if g.FormationKey(list[j]) > g.FormationKey(list[i]) {
				list[i], list[j] = list[j], list[i]
			}
		}
	}
}
