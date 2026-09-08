//go:build oracle

package parity

import (
	"fmt"
	"path/filepath"
	"strings"
	"testing"

	"github.com/wicanr2/dosgolem/oracle"

	"github.com/wicanr2/softworld_san1_remake/internal/state"
)

// 指定軍師掃的名單是誰建的、什麼順序（`docs/mechanics/70-ai` §2.7 的缺口）。
//
// 常式本身（`0xd7ae`）不建名單也不排序——碼段裡 `0xf17:0x0aae`（建名單
// ＝ `0xfc1e`）21 處、`0xf17:0x03b0`（排序 ＝ `0xf520`）5 處，一處都不落在
// `0xd7ae`–`0xd95a`。所以順序是**呼叫端上一次留下來的**，靜態追不如
// 直接跑：三支各攔一次，記時間順序，再在 `0xd7ae` 進去的那一刻把名單
// 讀出來看它到底排成什麼樣。
//
// 名單在段 `[0xa668]` 的 `0x58c` 起（word 陣列），筆數在段 `[0xa666]`
// 的 `0x0c`（`docs/re/07` §6）。
func TestChiefRosterOrder(t *testing.T) {
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
	genBase := base + uint32(state.MasterTableSize) + uint32(state.PrefectureTableSize)

	// **要讓電腦真的去拜軍師**：等級拉到 5，軍師欄清成 `0xFFFF`
	// （有軍師的話門檻是他的謀略，換人要更好，多半不會動）。
	armed := 0
	for i := 0; i < state.MasterTableSize/72; i++ {
		if o.Word(addr(base+uint32(i*72))) != 2 {
			continue
		}
		o.SetWord(addr(base+uint32(i*72+4)), 5)      // AI 等級
		o.SetWord(addr(base+uint32(i*72+6)), 0xFFFF) // 軍師 ← 沒有
		armed++
	}
	t.Logf("%d 個電腦勢力：等級 5、軍師欄清空（門檻降到 79）", armed)

	var dgroup uint16
	type ev struct {
		kind       string
		pref, mode int
	}
	var log []ev
	o.OnCall(addr(0xfc1e), func(o *oracle.Oracle) {
		if dgroup == 0 {
			dgroup = o.DSReg()
		}
		if len(log) < 400 {
			log = append(log, ev{"建名單", int(o.Arg(0)), int(o.Arg(1))})
		}
	})
	// **排序有兩支**：`0xf17:0x0000`（＝ `0xf170`，選行動者那一張用的）
	// 與 `0xf17:0x03b0`（＝ `0xf520`）。只攔一支會得到「沒排過序」的假結論。
	for _, a := range []uint32{0xf170, 0xf520} {
		at := a
		o.OnCall(addr(at), func(*oracle.Oracle) {
			if len(log) < 400 {
				log = append(log, ev{fmt.Sprintf("排序 %#x", at), -1, -1})
			}
		})
	}

	// 進 `0xd7ae` 的那一刻把名單抓下來。
	type shot struct {
		n      int
		slots  []int
		key    []int
		intel  []int
		status []int
	}
	var shots []shot
	o.OnCall(addr(0xd7ae), func(o *oracle.Oracle) {
		if dgroup == 0 || len(shots) >= 12 {
			log = append(log, ev{"指定軍師（沒抓到名單）", -1, -1})
			return
		}
		listSeg := o.Word(oracle.Addr{Seg: dgroup, Off: 0xa668})
		cntSeg := o.Word(oracle.Addr{Seg: dgroup, Off: 0xa666})
		n := int(o.Word(oracle.Addr{Seg: cntSeg, Off: 0x0c}))
		if n < 0 || n > 60 {
			log = append(log, ev{fmt.Sprintf("指定軍師（筆數 %d 不合理）", n), -1, -1})
			return
		}
		s := shot{n: n}
		for i := 0; i < n; i++ {
			v := int(o.Word(oracle.Addr{Seg: listSeg, Off: uint16(0x58c + i*2)}))
			s.slots = append(s.slots, v)
			if v < 350 {
				g := genBase + uint32(v)*30
				intel := int(o.Byte(addr(g + 9)))
				war := int(o.Byte(addr(g + 10)))
				st := int(o.Byte(addr(g + 17)))
				w := 0
				if st >= 0 && st < len(actorWeight) {
					w = actorWeight[st]
				}
				s.intel = append(s.intel, intel)
				s.status = append(s.status, st)
				s.key = append(s.key, intel+war+w)
			} else {
				s.intel = append(s.intel, -1)
				s.status = append(s.status, -1)
				s.key = append(s.key, -1)
			}
		}
		shots = append(shots, s)
		log = append(log, ev{"指定軍師", -1, -1})
	})

	// 推幾個月讓電腦動。
	for i := 1; i <= 3; i++ {
		for _, keys := range strings.Split(envOr("SAN1_TURNKEY", "4\r|4\r|Y"), "|") {
			o.Drain()
			o.PressScan(keys)
			if err := o.Run(200_000_000); err != nil {
				t.Fatalf("第 %d 個月送 %q 停止：%v", i, keys, err)
			}
		}
	}

	if len(shots) == 0 {
		var tail []string
		for _, e := range log[max(0, len(log)-12):] {
			tail = append(tail, fmt.Sprintf("%s(%d,%d)", e.kind, e.pref, e.mode))
		}
		t.Fatalf("三個月裡電腦一次都沒去拜軍師（事件尾巴：%s）",
			strings.Join(tail, " → "))
	}

	// 每一次指定軍師之前，最近的建名單與排序各是什麼。
	for i, e := range log {
		if !strings.HasPrefix(e.kind, "指定軍師") {
			continue
		}
		var lastBuild, lastSort = -1, -1
		for j := i - 1; j >= 0; j-- {
			if lastBuild < 0 && log[j].kind == "建名單" {
				lastBuild = j
			}
			if lastSort < 0 && strings.HasPrefix(log[j].kind, "排序") {
				lastSort = j
			}
			if lastBuild >= 0 && lastSort >= 0 {
				break
			}
		}
		desc := "沒有任何建名單在它之前"
		if lastBuild >= 0 {
			desc = fmt.Sprintf("最近的建名單是 (郡 %d, 模式 %d)，中間隔 %d 個事件",
				log[lastBuild].pref, log[lastBuild].mode, i-lastBuild-1)
			if lastSort > lastBuild {
				var between []string
				for j := lastBuild + 1; j < i; j++ {
					between = append(between, log[j].kind)
				}
				desc += "；建完之後：" + strings.Join(between, " → ")
			} else {
				desc += "；建完之後沒有再排過序"
			}
		}
		t.Logf("%s：%s", e.kind, desc)
	}

	ascRuns, descKey := 0, 0
	for i, s := range shots {
		asc := true
		for j := 1; j < len(s.slots); j++ {
			if s.slots[j] < s.slots[j-1] {
				asc = false
			}
		}
		// 期望：**鍵遞減**（＝ 選行動者取名單[0] 的那個排法）。
		down := true
		for j := 1; j < len(s.key); j++ {
			if s.key[j] > s.key[j-1] {
				down = false
			}
		}
		if asc {
			ascRuns++
		}
		if down {
			descKey++
		}
		t.Logf("第 %d 次（%d 筆）：槽 %v｜身分 %v｜謀略 %v｜"+
			"鍵(謀略+戰力+加權) %v｜槽號遞增 %v｜鍵遞減 %v",
			i+1, s.n, s.slots, s.status, s.intel, s.key, asc, down)
	}
	t.Logf("共 %d 次：槽號遞增 %d 次、鍵遞減 %d 次", len(shots), ascRuns, descKey)
	if descKey != len(shots) {
		t.Errorf("有 %d 次的名單不是按「謀略+戰力+加權[身分]」遞減排的",
			len(shots)-descKey)
	}
}
