package battle

import (
	"fmt"
	"math/big"

	"github.com/wicanr2/softworld_san1_remake/internal/assets"
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

	// Speeches 是還沒畫的對白（`speech.go`），畫面那一層一格一格收。
	Speeches []Speech

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

	// base 是各方編隊時的基準格（主攻軍是入口、守方是城池），
	// 重編（`Reform`）要照同一個基準重排。
	base [sideCount]Hex

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

	// Computer 標記哪幾方是電腦在操縱（諸侯記錄 offset 0 == 2）。原版在
	// 被擒處置（`0x25a87`）與退兵的去處（`0x23fcf`）都先問這一格：
	// 電腦當場擲骰決定，玩家出選單問人。
	Computer [sideCount]bool

	// Renown 是各方勢力的人望（諸侯記錄 offset 8），招降判定要
	// （`0x25f02`）。
	Renown [sideCount]int

	// Origin 是各方出兵的郡（軍力記錄 offset 18）與它的現役武將數：
	// 電腦的助守軍與攻方退兵時，原郡還裝得下（現役 ＋ 將領 ≤ 50）就
	// 直接回去，不擲骰挑鄰郡（`0x23fd7`–`0x2400f`）。主守軍不看這一格。
	// Prefecture 為 0 表示沒填。
	Origin [sideCount]Escape

	// PlayerSkirmish 是對戰子畫面裡玩家那一方將領的介面（`SkirmishPlayer`）。
	// 電腦向玩家的部隊發動對戰時（`0x2deb0` 由選項 7 進入）從這裡問；
	// nil 就照電腦的判斷式走。
	PlayerSkirmish SkirmishPlayer
	// PlayerDuelAnswer 是玩家那一方的將領在子畫面裡被叫陣時的答案
	// （`SkirmishAnswer`）；nil 就當接受。
	PlayerDuelAnswer SkirmishAnswer

	// PlayerCaptive 是玩家捕獲時的「1.斬首 2.囚禁 3.釋放 4.招降」
	// （`0x25af0`，`docs/spec/018` R1）：回四種處置之一，被拒絕（君主不能
	// 囚禁、在野滿 50、招降不從）就再問一次。nil 時玩家捕獲的人留
	// `FateNone`，戰後由戰略層處理。
	PlayerCaptive func(captor Side, x *Leader) Fate
	// PlayerCamp 是玩家那一方的空部隊收到第一位將領時問紮在哪
	// （`0x2731a`）；回的格子紮不了就再問。nil 時由 `spotNear` 擺。
	PlayerCamp func(u *Unit) Hex
	// Host 給被擒處置問戰場郡的兩件事（`CaptiveHost`）；nil 時在野永遠有
	// 空位、釋放沒有去處。
	Host CaptiveHost

	// inSkirmish 是正在跑的對戰子畫面（`Skirmish.Run` 期間），對白要記
	// 那兩支部隊（`Speech.Units`）。
	inSkirmish *Skirmish
	// helperSaid 記助軍回郡那一句說過了（`SayHelperReturn`）。
	helperSaid bool

	rng    *rand
	rollFn func(int) int
}

// Setup 是開一場戰役要的東西。
type Setup struct {
	Field   *Field
	Weather Weather
	Seed    uint32

	// FixedWeather 為真就用 Weather 的值開場；否則照原版在佈陣完擲
	// `RND(3)` 決定（`0x23b78`，`L0`）。對拍從原版讀天候時用前者。
	FixedWeather bool

	// Attackers／Defenders 是各方的將領，會被分成五種隊伍。
	Attackers []Leader
	Defenders []Leader

	// AidAttackers／AidDefenders 是助攻軍與助守軍，可以是空的。
	AidAttackers []Leader
	AidDefenders []Leader

	// Rules 是版本規則（`RulesFor`）。零值是原版。
	Rules Rules

	// AI、Difficulty、Escapes、Computer、Renown、Origin 見 Battle 的同名欄位。
	AI         AI
	Difficulty int
	Escapes    [sideCount][]Escape
	Computer   [sideCount]bool
	Renown     [sideCount]int
	Origin     [sideCount]Escape

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
		AI: s.AI, Difficulty: s.Difficulty, Escapes: s.Escapes,
		Computer: s.Computer, Renown: s.Renown, Origin: s.Origin}
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
		b.base[side] = base
		b.Units = append(b.Units, b.formUp(side, pool, base)...)
	}
	if !s.FixedWeather {
		b.Weather = weatherFromOriginal(b.roll(WeatherKinds))
	}
	b.note("blog.start", b.Weather.Label())
	return b
}

// 天候的骰（`L0`）：開場 `RND(3)`（`0x23b78`）；之後每天結束
// `RND(10) > 5` 才重擲 `RND(3)`（`0x26f57`–`0x26f74`），否則沿用。
// 擲出來的是原版的編號（0 晴、1 雨、2 風），`weatherFromOriginal`
// 對回列舉。
const (
	WeatherKinds       = 3
	WeatherChangeRange = 10
	WeatherChangeAbove = 5
)

// weatherFromOriginal 把原版的天氣編號換成列舉（`OriginalIndex` 的反向）。
func weatherFromOriginal(i int) Weather {
	switch i {
	case 1:
		return Rainy
	case 2:
		return Windy
	}
	return Clear
}

// EndTurn 是一支部隊行動之後、原版回合常式（`0x24cf6`）收尾的兩件事：
// 陣前投敵的判定（`defections`），再把移動力回填到上限
// （`0x24ee1`–`0x24f0d`：`剩下的 ← max(剩下的, 上限)`）。
//
// **回填是每一支自己的回合結束時做，不是一天結束時**，所以一支部隊
// 行動完就滿血，休息多加的 2 也因此留得住（量到上限 12、剩下 14，
// `TestZZBattleDaySweep`）。上限本身整場只算一次（`0x27114`）。
//
// 退了兵的部隊不算：原版退兵時把將領搬回郡裡，部隊記錄的將領數歸零，
// `0x27604` 進去就回頭。
func (b *Battle) EndTurn(u *Unit) {
	if u == nil || u.Retreated {
		return
	}
	if u.LeaderCount() > 0 {
		b.defections(u)
	}
	if cap := u.moveCap(); u.Move < cap {
		u.Move = cap
	}
}

// SkipTrappedTurn 是中了陷阱的部隊輪到時原版做的事（`0x24e71`–`0x24e94`）：
// 印一句「不能動」、倒數減一，**不判投敵**，直接回填移動力。
func (b *Battle) SkipTrappedTurn(u *Unit) {
	if u == nil || u.Trapped <= 0 {
		return
	}
	u.Trapped--
	if cap := u.moveCap(); u.Move < cap {
		u.Move = cap
	}
}

// 陣前投敵（`0x27604`，`L0`）的常數。
const (
	// DefectionSpread：每一位（統帥除外）擲 `RND(5)`，擲到 0 才往下判。
	DefectionSpread = 5
)

// defections 是一支部隊回合結束時的投敵判定（`0x27604`，`L0`）。
// 從第 9 槽往第 0 槽走，統帥（軍力記錄 offset 0）不判：
//
//	RND(5) != 0 → 不判
//	對方主軍勢力的人望 ÷ 2 > 這一位的忠誠 → 往下（在野的忠誠是 0xFF，讀成 −1）
//	對方主軍的將領數 ≥ 50 → 不判
//	→ 印一句對白（RND(8)），帶著兵離隊，投進對方主軍最後一支還有位子的
//	  部隊（`0x25cd2`），忠誠變成 min(100, 100 − 原忠誠)
//
// 「對方主軍」是守方兩軍看主攻軍、攻方兩軍看主守軍（`0x27642`）。
func (b *Battle) defections(u *Unit) {
	opp := MainDefender
	if !u.Side.Attacking() {
		opp = MainAttacker
	}
	for i := len(u.Leaders) - 1; i >= 0; i-- {
		x := &u.Leaders[i]
		if !x.InUnit() || x.Index == b.Commander[u.Side] {
			continue
		}
		if b.roll(DefectionSpread) != 0 {
			continue
		}
		if b.Renown[opp]/2 <= x.Loyalty {
			continue
		}
		if b.leadersOf(opp) >= SurrenderRoom {
			continue
		}
		b.say(x, BoxThird, false, "bub.desert") // `0x27768`
		loyalty := 100 - x.Loyalty
		if loyalty > 100 {
			loyalty = 100
		}
		x.Deserted = true
		x.Fate = Defected
		x.CapturedBy = opp
		b.enlist(opp, x, loyalty, x.Soldiers)
		b.note("blog.deserted", pn(x.Name), opp.Label())
	}
	b.wipeCheck(u)
}

// formUp 把一批將領分成五種隊伍並紮營。
//
// 「如果派出全部兵力，各戰鬥組的將領人數必須平均分配」「每組最多 10 名將領」
// （說明書 p.27）。
//
// **電腦的那一方照原版的整編填**（`0x23789`–`0x23886`，`L0`、`[both]`）：
// n 位將領，每支 (n−1)÷5 位，前 n − 5×那個數 支多一位，**照清單的順序
// 依序填進第 1–5 支**——清單的順序是呼叫端定的（主攻軍是洗過牌再留守
// 的那一份，主守軍是郡的守將清單）。玩家那一方原版是逐位問「分到那一軍」，
// remake 這裡照戰力排序再輪流分配，中軍先分到最強的。
func (b *Battle) formUp(side Side, pool []Leader, base Hex) []*Unit {
	// **沒有介面的那一方照電腦的整編填**（Issue #98 的裁定）：月度對拍、
	// 示範模式、批次跑都走這裡，沒有人可以逐位分配。原版在這一刻是問人，
	// 所以這是登記在案的 remake 差異，不是「對回原版」。
	n := len(pool)
	per, rem := (n-1)/5, n-((n-1)/5)*5
	var counts []int
	for i := 0; i < int(formationCount); i++ {
		c := per
		if i < rem {
			c++
		}
		if c > 0 {
			counts = append(counts, c)
		}
	}
	groups := make([][]Leader, 0, len(counts))
	k := 0
	for _, c := range counts {
		groups = append(groups, append([]Leader(nil), pool[k:k+c]...))
		k += c
	}
	return b.buildUnits(side, groups, base)
}

// buildUnits 把分好的隊伍變成部隊：依 `DeployOrder` 給番號、算箭、
// 從基準格往外紮營、記下開戰時的兵力與綜合能力。空的隊伍不佔番號。
func (b *Battle) buildUnits(side Side, groups [][]Leader, base Hex) []*Unit {
	order := DeployOrder()
	units := make([]*Unit, 0, len(groups))
	for i, g := range groups {
		if len(g) == 0 || i >= len(order) {
			continue
		}
		units = append(units, &Unit{Side: side, Formation: order[i], Leaders: g})
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
		u.Cap = u.MovePoints()
		u.Move = u.Cap
		u.Started = u.Soldiers()
		u.Quality = u.Ability() // 整編寫進 offset 32 的值；每天輪到之前再重算
		spot = base.Step(Dirs()[i%6])
	}
	return units
}

// Reform 把一方的部隊照玩家的整編重編（原版 `0x20a30` 的
// 「分配那一位將軍」→「將%s分到那一軍」迴圈，Issue #98）。
//
// groups 與這一方**現在的將領**一一對應（照部隊、部隊內的順序攤平），
// 值 1–5 是要分到第幾軍（`DeployOrder` 的序），**0 表示不出征**——
// 原版沒有被分配到的人留在家裡（`0x20c9a` 只把編進部隊的人的所在郡寫 0）。
//
// 只能在開戰前用：日迴圈跑起來之後重編等於憑空搬動部隊。
func (b *Battle) Reform(side Side, groups []int) error {
	if b.Day != 1 {
		return fmt.Errorf("battle: 整編只在開戰前")
	}
	var pool []Leader
	for _, u := range b.Units {
		if u.Side == side {
			pool = append(pool, u.Leaders...)
		}
	}
	if len(groups) != len(pool) {
		return fmt.Errorf("battle: 整編給了 %d 個位置，這一方有 %d 位將領",
			len(groups), len(pool))
	}
	picked := make([][]Leader, int(formationCount))
	count := 0
	for i, g := range groups {
		if g == 0 {
			continue
		}
		if g < 1 || g > int(formationCount) {
			return fmt.Errorf("battle: 第 %d 位分到第 %d 軍，只有 1–%d 軍",
				i+1, g, formationCount)
		}
		if len(picked[g-1]) >= MaxLeaders {
			return fmt.Errorf("battle: 第 %d 軍已經滿 %d 位", g, MaxLeaders)
		}
		picked[g-1] = append(picked[g-1], pool[i])
		count++
	}
	if count == 0 {
		return fmt.Errorf("battle: 一位都沒有分配")
	}
	kept := b.Units[:0]
	for _, u := range b.Units {
		if u.Side != side {
			kept = append(kept, u)
		}
	}
	b.Units = append(kept, b.buildUnits(side, picked, b.base[side])...)
	// **統帥跟著換**：勝負判定拿 `Commander` 與「該方第一支部隊的第一位
	// 將領」比（`0x24f8c`），對不上就當成統帥不在了。
	for _, u := range b.Units {
		if u.Side == side {
			b.Commander[side] = u.Leaders[0].Index
			break
		}
	}
	return nil
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
//
// 玩家的休息印一句對白（`0x27cb0` → `RND(8)`）；電腦的休息（選項 9，
// `0x29e2e`）不印，那一支在 `baseRest`。
func (b *Battle) Rest(u *Unit) error {
	if err := b.canAct(u); err != nil {
		return err
	}
	b.sayUnit(u, "bub.camp") // `0x27cb0`
	u.Move += RestMove
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
		if !x.InUnit() || x.Soldiers <= 0 {
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
//
// **骰序**（`docs/re/05` §12.2，`L0`＋`L1`）：進來先印一句對白
// （`0x2a2b6` → `RND(8)`），算完雙方的殺傷、逐將領扣完兵，最後把兩邊
// 打光的將領交給對方處置（`0x2a7b7`／`0x2a7e6`）——**先出手方的、再
// 承受方的**，各照槽位由 0 往 9。處置本身的骰見 `capture`。
func (b *Battle) exchange(a, d *Unit, mode int) (int, int) {
	b.sayUnit(a, "bub.kill") // `0x2a2b6`
	// 部隊的兵士數（offset 30）是 **16 位元有號數**（`fimuls`／`fidivrs`）：
	// 兩位各兩萬七的部隊在這裡是負的，比例那一步 `兵 ≤ 0 → 0.0`，
	// 只剩逐將領那道 −1（盤面丙量到，`L1`）。
	sa, sd := s16(a.Soldiers()), s16(d.Soldiers())
	da := MeleeDamage(b.meleeAttackValue(b.Field.At(a.At)),
		sa, a.Quality, StrikeMultiplier(mode), MeleeAttackScale)
	dd := MeleeDamage(b.meleeDefendValue(b.Field.At(d.At)),
		sd, d.Quality, 1, MeleeDefendScale)
	ra, rd := MeleeRatio(dd, sa), MeleeRatio(da, sd)
	wasA, wasD := a.Soldiers(), d.Soldiers()
	fellA := b.thin(a, ra, false)
	// **承受方的第一位在出手方打光時留下**（`0x2a684`–`0x2a68e`，`L0`）：
	// 出手方的將領數（迴圈裡逐一遞減）歸零之後，承受方第 0 槽那一位
	// 兵扣到 0 也不除名——兩支不會在同一次結算裡一起消失。
	fellD := b.thin(d, rd, a.LeaderCount() == 0)
	for _, i := range fellA {
		b.capture(d.Side, a, &a.Leaders[i])
	}
	for _, i := range fellD {
		b.capture(a.Side, d, &d.Leaders[i])
	}
	return wasD - d.Soldiers(), wasA - a.Soldiers()
}

// MessageLines 是戰場對白表每一格的句數（`DS:0x9a32`，`docs/re/05`
// §11）：對白常式 `0x3273e` 每次進去都擲一次 `RND(8)` 挑句子
// （`0x32d4d`，沒有提早返回的路，`L0`）。remake 的戰報是固定句，
// 這一擲只為了與原版同序同數。
const MessageLines = 8

// EffectVariants：特效常式 `0x32dfa` → `0x32e40` 進去先擲 `RND(4)`
// （`0x32e4f`，`L0`）挑拉幕的方向，把場景圖拉進來（`docs/spec/010`）。
const EffectVariants = 4

// msg 是原版印一句戰場對白（`0x3273e`）時的那一擲。
func (b *Battle) msg() int { return b.roll(MessageLines) }

// fx 是原版播一段特效（`0x32dfa`）時的那一擲。
func (b *Battle) fx() int { return b.roll(EffectVariants) }

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
//
// 回傳這一次打光、被俘的將領在 `Leaders` 裡的索引，**由小到大**——
// 原版把它們記在一張照槽位排的表，處置時從第 0 格往後走。
//
// keepFirst 為真時第一位還在隊上的將領兵扣到 0 也留下（見 `exchange`）。
// 留下的那一位兵是 0，下一次結算 `ftol(0 × …) − 1` 還是 ≤ 0，
// 到時再被俘——所以這裡**不跳過兵已經是 0 的將領**。
func (b *Battle) thin(u *Unit, ratio float64, keepFirst bool) []int {
	first := -1
	for i := range u.Leaders {
		if u.Leaders[i].InUnit() {
			first = i
			break
		}
	}
	var fell []int
	for i := len(u.Leaders) - 1; i >= 0; i-- {
		x := &u.Leaders[i]
		if !x.InUnit() {
			continue
		}
		n := MeleeSurvivors(s16(x.Soldiers), ratio)
		if n == 0 && !(keepFirst && i == first) {
			x.Captured = true
			fell = append(fell, i)
			b.note("blog.captured", pn(x.Name))
		}
		x.Soldiers = n
	}
	for i, j := 0, len(fell)-1; i < j; i, j = i+1, j-1 {
		fell[i], fell[j] = fell[j], fell[i]
	}
	b.wipeCheck(u)
	return fell
}

// wipeCheck 把沒有將領了的部隊標成全滅（原版：將領人數 0 的部隊從
// 佔位圖上消失、格子畫回地形，`0x2a732`／`0x2a790`）。
func (b *Battle) wipeCheck(u *Unit) {
	if u.LeaderCount() == 0 && !u.Wiped {
		u.Wiped = true
		b.note("blog.wiped", u.Name())
	}
}

// noInterfaceCaptives 數「玩家那一方抓到人、而沒有介面可問」的次數
// （Issue #100）。**只為了量**：改用電腦的判斷式會多擲一次 `RND(10)`，
// 先數清楚哪些盤面真的會走到這裡，再決定動不動
// （`~/diagnosis-notes/docs/03-silence-is-not-success`：沒有紅不等於沒影響）。
var noInterfaceCaptives int

// NoInterfaceCaptives 是目前數到幾次；ResetNoInterfaceCaptives 歸零。
func NoInterfaceCaptives() int      { return noInterfaceCaptives }
func ResetNoInterfaceCaptives()     { noInterfaceCaptives = 0 }

// 被擒處置（`0x259fe`，`L0`；`docs/re/05` §12.2）裡的常數。
const (
	// CaptiveExecuteRange／CaptiveExecuteBelow：電腦捕獲方先擲 `RND(10)`，
	// 小於 2 是斬首、否則囚禁（`0x25a93`–`0x25aa8`）；招降判定過了再改成
	// 招降，君主一律改回斬首。
	CaptiveExecuteRange = 10
	CaptiveExecuteBelow = 2
	// SurrenderRoom：捕獲方那個軍力的將領數到 50 就不招降（`0x25e82`）。
	SurrenderRoom = 50
	// SurrenderBondWeight：牽絆對象在同一勢力，門檻加 1000（`0x25ed6`）
	// ——等於招不動。
	SurrenderBondWeight = 1000
	// SurrenderSpread：門檻除以 `RND(3) + 1`（`0x25edf`）。
	SurrenderSpread = 3
)

// CaptiveHost 是被擒處置要問戰場那一郡的兩件事（`docs/spec/018` R2）。
// 戰術層不知道州郡表，由 `game.Pending` 實作。
type CaptiveHost interface {
	// IdleRoom 回報戰場那一郡的在野數還不到 50（州郡 offset 23，
	// `0x26118`／`0x262d7`），含這一場已經囚禁或釋放進去的人。
	IdleRoom() bool
	// AddIdle 記一位進了戰場那一郡的在野（`0x26267`／`0x264fc`）。
	AddIdle()
	// ReleaseTo 是釋放的去處（`0x265ac`）：他勢力裡現役未滿 50 的郡，
	// 沒有就無主的郡，扣掉戰場郡，`roll(清單長)` 挑一個；清單空回 −1、
	// 不擲。
	ReleaseTo(general int, roll func(int) int) int
}

// Capture 是 `capture` 的公開入口：對拍直接呼叫原版的 `0x259fe` 時，
// remake 這一邊從這裡進同一段處置。
func (b *Battle) Capture(captor Side, u *Unit, x *Leader) {
	x.Captured = true
	b.capture(captor, u, x)
}

// capture 是一位將領被 `captor` 這一方擒住之後的處置（`0x259fe`，
// `docs/spec/018` §1）。
//
// **電腦捕獲**（`0x25a87`–`0x25ae8`，`L0`）：
//
//	RND(10) < 2 → 斬首，否則囚禁
//	招降判定（surrenderChance）> 0 → 招降
//	被擒的是君主 → 斬首
//
// **玩家捕獲**問 `PlayerCaptive`；沒有就留給戰略層。四種處置各自可能
// 拒絕（`dispose` 回 false），拒絕就從頭再決定一次——電腦重擲 `RND(10)`，
// 玩家再問一次。
func (b *Battle) capture(captor Side, u *Unit, x *Leader) {
	x.CapturedBy = captor
	// **沒有介面時照電腦的判斷式處置**（Issue #100 的裁定）：示範模式、
	// 月度對拍、批次跑，以及「守城」開關關著的那一條，都沒有人可以答。
	// 先前那裡直接 `return`，那一位留 `FateNone`——被抓了卻什麼都沒發生，
	// **原版沒有這個狀態**（它在同一刻是問人）。
	//
	// ⚠ 這是**登記在案的 remake 差異，不是「對回原版」**：原版問人、不擲骰，
	// 這裡改用電腦那一套會多擲一次 `RND(10)`（`docs/spec/018` §R5）。
	// 動之前量過：六張逐日對拍盤面與三十六個月的無畫面局都**一次都沒走到**
	// 這一條（計數器 `noInterfaceCaptives`，正對照見
	// `TestNoInterfaceCaptiveCounterFires`），所以既有對拍的骰序不受影響。
	human := !b.Computer[captor] && b.PlayerCaptive != nil
	if !b.Computer[captor] && b.PlayerCaptive == nil {
		noInterfaceCaptives++
	}
	for {
		var fate Fate
		if human {
			fate = b.PlayerCaptive(captor, x)
		} else {
			fate = Jailed
			if b.roll(CaptiveExecuteRange) < CaptiveExecuteBelow {
				fate = Executed
			}
			if b.surrenderChance(captor, x) > 0 {
				fate = Defected
			}
			if x.Lord {
				fate = Executed
			}
		}
		if b.dispose(captor, x, fate) {
			return
		}
	}
}

// dispose 執行一種處置（`docs/spec/018` §2）；回 false 表示原版回 −1、
// 要重新決定。
func (b *Battle) dispose(captor Side, x *Leader, fate Fate) bool {
	idleRoom := b.Host == nil || b.Host.IdleRoom()
	switch fate {
	case Executed:
		b.say(x, BoxThird, false, "bub.captiveDie") // `0x25fcb`
		x.Fate = Executed
		b.note("blog.executed", pn(x.Name))
		return true
	case Jailed:
		// 君主不能囚禁、戰場郡在野滿 50 也不行（`0x260f4`／`0x26118`）。
		if x.Lord || !idleRoom {
			return false
		}
		b.say(x, BoxThird, false, "bub.captiveJailed") // `0x26178`
		b.scene(assets.SceneJail)                      // `0x261a1`
		x.Fate = Jailed
		if b.Host != nil {
			b.Host.AddIdle()
		}
		b.note("blog.jailed", pn(x.Name))
		return true
	case Released:
		if !idleRoom {
			return false // `0x262d7`
		}
		to := -1
		if b.Host != nil {
			to = b.Host.ReleaseTo(x.Index, b.roll) // `0x265ac`，對白之前擲
		}
		b.say(x, BoxThird, false, "bub.captiveRelease") // `0x2635a`
		b.scene(assets.SceneDismiss)                    // `0x26383`
		x.Fate, x.ReleasedTo = Released, to
		if to < 0 && b.Host != nil {
			b.Host.AddIdle()
		}
		b.note("blog.released", pn(x.Name))
		return true
	case Defected:
		c := b.surrenderChance(captor, x)
		if c <= 0 {
			b.say(x, BoxThird, false, "bub.captiveRefuse", pn(x.Name)) // `0x25c0e`
			return false
		}
		b.say(x, BoxThird, false, "bub.captiveYield") // `0x25c82`
		b.scene(assets.SceneJoin)                     // `0x25cae`
		x.Fate = Defected
		x.Loyalty = c
		b.enlist(captor, x, c, 0)
		b.note("blog.defected", pn(x.Name), captor.Label())
		return true
	}
	return false
}

// surrenderChance 是招降判定（`0x25e50`，`L0`）：回 0 表示不招降，
// 否則是招降之後的忠誠。
//
//	君主 → 0
//	捕獲方軍力的將領數 ≥ 50 → 0
//	門檻 ＝ max(戰力, 謀略)；牽絆對象同勢力再 ＋1000
//	門檻 ÷= RND(3) + 1
//	捕獲方的人望 < 門檻 → 0
//	忠誠 ＝ ftol((100 − 忠誠 ÷ 2) × 人望 × 0.01)，夾 0..100
//
// 最後那一步用整數算：`0.01` 的 double 比百分之一略大，整數乘積截尾
// 的結果與 `÷ 100` 相同。
func (b *Battle) surrenderChance(captor Side, x *Leader) int {
	if x.Lord {
		return 0
	}
	if b.leadersOf(captor) >= SurrenderRoom {
		return 0
	}
	best := int(x.War)
	if int(x.Intel) > best {
		best = int(x.Intel)
	}
	if x.BondAlly {
		best += SurrenderBondWeight
	}
	best /= b.roll(SurrenderSpread) + 1
	if b.Renown[captor] < best {
		return 0
	}
	c := (100 - x.Loyalty/2) * b.Renown[captor] / 100
	if c > 100 {
		c = 100
	}
	if c < 0 {
		c = 0
	}
	return c
}

// leadersOf 是一個軍力還在隊上的將領總數（軍力記錄 offset 12）。
func (b *Battle) leadersOf(s Side) int {
	n := 0
	for _, u := range b.Units {
		if u.Side == s {
			n += u.LeaderCount()
		}
	}
	return n
}

// enlist 把招降來的將領放進捕獲方的部隊（`0x25cd2`，`L0`）：五支部隊
// 由前往後看，**最後一支**將領數不到 10 的收下他，兵是 0。
//
// 原版把空著的部隊收到第一位將領時會重新初始化那支部隊
// （`0x2731a`／`0x26fc6`／`0x270f8`／`0x27226`）；remake 只把人放進去，
// 那四支還沒讀（`docs/re/05` §12.2 的未解）。
//
// **五個槽位都算**，空的槽位將領數是 0，所以正常都落在後軍——後軍不存在
// 就生一支。生出來的那一支照原版重新初始化：紮寨（`0x2731a`，玩家的
// 出提示問位置、這裡先放在同一方部隊旁邊的空格，`Unplaced` 標著給畫面
// 或對拍改）、綜合能力（`0x26fc6`）、移動力上限（`0x270f8`）、箭（`0x27226`）。
func (b *Battle) enlist(captor Side, x *Leader, loyalty, soldiers int) {
	var into *Unit
	var form Formation
	for _, f := range DeployOrder() {
		u := b.unitSlot(captor, f)
		if u == nil || u.LeaderCount() < MaxLeaders {
			into, form = u, f
		}
	}
	y := *x
	y.Captured, y.Dead, y.Deserted = false, false, false
	y.Fate, y.CapturedBy = Defected, captor
	y.Loyalty, y.Soldiers = loyalty, soldiers
	if into == nil {
		into = &Unit{Side: captor, Formation: form, At: b.spotNear(captor), Unplaced: true}
		b.Units = append(b.Units, into)
	}
	into.Leaders = append(into.Leaders, y)
	if into.LeaderCount() == 1 {
		// 空部隊收到第一位：紮寨（`0x2731a`）。玩家那一方問 `PlayerCamp`，
		// 挑到紮得下的格子為止；電腦那一方新生的照 `spotNear`（R5）。
		if !b.Computer[captor] && b.PlayerCamp != nil {
			for {
				if at := b.PlayerCamp(into); b.CampArea(into, at) {
					into.At, into.Unplaced = at, false
					break
				}
			}
		}
		into.Wiped, into.Retreated = false, false
		b.RefreshQuality(into)
		into.Cap = into.MovePoints()
		into.Move = into.Cap
		into.Arrows = ArrowCount(into.Leaders)
		into.Started = into.Soldiers()
	}
}

// unitSlot 找某一方的某一隊，不管在不在場上；沒有回 nil。
func (b *Battle) unitSlot(s Side, f Formation) *Unit {
	for _, u := range b.Units {
		if u.Side == s && u.Formation == f {
			return u
		}
	}
	return nil
}

// spotNear 找一格給新生的部隊：同一方還在場上的部隊旁邊第一個走得進去
// 的空格；一支都沒有就用城池或入口。原版是問玩家（`0x2731a`）。
func (b *Battle) spotNear(s Side) Hex {
	for _, u := range b.Units {
		if u.Side != s || !u.Alive() {
			continue
		}
		for _, d := range Dirs() {
			h := u.At.Step(d)
			if b.Field.InBounds(h) && b.Field.At(h).Passable() && b.UnitAt(h) == nil {
				return h
			}
		}
	}
	if s.Attacking() {
		return FromOffset(0, FieldH/2)
	}
	return b.Field.CityAt
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
		if !x.InUnit() || x.Soldiers == 0 {
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
	// 兵打光就被俘（原版 0x30716）。
	for i := range u.Leaders {
		x := &u.Leaders[i]
		if x.Soldiers <= 0 && x.InUnit() {
			x.Captured = true
			b.note("blog.captured", pn(x.Name))
		}
	}
	b.wipeCheck(u)
}

// QuickBattle 是「快戰」：雙方直接正面作戰（說明書 p.32）。玩家的
// 快戰把移動力歸零（`0x280e1`）。
func (b *Battle) QuickBattle(a *Unit, d Dir) error {
	return b.meleeMode(a, d, MeleeStrike, false, true)
}

// DeathBattle 是「死戰」：一決生死的激戰，**雙方將互戰至分出勝負為止**
// （說明書 p.32）。
func (b *Battle) DeathBattle(a *Unit, d Dir) error {
	return b.meleeMode(a, d, MeleeStrike, true, true)
}

// meleeMode 是快戰／死戰加上交戰結算的模式（倍率格，`StrikeMultiplier`）：
// 玩家的對戰傳 8（夾成 1），電腦的快戰傳 難度÷5＋1、死戰傳 難度÷5
//（`0x29b13`／`0x29d33`）。
//
// spend 為真才把移動力歸零：玩家的快戰與死戰、電腦的死戰會（`0x280e1`、
// `0x28822`、`0x29dc7`），**電腦的快戰不會**（`0x29ade` 不碰 offset 36）
// ——差別在回合結束回填是 `max(剩下的, 上限)`，休息攢的 2 留不留得住。
//
// 死戰打到**一方沒有將領**為止（`0x29d7d`／`0x29da8` 看 offset 28）。
func (b *Battle) meleeMode(a *Unit, d Dir, mode int, toTheDeath, spend bool) error {
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
	for i := 0; ; i++ {
		// **同時**：原版先把雙方的殺傷都算出來，再各自扣兵
		// （`0x30618`／`0x306bb`）。先扣一邊再算另一邊的話，
		// 先手會佔到不該有的便宜。
		// 主戰場的交戰走 `0x2a224`（`docs/re/05` §3.6）。量到玩家的
		// 「對戰」傳的模式是 8——落在 0..7 之外，被 `0x2a2c9` 夾成 1，
		// 也就是倍率 100。
		la, lb := b.exchange(a, t, mode)
		if t.LeaderCount() == 0 {
			b.note("blog.rout", a.Name(), t.Name(), la)
			break
		}
		if a.LeaderCount() == 0 {
			b.note("blog.counter", t.Name(), lb)
			break
		}
		if !toTheDeath {
			b.note("blog.clash", a.Name(), t.Name(), la, lb)
			break
		}
		if i >= 50 { // 兩邊都打不動對方時的保險，原版沒有這一道
			break
		}
	}
	if spend {
		a.Move = 0
	}
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
	//	（加強版的尺度是 5e-5，`Rules.ArrowHalfScale`）
	//	比例 ＝ 殺傷 ÷ 目標.兵士數
	//	逐將領：新兵 ＝ max(0, ftol(兵 × (1 − 比例)))
	//
	// 骰序（`0x2a80a`，`L0`）：每一箭先印一句對白（`0x2a88e` → `RND(8)`）、
	// 播一段特效（`0x2a8c8` → `RND(4)`），再算殺傷；殺傷本身不擲骰。
	// 兵被射光的將領**不被俘**——這一支沒有交戰結算那段處置。
	total := 0
	for i := 0; i < n; i++ {
		if !t.Alive() {
			break
		}
		b.sayUnit(a, "bub.arrows")  // `0x2a88e`
		b.scene(assets.SceneArrows) // `0x2a8c8`
		d := MeleeDamage(ArrowTerrainValue(b.Field.At(a.At)),
			s16(a.Soldiers()), a.Quality, 1, b.arrowScale())
		r := MeleeRatio(d, s16(t.Soldiers()))
		was := t.Soldiers()
		for j := range t.Leaders {
			x := &t.Leaders[j]
			if !x.InUnit() || x.Soldiers <= 0 {
				continue
			}
			x.Soldiers = ArrowSurvivors(s16(x.Soldiers), r)
		}
		total += was - t.Soldiers()
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
	// 骰序（`0x23dd4`，`L0`）：列完逃得去的鄰郡先印一句對白（`0x23fa1` →
	// `RND(8)`）；電腦的部隊接著挑去處——主守軍以外的軍力，原郡還裝得下
	// （現役 ＋ 本隊將領 ≤ 50）就直接回原郡，否則 `RND(鄰郡數)` 挑一郡
	// （`0x2401b`）；玩家出選單問人，不擲。走出去的動畫播一段特效
	// （`0x24395` → `RND(4)`）。
	//
	// 原版是挑完去處才找路，找不到印「逃不掉」回頭重挑；remake 先查
	// 有沒有路，查不到在擲骰之前就回錯——只有退不成的那一趟骰數不同。
	b.sayUnit(u, "bub.retreat") // `0x23fa1`
	if b.Computer[u.Side] {
		home := b.Origin[u.Side]
		if u.Side == MainDefender || home.Prefecture == 0 || u.LeaderCount()+home.Active > baseEscapeRoom {
			b.roll(len(b.Escapes[u.Side]))
		}
	}
	b.scene(assets.SceneRetreat) // `0x24395`
	// **軍力的錢糧不動**：退兵常式（`0x23dd4`–`0x24460`）一個字都沒碰
	// 軍力記錄的 offset 6／8（`L0`；盤面乙量到守方退了一支之後照樣
	// 用得起 400 金的誘敵）。說明書 p.34 的「原先擁有的錢糧都會損失」
	// 是整支軍力退光之後戰役結束時的事，在戰略層收尾。
	u.Retreated = true
	b.note("blog.retreat", u.Name())
	b.checkOver()
	return nil
}

// EndDay 結束這一天：糧草、換日、天候、判定勝負。
func (b *Battle) EndDay() {
	// 移動力的回填與陷阱的倒數都在各部隊自己的回合（`EndTurn`／
	// `SkipTrappedTurn`），這裡只剩糧草、換日與天候。
	//
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
				if !x.InUnit() || x.Soldiers <= 0 {
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
	// 三十天期滿（`0x250d4`）在**這一天的部隊都動完之後、天數加一之前**判
	// （`0x23c3b` 先 `call 0x250d4` 再 `incw 0x2100`；加強版 `0x22aae` 同）
	// ——所以第 30 天是打完整的一天，判定看的是第 30 天結束時誰站在城池。
	// 天數留在 30 給戰報用；原版的計數器之後還會加到 31，但那時勝方
	// 已經定了，沒有人再讀它。
	if !b.Over && b.Day >= BattleDays {
		b.timeUp()
		return
	}
	b.Day++
	b.checkOver()
	// 天候每天重擲一次（`0x26f48`）：`RND(10) > 5` 才換，換成 `RND(3)`。
	if b.roll(WeatherChangeRange) > WeatherChangeAbove {
		b.Weather = weatherFromOriginal(b.roll(WeatherKinds))
	}
}

// timeUp 是三十天期滿的判定（`0x250d4`，說明書 p.35）：看**此刻**誰站在
// 城池那一格，空著算守方。
func (b *Battle) timeUp() {
	b.Over = true
	b.AttackerWon = b.CityHolder().Attacking()
	if b.AttackerWon {
		b.note("blog.timeAtt")
	} else {
		b.note("blog.timeDef")
	}
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
	}
	// 三十天期滿不在這裡：它只在一天結束時判（`EndDay` → `timeUp`），
	// 第 30 天當天的行動不受它影響。
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
