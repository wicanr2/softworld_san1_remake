// Package game 是規則層：一局進行中的遊戲，以及改變它的規則。
//
// **這一層不依賴 Ebiten 也不依賴檔案格式**。輸入是 `internal/state`
// 解出來的劇本，輸出是可以被測試逐項檢查的狀態轉移。
//
// 規則的來源分兩種，每一條都標出處：
//
//   - `說明書` —— 手冊寫明的數字（`docs/reference/01-manual-20-mechanics.md`）。
//     手冊是繁中原文的權威來源，但**它寫的是玩家看得到的規則，不是實作**；
//     沒有第二個來源佐證的數字只當假設。
//   - `對拍` —— 用 dosgolem 跑原版量到的。
//
// ⚠ **沒有出處的數字不要寫進這一層。** 猜出來的公式會自洽、會通過測試、
// 而且玩起來「差不多」——那是最難發現的錯（`CLAUDE.md` §7 第 18 條）。
package game

import (
	"math/big"

	"github.com/wicanr2/softworld_san1_remake/internal/state"
)

// 資源上限（說明書 p.22）。
const (
	MaxGold = 30000
	MaxRice = 30000
)

// MinPopulationToConscript 是徵兵需要的人口下限（說明書 p.20、p.37）：
// 「少於 3000 人就無法徵兵」。
const MinPopulationToConscript = 3000

// MaxForts 是一個郡最多幾座城寨（說明書 p.21，不含城池）。
const MaxForts = 5

// MinIntelForChief 是受封軍師的謀略下限（說明書 p.23）。
const MinIntelForChief = 80

// 各項花費，單位是金（說明書 p.20–34）。
const (
	CostConscriptPerSoldier = 1   // 徵兵，每人 1 金
	CostArmsPer100          = 1   // 武器，每 100 單位 1 金
	CostReclaim             = 10  // 開墾
	CostFloodControl        = 10  // 防洪
	CostSearch              = 5   // 尋訪
	CostRecruit             = 30  // 登用
	CostDismiss             = 10  // 撤職
	CostHeadhunt            = 100 // 挖角
	CostScoutEnemy          = 10  // 查看敵軍
	MaxReward               = 100 // 賞賜上限
)

// FortCost 是建一座城寨的花費：當月物價的 100 倍（說明書 p.21）。
func FortCost(priceLevel uint8) int { return int(priceLevel) * 100 }

// TroopShare 是調整兵力時一個人分到的兵（`L1`、`[base]`、`0xc4c3`）。
//
//	份額 ＝ min(trunc(帶兵上限 × (總兵力 ÷ 總上限) ＋ 0.5), 帶兵上限)
//
// ⚠ **不能寫成整數的四捨五入**（`(總兵力 × 上限 × 2 ÷ 總上限 ＋ 1) ÷ 2`）。
// 原版先把「總兵力 ÷ 總上限」存成一個 double，再用 80 位元的浮點乘回去；
// 那個商比真值小一點點，所以**剛好落在 .5 的案例會少一格**：
// 上限 2000、總兵 12565、總上限 28000 的真值是 897.5，原版給 897。
// 整數的四捨五入給 898，279 個樣本裡就是這 4 個對不上。
//
// 這裡用精確有理數重現：先取「商」那一步的 double 捨入，再精確算乘法。
func TroopShare(cap, total, capSum int) int {
	if capSum <= 0 || cap <= 0 {
		return 0
	}
	r := new(big.Rat).SetFloat64(float64(total) / float64(capSum))
	if r == nil {
		return 0
	}
	r.Mul(r, new(big.Rat).SetInt64(int64(cap)))
	r.Add(r, big.NewRat(1, 2))
	n := int(new(big.Int).Quo(r.Num(), r.Denom()).Int64())
	if n > cap {
		n = cap
	}
	return n
}

// TroopCap 是一個職位帶得動的最大兵力（`L0`、`[base]`）。
//
// 原版的表在 `es:[0x666e]`，**職位 × 2 索引**；徵兵（`0xbeb8`）與
// 調整兵力（`0xc2c4`）都拿它當上限。從執行期記憶體讀出來的九項是
// 5000／3000／2500／2000／1500／3000／2500／2000／1500，與下面
// **逐格相同**（`docs/mechanics/70-ai` §2.13.2）。
//
// 說明書 p.18 的「官階與帶兵上限」給的是同一組數字，劇本資料也對得上
// ——346 位人物零人超標，九個職位裡有八個的實際最大兵數正好等於上限
// （謀士那一格劇本裡最高只有 200，沒有頂到）。三條路各自獨立。
//
// 職位的編號是原版字串表的順序，文官與武官成對：
func TroopCap(r state.Rank) int {
	switch r {
	case state.RankLord:
		return 5000
	case state.RankStrategist, state.RankGeneral:
		return 3000
	case state.RankStaff, state.RankViceGeneral:
		return 2500
	case state.RankClerk, state.RankSubGeneral:
		return 2000
	case state.RankAdvisor, state.RankJuniorGeneral:
		return 1500
	}
	return 0
}

// TrainGain 是「訓練兵士」一次提升多少訓練度（`L0`、`[base]`）。
//
// 從原版的碼讀出來的。共用常式在線性 `0xbd70`，對清單裡的每一位算：
//
//	mov al, es:[bx+0x2219]   ; 智
//	idiv 3
//	mov al, es:[bx+0x221a]   ; 武
//	idiv 2
//	add                      ; 智/3 + 武/2
//	idiv word [bp+6]         ; 除以呼叫端給的常數
//	add 原本的訓練度
//	上限 100
//
// **逐項截斷**：智與武各自先整數除，再相加，再除。
//
// 那個常數由勢力的 AI 等級選（`AITrainDivisor`），不是固定的
// （`docs/re/03` §1.3–1.4）。說明書只說「各將的能力影響其麾下的訓練度
// 提升」——方向對，係數是碼裡才有的。
func TrainGain(intel, war, aiLevel int) int {
	return (intel/3 + war/2) / AITrainDivisor(aiLevel)
}

// AITrainDivisor 是訓練提升的除數，由勢力的 AI 等級選（`L0`、`[base]`）。
//
// 原版的指令分派表 `0x5554` 有八個 far pointer，每一個是一個 thunk，
// 推一個常數再呼叫共用常式：
//
//	等級 0 1 2 → 5
//	等級 3 4   → 4
//	等級 5     → 3
//	等級 6 7   → 空操作（`xor ax,ax; lret`）
//
// **等級 6 與 7 到不了**：分派前 `[bp+6]` 被夾在 0–5，那兩格只是把表
// 補成 2 的冪。所以除數越小（等級越高）練得越快。
func AITrainDivisor(level int) int {
	switch {
	case level <= 2:
		return 5
	case level <= 4:
		return 4
	default:
		return 3
	}
}

// 內政兩項的量：**玩家與電腦走的是不同的常式**（`L0`、`[base]`）。
//
// 玩家選單那一條在線性 `0x1a6d2`（土地開墾）與 `0x1a93a`（洪水防治），
// 電腦那一條在分派表 `0x5534` 底下的六支（`0xba9c` 起，每支 0x76 bytes），
// 兩邊共用寫回州郡的小常式 `0xba02`／`0xba4c`。
//
// 差別不只係數：
//
//   - 玩家：`max(謀略 − 50, 0) ÷ 12`——謀略不足 50 就是**加 0**
//   - 電腦：`(謀略 − 底) ÷ 12`，算出來 ≤ 0 時改擲 `RND(2)`（0 或 1）
//
// 也就是說**蠢將替電腦開墾偶爾還有 1 點，替玩家開墾就是白做**。
//
// ReclaimGain 是玩家那一條。
func ReclaimGain(intel int) int {
	n := intel - ReclaimIntelFloor
	if n < 0 {
		n = 0
	}
	return n / ReclaimIntelDiv
}

// AIReclaimGain 是電腦那一條：底隨 AI 等級變，非正就改用擲的。
func AIReclaimGain(intel, floor, roll int) int {
	if n := (intel - floor) / ReclaimIntelDiv; n > 0 {
		return n
	}
	return roll // RND(2)：0 或 1
}

// ReclaimIntelFloor／ReclaimIntelDiv 是玩家那一條的兩個常數
// （`0x1a73b` 的 `sub $0x32`、`0x1a74d` 的 `idiv 12`）。
// 除數 12 六個 AI 等級也都一樣，只有底會變。
const (
	ReclaimIntelFloor = 50
	ReclaimIntelDiv   = 12
)

// FloodDrop 是「洪水防治」一次降多少洪水率（`L0`、`[base]`）。
//
// 寫回的常式在線性 `0xba4c`：讀州郡 offset 28，減掉呼叫端給的量，
// **負的夾到 0**，寫回去。玩家那一條給的量是 `謀略 ÷ 10`（`0x1a94b`），
// 電腦那一條的除數隨 AI 等級變（`AffairsTier`）。
func FloodDrop(intel int) int { return intel / FloodIntelDiv }

// AIFloodDrop 是電腦那一條。
func AIFloodDrop(intel, div int) int {
	if div < 1 {
		div = 1
	}
	return intel / div
}

// FloodIntelDiv 是玩家那一條的除數。
const FloodIntelDiv = 10

// AffairsTier 是電腦內政那張表六個等級各自的三個常數（`L0`、`[base]`）。
//
// 六支常式是同一段碼，只有三個立即數不同（逐位元組 diff 過）：
//
//	等級  RND(K)  開墾的底  防洪的除數
//	  0      4       50         10
//	  1      4       60         15
//	  2      4       60         15
//	  3      3       50         14
//	  4      3       40         12
//	  5      2       50         10
//
// **這不是一條由弱到強的階梯**：K 越小動手越勤（等級 5 從不閒著），
// 但等級 1 與 2 的開墾底比等級 0 還高、防洪除數也更大，做起來反而比
// 等級 0 差。等級 4 的開墾最狠（底 40），等級 5 勤但量普通。
type AffairsTier struct {
	Chance    int // RND(K)：擲 0 開墾、1 防洪，其餘不做
	LandFloor int // 開墾：(謀略 − 這個) ÷ 12
	FloodDiv  int // 防洪：謀略 ÷ 這個
}

var affairsTiers = [6]AffairsTier{
	{4, 50, 10},
	{4, 60, 15},
	{4, 60, 15},
	{3, 50, 14},
	{3, 40, 12},
	{2, 50, 10},
}

// AffairsTierFor 取某個 AI 等級的那一組；越界夾住。
func AffairsTierFor(level int) AffairsTier {
	if level < 0 {
		level = 0
	}
	if level >= len(affairsTiers) {
		level = len(affairsTiers) - 1
	}
	return affairsTiers[level]
}

// SearchTier 是尋訪人才的三個常數（`L1`、`[base]`）。
//
// 六支分派常式在 `0xcd20` 起（間隔 `0x3a`），每一支的形狀相同而**三個
// 立即數都隨等級變**：
//
//	RND(10) <= Bar → 這回合不做
//	門檻 ＝ RND(Spread) ＋ Floor
//	尋訪者的謀略 > 門檻 → 找到一位身分 9 的人，把他改成身分 8
//
// | 等級 | 0 | 1 | 2 | 3 | 4 | 5 |
// |---|---|---|---|---|---|---|
// | Bar | 7 | 7 | 7 | 6 | 5 | 4 |
// | Spread | 65 | 65 | 65 | 45 | 20 | 20 |
// | Floor | 30 | 30 | 30 | 30 | 30 | 15 |
//
// 等級 0 的門檻是 30–94（謀略 95 才穩），等級 5 是 15–34——**高等級的
// 電腦幾乎每次尋訪都會找到人**，而且出手的機率從 20 % 升到 50 %。
//
// 說明書只說「負責尋訪的將領謀略越高，成功的機率越大」。
type SearchTier struct{ Bar, Spread, Floor int }

var searchTiers = [6]SearchTier{
	{7, 65, 30}, {7, 65, 30}, {7, 65, 30},
	{6, 45, 30}, {5, 20, 30}, {4, 20, 15},
}

// SearchTierFor 取一個 AI 等級的尋訪常數，越界夾住。
func SearchTierFor(level int) SearchTier {
	if level < 0 {
		level = 0
	}
	if level >= len(searchTiers) {
		level = len(searchTiers) - 1
	}
	return searchTiers[level]
}

// Weapons／ArmsOf 是武裝度與武器數之間的換算。
//
// **武裝度是「有武器的兵佔多少百分比」**，不是武器的絕對數量。
// 原版把它存成百分比，實際運算時先還原成武器數、加減、再換回百分比——
// 所以兵力一變，武裝度就跟著動。說明書「人員損耗後，其持有的軍械也隨同失去」
// 講的就是這件事。
// ArmsPerGold 是一金買得到幾單位武器（說明書 p.20；原版 `0xc168`
// 用 `idiv 100` 把缺口換成金額，`L0`）。
const ArmsPerGold = 100

func Weapons(arms, soldiers int) int { return arms * soldiers / 100 }

// WeaponsF32 是**徵兵那條路**的武器數（`L0`、`0xbeb8` 與 `0x19b99`）。
//
// 原版乘的是 **float32 的 0.01**（0.009999999776…），比 1/100 小一點點，
// 所以整除的時候會少一件：兵 20 × 武裝度 100 算出來是 **19** 件不是 20 件。
// 買武器那一支（`0xc168`）乘的是 float64 的 0.01，整除不會少——
// **兩條路的常數不同，不要合成一支**。
//
// 位址：稀釋是 `fmuls DS:0xa5d0`（電腦）／`fmuls DS:0xa7c2`（玩家），
// 兩個位址存的都是 `0.009999999776482582`；買武器是
// `fmull DS:0xa5c8` ＝ `0.01`（float64）。
func WeaponsF32(arms, soldiers int) int {
	n := arms * soldiers
	if n > 0 && n%100 == 0 {
		return n/100 - 1
	}
	return n / 100
}

// DiluteAfterRecruit 是徵兵之後訓練度與武裝度的稀釋（`L0`、`L1`、`[base]`）。
//
//	新值 ＝ trunc(WeaponsF32(舊值, 舊兵力) × 100 ÷ 新兵力)
//
// 新兵沒受訓也沒武器，所以兩個欄位走同一條加權平均——原版的碼也是
// 同一段跑兩次（`0xbfdd`–`0xc057`）。
func DiluteAfterRecruit(v, oldMen, newMen int) int {
	if newMen <= 0 {
		return 0
	}
	return WeaponsF32(v, oldMen) * 100 / newMen
}

func ArmsOf(weapons, soldiers int) int {
	if soldiers <= 0 {
		return 0
	}
	return clampTo(weapons*100/soldiers, 100)
}

// ArmsAfterPurchase 是買武器之後的武裝度（`L0`、`[base]`）。
//
// 原版的常式（線性 `0xc168`）用浮點算，而 MSC 的浮點模擬器把 x87 指令
// 編碼成 `INT 34h`–`3Bh` ＋ 原本的 modrm（對應 `D8`–`DF`，`3Ch` 是帶段
// 前綴的形式）。還原出來的序列是
//
//	fild 武裝度 ; fild 兵力 ; fst qword [bp-12]
//	fmulp                      ; 武裝度 × 兵力
//	fmul qword ds:[0xa5c8]     ; × 0.01
//	fiadd word [bp-2]          ; + 新增的武器
//	fdiv qword [bp-12]         ; ÷ 兵力
//	fmul qword ds:[0xa5a8]     ; × 100
//
// 兩個常數從執行期記憶體讀出來是 `0.01` 與 `100`（`TestZZDispatch`），
// 所以整段就是「換成武器數 → 加上新買的 → 換回百分比」。
// 兵力為零時武裝度歸零，那是同一支常式開頭的 `cmpw es:[bx+0x2226], 0`。
//
// ⚠ **原版的「兵力」與「新增的武器」各自的單位還沒量**（`L3`）：
// 這裡照 remake 自己的單位算，兩邊的比例對得上才有意義。
//
// **對拍過**（`L1`，505 次逐次相同，`docs/playtest/02`），但兩處寫法
// 與原版不同，而取樣到的範圍剛好分不出來：
//
//   - 原版把缺口加到**還沒截斷**的乘積上（`fiadds` 之前沒有轉整數），
//     這裡先 `Weapons` 截斷再加。兩者只在**兵力 < 100** 時差 1
//     （原版給 101，這裡夾成 100），而電腦諸侯的部隊都是幾千人。
//   - 原版**沒有夾 100 的上限**，寫回去的是低位那個 byte；買的量超過
//     缺口時原版會寫出大於 100 的武裝度。電腦一律只補到滿編，
//     所以這條路它走不到。
//
// 兩件事都是從碼讀出來的（`L0`），但**沒有實跑背書**——改之前要先讀
// 玩家那一支買武器的常式，確認它有沒有把數量夾在缺口以內。
func ArmsAfterPurchase(arms, soldiers, bought int) int {
	return ArmsOf(Weapons(arms, soldiers)+bought, soldiers)
}

// 登用人才的判定（`L0`、`[base]`，常式 `0xce8c`）。
//
// 原版把它拆成「說服力」與「難度」兩個數，說服力大於難度才成功：
//
//	說服力 ＝ (人望 × 3 ＋ 太守魅力) ÷ 3 ＋ 加成
//	難度   ＝ 謀略 ÷ (RND(4)+3) ＋ 戰力 ÷ (RND(4)+3)
//
// 難度前面還有一道**牽絆閘門**，蓋過能力值（`General.Bond`）：
//
//	Bond 指到的人效力於招募方   → 難度 0     ; 一定成功
//	Bond 指到的人在野           → 照上面算
//	效力於第三方                → 難度 160 + RND(10) ; 幾乎不可能
//
// 成功之後忠誠 ＝ 人望 ÷ 2 ＋ RND(人望 ÷ 2) ＋ 加成，夾到 1..100；
// 算出來不到 1 就當沒成功。
//
// **加成與費用由電腦諸侯的等級決定**：(30,0) 等級 0–2、(20,10) 等級 3、
// (10,20) 等級 4、(0,40) 等級 5——等級越高越便宜也越容易。
// 等級 0–2 那一組的費用 30 金正是說明書給玩家的價目，所以玩家這一邊
// 照 (30, 0) 算（`L2`：玩家的常式沒有單獨讀過）。
const (
	RecruitBondFree   = 0   // Bond 效力於招募方：難度 0
	RecruitBondWall   = 160 // Bond 效力於第三方：難度 160 + RND(10)
	RecruitAbilityDiv = 3   // 亂數除數的底：RND(4) + 3
)

// RecruitBonus 是電腦諸侯等級帶來的登用加成（第二個參數）。
func RecruitBonus(level int) int {
	switch {
	case level <= 2:
		return 0
	case level == 3:
		return 10
	case level == 4:
		return 20
	default:
		return 40
	}
}

// RecruitFee 是電腦諸侯等級帶來的登用費用（第一個參數）。
func RecruitFee(level int) int {
	switch {
	case level <= 2:
		return 30
	case level == 3:
		return 20
	case level == 4:
		return 10
	default:
		return 0
	}
}

// RecruitPersuasion 是說服力。
func RecruitPersuasion(prestige, governorCharm, bonus int) int {
	return (prestige*3+governorCharm)/3 + bonus
}

// RecruitDifficulty 是難度的能力值部分。r1、r2 是兩次 RND(4)。
func RecruitDifficulty(intel, war, r1, r2 int) int {
	return intel/(r1+RecruitAbilityDiv) + war/(r2+RecruitAbilityDiv)
}

// RecruitLoyalty 是登用成功之後的忠誠。r 是 RND(人望 ÷ 2)。
func RecruitLoyalty(prestige, r, bonus int) int {
	return prestige/2 + r + bonus
}

// 開倉賑民的判定與效果（`L0`、`[base]`，分派表 `0x55f4`／常式 `0xc8f6`）。
//
// 原版對每一個電腦諸侯的郡跑一次：
//
//	門檻 ＝ 門檻底[等級] ＋ RND(20)
//	民眾忠誠 >= 門檻 → 這回合不做
//	量   ＝ max((100 − 物價) ÷ 除數[等級], 5)
//	增幅 ＝ min(量 × 花的金 ÷ (人口 ÷ 1200), 太守魅力 ÷ 2)
//	民眾忠誠 ＝ min(民眾忠誠 ＋ 增幅, 100)
//
// **物價越低，同樣的金換到的忠誠越多**——那正是「拿金在當月物價買米
// 發下去」的形狀，也是這條規則與商業選單放在一起的理由。
// 說明書 p.22 寫的是撥米，而原版這一支扣的是金（`0xec24` 減的是
// 州郡 offset 18）；**一手的碼贏二手的敘述**。
//
// 「太守魅力越高，效果越好」在公式裡是增幅的上限 `魅力 ÷ 2`。
const ReliefMinRate = 5 // 量的下限（物價高到算出 0 時用它）

// ReliefThreshold 是「民眾忠誠低於多少才賑」的底（再加 RND(20)）。
func ReliefThreshold(level int) int {
	switch level {
	case 2:
		return 70
	case 3:
		return 60
	default:
		return 80
	}
}

// ReliefRate 是每一分錢換多少忠誠的係數的分母。
func ReliefRate(level int) int {
	switch level {
	case 4:
		return 9
	case 5:
		return 7
	default:
		return 10
	}
}

// ReliefGain 是賑一次漲多少民眾忠誠（`L1`、`0xc8f6`）。
//
//	每格 ＝ 人口 ÷ 1200
//	增幅 ＝ min(量 × 撥出的金 ÷ 每格, 太守魅力 ÷ 2)
//
// ⚠ **`量 × 撥出的金` 是 16 位元有號乘法**：原版的 `imulw` 之後接 `cwd`，
// 高位字被蓋掉（`0xc96a`–`0xc96e`）。郡的金到上限時真的溢位——
// 量 6 × 預算 6000 ＝ 36000 → −29536，忠誠反而暴跌。
// 這不是模擬器的誤差，是原版的算術，99 次實跑逐次相同。
func ReliefGain(priceLevel, population, gold, governorCharm, level int) int {
	per := ReliefPerStep(population)
	if per <= 0 {
		return 0
	}
	gain := int(int16(ReliefRateOf(priceLevel, level)*gold)) / per
	if cap := governorCharm / 2; gain > cap {
		gain = cap
	}
	return gain
}

// ReliefRateOf 是一分錢換多少忠誠的係數（`L1`、`0xca4b` 起的六份）。
//
// **不是 `max(量, 5)`**：原版只在算出來 `<= 0` 時才換成 5
// （`0xc92f` 的 `cmpw $0, 量; jg`），物價高到讓量掉成 1–4 時就用 1–4。
func ReliefRateOf(priceLevel, level int) int {
	rate := (100 - priceLevel) / ReliefRate(level)
	if rate <= 0 {
		rate = ReliefMinRate
	}
	return rate
}

// ReliefPerStep 是「多少人口換一格忠誠」（原版 `0xc95c` 的 `idiv 12`，
// 而人口存的是實際值 ÷ 100，所以是 人口 ÷ 1200）。
func ReliefPerStep(population int) int { return population / 100 / 12 }

// ReliefSecondCharge 是賑民**第二次**扣的錢（`L1`、`0xc9d6`）。
//
// 原版在寫回忠誠之前照**實際**漲到的幅度再收一次
// （`增幅 ÷ 量 × 每格`，浮點算完轉回 16 位元整數）——也就是整份預算之外
// 再付一次。99 次實跑逐次相同（`docs/playtest/02`）。
func ReliefSecondCharge(gain, rate, per int) int {
	if rate <= 0 {
		return 0
	}
	return int(int16(gain * per / rate))
}

// 賞賜金帛的效果（`L0`、`[base]`，分派表 `0x5654`／常式 `0xd302`）。
//
//	金   ＝ min(本回合預算, 100)                    ; 說明書 p.23 的上限就在碼裡
//	效果 ＝ RND(加成 ÷ 2) ＋ 太守魅力 ÷ 3 ＋ 加成
//	增幅 ＝ 效果 × 金 ÷ 100                         ; 忠誠夾到 100
//	花費 ＝ 實際增幅 × 100 ÷ 效果                   ; 只付真的換到的那一段
//
// **加成由電腦諸侯的等級決定**：0、0、0、10、30、40。
// 兩個係數（`0.01` 與 `100`）從執行期記憶體讀出來。
// 君主自己不受賞（呼叫端跳過身分 0）。
//
// 「花費按實際增幅反算」是這一版 AI 反覆出現的形狀：先用整份預算算出
// 想要的效果，夾住之後再回頭付帳（開倉賑民也是，見 `ReliefGain`）。
func RewardBonus(level int) int {
	switch level {
	case 3:
		return 10
	case 4:
		return 30
	case 5:
		return 40
	}
	return 0
}

// RewardEffect 是一分錢換多少忠誠的係數（百分之一為單位）。
// roll 是 `RND(加成 ÷ 2)`。
func RewardEffect(governorCharm, bonus, roll int) int {
	return roll + governorCharm/3 + bonus
}

// RewardGain 是賞 gold 金換到的忠誠（還沒夾上限）。
func RewardGain(effect, gold int) int { return effect * gold / 100 }

// RewardCost 是照實際增幅反算回來的花費。
func RewardCost(effect, gain int) int {
	if effect <= 0 {
		return 0
	}
	return clampTo(gain*100/effect, MaxReward)
}

// RicePerGold 是一金在當月的物價買得到幾單位米（`L0`、`[base]`）。
//
// 原版的電腦諸侯買米走 `0xc634`，一金換到的量是 `(100 − 物價) ÷ 10`
// ——**物價越低買到越多**，而不是「一單位米固定值多少金」。
// 開倉賑民（`0xc8f6`）拿的是同一個量當「一分錢換多少忠誠」的係數，
// 兩邊對得起來：賑民就是拿金在當月物價買米發下去。
//
// ⚠ **下限 1 是 remake 加的**：原版沒有擋，物價到 100 會在浮點除法裡
// 變成無窮大。實測十六個月四十二個郡的物價全部落在 30–68
// （`docs/mechanics/60-economy`），所以那條路走不到。
//
// ⚠ **這是玩家那一條的除數（10）。** 電腦買米的除數隨 AI 等級變
// （`AIRicePerGold`），等級 5 是 3——同一個物價下它換到的米是玩家的
// 三倍有餘。
//
// **買賣共用這一條**：玩家買米 `0x1b20e` 與賣米 `0x1b49c` 算的是同一個
// 量。買是「一金換 rate 米」，賣是「rate 米換一金」——所以在同一個月
// 買進再賣出剛好不賺不賠，零頭還會被除法吃掉。
func RicePerGold(priceLevel uint8) int { return ricePerGold(priceLevel, RiceRateDiv) }

// RiceRateDiv 是玩家那一條的除數（`0x1b20e`／`0x1b49c`）。
const RiceRateDiv = 10

// riceRateDiv 是電腦買米那六支各自的除數（呼叫端 `0x0c7d1`、`0x0c805`、
// `0x0c839`、`0x0c86d`、`0x0c8a1`、`0x0c8dd`，`L0`）。
//
// 前五支是 `mov cx,除數` ＋ `idiv cx`；**第六支不是除法**，
// 而是 `mov cx,3` ＋ `sar cl,ax` 加上前後兩次 `xor dx / sub dx`
// 的正負號修正（`0xc8ce`–`0xc8db`）——那個 3 是**位移量**，除數是 8。
//
// 等級 4 除以 9、等級 5 除以 8，其餘除以 10。物價 50 時等級 0–3 一金換
// 5 單位米，等級 5 換 6。這與內政那張表同一個形狀（六支同樣的碼、
// 只有立即數不同），也是「六個等級是六種性格」的第三個例子。
var riceRateDiv = [6]int{10, 10, 10, 10, 9, 8}

// AIRicePerGold 是電腦諸侯買米的匯率；等級越界夾住。
func AIRicePerGold(priceLevel uint8, level int) int {
	if level < 0 {
		level = 0
	}
	if level >= len(riceRateDiv) {
		level = len(riceRateDiv) - 1
	}
	return ricePerGold(priceLevel, riceRateDiv[level])
}

func ricePerGold(priceLevel uint8, div int) int {
	if div < 1 {
		div = 1
	}
	if n := (100 - int(priceLevel)) / div; n > 1 {
		return n
	}
	return 1
}

// 挖角的候選條件（`L0`、`[base]`，分派表 `0x56d4`／挑人 `0xe0bc`／
// 判定 `0x1dc0a`）。原版掃全部 350 人，收進候選清單的條件是：
//
//	有主（效力勢力 != 0xFF）
//	不是招募方的人
//	身分 != 0（君主挖不動）
//	忠誠 < RND(15) + 80
//	Bond 指向自己，或 Bond 指到的人**不在**他現在的勢力
//
// 最後那一條與登用是**同一道牽絆閘門的鏡像**（`RecruitBondFree`）：
// 登用問「他的牽絆對象在不在我這邊」，挖角問「在不在他那邊」。
const (
	HeadhuntLoyaltyFloor  = 80 // 忠誠門檻的底
	HeadhuntLoyaltySpread = 15 // 再加 RND(15)
)

// HeadhuntLoyaltyBar 是這一次挖角的忠誠門檻；忠誠不低於它就挖不動。
func HeadhuntLoyaltyBar(roll int) int { return HeadhuntLoyaltyFloor + roll }

// ---- 軍師勸諫 ------------------------------------------------------------

// 發動戰役之前，軍師有機會跳出來勸一次（原版 `0x18a90`，`L0`）：
//
//	RND(5) + 80 < 軍師的謀略 → 勸諫，答 N 就取消出兵
//
// 門檻的下界是 AdvisorWarnFloor、亂數的寬度是 AdvisorWarnSpread，
// 所以謀略 85 以上一定勸、80 以下一定不勸。沒有軍師的勢力不會勸
// ——原版那一格存 `0xFFFF`，帶號比較之下永遠不成立。
const (
	AdvisorWarnFloor  = 80
	AdvisorWarnSpread = 5
)

// AdvisorWarns 回報這一次發動戰役軍師會不會出來勸。
func AdvisorWarns(chiefIntel, roll int) bool {
	return AdvisorWarnFloor+roll < chiefIntel
}
