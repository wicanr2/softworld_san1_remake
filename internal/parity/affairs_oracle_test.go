//go:build oracle

package parity

import (
	"path/filepath"
	"sort"
	"testing"

	"github.com/wicanr2/dosgolem/oracle"

	"github.com/wicanr2/softworld_san1_remake/internal/game"
	"github.com/wicanr2/softworld_san1_remake/internal/state"
)

// 內政兩項的對拍（`docs/mechanics/10` §3、`docs/mechanics/70-ai` §2.5）。
//
// **不走玩家選單**：玩家那條要一路按進子選單再選將領，按鍵序列每試一次
// 就是一分鐘。電腦諸侯每個月都在做同樣兩件事，而且**與玩家共用寫回的
// 常式**（`0xba02` 土地開墾、`0xba4c` 洪水防治），所以掛在那兩支上
// 收得到真正的參數。
//
// 判準是 remake 的公式**算得出原版當場給的那個數**：
//
//	土地開墾 add ＝ max((謀略 − 底) ÷ 12, RND(2))    底隨 AI 等級
//	洪水防治 drop ＝ 謀略 ÷ 除數                      除數隨 AI 等級
//
// 等級由**呼叫點**分辨：六份常式各自呼叫，位址差 0x76。

// affairsCallers 是六個等級各自的兩個呼叫點（`docs/mechanics/70-ai` §2.5），
// 鍵是**返回位址**。
//
// ⚠ `Caller()` 給的是「呼叫端的下一道指令」不是呼叫指令本身。原版這裡是
// `push cs`（1 byte）＋ `call rel16`（3 bytes），所以要加 4——拿呼叫點
// 本身當鍵一次都查不到，而那看起來與「常式沒被呼叫」一模一樣。
func affairsCallers() (land, flood map[uint32]int) {
	const retOffset = 4
	land, flood = map[uint32]int{}, map[uint32]int{}
	for k := 0; k < 6; k++ {
		land[uint32(0x0bad5+k*0x76+retOffset)] = k
		flood[uint32(0x0bb09+k*0x76+retOffset)] = k
	}
	return
}

// TestAffairsMatchTheOriginal 讓原版自己跑三個月的內政，逐次核對公式。
func TestAffairsMatchTheOriginal(t *testing.T) {
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

	// **盤面自己擺**：開局的電腦諸侯 AI 等級只有 4 與 5，照劇本跑只驗得到
	// 六份常式裡的兩份。把等級（`BASEMAS` offset 4）輪流設成 0–5。
	//
	// ⚠ **只數活著的槽**：十六個槽有兩個沒在用（offset 0 ＝ 0xFFFF），
	// 照槽號取模會把兩個等級指給不存在的勢力——上一輪等級 4 就這樣
	// 一次都沒走到，而輸出看起來只是「這三個月剛好沒輪到」。
	alive := 0
	for i := 0; i < state.MasterTableSize/72; i++ {
		if o.Word(addr(base+uint32(i*72))) == 0xFFFF {
			continue
		}
		o.SetWord(addr(base+uint32(i*72+4)), uint16(alive%6))
		alive++
	}
	t.Logf("活著的勢力 %d 個，AI 等級輪流設成 0–5", alive)

	type shot struct {
		level, arg, intel int
		flood             bool
	}
	var shots []shot
	landCall, floodCall := affairsCallers()

	// 執行者的謀略：**從 BX 讀，不要猜段變數**。
	//
	// 呼叫端算完 `30 × 槽號` 就把它留在 BX，一路到 `push cs; call` 都沒
	// 動過（`0xbabf` 的 `mov bx,ax`）。段變數那條路我猜過一次，
	// 讀回來每次都是同一個數——**一個不會變的「觀測值」是讀錯位址的
	// 徵兆**，因為八十次內政不可能都由同一個人執行。
	actorIntel := func(o *oracle.Oracle) int {
		slot := int(o.BX()) / 30
		if slot < 0 || slot >= 350 {
			return -1
		}
		lin := base + uint32(state.MasterTableSize) + uint32(state.PrefectureTableSize) +
			uint32(slot*30+9)
		return int(o.Byte(addr(lin)))
	}

	record := func(isFlood bool, table map[uint32]int) func(*oracle.Oracle) {
		return func(o *oracle.Oracle) {
			lvl, ok := table[o.Caller().Linear()]
			if !ok {
				return // 別的呼叫端（玩家那一條）
			}
			shots = append(shots, shot{
				level: lvl, arg: int(int16(o.Arg(0))),
				intel: actorIntel(o), flood: isFlood,
			})
		}
	}
	// ⚠ **位址要用 `addr()` 不是 `IDA()`**：這裡的數字是執行期的線性
	// 位址（碼段 dump 從 0x0b000 起），`IDA()` 會再減一次偏移，
	// 掛上去的是別的地方——而 hook 沒被觸發看起來與「電腦沒做內政」
	// 一模一樣。
	o.OnCall(addr(0x0ba02), record(false, landCall))
	o.OnCall(addr(0x0ba4c), record(true, floodCall))

	// **正對照**：亂數一定會被呼叫。它一次都沒中就表示 hook 或月份
	// 推進壞了，而不是電腦這幾個月剛好沒做內政。
	rnd := 0
	o.OnCall(addr(0x1058*16+0x058c), func(*oracle.Oracle) { rnd++ })



	const settle = 40_000_000
	for m := 0; m < 4; m++ {
		for _, k := range []string{"4\r", "4\r", "Y"} {
			o.Drain()
			o.PressScan(k)
			if err := o.Run(settle * 3); err != nil {
				t.Fatalf("第 %d 個月送 %q 時停止：%v", m+1, k, err)
			}
		}
	}

	t.Logf("四個月裡亂數被呼叫 %d 次（正對照）", rnd)
	if rnd == 0 {
		t.Fatal("連亂數都沒被呼叫：hook 沒掛上，或者按鍵序列沒推動月份")
	}
	if len(shots) == 0 {
		t.Fatal("亂數有動、常式也跑了，但套用常式一次都沒收到——" +
			"呼叫點的返回位址對不上")
	}

	// 逐次核對。
	seen := map[int]int{}
	bad := 0
	for _, s := range shots {
		seen[s.level]++
		if s.intel < 0 {
			t.Errorf("等級 %d：執行者槽號讀不出來", s.level)
			bad++
			continue
		}
		tier := game.AffairsTierFor(s.level)
		if s.flood {
			if want := game.AIFloodDrop(s.intel, tier.FloodDiv); s.arg != want {
				t.Errorf("等級 %d 防洪：謀略 %d 原版給 %d，remake 算 %d",
					s.level, s.intel, s.arg, want)
				bad++
			}
			continue
		}
		// 開墾：非正的量原版會改擲 RND(2)，那一段在常式裡而不是呼叫端，
		// 所以這裡比的是**呼叫端算出來的原始值**。
		if want := (s.intel - tier.LandFloor) / game.ReclaimIntelDiv; s.arg != want {
			t.Errorf("等級 %d 開墾：謀略 %d 原版給 %d，remake 算 %d",
				s.level, s.intel, s.arg, want)
			bad++
		}
	}
	levels := make([]int, 0, len(seen))
	for k := range seen {
		levels = append(levels, k)
	}
	sort.Ints(levels)
	for _, k := range levels {
		t.Logf("等級 %d：%d 次", k, seen[k])
	}
	t.Logf("四個月共 %d 次內政，%d 次對不上", len(shots), bad)
	if len(seen) < 6 {
		t.Errorf("只走到 %d 個等級的常式，六個都要驗到才算數", len(seen))
	}
}
