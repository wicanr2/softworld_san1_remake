package battle

import (
	"fmt"
	"math/big"
	"sort"

	"github.com/wicanr2/softworld_san1_remake/internal/i18n"
)

// 主戰場的一場戰役（說明書 p.28–35）。

// Weather 是天氣。火攻要刮風、水淹要下雨、燒糧下雨不能用（p.32–34）。
type Weather uint8

const (
	Clear Weather = iota // 晴
	Windy                // 刮風
	Rainy                // 下雨
)

// OriginalIndex 是原版的天氣編號：**晴 0、雨 1、風 2**。
//
// remake 的列舉照說明書講到的順序排（晴、刮風、下雨），原版不是。
// 判準有兩條，同一張基準畫面上互相印證：天氣名表在 `DS:0x7892`，
// 三筆每筆 5 byte，依序是「 晴 」「 雨 」「 風 」；而畫面左欄的圖示是
// `WEATHER%d.IMG`（`0x21969` 算的是 `237 + 天氣 mod 3`），基準畫面上
// 那一張是 `WEATHER1`，配的字正是「雨」。
//
// **只有在對回原版的編號時才用它**：規則層一律用列舉本身。
func (w Weather) OriginalIndex() int {
	switch w {
	case Rainy:
		return 1
	case Windy:
		return 2
	}
	return 0
}

func (w Weather) String() string {
	switch w {
	case Windy:
		return "刮風"
	case Rainy:
		return "下雨"
	}
	return "晴"
}

// Battle 是一場進行中的戰役。
type Battle struct {
	Field *Field

	// Day 是第幾天，從 1 起。主戰場每一回合耗去一天（說明書 p.28）。
	Day     int
	Weather Weather

	Units []*Unit

	// Gold／Rice 是各軍隨軍攜帶的錢糧（說明書 p.28）。
	// 「沒有帶錢就無法用計」「沒米給軍隊吃，士兵將會陸續逃亡」。
	Gold [sideCount]int
	Rice [sideCount]int

	// CityHeld 是城池目前在誰手上。守方一開始持有。
	CityHeld Side

	Log []string

	// Over／AttackerWon 是結果。
	Over        bool
	AttackerWon bool

	// Rules 是版本與難度決定的規則（`rules.go`）。零值是原版。
	Rules Rules

	// AI 是電腦部隊用哪一套判斷式（`autobase.go`）。零值是原版。
	AI AI

	// Difficulty 是難度：原版的快戰與死戰把它除以 5 當交戰結算的模式
	// （`0x29b13`／`0x29d33`）。
	Difficulty int

	// Escapes 是各方退兵時逃得去的鄰郡（`Escape`）。沒填就逃不了。
	Escapes [sideCount][]Escape

	// Commander 是四種軍力的統帥（人物槽號），−1 表示這一方沒出場。
	//
	// 原版把它存在軍力記錄的第 0 個欄位（`es:[0x175e + 22×軍力]`），
	// 勝負判定拿它和「該方第一支部隊的第一位將領」比（`0x24f8c`）——
	// **對不上就等於這一方的統帥不在了**。
	//
	// 取的是該方名單的排頭：原版的統帥就是編隊時排在最前面的那一位。
	// **沒有另設欄位讓呼叫端指定**——只有這一條證據，多造一個旋鈕會
	// 固定住一個還沒量過的假設。
	Commander [sideCount]int

	rng    *rand
	rollFn func(int) int
}

// Setup 是開一場戰役要的東西。
type Setup struct {
	Field   *Field
	Weather Weather
	Seed    uint32

	// Attackers／Defenders 是各方的將領，會被分成五種隊伍。
	Attackers []Leader
	Defenders []Leader

	// AidAttackers／AidDefenders 是助攻軍與助守軍，可以是空的。
	AidAttackers []Leader
	AidDefenders []Leader

	// Rules 是版本規則（`RulesFor`）。零值是原版。
	Rules Rules

	// AI、Difficulty、Escapes 見 Battle 的同名欄位。
	AI         AI
	Difficulty int
	Escapes    [sideCount][]Escape

	// FromGate 是主攻軍的入口（來犯的鄰郡編號）。
	FromGate int

	AttackerGold, AttackerRice int
	DefenderGold, DefenderRice int
}

// New 開一場戰役。
//
// 編隊順序是主攻軍 → 助攻軍 → 主守軍 → 助守軍（說明書 p.27），
// 紮營順序是中軍 → 先鋒 → 左軍 → 右軍 → 後軍（p.28）。
func New(s Setup) *Battle {
	b := &Battle{Field: s.Field, Day: 1, Weather: s.Weather,
		CityHeld: MainDefender, rng: newRand(s.Seed), Rules: s.Rules,
		AI: s.AI, Difficulty: s.Difficulty, Escapes: s.Escapes}
	b.Gold[MainAttacker], b.Rice[MainAttacker] = s.AttackerGold, s.AttackerRice
	b.Gold[MainDefender], b.Rice[MainDefender] = s.DefenderGold, s.DefenderRice

	for i := range b.Commander {
		b.Commander[i] = -1
	}

	entry := s.Field.Gate(s.FromGate)
	if !s.Field.InBounds(entry) {
		entry = FromOffset(0, FieldH/2)
	}
	for _, side := range SideDeployOrder() {
		var pool []Leader
		switch side {
		case MainAttacker:
			pool = s.Attackers
		case AidAttacker:
			pool = s.AidAttackers
		case MainDefender:
			pool = s.Defenders
		case AidDefender:
			pool = s.AidDefenders
		}
		if len(pool) == 0 {
			continue
		}
		b.Commander[side] = pool[0].Index
		base := entry
		if !side.Attacking() {
			base = s.Field.CityAt
		}
		b.Units = append(b.Units, b.formUp(side, pool, base)...)
	}
	b.note("blog.start", b.Weather.Label())
	return b
}

// formUp 把一批將領分成五種隊伍並紮營。
//
// 「如果派出全部兵力，各戰鬥組的將領人數必須平均分配」「每組最多 10 名將領」
// （說明書 p.27）。
func (b *Battle) formUp(side Side, pool []Leader, base Hex) []*Unit {
	// 依戰力排序再輪流分配：分組是決定性的，而且中軍先分到最強的
	// （紮營順序以中軍為首，p.28）。手冊只要求「人數平均分配」，
	// 沒說哪一隊該強，強弱的取捨是 remake 的。
	sorted := append([]Leader(nil), pool...)
	sort.SliceStable(sorted, func(i, j int) bool { return sorted[i].War > sorted[j].War })

	groups := int(formationCount)
	if len(sorted) < groups {
		groups = len(sorted)
	}
	units := make([]*Unit, 0, groups)
	order := DeployOrder()
	for i := 0; i < groups; i++ {
		units = append(units, &Unit{Side: side, Formation: order[i]})
	}
	for i, l := range sorted {
		u := units[i%groups]
		if len(u.Leaders) >= MaxLeaders {
			continue
		}
		u.Leaders = append(u.Leaders, l)
	}
	for _, u := range units {
		u.Arrows = ArrowCount(u.Leaders)
	}
	// 紮營：依 DeployOrder 從基準點往外排。
	spot := base
	for i, u := range units {
		h := spot
		for n := 0; n < 6 && (!b.Field.At(h).Passable() || b.occupied(h)); n++ {
			h = spot.Step(Dirs()[n])
		}
		u.At = h
		u.Move = u.MovePoints()
		u.Started = u.Soldiers()
		spot = base.Step(Dirs()[i%6])
	}
	return units
}

// Camp 把一支部隊移到指定的格子上——**紮營，不是移動**：
// 不扣移動力、不看距離，但落點要走得進去而且沒有人。
//
// 原版逐隊問位置：`(%2d%s)%s之%s請%s將軍紮寨`（`AA.EXE` `0x46b67`，
// `docs/re/04` §5）。
func (b *Battle) Camp(u *Unit, at Hex) error {
	if u == nil || !u.Alive() {
		return fmt.Errorf("battle: 這支部隊不在場上")
	}
	if b.Day != 1 {
		return fmt.Errorf("battle: 紮營只在開戰前")
	}
	if !b.Field.InBounds(at) {
		return fmt.Errorf("battle: 出界了")
	}
	if !b.Field.At(at).Passable() {
		return fmt.Errorf("battle: %s 紮不了營", b.Field.At(at))
	}
	if x := b.UnitAt(at); x != nil && x != u {
		return fmt.Errorf("battle: 那一格已經有 %s", x.Name())
	}
	u.At = at
	if at == b.Field.CityAt {
		b.CityHeld = u.Side
	}
	return nil
}

// CampArea 回報一支部隊能不能在這一格紮營，給畫面先擋掉不能選的格子。
func (b *Battle) CampArea(u *Unit, at Hex) bool {
	if !b.Field.InBounds(at) || !b.Field.At(at).Passable() {
		return false
	}
	x := b.UnitAt(at)
	return x == nil || x == u
}

func (b *Battle) occupied(h Hex) bool {
	for _, u := range b.Units {
		if u.Alive() && u.At == h {
			return true
		}
	}
	return false
}

// UnitAt 回傳某一格上的部隊；沒有回 nil。
func (b *Battle) UnitAt(h Hex) *Unit {
	for _, u := range b.Units {
		if u.Alive() && u.At == h {
			return u
		}
	}
	return nil
}

// note 記一則逐日戰報。key 是譯文鍵（`blog.*`），參數照那一句的順序給；
// 人名要先過 `pn`，天氣、計謀、地形與軍別給 `Label()`。
//
// **戰報是當下的語系寫成的**：切換語系之後，已經寫下的那幾行不會跟著換。
func (b *Battle) note(key string, a ...any) {
	b.Log = append(b.Log, i18n.Sf("blog.day", b.Day)+i18n.Sf(key, a...))
}

// pn 是戰報裡的人名（英文轉拼音、日文換新字體）。
func pn(name string) string { return i18n.PersonName(name) }

// sideAlive 回報某個軍力還有沒有部隊在場上。
func (b *Battle) sideAlive(s Side) bool {
	for _, u := range b.Units {
		if u.Side == s && u.Alive() {
			return true
		}
	}
	return false
}

// Rest 是「休息」：在原地不動，**增加移動力 2、夾在 15**
//（原版 `0x27c00`，`L0`）：
//
//	addw $2, es:[bx+0x3526]        ; 部隊記錄 offset 36
//	cmpw $0xf, es:[bx+0x3526]
//	jle  skip
//	movw $0xf, es:[bx+0x3526]
//
// 說明書 p.29、p.30 的「每休息一次可增加移動力 2」只給了那個 2，
// **上限 15 是碼裡才有的**——原版量到連休七天的部隊停在 15
//（`TestBattleUnitsMatchTheOriginal`）。
func (b *Battle) Rest(u *Unit) error {
	if err := b.canAct(u); err != nil {
		return err
	}
	u.Move += TuneRestMove
	if u.Move > MoveMax {
		u.Move = MoveMax
	}
	// 「並恢復將領的體力」（p.30）。
	for i := range u.Leaders {
		if u.Leaders[i].Stamina < 100 {
			u.Leaders[i].Stamina++
		}
	}
	return nil
}

func (b *Battle) canAct(u *Unit) error {
	if b.Over {
		return fmt.Errorf("battle: 這場戰役已經結束")
	}
	if u == nil || !u.Alive() {
		return fmt.Errorf("battle: 這支部隊已經不在場上")
	}
	if u.Trapped > 0 {
		return fmt.Errorf("battle: %s 中了陷阱，還有 %d 天不能活動", u.Name(), u.Trapped)
	}
	return nil
}

// Move 是「移動」：往一個方向走一格。
//
// 「不得穿越其他軍隊」「大山無法穿越」（說明書 p.29）。
func (b *Battle) Move(u *Unit, d Dir) error {
	if err := b.canAct(u); err != nil {
		return err
	}
	to := u.At.Step(d)
	t := b.Field.At(to)
	if !t.Passable() {
		return fmt.Errorf("battle: %s 過不去", t)
	}
	if !b.Field.InBounds(to) {
		return fmt.Errorf("battle: 出界了")
	}
	if b.UnitAt(to) != nil {
		return fmt.Errorf("battle: 那一格有部隊，不得穿越")
	}
	cost := MoveCost(t, u.Troop())
	if u.Move < cost {
		return fmt.Errorf("battle: 移動力不足（要 %d，剩 %d）", cost, u.Move)
	}
	u.Move -= cost
	u.At = to
	if to == b.Field.CityAt {
		b.CityHeld = u.Side
		b.note("blog.takeCity", u.Name())
	}
	return nil
}

// power 是一支部隊此刻打出去的殺傷（原版 `0x305a9`，`L0`）。
//
// 原版對每一位將領各算一次，然後把兵士數乘進去：
//
//	殺傷 ＝ 兵士數 × 戰力值 ÷ 100
//
// 戰力值就是 LeaderPower。**主動的那一邊用地形的攻值、被打的那一邊
// 用守值**，而兩邊都用自己所在那一格的地形——所以地形對守方的保護
// 走的是守方自己那一份殺傷，不是把攻方的減掉。
func (b *Battle) power(u *Unit) int { return b.strike(u, true) }

// defence 是一支部隊被打時打回去的殺傷。
func (b *Battle) defence(u *Unit) int { return b.strike(u, false) }

// strike 是一支部隊此刻的殺傷，attacking 決定地形取攻值還是守值。
func (b *Battle) strike(u *Unit, attacking bool) int {
	t := b.Field.At(u.At)
	total := 0
	for i := range u.Leaders {
		x := &u.Leaders[i]
		if x.Dead || x.Captured || x.Soldiers <= 0 {
			continue
		}
		p := LeaderPower(int(x.War), int(x.Arms), x.Troop, t, attacking)
		total += x.Soldiers * p / 100
	}
	return total
}

// hit 讓 a 打 b 一次，回傳 b 的損失。
//
// pct 是這一擊的強度百分比，100 是白刃相接的基準。弓箭比它輕
// （`TuneArrowDamage`）；計謀那一側的倍率是量到的（`strikeMultiplier`）。
// **用百分比不用整數倍**：整數倍表示不了「箭只有一半殺傷」。
func (b *Battle) hit(a, d *Unit, pct int) int {
	return b.apply(d, b.power(a)*pct/100)
}

// strikeMultiplier 是交戰結算的傷害倍率表（`DS:0x81a2`，八格，`L0`）。
//
// 共同的交戰結算 `0x2a224` 拿模式當索引（`0x2a444`：`shl si` 之後
// `fimuls DS:0x81a2(%si)`）。**模式在 0..7 之外一律夾成 1**
// （`0x2a2c9`），所以第 1 格的 100 也是「傳錯值」的落點。
var strikeMultiplier = [8]int{80, 100, 150, 200, 250, 300, 350, 400}

// 用得到的兩格。
const (
	// ⚠ 圍攻**沒有固定的倍率格**：`0x2bc95` 起算 0，之後兩處加一
	// （`0x2bd04` 每一支圍著目標的敵方部隊、`0x2bd41` 施法者的領隊
	// 謀略 ≥ 98）。算法在 `UseStratagem` 的 `case Siege`。
	SiegeStrike = 0
	// MeleeStrike 是主戰場「對戰」的倍率格。**原版傳 8**（量到的），
	// 落在 0..7 之外被 `0x2a2c9` 夾成 1，所以倍率是 100。
	MeleeStrike = 1
	// LureStrike 是誘敵的倍率格。**原版傳的是 8（施法者謀略 ≥ 98 時 9），
	// 兩個都在 0..7 之外，被 `0x2a2c9` 夾成 1**——所以那個「謀略高就
	// 加碼」完全沒有作用，是原版的 bug。remake 照它，不修。
	LureStrike = 1
)

// StrikeMultiplier 是某個模式的傷害倍率（百分比）。
//
// 越界回第 1 格，與原版 `0x2a2c9` 相同——**不是防禦式寫法**，
// 誘敵就是靠這個落點決定倍率的。
func StrikeMultiplier(mode int) int {
	if mode < 0 || mode >= len(strikeMultiplier) {
		mode = 1
	}
	return strikeMultiplier[mode]
}

// 交戰結算的兩張地形表（`DS:0x8162` 攻／`DS:0x8182` 守，`L0`）。
//
// **與 `terrainAttack`／`terrainDefence` 不是同一組。** 那兩張是對戰
// 子畫面裡算單一將領戰力值用的（`0x2e01a`／`0x2e13a`）；主戰場的交戰
// 結算查的是這兩張，而且乘的是**部隊的**綜合能力不是將領的戰力
//（`docs/re/05` §3.6）。
var (
	meleeAttack = [terrainCount]int{
		Hill: 30, Shallow: 15, Deep: 10, City: 40,
		Fort: 30, Plain: 25, Forest: 20, Desert: 20, Mountain: 0,
	}
	meleeDefend = [terrainCount]int{
		Hill: 30, Shallow: 20, Deep: 10, City: 50,
		Fort: 40, Plain: 20, Forest: 25, Desert: 20, Mountain: 0,
	}
)

// exchange 是一次交戰結算（原版 `0x2a224`，`L0`）：a 打 d，
// 雙方同時互扣，回傳（d 的損失, a 的損失）。
//
//	a 的殺傷 ＝ ftol(攻值[a 那格] × a.兵士數 × a.綜合能力 × 倍率[模式] × 1e-6)
//	d 的殺傷 ＝ ftol(守值[d 那格] × d.兵士數 × d.綜合能力 × 1e-4)
//	a 的比例 ＝ d 的殺傷 ÷ a.兵士數      （兵士數 ≤ 0 → 0）
//	d 的比例 ＝ a 的殺傷 ÷ d.兵士數
//
// **倍率只乘在出手的那一邊**：表只被讀一次（`0x2a444`）。`1e-4` 正好是
// `1e-6 × 100`，也就是模式 1 的倍率——兩邊同一個尺度。
//
// **兩個比例都要在扣兵之前算完**：原版先把雙方的殺傷都算出來
//（`0x2a457`／`0x2a498`）再逐將領套，先扣一邊會讓先手佔便宜。
func (b *Battle) exchange(a, d *Unit, mode int) (int, int) {
	da := MeleeDamage(MeleeAttackValue(b.Field.At(a.At)),
		a.Soldiers(), a.Ability(), StrikeMultiplier(mode), MeleeAttackScale)
	dd := MeleeDamage(MeleeDefendValue(b.Field.At(d.At)),
		d.Soldiers(), d.Ability(), 1, MeleeDefendScale)
	ra, rd := MeleeRatio(dd, a.Soldiers()), MeleeRatio(da, d.Soldiers())
	wasA, wasD := a.Soldiers(), d.Soldiers()
	b.thin(a, ra)
	b.thin(d, rd)
	return wasD - d.Soldiers(), wasA - a.Soldiers()
}

// arrowTerrain 是弓箭的地形表（`DS:0x81c0`，`L0`）。
//
// **第三張表**：交戰查 `DS:0x8162`／`DS:0x8182`，對戰子畫面查
// `DS:0x85c2`／`DS:0x85e2`，弓箭查這一張。深水最重、山丘最輕
// ——空曠處沒有遮蔽。
var arrowTerrain = [terrainCount]int{
	Hill: 5, Shallow: 16, Deep: 20, City: 7,
	Fort: 8, Plain: 12, Forest: 9, Desert: 13, Mountain: 0,
}

// ArrowTerrainValue 是弓箭的地形值。
func ArrowTerrainValue(t Terrain) int { return arrowTerrain[t] }

// ArrowSurvivors 是一位將領挨了一次箭之後剩下的兵：
//
//	max(0, ftol(兵 × (1 − 比例)))
//
// **沒有交戰那道 −1**（`0x2aae3` 只把 ≤ 0 夾成 0），迴圈也是從第 0 格
// 往第 9 格走，與交戰相反。
func ArrowSurvivors(soldiers int, ratio float64) int {
	f := new(big.Float).SetPrec(64).Sub(x87(1),
		new(big.Float).SetPrec(64).SetFloat64(ratio))
	f.Mul(f, x87(int64(soldiers)))
	v, _ := f.Int64()
	if v < 0 {
		v = 0
	}
	return int(v)
}

// MeleeAttackValue／MeleeDefendValue 是交戰結算的地形值
//（`DS:0x8162`／`DS:0x8182`）。
func MeleeAttackValue(t Terrain) int { return meleeAttack[t] }
func MeleeDefendValue(t Terrain) int { return meleeDefend[t] }

// 兩邊的比例常數（`DS:0xa986`／`DS:0xa98e`）。守方那一邊不乘倍率，
// 而 `1e-4` 正好是 `1e-6 × 100`，也就是模式 1 的倍率。
const (
	MeleeAttackScale = 1e-6
	MeleeDefendScale = 1e-4
)

// x87 的暫存器是 **80 位元、64 位元尾數**，Go 的 `float64` 只有 53 位元。
// 這個差在交戰結算上看得到：`1500 × (1 − 585/1500)` 數學上是 915，
// 用 `float64` 算會捨進成剛好 `915.0`（915 附近的間距是 1.1e-13，
// 誤差 2e-14 不到半格），用 64 位元尾數算是 `914.99999999999998`
// ——`ftol` 截尾差 1，量到的原版是後者。所以這幾支照 x87 的寬度算。
func x87(v int64) *big.Float { return new(big.Float).SetPrec(64).SetInt64(v) }

// MeleeDamage 是交戰結算的殺傷。**照指令順序乘**，不要代數化簡——
// 原版的捨入誤差是行為的一部分（`docs/playtest/02`）。
//
//	filds 地形值；fimuls 兵士數；fimuls 綜合能力；fimuls 倍率；fmull k；ftol
func MeleeDamage(terrain, soldiers, ability, mult int, k float64) int {
	f := x87(int64(terrain))
	f.Mul(f, x87(int64(soldiers)))
	f.Mul(f, x87(int64(ability)))
	f.Mul(f, x87(int64(mult)))
	f.Mul(f, new(big.Float).SetPrec(64).SetFloat64(k))
	n, _ := f.Int64() // ftol 截尾
	return int(n)
}

// MeleeRatio 是傷亡比例。兵士數 ≤ 0 時原版取 `DS:0xa996` ＝ 0.0
//（`0x2a4a0`）。
//
// `fidivrs` 在 80 位元算完之後 `fstpl` 存成 **double**，所以比例本身
// 是 `float64`——這一步的捨入是原版就有的。
func MeleeRatio(damage, soldiers int) float64 {
	if soldiers <= 0 {
		return 0
	}
	q := new(big.Float).SetPrec(64).Quo(x87(int64(damage)), x87(int64(soldiers)))
	v, _ := q.Float64()
	return v
}

// MeleeSurvivors 是一位將領在交戰之後剩下的兵：
//
//	max(0, ftol(兵 × (1 − 比例)) − 1)
//
// **那個 −1 是原版的**（`0x2a5c9` 的 `dec ax`）。
func MeleeSurvivors(soldiers int, ratio float64) int {
	f := new(big.Float).SetPrec(64).Sub(x87(1),
		new(big.Float).SetPrec(64).SetFloat64(ratio))
	f.Mul(f, x87(int64(soldiers)))
	v, _ := f.Int64() // ftol 截尾
	n := int(v) - 1
	if n < 0 {
		n = 0
	}
	return n
}

// thin 把傷亡比例逐將領套上去（`0x2a57d`–`0x2a603`）。
//
//	新兵 ＝ max(0, ftol(兵 × (1 − 比例)) − 1)
//
// **那個 −1 是原版的**（`0x2a5c9` 的 `dec ax`）：少了它，量到的四項裡
// 三項會多 1。歸 0 的將領當場被俘、從部隊裡除名。
//
// 原版**從第 9 格往第 0 格走**，所以這裡也倒著走——同分時誰先被結算
// 會影響被俘的順序。
func (b *Battle) thin(u *Unit, ratio float64) {
	for i := len(u.Leaders) - 1; i >= 0; i-- {
		x := &u.Leaders[i]
		if x.Dead || x.Captured || x.Soldiers <= 0 {
			continue
		}
		n := MeleeSurvivors(x.Soldiers, ratio)
		if n == 0 {
			x.Captured = true
			b.note("blog.captured", pn(x.Name))
		}
		x.Soldiers = n
	}
	if u.Soldiers() == 0 && !u.Wiped {
		u.Wiped = true
		b.note("blog.wiped", u.Name())
	}
}

// apply 把一次殺傷落到部隊上，回傳實際的損失。
func (b *Battle) apply(d *Unit, loss int) int {
	if loss < 1 {
		loss = 1
	}
	if loss > d.Soldiers() {
		loss = d.Soldiers()
	}
	b.casualty(d, loss)
	return loss
}

// casualty 把損失分攤到隊伍裡的各將領。
//
// 「人員損耗後，其持有的軍械也隨同失去」（說明書 p.20）——所以
// 兵沒了武裝度不變（那是比率），但總量跟著少。
//
// **兵打光的將領被俘**（原版 `0x30716` 印「我們抓到%s」，接上
// 「1.斬首 2.囚禁 3.釋放 4.招降」的處置，`L0`）。
func (b *Battle) casualty(u *Unit, loss int) {
	total := u.Soldiers()
	if total <= 0 {
		return
	}
	left := loss
	for i := range u.Leaders {
		x := &u.Leaders[i]
		if x.Dead || x.Captured || x.Soldiers == 0 {
			continue
		}
		n := loss * x.Soldiers / total
		if n > x.Soldiers {
			n = x.Soldiers
		}
		x.Soldiers -= n
		left -= n
	}
	for i := range u.Leaders {
		if left <= 0 {
			break
		}
		x := &u.Leaders[i]
		n := left
		if n > x.Soldiers {
			n = x.Soldiers
		}
		x.Soldiers -= n
		left -= n
	}
	if u.Soldiers() == 0 {
		u.Wiped = true
		b.note("blog.wiped", u.Name())
	}
	// 兵打光就被俘（原版 0x30716）。
	for i := range u.Leaders {
		x := &u.Leaders[i]
		if x.Soldiers <= 0 && !x.Dead && !x.Captured {
			x.Captured = true
			b.note("blog.captured", pn(x.Name))
		}
	}
}

// QuickBattle 是「快戰」：雙方直接正面作戰（說明書 p.32）。
func (b *Battle) QuickBattle(a *Unit, d Dir) error {
	return b.melee(a, d, false)
}

// DeathBattle 是「死戰」：一決生死的激戰，**雙方將互戰至分出勝負為止**
// （說明書 p.32）。
func (b *Battle) DeathBattle(a *Unit, d Dir) error {
	return b.melee(a, d, true)
}

func (b *Battle) melee(a *Unit, d Dir, toTheDeath bool) error {
	return b.meleeMode(a, d, MeleeStrike, toTheDeath)
}

// meleeMode 是 melee 加上交戰結算的模式（倍率格，`StrikeMultiplier`）：
// 玩家的對戰傳 8（夾成 1），電腦的快戰傳 難度÷5＋1、死戰傳 難度÷5
//（`0x29b13`／`0x29d33`）。
func (b *Battle) meleeMode(a *Unit, d Dir, mode int, toTheDeath bool) error {
	if err := b.canAct(a); err != nil {
		return err
	}
	t := b.UnitAt(a.At.Step(d))
	if t == nil {
		return fmt.Errorf("battle: 那個方向沒有部隊")
	}
	if t.Side.Attacking() == a.Side.Attacking() {
		return fmt.Errorf("battle: 那是友軍")
	}
	rounds := 1
	if toTheDeath {
		rounds = 50 // 打到分出勝負；上限避免無窮迴圈
	}
	for i := 0; i < rounds; i++ {
		// **同時**：原版先把雙方的殺傷都算出來，再各自扣兵
		// （`0x30618`／`0x306bb`）。先扣一邊再算另一邊的話，
		// 先手會佔到不該有的便宜。
		// 主戰場的交戰走 `0x2a224`（`docs/re/05` §3.6）。量到玩家的
		// 「對戰」傳的模式是 8——落在 0..7 之外，被 `0x2a2c9` 夾成 1，
		// 也就是倍率 100。
		la, lb := b.exchange(a, t, mode)
		if !t.Alive() {
			b.note("blog.rout", a.Name(), t.Name(), la)
			break
		}
		if !a.Alive() {
			b.note("blog.counter", t.Name(), lb)
			break
		}
		if !toTheDeath {
			b.note("blog.clash", a.Name(), t.Name(), la, lb)
			break
		}
	}
	a.Move = 0
	// 一擊打光對方最後一支部隊時當場分勝負，不必等這一天結束。
	b.checkOver()
	return nil
}

// Archery 是「弓箭」：射箭削弱敵軍兵力（說明書 p.32）。
//
// 「攻擊目標必須**相間一格**，且不能隔著大山、城池或關寨。」
func (b *Battle) Archery(a *Unit, target Hex) error { return b.archery(a, target, a.Arrows) }

// ArcheryOnce 照原版一次射一箭（`0x2ab90` 一次遞減一）——電腦部隊的
// 弓箭選項（`autobase.go`）走這裡，整壺射完是玩家指令那一層的 remake 差異。
func (b *Battle) ArcheryOnce(a *Unit, target Hex) error { return b.archery(a, target, 1) }

func (b *Battle) archery(a *Unit, target Hex, n int) error {
	if err := b.canAct(a); err != nil {
		return err
	}
	mid, ok := lineMid(a.At, target)
	if !ok {
		return fmt.Errorf("battle: 弓箭的目標必須在同一直線上相間一格")
	}
	switch b.Field.At(mid) {
	case Mountain, City, Fort:
		return fmt.Errorf("battle: 不能隔著%s射箭", b.Field.At(mid))
	}
	t := b.UnitAt(target)
	if t == nil {
		return fmt.Errorf("battle: 那裡沒有部隊")
	}
	if t.Side.Attacking() == a.Side.Attacking() {
		return fmt.Errorf("battle: 那是友軍")
	}
	if a.Arrows <= 0 {
		return fmt.Errorf("battle: 箭射完了")
	}
	if n > a.Arrows {
		n = a.Arrows
	}
	// ⚠ **registered remake 差異**：原版一次射一箭，選單上印著剩幾次
	// （`DS:0x80ae`「弓箭攻擊 次數:%d」）；remake 一次把整壺射完，
	// 因為一次一箭配上一次一回合，在收斂成部隊對部隊的這一層太細。
	// 次數本身是量到的（`ArrowCount`），而且**會用完**。
	// 一箭的殺傷照原版（`0x2aa3b`–`0x2ab00`，`L0`）：
	//
	//	殺傷 ＝ ftol(弓箭表[射手那格] × 射手.兵士數 × 射手.綜合能力 × 1e-4)
	//	比例 ＝ 殺傷 ÷ 目標.兵士數
	//	逐將領：新兵 ＝ max(0, ftol(兵 × (1 − 比例)))
	total := 0
	for i := 0; i < n; i++ {
		if !t.Alive() {
			break
		}
		d := MeleeDamage(ArrowTerrainValue(b.Field.At(a.At)),
			a.Soldiers(), a.Ability(), 1, MeleeDefendScale)
		r := MeleeRatio(d, t.Soldiers())
		was := t.Soldiers()
		for j := range t.Leaders {
			x := &t.Leaders[j]
			if x.Dead || x.Captured || x.Soldiers <= 0 {
				continue
			}
			x.Soldiers = ArrowSurvivors(x.Soldiers, r)
		}
		total += was - t.Soldiers()
	}
	if t.Soldiers() == 0 && !t.Wiped {
		t.Wiped = true
		b.note("blog.wiped", t.Name())
	}
	a.Arrows -= n
	b.note("blog.arrows", a.Name(), n, t.Name(), total)
	a.Move = 0
	return nil
}

// lineMid 回傳 a 與 target 之間的那一格，target 不在同一直線上就回 false。
//
// **「相間一格」不等於「距離 2」。** 折線走兩步到的格子中間有兩格，
// 說不出箭「隔著什麼」飛過去；只有同一方向連走兩步才唯一。
func lineMid(a, target Hex) (Hex, bool) {
	for _, d := range Dirs() {
		m := a.Step(d)
		if m.Step(d) == target {
			return m, true
		}
	}
	return Hex{}, false
}

// Retreat 是「退兵」：逃至鄰郡（說明書 p.34）。
//
// 「若被地形和敵軍完全包圍則逃不掉」「若退兵成功，原先擁有的錢糧都會損失」。
func (b *Battle) Retreat(u *Unit) error {
	if err := b.canAct(u); err != nil {
		return err
	}
	free := false
	for _, d := range Dirs() {
		h := u.At.Step(d)
		if b.Field.At(h).Passable() && b.UnitAt(h) == nil && b.Field.InBounds(h) {
			free = true
			break
		}
	}
	if !free {
		return fmt.Errorf("battle: 被完全包圍，逃不掉")
	}
	// 「若退兵成功，原先擁有的錢糧都會損失」（p.34）——帶走的是**自己那一份**。
	// 整個軍力的錢糧一次歸零的話，一支殘兵退走會讓還在打的友軍突然用不起計。
	share := u.Soldiers()
	total := 0
	for _, x := range b.Units {
		if x.Side == u.Side && x.Alive() {
			total += x.Soldiers()
		}
	}
	if total <= 0 || share >= total {
		b.Gold[u.Side], b.Rice[u.Side] = 0, 0
	} else {
		b.Gold[u.Side] -= b.Gold[u.Side] * share / total
		b.Rice[u.Side] -= b.Rice[u.Side] * share / total
	}
	u.Retreated = true
	b.note("blog.retreat", u.Name())
	b.checkOver()
	return nil
}

// EndDay 結束這一天：回填移動力、遞減狀態、判定勝負。
//
// **移動力是回填不是重設**（原版 `0x24ee1`–`0x24f0d`，`L0`）：
//
//	ax ← 上限（部隊記錄 offset 34）
//	剩下的（offset 36）比 ax 小才寫回去，大就留著
//
// 也就是 `剩下的 ← max(剩下的, 上限)`。休息多加的 2 因此**不會被砍掉**
// ——量到紮完寨的部隊上限 12、剩下 14，走了七天還是 14
// （`TestZZBattleDaySweep`）。無條件重設會把那 2 吃掉。
//
// 上限本身**整場只算一次**（`0x27114` 在佈陣時跑，日循環裡一次都沒有），
// 所以傷亡讓兵力變少不會讓部隊變慢。
func (b *Battle) EndDay() {
	for _, u := range b.Units {
		if !u.Alive() {
			continue
		}
		if cap := u.MovePoints(); u.Move < cap {
			u.Move = cap
		}
		if u.Trapped > 0 {
			u.Trapped--
		}
	}
	// 糧草（`0x25040`–`0x250c7`，`L0`）。兩件事分開，順序也分開：
	//
	//	每天：四個軍團各查一次，米 ≤ 0 → 逃亡（`0x25073`）
	//	天數 % 3 == 0：四個軍團各扣一次，量是軍團記錄 offset 14
	//	              ＝ 該軍團的兵士（百）（`0x2533f` 設、`0x250ad` 讀），
	//	              扣成負的歸 0（`0x250b9`）
	//
	// **三天扣一次不是每天**，而且**守方也扣**——原版的迴圈四個軍團
	// 都走（`0x2509c`：`cmpw $0x4`）。
	for _, s := range SideDeployOrder() {
		if !b.sideAlive(s) || b.Rice[s] > 0 {
			continue
		}
		// **逃亡是除不是減**（`0x25576`）：印「沒米了」（`DS:0x7d11`），
		// 然後這個軍團五支部隊、每支十個將領槽，每一位的兵
		// `÷= RND(2) + 2`——剩一半或三分之一。說明書 p.28 的
		// 「士兵將會陸續逃亡」一點都不含蓄。
		for _, u := range b.Units {
			if u.Side != s || !u.Alive() {
				continue
			}
			for j := range u.Leaders {
				x := &u.Leaders[j]
				if x.Dead || x.Captured || x.Soldiers <= 0 {
					continue
				}
				x.Soldiers /= b.roll(DesertionSpread) + DesertionFloor
			}
		}
		b.note("blog.starve", s.Label())
	}
	if b.Day%RiceUpkeepEvery == 0 {
		for _, s := range SideDeployOrder() {
			if !b.sideAlive(s) {
				continue
			}
			need := 0
			for _, u := range b.Units {
				if u.Side == s && u.Alive() {
					need += u.Soldiers() / 100
				}
			}
			if b.Rice[s] -= need; b.Rice[s] < 0 {
				b.Rice[s] = 0
			}
		}
	}
	b.Day++
	b.checkOver()
}

// checkOver 判定勝負（說明書 p.35）。
//
//   - 守方：堅持滿卅天且城池未被奪去，或攻方全滅／全撤退
//   - 攻方：攻下城池並堅持到卅天為止
func (b *Battle) checkOver() {
	if b.Over {
		return
	}
	attackers := b.sideAlive(MainAttacker) || b.sideAlive(AidAttacker)
	defenders := b.sideAlive(MainDefender) || b.sideAlive(AidDefender)
	atkChief := b.CommanderAlive(MainAttacker) || b.CommanderAlive(AidAttacker)
	defChief := b.CommanderAlive(MainDefender) || b.CommanderAlive(AidDefender)
	switch {
	// 統帥條件先於全滅條件，而且**守方那一條蓋過攻方那一條**：原版
	// `0x24f8c` 先寫「攻方統帥全滅 → 守方勝」再寫「守方統帥全滅 →
	// 攻方勝」，後者覆蓋前者，所以兩邊統帥都不在時判攻方勝。
	case !defChief && !b.Rules.DefenderCommanderLossIgnored:
		b.Over, b.AttackerWon = true, true
		b.note("blog.noDefChief")
	case !atkChief:
		b.Over, b.AttackerWon = true, false
		b.note("blog.noAttChief")
	// 底下兩個只有在守方統帥那一條被關掉時才走得到（加強版難度 11–20），
	// 對應的是加強版獨有的「總兵數為 0 者敗」（`0x229ba`）：打光守方的
	// 統帥不再算贏，打光守方的兵還是算。原版走不到這裡——兵打光了統帥
	// 也就不在了，上面那一條先成立。
	case !defenders:
		b.Over, b.AttackerWon = true, true
		b.note("blog.defZero")
	case !attackers:
		b.Over, b.AttackerWon = true, false
		b.note("blog.attZero")
	case b.Day >= BattleDays:
		b.Over = true
		b.AttackerWon = b.CityHolder().Attacking()
		if b.AttackerWon {
			b.note("blog.timeAtt")
		} else {
			b.note("blog.timeDef")
		}
	}
}

// CommanderAlive 回報這一方的統帥還在不在場上。
//
// 原版比的是「軍力記錄的統帥」與「該方第一支部隊的第一位將領」
// （`0x24f8c`）：死了、被俘了、部隊撤退或全滅了都會對不上。
// 這裡等價地問「他本人還在某一支還在場上的部隊裡」。
//
// 沒出場的一方（統帥 −1）回 false——**四種軍力不是每場都到齊**，
// 助攻軍與助守軍多半是空的，那一方的統帥當然不在陣中。
func (b *Battle) CommanderAlive(s Side) bool {
	id := b.Commander[s]
	if id < 0 {
		return false
	}
	for _, u := range b.Units {
		if u.Side != s || !u.Alive() {
			continue
		}
		for i := range u.Leaders {
			l := &u.Leaders[i]
			if l.Index == id && !l.Dead && !l.Captured {
				return true
			}
		}
	}
	return false
}

// CityHolder 是**此刻站在城池那一格**的那一方；沒有人就算守方。
//
// 三十天期滿的勝負看的是它，不是 CityHeld（原版 `0x25109`：
// 直接拿城池的座標去查「哪一格站著誰」那張地圖，空的話結果是 0，
// 也就是主守軍）。**兩者不一樣**：攻方進去又走掉的話，
// CityHeld 還記著攻方，而原版判守方贏。
func (b *Battle) CityHolder() Side {
	if u := b.UnitAt(b.Field.CityAt); u != nil && u.Alive() {
		return u.Side
	}
	return MainDefender
}

// Order 依行動順序回傳還在場上的部隊（說明書 p.28）。
func (b *Battle) Order() []*Unit {
	var out []*Unit
	for _, s := range SideActionOrder() {
		for _, f := range ActionOrder() {
			for _, u := range b.Units {
				if u.Side == s && u.Formation == f && u.Alive() {
					out = append(out, u)
				}
			}
		}
	}
	return out
}
