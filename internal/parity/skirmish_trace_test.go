//go:build oracle

package parity

import (
	"fmt"
	"testing"

	"github.com/wicanr2/dosgolem/oracle"
)

// 對戰子畫面（`0x2deb0`，`docs/re/05` §10）的逐步追蹤（Issue #30）。
//
// `SAN1_SKIRMISH=1` 時 `TestZZUnitAIDayParity` 把原版子畫面裡每一位將領
// 的每一步記下來：哪一位、站在哪、剩幾步、體能多少、走了哪條路（休息／
// 往帥隊靠／找目標／逃／往敵帥靠）、有沒有動、攻擊或單挑了誰、誰被抓
// ——連同同一段裡的每一擲，是 remake 這一層的 `L1` 對照。只掛原版
// （`AA.EXE`）的位址。
func attachSkirmishTrace(t *testing.T, o *oracle.Oracle, work func() uint16, w16 func(int) int, genBase uint32) *[]string {
	var lines []string
	inside := false
	log := func(format string, a ...any) {
		lines = append(lines, fmt.Sprintf(format, a...))
	}
	// 子畫面裡的陣列都以 (陣營×10 ＋ 將領槽) 索引：欄 `0x9a6`、列 `0x15d8`、
	// 剩餘行動力 `0x3c9c`、移動力 `0x1706`、戰力值 `0x548`、人物槽 `0x1bc8`。
	idx := func(side, slot int) int { return side*10 + slot }
	general := func(side, slot int) int { return int(int16(w16(0x1bc8 + idx(side, slot)*2))) }
	stamina := func(g int) int {
		if g < 0 || g >= 350 {
			return -1
		}
		return int(o.Byte(addr(genBase + uint32(g*30) + 8)))
	}
	soldiers := func(g int) int {
		if g < 0 || g >= 350 {
			return -1
		}
		return int(o.Word(addr(genBase + uint32(g*30) + 22)))
	}
	war := func(g int) int {
		if g < 0 || g >= 350 {
			return -1
		}
		return int(o.Byte(addr(genBase + uint32(g*30) + 10)))
	}
	pos := func(side, slot int) string {
		return fmt.Sprintf("(%d,%d)", w16(0x9a6+idx(side, slot)*2), w16(0x15d8+idx(side, slot)*2))
	}
	who := func(side, slot int) string {
		g := general(side, slot)
		return fmt.Sprintf("%d/%d=人物%d 兵%d 武%d 體%d 步%d/%d @%s", side, slot, g, soldiers(g), war(g),
			stamina(g), w16(0x3c9c+idx(side, slot)*2), w16(0x1706+idx(side, slot)*2), pos(side, slot))
	}
	local := func(o *oracle.Oracle, off int) int {
		r := o.Regs()
		return int(int16(o.Word(addr(uint32(r.SS)*16 + uint32(int(r.BP)+off)))))
	}
	o.OnCall(addr(0x2deb0), func(o *oracle.Oracle) {
		inside = true
		log("對戰 攻(%d,%d) 守(%d,%d)", int(int16(o.Arg(0))), int(int16(o.Arg(1))), int(int16(o.Arg(2))), int(int16(o.Arg(3))))
	})
	o.OnCall(addr(0x2e311), func(o *oracle.Oracle) {
		// -0x18 是版型索引：(攻方所在格的地形碼 − 2) × 2，窄圖再 +1。
		log("版型索引 %d（攻方地形碼 %d，守方地形碼 %d；戰場 (0,8) 的格 %02x）",
			local(o, -0x18), local(o, -0x6), local(o, -0x12), int(o.Byte(oracle.Addr{Seg: work(), Off: 0x1642})))
	})
	// 佈完局（0x2e493 是每一輪開頭的重畫）：第一次到這裡把雙方的初始
	// 位置、戰力值、移動力倒出來。
	dumped := false
	o.OnCall(addr(0x2e493), func(o *oracle.Oracle) {
		if dumped {
			return
		}
		dumped = true
		for side := 0; side < 2; side++ {
			for slot := 0; slot < 10; slot++ {
				if general(side, slot) < 0 || general(side, slot) >= 350 {
					continue
				}
				log("佈局 %s 戰力值 %d", who(side, slot), int(int16(w16(0x548+idx(side, slot)*2))))
			}
		}
		// 子地圖：地形碼（低四位）與佔位。
		for r := 0; r < 10; r++ {
			row := ""
			for c := 0; c < 12; c++ {
				v := w16(0x23c2 + (r*12+c)*2)
				occ := int(int16(w16(0x3b9c + (r*12+c)*2)))
				if occ >= 0 {
					row += fmt.Sprintf("%x[%d/%d] ", v&0xf, occ/10, occ%10)
				} else {
					row += fmt.Sprintf("%x ", v&0xf)
				}
			}
			log("地圖 列 %d：%s", r, row)
		}
	})
	o.OnCall(addr(0x2e493), func(o *oracle.Oracle) {
		log("── 時刻 %d", w16(0x31a6))
	})
	o.OnCall(addr(0x2e952), func(o *oracle.Oracle) {
		log("步 %s", who(int(int16(o.Arg(0))), int(int16(o.Arg(1)))))
	})
	o.OnCall(addr(0x2eaa2), func(o *oracle.Oracle) { log("  體能不足，自動休息") })
	o.OnCall(addr(0x2fb14), func(o *oracle.Oracle) { log("  玩家選單") })
	o.OnCall(addr(0x2eb8a), func(o *oracle.Oracle) { log("  電腦決策") })
	o.OnCall(addr(0x2ec01), func(o *oracle.Oracle) { log("  → 休息（RND(30) 沒過或沒事做）") })
	o.OnCall(addr(0x2ec80), func(o *oracle.Oracle) { log("  → 行動") })
	o.OnCall(addr(0x2ed2f), func(o *oracle.Oracle) { log("  帥隊旁邊沒有敵人 → 往帥隊靠") })
	o.OnCall(addr(0x2edb8), func(o *oracle.Oracle) { log("  找相鄰的敵人") })
	o.OnCall(addr(0x2ef4a), func(o *oracle.Oracle) {
		log("  兵少於最強鄰敵 %d ÷ (RND(3)+1) → 想逃（最強在方向 %d）", local(o, -0xc), local(o, -0x4))
	})
	o.OnCall(addr(0x2f067), func(o *oracle.Oracle) {
		log("  攻擊分支：目標槽 %d 敵陣營 %d 敵帥相鄰 %d 有鄰敵 %d", local(o, -0x6), local(o, -0x14), local(o, -0x2), local(o, -0x12))
	})
	o.OnCall(addr(0x2f095), func(o *oracle.Oracle) { log("  RND(16)!=0 → 往敵帥靠") })
	o.OnCall(addr(0x2f122), func(o *oracle.Oracle) { log("  交手：目標槽 %d", local(o, -0x6)) })
	o.OnCall(addr(0x2f5d8), func(o *oracle.Oracle) {
		log("  尋路 (%d,%d) → (%d,%d) 陣營 %d", int(int16(o.Arg(0))), int(int16(o.Arg(1))), int(int16(o.Arg(2))), int(int16(o.Arg(3))), int(int16(o.Arg(4))))
	})
	o.OnCall(addr(0x2f28e), func(o *oracle.Oracle) {
		log("  走一步？ %d/%d 在 (%d,%d)", int(int16(o.Arg(0))), int(int16(o.Arg(1))), int(int16(o.Arg(2))), int(int16(o.Arg(3))))
	})
	o.OnCall(addr(0x2f34e), func(o *oracle.Oracle) { log("    沒走（沒路、步數不夠或旁邊有更強的）") })
	o.OnCall(addr(0x2f4ea), func(o *oracle.Oracle) {
		log("    走到 (%d,%d) 花 %d", local(o, -0xc), local(o, -0xe), local(o, -0x6))
	})
	o.OnCall(addr(0x30364), func(o *oracle.Oracle) {
		a, b, c, d := int(int16(o.Arg(0))), int(int16(o.Arg(1))), int(int16(o.Arg(2))), int(int16(o.Arg(3)))
		log("  攻擊 %s ⇒ %s", who(a, b), who(c, d))
	})
	o.OnCall(addr(0x30716), func(o *oracle.Oracle) {
		log("    傷亡後 攻方兵 %d 守方兵 %d", local(o, -0xc), local(o, -0x8))
	})
	o.OnCall(addr(0x3071d), func(o *oracle.Oracle) { log("    攻方被抓") })
	o.OnCall(addr(0x307f7), func(o *oracle.Oracle) { log("    守方被抓") })
	o.OnCall(addr(0x30a1e), func(o *oracle.Oracle) {
		a, b, c, d := int(int16(o.Arg(0))), int(int16(o.Arg(1))), int(int16(o.Arg(2))), int(int16(o.Arg(3)))
		log("  單挑 %s ⇒ %s", who(a, b), who(c, d))
	})
	o.OnCall(addr(0x2e508), func(o *oracle.Oracle) { log("── 這一時刻結束") })
	o.OnCall(addr(0x2e51b), func(o *oracle.Oracle) {
		inside = false
		log("對戰結束：時刻 %d", w16(0x31a6))
		for side := 0; side < 2; side++ {
			for slot := 0; slot < 10; slot++ {
				if g := int(int16(w16(0x1732 + idx(side, slot)*2))); g >= 0 {
					log("  陣營 %d 抓到 人物 %d（槽 %d）", side, g, slot)
				}
			}
		}
	})
	o.OnCall(addr(0x10b0c), func(o *oracle.Oracle) {
		if n := int(int16(o.Arg(0))); n > 0 && inside {
			log("    RND(%d)@%05x", n, o.Caller().Linear())
		}
	})
	// 讀鍵的迴圈每等一輪就 `srand`（`docs/re/03` §1.45）——記成一格。
	o.OnCall(oracle.Addr{Seg: 0x5c4, Off: 0x2c9e}, func(o *oracle.Oracle) {
		if inside {
			if n := len(lines); n > 0 && len(lines[n-1]) > 9 && lines[n-1][:9] == "    srand" {
				return
			}
			log("    srand@%05x", o.Caller().Linear())
		}
	})
	return &lines
}
