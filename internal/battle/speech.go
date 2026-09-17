package battle

import (
	"errors"

	"github.com/wicanr2/softworld_san1_remake/internal/i18n"
)

// 戰場上的對白（原版的訊息常式 `0x3273e`，`docs/spec/005` §9.7，`L0`、`[base]`）。
//
// 主戰場上一句話就是第三塊面板那一格：肖像在右（只有「汝計已被吾識破」
// 肖像在左）、說話者是部隊的統帥或當事的將領。單挑的話用兩塊側面板：
// 攻方那一塊肖像在左、守方那一塊肖像在右（`DS:0x7b24`／`0x7b34` 的位置表
// 索引 `版面×4＋側`、`DS:0x8894` 的左右表，`0x30a43`–`0x30b9b`）。
// 三塊面板在畫面上的位置隨版面走——寬版面排在場地下方、窄版面疊在右邊
// （`assets.BattleLayout.Panel`），這裡只記是哪一塊。字色照原版是進去就擲的
// `RND(8)`——這一擲 remake 本來就在擲（`msg`），現在把值留下來畫。

// SpeechBox 是對白用哪一塊面板。
type SpeechBox int

const (
	BoxThird    SpeechBox = iota // 第三塊面板（主戰場的對白）
	BoxAttacker                  // 攻方那一塊（單挑）
	BoxDefender                  // 守方那一塊（單挑）
)

// Speech 是一句對白：說話者（人物槽）、哪一塊、肖像在左、字色、內容。
//
// Units 是說這句話時對戰子畫面裡的兩支部隊（攻方陣營那一支、守方陣營
// 那一支），不在子畫面裡就都是 nil。原版進子畫面（`0x2deb0`）先把兩塊
// 軍力面板換成這兩支部隊的面板（`0x320a6`：第 0 槽那一位的肖像與名字、
// 君主、軍力名、隊伍名與將數、兵數），對白就畫在那樣的畫面上。
type Speech struct {
	Speaker int
	Box     SpeechBox
	Left    bool
	Color   int
	Text    string
	Units   [2]*Unit

	// Scene 不是 0 時這一格不是對白，是一張場景圖 `SCG%02d` 拉進第三塊
	// 面板 (448,268)（`0x32dfa`，`docs/spec/010`）：Style 是 `RND(4)` 挑的
	// 拉幕方向。
	Scene, Style int

	// LureFlash 為真時這一格是誘敵成功、對白之後在施法者那一格閃的特效
	// （`0x2b783`，`docs/spec/005` §8「誘敵的特效」）：At 是那一格。不擲骰。
	LureFlash bool
	At        Hex
}

// Panel 是這一塊在版面裡的面板編號：攻方 0、守方 1、指令列（第三塊）2，
// 與 `assets.BattleLayout.Panel` 的順序相同。
func (b SpeechBox) Panel() int {
	switch b {
	case BoxAttacker:
		return 0
	case BoxDefender:
		return 1
	}
	return 2
}

// scene 播一段特效：擲拉幕那一擲（與 `fx` 同一擲），把場景圖排進 Speeches。
func (b *Battle) scene(n int) {
	style := b.fx()
	b.Speeches = append(b.Speeches, Speech{Box: BoxThird, Scene: n, Style: style})
}

// say 印一句對白：擲字色那一擲（與 `msg` 同一擲），排進 Speeches。
// speaker 為 nil 時只擲不排（沒有人可畫）。
func (b *Battle) say(speaker *Leader, box SpeechBox, left bool, key string, a ...any) {
	c := b.msg()
	if speaker == nil {
		return
	}
	sp := Speech{Speaker: speaker.Index, Box: box, Left: left, Color: c, Text: i18n.Sf(key, a...)}
	if s := b.inSkirmish; s != nil {
		for _, u := range s.Units {
			if u.Side.Attacking() {
				sp.Units[0] = u
			} else {
				sp.Units[1] = u
			}
		}
	}
	b.Speeches = append(b.Speeches, sp)
}

// sayUnit 是部隊的統帥在第三塊面板說一句。
func (b *Battle) sayUnit(u *Unit, key string, a ...any) {
	b.say(u.Chief(), BoxThird, false, key, a...)
}

// duelBox 是單挑時這位將領那一塊面板與肖像的左右：攻方左、守方右。
func duelBox(side Side) (SpeechBox, bool) {
	if side.Attacking() {
		return BoxAttacker, true
	}
	return BoxDefender, false
}

// TakeSpeeches 交出累積的對白，交出就清掉。
func (b *Battle) TakeSpeeches() []Speech {
	out := b.Speeches
	b.Speeches = nil
	return out
}

// 玩家下計謀時的三道門（`0x28bc2`–`0x28cac`）：錢不夠、領隊謀略不夠、
// 天候地理不合，各配一句對白（479／480／481），由那支部隊第 0 槽那一位
// 在第三塊面板說（`0x28cd5`–`0x28d48`，先填藍，肖像在右），說完等鍵回
// 到提示。電腦下計謀走另一條檢查（`0x2a22:0x1cc8`），不說話。
var (
	ErrPlotGold  = errors.New("資金不足 無法用計")
	ErrPlotIntel = errors.New("將軍謀略不足 無法用計")
	ErrPlotPlace = errors.New("天侯地理因素 無法用計")
)

// SayPlotGate 把 `UseStratagem` 撞到的那一道門說出來（玩家那一邊才叫）。
// 不是三道門之一（沒目標、不相鄰、友軍）原版是直接取消，不說話。
func (b *Battle) SayPlotGate(u *Unit, err error) bool {
	key := ""
	switch {
	case errors.Is(err, ErrPlotGold):
		key = "bub.plotGold"
	case errors.Is(err, ErrPlotIntel):
		key = "bub.plotIntel"
	case errors.Is(err, ErrPlotPlace):
		key = "bub.plotPlace"
	default:
		return false
	}
	b.say(u.Head(), BoxThird, false, key)
	return true
}

// SayHelperReturn 是打完之後助軍回郡那一句（`0x25652`，日循環結束、印完
// 勝負之後 `0x23d04` 叫）：勝方的助軍還有將領、勝方的主軍也還有將領，
// 助軍的統帥在第三塊面板說 478「吾軍眾將聽令 回本郡駐守」。只說一次。
func (b *Battle) SayHelperReturn() bool {
	if !b.Over || b.helperSaid {
		return false
	}
	b.helperSaid = true
	main, helper := MainDefender, AidDefender
	if b.AttackerWon {
		main, helper = MainAttacker, AidAttacker
	}
	if b.leaderCount(helper) == 0 || b.leaderCount(main) == 0 {
		return false
	}
	b.say(b.commanderOf(helper), BoxThird, false, "bub.helperReturn")
	return true
}

// leaderCount 是一個軍力還在隊裡的將領數（軍力記錄 offset 12）。
func (b *Battle) leaderCount(s Side) int {
	n := 0
	for _, u := range b.Units {
		if u.Side == s {
			n += u.LeaderCount()
		}
	}
	return n
}

// commanderOf 是一個軍力的統帥（軍力記錄 offset 0）；找不到就拿第一支
// 部隊第 0 槽那一位。
func (b *Battle) commanderOf(s Side) *Leader {
	id := b.Commander[s]
	for _, u := range b.Units {
		if u.Side != s {
			continue
		}
		for i := range u.Leaders {
			if u.Leaders[i].Index == id && id >= 0 {
				return &u.Leaders[i]
			}
		}
	}
	for _, u := range b.Units {
		if u.Side == s && u.Head() != nil {
			return u.Head()
		}
	}
	return nil
}
