package game

import (
	"errors"
	"fmt"

	"github.com/wicanr2/softworld_san1_remake/internal/assets"
	"github.com/wicanr2/softworld_san1_remake/internal/state"
)

// Order 是一個「要下的命令」，做成資料而不是直接呼叫方法。
//
// 理由是**電腦 AI 要能被檢查**：AI 產出一串 Order，可以先印出來、
// 存起來、與原版的行為逐項比對，然後才套用。直接呼叫方法的話
// 「AI 打算做什麼」與「做了什麼」分不開，對拍就無從下手。
type Order interface {
	// Apply 把命令套用到局面上。條件不滿足時回錯誤，**不會部分套用**。
	Apply(g *State, by state.FactionID) error

	// Describe 是給人看的一行說明，對拍紀錄與除錯都靠它。
	Describe(g *State) string

	// Prefecture 是這個命令下在哪個郡。每郡每月一次的限制靠它判斷。
	Prefecture() int
}

// ReclaimOrder 是開墾。General 是負責的將領，−1 表示不指定（效果取最低）。
type ReclaimOrder struct {
	At      int
	General int

	// Auto 為真表示走電腦諸侯那一條（`0xbcea` 起的六支 → `0xba02`）。
	// **原版那一條不收錢**：`0xba02` 只寫州郡 offset 27（土地價值）與
	// 人口，從頭到尾沒有碰 offset 18。10 金是說明書 p.21 給玩家的規則。
	Auto bool
}

func (o ReclaimOrder) Prefecture() int { return o.At }
func (o ReclaimOrder) Apply(g *State, by state.FactionID) error {
	if !o.Auto {
		g.commandScene(o.At, assets.SceneLand, by) // `0x1a721`
	}
	return g.reclaim(o.At, o.General, by, !o.Auto)
}
func (o ReclaimOrder) Describe(g *State) string {
	return tf("log.reclaim", prefName(g, o.At), byWhom(g, o.General))
}

// FloodControlOrder 是防洪。
type FloodControlOrder struct {
	At      int
	General int

	// Auto 同 `ReclaimOrder`：電腦那一條（`0xbd39`）只寫洪水率，不收錢。
	Auto bool
}

func (o FloodControlOrder) Prefecture() int { return o.At }
func (o FloodControlOrder) Apply(g *State, by state.FactionID) error {
	if !o.Auto {
		g.commandScene(o.At, assets.SceneLand, by) // `0x1a932`
	}
	return g.floodControl(o.At, o.General, by, !o.Auto)
}
func (o FloodControlOrder) Describe(g *State) string {
	return tf("log.flood", prefName(g, o.At), byWhom(g, o.General))
}

// TrainOrder 是訓練兵士。
//
// **沒有 General 欄位**：原版對整個守軍跑一遍，不挑人（`State.Train`）。
//
// Units 給了就只練名單上的人（電腦那一條走共用的守將清單，見
// `TrainUnits`）；nil 練整郡。
type TrainOrder struct {
	At    int
	Units []int
}

func (o TrainOrder) Prefecture() int { return o.At }
func (o TrainOrder) Apply(g *State, by state.FactionID) error {
	g.commandScene(o.At, assets.SceneTrain, by) // `0x197f6`
	return g.TrainUnits(o.At, o.Units, by)
}
func (o TrainOrder) Describe(g *State) string {
	return tf("log.trainAll", prefName(g, o.At))
}

// ReliefOrder 是開倉賑民。Gold 是這次撥出去的金額。
type ReliefOrder struct{ At, Gold int }

func (o ReliefOrder) Prefecture() int { return o.At }
func (o ReliefOrder) Apply(g *State, by state.FactionID) error {
	return g.Relief(o.At, o.Gold, by)
}
func (o ReliefOrder) Describe(g *State) string {
	return tf("log.relief", prefName(g, o.At))
}

// SellRiceOrder／BuyRiceOrder 是米糧買賣。
type SellRiceOrder struct{ At, Units int }

func (o SellRiceOrder) Prefecture() int { return o.At }
func (o SellRiceOrder) Apply(g *State, by state.FactionID) error {
	g.commandScene(o.At, assets.SceneRice, by) // `0x1b5b2`
	return g.SellRice(o.At, o.Units, by)
}
func (o SellRiceOrder) Describe(g *State) string {
	return tf("log.sell", prefName(g, o.At), o.Units)
}

type BuyRiceOrder struct{ At, Units int }

func (o BuyRiceOrder) Prefecture() int { return o.At }
func (o BuyRiceOrder) Apply(g *State, by state.FactionID) error {
	g.commandScene(o.At, assets.SceneRice, by) // `0x1b2c7`
	return g.BuyRice(o.At, o.Units, by)
}
func (o BuyRiceOrder) Describe(g *State) string {
	return tf("log.buy", prefName(g, o.At), o.Units)
}

// RiceTradeOrder 是電腦諸侯的米糧買賣（`0xc634`）：**存糧往目標靠**，
// 低了買、高了賣，同一條算式。玩家那兩道是 `BuyRiceOrder`／`SellRiceOrder`。
type RiceTradeOrder struct{ At, Target int }

func (o RiceTradeOrder) Prefecture() int { return o.At }
func (o RiceTradeOrder) Apply(g *State, by state.FactionID) error {
	return g.TradeRiceTo(o.At, o.Target, by)
}
func (o RiceTradeOrder) Describe(g *State) string {
	return tf("log.buy", prefName(g, o.At), o.Target)
}

// RecruitOrder 是登用本地在野人才。
type RecruitOrder struct{ At, Target int }

func (o RecruitOrder) Prefecture() int { return o.At }
func (o RecruitOrder) Apply(g *State, by state.FactionID) error {
	// 玩家那一條的畫面（`0x1c088`–`0x1c129`）：主事者先在上格問
	// 「久聞 X 之才 是否願意相助」，判完那一位在下格答應或婉拒。
	// 問之前先播 `SCG07`（`0x1c00e`），答應之後再播 `SCG04`（`0x1c1ba`）。
	t := g.General(o.Target)
	player := g.playerCommand(by) && t != nil
	if player {
		g.commandScene(o.At, assets.SceneRecruit, by)
		g.say(g.Governor(o.At), true, true, tf("bub.recruitAsk", personName(t.Name)), o.At, 0x1c088)
	}
	err := g.Recruit(o.At, o.Target, by)
	switch {
	case !player:
	case err == nil:
		g.say(t, false, false, tf("bub.recruitYes", personName(t.Name)), o.At, 0x1c129)
		g.commandScene(o.At, assets.SceneJoin, by)
	case errors.Is(err, ErrDeclined):
		g.say(t, false, false, t_("bub.recruitNo"), o.At, 0x1c0cc)
	}
	return err
}
func (o RecruitOrder) Describe(g *State) string {
	return tf("log.recruit", prefName(g, o.At), byWhom(g, o.Target))
}

// AttackOrder 是發動戰役。
//
// **結果不在 Describe 裡**：`Apply` 之後要拿結果的呼叫端應該直接用
// `State.Attack`。命令層只保證「這一步做了什麼」。
// AttackOrder 是發動戰役。**At 是下令的郡（回合記在它身上），From 是出兵的郡**——
// 原版「從那一郡攻打」（`0x18998`）收任何自己的郡，`From` 是 0 就與 At 相同。
// 郡回合入口 `0x1746e` 只擋無主與非玩家控制，**沒有「這個月下過令」的旗標**，
// 所以來源郡自己那一次回合還在（月順序是洗過的，`0x1740a`；Issue #82）。
type AttackOrder struct {
	At, From, To int
	Force        []int

	// Keep 非 nil 時 Force 不用：出征的名單由戰役入口的整編決定
	// （原版電腦出兵的路，`ComputerAttack`），這個函式給的是整編那一刻
	// 的留守目標。
	Keep KeepFunc
}

func (o AttackOrder) Prefecture() int { return o.At }

// src 是出兵的郡：沒指定就是下令的郡。
func (o AttackOrder) src() int {
	if o.From != 0 {
		return o.From
	}
	return o.At
}

func (o AttackOrder) Apply(g *State, by state.FactionID) error {
	if o.Keep != nil {
		_, err := g.ComputerAttack(o.src(), o.To, by, o.Keep)
		return err
	}
	_, err := g.Attack(o.src(), o.To, o.Force, by)
	return err
}
func (o AttackOrder) Describe(g *State) string {
	return tf("log.attack", prefName(g, o.src()), prefName(g, o.To))
}

func byWhom(g *State, index int) string {
	if x := g.General(index); x != nil && x.Name != "" {
		return "（" + personName(x.Name) + "）"
	}
	return ""
}

// ConscriptOrder 是徵兵。
type ConscriptOrder struct {
	At      int
	General int
	Count   int
}

func (o ConscriptOrder) Prefecture() int { return o.At }
func (o ConscriptOrder) Apply(g *State, by state.FactionID) error {
	g.commandScene(o.At, assets.SceneArms, by) // `0x199c8`
	return g.Conscript(o.At, o.General, o.Count, by)
}
func (o ConscriptOrder) Describe(g *State) string {
	name := "?"
	if x := g.General(o.General); x != nil {
		name = personName(x.Name)
	}
	return tf("log.conscript", prefName(g, o.At), name, o.Count)
}

// ArmsOrder 是購置武器。
type ArmsOrder struct {
	At      int
	General int
	Units   int
}

func (o ArmsOrder) Prefecture() int { return o.At }
func (o ArmsOrder) Apply(g *State, by state.FactionID) error {
	g.commandScene(o.At, assets.SceneArms, by) // `0x19d66`
	return g.BuyArms(o.At, o.General, o.Units, by)
}
func (o ArmsOrder) Describe(g *State) string {
	name := "?"
	if x := g.General(o.General); x != nil {
		name = personName(x.Name)
	}
	return tf("log.arms", prefName(g, o.At), name, o.Units)
}

func prefName(g *State, id int) string {
	if p := g.Prefecture(id); p != nil {
		return fmt.Sprintf("%d %s", p.ID, placeName(p.Name))
	}
	return tf("msg.prefN", id)
}

// ApplyAll 依序套用一串命令，回傳套用成功的筆數與第一個錯誤。
//
// **違規就停，不跳過。** AI 產出一個違規的命令是 bug，
// 靜靜跳過會讓那個 bug 變成「AI 這回合比較保守」——看不出來。
//
// **`ErrDeclined` 不算違規**：登用被婉拒是判定的正常結果，命令本身
// 執行成功了。把它當成中斷的理由，會讓一次登用失敗連帶吃掉同一輪
// 後面所有的命令（指定太守就在後面），而外面只看得到「AI 這個月
// 做得比較少」。
func (g *State) ApplyAll(orders []Order, by state.FactionID) (int, error) {
	n := 0
	for i, o := range orders {
		err := o.Apply(g, by)
		switch {
		case err == nil:
			n++
		case errors.Is(err, ErrDeclined):
			n++
		default:
			return i, fmt.Errorf(t("log.orderN"), i+1, o.Describe(g), err)
		}
	}
	return n, nil
}

// ---- 其餘的命令型別 ------------------------------------------------------

// MoveOrder 是玩家的「調動軍隊」：Generals 是多選清單挑的那一份（`game.Move`）。
// MoveOrder 是調動軍隊。**At 是下令的郡，From 是移出的郡**（`0x18c6d`「從那一郡移出」
// 收任何自己的郡；Issue #82）。From 是 0 就與 At 相同。
type MoveOrder struct {
	At, From, To int
	Generals     []int
	Gold, Rice   int
}

func (o MoveOrder) Prefecture() int { return o.At }

// src 是移出的郡：沒指定就是下令的郡。
func (o MoveOrder) src() int {
	if o.From != 0 {
		return o.From
	}
	return o.At
}

func (o MoveOrder) Apply(g *State, by state.FactionID) error {
	g.commandScene(o.src(), assets.SceneMove, by) // `0x18f99`
	return g.Move(o.src(), o.To, o.Generals, o.Gold, o.Rice, by)
}
func (o MoveOrder) Describe(g *State) string {
	lead := -1
	if len(o.Generals) > 0 {
		lead = o.Generals[0]
	}
	return tf("log.move", byWhom(g, lead),
		prefName(g, o.At), prefName(g, o.To))
}

// RelocateOrder 是**電腦諸侯**的移防（`docs/spec/007`，`L0`）。
//
// 與玩家的「調動軍隊」（`MoveOrder`／`0x18bc8`）搬運共用 `0x1938a`，呼叫端的閘門不同，
// 所以分成兩個命令型別而不是加旗標：
//
//	整份出征名單一起搬（`0x1938a` 的迴圈），不是一位
//	目標郡的金米加完溢位時夾 30000，來源郡減完夾 0
//	沒有「主事者要有人接手」這道閘門——整郡搬空是允許的
type RelocateOrder struct {
	At, To     int
	Force      []int
	Gold, Rice int
}

func (o RelocateOrder) Prefecture() int { return o.At }
func (o RelocateOrder) Apply(g *State, by state.FactionID) error {
	return g.Relocate(o.At, o.To, o.Force, o.Gold, o.Rice, by)
}
func (o RelocateOrder) Describe(g *State) string {
	lead := -1
	if len(o.Force) > 0 {
		lead = o.Force[0]
	}
	return tf("log.move", byWhom(g, lead),
		prefName(g, o.At), prefName(g, o.To))
}

// TransportOrder 是運送錢糧。**At 是下令的郡，From 是送出的郡**（`0x19092`「從那一郡送出」
// 收任何自己的郡；Issue #82）。From 是 0 就與 At 相同。
type TransportOrder struct {
	At, From, To int
	Gold, Rice   int
}

func (o TransportOrder) Prefecture() int { return o.At }

// src 是送出的郡：沒指定就是下令的郡。
func (o TransportOrder) src() int {
	if o.From != 0 {
		return o.From
	}
	return o.At
}

func (o TransportOrder) Apply(g *State, by state.FactionID) error {
	g.commandScene(o.src(), assets.SceneMove, by) // `0x19273`
	return g.Transport(o.src(), o.To, o.Gold, o.Rice, by)
}
func (o TransportOrder) Describe(g *State) string {
	return tf("log.transport",
		prefName(g, o.src()), prefName(g, o.To), o.Gold, o.Rice)
}

type RedistributeOrder struct {
	At    int
	Units []int
	Auto  bool

	// CapSum 是電腦那一張表用的「總上限」：原版在**分派器入口**就把
	// 當時守將的帶兵上限加總存進 `es:[0x347e]`（`0xf030`），調整兵力
	// （`0xc2c4`）只重算總兵力，除的還是入口那一份總上限——這一輪登用
	// 進來的人有兵、沒上限，比例因此大於 1，每個人都填到上限（加強版
	// 月度對拍量到郡 16：11530 名兵攤成 13500）。0 表示照當下的部隊算。
	CapSum int
}

func (o RedistributeOrder) Prefecture() int { return o.At }
func (o RedistributeOrder) Apply(g *State, by state.FactionID) error {
	if o.Auto {
		return g.redistributeWith(o.At, o.Units, by, 1, o.CapSum)
	}
	g.commandScene(o.At, assets.SceneTrain, by) // `0x1a075`
	return g.Redistribute(o.At, o.Units, by)
}
func (o RedistributeOrder) Describe(g *State) string {
	return tf("log.balance", prefName(g, o.At), len(o.Units))
}

// BuildFortOrder 是築關。Cell 是戰場地圖索引加一（原版玩家用游標挑的
// 那一格）；0 表示讓 remake 自己挑（`BuildFort`）。
type BuildFortOrder struct{ At, General, Cell int }

func (o BuildFortOrder) Prefecture() int { return o.At }
func (o BuildFortOrder) Apply(g *State, by state.FactionID) error {
	g.commandScene(o.At, assets.SceneFort, by) // `0x1ab84`
	return g.BuildFortAt(o.At, o.General, o.Cell-1, by)
}
func (o BuildFortOrder) Describe(g *State) string {
	return tf("log.fort", prefName(g, o.At), byWhom(g, o.General))
}

type RestOrder struct{ At int }

func (o RestOrder) Prefecture() int { return o.At }
func (o RestOrder) Apply(g *State, by state.FactionID) error {
	return g.Rest(o.At, by)
}
func (o RestOrder) Describe(g *State) string {
	return tf("log.rest", prefName(g, o.At))
}

type SearchOrder struct {
	At, General int

	// Auto 為真表示走電腦諸侯那一條（`0xcd20` 起的六支 → `0xcc86`）。
	// **原版那一條不收錢**：`0xcc86` 只把一位身分 9 的人改成 8、
	// 把州郡 offset 23（在野武將數）加一，從頭到尾沒有碰 offset 18。
	// 5 金是說明書給玩家的規則（p.22），兩條不是同一組。
	Auto bool
}

func (o SearchOrder) Prefecture() int { return o.At }
func (o SearchOrder) Apply(g *State, by state.FactionID) error {
	found, err := g.search(o.At, o.General, by, !o.Auto)
	if err == nil && !o.Auto && g.playerCommand(by) {
		// 玩家那一條之後有畫面：找到的人先亮肖像，再由尋訪者報結果。
		g.pending = append(g.pending, g.searchEvents(g.General(o.General), found)...)
	}
	return err
}
func (o SearchOrder) Describe(g *State) string {
	return tf("log.search", prefName(g, o.At), byWhom(g, o.General))
}

// RewardOrder 是賞賜金帛的一位。Round 非 nil 時是玩家一道命令裡的一位（`OpenReward`）：
// 同一道命令裡賞過的擋下（「%s已賞賜過了」），不看每月旗標，也不結束回合——等 CloseGift。
type RewardOrder struct {
	At, Target, Gold int
	Round            *GiftRound
}

func (o RewardOrder) Prefecture() int { return o.At }

// KeepsTurn 為真時 `session.Do` 不把這一道當成郡回合的結束。
func (o RewardOrder) KeepsTurn() bool { return o.Round != nil }

func (o RewardOrder) Apply(g *State, by state.FactionID) error {
	if r := o.Round; r != nil {
		if r.given[o.Target] {
			return ErrAlreadyGifted
		}
		// 名單上點到就記下（`0x1c35f` 在問金額之前）。
		r.given[o.Target] = true
	}
	g.commandScene(o.At, assets.SceneReward, by) // `0x1c501`
	err := g.reward(o.At, o.Target, o.Gold, by, o.Round == nil)
	if err == nil && o.Round != nil {
		o.Round.Gave = true
	}
	if err == nil && g.playerCommand(by) {
		g.say(g.General(o.Target), false, false, t_("bub.rewardThanks"), o.At, 0x1c547) // `0x1c547`
	}
	return err
}
func (o RewardOrder) Describe(g *State) string {
	return tf("log.reward", byWhom(g, o.Target), o.Gold)
}

type DismissOrder struct{ At, Target int }

func (o DismissOrder) Prefecture() int { return o.At }
func (o DismissOrder) Apply(g *State, by state.FactionID) error {
	g.commandScene(o.At, assets.SceneDismiss, by) // `0x1c6ac`
	err := g.Dismiss(o.At, o.Target, by)
	if err == nil && g.playerCommand(by) {
		g.say(g.General(o.Target), true, false, t_("bub.dismissed"), o.At, 0x1c70a) // `0x1c70a`
	}
	return err
}
func (o DismissOrder) Describe(g *State) string {
	return tf("log.dismiss", byWhom(g, o.Target))
}

type AppointChiefOrder struct {
	At, Target int

	// Auto 為真表示走電腦諸侯那一條（`0xd7ae`）：君主不必在場，
	// 候選也不比對勢力。
	Auto bool
}

func (o AppointChiefOrder) Prefecture() int { return o.At }
func (o AppointChiefOrder) Apply(g *State, by state.FactionID) error {
	if o.Auto {
		return g.appointChief(o.At, o.Target, by)
	}
	g.commandScene(o.At, assets.SceneAppoint, by) // `0x1c9e5`
	err := g.AppointChief(o.At, o.Target, by)
	if err == nil && g.playerCommand(by) {
		// `0x1ca9e`／`0x1cad5`：君主在上格下令，新軍師在下格領命。
		t := g.General(o.Target)
		g.say(g.Lord(by), true, false, tf("bub.chiefOrder", personName(t.Name)), o.At, 0x1ca9e)
		g.say(t, false, true, t_("bub.chiefReply"), o.At, 0x1cad5)
	}
	return err
}
func (o AppointChiefOrder) Describe(g *State) string {
	return tf("log.chief", byWhom(g, o.Target))
}

// AppointGovernorOrder 是指定太守。玩家那一條（`0x1caea`）At 是下令的郡、Pref 是被指定的郡
// （`GovernorTarget`），**不耗回合**——君主選單派工器 `0x1c7a2` 回傳預設 0xFFFF，這一支
// （`0x1c82f`）的回傳丟掉。電腦那一條（Auto）只用 At。
type AppointGovernorOrder struct {
	At, Pref, Target int

	// Auto 為真表示走電腦諸侯那一條（`0xd652`）。**原版挑名單時不比對
	// 勢力**，所以目標可能是站在郡裡的外勢力武將；玩家那一條不收這種。
	Auto bool
}

func (o AppointGovernorOrder) Prefecture() int { return o.At }

// KeepsTurn 見 AppointGovernorOrder。
func (o AppointGovernorOrder) KeepsTurn() bool { return !o.Auto }

// target 是被指定的郡：玩家那一條是 Pref，電腦那一條是 At。
func (o AppointGovernorOrder) target() int {
	if o.Auto {
		return o.At
	}
	return o.Pref
}

func (o AppointGovernorOrder) Apply(g *State, by state.FactionID) error {
	if o.Auto {
		return g.appointGovernor(o.At, o.Target, by, false)
	}
	if !g.GovernorTarget(o.At, o.Pref) {
		return ErrNotYours
	}
	g.commandScene(o.Pref, assets.SceneAppoint, by) // `0x1cc77`
	err := g.AppointGovernor(o.Pref, o.Target, by)
	if err == nil && g.playerCommand(by) {
		// `0x1ccd7`／`0x1cd24`：君主在上格下令，新太守在下格領命。
		t := g.General(o.Target)
		g.say(g.Lord(by), true, false, tf("bub.governorOrder", personName(t.Name)), o.Pref, 0x1ccd7)
		g.say(t, false, true, tf("bub.governorReply", personName(t.Name)), o.Pref, 0x1cd24)
	}
	return err
}
func (o AppointGovernorOrder) Describe(g *State) string {
	return tf("log.governor", prefName(g, o.target()), byWhom(g, o.Target))
}

// AutonomyOrder 是君主→3.郡縣自冶的一次設定：At 是下令的郡，Pref 是被授權的郡
// （`AutonomyTarget`）。**不耗回合**：君主選單的派工器（`0x1c7a2`）把回傳值預設成
// 0xFFFF，自冶那一支（`0x1c835`）的回傳直接丟掉，主命令迴圈照舊回到選單。
type AutonomyOrder struct {
	At   int
	Pref int
	Mode Autonomy
}

func (o AutonomyOrder) Prefecture() int { return o.At }

// KeepsTurn 見 AutonomyOrder。
func (o AutonomyOrder) KeepsTurn() bool { return true }

func (o AutonomyOrder) Apply(g *State, by state.FactionID) error {
	if !g.AutonomyTarget(o.At, o.Pref) {
		return ErrNotYours
	}
	err := g.SetAutonomy(o.Pref, o.Mode, by)
	if err == nil && g.playerCommand(by) {
		// `0x1cf7d`／`0x1cfc6`：君主在上格把郡交給主事者，主事者在下格領命。
		if gov := g.Governor(o.Pref); gov != nil {
			g.say(g.Lord(by), true, false, tf("bub.autonomyOrder", personName(gov.Name)), o.Pref, 0x1cf7d)
			g.say(gov, false, true, tf("bub.autonomyReply", personName(gov.Name)), o.Pref, 0x1cfc6)
		}
	}
	return err
}
func (o AutonomyOrder) Describe(g *State) string {
	return tf("log.autonomy", prefName(g, o.Pref), AutonomyName(o.Mode))
}

type GiftOrder struct {
	At, Target int
	What       Treasure

	// Auto 為真表示走電腦諸侯那一條（`0xd961` 起的四支）。**原版那四支
	// 沒有「君主要在場」的檢查**——那是說明書給玩家的規則（p.24），
	// 兩條不是同一組。
	Auto bool

	// Round 是玩家那一道賞賜物品（`GiftRound`）；送完這一件**不結束**
	// 這個郡的回合，等 `CloseGift`。nil 表示只送這一件就收掉。
	Round *GiftRound
}

func (o GiftOrder) Prefecture() int { return o.At }

// KeepsTurn 為真時 `session.Do` 不把這一道當成郡回合的結束。
func (o GiftOrder) KeepsTurn() bool { return o.Round != nil }

func (o GiftOrder) Apply(g *State, by state.FactionID) error {
	if o.Auto {
		return g.giftTreasureAuto(o.At, o.Target, o.What, by)
	}
	r := o.Round
	if r == nil {
		var err error
		if r, err = g.OpenGift(o.At, by); err != nil {
			return err
		}
	}
	if err := g.GiftCheck(r, o.Target, o.What); err != nil {
		return err
	}
	// 選了那一件：先貼 `SCG24.IMG`（`0x1d264`），再扣寶庫、加能力與忠誠。
	g.commandScene(o.At, assets.SceneReward, by)
	if err := g.Gift(r, o.Target, o.What); err != nil {
		return err
	}
	if g.playerCommand(by) {
		// `0x1d4c1` 道謝之後再畫一次受賜者的資料卡（`0x1d4d1`），等鍵。
		t := g.General(o.Target)
		g.say(t, true, false, tf("bub.giftThanks", personName(t.Name)), o.At, 0x1d4c1)
		g.showCard(t)
	}
	if o.Round == nil {
		g.CloseGift(r)
	}
	return nil
}
func (o GiftOrder) Describe(g *State) string {
	return tf("log.gift", TreasureName(o.What), byWhom(g, o.Target))
}

// HeadhuntOrder 是挖角。Vetted 表示目標是電腦的挑人常式（`0xe0bc`）
// 掃全表挑出來的——那一支已經替每一位候選擲過忠誠那一道 `RND(15)`，
// 判定常式 `0x1dc0a` 不再擲；玩家下的命令則在這裡擲（`Headhuntable`）。
// Bonus 是電腦那三張表挑完人之後加在開價上的數（`0x1dcb5` 的第三個參數）：
// 等級 3 是 0、等級 4 `RND(10)+3`（`0xe3a6`）、等級 5 `RND(10)+10`（`0xe44e`）。
type HeadhuntOrder struct {
	At, Target int
	Vetted     bool
	Bonus      int
}

func (o HeadhuntOrder) Prefecture() int { return o.At }
func (o HeadhuntOrder) Apply(g *State, by state.FactionID) error {
	if o.Vetted {
		return g.headhuntVetted(o.At, o.Target, by, o.Bonus)
	}
	// 玩家那一條的畫面（`0x1dad0`–`0x1dbc9`）：君主先在上格說「尊駕之才
	// 吾仰慕久矣」，判完那一位在下格投靠或回絕。
	// 開口之前先播 `SCG13`（`0x1da72`）；投靠的話先播 `SCG04`（`0x1db5d`）再說話。
	t := g.General(o.Target)
	player := g.playerCommand(by) && t != nil
	if player {
		g.commandScene(o.At, assets.ScenePlotSow, by)
		g.say(g.Lord(by), true, false, t_("bub.headhuntAsk"), o.At, 0x1dad0)
	}
	err := g.Headhunt(o.At, o.Target, by)
	switch {
	case !player:
	case err == nil:
		g.commandScene(o.At, assets.SceneJoin, by)
		g.say(t, false, true, tf("bub.headhuntYes", personName(t.Name)), o.At, 0x1dbc9)
	case errors.Is(err, ErrDeclined):
		g.say(t, false, true, t_("bub.headhuntNo"), o.At, 0x1db1f)
	}
	return err
}
func (o HeadhuntOrder) Describe(g *State) string {
	return tf("log.headhunt", byWhom(g, o.Target))
}

type PlotOrder struct {
	At, To int
	What   Plot
	Envoy  int

	// Plan 非 nil 時照玩家那幾問的完整計畫下（出使的郡、要打的郡、我方的郡，`PlotPlan`）；
	// To／Envoy 不用。
	Plan *PlotPlan
}

func (o PlotOrder) Prefecture() int { return o.At }
func (o PlotOrder) Apply(g *State, by state.FactionID) error {
	envoy := o.Envoy
	var ok bool
	var err error
	if o.Plan != nil {
		envoy = o.Plan.Envoy
		ok, err = g.UsePlotPlan(o.At, o.What, *o.Plan, by)
	} else {
		ok, err = g.UsePlot(o.At, o.To, o.What, o.Envoy, by)
	}
	if err == nil && g.playerCommand(by) && o.What != PlotJointAttack {
		// `0x2c962`／`0x2c9ae` 等：使者回來在上格報成敗。
		key := "bub.plotFailed"
		if ok {
			key = "bub.plotWorked"
		}
		g.say(g.General(envoy), true, false, t_(key), o.At, 0x2c962)
	}
	return err
}
func (o PlotOrder) Describe(g *State) string {
	return tf("log.plot", prefName(g, o.At), prefName(g, o.To), PlotName(o.What))
}
