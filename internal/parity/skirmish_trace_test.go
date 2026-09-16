//go:build oracle

package parity

import (
	"fmt"
	"testing"

	"github.com/wicanr2/dosgolem/oracle"
)

// 對戰子畫面（`0x2deb0`，`docs/re/05` §10）的逐步追蹤（Issue #30／#37）。
//
// `SAN1_SKIRMISH=1` 時 `TestZZUnitAIDayParity`（與加強版那支）把原版子畫面
// 裡每一位將領的每一步記下來：哪一位、站在哪、剩幾步、體能多少、走了哪
// 條路（休息／往帥隊靠／找目標／逃／往敵帥靠）、有沒有動、攻擊或單挑了
// 誰、誰被抓——連同同一段裡的每一擲，是 remake 這一層的 `L1` 對照。
// 兩版的路標各一份（`skirmishSites`）。

// skirmishSites 是子畫面裡要攔的位址（線性）與工作區的位移。加強版的
// 位址是把兩版 `0x2deb0`–`0x33000`／`0x2ae66`–`0x2fe00` 的指令流正規化後
// 對齊出來的（`docs/re/05` §10.9），純量的位移照 `docs/re/05` §8.2。
type skirmishSites struct {
	enter, layout, redraw, step, autoRest, playerMenu, computer, rest, act,
	toLeader, findAdj, flee, attackBranch, routeLeader, engage, route,
	stepOne, noStep, stepped, attack, casualties, attCaught, defCaught,
	duel, hourEnd, finish, rnd uint32
	srand oracle.Addr
	// 工作區的位移：時刻、子地圖、佔位、剩餘行動力。
	hour, mapOff, occOff, leftOff int
}

var (
	baseSkirmishSites = skirmishSites{
		enter: 0x2deb0, layout: 0x2e311, redraw: 0x2e493, step: 0x2e952, autoRest: 0x2eaa2,
		playerMenu: 0x2fb14, computer: 0x2eb8a, rest: 0x2ec01, act: 0x2ec80, toLeader: 0x2ed2f,
		findAdj: 0x2edb8, flee: 0x2ef4a, attackBranch: 0x2f067, routeLeader: 0x2f095, engage: 0x2f122,
		route: 0x2f5d8, stepOne: 0x2f28e, noStep: 0x2f34e, stepped: 0x2f4ea, attack: 0x30364,
		casualties: 0x30716, attCaught: 0x3071d, defCaught: 0x307f7, duel: 0x30a1e,
		hourEnd: 0x2e508, finish: 0x2e51b, rnd: 0x10b0c,
		srand: oracle.Addr{Seg: 0x5c4, Off: 0x2c9e},
		hour:  0x31a6, mapOff: 0x23c2, occOff: 0x3b9c, leftOff: 0x3c9c,
	}
	plusSkirmishSites = skirmishSites{
		enter: 0x2ae66, layout: 0x2b295, redraw: 0x2b40b, step: 0x2b888, autoRest: 0x2b9b4,
		playerMenu: 0x2c9c0, computer: 0x2ba8c, rest: 0x2bb02, act: 0x2bb78, toLeader: 0x2bc22,
		findAdj: 0x2bca5, flee: 0x2be1a, attackBranch: 0x2bf30, routeLeader: 0x2bf5d, engage: 0x2bfe4,
		route: 0x2c4b0, stepOne: 0x2c18a, noStep: 0x2c246, stepped: 0x2c3cb, attack: 0x2d1bc,
		casualties: 0x2d513, attCaught: 0x2d51a, defCaught: 0x2d5e7, duel: 0x2d7ec,
		hourEnd: 0x2b47c, finish: 0x2b48f, rnd: plusRndFn,
		srand: oracle.Addr{Seg: 0x5b9, Off: 0x2ca0},
		hour:  0x31b2, mapOff: 0x23c4, occOff: 0x3ba8, leftOff: 0x3ca8,
	}
)

func attachSkirmishTrace(t *testing.T, o *oracle.Oracle, work func() uint16, w16 func(int) int, genBase uint32, sk skirmishSites) *[]string {
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
			stamina(g), w16(sk.leftOff+idx(side, slot)*2), w16(0x1706+idx(side, slot)*2), pos(side, slot))
	}
	local := func(o *oracle.Oracle, off int) int {
		r := o.Regs()
		return int(int16(o.Word(addr(uint32(r.SS)*16 + uint32(int(r.BP)+off)))))
	}
	o.OnCall(addr(sk.enter), func(o *oracle.Oracle) {
		inside = true
		log("對戰 攻(%d,%d) 守(%d,%d)", int(int16(o.Arg(0))), int(int16(o.Arg(1))), int(int16(o.Arg(2))), int(int16(o.Arg(3))))
	})
	o.OnCall(addr(sk.layout), func(o *oracle.Oracle) {
		// -0x18 是版型索引：(攻方所在格的地形碼 − 2) × 2，窄圖再 +1。
		log("版型索引 %d（攻方地形碼 %d，守方地形碼 %d；戰場 (0,8) 的格 %02x）",
			local(o, -0x18), local(o, -0x6), local(o, -0x12), int(o.Byte(oracle.Addr{Seg: work(), Off: 0x1642})))
	})
	// 佈完局（0x2e493 是每一輪開頭的重畫）：第一次到這裡把雙方的初始
	// 位置、戰力值、移動力倒出來。
	dumped := false
	o.OnCall(addr(sk.redraw), func(o *oracle.Oracle) {
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
				v := w16(sk.mapOff + (r*12+c)*2)
				occ := int(int16(w16(sk.occOff + (r*12+c)*2)))
				if occ >= 0 {
					row += fmt.Sprintf("%x[%d/%d] ", v&0xf, occ/10, occ%10)
				} else {
					row += fmt.Sprintf("%x ", v&0xf)
				}
			}
			log("地圖 列 %d：%s", r, row)
		}
	})
	o.OnCall(addr(sk.redraw), func(o *oracle.Oracle) {
		log("── 時刻 %d", w16(sk.hour))
	})
	o.OnCall(addr(sk.step), func(o *oracle.Oracle) {
		log("步 %s", who(int(int16(o.Arg(0))), int(int16(o.Arg(1)))))
	})
	o.OnCall(addr(sk.autoRest), func(o *oracle.Oracle) { log("  體能不足，自動休息") })
	o.OnCall(addr(sk.playerMenu), func(o *oracle.Oracle) { log("  玩家選單") })
	o.OnCall(addr(sk.computer), func(o *oracle.Oracle) { log("  電腦決策") })
	o.OnCall(addr(sk.rest), func(o *oracle.Oracle) { log("  → 休息（行動門沒過或沒事做）") })
	o.OnCall(addr(sk.act), func(o *oracle.Oracle) { log("  → 行動") })
	o.OnCall(addr(sk.toLeader), func(o *oracle.Oracle) { log("  帥隊旁邊沒有敵人 → 往帥隊靠") })
	o.OnCall(addr(sk.findAdj), func(o *oracle.Oracle) { log("  找相鄰的敵人") })
	o.OnCall(addr(sk.flee), func(o *oracle.Oracle) {
		log("  兵少於最強鄰敵 %d ÷ (RND(3)+1) → 想逃（最強在方向 %d）", local(o, -0xc), local(o, -0x4))
	})
	o.OnCall(addr(sk.attackBranch), func(o *oracle.Oracle) {
		log("  攻擊分支：目標槽 %d 敵陣營 %d 敵帥相鄰 %d 有鄰敵 %d", local(o, -0x6), local(o, -0x14), local(o, -0x2), local(o, -0x12))
	})
	o.OnCall(addr(sk.routeLeader), func(o *oracle.Oracle) { log("  RND(16) 沒中 → 往敵帥靠") })
	o.OnCall(addr(sk.engage), func(o *oracle.Oracle) { log("  交手：目標槽 %d", local(o, -0x6)) })
	o.OnCall(addr(sk.route), func(o *oracle.Oracle) {
		log("  尋路 (%d,%d) → (%d,%d) 陣營 %d", int(int16(o.Arg(0))), int(int16(o.Arg(1))), int(int16(o.Arg(2))), int(int16(o.Arg(3))), int(int16(o.Arg(4))))
	})
	o.OnCall(addr(sk.stepOne), func(o *oracle.Oracle) {
		log("  走一步？ %d/%d 在 (%d,%d)", int(int16(o.Arg(0))), int(int16(o.Arg(1))), int(int16(o.Arg(2))), int(int16(o.Arg(3))))
	})
	o.OnCall(addr(sk.noStep), func(o *oracle.Oracle) { log("    沒走（沒路、步數不夠或旁邊有更強的）") })
	o.OnCall(addr(sk.stepped), func(o *oracle.Oracle) {
		log("    走到 (%d,%d) 花 %d", local(o, -0xc), local(o, -0xe), local(o, -0x6))
	})
	o.OnCall(addr(sk.attack), func(o *oracle.Oracle) {
		a, b, c, d := int(int16(o.Arg(0))), int(int16(o.Arg(1))), int(int16(o.Arg(2))), int(int16(o.Arg(3)))
		log("  攻擊 %s ⇒ %s", who(a, b), who(c, d))
	})
	o.OnCall(addr(sk.casualties), func(o *oracle.Oracle) {
		log("    傷亡後 攻方兵 %d 守方兵 %d", local(o, -0xc), local(o, -0x8))
	})
	o.OnCall(addr(sk.attCaught), func(o *oracle.Oracle) { log("    攻方被抓") })
	o.OnCall(addr(sk.defCaught), func(o *oracle.Oracle) { log("    守方被抓") })
	o.OnCall(addr(sk.duel), func(o *oracle.Oracle) {
		a, b, c, d := int(int16(o.Arg(0))), int(int16(o.Arg(1))), int(int16(o.Arg(2))), int(int16(o.Arg(3)))
		log("  單挑 %s ⇒ %s", who(a, b), who(c, d))
	})
	o.OnCall(addr(sk.hourEnd), func(o *oracle.Oracle) { log("── 這一時刻結束") })
	o.OnCall(addr(sk.finish), func(o *oracle.Oracle) {
		inside = false
		log("對戰結束：時刻 %d", w16(sk.hour))
		for side := 0; side < 2; side++ {
			for slot := 0; slot < 10; slot++ {
				if g := int(int16(w16(0x1732 + idx(side, slot)*2))); g >= 0 {
					log("  陣營 %d 抓到 人物 %d（槽 %d）", side, g, slot)
				}
			}
		}
	})
	o.OnCall(addr(sk.rnd), func(o *oracle.Oracle) {
		if n := int(int16(o.Arg(0))); n > 0 && inside {
			log("    RND(%d)@%05x", n, o.Caller().Linear())
		}
	})
	// 讀鍵的迴圈每等一輪就 `srand`（`docs/re/03` §1.45）——記成一格。
	o.OnCall(sk.srand, func(o *oracle.Oracle) {
		if inside {
			if n := len(lines); n > 0 && len(lines[n-1]) > 9 && lines[n-1][:9] == "    srand" {
				return
			}
			log("    srand@%05x", o.Caller().Linear())
		}
	})
	return &lines
}
