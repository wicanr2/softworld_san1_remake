package main

// 主戰場的操作。
//
// 指令與編號照原版（`docs/re/04` §4）：
//
//	部隊層  1.移動 2.對戰 3.快戰 4.死戰 5.弓箭 6.策略 7.查看 8.退兵 0.休息
//	單位層  1.行軍 2.單挑 3.攻擊 7.查看 0.休息
//	方向    4 5 6 / 1 2 3
//
// 畫面在 `internal/ui`，規則在 `internal/battle`。這一檔只接按鍵。

import (
	"fmt"

	"github.com/wicanr2/softworld_san1_remake/internal/battle"
	"github.com/wicanr2/softworld_san1_remake/internal/game"
	"github.com/wicanr2/softworld_san1_remake/internal/ui"
)

// fight 是進行中的戰役狀態。
type fight struct {
	pending *game.Pending
	runner  *battle.Runner
	acting  *battle.Unit

	// waiting 是等下一個輸入的種類。
	waiting waitFor

	// cmd 是已經選好、正在等方向或目標的指令。
	cmd battle.Command

	// engage 為真表示在單位層（原版的「對戰」）。
	engage bool

	view ui.BattleView
}

type waitFor int

const (
	waitCommand waitFor = iota // 等部隊層的指令
	waitDir                    // 等方向
	waitPlot                   // 等計謀編號
	waitEngage                 // 等單位層的指令
)

// startBattle 開一場由玩家指揮的戰役。
func (a *app) startBattle(from, to int, force []int) {
	p, err := a.s.G.BeginAttack(from, to, force, a.s.Player)
	if err != nil {
		a.view.Prompt = fmt.Sprintf("出兵失敗：%v", err)
		return
	}
	f := &fight{pending: p}
	// 玩家指揮攻方；守方交給電腦。是玩家自己按下「發動戰役」才走到這裡，
	// 所以這一側一定是攻方（「主守軍必須派出所有兵力」，p.27）。
	f.runner = battle.NewRunner(p.Battle(), func(s battle.Side) bool {
		return s.Attacking()
	})
	a.fight = f
	a.nextActor()
}

// nextActor 推進到下一支要玩家下令的部隊；沒有就收尾。
func (a *app) nextActor() {
	f := a.fight
	f.acting = f.runner.Next()
	f.engage, f.waiting = false, waitCommand
	if f.acting == nil {
		a.endBattle()
		return
	}
	f.view.Acting = f.acting
	f.view.Cursor = ui.Hexer{At: f.acting.At, Shown: true}
	f.view.Menu, f.view.Items = "指令", ui.BattleCommandLines()
	f.view.Prompt = fmt.Sprintf("%s　餘步 %d", f.acting.Name(), f.acting.Move)
	f.view.Page, f.view.PageTitle = nil, ""
}

// endBattle 把打完的戰役搬回局面，回到主畫面。
func (a *app) endBattle() {
	r := a.s.G.FinishAttack(a.fight.pending)
	a.fight = nil
	a.s.Drain()
	if r != nil {
		a.view.PageTitle, a.view.Page = ui.BattleReport(a.s.G, r)
	}
	a.view.Prompt = "戰役結束。Esc 收起戰報"
}

// battleKey 收戰場上的一個按鍵。
func (a *app) battleKey(k byte) {
	f := a.fight
	if f == nil || f.acting == nil {
		return
	}
	b := f.pending.Battle()
	say := func(format string, v ...any) { f.view.Prompt = fmt.Sprintf(format, v...) }
	done := func(err error) {
		if err != nil {
			say("%v", err)
			f.waiting = waitCommand
			f.view.Menu, f.view.Items = "指令", ui.BattleCommandLines()
			return
		}
		// 一支部隊一天可以做好幾件事；移動之後還有餘步就繼續。
		if f.acting.Alive() && f.acting.Move > 0 && f.cmd == battle.CmdMove {
			f.waiting = waitDir
			say("往哪個方向（4 5 6 / 1 2 3，0 結束）　餘步 %d", f.acting.Move)
			return
		}
		f.runner.Done()
		a.nextActor()
	}

	switch f.waiting {
	case waitCommand:
		a.battleCommand(k, done, say)
	case waitEngage:
		a.battleEngage(k, done, say)
	case waitDir:
		if k == '0' {
			f.runner.Done()
			a.nextActor()
			return
		}
		d, ok := dirFromKey(k)
		if !ok {
			say("方向是 4 5 6 / 1 2 3")
			return
		}
		done(a.applyDir(d))
	case waitPlot:
		s := battle.Stratagem(k - '0')
		if s < battle.Fire || s > battle.Siege {
			say("計謀是 1–6")
			return
		}
		t := b.UnitAt(f.view.Cursor.At)
		if t == nil {
			say("游標上沒有部隊——先用方向鍵移到目標上")
			return
		}
		done(b.UseStratagem(f.acting, s, t.At))
	}
}

// battleCommand 是部隊層的九個指令。
func (a *app) battleCommand(k byte, done func(error), say func(string, ...any)) {
	f := a.fight
	b := f.pending.Battle()
	f.cmd = battle.Command(k - '0')
	switch f.cmd {
	case battle.CmdRest:
		done(b.Rest(f.acting))
	case battle.CmdMove, battle.CmdQuick, battle.CmdDeath, battle.CmdArchery:
		f.waiting = waitDir
		f.view.Menu, f.view.Items = f.cmd.String(), []string{"4 5 6", "1 2 3"}
		say("往哪個方向（4 5 6 / 1 2 3）")
	case battle.CmdEngage:
		f.engage = true
		f.waiting = waitEngage
		f.view.Menu, f.view.Items = "對戰", ui.BattleEngageLines()
		say("對戰：1.行軍 2.單挑 3.攻擊 7.查看 0.休息")
	case battle.CmdPlot:
		f.waiting = waitPlot
		f.view.Menu, f.view.Items = "策略", []string{
			"1.火攻 2.水洽 3.陷阱", "4.誘敵 5.燒糧 6.圍攻",
		}
		say("對哪一支用計（先把游標移到目標上，再按 1–6）")
	case battle.CmdInspect:
		u, err := b.Inspect(f.acting, f.view.Cursor.At)
		if err != nil {
			say("%v", err)
			return
		}
		f.view.PageTitle, f.view.Page = ui.BattleUnitPage(u)
		say("Esc 收起")
	case battle.CmdRetreat:
		done(b.Retreat(f.acting))
	default:
		say("指令是 0–8")
	}
}

// battleEngage 是單位層（原版的「對戰」）：行軍／單挑／攻擊。
func (a *app) battleEngage(k byte, done func(error), say func(string, ...any)) {
	f := a.fight
	b := f.pending.Battle()
	switch k {
	case '0':
		done(b.Rest(f.acting))
	case '1':
		f.cmd = battle.CmdMove
		f.waiting = waitDir
		say("行軍方向（4 5 6 / 1 2 3）　餘步 %d", f.acting.Move)
	case '2':
		f.cmd = battle.CmdEngage
		f.waiting = waitDir
		say("向哪個方向單挑（4 5 6 / 1 2 3）")
	case '3':
		f.cmd = battle.CmdQuick
		f.waiting = waitDir
		say("攻擊哪個方向（4 5 6 / 1 2 3）")
	case '7':
		u, err := b.Inspect(f.acting, f.view.Cursor.At)
		if err != nil {
			say("%v", err)
			return
		}
		f.view.PageTitle, f.view.Page = ui.BattleUnitPage(u)
		say("Esc 收起")
	default:
		say("對戰的指令是 1 2 3 7 0")
	}
}

// applyDir 把選好的指令套到一個方向上。
func (a *app) applyDir(d battle.Dir) error {
	f := a.fight
	b := f.pending.Battle()
	switch f.cmd {
	case battle.CmdMove:
		err := b.Move(f.acting, d)
		if err == nil {
			f.view.Cursor = ui.Hexer{At: f.acting.At, Shown: true}
		}
		return err
	case battle.CmdQuick:
		return b.QuickBattle(f.acting, d)
	case battle.CmdDeath:
		return b.DeathBattle(f.acting, d)
	case battle.CmdEngage:
		// 單位層的「單挑」。對方接不接受是它的事，這裡一律叫陣。
		return b.Duel(f.acting, d, true)
	case battle.CmdArchery:
		// 「相間一格」：同一方向連走兩步。
		return b.Archery(f.acting, f.acting.At.Step(d).Step(d))
	}
	return fmt.Errorf("這個指令不吃方向")
}

// dirFromKey 把數字鍵換成方向。原版的鍵盤是 `4 5 6 / 1 2 3`。
func dirFromKey(k byte) (battle.Dir, bool) {
	switch k {
	case '1':
		return battle.DirDownLeft, true
	case '2':
		return battle.DirDown, true
	case '3':
		return battle.DirDownRight, true
	case '4':
		return battle.DirUpLeft, true
	case '5':
		return battle.DirUp, true
	case '6':
		return battle.DirUpRight, true
	}
	return 0, false
}

// battleMove 讓游標在戰場上移動（查看與用計要先指目標）。
func (a *app) battleMove(d battle.Dir) {
	f := a.fight
	if f == nil {
		return
	}
	to := f.view.Cursor.At.Step(d)
	if f.pending.Battle().Field.InBounds(to) {
		f.view.Cursor.At = to
	}
}
