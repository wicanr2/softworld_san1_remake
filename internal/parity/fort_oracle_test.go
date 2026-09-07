//go:build oracle

package parity

import (
	"path/filepath"
	"testing"

	"golang.org/x/text/encoding/traditionalchinese"

	"github.com/wicanr2/dosgolem/oracle"

	"github.com/wicanr2/softworld_san1_remake/internal/game"
	"github.com/wicanr2/softworld_san1_remake/internal/state"
)

// 建築關寨的對拍（`docs/mechanics/10` §3、`docs/spec/003` §3.5）。
//
// 這一條**沒有電腦諸侯的分派表**，只能走玩家選單。代價是按鍵序列要試，
// 好處是 `o.Save()`／`o.Restore()` 一次快照約一毫秒——開機只付一次，
// 之後每試一組序列就還原一次。
//
// 掛四個點：
//
//	0x1aae9  mov [bp-2],ax     ; ax ＝ 100 × 物價，也就是花費
//	0x1ab9e  incb es:0x499(bx) ; 關寨數 +1（指令執行前，記憶體還是舊值）
//	0x1aba6  sub ax,es:0x492(bx) ; 金 −= 花費
//	0x1afb2  mov es:0x4b7(bx),al ; 地圖那一格，al ＝ 新值
//
// fortProbe 是「這一次真的走進建築關寨」的正對照：
// `0x1aae2` 是算花費那一段（`mov al,100; imul 物價`），只有進到指令
// 本體才會經過。
//
// ⚠ 別拿 `0x1aa60` 當入口——那是**迴圈頂端**的「主公是否繼續呢(Y/N)」，
// 蓋完第一座才會走到。掛在那裡會得到 0 次，看起來像指令沒被執行。
const fortProbe = 0x1aae2

// TestFortMatchesTheOriginal 讓原版自己蓋一座關寨，核對花費、上限與地圖。
func TestFortMatchesTheOriginal(t *testing.T) {
	root := origRoot(t)
	c := openContainer(t, filepath.Join(root, "DATA2"))
	sc0, err := state.LoadScenario(c, state.Slot("001"))
	if err != nil {
		t.Fatal(err)
	}
	seedMas, _, _ := sc0.Tables()

	o, err := oracle.Load(filepath.Join(root, "AA.EXE"), root)
	if err != nil {
		t.Fatal(err)
	}
	defer o.Close()

	base := bootToGame(t, o, seedMas)
	staBase := base + uint32(state.MasterTableSize)
	genBase := staBase + uint32(state.PrefectureTableSize)

	// 目前輪到的郡：原版把它放在 es:0x30fc（`docs/re/06` §9.5 的月內迴圈）。
	pref := int(o.Word(addr(0x03e640 + 0x30fc)))
	if pref < 1 || pref > state.PrefectureCount {
		t.Skipf("目前的郡讀出來是 %d，開機序列停的位置與預期不同", pref)
	}
	rec := func(off uint32) uint32 { return staBase + uint32(pref*176) + off }
	t.Logf("目前輪到郡 %d，關寨 %d、金 %d、物價 %d",
		pref, o.Byte(addr(rec(25))), o.Word(addr(rec(18))), o.Byte(addr(rec(29))))

	// **盤面自己擺**：金拉滿、關寨歸零、物價固定，再確保郡裡有一位謀略 90 的人。
	o.SetWord(addr(rec(18)), 30000)
	o.SetByte(addr(rec(25)), 0)
	o.SetByte(addr(rec(29)), 40)
	// **地圖也自己擺**：整張填成「沒有標記的平原」（0xF7）。
	//
	// 游標畫面的按鍵是 ASCII 的 '0'–'6' 與 ESC（`0x1ae29` 之後的分派），
	// 而 **'0' 落在不能蓋的格子上時原版什麼都不做**——直接跳回輸入迴圈，
	// 連訊息都不印（`0x1aecb`／`0x1aed1` 的 `je 0x1af1a`）。
	// 所以「按了沒反應」與「按鍵不對」在畫面上一模一樣；
	// 把整張圖擺成合法的，就把這個變數消掉。
	for i := 0; i < 120; i++ {
		o.SetByte(addr(rec(55)+uint32(i)), 0xF7)
	}

	wise := -1
	for i := 0; i < 350; i++ {
		g := genBase + uint32(i*30)
		if int(o.Byte(addr(g+19))) != pref { // 所在郡
			continue
		}
		if o.Byte(addr(g+17)) == 0xFF { // 身分：在野的不算
			continue
		}
		o.SetByte(addr(g+9), 90) // 謀略
		wise = i
		break
	}
	if wise < 0 {
		t.Skipf("郡 %d 沒有現役武將", pref)
	}
	t.Logf("把人物槽 %d 的謀略設成 90", wise)

	type shot struct{ cost, price, oldForts, oldGold, newGold, cell, newCell int }
	var got shot
	hits, picked, cursor, gates := 0, -2, 0, 0
	o.OnCall(addr(0x1ab30), func(o *oracle.Oracle) { picked = int(int16(o.AX())) })
	o.OnCall(addr(0x1acba), func(*oracle.Oracle) { cursor++ })
	o.OnCall(addr(0x1aeba), func(*oracle.Oracle) { gates++ })
	o.OnCall(addr(fortProbe), func(*oracle.Oracle) { hits++ })
	// 物價要**在算花費的當下**讀。指令跑完月份就換了，物價每個月重抽
	// （`docs/mechanics/60-economy` §1.2）——事後再讀會拿到下個月的值，
	// 而那看起來只是「公式差了一點」。
	o.OnCall(addr(0x1aae9), func(o *oracle.Oracle) {
		got.cost = int(int16(o.AX()))
		got.price = int(o.Byte(addr(rec(29))))
	})
	o.OnCall(addr(0x1ab9e), func(o *oracle.Oracle) {
		got.oldForts = int(o.Byte(addr(rec(25))))
		got.oldGold = int(o.Word(addr(rec(18))))
	})
	// ⚠ **BX 裡沒有那 55**：位移 `0x4b7`（＝ 0x480 + 55）是指令自己帶的，
	// BX 只有 `176 × 郡 + 12 × 列 + 欄`。多減一次 55 會讓索引變負，
	// 而那看起來只是「hook 沒被觸發」。
	o.OnCall(addr(0x1afb2), func(o *oracle.Oracle) {
		i := int(o.BX()) - pref*176
		if i >= 0 && i < 120 {
			got.cell = int(o.Byte(addr(rec(55) + uint32(i))))
			got.newCell = int(o.AX() & 0xFF)
		}
	})

	// **提示字串的追蹤器**：原版每次要玩家看什麼都走同一支
	// （`0x33d8:0x0cc0`，遠呼叫，第一個參數是字串的 far pointer）。
	// 掛在它的入口就拿得到「畫面現在在問什麼」——比看畫素差或存圖
	// 都直接，而且**看得懂卡在哪一個提示**。
	var prompts []string
	o.OnCall(addr(0x33d8*16+0x0cc0), func(o *oracle.Oracle) {
		off, seg := uint32(o.Arg(0)), uint32(o.Arg(1))
		b := o.Bytes(addr(seg*16+off), 48)
		n := 0
		for n < len(b) && b[n] != 0 {
			n++
		}
		prompts = append(prompts, decodeBig5Loose(b[:n]))
	})

	const settle = 40_000_000
	snap := o.Save()
	step := func(keys string) {
		o.Drain()
		if keys != "" {
			o.PressScan(keys)
		}
		if err := o.Run(settle * 2); err != nil {
			t.Fatalf("送 %q 時停止：%v", keys, err)
		}
	}
	// 提示追蹤器問出來的序列：`4` 內政 → `3` 建築關寨 → 直接就是
	// 「那一位去(1-1)」，**中間沒有 Y/N**。挑完人進地圖游標畫面
	// （`DS:0x70cc`「數字鍵選方向」），`0` 是建關。
	seq := []string{"4\r", "3\r", "1\r", "0", "Y", "\r", "N"}
	for i, k := range seq {
		prompts = nil
		step(k)
		t.Logf("第 %d 步送 %-4q：常式 %d 次、選將回 %d、游標 %d 次、"+
			"格子判定 %d 次、關寨 %d、金 %d、提示 %q",
			i+1, k, hits, picked, cursor, gates,
			o.Byte(addr(rec(25))), o.Word(addr(rec(18))), prompts)
	}

	if hits == 0 {
		o.Restore(snap)
		t.Fatal("建築關寨的常式一次都沒進去——按鍵序列不對")
	}
	if got.oldForts == 0 && got.cost == 0 {
		t.Fatal("常式進去了，但沒走到扣錢那一步")
	}

	// 判準一：花費 ＝ 當月物價 × 100。
	if want := game.FortCost(uint8(got.price)); got.cost != want {
		t.Errorf("物價 %d：原版收 %d 金，remake 的公式給 %d",
			got.price, got.cost, want)
	}
	// 判準二：真的扣了那麼多金、關寨真的加一。
	if spent := got.oldGold - int(o.Word(addr(rec(18)))); spent != got.cost {
		t.Errorf("金少了 %d，花費是 %d", spent, got.cost)
	}
	if now := int(o.Byte(addr(rec(25)))); now != got.oldForts+1 {
		t.Errorf("關寨 %d → %d", got.oldForts, now)
	}
	// 判準三：地圖那一格低四位變 6、高四位不動。
	if got.newCell != got.cell&0xF6|6 {
		t.Errorf("地圖那一格 0x%02X → 0x%02X，remake 的寫法給 0x%02X",
			got.cell, got.newCell, got.cell&0xF6|6)
	}
	if !game.CanBuildFortOn(byte(got.cell)) {
		t.Errorf("原版蓋在 0x%02X，但 remake 的 CanBuildFortOn 說不行", got.cell)
	}
	t.Logf("原版：花 %d 金（物價 %d）、關寨 %d → %d、地圖 0x%02X → 0x%02X",
		got.cost, got.price, got.oldForts, o.Byte(addr(rec(25))),
		got.cell, got.newCell)
	o.Restore(snap)
}

// decodeBig5Loose 把提示字串解成看得懂的樣子；解不開就照原樣印。
func decodeBig5Loose(b []byte) string {
	out, err := traditionalchinese.Big5.NewDecoder().Bytes(b)
	if err != nil {
		return string(b)
	}
	return string(out)
}
