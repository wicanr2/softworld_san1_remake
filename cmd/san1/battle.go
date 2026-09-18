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

	// plotAt 是「設計那一軍」選好的目標格（相鄰那一格）。
	plotAt battle.Hex

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
	// answerPref 是玩家剛答的郡編號（退兵的去處；0 ＝ 取消）。
	answerPref int
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
	// retreat 是正在退兵的部隊，escapes 是逃得去的郡（`0x2408e`）。
	retreat *battle.Unit
	escapes []battle.Escape
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
	// 戰術層問的這幾格還是選單＋提示的畫法（被擒處置、中途紮寨、子畫面）；
	// 文字視窗在答完、還原 saved 時回來。
	f.view.Window = ""
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
	case q.retreat != nil:
		f.view.Acting = q.retreat
		f.view.Menu, f.view.Items = t("bat.retreatTo"), retreatItems(a.s.G, q.escapes)
		f.view.Prompt = tf("bat.retreatWho", q.retreat.Name())
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
	case q.retreat != nil:
		// `0` ＝ 取消整個退兵（原版是空欄位 Enter，`0x2416a`）。
		if k == '0' {
			f.answerPref = 0
			break
		}
		i := int(k - '1')
		if i < 0 || i >= len(q.escapes) {
			return
		}
		f.answerPref = q.escapes[i].Prefecture
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
	// 「%s逃向那一郡」（`0x2408e`，Issue #101）：收郡編號，0 ＝ 取消退兵。
	b.PlayerRetreat = func(u *battle.Unit, cands []battle.Escape) int {
		f.yield(engineAsk{retreat: u, escapes: cands})
		return f.answerPref
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
	waitCommand   waitFor = iota // 等部隊層的指令
	waitDir                      // 等方向
	waitPlotDir                  // 用計：等「設計那一軍」的方向
	waitPlot                     // 等計謀編號
	waitRestYN                   // 「休息 確認(Y/N)」
	waitRetreatYN                // 「退兵 確認(Y/N)」
	waitCamp                     // 開戰前紮營
)

// takesYN 回報這一格收不收 Y／N／Enter。
func (w waitFor) takesYN() bool {
	return w == waitRestYN || w == waitRetreatYN || w == waitPlotDir || w == waitPlot
}

// startBattle 開一場由玩家指揮的戰役。
// startBattle 開一場玩家親自指揮的戰役。**at 是下令的郡（回合記在它身上），from 是出兵的郡**
// ——原版「從那一郡攻打」收任何自己的郡（Issue #82）。
func (a *app) startBattle(at, from, to int, force []int, groups []int, sup game.Supply) {
	if w := a.s.Waiting(); w != 0 && at != w {
		a.view.Prompt = tf("msg.notThisPref", prefName(a.s.G, w))
		return
	}
	p, err := a.s.G.BeginAttack(from, to, force, a.s.Player, sup)
	if err != nil {
		a.view.Prompt = tf("bat.attackFailed", err)
		return
	}
	// 整編照玩家分的（Issue #98）：`BeginAttack` 建好的是預設分隊，
	// 這裡按玩家的答案重編。**只挑出征的那幾位**，所以 groups 要先濾成
	// 與 `force` 對齊的那一份。
	if len(groups) > 0 {
		var mine []int
		for _, k := range groups {
			if k > 0 {
				mine = append(mine, k)
			}
		}
		if err := p.Battle().Reform(battle.MainAttacker, mine); err != nil {
			a.view.Prompt = tf("bat.attackFailed", err)
			return
		}
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

// startDefence 是「電腦來攻、玩家親自守」那一場（Issue #64）：整編已經在
// `game.ComputerAttack` 做完了，這裡只把指揮權接過來。與 `startBattle`
// 差兩處——這一側是守方，紮寨的也是守方。
func (a *app) startDefence() {
	p := a.s.G.PendingDefence()
	if p == nil {
		return
	}
	// **守方也要整編**（原版 `0x20a30` 對四個軍團各跑一輪，Issue #98）。
	// 差在主守軍**必須派出所有兵力**（說明書 p.27），所以每一位都得分到
	// 某一軍，不能留在家裡——`askAssign` 收工時再補齊沒分到的那幾位。
	var pool []int
	for _, u := range p.Battle().Units {
		if !u.Side.Attacking() && u.Alive() {
			for _, l := range u.Leaders {
				pool = append(pool, l.Index)
			}
		}
	}
	if a.form != nil {
		return // 整編的問句正在進行中（`startDefence` 每一幀都會被叫到）
	}
	if len(pool) > 1 {
		a.askAssign(&formation{pool: pool, groups: make([]int, len(pool)),
			fillRest: true,
			then:     func(_ []int, groups []int) { a.defendWith(p, groups) }})
		return
	}
	a.defendWith(p, nil)
}

// defendWith 是守方整編之後那一段：重編、接指揮權、逐隊紮寨。
func (a *app) defendWith(p *game.Pending, groups []int) {
	if len(groups) > 0 {
		if err := p.Battle().Reform(battle.MainDefender, groups); err != nil {
			a.view.Prompt = tf("bat.attackFailed", err)
		}
	}
	f := &fight{pending: p}
	f.runner = battle.NewRunner(p.Battle(), func(s battle.Side) bool {
		return !s.Attacking()
	})
	a.fight = f
	f.hookEngine()
	for _, u := range p.Battle().Units {
		if !u.Side.Attacking() && u.Alive() {
			f.camping = append(f.camping, u)
		}
	}
	a.inEngine(a.nextCamp)
}

// retreatItems 把逃得去的郡列成選單（Issue #101）。
//
// **remake 差異**：原版收的是**郡編號**（`0x24147` 的 `0x115e(1, 42)`，
// 螢幕上另外列出候選郡名）；主戰場這一層沒有數字欄位，只收一個鍵
// （`docs/re/05` §7.0），所以這裡列成 1–n 的清單，`0` 取消。
// 收到的郡是同一組，差的是打法。
func retreatItems(g *game.State, cands []battle.Escape) []string {
	out := make([]string, 0, len(cands)+1)
	for i, e := range cands {
		out = append(out, fmt.Sprintf("%d.%s", i+1, prefName(g, e.Prefecture)))
	}
	return append(out, t("bat.retreatStay"))
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
	f.view.Menu, f.view.Items, f.view.Prompt = "", nil, ""
	f.view.Window = a.campWindow(u)
}

// campWindow 是紮寨那一格的文字視窗（`ui.BattleCampWindow`）。
func (a *app) campWindow(u *battle.Unit) string {
	id := a.fight.pending.SidePrefecture(u.Side)
	name := ""
	if p := a.s.G.Prefecture(id); p != nil {
		name = p.Name
	}
	return ui.BattleCampWindow(id, name, u.Side, u.Formation, leaderName(u))
}

// commandWindow 是每天命令提示的文字視窗（`ui.BattleCommandWindow`）：君主是
// 帶隊那一位所屬勢力的君主。
func (a *app) commandWindow(u *battle.Unit) string {
	lord := ""
	if len(u.Leaders) > 0 {
		if x := a.s.G.General(u.Leaders[0].Index); x != nil {
			if l := a.s.G.Lord(x.Faction); l != nil {
				lord = l.Name
			}
		}
	}
	return ui.BattleCommandWindow(lord, u.Formation, u.Move, leaderName(u))
}

// leaderName 是部隊第 0 槽那一位的名字（原版部隊記錄 offset 38 那一格）。
func leaderName(u *battle.Unit) string {
	if len(u.Leaders) == 0 {
		return ""
	}
	return u.Leaders[0].Name
}

// nextActor 推進到下一支要玩家下令的部隊；沒有就收尾——打完先讓助軍
// 回郡那一句（`0x25652`）與其餘還沒畫的對白畫完再回主畫面（`ending`）。
func (a *app) nextActor() {
	f := a.fight
	f.acting = f.runner.Next()
	f.waiting = waitCommand
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
	f.view.Menu, f.view.Items, f.view.Prompt = "", nil, ""
	f.view.Window = a.commandWindow(f.acting)
	f.view.ClosePage()
}

// endBattle 把打完的戰役搬回局面，回到主畫面。
func (a *app) endBattle() {
	// 玩家自己守的那一場由 session 收尾（它還要把月游標往下推）。
	if a.s.G.PendingDefence() == a.fight.pending {
		a.fight = nil
		a.s.FinishDefence()
		a.view.Prompt = t("bat.finished")
		return
	}
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
	// say 在文字視窗裡印一句（原版的訊息常式接著寫在視窗裡）。
	say := func(format string, v ...any) { f.view.Window = fmt.Sprintf(format, v...) }
	// backToCommand 回到每天的命令提示；msg 不是空字串就先印它再印選單。
	backToCommand := func(msg string) {
		f.waiting = waitCommand
		f.view.Window = a.commandWindow(f.acting)
		if msg != "" {
			f.view.Window = msg + "\n" + f.view.Window
		}
	}
	done := func(err error) {
		if err != nil {
			backToCommand(game.ErrorText(err))
			return
		}
		// 一支部隊一天可以做好幾件事；移動之後還有餘步就繼續。
		if f.acting.Alive() && f.acting.Move > 0 && f.cmd == battle.CmdMove {
			f.waiting = waitDir
			f.view.Window = ui.BattleDirWindow(battle.CmdMove, f.acting)
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
				say("%s\n%s", game.ErrorText(err), a.campWindow(f.acting))
				return
			}
			f.camping = f.camping[1:]
			a.nextCamp()
		default:
			// 原版 1–6 移游標（`0x21d57`）；方向鍵也可以（remake 加的）。
			if d, ok := dirFromKey(k); ok {
				a.battleMove(d)
			}
		}
	case waitCommand:
		a.battleCommand(k, done, say)
	case waitRestYN, waitRetreatYN:
		switch k {
		case 'Y':
			if f.waiting == waitRestYN {
				done(b.Rest(f.acting))
			} else {
				done(b.Retreat(f.acting))
			}
		case 'N', '\r':
			backToCommand("")
		}
	case waitPlotDir:
		// 「設計那一軍」：方向鍵指相鄰那一格，那一格沒有敵軍就取消回命令提示
		// （`0x28d7a` 回 0xFFFF，`docs/re/05` §4）。Enter 取消。
		if k == '\r' {
			backToCommand("")
			return
		}
		d, ok := dirFromKey(k)
		if !ok {
			return
		}
		at := f.acting.At.Step(d)
		if u := b.UnitAt(at); u == nil || u.Side.Attacking() == f.acting.Side.Attacking() {
			backToCommand("")
			return
		}
		f.plotAt = at
		f.waiting = waitPlot
		f.view.Window = t("bat.win.plotList")
	case waitDir:
		if k == '0' {
			f.runner.Done()
			a.nextActor()
			return
		}
		d, ok := dirFromKey(k)
		if !ok {
			return // 原版不是 1–6 就重讀
		}
		done(a.applyDir(d))
	case waitPlot:
		if k == '\r' {
			backToCommand("")
			return
		}
		s := battle.Stratagem(k - '0')
		if s < battle.Fire || s > battle.Siege {
			return // 原版不在 1–6 就重讀
		}
		err := b.UseStratagem(f.acting, s, f.plotAt)
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
		f.waiting = waitRestYN
		f.view.Window = t("bat.win.restYN")
	case battle.CmdMove, battle.CmdQuick, battle.CmdDeath, battle.CmdArchery, battle.CmdEngage:
		f.waiting = waitDir
		f.view.Window = ui.BattleDirWindow(f.cmd, f.acting)
	case battle.CmdPlot:
		f.waiting = waitPlotDir
		f.view.Window = t("bat.win.plotDir")
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
		f.waiting = waitRetreatYN
		f.view.Window = t("bat.win.retreatYN")
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
		// 對戰子畫面（原版 `0x2deb0`）。玩家那一方的將領走 `PlayerSkirmish`
		// 那個介面（Issue #56）；沒有介面的那一方才照電腦的判斷式
		// （`docs/mechanics/40` §8）。
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
