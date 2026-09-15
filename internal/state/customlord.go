package state

import (
	"encoding/binary"
	"fmt"
)

// 自創君主（`docs/spec/013`）。
//
// 原版的「選角色」那一層除了在用的諸侯，還列著**新君主欄**——劇本 001
// 的十六個諸侯槽裡有兩個（槽 14、15）的君主指向填充筆（人物 346／347）、
// 一個郡都沒有，那兩個就是。手冊 p.7 寫「16（含 2 個新君主欄）」，
// 與資料完全對得上。
//
// 初始值不是猜的：`AA.EXE` 的 `0x3e44c` 有**四筆 30 byte 的人物記錄**
// 當範本（`docs/re/08` §3），姓名用 Big5 的 `A141`–`A14C` 十二個碼位
// ——那在標準 Big5 是全形標點，原版把它們挪用成造字，字模存在 `BASEPRE`。

// CustomLords 的範本（`L0`，`0x3e44c`）。四個名額只有肖像不同。
const (
	CustomLordAge       = 20
	CustomLordStamina   = 90
	CustomLordIntellect = 60
	CustomLordMight     = 80
	CustomLordCharm     = 90
)

// CustomLordPortrait 是四個名額各自的肖像編號（`L0`）。
var CustomLordPortrait = [CustomLords]int{11, 1, 15, 9}

// CustomLordPoints 是能分配的點數（手冊 p.14：「每位新加主有 100 點基數，
// 修改二、三、四、五項均需使用點數」）。
//
// ⚠ **怎麼花是 `L3`**：手冊沒寫加一點花幾點、也沒寫上限。remake 的規則
// 是「一點加一，四項各自夾在 `CustomLordStatCap`」——等 `DATA0.GRP`
// 那支 overlay 反組譯出來再校準（`docs/spec/013` §4）。
const (
	CustomLordPoints  = 100
	CustomLordStatCap = 100
)

// CustomLord 是玩家設定出來的那一位。
//
// 四項能力是**加上去的點數**不是最終值：範本已經給了底（90／60／80／90），
// 手冊說的 100 點是在那之上分配的。
type CustomLord struct {
	// Name 是三個字。**字模另外存**（`Glyphs`）——人物表裡放的是造字碼位。
	Name [CustomLordNameChars]rune
	// Stamina／Intellect／Might／Charm 是各自加了幾點。
	Stamina, Intellect, Might, Charm int
	// Prefecture 是起始領地，**必須是空白郡**（手冊 p.14）。
	Prefecture int
}

// Spent 是這一位用掉幾點。
func (c CustomLord) Spent() int {
	return c.Stamina + c.Intellect + c.Might + c.Charm
}

// Stats 是四項的最終值（範本加上分配的點數，各自夾上限）。
func (c CustomLord) Stats() (stamina, intellect, might, charm int) {
	cap := func(base, add int) int {
		v := base + add
		if v > CustomLordStatCap {
			return CustomLordStatCap
		}
		if v < 0 {
			return 0
		}
		return v
	}
	return cap(CustomLordStamina, c.Stamina), cap(CustomLordIntellect, c.Intellect),
		cap(CustomLordMight, c.Might), cap(CustomLordCharm, c.Charm)
}

// Validate 檢查這一位設定得合不合法。
func (c CustomLord) Validate(s *Scenario) error {
	if c.Stamina < 0 || c.Intellect < 0 || c.Might < 0 || c.Charm < 0 {
		return fmt.Errorf("state: 分配的點數不能是負的")
	}
	if n := c.Spent(); n > CustomLordPoints {
		return fmt.Errorf("state: 用掉 %d 點，只有 %d 點", n, CustomLordPoints)
	}
	p, err := s.Prefecture(c.Prefecture)
	if err != nil {
		return err
	}
	// **必須是空白郡**（手冊 p.14）。不擋的話新君主會直接搶走別人的郡，
	// 而畫面上只會看到那個諸侯莫名其妙少一塊地。
	if p.Owned() {
		return fmt.Errorf("state: 郡 %d（%s）已經有主，新君主只能從空白郡起家",
			c.Prefecture, p.Name)
	}
	for _, r := range c.Name {
		if r == 0 {
			return fmt.Errorf("state: 新君主的名字要三個字")
		}
	}
	return nil
}

// CustomLordSlots 回傳還空著的新君主欄（諸侯槽號），順序由小到大。
//
// 判準照原版：**君主槽指向範本**（人物 346 起，`0x123b6` 的
// `cmp es:[bx+2],0x15a`，有號比較）而且還沒被用掉（`ActiveFactions`
// 沒把它算成在用的）。劇本三到六的範本槽夾在中間（4、5、10、11…）
// 而且操縱方是 2，單看操縱方會把它們當成在用的勢力。
func (s *Scenario) CustomLordSlots() []int {
	live := map[int]bool{}
	for _, f := range s.ActiveFactions() {
		live[f] = true
	}
	var out []int
	for i := 0; i < masterCount; i++ {
		if s.templateLord(i) && !live[i] {
			out = append(out, i)
		}
	}
	return out
}

// templateLord 說這個槽的君主欄是不是指向自創君主的範本（人物 346 起）。
// 原版是有號比較：`0xFFFF`（絕嗣）算 −1，不是範本。
func (s *Scenario) templateLord(faction int) bool {
	if faction < 0 || faction >= len(s.masters) {
		return false
	}
	return int(int16(s.masters[faction].LordIndex)) >= CustomLordTemplateFrom
}

// CustomLordTemplateFrom 是四筆自創君主範本在人物表的第一筆（`docs/re/08` §6）。
const CustomLordTemplateFrom = 346

// WithCustomLord 回傳一份**加了自創君主**的劇本。原來那一份不動。
//
// faction 要是 `CustomLordSlots` 給的其中一個。寫進去的東西：
//
//	人物槽（諸侯表 offset 2 指的那一位）：姓名碼位、年齡、四項能力、
//	                                      身分 0（君主）、勢力、領地、忠誠 100
//	州郡表那一郡：所屬 ← faction、主事者 ← 那個人物槽
//
// **諸侯表不動**：君主欄本來就指著那一筆，其餘 70 個位元組還沒解
//（`docs/spec/003`），原封不動比填一個猜的值安全。
func (s *Scenario) WithCustomLord(faction int, c CustomLord) (*Scenario, error) {
	if err := c.Validate(s); err != nil {
		return nil, err
	}
	slots := s.CustomLordSlots()
	nth := -1
	for i, f := range slots {
		if f == faction {
			nth = i
			break
		}
	}
	if nth < 0 {
		return nil, fmt.Errorf("state: 諸侯槽 %d 不是空的新君主欄（空的是 %v）",
			faction, slots)
	}
	if nth >= CustomLords {
		return nil, fmt.Errorf("state: 第 %d 個新君主超過名額 %d", nth+1, CustomLords)
	}

	mas, sta, gen := s.Tables()
	who := int(binary.LittleEndian.Uint16(mas[faction*masterSize+2:]))
	if who < 0 || who >= genCount {
		return nil, fmt.Errorf("state: 諸侯槽 %d 的君主欄是 %d，不是一個人物槽",
			faction, who)
	}

	rec := gen[who*generalSize : (who+1)*generalSize]
	// 姓名：三個造字碼位，Big5 高位在前。
	for i := 0; i < CustomLordNameChars; i++ {
		code := CustomGlyphBase + nth*CustomLordNameChars + i
		rec[i*2] = byte(code >> 8)
		rec[i*2+1] = byte(code)
	}
	rec[6] = 0 // 姓名的結尾
	st, in, mi, ch := c.Stats()
	rec[7] = CustomLordAge
	rec[8] = byte(st)
	rec[9] = byte(in)
	rec[10] = byte(mi)
	rec[11] = byte(ch)
	rec[13] = byte(c.Prefecture) // 出身郡
	rec[16] = 100                // 忠誠：對自己
	rec[17] = 0                  // 身分：君主
	rec[18] = byte(faction)
	rec[19] = byte(c.Prefecture)
	rec[27] = byte(CustomLordPortrait[nth])

	pref := sta[c.Prefecture*prefSize : (c.Prefecture+1)*prefSize]
	pref[30] = byte(faction)
	binary.LittleEndian.PutUint16(pref[32:], uint16(who))

	return DecodeTables(s.Slot, mas, sta, gen)
}
