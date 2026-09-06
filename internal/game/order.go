package game

import (
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
	return fmt.Sprintf("開墾 %s%s", prefName(g, o.At), byWhom(g, o.General))
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
	return fmt.Sprintf("防洪 %s%s", prefName(g, o.At), byWhom(g, o.General))
}

// TrainOrder 是訓練兵士。
type TrainOrder struct {
	At      int
	General int
}

func (o TrainOrder) Prefecture() int { return o.At }
func (o TrainOrder) Apply(g *State, by state.FactionID) error {
	return g.Train(o.At, o.General, by)
}
func (o TrainOrder) Describe(g *State) string {
	return fmt.Sprintf("訓練 %s%s", prefName(g, o.At), byWhom(g, o.General))
}

// ReliefOrder 是開倉賑民。
type ReliefOrder struct{ At int }

func (o ReliefOrder) Prefecture() int { return o.At }
func (o ReliefOrder) Apply(g *State, by state.FactionID) error {
	return g.Relief(o.At, by)
}
func (o ReliefOrder) Describe(g *State) string {
	return fmt.Sprintf("賑民 %s", prefName(g, o.At))
}

// SellRiceOrder／BuyRiceOrder 是米糧買賣。
type SellRiceOrder struct{ At, Units int }

func (o SellRiceOrder) Prefecture() int { return o.At }
func (o SellRiceOrder) Apply(g *State, by state.FactionID) error {
	return g.SellRice(o.At, o.Units, by)
}
func (o SellRiceOrder) Describe(g *State) string {
	return fmt.Sprintf("賣米 %s %d 單位", prefName(g, o.At), o.Units)
}

type BuyRiceOrder struct{ At, Units int }

func (o BuyRiceOrder) Prefecture() int { return o.At }
func (o BuyRiceOrder) Apply(g *State, by state.FactionID) error {
	return g.BuyRice(o.At, o.Units, by)
}
func (o BuyRiceOrder) Describe(g *State) string {
	return fmt.Sprintf("買米 %s %d 單位", prefName(g, o.At), o.Units)
}

// RecruitOrder 是登用本地在野人才。
type RecruitOrder struct{ At, Target int }

func (o RecruitOrder) Prefecture() int { return o.At }
func (o RecruitOrder) Apply(g *State, by state.FactionID) error {
	return g.Recruit(o.At, o.Target, by)
}
func (o RecruitOrder) Describe(g *State) string {
	return fmt.Sprintf("登用 %s%s", prefName(g, o.At), byWhom(g, o.Target))
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
	return fmt.Sprintf("%s 出兵攻 %s", prefName(g, o.At), prefName(g, o.To))
}

func byWhom(g *State, index int) string {
	if x := g.General(index); x != nil && x.Name != "" {
		return "（" + x.Name + "）"
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
		name = x.Name
	}
	return fmt.Sprintf("徵兵 %s／%s %d 人", prefName(g, o.At), name, o.Count)
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
		name = x.Name
	}
	return fmt.Sprintf("武器 %s／%s %d 單位", prefName(g, o.At), name, o.Units)
}

func prefName(g *State, id int) string {
	if p := g.Prefecture(id); p != nil {
		return fmt.Sprintf("%d %s", p.ID, p.Name)
	}
	return fmt.Sprintf("郡 %d", id)
}

// ApplyAll 依序套用一串命令，回傳套用成功的筆數與第一個錯誤。
//
// **不會跳過錯誤繼續跑。** AI 產出一個違規的命令是 bug，
// 靜靜跳過會讓那個 bug 變成「AI 這回合比較保守」——看不出來。
func (g *State) ApplyAll(orders []Order, by state.FactionID) (int, error) {
	for i, o := range orders {
		if err := o.Apply(g, by); err != nil {
			return i, fmt.Errorf("第 %d 個命令（%s）：%w", i+1, o.Describe(g), err)
		}
	}
	return len(orders), nil
}
