package battle

import (
	"math/big"

	"github.com/wicanr2/softworld_san1_remake/internal/i18n"
)

// 軍力與戰鬥隊伍（說明書 p.26–28）。

// Side 是四種軍力。
type Side uint8

const (
	MainAttacker Side = iota // 主攻軍：發動戰爭的軍隊
	AidAttacker              // 助攻軍：必須運用謀略才能得到
	MainDefender             // 主守軍：戰場所在州郡的當地駐軍
	AidDefender              // 助守軍：應付「聯合出兵」時才出動
	sideCount
)

func (s Side) String() string {
	switch s {
	case MainAttacker:
		return "主攻軍"
	case AidAttacker:
		return "助攻軍"
	case MainDefender:
		return "主守軍"
	case AidDefender:
		return "助守軍"
	}
	return "?"
}

// Attacking 回報這一方是不是攻方。
func (s Side) Attacking() bool { return s == MainAttacker || s == AidAttacker }

// Formation 是五種戰鬥隊伍。
type Formation uint8

const (
	Vanguard Formation = iota // 先鋒
	Left                      // 左軍
	Right                     // 右軍
	Centre                    // 中軍
	Rear                      // 後軍
	formationCount
)

func (f Formation) String() string {
	switch f {
	case Vanguard:
		return "先鋒"
	case Left:
		return "左軍"
	case Right:
		return "右軍"
	case Centre:
		return "中軍"
	case Rear:
		return "後軍"
	}
	return "?"
}

// MaxLeaders 是每組最多幾名將領（說明書 p.27）。
const MaxLeaders = 10

// ActionOrder 是作戰時各隊伍的行動順序（說明書 p.28）：
// 先鋒 → 左軍 → 右軍 → 中軍 → 後軍。
func ActionOrder() []Formation { return []Formation{Vanguard, Left, Right, Centre, Rear} }

// SideActionOrder 是作戰時各軍力的行動順序（說明書 p.28）：
// 主守軍 → 助守軍 → 主攻軍 → 助攻軍。
func SideActionOrder() []Side {
	return []Side{MainDefender, AidDefender, MainAttacker, AidAttacker}
}

// SideDeployOrder 是佈置全軍的順序（說明書 p.28）：
// 主攻軍 → 助攻軍 → 助守軍 → 主守軍。
func SideDeployOrder() []Side {
	return []Side{MainAttacker, AidAttacker, AidDefender, MainDefender}
}

// DeployOrder 是紮營的隊伍順序（說明書 p.28）：
// 中軍 → 先鋒 → 左軍 → 右軍 → 後軍。
func DeployOrder() []Formation {
	return []Formation{Centre, Vanguard, Left, Right, Rear}
}

// Leader 是隊伍裡的一位將領。欄位對應 `internal/game` 的人物。
type Leader struct {
	Index    int // 人物槽號
	Name     string
	War      uint8 // 戰力
	Intel    uint8 // 謀略
	Stamina  uint8 // 體能
	Charm    uint8 // 魅力
	Soldiers int
	Training uint8
	Arms     uint8
	Troop    TroopKind

	// Captured／Dead 是決勝之後的處置要看的。
	Captured bool
	Dead     bool

	// Lord／Loyalty／BondAlly 是被擒之後**電腦捕獲方**當場處置要看的
	// （`0x259fe`／`0x25e50`，`L0`）：君主一律斬首；招降判定看忠誠、
	// 人望，以及牽絆對象（人物 offset 14）是不是同一勢力的人。
	Lord     bool
	Loyalty  int
	BondAlly bool

	// Fate 是電腦捕獲方當場做的處置，CapturedBy 是哪一方抓的。玩家
	// 捕獲的留給戰略層問（`Fate == FateNone`）。
	Fate       Fate
	CapturedBy Side

	// Deserted 表示這一位在自己的回合結束時投奔了敵軍（`0x27604`）：
	// 這一格只是佔位，人（帶著兵）已經在對方的部隊裡，`Fate` 是 Defected。
	Deserted bool
}

// InUnit 回報這一位還在不在隊上（沒死、沒被俘、沒投奔）。
func (l *Leader) InUnit() bool { return !l.Dead && !l.Captured && !l.Deserted }

// Fate 是被擒之後的下場（`0x259fe` 的四條路；電腦捕獲方只會走前兩條
// 與招降）。
type Fate uint8

const (
	FateNone Fate = iota
	Executed      // 斬首（`0x25f6a`）
	Jailed        // 囚禁（`0x260dc`）
	Released      // 釋放（`0x262b8`）——只有玩家會選
	Defected      // 招降（`0x25b94`）
)

func (f Fate) String() string {
	switch f {
	case Executed:
		return "斬首"
	case Jailed:
		return "囚禁"
	case Released:
		return "釋放"
	case Defected:
		return "招降"
	}
	return "未處置"
}

// Unit 是一支在戰場上的部隊：一個軍力的一個隊伍。
type Unit struct {
	Side      Side
	Formation Formation
	Leaders   []Leader

	At Hex

	// Move 是這一天剩下的移動力。
	Move int

	// Trapped 是中了陷阱之後不能活動的天數（說明書 p.33：九日）。
	Trapped int

	// Arrows 是還能射幾次箭。**這是狀態不是算出來的**：原版把它存在
	// 部隊記錄 offset 20，開戰時算一次（`ArrowCount`），每射一次遞減
	// （`0x2abae`）。每次都重算的話一支部隊可以無限次射滿箭。
	Arrows int

	// Retreated／Wiped 表示已經離開戰場。
	Retreated bool
	Wiped     bool

	// Unplaced 標記這支是戰役中途才生出來的（招降或投敵的人進了空的槽位，
	// `enlist`），位置是 remake 先擺的；原版問玩家紮在哪。
	Unplaced bool

	// Started 是開戰時的兵力。自動作戰用它判斷「敗到該退兵了」——
	// 絕對人數說明不了，一千人的隊伍剩三百與三萬人的隊伍剩三百
	// 是完全不同的處境。
	Started int

	// Cap 是部隊記錄 offset 34：一天的移動力上限，**整場只在編成時算
	// 一次**（`0x27114`；傷亡不會讓它變）。0 表示沒算過，回填時用
	// `MovePoints` 現算。
	Cap int

	// Quality 是部隊記錄 offset 32 的**現值**：整編時是 `Ability`
	// （謀略與戰力的平均），之後**每一天輪到這支部隊之前重算**
	// （`0x26fc6`，`RefreshQuality`）——算式換成兵數加權的武裝、訓練
	// 與戰力。交戰結算與弓箭讀的是這一格，不是重算的平均。
	Quality int
}

// Alive 回報這支部隊還在不在場上。
//
// 判準是**還有將領**（部隊記錄 offset 28），不是還有兵：交戰結算裡
// 承受方的第一位在出手方打光時會留下來、兵是 0（`0x2a684`），原版的
// 日循環照樣輪到它。兵打光的將領正常都當場被俘、離隊，所以兩個判準
// 只在那一種情況下不同。
func (u *Unit) Alive() bool { return !u.Retreated && !u.Wiped && u.LeaderCount() > 0 }

// LeaderCount 是還在隊上的將領數（部隊記錄 offset 28）。
func (u *Unit) LeaderCount() int {
	n := 0
	for i := range u.Leaders {
		if u.Leaders[i].InUnit() {
			n++
		}
	}
	return n
}

// Soldiers 是這支部隊的總兵力。
func (u *Unit) Soldiers() int {
	n := 0
	for i := range u.Leaders {
		if u.Leaders[i].InUnit() {
			n += u.Leaders[i].Soldiers
		}
	}
	return n
}

// 每天輪到部隊之前重算綜合能力用的三個 double（`DS:0xa8e4`、
// `DS:0xa8d2`、`DS:0xa8ec`、`DS:0xa8f4`，`L0`）。
const (
	qualityTrainingWeight = 0.05
	qualityPercent        = 0.01
	qualityHalf           = 0.5
	qualityScale          = 100.0
)

// RefreshQuality 是原版每天輪到這支部隊之前重算的綜合能力
// （`0x26fc6`，`L0`；招降來的人讓空部隊重新有人時也叫一次）：
//
//	逐將領：v ＝ ftol((武裝 × 5 ÷ 20 ＋ 訓練 × 0.05 ＋ 戰力 × 14 ÷ 20) × 兵 × 0.01 ＋ 0.5)
//	        S ＋= v；N ＋= 兵
//	N > 0：offset 32 ＝ ftol(S ÷ N × 100 ＋ 0.5)
//	N ≤ 0：offset 32 ＝ 0
//
// 兩個 `÷ 20` 是整數除法（`idiv`），其餘在 x87 上算。**謀略不在裡面**
// ——整編那一支（`Ability`）才看謀略，第一天輪到之前用的還是那個值。
//
// **S 與 N 都是 16 位元有號數**（`[bp-2]`／`[bp-6]`，`fidivs`）：兩位各
// 兩萬七的部隊 N ＝ 54000 → −11536 → `N ≤ 0` → 綜合能力 0——那支部隊
// 打誰都不痛（盤面丙量到，`L1`）。照做，不修。
func (u *Unit) RefreshQuality() {
	var sum, total int16
	for i := range u.Leaders {
		x := &u.Leaders[i]
		if !x.InUnit() {
			continue
		}
		f := x87(int64(int(x.Arms) * 5 / 20))
		f.Add(f, new(big.Float).SetPrec(64).Mul(x87(int64(x.Training)),
			new(big.Float).SetPrec(64).SetFloat64(qualityTrainingWeight)))
		f.Add(f, x87(int64(int(x.War)*14/20)))
		f.Mul(f, x87(int64(int16(x.Soldiers))))
		f.Mul(f, new(big.Float).SetPrec(64).SetFloat64(qualityPercent))
		f.Add(f, new(big.Float).SetPrec(64).SetFloat64(qualityHalf))
		v, _ := f.Int64()
		sum += int16(v)
		total += int16(x.Soldiers)
	}
	if total <= 0 {
		u.Quality = 0
		return
	}
	f := new(big.Float).SetPrec(64).Quo(x87(int64(sum)), x87(int64(total)))
	f.Mul(f, new(big.Float).SetPrec(64).SetFloat64(qualityScale))
	f.Add(f, new(big.Float).SetPrec(64).SetFloat64(qualityHalf))
	v, _ := f.Int64()
	u.Quality = int(v)
}

// Chief 是這支部隊的領隊：戰力最高的那一位。
//
// 單挑與計謀的智力門檻都看領隊（說明書 p.30、p.32–34）。
func (u *Unit) Chief() *Leader {
	var best *Leader
	for i := range u.Leaders {
		x := &u.Leaders[i]
		if !x.InUnit() {
			continue
		}
		if best == nil || x.War > best.War {
			best = x
		}
	}
	return best
}

// Smartest 是隊伍裡謀略最高的那一位（用計要看他）。
func (u *Unit) Smartest() *Leader {
	var best *Leader
	for i := range u.Leaders {
		x := &u.Leaders[i]
		if !x.InUnit() {
			continue
		}
		if best == nil || x.Intel > best.Intel {
			best = x
		}
	}
	return best
}

// AvgTraining／AvgArms 是以兵數加權的平均。
//
// **加權不是算術平均**：一支一千人的精兵與一支十人的新兵，
// 算術平均會把整隊看成中等。
func (u *Unit) AvgTraining() int { return u.weighted(func(l *Leader) int { return int(l.Training) }) }
func (u *Unit) AvgArms() int     { return u.weighted(func(l *Leader) int { return int(l.Arms) }) }

func (u *Unit) weighted(f func(*Leader) int) int {
	num, den := 0, 0
	for i := range u.Leaders {
		x := &u.Leaders[i]
		if !x.InUnit() || x.Soldiers == 0 {
			continue
		}
		num += f(x) * x.Soldiers
		den += x.Soldiers
	}
	if den == 0 {
		return 0
	}
	return num / den
}

// Troop 是這支部隊的兵種：以兵數計，最多的那一種。
func (u *Unit) Troop() TroopKind {
	var count [7]int
	for i := range u.Leaders {
		x := &u.Leaders[i]
		if !x.InUnit() {
			continue
		}
		if int(x.Troop) < len(count) {
			count[x.Troop] += x.Soldiers
		}
	}
	best := TroopLand
	for k := range count {
		if count[k] > count[best] {
			best = TroopKind(k)
		}
	}
	return best
}

// ArrowCount 是這支部隊開戰時的弓箭次數，原版存在部隊記錄 offset 20
// （`0x272eb`，`L0`）。
//
// 原版逐位在場將領累加，再一起收斂：
//
//	S ＝ Σ round(武裝度ᵢ × 兵士數ᵢ ÷ 100)
//	N ＝ Σ 兵士數ᵢ
//	次數 ＝ round(S ÷ N × 100) ÷ 20      （N ≤ 0 時為 0）
//
// 也就是**兵數加權**的平均武裝度 ÷ 20。手冊 p.32 的算例
// （`(75 + 80 + 50 + 100) ÷ 4 = 76.25`，`76.25 ÷ 20 = 3.8125`，射三次）
// 看起來像算術平均，那是因為算例沒提兵數——**以碼為準**：
// 一支千人的精銳與一支十人的殘兵放在一起時，兩種算法差很多。
func ArrowCount(leaders []Leader) int {
	sum, troops := 0, 0
	for i := range leaders {
		x := &leaders[i]
		if !x.InUnit() || x.Soldiers <= 0 {
			continue
		}
		sum += (int(x.Arms)*x.Soldiers + 50) / 100
		troops += x.Soldiers
	}
	if troops <= 0 {
		return 0
	}
	return (sum*100 + troops/2) / troops / 20
}

// Name 是這支部隊給人看的名字。
//
// 走譯文：戰場上的提示與逐日戰報都用它，先前拿中文的 `String()` 拼，
// 英日文的戰場上部隊名全是中文。
func (u *Unit) Name() string {
	if c := u.Chief(); c != nil {
		return i18n.Sf("unit.name", u.Side.unitLabel(), u.Formation.Label(), i18n.PersonName(c.Name))
	}
	return i18n.Sf("unit.nameBare", u.Side.unitLabel(), u.Formation.Label())
}

// MovePoints 是這支部隊一天的移動力上限（原版 `0x27114`–`0x271dc`，`L0`）。
//
// 走十個將領槽，空的（`0xFFFF`）跳過：
//
//	每一位：值 ＝ ftol((訓練 × 0.75 + 武裝 ÷ 4) × 兵力 × 0.01 + 0.5)
//	上限   ＝ ftol(Σ值 ÷ Σ兵力 × 10 + 0.5) + 2
//	Σ兵力 ≤ 0 → 上限 ＝ 2（`0x271f9` 那條分支）
//
// 訓練度愈高愈遠、武裝度愈高**愈遠**——武裝在這裡是加分不是扣分
// （係數 ¼），與說明書 p.29–30 的「移動力來源是訓練度」「全副武裝將
// 稍減移動力」後半句不合。**以碼為準**：訓 97／武裝 97 的部隊量到 12，
// 訓 50／武裝 50 量到 7，訓 80／武裝 80 量到 10，三支都對得上
// （`TestBattleUnitsMatchTheOriginal`）。
//
// 常數 0.75／0.01／0.5／10.0 是 `DS:0xa8fc`／`0xa8d2`／`0xa8ec`／`0xa904`
// 四個 double，直接從資料段讀出來的。
//
// ⚠ **兩個累計器都是 16 位元**（`add %ax,-0x2(%bp)`／`-0x6(%bp)`），
// 而且分母走 `fidivs`（整數除數）——十位將領的兵加起來超過 32767 就會
// 繞回去。那是原版的行為，照抄。
//
// ⚠ **`0x2e07a` 的 `min(15, (訓練 − 武裝 + 100) ÷ 10 + 1)` 是另一件事**：
// 那一支算的是對戰子畫面裡**單一將領**的值，不是部隊在 12×10 的戰場上
// 一天能走多遠。
func (u *Unit) MovePoints() int {
	var num, den int16
	for i := range u.Leaders {
		x := &u.Leaders[i]
		if !x.InUnit() || x.Soldiers <= 0 {
			continue
		}
		v := (float64(x.Training)*0.75 + float64(int(x.Arms)/4)) *
			float64(x.Soldiers) * 0.01
		num += int16(v + 0.5)
		den += int16(x.Soldiers)
	}
	if den <= 0 {
		return MoveFloor
	}
	return int(float64(num)/float64(den)*10+0.5) + MoveFloor
}

// moveCap 是回填用的上限：編成時算好的 Cap，沒有就現算。
func (u *Unit) moveCap() int {
	if u.Cap > 0 {
		return u.Cap
	}
	return u.MovePoints()
}

// MoveFloor 是移動力上限的底（`0x271b8` 的 `inc ax` 兩次，以及
// `0x271f9` 那條分支寫死的 2）。訓練與武裝都拉滿，上限本身也只到 12。
const MoveFloor = 2

// MoveMax 是**剩下的**移動力的天花板，休息累加時夾在這裡
//（`0x27c2d` 的 `cmpw $0xf`）。上限本身（`MovePoints`）到不了它，
// 但一天 +2 的休息會——原版量到連休七天的部隊停在 15。
const MoveMax = 15

// OriginalIndex 是原版部隊陣列裡的軍力編號。
//
// 原版把四個軍力排成 主守、助守、主攻、助攻，也就是行動順序
// （說明書 p.28）：部隊記錄在 `es:[0x3502 + (軍力×10 + 隊伍)×42]`，
// 旗幟組名表 `DS:0x5da2` 依同一個順序放 `D0`、`D1`、`A0`、`A1`。
// 對上的證據是紮完寨的基準畫面——主守的周瑜軍掛 `D0`、主攻的陳就軍
// 掛 `A0`（`internal/assets/battlefield.go`）。
func (s Side) OriginalIndex() int {
	for i, v := range SideActionOrder() {
		if v == s {
			return i
		}
	}
	return -1
}

// OriginalIndex 是原版旗幟上的隊伍編號：0–4 依序是中軍、先鋒、
// 左軍、右軍、後軍，與紮營順序相同（說明書 p.28）。
//
// remake 的 `Formation` 照的是行動順序，兩者不同，所以要換算。
func (f Formation) OriginalIndex() int {
	for i, v := range DeployOrder() {
		if v == f {
			return i
		}
	}
	return -1
}

// Ability 是這支部隊的**綜合能力**（原版部隊記錄 offset 32，
// `docs/re/05` §3.3）：
//
//	Σ(謀略 × 2 ÷ 5 ＋ 戰力 × 3 ÷ 5) ÷ 將領人數
//
// **逐人先除再加**，兩個除法都是整數除法。電腦對電腦的戰役拿它當戰力
// 的品質（`AutoResolveAI`）——那條路不進戰術層，整場只用這一個數與
// 兵士數。
func (u *Unit) Ability() int {
	sum, n := 0, 0
	for i := range u.Leaders {
		x := &u.Leaders[i]
		if !x.InUnit() {
			continue
		}
		sum += int(x.Intel)*2/5 + int(x.War)*3/5
		n++
	}
	if n == 0 {
		return 0
	}
	return sum / n
}
