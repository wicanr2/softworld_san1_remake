package battle

import "github.com/wicanr2/softworld_san1_remake/internal/i18n"

// 戰場上的對白（原版的訊息常式 `0x3273e`，`docs/spec/005` §9.7，`L0`、`[base]`）。
//
// 主戰場上一句話就是第三塊面板那一格：框 (448,268)–(623,363)、肖像在右
// （只有「汝計已被吾識破」肖像在左）、說話者是部隊的統帥或當事的將領。
// 單挑的話用兩塊側面板：攻方那一塊 (64,268)–(239,363) 肖像在左、守方那一塊
// (256,268)–(431,363) 肖像在右（`DS:0x7b24`／`0x7b34` 的位置表、`DS:0x8894`
// 的左右表，`0x30a43`–`0x30b9b`）。字色照原版是進去就擲的 `RND(8)`——
// 這一擲 remake 本來就在擲（`msg`），現在把值留下來畫。

// SpeechBox 是對白用哪一塊面板。
type SpeechBox int

const (
	BoxThird    SpeechBox = iota // 第三塊面板（主戰場的對白）
	BoxAttacker                  // 攻方那一塊（單挑）
	BoxDefender                  // 守方那一塊（單挑）
)

// Speech 是一句對白：說話者（人物槽）、哪一塊、肖像在左、字色、內容。
type Speech struct {
	Speaker int
	Box     SpeechBox
	Left    bool
	Color   int
	Text    string
}

// Rect 是這一格的四個角（含端點）；主戰場 12×7 的版面。
func (s Speech) Rect() (x1, y1, x2, y2 int) {
	switch s.Box {
	case BoxAttacker:
		return 64, 268, 239, 363
	case BoxDefender:
		return 256, 268, 431, 363
	}
	return 448, 268, 623, 363
}

// say 印一句對白：擲字色那一擲（與 `msg` 同一擲），排進 Speeches。
// speaker 為 nil 時只擲不排（沒有人可畫）。
func (b *Battle) say(speaker *Leader, box SpeechBox, left bool, key string, a ...any) {
	c := b.msg()
	if speaker == nil {
		return
	}
	b.Speeches = append(b.Speeches, Speech{Speaker: speaker.Index, Box: box, Left: left,
		Color: c, Text: i18n.Sf(key, a...)})
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
