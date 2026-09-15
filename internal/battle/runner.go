package battle

import "fmt"

// 一步一步打的戰役。
//
// `Auto` 把整場打完，適合電腦諸侯之間的戰役與無頭測試；
// 玩家要親自指揮就得能**停在自己的部隊上等輸入**。
//
// 原版的主戰場有兩層指令（`docs/re/04` §4）：
//
//	部隊層  1.移動 2.對戰 3.快戰 4.死戰 5.弓箭 6.策略 7.查看 8.退兵 0.休息
//	單位層  1.行軍 2.單挑 3.攻擊 7.查看 0.休息
//
// 這一層只負責**輪到誰**；動作本身還是 `Battle` 的方法。

// Runner 依手冊 p.28 的順序輪流讓每一支部隊行動，遇到玩家的部隊就停下來。
type Runner struct {
	B *Battle

	// Human 回報某個軍力是不是玩家指揮的。nil 表示全部交給電腦。
	Human func(Side) bool

	queue []*Unit
}

// NewRunner 開一個逐步推進的戰役。
func NewRunner(b *Battle, human func(Side) bool) *Runner {
	r := &Runner{B: b, Human: human}
	r.queue = b.Order()
	return r
}

func (r *Runner) human(s Side) bool { return r.Human != nil && r.Human(s) }

// Next 推進到下一支**要玩家下令**的部隊，沿路把電腦的部隊打完。
//
// 回傳 nil 表示這場戰役結束了。
func (r *Runner) Next() *Unit {
	for !r.B.Over {
		for len(r.queue) > 0 {
			u := r.queue[0]
			if !u.Alive() {
				r.queue = r.queue[1:]
				continue
			}
			if r.human(u.Side) {
				// 玩家的部隊：輪到之前一樣重算綜合能力；中了陷阱就
				// 只倒數（原版也不讓玩家下令，`0x24e71`）。
				u.RefreshQuality()
				if u.Trapped > 0 {
					r.B.SkipTrappedTurn(u)
					r.queue = r.queue[1:]
					continue
				}
				return u
			}
			r.B.AutoTurn(u)
			r.queue = r.queue[1:]
			if r.B.Over {
				return nil
			}
		}
		r.B.EndDay()
		if r.B.Over {
			return nil
		}
		r.queue = r.B.Order()
	}
	return nil
}

// Done 結束目前這一支部隊的回合。
//
// **要玩家自己說「我下完了」**：一支部隊一天可以做好幾件事
// （走幾步再攻擊），沒有這一步就分不出「還沒動」與「不想動」。
//
// 收尾照原版的回合常式：判投敵、回填移動力（`EndTurn`）。
func (r *Runner) Done() {
	if len(r.queue) > 0 {
		r.B.EndTurn(r.queue[0])
		r.queue = r.queue[1:]
	}
}

// Remaining 是這一天還沒行動的部隊數（含目前這一支）。
func (r *Runner) Remaining() int { return len(r.queue) }

// Command 是部隊層的九個指令，編號與原版相同（`docs/re/04` §4）。
type Command int

const (
	CmdRest    Command = 0 // 0.休息
	CmdMove    Command = 1 // 1.移動
	CmdEngage  Command = 2 // 2.對戰
	CmdQuick   Command = 3 // 3.快戰
	CmdDeath   Command = 4 // 4.死戰
	CmdArchery Command = 5 // 5.弓箭
	CmdPlot    Command = 6 // 6.策略
	CmdInspect Command = 7 // 7.查看
	CmdRetreat Command = 8 // 8.退兵
)

func (c Command) String() string {
	switch c {
	case CmdRest:
		return "休息"
	case CmdMove:
		return "移動"
	case CmdEngage:
		return "對戰"
	case CmdQuick:
		return "快戰"
	case CmdDeath:
		return "死戰"
	case CmdArchery:
		return "弓箭"
	case CmdPlot:
		return "策略"
	case CmdInspect:
		return "查看"
	case CmdRetreat:
		return "退兵"
	}
	return "?"
}

// Commands 是部隊層的指令，順序照原版的選單。
func Commands() []Command {
	return []Command{CmdMove, CmdEngage, CmdQuick, CmdDeath, CmdArchery,
		CmdPlot, CmdInspect, CmdRetreat, CmdRest}
}

// TuneInspectCost 是「查看」敵軍要花的金。**這個有出處**：
// 原版的訊息是 `查看敵軍須10金!`（`AA.EXE` `0x46ccf`）。
const TuneInspectCost = 10

// Inspect 是「查看」：看一支敵軍的細節，要花 10 金。
//
// 看自己的部隊不用錢——原版的訊息只在敵軍那一側。
func (b *Battle) Inspect(u *Unit, target Hex) (*Unit, error) {
	if err := b.canAct(u); err != nil {
		return nil, err
	}
	t := b.UnitAt(target)
	if t == nil {
		return nil, fmt.Errorf("battle: 那裡沒有部隊")
	}
	if t.Side.Attacking() == u.Side.Attacking() {
		return t, nil
	}
	if b.Gold[u.Side] < TuneInspectCost {
		return nil, fmt.Errorf("battle: 查看敵軍須 %d 金", TuneInspectCost)
	}
	b.Gold[u.Side] -= TuneInspectCost
	b.note("blog.inspect", u.Name(), t.Name(), TuneInspectCost)
	return t, nil
}
