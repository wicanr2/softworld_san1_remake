package ui

import (
	"github.com/wicanr2/softworld_san1_remake/internal/battle"
	"github.com/wicanr2/softworld_san1_remake/internal/cells"
)

// 主戰場第三塊面板的文字視窗（`docs/spec/014` §7，`L0`＋`L1`、`[base]`、Issue #75）。
// 原版每一個問玩家的地方都是「`0x33d8:0x1636` 清視窗 → `0x33d8:0xcc0` 寫字 →
// `0x1058:0xe24` 讀一個鍵」，字串照抄在 `bat.win.*`。

const (
	// BattleWindowCols／Rows 是文字視窗的格數（176×96 像素）。
	BattleWindowCols = 22
	BattleWindowRows = 6
	// battleWindowInk／Paper 是視窗裡字與底的顏色（黃 14 在青 3 上，量自原版
	// 三張基準畫面）。
	battleWindowInk   = 14
	battleWindowPaper = 3
)

// BattleWindowLines 照訊息常式排字：`\n` 換行、一行滿 22 格就折到下一行
// （全形字不拆），超過 6 列只留最後 6 列（往上捲）。
func BattleWindowLines(text string, cols, rows int) []string {
	lines := []string{""}
	for _, r := range text {
		cur := len(lines) - 1
		if r == '\n' {
			lines = append(lines, "")
			continue
		}
		if cells.Width(lines[cur])+cells.RuneWidth(r) > cols {
			lines = append(lines, "")
			cur++
		}
		lines[cur] += string(r)
	}
	if len(lines) > rows {
		lines = lines[len(lines)-rows:]
	}
	return lines
}

// NameField 照原版人物表 6 byte 的姓名欄排名字：兩字名前後各補一個空白。
func NameField(name string) string { return battlePaddedName(name) }

// BattleCommandWindow 是每天的命令提示（`0x27a40`）：三行選單（`DS:0x7f22`）
// 接「%s主公請下%s(%2d)%s的命令(0-8):」（`DS:0x7f02`）——君主姓名欄、隊伍名
// （`DS:0x7860`）、剩下的移動力、帶隊那一位的姓名欄。
func BattleCommandWindow(lord string, f battle.Formation, move int, leader string) string {
	return t("bat.win.menu") + tf("bat.win.order", NameField(PersonName(lord)), f.Label(), move, NameField(PersonName(leader)))
}

// BattleDirWindow 是選了要方向的指令之後那一格：移動（`DS:0x7f95`，帶餘步）、
// 對戰（`0x8144`）、快戰（`0x7fbb`）、死戰（`0x8091`）、弓箭（`0x80ae`，帶次數）。
func BattleDirWindow(cmd battle.Command, u *battle.Unit) string {
	switch cmd {
	case battle.CmdMove:
		return tf("bat.win.move", u.Move)
	case battle.CmdEngage:
		return t("bat.win.engage")
	case battle.CmdQuick:
		return t("bat.win.quick")
	case battle.CmdDeath:
		return t("bat.win.death")
	case battle.CmdArchery:
		return tf("bat.win.archery", u.Arrows)
	}
	return ""
}

// BattleCampWindow 是紮寨那一格（`0x22ae0`＋`0x21d2b`–`0x21d45`）：
// 「(%2d%s)%s之%s請%s將軍紮寨」——那支軍力的來源郡號與郡名、軍力名
// （`DS:0x7844`）、隊伍名、帶隊那一位的姓名欄——接方向鍵的提示。
func BattleCampWindow(prefID int, prefName string, s battle.Side, f battle.Formation, leader string) string {
	return tf("bat.win.camp", prefID, PlaceName(prefName), SideName(s), f.Label(), NameField(PersonName(leader)))
}
