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
	"errors"
	"fmt"
	"os"
	"strconv"

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
// PrefecturePlanner 是「只替一個郡規劃、而且用指定等級」的可選能力。
//
// **郡縣自治要用它**：自治的郡屬於玩家，卻由電腦下令，等級來自玩家選的
// 型態而不是勢力的 AI 等級（`game.AutonomyAILevel`）。
// 沒有實作這個介面的 Brain，自治就只是一個設定不會有動作。
type PrefecturePlanner interface {
	PlanPrefecture(g *game.State, id state.FactionID, prefectureID, level int) []game.Order

	// ActPrefecture 是執行版：發一道套一道（見 `Brain.Act`）。
	ActPrefecture(g *game.State, id state.FactionID,
		prefectureID, level int) ([]game.Order, int, error)

	// TraceDraws 打開逐表的抽樣計數（對拍用；傳 nil 關掉）。
	TraceDraws(m map[string]int)
}

type Brain interface {
	Mode() Mode

	// Name 是給人看的名字。
	Name() string

	// Derived 回報這個 AI 是不是已經**完整**從原版還原出來的。
	//
	// **false 表示它還不會下完整的棋**，不是「它比較弱」。呼叫端要把
	// 這件事顯示出來——一個安靜地什麼都不做的電腦諸侯，在畫面上看起來
	// 就只是「這個諸侯這回合沒動作」。
	//
	// ⚠ **`Coverage()` 的分母是十八，不是九。** 分派器是一條直線，
	// `0xe926`–`0xec1b` 連續十八個 `lcall far [bx+表]` 之後 `lret`，
	// 中間沒有分支（`docs/re/03` §1.4）。而且**滿分也不等於
	// `Derived()` 為真**：判斷式讀出來之後，表底下還有沒量到的量
	// （賞賜的忠誠增幅與挑人的排序鍵、原版的亂數產生器）。
	Derived() bool

	// Coverage 回報十八種行為裡解出了幾種。
	//
	// 原版的電腦諸侯每個郡每回合把**十八種行為都跑一遍**，每種各有
	// 自己的條件（`docs/mechanics/70-ai`）——不是「從十道命令裡挑一道」。
	// 所以進度是「十八分之幾」，不是布林值。
	Coverage() (done, total int)

	// Plan 回傳某個勢力這個月要下的命令。**不改變局面**。
	//
	// ⚠ 它算出來的是「對著月初的盤面，這十八張表會做什麼」。原版不是
	// 這樣跑的——見 `Act`。要**執行**電腦的回合請用 `Act`，`Plan` 只
	// 適合預覽。
	Plan(g *game.State, f state.FactionID) []game.Order

	// Act 跑完某個勢力這個月的回合：**發一道就套一道**。
	//
	// 原版的分派器是逐郡、逐表即時執行的，所以第 n 張表看到的是前
	// n−1 張改過的盤面——徵兵加了兵，接著的調整兵力就攤平新的總數；
	// 開墾改了地力，接著的收成就用新的。`Plan` ＋ `ApplyAll` 是先對
	// 月初的盤面把十八張表全部算完再一次套上，兩者在**同一個郡裡**
	// 就會分岔（`docs/mechanics/70-ai` §2.14）。
	//
	// 回傳發出去的命令、成功套上的道數，以及第一道套不上去的錯誤。
	// **套不上去是 bug**：原版每一支常式自己檢查前提，不會失敗。
	Act(g *game.State, f state.FactionID) ([]game.Order, int, error)
}

// Edition 是這個 AI 要還原的原版版本；`enhanced` 沒有對應的版本，回空字串。
//
// **`base` 的 AI 配加強版的規則是不相容的組合**：兩者的難度係數表不同，
// 而且加強版收得下的難度原版的表放不下。呼叫端在開局時就要擋掉，不要等
// 到電腦諸侯出兵才讀到表外的位元組——那時候看起來只是「AI 有點怪」。
func (m Mode) Edition() state.Edition {
	switch m {
	case ModeBase:
		return state.EditionBase
	case ModePlus:
		return state.EditionPlus
	}
	return ""
}

// CheckEdition 回報這個 AI 版本能不能跑在這一版規則上。
func CheckEdition(m Mode, ed state.Edition) error {
	want := m.Edition()
	if want == "" || ed == "" || want == ed {
		return nil
	}
	return fmt.Errorf("ai: %q 是還原 %s 的 AI，不能跑在 %s 的規則上（要還原就兩邊同版，要混搭請用 %q）",
		m, want, ed, ModeEnhanced)
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
// **只做已經從原版讀出來的行為。** 十八種行為裡解出三種
// （內政、訓練兵士、指定太守、指定軍師、賞賜物品、尋訪人才、登用人才）。
// 沒解出來的一律不做——填一個「差不多的」策略進去，之後就再也分不出
// 哪些行為是還原的、哪些是我編的。
type faithful struct {
	// trace 非 nil 時，`planIn` 會把每一張表抽了幾次亂數記進去。
	// 對拍用：原版那一邊攔分派點就數得到，兩邊逐表比才定位得出
	// 「哪一支的迴圈次數不一樣」（`CONTEXT.md` 的亂數路線圖）。
	trace map[string]int

	mode Mode
	name string
}

func (f *faithful) Mode() Mode    { return f.mode }
func (f *faithful) Name() string  { return f.name }
func (f *faithful) Derived() bool { return false }

// Coverage 回報十八張分派表解出幾張。
//
// **`0x5514` 算解出來的**：六個等級的 far pointer 全部指向
// `33 c0 9a 1c 05 c4 05 cb`——配 0 位元組堆疊之後直接 `retf`，
// 也就是空操作。那張表在任何等級都不做事，所以「已解」的正確做法
// 就是不發任何命令（`docs/re/03` §1.4）。
func (f *faithful) Coverage() (int, int) { return 18, 18 }

// Plan 只發出已經解出來的那一種行為。
//
// 內政（原版的分派表 `0x5534`，`docs/re/03` §1.4）：
//
//	r1 = RND(K)     K ＝ [4,4,4,3,3,2]，由勢力的 AI 等級選
//	r1 == 0 → 土地開發（**不再擲**）
//	否則 r2 = RND(K)
//	r2 == 1 → 洪水防治
//	否則    → 這回合不做
//
// **等級越高範圍越小、動手的機率越大**：等級 5 是 `RND(2)`，開墾 ½、
// 防洪 ¼、閒著 ¼；等級 0 是 `RND(4)`，9/16 的回合什麼都不做。
//
// 「做多少」也跟著等級走（`game.AffairsTier`）：開墾的底是
// `[50,60,60,50,40,50]`、防洪的除數是 `[10,15,15,14,12,10]`。
// 六支常式是同一段碼，只有這三個立即數不同。
func (f *faithful) Plan(g *game.State, id state.FactionID) []game.Order {
	out, _, _ := f.planIn(g, id, g.Territory(id), g.AILevel(id), false)
	return out
}

// Act 是實際跑電腦的回合：發一道套一道（`Brain.Act`）。
func (f *faithful) Act(g *game.State, id state.FactionID) ([]game.Order, int, error) {
	return f.planIn(g, id, g.Territory(id), g.AILevel(id), true)
}

// PlanPrefecture 只替一個郡規劃，而且用指定的 AI 等級。
//
// **郡縣自治用的是這一條**：原版的郡回合入口看到州郡 offset 12 不是 0，
// 就拿那個值減一當等級去跑同一個分派器（`0x17550`，`game.AutonomyAILevel`）
// ——所以自治的郡跑的是電腦的行為，只是等級由玩家指定的型態決定。
func (f *faithful) PlanPrefecture(g *game.State, id state.FactionID,
	prefectureID, level int) []game.Order {
	out, _, _ := f.planIn(g, id, []int{prefectureID}, level, false)
	return out
}

// ActPrefecture 是自治郡那一條的執行版：同樣發一道套一道。
func (f *faithful) ActPrefecture(g *game.State, id state.FactionID,
	prefectureID, level int) ([]game.Order, int, error) {
	return f.planIn(g, id, []int{prefectureID}, level, true)
}

// TraceDraws 打開逐表的抽樣計數，寫進 m。傳 nil 關掉。
func (f *faithful) TraceDraws(m map[string]int) { f.trace = m }

func (f *faithful) planIn(g *game.State, id state.FactionID,
	territory []int, aiLevel int, live bool) ([]game.Order, int, error) {
	var out []game.Order
	var failed error
	applied := 0
	// emit 是「發一道」。`live` 為真時**當場套上去**，後面的表因此看到
	// 前面改過的盤面——原版的分派器就是這樣跑的。
	emit := func(o game.Order) {
		if failed != nil {
			return
		}
		out = append(out, o)
		if !live {
			return
		}
		// **`ErrDeclined` 不算違規**：登用被婉拒是判定的正常結果，
		// 命令本身執行成功了（`game.ApplyAll` 同一條判準）。當成中斷
		// 的理由會讓一次登用失敗吃掉同一輪後面所有的命令。
		if err := o.Apply(g, id); err != nil && !errors.Is(err, game.ErrDeclined) {
			failed = fmt.Errorf("第 %d 道（%s）：%w", len(out), o.Describe(g), err)
			return
		}
		applied++
	}
	lastDraws := g.RandDraws()
	// 追值的時候順便記下這一張表跑完之後郡的兵、金、米。抽樣次數對上
	// 之後剩下的就是**量**的差，而量的差要逐表看才知道是哪一支。
	curP, watch := -1, watchPrefecture()
	mark := func(name string) {
		if f.trace == nil {
			return
		}
		now := g.RandDraws()
		f.trace[name] += now - lastDraws
		lastDraws = now
		if curP == watch {
			if q := g.Prefecture(curP); q != nil {
				n := 0
				for _, x := range g.Garrison(curP) {
					n += x.Soldiers
				}
				f.trace[fmt.Sprintf("值|%-10s 人口 %6d 兵(百) %3d 金 %5d 米 %5d",
					name, q.Population, n/100, q.Gold, q.Rice)]++
			}
		}
	}
	k := internalAffairsRange(aiLevel)
	for _, p := range territory {
		curP = p
		if failed != nil {
			break
		}
		// **錢包要跟著這一輪扣。** 原版每一支常式開頭都看一次本回合的
		// 預算（`es:[0x3d16]`）；remake 這一邊沒有那個數，用郡的金頂著。
		// 對著開局餘額規劃的話，後面幾道會被 `ErrNoGold` 擋下來，
		// 而 `ApplyAll` 會連同再後面的命令一起作廢。
		purse := 0
		if x := g.Prefecture(p); x != nil {
			purse = x.Gold
		}
		// **付不付得起比的是郡的金**，不是本回合預算：尋訪、開墾、登用
		// 這幾支不在那六張「呼叫前先算預算」的名單裡，它們直接扣郡的金。
		// 拿模型化的錢包判斷會讓後面的表誤以為沒錢，跟著少抽一批。
		gold := func() int {
			if !live {
				return purse
			}
			if q := g.Prefecture(p); q != nil {
				return q.Gold
			}
			return 0
		}
		afford := func(cost int) bool {
			if gold() < cost {
				return false
			}
			purse -= cost
			return true
		}
		// **行動者不是太守**：表 `0x54d4` 按「智 ＋ 武 ＋ 加權表[身分]」
		// 排序，取第一位——君主優先，其次軍師、太守、一般武將。
		act := actor(g, id, p)
		if act == nil {
			continue
		}
		gov := act
		// **內政與尋訪讀的是「智最高」那個全域**（`es:[0x4196]`），
		// 不是行動者。
		brain := smartest(g, p)
		if brain == nil {
			brain = act
		}
		// **本回合預算的分母是「當下的郡的金」，不是一路扣下來的錢包。**
		// 分派器在那六張表之前各算一次（`0xe9a1`–`0xe9fc`）：
		//
		//	索引 = 等級×4 + 月 mod 4
		//	es:[0x3d16] = 係數表[索引] × 州郡 offset 18（金）× 0.01
		//
		// 每一張都重算，所以前一張花掉的錢只透過**郡的金**影響後一張，
		// 不是把額度直接扣掉。拿剩餘錢包當分母會讓越後面的表越窮
		//（挖角那道 `預算 >= 100` 反而過得太寬，量到多觸發四次）。
		// ⚠ **預覽（`Plan`）讀不到真的支出**：那一條不套用命令，郡的金
		// 不會動，所以只能用模型化的錢包頂著，否則會排出付不出來的命令。
		// **順序照原版的分派器**（`0xe926`–`0xec1b`，`docs/re/03` §1.4）：
		// 行動者 → 指定軍師 → 指定太守 → 尋訪 → 登用 → 訓練 → 內政 →
		// 賞賜物品 → 武器 → 徵兵 → 賑民 → 賞賜金帛 → 挖角 → 計略 →
		// 調整兵力 → 買米 → 出兵。十八張全部無條件執行，中間沒有分支。
		//
		// **順序有意義**：這裡是發一道套一道（`emit`），後面的表看到的
		// 是前面改過的盤面，而錢包也一路扣下去。
		mark("行動者")
		// 指定軍師（表 `0x5694`）：跑在指定太守之前。
		if f.trace != nil && p == watch {
			who := -1
			if x := betterChief(g, id, p); x != nil {
				who = x.Index
			}
			cur := -1
			if fa := g.Faction(id); fa != nil {
				cur = fa.Chief
			}
			f.trace[fmt.Sprintf("軍師｜郡 %d 現任 %d 換成 %d", p, cur, who)]++
		}
		if x := betterChief(g, id, p); x != nil {
			emit(game.AppointChiefOrder{At: p, Target: x.Index, Auto: true})
		}
		mark("指定軍師")
		// 指定太守（表 `0x5674`）：守軍按魅力由高到低排序，第一位當
		// 太守。**已經是他就不必再指一次**——原版那一段是直接寫欄位，
		// remake 這一邊走命令，重複指定會白費一道紀錄。
		if best := mostCharming(g, id, p); best != nil && best.Index != gov.Index {
			emit(game.AppointGovernorOrder{At: p, Target: best.Index, Auto: true})
		}
		mark("指定太守")
		// 尋訪人才（表 `0x5614`）：`RND(10) > Bar[等級]`。
		// **三個常數都隨等級變**（`game.SearchTierFor`，`L1`）：
		// 出手的機率從 20 % 升到 50 %，門檻從 30–94 降到 15–34。
		// **沒有錢的閘門**：原版那一條不收錢（`0xcc86` 沒碰 offset 18），
		// 所以「付不付得起」根本不是條件。加上它會讓窮郡少做一次尋訪，
		// 而且郡的金也跟著錯。
		if g.Roll(10, int(id), p, 0x5614) > game.SearchTierFor(aiLevel).Bar {
			emit(game.SearchOrder{At: p, General: brain.Index, Auto: true})
		}
		mark("尋訪")
		// 登用人才（表 `0x5634`）。**每郡最多 50 位將軍**
		//（`0xced2` 的 `cmpw es:[0xc],50`）；判定在 `game.Recruit`
		//（`0xce8c`，`docs/re/03` §1.4），等級參數
		// (30,0)/(20,10)/(10,20)/(0,40) 是**費用與加成**。
		//
		// **不挑人，全部都試一遍**（`0xd0ae`）：掃 0–349，所在郡相符
		// 且身分 ∈ {8, 10} 的每一位都呼叫一次 `0xce8c`，照槽號由小到大。
		// 擋住的是 `0xce8c` 裡的每回合預算（`es:[0x3d16]`）與 50 位上限，
		// 不是「只登用一位」。
		//
		// 席次照**成功**算：`ApplyAll` 把擋下來的命令當成違規、中斷同一
		// 輪後面全部的命令，所以寧可少發也不要發出套不上去的。
		seats := game.MaxGeneralsPerPrefecture - g.ActiveGenerals(p)
		for _, who := range g.Recruitable(p) {
			if seats <= 0 || !afford(game.RecruitFee(aiLevel)) {
				break
			}
			seats--
			emit(game.RecruitOrder{At: p, Target: who.Index})
		}
		mark("登用")
		// 訓練兵士（表 `0x5554`）：分派器每回合都跑，常式自己對整個
		// 守軍算，沒有額外的條件。
		emit(game.TrainOrder{At: p})
		mark("訓練")
		// 內政（表 `0x5534`，六份常式 `0xba9c`／`0xbb12`／`0xbb88`／
		// `0xbbfe`／`0xbc74`／`0xbcea`，`L0`）。**是兩次擲骰不是一次**：
		//
		//	r1 = RND(K)                    ; 0xbcf5
		//	r1 == 0 → 開墾，**不再擲**      ; 0xbcff 的 jne 跳過第二次
		//	否則 r2 = RND(K)               ; 0xbd2a
		//	r2 == 1 → 防洪，否則這回合不做  ; 0xbd36 的 dec/jne
		//
		// 所以機率是開墾 `1/K`、防洪 `(1−1/K)/K`——**不是各 `1/K`**。
		// 等級 5（K ＝ 2）是開墾 ½、防洪 ¼、閒著 ¼。
		if f.trace != nil && p == watch {
			for i, x := range roster(g, p) {
				f.trace[fmt.Sprintf("名單｜郡 %d 第 %d 位 %d 勢力 %d 身分 %d "+
					"智 %d 武 %d 魅 %d 鍵 %d", p, i, x.Index, x.Faction,
					x.Status, x.Intel, x.War, x.Charm, actorKey(x))]++
			}
			t := game.AffairsTierFor(aiLevel)
			f.trace[fmt.Sprintf("內政｜郡 %d 等級 %d K %d 智最高 %d 智 %d 底 %d 量 %d",
				p, aiLevel, k, brain.Index, brain.Intel, t.LandFloor,
				(int(brain.Intel)-t.LandFloor)/12)]++
		}
		if g.Roll(k, int(id), p, 0x5534) == 0 {
			// 開墾不會因為錢不夠而失敗（「若財庫已空則徒手開墾」）。
			// **電腦那一條不收錢**（`0xba02`／`0xbd39` 都沒碰
			// 州郡 offset 18），所以也沒有「付不付得起」這一關。
			emit(game.ReclaimOrder{At: p, General: brain.Index, Auto: true})
		} else if g.Roll(k, int(id), p, 0x5534, 1) == 1 {
			emit(game.FloodControlOrder{At: p, General: brain.Index, Auto: true})
		}
		mark("內政")
		// 賞賜物品（表 `0x56b4`）：**等級 0–2 完全不做**（那三格是空操作）。
		// 四種寶物各記一個桶，才對得上原版那四支常式
		// （`0xd962`／`0xdac0`／`0xdc1e`／`0xdd94`）的分帳。
		f.rewards(g, id, p, emit, mark)
		mark("賞賜物品")
		// 購置武器（表 `0x5594`）：預算是郡的金的 2 %。
		bought := armsPurchase(g, p, aiBudget(gold(), aiLevel, tableArms))
		for _, o := range bought {
			emit(o)
		}
		// **扣的是真的花掉的，不是配下去的額度**：原版每一支常式都重讀
		// 一次郡的金，而金只被實際的支出扣減。
		for _, o := range bought {
			purse -= o.(game.ArmsOrder).Units / game.ArmsPerGold
		}
		mark("武器")
		// 徵兵（表 `0x5574`）：預算是**剩下的**金的 30–50 %。
		// 原版每一支常式都重讀一次郡的金，所以後面的表看到的是
		// 前面花剩的（`docs/mechanics/70-ai` §2.14）。
		drafted := conscript(g, p, aiLevel, aiBudget(gold(), aiLevel, tableConscript))
		if f.trace != nil && p == watch {
			out := ""
			for _, o := range drafted {
				c := o.(game.ConscriptOrder)
				out += fmt.Sprintf(" %d:+%d", c.General, c.Count)
			}
			f.trace[fmt.Sprintf("徵兵｜郡 %d 預算 %d 金 %d%s", p,
				aiBudget(gold(), aiLevel, tableConscript), gold(), out)]++
		}
		for _, o := range drafted {
			emit(o)
		}
		// 徵兵是一兵一金（說明書 p.20），錢包一樣要跟著扣——
		// **不扣的話最後那一張「出兵」會拿月初的餘額去算隨行的錢**，
		// 執行時就撞上 `ErrNoGold`，而 `ApplyAll` 會把整批作廢。
		for _, o := range drafted {
			purse -= o.(game.ConscriptOrder).Count
		}
		if f.trace != nil && p == watch {
			out := ""
			for _, x := range g.Garrison(p) {
				out += fmt.Sprintf(" [%d 兵 %d 訓 %d 武裝 %d]",
					x.Index, x.Soldiers, x.Training, x.Arms)
			}
			f.trace["守軍｜徵兵之後"+out]++
		}
		mark("徵兵")
		// 開倉賑民（表 `0x55f4`）：民眾忠誠低於「底 ＋ RND(20)」才做，
		// 撥的是**整份預算**（郡的金的 10–20 %）。
		if o, ok := relief(g, p, id, aiBudget(gold(), aiLevel, tableRelief)); ok {
			emit(o)
			purse -= o.Gold
		}
		mark("賑民")
		// 賞賜金帛（表 `0x5654` → `0xd5e6`）：**走名單，跳過君主，
		// 每一位都給一次**（`0xd302`）。每一位抽一次 `RND(加成/2)`
		// （在 `game.Reward` 裡），而花掉的是「真的換到的那一段」
		// ——忠誠接近 100 的人賞下去的錢很少，所以同一份預算撐得比
		// 「每人 100」久得多。原版一輪在這一支抽了 196 次，幾乎等於
		// 名單的總長度（`CONTEXT.md` 的亂數路線圖）。
		rewardBudget := aiBudget(gold(), aiLevel, tableReward)
		for _, x := range roster(g, p) {
			if rewardBudget <= 0 {
				break
			}
			// ⚠ 原版的名單不比對勢力，但 `game.Reward` 要求同勢力
			// ——`0xd302` 有沒有這一道還沒讀，先留著。
			if x.Faction != id || x.Status == state.StatusLord || x.Rewarded {
				continue
			}
			gold := rewardBudget
			if gold > game.MaxReward {
				gold = game.MaxReward
			}
			// **賞金要夾在郡的現金之內**：`game.Reward` 錢不夠會回
			// `ErrNoGold`，而那會中斷同一輪後面全部的命令。
			was := 0
			if q := g.Prefecture(p); q != nil {
				was = q.Gold
				if gold > q.Gold {
					gold = q.Gold
				}
			}
			if gold <= 0 {
				break
			}
			emit(game.RewardOrder{At: p, Target: x.Index, Gold: gold})
			spent := gold
			if q := g.Prefecture(p); q != nil && was-q.Gold >= 0 && live {
				spent = was - q.Gold
			}
			rewardBudget -= spent
			purse -= spent
		}
		mark("賞賜金帛")
		// 挖角（表 `0x56d4`）：**君主要在本郡**，機率隨等級 30／60／80 %，
		// 預算要 ≥ 100，費用是直接扣的 100 金。
		hhBefore := g.RandDraws()
		if o, ok := headhunt(g, p, id, gold()); ok {
			emit(o)
			purse -= game.CostHeadhunt
		}
		// 掃描一次約抽兩百次，所以「有沒有觸發」用抽了幾次就分得出來。
		if f.trace != nil && g.RandDraws()-hhBefore > 5 {
			f.trace[fmt.Sprintf("挖角：郡 %d 金 %d", p, gold())]++
		}
		mark("挖角")
		// 計略（表 `0x56f4`）：**軍師本人要在這個郡**，機率隨等級
		// 10／12.5／20 %；目標是**全圖**任何一個敵郡，使者取本郡魅力
		// 最高的人。
		if o, ok := plot(g, p, id); ok {
			emit(o)
		}
		mark("計略")
		// 調整兵力（表 `0x55b4`）：**不花錢，也不隨等級變**——六格全部
		// thunk 到同一支 `0xc2c4`。它把整郡的兵按帶兵上限重新攤平，
		// 訓練度與武裝度拉到全郡的加權平均。
		if who := garrisonIndices(g, p); len(who) >= 2 {
			emit(game.RedistributeOrder{At: p, Units: who})
		}
		mark("調整兵力")
		// 米糧買賣（表 `0x55d4`）：**不走回合預算也不打折**，
		// 它是市場交易，而且**雙向**——存糧目標跟著兵力走，低了買、
		// 高了賣（`game.TradeRiceTo`）。
		if o, ok := buyRice(g, p, id, gold()); ok {
			was := 0
			if q := g.Prefecture(p); q != nil {
				was = q.Gold
			}
			emit(o)
			if q := g.Prefecture(p); q != nil && live {
				purse -= was - q.Gold
			}
		}
		mark("買米")
		// 出兵／移防（表 `0x54f4`）：**分派器的最後一張**，等級 3 以上
		// 才做。四道門檻、洗牌編隊、三選一目標，見 sortie。
		if o, ok := f.sortie(g, p, id, purse); ok {
			emit(o)
		}
		mark("出兵")
	}
	return out, applied, failed
}

// 出兵的四道門檻（`0xb666`／`0xb47a`，`L0`、`[base]`）。
const (
	SortieMinTroops   = 5  // 州郡.兵士（百）至少 5，也就是 500 人
	SortieRicePerUnit = 15 // 米要有 兵士（百） × 15
	SortieMinLevel    = 3  // 等級 0–2 那三格是空操作
	SortieMaxGenerals = 50 // 目標郡的現役將加上出征人數不得超過 50
)

// 難度係數表（`DS:0x5430`，一格 8 byte 的 double，`L0`）。
//
// **只有「打敵國」那一條分支用得到**：
// `係數 × 出征兵力（百） < 目標郡的兵士（百）` 就不打。
// 係數越小同樣的兵力越難過門檻——原版難度 10 要帶到守軍的兩倍才動手。
//
// 兩版的表不同，而且**加強版長一倍**（`docs/spec/004` §3）：
// 原版 11 格、加強版 21 格，第 0 格用不到。加強版的 11–20 與 1–10
// 逐格相同——多出來的十級不改變出兵的積極度。
var sortieOdds = map[state.Edition][]int{
	state.EditionBase: {100, 90, 100, 80, 80, 70, 70, 60, 60, 50},
	state.EditionPlus: {
		100, 90, 90, 80, 70, 75, 70, 75, 70, 60,
		100, 90, 90, 80, 70, 75, 70, 75, 70, 60,
	},
}

// SortieOdds 是某個版本、某個難度的出兵係數，以百分比表示。
//
// 版本空字串當原版。**越界回 100 不是「安全的預設」而是「不加碼也不
// 打折」**——回 0 會讓電腦諸侯一次都不出兵，而那在畫面上看起來只是
// 「這個諸侯很消極」。
func SortieOdds(ed state.Edition, difficulty int) int {
	t, ok := sortieOdds[ed]
	if !ok {
		t = sortieOdds[state.EditionBase]
	}
	if difficulty < 1 || difficulty > len(t) {
		return 100
	}
	return t[difficulty-1]
}

// sortie 是「出兵／移防」（表 `0x54f4`，`L0`＋`L1`、`[base]`）。
//
// 四道門檻任何一道不過就整個不做（`0xb666`）：
//
//	兵士（百） >= 5
//	兵士（百） <= 郡的金
//	米 >= 兵士（百） × 15
//	出征清單非空
//
// 目標是 `RND(4)` 三選一（`0xb47a`）：0 無主的鄰郡（占領）、
// 1 自己的鄰郡（移防）、2 與 3 敵國的鄰郡（進攻）。
//
// **原版編兩次隊**：`0xb2b4` 在挑目標之前先編一次，那一次決定進攻要用的
// 兵力；`0xb706` 在出發時**再洗一次牌重編**，實際走的是後面那一隊。兩次
// 各自擲骰，所以評估的部隊與上路的部隊不是同一批。差別在收尾：
// `0xb2b4` 允許整郡被留守吃光（清單長度收成 0 → 這次出兵作廢），
// `0xb706` 的迴圈用 `jg`，位置 0 永遠留著，所以出發時至少有一個人。
func (f *faithful) sortie(g *game.State, prefecture int, id state.FactionID,
	purse int) (game.Order, bool) {
	if g.AILevel(id) < SortieMinLevel {
		return nil, false
	}
	p := g.Prefecture(prefecture)
	if p == nil {
		return nil, false
	}
	troops := 0
	for _, x := range g.Garrison(prefecture) {
		troops += x.Soldiers
	}
	units := troops / 100 // 原版整份用「百」當單位
	// **入口只有一道門檻**：`0xb666`／`0xb696`／`0xb6c6` 三個等級的碼
	// 一模一樣，都只判「州郡 offset 16（兵士，百）>= 5」，然後就
	// `0xb2b4`（編隊）再 `0xb47a`（挑目標）。
	if units < SortieMinTroops {
		return nil, false
	}

	free, mine, foe := neighbourLists(g, p, id)
	want := sortieTarget(g, prefecture, foe)

	// **先編隊再擋。** 金與米那兩道在 `0xb47a` 裡，而編隊的洗牌
	// （`0xb2b4`）排在它前面——所以被擋下來的那幾次，原版**還是抽過了**。
	// 先擋再編會少抽一整批（月度對拍量到出兵這一支少 106 次）。
	survivors := muster(g, prefecture, id, want, 2, false)

	// `0xb47a` 的三道，順序照原版：
	//
	//	兵士(百) > 州郡 offset 18（金）→ 作廢          ; 0xb49c
	//	兵士(百) × 15 > 州郡 offset 20（米）→ 作廢     ; 0xb4cb
	//	清單長度 < 1 → 作廢                            ; 0xb4d8
	//
	// **金看的是郡的金，不是本回合預算**——出兵是分派器的最後一張，
	// 前面十七張花掉的錢不影響這一道。
	if f.trace != nil {
		f.trace[fmt.Sprintf("出兵：郡 %d 兵(百) %d 金 %d 米 %d 守軍 %d 留下 %d",
			prefecture, units, p.Gold, p.Rice,
			len(g.Garrison(prefecture)), len(survivors))]++
	}
	if units > p.Gold || p.Rice < units*SortieRicePerUnit || len(survivors) == 0 {
		return nil, false
	}

	var targets []int
	switch g.Roll(4, int(id), prefecture, tableSortie) {
	case 0:
		targets = free
	case 1:
		targets = mine
	default:
		targets = foe
	}
	if len(targets) == 0 {
		return nil, false
	}
	to := targets[g.Roll(len(targets), int(id), prefecture, tableSortie, 1)]

	// 第二次編隊（`0xb706`）：另擲一輪，這一隊才是真的上路的。
	force := muster(g, prefecture, id, want, 1000, true)
	if len(force) == 0 {
		return nil, false
	}
	// 「每郡最多 50 位將軍」在這裡是**裁隊伍**，不是取消出兵（`0xb706`）。
	if room := SortieMaxGenerals - len(g.Garrison(to)); len(force) > room {
		if room <= 0 {
			return nil, false
		}
		force = force[:room]
	}
	sent := 0
	for _, i := range force {
		if x := g.General(i); x != nil {
			sent += x.Soldiers
		}
	}

	if q := g.Prefecture(to); q != nil && q.Owned() && q.Owner != id {
		// 進攻多一道兵力比較（`0xb47a`）。
		enemy := 0
		for _, x := range g.Garrison(to) {
			enemy += x.Soldiers
		}
		if game.SortieThreshold(SortieOdds(g.Edition, g.Difficulty), sent/100) < enemy/100 {
			return nil, false
		}
		return game.AttackOrder{At: prefecture, To: to, Force: force}, true
	}
	// 無主的郡與自己的郡是移防，不是戰役。**帶走的錢糧按兵力比例**
	// （`0xb706`）：`郡的金 ÷ 兵士（百） × 出征兵力（百）`，走浮點。
	return game.MoveOrder{
		At: prefecture, To: to, General: force[0],
		Gold: min(game.SortieShare(purse, units, sent/100), purse),
		Rice: min(game.SortieShare(p.Rice, units, sent/100), p.Rice),
	}, true
}

// neighbourLists 把鄰郡分成三堆：無主的、自己的、別人的
// （`0xee1e`／`0xeee8`／`0xef5c`，掃的是十格的相鄰表）。
func neighbourLists(g *game.State, p *game.Prefecture, id state.FactionID) (free, mine, foe []int) {
	for _, n := range p.Neighbours {
		q := g.Prefecture(n)
		if q == nil {
			continue
		}
		switch {
		case !q.Owned():
			free = append(free, n)
		case q.Owner == id:
			mine = append(mine, n)
		default:
			foe = append(foe, n)
		}
	}
	return
}

// SortieTargetFloor 是兵力目標的起始值（`0xeccb` 的 `movw $5`）。
const SortieTargetFloor = 5

// sortieTarget 是**留守**的兵力目標（`es:[0x2e62]`，`L0`＋`L1`）。
//
//	起始 5
//	→ 本郡守將裡最小的非零 `兵力 ÷ 100`（只往下改，`0xede6`）
//	→ 每一個敵國鄰郡的 `兵士（百）` 取最大（只往上改，`0xee99`）
//
// **整條都以「百」為單位**，編隊那一支再乘回 100。編隊從尾端拿人拿到
// 累計兵力跨過這個數為止，拿掉的留在家裡、留下的出征——所以這個數是
// 「家裡要留多少」，不是「要帶多少出去」。原版關掉出兵的三個地方
// （`0xf0a9` 等）就是把它設成 9999，讓整郡的守將全部被留守吃掉。
func sortieTarget(g *game.State, prefecture int, foe []int) int {
	want := SortieTargetFloor
	for _, x := range g.Garrison(prefecture) {
		if v := x.Soldiers / 100; v != 0 && v < want {
			want = v
		}
	}
	for _, id := range foe {
		total := 0
		for _, x := range g.Garrison(id) {
			total += x.Soldiers
		}
		if v := total / 100; v > want {
			want = v
		}
	}
	return want
}

// muster 編隊（`0xb2b4`／`0xb706`，`L0`＋`L1`）。
//
// 守將清單先逐格與 `RND(n)` 交換洗牌，再**從尾端往前把人拿掉**，
// 累計被拿掉的兵力到 `100 × want` 為止，然後把清單截到停下來的位置。
// **留下來的頭段才是出征的部隊**：原版接著只掃 `0..長度−1` 求和，
// 那個和是出征兵力，也是帶走錢糧的比例基準，還是 50 人上限要裁的對象
// （`目標郡的現役將 ＋ 出征人數 <= 50`）。被拿掉的那一批留在原郡。
//
// `keepOne` 是兩支的唯一差別：規劃那一次（`0xb2b4`）用 `jge`，位置 0
// 也會被拿掉，整郡吃光就回空；出發那一次（`0xb706`）用 `jg`，位置 0
// 永遠留著。
func muster(g *game.State, prefecture int, id state.FactionID, want, salt int, keepOne bool) []int {
	who := garrisonIndices(g, prefecture)
	if len(who) == 0 {
		return nil
	}
	// 逐格與 RND(n) 交換——原版就是這樣洗的。
	for i := range who {
		j := g.Roll(len(who), int(id), prefecture, tableSortie, salt+i)
		who[i], who[j] = who[j], who[i]
	}
	stop := 0
	if keepOne {
		stop = 1
	}
	left, got := len(who), 0
	for i := len(who) - 1; i >= stop && got < 100*want; i-- {
		if x := g.General(who[i]); x != nil {
			got += x.Soldiers
		}
		left = i
	}
	return who[:left]
}

// watchPrefecture 是要逐表記錄兵金米的那個郡，`SAN1_WATCH` 沒設就關掉。
// 對拍在追「抽樣次數對上但量不對」時用。
func watchPrefecture() int {
	v := os.Getenv("SAN1_WATCH")
	if v == "" {
		return -1
	}
	n, err := strconv.Atoi(v)
	if err != nil {
		return -1
	}
	return n
}

// 分派表的位址，當識別碼用。
const (
	tableArms      = 0x5594 // 購置武器
	tableConscript = 0x5574 // 徵兵
	tableRelief    = 0x55f4 // 開倉賑民
	tableReward    = 0x5654 // 賞賜金帛
	tableRice      = 0x55d4 // 買入米糧
	tableHeadhunt  = 0x56d4 // 挖角
	tablePlot      = 0x56f4 // 計略
	tableSortie    = 0x54f4 // 出兵／移防
)

// aiBudgetPercent 是「本回合預算佔郡的金的百分之幾」（`L0`、`[base]`）。
//
// 分派器在呼叫這幾支之前先算一次 `es:[0x3d16]`：
// `預算 ＝ 郡的金 × 係數 ÷ 100`，係數表在 `DS:0x5714`，
// 24 筆 ＝ 6 個等級 × 4 個相位（`docs/mechanics/70-ai` §2.14）。
//
// **這四張表的係數不隨季節變**，所以這裡不必看季節。
// 會隨季節變的是 `0x5514` 與 `0x56d4`（挖角）——`es:[0x3f08]` 就是季節
// （`docs/re/06`），而 `0x5514` 八格全是空操作，所以只有挖角受影響
// （`HeadhuntBudget`）。
var aiBudgetPercent = map[int][6]int{
	tableArms:      {2, 2, 2, 2, 2, 2},
	tableConscript: {30, 30, 40, 50, 50, 50},
	tableRelief:    {20, 20, 10, 10, 20, 20},
	tableReward:    {20, 20, 20, 15, 15, 15},
}

func aiBudget(gold, level, table int) int {
	pct, ok := aiBudgetPercent[table]
	if !ok || gold <= 0 {
		return 0
	}
	if level < 0 {
		level = 0
	}
	if level >= len(pct) {
		level = len(pct) - 1
	}
	return gold * pct[level] / 100
}

// conscript 是「徵兵」（表 `0x5574`，常式 `0xbeb8`，`L0`、`[base]`）。
//
// 走守軍清單，對每一位徵到帶兵上限為止：
//
//	空額 ＝ 帶兵上限[職位] − 兵力
//	人數 ＝ min(空額, 預算)                 ; 每人 1 金
//	人數 ＝ min(人數, max(人口 − 3000, 0))  ; 人口下限
//	人數 ≤ 0 → 這一位跳過
//
// 徵完人口等量減少，訓練度與武裝度都按新的兵力重算——**新兵沒受訓
// 也沒武器**，兩個欄位走的是同一個加權平均（`game.ArmsOf`）。
func conscript(g *game.State, prefecture, level, budget int) []game.Order {
	p := g.Prefecture(prefecture)
	if p == nil {
		return nil
	}
	people := p.Population
	var out []game.Order
	// **走排序後的名單**（`es:[0x58c]`，行動者那一張排完的順序），
	// 不是槽號順序——前面的人先徵，而預算是遞減的，順序換了配額就換人。
	for _, x := range roster(g, prefecture) {
		// **預算 ≤ 0 就收工**（`0xbf1a`）。
		if budget <= 0 {
			break
		}
		n := x.TroopCap() - x.Soldiers
		if n > budget {
			n = budget
		}
		if room := people - game.MinPopulationToConscript; n > room {
			n = room
		}
		// **`<= 0` 是整個迴圈結束，不是跳過這一位**（`0xbf98` 跳出去）。
		if n <= 0 {
			break
		}
		// **扣掉的是花掉的金，不是人數**（`0xc061` 走付錢那一支）。
		// 電腦有折扣，等級 5 是 0.75——所以徵 323 人只花 242 金，
		// 預算從 323 掉到 81 而不是掉到 0，下一位還徵得到。
		// 量到的一輪：323 → 81 → 21 → 6 → 2 → 1 → 1…（郡 11，26 位守軍）。
		// 寫成 `budget -= n` 會讓第一位吃光整份預算，其餘掛零。
		budget -= state.AICost(n*game.CostConscriptPerSoldier, level)
		people -= n
		out = append(out, game.ConscriptOrder{At: prefecture, General: x.Index, Count: n})
	}
	return out
}

// relief 是「開倉賑民」（表 `0x55f4`，常式 `0xc8f6`，`L0`、`[base]`）。
//
//	門檻 ＝ 門檻底[等級] ＋ RND(20)      ; 底 ＝ 80,80,70,60,80,80
//	民眾忠誠 >= 門檻 → 這回合不做
//	撥出整份預算，效果見 `game.ReliefGain`
//
// **等級 2、3 的門檻反而低**（70、60），做得比等級 0、1 少；
// 等級 4、5 門檻回到 80，但每一分錢換到的忠誠多（除數 9 與 7）。
func relief(g *game.State, prefecture int, id state.FactionID, budget int) (game.ReliefOrder, bool) {
	p := g.Prefecture(prefecture)
	if p == nil {
		return game.ReliefOrder{}, false
	}
	// **先擲再看預算。** 原版這一次無條件抽（月度對拍量到每郡一次，
	// 32／32），預算是不是 0 是後面才判的。
	bar := game.ReliefThreshold(g.AILevel(id)) + g.Roll(20, int(id), prefecture, tableRelief)
	if budget <= 0 {
		return game.ReliefOrder{}, false
	}
	if int(p.PublicLoyalty) >= bar {
		return game.ReliefOrder{}, false
	}
	return game.ReliefOrder{At: prefecture, Gold: budget}, true
}

// rewardGold 是「賞賜金帛」（表 `0x5654`，常式 `0xd302`，`L0`、`[base]`）。
//
// 走守軍清單，**跳過身分 0（君主自己）**，每一位賞 `min(剩下的預算, 100)`
// ——說明書 p.23 的賞金上限 100 就是常式裡的 `cmp ax, 100`。

// buyRice 是「買入米糧」（表 `0x55d4`，常式 `0xc634`，`L0`、`[base]`）。
//
//	量   ＝ (100 − 物價) ÷ 10                      ; 一金買到幾單位
//	目標 ＝ min(兵士（百）× (RND(10) + 12), 30000)  ; 存糧跟著兵力走
//	缺口 ＝ 目標 − 米
//	剩金 ＝ clamp(金 − 缺口 ÷ 量, 0, 30000)
//	買到 ＝ (金 − 剩金) × 量
//
// **下限是 0**——缺口夠大就把郡的金全部花光（`ds:[0xa5f2]` 讀出來是 0）。
// 這一支不經過折扣常式 `0xec24`，所以電腦諸侯買米沒有折扣。
func buyRice(g *game.State, prefecture int, id state.FactionID, purse int) (game.RiceTradeOrder, bool) {
	p := g.Prefecture(prefecture)
	if p == nil {
		return game.RiceTradeOrder{}, false
	}
	troops := 0
	for _, x := range g.Garrison(prefecture) {
		troops += x.Soldiers
	}
	// **先擲再看錢。** 同賑民：原版每郡都抽一次（32／32）。
	want := troops / 100 * (g.Roll(10, int(id), prefecture, tableRice) + 12)
	if want > game.MaxRice {
		want = game.MaxRice
	}
	// **不擋方向也不擋錢包**：這一支是雙向的，存糧高於目標就賣
	// （`game.TradeRiceTo`）。先前擋掉 `缺口 <= 0`，等於把賣米整個拿掉
	// ——月度對拍量到郡 1 的米因此多出 116 單位、金少 14，而出兵那一道
	// 比的正是米，於是連出兵的判定都跟著翻面。
	return game.RiceTradeOrder{At: prefecture, Target: want}, true
}

// headhunt 是「挖角」（表 `0x56d4`，`L0`、`[base]`）。
//
//	等級 0–2 不做（三格空操作）
//	君主的所在郡 != 本郡 → 不做
//	RND(10) <= K → 不做            ; K ＝ 6／3／1，也就是 30 % / 60 % / 80 %
//	本回合的錢 < 100 → 不做
//	掃全部人物挑候選（`game.Headhunt` 的 `headhuntable`），取第一位
//
// 費用 100 金是**直接扣的**，不經過等級折扣。
func headhunt(g *game.State, prefecture int, id state.FactionID, gold int) (game.HeadhuntOrder, bool) {
	level := g.AILevel(id)
	if level < 3 {
		return game.HeadhuntOrder{}, false
	}
	// **挖角的預算按季節開關**（係數表 `DS:0x5714`，`L0`）：某些
	// （等級, 季節）組合給 0%，那個季節就挖不了角。等級越高開放的
	// 季節越多。
	//
	// 門檻比的是**算出來的本回合預算**（`0xe438`：`es:[0x3d16] < 100`
	// 就回），而預算是「係數 × 郡的金 × 0.01」——拿郡的金直接比會
	// 讓它過得太寬（量到挖角多觸發四次、多抽 920 次）。
	// **索引是 `月 mod 4`**（`0xe9a9` 的 `idiv 4`），不是季節事件那個
	// 1／4／7／10 的季（`docs/mechanics/70-ai` §2.14）。兩者不對齊。
	// **順序照原版**：君主在不在（`0xe414`）→ `RND(10) > 門檻`
	// （`0xe427`）→ 本回合預算 >= 100（`0xe438`）。擲骰夾在中間，
	// 所以預算不夠的那幾次**還是抽過了**——先看預算會少抽一批。
	if lord := g.Lord(id); lord == nil || lord.Location != prefecture {
		return game.HeadhuntOrder{}, false
	}
	if g.Roll(10, int(id), prefecture, tableHeadhunt) <= headhuntBar(level) {
		return game.HeadhuntOrder{}, false
	}
	budget := gold * HeadhuntBudget(level, g.Date.Month%4) / 100
	if budget < game.CostHeadhunt {
		return game.HeadhuntOrder{}, false
	}
	for _, x := range g.AllGenerals() {
		if !x.Employed() || x.Faction == id || x.Status == state.StatusLord {
			continue
		}
		if g.Headhuntable(x, prefecture) {
			return game.HeadhuntOrder{At: prefecture, Target: x.Index}, true
		}
	}
	return game.HeadhuntOrder{}, false
}

// headhuntBar 是 `RND(10) > K` 裡的 K：等級 3／4／5 ＝ 6／3／1。
// HeadhuntBudget 是挖角這一季的預算百分比（係數表 `DS:0x5714`，`L0`）。
//
// **索引是「月 mod 4」**（分派器 `0xe9a9` 的 `idiv 4`），不是季節事件
// 那個 1／4／7／10 的季——兩者不對齊。0 表示這個相位不挖角：
// 等級 0–2 一律 0（那三格是空操作），等級 3 只有相位 3、
// 等級 4 是相位 1 與 3、等級 5 除了相位 0 都可以。
//
// **不是「機率低」是「完全不做」**——係數 0 算出來的預算是 0，
// 而挖角要 100 金。
func HeadhuntBudget(level, phase int) int {
	if level < 3 || phase < 0 || phase > 3 {
		return 0
	}
	open := map[int][4]bool{
		3: {false, false, false, true},
		4: {false, true, false, true},
		5: {false, true, true, true},
	}[level]
	if !open[phase] {
		return 0
	}
	return 20
}

func headhuntBar(level int) int {
	switch level {
	case 3:
		return 6
	case 4:
		return 3
	}
	return 1
}

// plot 是「計略」（表 `0x56f4`，`L0`、`[base]`）。
//
//	等級 0–2 不做（三格空操作）
//	RND(10 / 8 / 5) != 0 → 不做      ; 等級 3／4／5 ＝ 10 % / 12.5 % / 20 %
//	沒有軍師、或軍師不在這個郡 → 不做
//	掃全部 42 個郡，取「有主而且不是自己」的，隨機挑一個
//	使者 ＝ 本郡守軍裡魅力最高的
//
// **電腦諸侯只會用偽書使疑**（`0x0e79e` 呼叫 `0x2d1fa`）：五種計謀裡
// 只有這一種接在電腦的決策表底下，效果是壓低目標郡武將的忠誠。
// 成敗與效果在 `game.PlotScore`／`game.Forgery`。
func plot(g *game.State, prefecture int, id state.FactionID) (game.PlotOrder, bool) {
	level := g.AILevel(id)
	if level < 3 {
		return game.PlotOrder{}, false
	}
	// **先擲再看軍師在不在。** 原版這一次亂數是無條件抽的——月度對拍
	// 量到計略這一支剛好每郡一次（32／32），而 remake 先擋軍師只抽了 7 次。
	// 與尋訪、出兵、賞賜物品同一個形狀：做不做得成不影響抽不抽。
	if g.Roll(plotRange(level), int(id), prefecture, tablePlot) != 0 {
		return game.PlotOrder{}, false
	}
	chief := g.Chief(id)
	if chief == nil || chief.Location != prefecture {
		return game.PlotOrder{}, false
	}
	envoy := mostCharmingHere(g, id, prefecture)
	if envoy == nil {
		return game.PlotOrder{}, false
	}
	var targets []int
	for _, q := range g.Prefectures() {
		if q.Owned() && q.Owner != id {
			targets = append(targets, q.ID)
		}
	}
	if len(targets) == 0 {
		return game.PlotOrder{}, false
	}
	to := targets[g.Roll(len(targets), int(id), prefecture, tablePlot, 1)]
	return game.PlotOrder{
		At: prefecture, To: to, What: game.PlotForgery, Envoy: envoy.Index,
	}, true
}

// plotRange 是 `RND(n) == 0` 裡的 n：等級 3／4／5 ＝ 10／8／5。
func plotRange(level int) int {
	switch level {
	case 3:
		return 10
	case 4:
		return 8
	}
	return 5
}

// mostCharmingHere 是本郡守軍裡魅力最高的一位（**不看君主在不在**，
// 與 `mostCharming` 那個「指定太守」用的不同）。
func mostCharmingHere(g *game.State, id state.FactionID, prefecture int) *game.General {
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

// garrisonIndices 是這一郡守軍的槽號，照清單順序。
func garrisonIndices(g *game.State, prefecture int) []int {
	var out []int
	for _, x := range g.Garrison(prefecture) {
		out = append(out, x.Index)
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
func armsPurchase(g *game.State, prefecture, budget int) []game.Order {
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
// 四種寶物各跑一次（諸侯 offset 15／16／17／18），每一次：
//
//	RND(100) > 40 → 跳過          ; 41 % 才進行
//	存量 <= RND(2) + 2 → 跳過      ; 手上要夠多才送得出去
//	按「該寶物要提升的能力 ＋ 加權表[身分]」排序，挑第一個過門檻的
//	該人能力與忠誠上升，寶物存量 −1
func (f *faithful) rewards(g *game.State, id state.FactionID, prefecture int,
	emit func(game.Order), mark func(string)) {
	if g.AILevel(id) < 3 {
		return // 等級 0–2 那三格是空操作
	}
	fa := g.Faction(id)
	if fa == nil {
		return
	}
	// 四支各一次，形狀相同（`0xe03c` 等級 5、`0xdfcc` 等級 4、
	// `0xdf5c` 等級 3）：
	//
	//	r = RND(2) + 2
	//	0xd962(門檻, r)          ; 門檻 40／60／80，隨等級
	//
	// 常式裡（`0xd962`）再抽一次 `RND(100)`，**門檻就是那個常數**：
	//
	//	RND(100) > 門檻 → 回                 ; 0xd97a，40／60／80
	//	諸侯[所屬] 的庫存 <= r → 回          ; 0xd9b4
	//	掃名單挑人（迴圈裡每個都抽 RND(20)）  ; 0xd9f2
	//
	// **兩次抽樣的順序是「呼叫端的 `RND(2)` 在前、常式裡的 `RND(100)`
	// 在後」**，而且兩次都一定會抽。remake 原本只抽一次、門檻寫死 40，
	// 而且順序相反。
	bar := treasureBar(g.AILevel(id))
	// 諸侯 offset 15–18 那四格會被送出去；14 是玉璽（不能送人）。
	for i, t := range []game.Treasure{
		game.TreasureBook, game.TreasureBlade,
		game.TreasureBeauty, game.TreasureHorse,
	} {
		r := g.Roll(2, int(id), prefecture, i) + 2
		if g.Roll(100, int(id), prefecture, i, 0x56b4) > bar {
			mark(treasureBucket[i])
			continue
		}
		if fa.Treasury[t] <= r {
			mark(treasureBucket[i])
			continue
		}
		who := f.rewardTarget(g, id, prefecture, t, i)
		if who == nil {
			mark(treasureBucket[i])
			continue
		}
		// **就地發下去**：原版是常式自己改人物表（`0xda36` 起的
		// `RND(2)+2` 與 `RND(30)`），那兩次抽樣屬於這一支。
		emit(game.GiftOrder{
			At: prefecture, Target: who.Index, What: t, Auto: true})
		mark(treasureBucket[i])
	}
}


// rewardTarget 是賞賜的對象（`0xd9bc`／`0xdb1a`／`0xdc78`／`0xddee`，`L0`）。
//
// 四支各自呼叫**不同的排序常式**，鍵是「那件寶物要提升的能力 ＋
// 加權表[身分]」——加權表就是 `actorWeight` 那一張（`DS:0x5986`）：
//
//	兵書 → 謀略（0xf360）    寶刀 → 戰力（0xf440）
//	美女 → 魅力（0xf520）    駿馬 → 戰力（0xf440）
//
// 排完之後從頭找第一個「該能力 > `RND(20) + 60`，**或者是君主**」而且
// 該能力 < 90 的人。**君主一律跳過能力門檻**，而且君主本來就排在最前面
// （權重 2000 壓過任何能力值），所以只要他那一項還沒滿 90，寶物就是他的。
//
// ⚠ 門檻是**逐人重擲**的：`RND` 的呼叫點（`0xd9f2`）在迴圈裡面。
// treasureBar 是賞賜物品傳進 `0xd962` 的那個常數，隨等級變（`L0`）：
// 等級 3 是 `0x28`（40，`0xdf72`）、4 是 `0x3c`（60，`0xdfe2`）、
// 5 是 `0x50`（80，`0xe052`）。等級 0–2 那三格是空操作。
// treasureFloor 是候選門檻 `RND(20) + 底` 裡的底（`L0`、`[base]`）。
// 駿馬那一支是 70，其餘三支是 60。
func treasureFloor(t game.Treasure) int {
	if t == game.TreasureHorse {
		return 70
	}
	return 60
}

// treasureBucket 對上原版四支常式的位址，逐種分帳用。
var treasureBucket = [4]string{
	"賞賜物品：兵書", "賞賜物品：寶刀", "賞賜物品：美女", "賞賜物品：駿馬",
}

func treasureBar(level int) int {
	switch {
	case level >= 5:
		return 80
	case level == 4:
		return 60
	default:
		return 40
	}
}

func (f *faithful) rewardTarget(g *game.State, id state.FactionID, prefecture int,
	t game.Treasure, salt int) *game.General {
	ability := func(x *game.General) int {
		switch t {
		case game.TreasureBook:
			return int(x.Intel)
		case game.TreasureBeauty:
			return int(x.Charm)
		default: // 寶刀與駿馬都看戰力
			return int(x.War)
		}
	}
	weight := func(x *game.General) int {
		if int(x.Status) < len(actorWeight) {
			return actorWeight[x.Status]
		}
		return 0
	}
	// **接在排過一輪的名單後面**：原版的清單由 `0xec86` 建好、行動者那
	// 一張（`智 + 武 + 加權`）先排過，四支賞賜常式再按自己的鍵排一次
	// （`0xf360`／`0xf440`／`0xf520`）。交換排序不穩定，所以**進去的
	// 順序會影響同鍵的結果**——從槽號順序開始排會挑到另一個人
	// （郡 12 的兩位都是智 79，郡 30 有兩處同樣）。
	//
	// **不比對勢力**：名單是 `buildRoster` 模式 2 建的，混編的郡裡別的
	// 勢力的人也在裡面。
	list := roster(g, prefecture)
	// **交換排序**，和行動者那一張同一支（`0xf360`／`0xf440`／`0xf520`，
	// 鍵是「該寶物要提升的能力 ＋ 加權表[身分]」）。**不能用穩定排序**
	// ——內層一比到更大的就當場對調，同鍵的其餘元素會被打亂，而收禮的
	// 是名單裡第一個過門檻的人。月度對拍量到郡 12 的兩位都是智 79，
	// 原版給後面那位、穩定排序給前面那位（郡 30 兩處同樣）。
	key := func(x *game.General) int { return ability(x) + weight(x) }
	for i := range list {
		for j := i + 1; j < len(list); j++ {
			if key(list[j]) > key(list[i]) {
				list[i], list[j] = list[j], list[i]
			}
		}
	}
	for i, x := range list {
		// **`RND(20)` 在迴圈頂端，每一筆都抽**（`0xd9f2`）——包括能力
		// 已經到頂、以及君主那種不必比的。寫成短路條件會把那些抽樣吃掉
		// （Go 的 `&&`／`||` 都短路），整條序列跟著錯開。
		// **門檻的底四支不一樣**（`L0`）：兵書 `0xd9fa`、寶刀 `0xdb58`、
		// 美女 `0xdcb6` 都是 `add $0x3c`（60），駿馬 `0xde2c` 是
		// `add $0x46`（**70**）。寬度四支都是 `RND(20)`。
		// 駿馬那一支因此挑剔得多——武力 70 以下的人幾乎拿不到。
		r := g.Roll(20, int(id), prefecture, salt, i, 0x56b4) + treasureFloor(t)
		if v := ability(x); v < game.TreasureCap &&
			(x.Status == state.StatusLord || v > r) {
			return x
		}
	}
	return nil
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
// 「最後一位」跟著清單順序走，而**清單順序就是行動者那一張表排完的
// 順序**（`roster`）：`智 + 武 + 加權表[身分]` 由大到小。
//
// **君主不必在場**（`0xd7ae` 整支常式沒有這個檢查——它從州郡 offset 30
// 取所屬，直接改諸侯 offset 6），**候選也不比對勢力**（走的是指定太守
// 那一份名單，`docs/re/07` §6）。兩件都走 `AppointChiefOrder.Auto`。
func betterChief(g *game.State, id state.FactionID, prefecture int) *game.General {
	floor := state.ChiefIntelFloor
	if cur := g.Chief(id); cur != nil {
		floor = int(cur.Intel)
	}
	var pick *game.General
	for _, x := range roster(g, prefecture) {
		if int(x.Intel) <= floor {
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
// actor 是行動者：**排序後名單的第一位**（`0x54d4` → `0xf170`）。
//
// **不比對勢力**——建表的 `buildRoster(郡, 模式 2)` 只看「所在郡相同、
// 身分 ≤ 3」（`docs/re/07` §6）。混編的郡因此可能由別的勢力的人出面，
// 而原版就是這樣：月度對拍量到郡 14（勢力 5）的行動者是 124，
// 而州郡 offset 32 記著的是 91——兩個人的智差 1，`(智 − 底) ÷ 12` 一個
// 是 1 一個是 0，開墾那一支因此差一次亂數（`0xba02` 只有量非正才擲）。
func actor(g *game.State, id state.FactionID, prefecture int) *game.General {
	if list := roster(g, prefecture); len(list) > 0 {
		return list[0]
	}
	return nil
}

// smartest 是名單裡**智最高**的一位（`0xec86` 的 `es:[0x4196]`，`L0`）。
//
// 分派器在跑十八張表之前先呼叫 `0xec86(郡)`：建表 → 按
// `智 + 武 + 加權表[身分]` 排序 → 然後**從第一位開始逐一比**，
// 挑出三個人存進三個全域：
//
//	es:0x4196 ← 智最高（人物 offset 9）   ; 0xed1e —— 內政、尋訪讀它
//	es:0x3c94 ← 武最高（offset 10）       ; 0xed66
//	es:0x20ee ← 魅最高（offset 11）       ; 0xedae
//
// 三個都是「嚴格大於才換」，所以並列時**排序後排在前面的留下**——
// 排序的順序因此仍然有意義。
//
// ⚠ **不是行動者**：行動者（名單第一位）是 `智 + 武 + 加權` 的最大，
// 加權讓太守壓過武將；智最高的可以是另一個人。月度對拍量到郡 14
// 兩者相差一人（太守 91 智 61、武將 124 智 62），而開墾的量
// `(智 − 底) ÷ 12` 一個是 0 一個是 1——差一次亂數。
func smartest(g *game.State, prefecture int) *game.General {
	list := roster(g, prefecture)
	if len(list) == 0 {
		return nil
	}
	best := list[0]
	for _, x := range list[1:] {
		if x.Intel > best.Intel {
			best = x
		}
	}
	return best
}

// actorKey 是行動者那一張表的排序鍵：`智 + 武 + 加權表[身分]`。
func actorKey(x *game.General) int {
	w := 0
	if int(x.Status) < len(actorWeight) {
		w = actorWeight[x.Status]
	}
	return int(x.Intel) + int(x.War) + w
}

// roster 是分派器手上那一份名單（`es:[0x58c]`），**照原版的順序**。
//
// 建表的是 `buildRoster(郡, 模式 2)`：所在郡相同、身分 ≤ 3，**不比對
// 勢力**（`docs/re/07` §6，28 次量過集合相等）。接著**行動者那一張表
// （`0x54d4`）就地按 `智 + 武 + 加權表[身分]` 由大到小排**，後面的表
// 看到的就是排完的順序——`0xd652`（指定太守）與 `0xd7ae`（指定軍師）
// 都不自己建表。
//
// **順序會改變結果**：指定太守取「第一個最大魅力」（`0xf600` 的交換
// 條件是嚴格大於，並列時排在前面的留下），指定軍師取「最後一位合格
// 者」。加權讓身分 2（太守，1200）排在身分 3（武將，800）前面，所以
// 魅力並列時現任太守本來就站在前面，原版因此常常什麼都不換——照槽號
// 掃會在每個並列的郡都換一次人（月度對拍量到 5 個郡）。
func roster(g *game.State, prefecture int) []*game.General {
	out := append([]*game.General(nil), g.Garrison(prefecture)...)
	// **交換排序**（`0xf170`，`L0`）：內層一比到更大的就**當場對調**，
	// 不是記下最大值再換一次。位置 0 因此落在「第一個最大」上，而同鍵
	// 的其餘元素會被交換打亂——換成穩定排序會有差（`docs/playtest/02`
	// 量到 25 次裡有 3 次差在相鄰一對）。
	for i := range out {
		for j := i + 1; j < len(out); j++ {
			if actorKey(out[j]) > actorKey(out[i]) {
				out[i], out[j] = out[j], out[i]
			}
		}
	}
	return out
}

// mostCharming 是守軍裡魅力最高的一位。
//
// 原版在指定太守之前先把守軍清單**按魅力由高到低排序**
// （`0xf600` 起的交換排序，比的是人物 offset 11），然後取第一位。
// 說明書只說「太守魅力越高，登用與賑民的效果越好」——這裡是 AI 實際
// 用的判準。
// mostCharming 是指定太守要指的那一位（`0xd652`，`L0`＋`L1`）。
//
// **名單不比對勢力**（28 次量過）：混編的郡裡站著別的勢力的武將時，
// 他也在候選之列——原版的主事者本來就可能是外人（月度對拍量到郡 13 的
// 主事者是荀彧，勢力 5，站在勢力 4 的郡裡）。挑的是清單順序裡**第一個
// 最大魅力**的人（25 次逐次相同）。
//
// ⚠ 「君主在的郡不指太守」是 remake 這邊的權宜：原版判的是**州郡 offset 32
// （主事者）的身分是不是 0**（`CONTEXT.md` R28）。照原版改過一輪量出來
// 更差，成因另在別處，先留著。
func mostCharming(g *game.State, id state.FactionID, prefecture int) *game.General {
	if lord := g.Lord(id); lord != nil && lord.Location == prefecture {
		return nil
	}
	var best *game.General
	for _, x := range roster(g, prefecture) {
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
func internalAffairsRange(level int) int { return game.AffairsTierFor(level).Chance }
