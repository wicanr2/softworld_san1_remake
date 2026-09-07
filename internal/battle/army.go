package battle

import "fmt"

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

	// Started 是開戰時的兵力。自動作戰用它判斷「敗到該退兵了」——
	// 絕對人數說明不了，一千人的隊伍剩三百與三萬人的隊伍剩三百
	// 是完全不同的處境。
	Started int
}

// Alive 回報這支部隊還在不在場上。
func (u *Unit) Alive() bool { return !u.Retreated && !u.Wiped && u.Soldiers() > 0 }

// Soldiers 是這支部隊的總兵力。
func (u *Unit) Soldiers() int {
	n := 0
	for i := range u.Leaders {
		if !u.Leaders[i].Dead && !u.Leaders[i].Captured {
			n += u.Leaders[i].Soldiers
		}
	}
	return n
}

// Chief 是這支部隊的領隊：戰力最高的那一位。
//
// 單挑與計謀的智力門檻都看領隊（說明書 p.30、p.32–34）。
func (u *Unit) Chief() *Leader {
	var best *Leader
	for i := range u.Leaders {
		x := &u.Leaders[i]
		if x.Dead || x.Captured {
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
		if x.Dead || x.Captured {
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
		if x.Dead || x.Captured || x.Soldiers == 0 {
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
		if x.Dead || x.Captured {
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
		if x.Dead || x.Captured || x.Soldiers <= 0 {
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
func (u *Unit) Name() string {
	if c := u.Chief(); c != nil {
		return fmt.Sprintf("%s%s（%s）", u.Side, u.Formation, c.Name)
	}
	return fmt.Sprintf("%s%s", u.Side, u.Formation)
}

// MovePoints 是這支部隊一天的移動力。
//
// 原版對每一位將領各算一次（`0x2e07a`，`L0`）：
//
//	移動力 ＝ min(15, (訓練度 − 武裝度 + 100) ÷ 10 + 1)
//
// 訓練度愈高愈遠、武裝度愈高愈近，正是手冊 p.29–30 說的
// 「移動力來源是訓練度」「全副武裝將稍減移動力」。範圍 1–15。
//
// remake 這一層算的是部隊不是單一將領，所以拿部隊的加權平均代入。
func (u *Unit) MovePoints() int {
	mp := (u.AvgTraining()-u.AvgArms()+100)/10 + 1
	if mp > MoveMax {
		mp = MoveMax
	}
	if mp < 1 {
		mp = 1
	}
	return mp
}

// MoveMax 是移動力的上限，原版寫死在 `0x2e08f`。
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
		if x.Dead || x.Captured {
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
