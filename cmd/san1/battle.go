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
	"strings"

	"github.com/wicanr2/softworld_san1_remake/internal/battle"
	"github.com/wicanr2/softworld_san1_remake/internal/game"
	"github.com/wicanr2/softworld_san1_remake/internal/state"
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

	// camping 是還沒紮完營的部隊（開戰前逐隊指定位置）。
	camping []*battle.Unit

	view ui.BattleView
}

type waitFor int

const (
	waitCommand waitFor = iota // 等部隊層的指令
	waitDir                    // 等方向
	waitPlot                   // 等計謀編號
	waitEngage                 // 等單位層的指令
	waitCamp                   // 開戰前紮營
)

// startBattle 開一場由玩家指揮的戰役。
func (a *app) startBattle(from, to int, force []int, sup game.Supply) {
	p, err := a.s.G.BeginAttack(from, to, force, a.s.Player, sup)
	if err != nil {
		a.view.Prompt = tf("bat.attackFailed", err)
		return
	}
	f := &fight{pending: p}
	// 玩家指揮攻方；守方交給電腦。是玩家自己按下「發動戰役」才走到這裡，
	// 所以這一側一定是攻方（「主守軍必須派出所有兵力」，p.27）。
	f.runner = battle.NewRunner(p.Battle(), func(s battle.Side) bool {
		return s.Attacking()
	})
	a.fight = f
	// 開戰前逐隊紮營（原版 `(%2d%s)%s之%s請%s將軍紮寨`）。
	for _, u := range p.Battle().Units {
		if u.Side.Attacking() && u.Alive() {
			f.camping = append(f.camping, u)
		}
	}
	a.nextCamp()
}

// nextCamp 問下一支部隊要紮在哪裡；紮完就開打。
func (a *app) nextCamp() {
	f := a.fight
	for len(f.camping) > 0 && !f.camping[0].Alive() {
		f.camping = f.camping[1:]
	}
	if len(f.camping) == 0 {
		f.view.Menu, f.view.Items = "", nil
		a.nextActor()
		return
	}
	u := f.camping[0]
	f.acting = u
	f.waiting = waitCamp
	f.view.Acting = u
	f.view.Cursor = ui.Hexer{At: u.At, Shown: true}
	f.view.Menu, f.view.Items = t("bat.camp"),
		[]string{t("bat.arrowKeys"), t("bat.place"), t("bat.autoAll")}
	f.view.Prompt = tf("bat.campWho", u.Name(), len(f.camping))
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
	f.view.Menu, f.view.Items = t("page.command"), ui.BattleCommandLines()
	f.view.Prompt = tf("bat.unitMoves", f.acting.Name(), f.acting.Move)
	f.view.Page, f.view.PageTitle = nil, ""
}

// endBattle 把打完的戰役搬回局面，回到主畫面。
func (a *app) endBattle() {
	r := a.s.G.FinishAttack(a.fight.pending)
	a.fight = nil
	a.s.Drain()
	if r != nil {
		a.view.SetPage(ui.BattleReport(a.s.G, r))
	}
	a.view.Prompt = t("bat.finished")
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
			say("%s", game.ErrorText(err))
			f.waiting = waitCommand
			f.view.Menu, f.view.Items = t("page.command"), ui.BattleCommandLines()
			return
		}
		// 一支部隊一天可以做好幾件事；移動之後還有餘步就繼續。
		if f.acting.Alive() && f.acting.Move > 0 && f.cmd == battle.CmdMove {
			f.waiting = waitDir
			say(tf("bat.dirMore", f.acting.Move))
			return
		}
		f.runner.Done()
		a.nextActor()
	}

	switch f.waiting {
	case waitCamp:
		switch k {
		case '9':
			// 剩下的照 formUp 排好的位置紮，直接開打。
			f.camping = nil
			a.nextCamp()
		case '0':
			if err := b.Camp(f.acting, f.view.Cursor.At); err != nil {
				say("%s", game.ErrorText(err))
				return
			}
			f.camping = f.camping[1:]
			a.nextCamp()
		default:
			say(t("bat.campHint"))
		}
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
			say(t("bat.dirBad"))
			return
		}
		done(a.applyDir(d))
	case waitPlot:
		s := battle.Stratagem(k - '0')
		if s < battle.Fire || s > battle.Siege {
			say(t("bat.plotBad"))
			return
		}
		target := b.UnitAt(f.view.Cursor.At)
		if target == nil {
			say(t("bat.noTarget"))
			return
		}
		done(b.UseStratagem(f.acting, s, target.At))
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
		f.view.Menu, f.view.Items = ui.CommandName(f.cmd), []string{"4 5 6", "1 2 3"}
		say(t("bat.dir"))
	case battle.CmdEngage:
		f.engage = true
		f.waiting = waitEngage
		f.view.Menu, f.view.Items = ui.CommandName(battle.CmdEngage), ui.BattleEngageLines()
		say(tf("bat.engageHint", strings.Join(ui.BattleEngageLines(), " ")))
	case battle.CmdPlot:
		f.waiting = waitPlot
		f.view.Menu, f.view.Items = ui.CommandName(battle.CmdPlot), ui.BattleStratagemLines()
		say(t("bat.plotWho"))
	case battle.CmdInspect:
		u, err := b.Inspect(f.acting, f.view.Cursor.At)
		if err != nil {
			say("%s", game.ErrorText(err))
			return
		}
		f.view.SetPage(ui.BattleUnitPage(u))
		say(t("bat.close"))
	case battle.CmdRetreat:
		done(b.Retreat(f.acting))
	default:
		say(t("bat.cmdBad"))
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
		say(tf("bat.marchDir", f.acting.Move))
	case '2':
		f.cmd = battle.CmdEngage
		f.waiting = waitDir
		say(t("bat.duelDir"))
	case '3':
		f.cmd = battle.CmdQuick
		f.waiting = waitDir
		say(t("bat.strikeDir"))
	case '7':
		u, err := b.Inspect(f.acting, f.view.Cursor.At)
		if err != nil {
			say("%s", game.ErrorText(err))
			return
		}
		f.view.SetPage(ui.BattleUnitPage(u))
		say(t("bat.close"))
	default:
		say(t("bat.engageBad"))
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
		// 對戰子畫面（原版 `0x2deb0`）。玩家那一方的將領還沒有介面，
		// 先照電腦的判斷式走（`docs/mechanics/40` §8）。
		_, err := b.Engage(f.acting, d, nil)
		return err
	case battle.CmdArchery:
		// 「相間一格」：同一方向連走兩步。
		return b.Archery(f.acting, f.acting.At.Step(d).Step(d))
	}
	return fmt.Errorf("%s", t("bat.noDir"))
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

// battleInfo 是主戰場畫面上那些戰術層自己不知道的東西：郡名、州名、
// 郡編號、兩軍統帥的姓名與肖像。
func (a *app) battleInfo() ui.ArtBattleInfo {
	info := ui.ArtBattleInfo{Portrait: [2]int{-1, -1}}
	if a.s != nil && a.s.G != nil {
		info.Date, info.Calendar = a.s.G.Date, a.s.G.Options.Calendar
	}
	if a.fight == nil {
		return info
	}
	if p := a.s.G.Prefecture(a.fight.pending.Where()); p != nil {
		info.Prefecture = p.Name
		info.Province = state.ProvinceName(int(p.Province))
		info.Field = p.BattleField
		info.ID = p.ID
	}
	att, def := a.fight.pending.Chiefs()
	for i, who := range []*game.General{att, def} {
		if who == nil {
			continue
		}
		info.Commander[i] = who.Name
		info.Portrait[i] = int(who.Portrait)
	}
	return info
}
