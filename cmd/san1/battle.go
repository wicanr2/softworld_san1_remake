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
	"iter"
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

	// speeches 是還沒畫的戰場對白（原版的訊息框，`docs/spec/005` §9.7），
	// 一格一格按任意鍵收；從 `Battle.TakeSpeeches` 補進來。
	speeches []battle.Speech
	// ending 表示戰役已經打完，對白收完就回主畫面（`endBattle`）。
	ending bool

	view ui.BattleView

	// 戰術層協程（`docs/spec/018` R4）。戰術層是同步的：被擒處置與中途
	// 紮寨要在它跑到一半時問玩家。每一次會推進戰術層的操作都包在
	// `iter.Pull` 裡（`inEngine`），回呼把問題 yield 出來；UI 收到答案再
	// `resumeEngine`。兩邊一次只有一方在跑。
	next   func() (engineAsk, bool)
	stop   func()
	yield  func(engineAsk) bool
	asking *engineAsk
	// saved 是問之前的選單與提示，答完還原。
	saved ui.BattleView
	// answerFate／answerAt 是玩家剛答的處置與紮寨格。
	answerFate battle.Fate
	answerAt   battle.Hex
	// answerCmd／answerYes 是子畫面的一道命令與叫陣的答案；skm 是子畫面
	// 選單的子狀態。
	answerCmd battle.SkirmishCommand
	answerYes bool
	skm       byte
	// blinkTick 數幀：子畫面裡輪到的那一格每 blinkFrames 幀反白切換一次
	// （原版是計時器，`0x1538:0x58ac`；remake 差異：速度不同）。
	blinkTick int
}

const blinkFrames = 20

// engineAsk 是戰術層停下來問玩家的一件事：被擒的那一位、要紮寨的部隊、
// 對戰子畫面裡輪到的將領，或被叫陣的將領。
type engineAsk struct {
	captive *battle.Leader
	camp    *battle.Unit
	// skirmish 是子畫面裡輪到的玩家將領（`0x2fb14` 的選單）。
	skirmish *battle.SkirmishGeneral
	// challenged 是子畫面裡被叫陣的玩家將領，challenger 是叫陣的人
	// （「接受嗎(Y/N)」，`0x30d06`）。
	challenged, challenger *battle.SkirmishGeneral
}

// 子畫面選單的子狀態：選單本身、休息確認、行軍中、等單挑或攻擊的方向。
const (
	skmMenu byte = iota
	skmRest
	skmMarch
	skmDuel
	skmAttack
)

// inEngine 在協程裡跑 body。body 裡的戰術層呼叫可能停下來問玩家；
// 沒問就一路跑完。
func (a *app) inEngine(body func()) {
	f := a.fight
	if f == nil {
		return
	}
	if f.next != nil {
		body() // 已經在協程裡（body 裡又走到會推進戰術層的路）
		return
	}
	f.next, f.stop = iter.Pull(func(yield func(engineAsk) bool) {
		f.yield = yield
		body()
	})
	a.resumeEngine(f)
}

// resumeEngine 讓協程跑到下一個問題或跑完。
func (a *app) resumeEngine(f *fight) {
	q, ok := f.next()
	if !ok {
		f.stop()
		f.next, f.stop, f.yield, f.asking = nil, nil, nil, nil
		return
	}
	f.asking = &q
	f.saved = f.view
	switch {
	case q.captive != nil:
		f.view.Menu = t("bat.captive")
		f.view.Items = strings.Split(t("bat.captiveLines"), "|")
		f.view.Prompt = tf("bat.captiveWho", q.captive.Name)
	case q.camp != nil:
		f.view.Acting = q.camp
		f.view.Cursor = ui.Hexer{At: q.camp.At, Shown: true}
		f.view.Menu, f.view.Items = t("bat.camp"), []string{t("bat.arrowKeys"), t("bat.place")}
		f.view.Prompt = tf("bat.campMid", q.camp.Name())
	case q.skirmish != nil:
		f.skm = skmMenu
		f.skirmishPrompt(q.skirmish)
	case q.challenged != nil:
		f.view.SkirmishActing = q.challenged
		f.view.Menu, f.view.Items = t("bat.duel"), nil
		f.view.Prompt = tf("skm.accept", q.challenger.Leader.Name)
	}
}

// skirmishPrompt 是子畫面選單（`0x2fb14`）：「1.行軍 2.單挑 3.攻擊 /
// 7.查看 0.休息」，下一行「名字(餘步/移動力)(0-4):」。
func (f *fight) skirmishPrompt(g *battle.SkirmishGeneral) {
	f.view.SkirmishActing = g
	f.view.Menu = ""
	f.view.Items = strings.Split(t("skm.menu"), "|")
	switch f.skm {
	case skmRest:
		f.view.Prompt = t("skm.restConfirm")
	case skmMarch:
		f.view.Prompt = tf("bat.marchDir", g.Left)
	case skmDuel:
		f.view.Prompt = t("bat.duelDir")
	case skmAttack:
		f.view.Prompt = t("bat.strikeDir")
	default:
		f.view.Prompt = tf("skm.prompt", g.Leader.Name, g.Left, g.MoveCap)
	}
}

// answerSkirmish 收子畫面裡的一個鍵（`docs/re/05` §10.6）。回 true 表示
// 已經有一道命令，要交回戰術層。
func (f *fight) answerSkirmish(g *battle.SkirmishGeneral, k byte) bool {
	dir := func() (battle.Dir, bool) {
		if k >= '1' && k <= '6' {
			return battle.Dir(k - '0'), true
		}
		return 0, false
	}
	switch f.skm {
	case skmRest:
		f.skm = skmMenu
		if k == 'Y' {
			f.answerCmd = battle.SkirmishCommand{Kind: battle.SkirmishRest}
			return true
		}
	case skmMarch:
		if k == '\r' {
			f.answerCmd = battle.SkirmishCommand{Kind: battle.SkirmishMarchDone}
			f.skm = skmMenu
			return true
		}
		if d, ok := dir(); ok {
			f.answerCmd = battle.SkirmishCommand{Kind: battle.SkirmishMarch, Dir: d}
			return true
		}
	case skmDuel, skmAttack:
		kind := battle.SkirmishDuel
		if f.skm == skmAttack {
			kind = battle.SkirmishAttack
		}
		f.skm = skmMenu
		if d, ok := dir(); ok {
			f.answerCmd = battle.SkirmishCommand{Kind: kind, Dir: d}
			return true
		}
	default:
		switch k {
		case '0':
			f.skm = skmRest
		case '1':
			f.skm = skmMarch
		case '2':
			f.skm = skmDuel
		case '3':
			f.skm = skmAttack
		case '7':
			f.view.SetPage(ui.BattleUnitPage(g.Unit))
			f.view.Inspecting = g.Unit
		}
	}
	f.skirmishPrompt(g)
	return false
}

// answerEngine 收玩家對 `asking` 的回答：被擒是 1–4，紮寨是 0（游標那一格）。
// 其他鍵不理（原版讀鍵迴圈也是，`0x25af0`）。
func (a *app) answerEngine(k byte) {
	f := a.fight
	q := f.asking
	switch {
	case q.captive != nil:
		fates := map[byte]battle.Fate{'1': battle.Executed, '2': battle.Jailed, '3': battle.Released, '4': battle.Defected}
		fate, ok := fates[k]
		if !ok {
			return
		}
		f.answerFate = fate
	case q.camp != nil:
		if k != '0' {
			return
		}
		f.answerAt = f.view.Cursor.At
	case q.skirmish != nil:
		if !f.answerSkirmish(q.skirmish, k) {
			return
		}
	case q.challenged != nil:
		switch k {
		case 'Y':
			f.answerYes = true
		case 'N':
			f.answerYes = false
		default:
			return
		}
	}
	cursor := f.view.Cursor
	f.view = f.saved
	if q.camp != nil {
		f.view.Cursor = cursor
	}
	f.asking = nil
	a.resumeEngine(f)
}

// hookEngine 把兩個回呼接到這一場的戰術層上。
func (f *fight) hookEngine() {
	b := f.pending.Battle()
	b.PlayerCaptive = func(_ battle.Side, x *battle.Leader) battle.Fate {
		f.yield(engineAsk{captive: x})
		return f.answerFate
	}
	b.PlayerCamp = func(u *battle.Unit) battle.Hex {
		f.yield(engineAsk{camp: u})
		return f.answerAt
	}
	b.PlayerSkirmish = func(_ *battle.Skirmish, g *battle.SkirmishGeneral) battle.SkirmishCommand {
		f.yield(engineAsk{skirmish: g})
		return f.answerCmd
	}
	b.PlayerDuelAnswer = func(_ *battle.Skirmish, g, t *battle.SkirmishGeneral) bool {
		f.yield(engineAsk{challenger: g, challenged: t})
		return f.answerYes
	}
}

// speech 是現在該畫的那一句戰場對白；沒有就是 nil。文字版面（沒有原版
// 素材）畫不出肖像，直接丟掉——戰報文字本來就有那一句。
func (f *fight) speech(art bool) *battle.Speech {
	if f.pending != nil {
		f.speeches = append(f.speeches, f.pending.Battle().TakeSpeeches()...)
	}
	if !art {
		f.speeches = nil
	}
	if len(f.speeches) == 0 {
		return nil
	}
	return &f.speeches[0]
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
	f.hookEngine()
	// 開戰前逐隊紮營（原版 `(%2d%s)%s之%s請%s將軍紮寨`）。
	for _, u := range p.Battle().Units {
		if u.Side.Attacking() && u.Alive() {
			f.camping = append(f.camping, u)
		}
	}
	a.inEngine(a.nextCamp)
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

// nextActor 推進到下一支要玩家下令的部隊；沒有就收尾——打完先讓助軍
// 回郡那一句（`0x25652`）與其餘還沒畫的對白畫完再回主畫面（`ending`）。
func (a *app) nextActor() {
	f := a.fight
	f.acting = f.runner.Next()
	f.engage, f.waiting = false, waitCommand
	if f.acting == nil {
		f.pending.Battle().SayHelperReturn()
		if a.fight.speech(a.artBattle != nil) != nil {
			f.ending = true
			return
		}
		a.endBattle()
		return
	}
	f.view.Acting = f.acting
	f.view.Cursor = ui.Hexer{At: f.acting.At, Shown: true}
	f.view.Menu, f.view.Items = t("page.command"), ui.BattleCommandLines()
	f.view.Prompt = tf("bat.unitMoves", f.acting.Name(), f.acting.Move)
	f.view.ClosePage()
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

// battleKey 收戰場上的一個按鍵。戰術層停著問玩家時交給 `answerEngine`，
// 否則在協程裡處理（`battleKeyStep`）。
func (a *app) battleKey(k byte) {
	f := a.fight
	if f == nil {
		return
	}
	if f.asking != nil {
		a.answerEngine(k)
		return
	}
	a.inEngine(func() { a.battleKeyStep(k) })
}

// battleKeyStep 是一個按鍵對戰術層做的事。
func (a *app) battleKeyStep(k byte) {
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
		err := b.UseStratagem(f.acting, s, target.At)
		// 三道門各一句（`0x28cd5`，第三塊面板，第 0 槽那一位）——說完回到
		// 指令提示，回合不算用掉。
		b.SayPlotGate(f.acting, err)
		done(err)
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
		f.view.Inspecting = u
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
		f.view.Inspecting = u
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
		_, err := b.Engage(f.acting, d, b.PlayerSkirmish)
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

// unitPanels 是兩支部隊的部隊面板（攻方陣營、守方陣營）。
func (a *app) unitPanels(units [2]*battle.Unit) *[2]ui.UnitPanel {
	var panels [2]ui.UnitPanel
	for i, u := range units {
		panels[i] = ui.UnitPanel{Unit: u, Portrait: -1}
		if u == nil {
			continue
		}
		if head := u.Head(); head != nil {
			if x := a.s.G.General(head.Index); x != nil {
				panels[i].Portrait = int(x.Portrait)
				if lord := a.s.G.Lord(x.Faction); lord != nil {
					panels[i].Lord = lord.Name
				}
			}
		}
	}
	return &panels
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
		if lord := a.s.G.Lord(who.Faction); lord != nil {
			info.Lord[i] = lord.Name
		}
	}
	// 對戰子畫面裡的對白：那時兩塊軍力面板是子畫面裡那兩支部隊的面板
	// （`docs/spec/005` §8「部隊面板」）。
	if sp := a.fight.speech(a.artBattle != nil); sp != nil && (sp.Units[0] != nil || sp.Units[1] != nil) {
		info.Units = a.unitPanels(sp.Units)
	}
	// 對戰子畫面停下來問玩家的時候畫子畫面：子地圖、將領標記、那兩支
	// 部隊的面板（`docs/spec/005` §8「對戰子畫面」）。
	if sk := a.fight.pending.Battle().InSkirmish(); sk != nil {
		info.Skirmish = sk
		if info.Units == nil {
			var pair [2]*battle.Unit
			for _, u := range sk.Units {
				if u.Side.Attacking() {
					pair[0] = u
				} else {
					pair[1] = u
				}
			}
			info.Units = a.unitPanels(pair)
		}
	}
	// 查看：第三塊面板換成那支部隊第 0 槽那一位（`docs/spec/005` §8「查看」）。
	if u := a.fight.view.Inspecting; u != nil {
		if head := u.Head(); head != nil {
			ins := &ui.InspectPanel{Leader: head, Side: u.Side, Portrait: -1}
			if x := a.s.G.General(head.Index); x != nil {
				ins.Portrait = int(x.Portrait)
			}
			info.Inspect = ins
		}
	}
	return info
}
