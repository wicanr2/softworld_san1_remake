package game

import (
	"errors"
	"fmt"

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
}

func (o ReclaimOrder) Prefecture() int { return o.At }
func (o ReclaimOrder) Apply(g *State, by state.FactionID) error {
	return g.Reclaim(o.At, o.General, by)
}
func (o ReclaimOrder) Describe(g *State) string {
	return tf("log.reclaim", prefName(g, o.At), byWhom(g, o.General))
}

// FloodControlOrder 是防洪。
type FloodControlOrder struct {
	At      int
	General int
}

func (o FloodControlOrder) Prefecture() int { return o.At }
func (o FloodControlOrder) Apply(g *State, by state.FactionID) error {
	return g.FloodControl(o.At, o.General, by)
}
func (o FloodControlOrder) Describe(g *State) string {
	return tf("log.flood", prefName(g, o.At), byWhom(g, o.General))
}

// TrainOrder 是訓練兵士。
//
// **沒有 General 欄位**：原版對整個守軍跑一遍，不挑人（`State.Train`）。
type TrainOrder struct{ At int }

func (o TrainOrder) Prefecture() int { return o.At }
func (o TrainOrder) Apply(g *State, by state.FactionID) error {
	return g.Train(o.At, by)
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
	return g.SellRice(o.At, o.Units, by)
}
func (o SellRiceOrder) Describe(g *State) string {
	return tf("log.sell", prefName(g, o.At), o.Units)
}

type BuyRiceOrder struct{ At, Units int }

func (o BuyRiceOrder) Prefecture() int { return o.At }
func (o BuyRiceOrder) Apply(g *State, by state.FactionID) error {
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
	return g.Recruit(o.At, o.Target, by)
}
func (o RecruitOrder) Describe(g *State) string {
	return tf("log.recruit", prefName(g, o.At), byWhom(g, o.Target))
}

// AttackOrder 是發動戰役。
//
// **結果不在 Describe 裡**：`Apply` 之後要拿結果的呼叫端應該直接用
// `State.Attack`。命令層只保證「這一步做了什麼」。
type AttackOrder struct {
	At, To int
	Force  []int
}

func (o AttackOrder) Prefecture() int { return o.At }
func (o AttackOrder) Apply(g *State, by state.FactionID) error {
	_, err := g.Attack(o.At, o.To, o.Force, by)
	return err
}
func (o AttackOrder) Describe(g *State) string {
	return tf("log.attack", prefName(g, o.At), prefName(g, o.To))
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

type MoveOrder struct {
	At, To, General int
	Gold, Rice      int
}

func (o MoveOrder) Prefecture() int { return o.At }
func (o MoveOrder) Apply(g *State, by state.FactionID) error {
	return g.Move(o.At, o.To, o.General, o.Gold, o.Rice, by)
}
func (o MoveOrder) Describe(g *State) string {
	return tf("log.move", byWhom(g, o.General),
		prefName(g, o.At), prefName(g, o.To))
}

type TransportOrder struct {
	At, To     int
	Gold, Rice int
}

func (o TransportOrder) Prefecture() int { return o.At }
func (o TransportOrder) Apply(g *State, by state.FactionID) error {
	return g.Transport(o.At, o.To, o.Gold, o.Rice, by)
}
func (o TransportOrder) Describe(g *State) string {
	return tf("log.transport",
		prefName(g, o.At), prefName(g, o.To), o.Gold, o.Rice)
}

type RedistributeOrder struct {
	At    int
	Units []int
}

func (o RedistributeOrder) Prefecture() int { return o.At }
func (o RedistributeOrder) Apply(g *State, by state.FactionID) error {
	return g.Redistribute(o.At, o.Units, by)
}
func (o RedistributeOrder) Describe(g *State) string {
	return tf("log.balance", prefName(g, o.At), len(o.Units))
}

type BuildFortOrder struct{ At, General int }

func (o BuildFortOrder) Prefecture() int { return o.At }
func (o BuildFortOrder) Apply(g *State, by state.FactionID) error {
	return g.BuildFort(o.At, o.General, by)
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
	_, err := g.search(o.At, o.General, by, !o.Auto)
	return err
}
func (o SearchOrder) Describe(g *State) string {
	return tf("log.search", prefName(g, o.At), byWhom(g, o.General))
}

type RewardOrder struct{ At, Target, Gold int }

func (o RewardOrder) Prefecture() int { return o.At }
func (o RewardOrder) Apply(g *State, by state.FactionID) error {
	return g.Reward(o.At, o.Target, o.Gold, by)
}
func (o RewardOrder) Describe(g *State) string {
	return tf("log.reward", byWhom(g, o.Target), o.Gold)
}

type DismissOrder struct{ At, Target int }

func (o DismissOrder) Prefecture() int { return o.At }
func (o DismissOrder) Apply(g *State, by state.FactionID) error {
	return g.Dismiss(o.At, o.Target, by)
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
	return g.AppointChief(o.At, o.Target, by)
}
func (o AppointChiefOrder) Describe(g *State) string {
	return tf("log.chief", byWhom(g, o.Target))
}

type AppointGovernorOrder struct {
	At, Target int

	// Auto 為真表示走電腦諸侯那一條（`0xd652`）。**原版挑名單時不比對
	// 勢力**，所以目標可能是站在郡裡的外勢力武將；玩家那一條不收這種。
	Auto bool
}

func (o AppointGovernorOrder) Prefecture() int { return o.At }
func (o AppointGovernorOrder) Apply(g *State, by state.FactionID) error {
	if o.Auto {
		return g.appointGovernor(o.At, o.Target, by, false)
	}
	return g.AppointGovernor(o.At, o.Target, by)
}
func (o AppointGovernorOrder) Describe(g *State) string {
	return tf("log.governor", prefName(g, o.At), byWhom(g, o.Target))
}

type AutonomyOrder struct {
	At   int
	Mode Autonomy
}

func (o AutonomyOrder) Prefecture() int { return o.At }
func (o AutonomyOrder) Apply(g *State, by state.FactionID) error {
	return g.SetAutonomy(o.At, o.Mode, by)
}
func (o AutonomyOrder) Describe(g *State) string {
	return tf("log.autonomy", prefName(g, o.At), AutonomyName(o.Mode))
}

type GiftOrder struct {
	At, Target int
	What       Treasure

	// Auto 為真表示走電腦諸侯那一條（`0xd961` 起的四支）。**原版那四支
	// 沒有「君主要在場」的檢查**——那是說明書給玩家的規則（p.24），
	// 兩條不是同一組。
	Auto bool
}

func (o GiftOrder) Prefecture() int { return o.At }
func (o GiftOrder) Apply(g *State, by state.FactionID) error {
	if o.Auto {
		return g.giftTreasure(o.At, o.Target, o.What, by, false)
	}
	return g.GiftTreasure(o.At, o.Target, o.What, by)
}
func (o GiftOrder) Describe(g *State) string {
	return tf("log.gift", TreasureName(o.What), byWhom(g, o.Target))
}

type HeadhuntOrder struct{ At, Target int }

func (o HeadhuntOrder) Prefecture() int { return o.At }
func (o HeadhuntOrder) Apply(g *State, by state.FactionID) error {
	return g.Headhunt(o.At, o.Target, by)
}
func (o HeadhuntOrder) Describe(g *State) string {
	return tf("log.headhunt", byWhom(g, o.Target))
}

type PlotOrder struct {
	At, To int
	What   Plot
	Envoy  int
}

func (o PlotOrder) Prefecture() int { return o.At }
func (o PlotOrder) Apply(g *State, by state.FactionID) error {
	_, err := g.UsePlot(o.At, o.To, o.What, o.Envoy, by)
	return err
}
func (o PlotOrder) Describe(g *State) string {
	return tf("log.plot", prefName(g, o.At), prefName(g, o.To), PlotName(o.What))
}
