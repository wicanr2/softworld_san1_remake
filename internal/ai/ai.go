// Package ai 是電腦諸侯的決策。
//
// **三個版本並列**，由旗標選：
//
//	base      三國演義（原版）的 AI —— 以還原原版行為為目標
//	plus      三國演義1加強版的 AI —— 同上，加強版另有一份
//	enhanced  remake 強化 AI —— remake 自己的，不宣稱與任何原版相同
//
// ⚠ **前兩個是還原，第三個是創作。** 兩者的驗收標準完全不同：
// `base`／`plus` 要與原版對拍（同一個局面下同一串命令），
// `enhanced` 只要「玩起來好」。把它們混在一起——例如在 `base` 裡
// 「順手改好一點」——會讓還原失去意義，而且**沒有人看得出來**，
// 因為兩者的輸出都是合法的命令。
//
// 還原用的 AI 目前**還沒解**（原版的決策程式碼還沒反組譯到）。
// 它們不會假裝自己會下棋：`Derived()` 回 false，`Plan` 回空，
// 呼叫端必須把這件事顯示出來。
package ai

import (
	"fmt"

	"github.com/wicanr2/softworld_san1_remake/internal/game"
	"github.com/wicanr2/softworld_san1_remake/internal/state"
)

// Mode 是 AI 的版本。
type Mode string

const (
	ModeBase     Mode = "base"
	ModePlus     Mode = "plus"
	ModeEnhanced Mode = "enhanced"
)

// Modes 是全部三個版本，順序固定（旗標說明與選單都用它）。
func Modes() []Mode { return []Mode{ModeBase, ModePlus, ModeEnhanced} }

// Brain 是一個 AI。
type Brain interface {
	Mode() Mode

	// Name 是給人看的名字。
	Name() string

	// Derived 回報這個 AI 是不是已經**完整**從原版還原出來的。
	//
	// **false 表示它還不會下完整的棋**，不是「它比較弱」。呼叫端要把
	// 這件事顯示出來——一個安靜地什麼都不做的電腦諸侯，在畫面上看起來
	// 就只是「這個諸侯這回合沒動作」。
	Derived() bool

	// Coverage 回報九種行為裡解出了幾種。
	//
	// 原版的電腦諸侯每個郡每回合把**九種行為都跑一遍**，每種各有自己的
	// 條件（`docs/mechanics/70-ai`）——不是「從十道命令裡挑一道」。
	// 所以進度是「九分之幾」，不是布林值。
	Coverage() (done, total int)

	// Plan 回傳某個勢力這個月要下的命令。**不改變局面**。
	Plan(g *game.State, f state.FactionID) []game.Order
}

// New 依版本造一個 AI。
func New(m Mode) (Brain, error) {
	switch m {
	case ModeBase:
		return &faithful{mode: ModeBase, name: "三國演義（原版）"}, nil
	case ModePlus:
		return &faithful{mode: ModePlus, name: "三國演義1加強版"}, nil
	case ModeEnhanced:
		return &enhanced{}, nil
	}
	return nil, fmt.Errorf("ai: 不認識的版本 %q（有 %v）", m, Modes())
}

// faithful 是「以還原原版為目標」的 AI 的共同外殼。
//
// **只做已經從原版讀出來的行為。** 九種行為裡解出三種
// （內政、訓練兵士、指定太守、指定軍師、賞賜物品、尋訪人才、登用人才）。
// 沒解出來的一律不做——填一個「差不多的」策略進去，之後就再也分不出
// 哪些行為是還原的、哪些是我編的。
type faithful struct {
	mode Mode
	name string
}

func (f *faithful) Mode() Mode           { return f.mode }
func (f *faithful) Name() string         { return f.name }
func (f *faithful) Derived() bool        { return false }
func (f *faithful) Coverage() (int, int) { return 8, 9 }

// Plan 只發出已經解出來的那一種行為。
//
// 內政（原版的分派表 `0x5534`，`docs/re/03` §1.4）：
//
//	r = RND(K)      K ＝ [4,4,4,3,3,2]，由勢力的 AI 等級選
//	r == 0 → 土地開發
//	r == 1 → 洪水防治
//	否則   → 這回合不做
//
// **等級越高範圍越小、動手的機率越大**：等級 5 是 `RND(2)`，兩件事
// 各半、從不閒著；等級 0 是 `RND(4)`，一半的回合什麼都不做。
//
// ⚠ **這裡只還原了「選哪一道」，沒有還原「做多少」。** 原版的量是
// 地力 `+= (智 − 50)/12`、洪水率 `-= 智/10`，而 remake 的 `Reclaim`／
// `FloodControl` 用的是自己的係數（`docs/design/02`）。要對拍得先把
// 那兩個量也接過來。
func (f *faithful) Plan(g *game.State, id state.FactionID) []game.Order {
	var out []game.Order
	k := internalAffairsRange(g.AILevel(id))
	for _, p := range g.Territory(id) {
		// **行動者不是太守**：表 `0x54d4` 按「智 ＋ 武 ＋ 加權表[身分]」
		// 排序，取第一位——君主優先，其次軍師、太守、一般武將。
		act := actor(g, id, p)
		if act == nil {
			continue
		}
		gov := act
		// 內政（表 `0x5534`）
		switch g.Roll(k, int(id), p, 0x5534) {
		case 0:
			out = append(out, game.ReclaimOrder{At: p, General: gov.Index})
		case 1:
			out = append(out, game.FloodControlOrder{At: p, General: gov.Index})
		}
		// 訓練兵士（表 `0x5554`）：分派器每回合都跑，常式自己對整個
		// 守軍算，沒有額外的條件。
		out = append(out, game.TrainOrder{At: p})
		// 指定軍師（表 `0x5694`）：跑在指定太守之前。
		if x := betterChief(g, id, p); x != nil {
			out = append(out, game.AppointChiefOrder{At: p, Target: x.Index})
		}
		// 指定太守（表 `0x5674`）：守軍按魅力由高到低排序，第一位當
		// 太守。**已經是他就不必再指一次**——原版那一段是直接寫欄位，
		// remake 這一邊走命令，重複指定會白費一道紀錄。
		if best := mostCharming(g, id, p); best != nil && best.Index != gov.Index {
			out = append(out, game.AppointGovernorOrder{At: p, Target: best.Index})
		}
		// 尋訪人才（表 `0x5614`）：`RND(10) > 7`，也就是 20 %。
		// **機率不隨等級變**（六個項目都推 10）。
		if g.Roll(10, int(id), p, 0x5614) > 7 {
			if gov != nil {
				out = append(out, game.SearchOrder{At: p, General: gov.Index})
			}
		}
		// 登用人才（表 `0x5634`）：掃本郡身分 8（在野露面）的人。
		// **每郡最多 50 位將軍**（`0xced2` 的 `cmpw es:[0xc],50`）。
		// ⚠ 成功率的公式還沒解——等級參數 (30,0)/(20,10)/(10,20)/(0,40)
		// 被傳給另一支函式（`docs/re/03` §1.4）。
		if len(g.Garrison(p)) < game.MaxGeneralsPerPrefecture {
			if who := f.recruitTarget(g, p); who != nil {
				out = append(out, game.RecruitOrder{At: p, Target: who.Index})
			}
		}
		// 賞賜物品（表 `0x56b4`）：**等級 0–2 完全不做**（那三格是空操作）。
		out = append(out, f.rewards(g, id, p)...)
		// 購置武器（表 `0x5594`）：補到滿編為止。
		out = append(out, armsPurchase(g, p)...)
	}
	return out
}

// armsPurchase 是「購置武器」（表 `0x5594`，常式 `0xc168`，`L0`、`[base]`）。
//
// 走守軍清單，對每一位算：
//
//	現有武器 ＝ 武裝度 × 兵力 ÷ 100
//	缺口     ＝ 兵力 − 現有武器           ; 目標是人人有武器
//	花費     ＝ 缺口 ÷ 100                ; 截斷
//	花費 > 本回合預算 → 缺口 ＝ 預算 × 100 ; 買到錢用完為止
//	缺口 <= 0 → 這一位跳過
//
// **目標一律是滿編**，沒有門檻也沒有機率——原版這一支不擲骰。
// 缺口為零才跳過，所以武裝度已經 100 的人不會被重買。
//
// ⚠ **原版的預算是勢力層級的**（`es:[0x3d16]`，§2.12），它怎麼算出來
// 還沒解（`L3`）。這裡拿郡的金當上限，因為那是 remake 這一邊唯一
// 擋得住的東西——`ApplyAll` 遇到買不起會整串中斷，不是少買一點。
func armsPurchase(g *game.State, prefecture int) []game.Order {
	p := g.Prefecture(prefecture)
	if p == nil {
		return nil
	}
	budget := p.Gold
	var out []game.Order
	for _, x := range g.Garrison(prefecture) {
		gap := x.Soldiers - game.Weapons(int(x.Arms), x.Soldiers)
		if cost := gap / game.ArmsPerGold; cost > budget {
			gap = budget * game.ArmsPerGold
		}
		if gap <= 0 {
			continue
		}
		budget -= gap / game.ArmsPerGold
		out = append(out, game.ArmsOrder{At: prefecture, General: x.Index, Units: gap})
	}
	return out
}

// rewards 是「賞賜物品」（表 `0x56b4`，`L0`、`[base]`）。
//
// 三種寶物各跑一次（諸侯 offset 16／17／18），每一次：
//
//	RND(100) > 40 → 跳過          ; 41 % 才進行
//	存量 <= RND(2) + 2 → 跳過      ; 手上要夠多才送得出去
//	排序守軍、挑一位**非君主**的
//	該人忠誠上升，寶物存量 −1
//
// ⚠ **忠誠上升多少還沒解**（`L3`）——常式裡看得到 0–100 的夾取，
// 增幅那一段在讀到的範圍之外。`game.GiftTreasure` 用的還是 remake
// 自己的幅度。
func (f *faithful) rewards(g *game.State, id state.FactionID, prefecture int) []game.Order {
	if g.AILevel(id) < 3 {
		return nil // 等級 0–2 那三格是空操作
	}
	fa := g.Faction(id)
	if fa == nil {
		return nil
	}
	var out []game.Order
	// 只有 offset 16／17／18 那三格會被送出去；14 是玉璽（不能送人）。
	for i, t := range []game.Treasure{
		game.TreasureBlade, game.TreasureBeauty, game.TreasureHorse,
	} {
		if g.Roll(100, int(id), prefecture, i, 0x56b4) > 40 {
			continue
		}
		if fa.Treasury[t] <= g.Roll(2, int(id), prefecture, i)+2 {
			continue
		}
		who := f.rewardTarget(g, id, prefecture)
		if who == nil {
			continue
		}
		out = append(out, game.GiftOrder{At: prefecture, Target: who.Index, What: t})
	}
	return out
}

// recruitTarget 是登用的對象：本郡身分 8（在野露面）的人。
//
// 原版還收身分 10——**那個編碼 remake 沒有**，還沒解出是什麼
// （`docs/mechanics/20-personnel`）。
func (f *faithful) recruitTarget(g *game.State, prefecture int) *game.General {
	for _, x := range g.Free(prefecture) {
		if x.Status == state.StatusAvailable {
			return x
		}
	}
	return nil
}

// rewardTarget 是賞賜的對象：守軍裡忠誠最低的非君主。
//
// 原版在挑人之前先排序清單並跳過身分 0（君主）。**排序的鍵還沒解**
// （`L3`），這裡用「忠誠最低」——那是最合理的猜測，而且標了出來。
func (f *faithful) rewardTarget(g *game.State, id state.FactionID, prefecture int) *game.General {
	var pick *game.General
	for _, x := range g.Garrison(prefecture) {
		if x.Faction != id || x.Status == state.StatusLord {
			continue
		}
		if pick == nil || x.Loyalty < pick.Loyalty {
			pick = x
		}
	}
	return pick
}

// betterChief 是守軍裡可以接任軍師的人（`L0`、`[base]`）。
//
// 原版的常式（`0xd7ae`）走守軍清單，條件是
//
//	智 > 門檻   且   身分 ∈ {太守, 一般武將}
//
// 門檻是**現任軍師的智**，沒有軍師時是 79——那正是說明書「受封軍師之人
// 謀略不得低於 80」。門檻在迴圈裡**不更新**，所以原版取的是清單順序中
// 最後一位合格者，不是智力最高的那位。
//
// ⚠ **「最後一位」跟著清單順序走，而清單順序還沒解**（`L3`）。
// 這裡取 remake 自己的守軍順序中的最後一位，形狀對、人選不保證相同。
func betterChief(g *game.State, id state.FactionID, prefecture int) *game.General {
	floor := state.ChiefIntelFloor
	if cur := g.Chief(id); cur != nil {
		floor = int(cur.Intel)
	}
	var pick *game.General
	for _, x := range g.Garrison(prefecture) {
		if x.Faction != id || int(x.Intel) <= floor {
			continue
		}
		if x.Status != state.StatusGovernor && x.Status != state.StatusOfficer {
			continue
		}
		pick = x
	}
	return pick
}

// actorWeight 是「這回合誰行動」的身分加權（`L0`、`[base]`）。
//
// 原版的排序鍵是 **`智 + 武 + 加權表[身分]`**（`0xf1d7` 的
// `add ax, [bx+0x5986]`），表的內容從記憶體讀出來：
//
//	身分 0 君主 2000、1 軍師 1600、2 太守 1200、3 一般武將 800
//	身分 4–7 重複 2000／1600／1200／800
//	身分 8、9（在野）與 11（未登場）0、身分 10 是 400
//
// **權重完全壓過能力值**（智 ＋ 武 最多 200，而權重差是 400 的倍數），
// 所以排序實際上是「先看身分，同身分再比智 ＋ 武」。
var actorWeight = [12]int{2000, 1600, 1200, 800, 2000, 1600, 1200, 800, 0, 0, 400, 0}

// actor 是這個郡這回合的行動者（表 `0x54d4` 選出來的那一位）。
//
// 分派器的第一個呼叫把它寫進全域，**後面八種行為讀的都是它**
// （`docs/re/03` §1.4）。
func actor(g *game.State, id state.FactionID, prefecture int) *game.General {
	var best *game.General
	bestKey := -1
	for _, x := range g.Garrison(prefecture) {
		if x.Faction != id {
			continue
		}
		w := 0
		if int(x.Status) < len(actorWeight) {
			w = actorWeight[x.Status]
		}
		if k := int(x.Intel) + int(x.War) + w; k > bestKey {
			best, bestKey = x, k
		}
	}
	return best
}

// mostCharming 是守軍裡魅力最高的一位。
//
// 原版在指定太守之前先把守軍清單**按魅力由高到低排序**
// （`0xf600` 起的交換排序，比的是人物 offset 11），然後取第一位。
// 說明書只說「太守魅力越高，登用與賑民的效果越好」——這裡是 AI 實際
// 用的判準。
func mostCharming(g *game.State, id state.FactionID, prefecture int) *game.General {
	var best *game.General
	for _, x := range g.Garrison(prefecture) {
		if x.Faction != id {
			continue
		}
		if best == nil || x.Charm > best.Charm {
			best = x
		}
	}
	return best
}

// internalAffairsRange 是內政那張分派表的亂數範圍（`L0`、`[base]`）。
//
// 六份常式是同一段碼，只有 `mov ax,K` 的常數不同：
// 等級 0–2 是 `RND(4)`、3–4 是 `RND(3)`、5 是 `RND(2)`。
func internalAffairsRange(level int) int {
	switch {
	case level <= 2:
		return 4
	case level <= 4:
		return 3
	default:
		return 2
	}
}
