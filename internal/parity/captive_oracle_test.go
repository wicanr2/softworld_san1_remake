//go:build oracle

package parity

import (
	"encoding/binary"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/wicanr2/dosgolem/oracle"
	"github.com/wicanr2/softworld_san1_remake/internal/battle"
	"github.com/wicanr2/softworld_san1_remake/internal/game"
	"github.com/wicanr2/softworld_san1_remake/internal/state"
)

// 玩家捕獲時的處置（`docs/spec/018`，Issue #35）：進主戰場之後直接呼叫
// `0x259fe(捕獲方軍力, 人物)`，把讀鍵常式換成指定的鍵序、把等鍵與延遲
// 換成立即返回，比兩邊的 `RND(n)` 序列與寫回人物表、州郡表的欄位。
var (
	captiveFn     = oracle.Addr{Seg: 0x2020, Off: 0x57fe} // ＝ 0x259fe
	captiveKeyFn  = oracle.Addr{Seg: 0x1058, Off: 0xe24}  // 讀一個鍵
	captiveWaitFn = oracle.Addr{Seg: 0x1058, Off: 0xe80}  // 延遲或等鍵
	captiveSayFn  = oracle.Addr{Seg: 0x1058, Off: 0x41aa} // 對白之後的停頓
	captiveCampFn = oracle.Addr{Seg: 0x2020, Off: 0x1a7c} // 中途紮寨的游標挑格
	captiveRandFn = oracle.Addr{Seg: 0x5c4, Off: 0x2cb0}  // MSC rand()
)

func TestZZPlayerCaptiveMatchesTheOriginal(t *testing.T) {
	root := origRoot(t)
	c2 := openContainer(t, filepath.Join(root, "DATA2"))
	sc0, err := state.LoadScenario(c2, state.Slot("001"))
	if err != nil {
		t.Fatal(err)
	}
	seedMas, _, _ := sc0.Tables()
	o, err := oracle.Load(filepath.Join(root, "AA.EXE"), root)
	if err != nil {
		t.Fatal(err)
	}
	defer o.Close()
	base, dgroup := battlePanelRig(t, o, seedMas)
	work := o.Word(oracle.Addr{Seg: dgroup, Off: battleWorkSeg})
	w16 := func(off int) int { return int(o.Word(oracle.Addr{Seg: work, Off: uint16(off)})) }
	nMas, nSta, nGen := state.MasterTableSize, state.PrefectureTableSize, state.GeneralTableSize
	staBase := base + uint32(nMas)
	genBase := staBase + uint32(nSta)
	at := w16(0x1bf8)

	// 被擒的人：主守軍第一支部隊的第 0 槽。捕獲方：主攻軍（軍力 2，玩家）。
	const captorArmy = 2
	captive := -1
	for tm := 0; tm < battleTeams && captive < 0; tm++ {
		rec := battleUnitBase + tm*battleUnitSize
		if w16(rec+unitLeaders) > 0 {
			captive = w16(rec)
		}
	}
	if captive < 0 {
		t.Fatal("主守軍沒有將領")
	}
	captorFaction := w16(0x175e + captorArmy*22 + 16)
	if ctl := o.Word(addr(base + uint32(captorFaction*72))); ctl != 1 {
		t.Fatalf("主攻軍的勢力 %d 操縱方是 %d，不是玩家", captorFaction, ctl)
	}
	t.Logf("戰場郡 %d，被擒的是人物 %d（勢力 %d），捕獲方勢力 %d", at, captive,
		o.Byte(addr(genBase+uint32(captive*30+18))), captorFaction)

	// 中途紮寨挑的那一格：從右下角往回找走得進去、沒有部隊的第一格——
	// 欄與列都不是 0，才驗得出游標常式回傳值的拆法（列 × 100 ＋ 欄）。
	field, err := battle.Load(o.Bytes(oracle.Addr{Seg: work, Off: 0x163a}, 120), nil)
	if err != nil {
		t.Fatal(err)
	}
	campCol, campRow := -1, -1
	for r := 9; r > 0 && campCol < 0; r-- {
		for c := 11; c > 0; c-- {
			if field.At(battle.FromOffset(c, r)).Passable() && w16(0x2532+(r*12+c)*2) == 0xFFFF {
				campCol, campRow = c, r
				break
			}
		}
	}

	var rndNs []int
	draws := 0
	recording := false
	o.OnCall(addr(rndFn), func(o *oracle.Oracle) {
		if n := int(int16(o.Arg(0))); recording && n > 0 {
			rndNs = append(rndNs, n)
		}
	})
	o.OnCall(captiveRandFn, func(*oracle.Oracle) {
		if recording {
			draws++
		}
	})
	var keys []byte
	keyMiss := false
	o.Stub(captiveKeyFn, func(*oracle.Oracle) uint32 {
		if len(keys) == 0 {
			keyMiss = true
			return '1'
		}
		k := keys[0]
		keys = keys[1:]
		return uint32(k)
	})
	o.StubValue(captiveWaitFn, 0x20)
	o.StubValue(captiveSayFn, 0)
	campAsked := 0
	o.Stub(captiveCampFn, func(*oracle.Oracle) uint32 {
		campAsked++
		return uint32(campRow*100 + campCol)
	})
	saved := o.Save()

	type setup struct {
		lord    bool
		renown  int
		loyalty int
	}
	cases := []struct {
		name  string
		keys  string
		setup setup
	}{
		{"斬首", "1", setup{renown: 50, loyalty: 90}},
		{"囚禁", "2", setup{renown: 50, loyalty: 90}},
		{"釋放", "3", setup{renown: 50, loyalty: 90}},
		{"招降", "4", setup{renown: 100, loyalty: 0}},
		{"招降不從之後斬首", "41", setup{renown: 0, loyalty: 90}},
		{"君主不能囚禁之後釋放", "23", setup{lord: true, renown: 50, loyalty: 90}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			o.Restore(saved)
			gen := genBase + uint32(captive*30)
			if tc.setup.lord {
				o.SetByte(addr(gen+17), 0)
			} else {
				o.SetByte(addr(gen+17), 3)
			}
			o.SetByte(addr(gen+16), byte(tc.setup.loyalty))
			o.SetWord(addr(base+uint32(captorFaction*72+8)), uint16(tc.setup.renown))

			live := o.Bytes(addr(base), nMas+nSta+nGen)
			// 捕獲方五支部隊的記錄要在呼叫之前讀：招降會把人放進去。
			type teamRec struct {
				n, col, row int
				idx         []int
			}
			var teams [battleTeams]teamRec
			for tm := range teams {
				rec := battleUnitBase + (captorArmy*battleUnitPer+tm)*battleUnitSize
				teams[tm] = teamRec{n: w16(rec + unitLeaders), col: w16(rec + unitCol), row: w16(rec + unitRow)}
				for pos := 0; pos < teams[tm].n && pos < 10; pos++ {
					teams[tm].idx = append(teams[tm].idx, w16(rec+pos*2))
				}
			}
			ds := o.DSReg()
			seed := uint32(o.Word(oracle.Addr{Seg: ds, Off: 0xa3ae})) | uint32(o.Word(oracle.Addr{Seg: ds, Off: 0xa3b0}))<<16

			keys, keyMiss, rndNs, draws, campAsked = []byte(tc.keys), false, nil, 0, 0
			recording = true
			if _, err := o.CallBudget(400_000_000, captiveFn, captorArmy, uint16(captive)); err != nil {
				t.Fatalf("原版的處置常式停止：%v", err)
			}
			recording = false
			if keyMiss || len(keys) != 0 {
				t.Fatalf("鍵序 %q 沒有剛好用完（多要：%v，剩下：%q）", tc.keys, keyMiss, keys)
			}
			after := o.Bytes(addr(base), nMas+nSta+nGen)

			// remake：同一份盤面、同一個種子。
			sc, err := state.DecodeTables(state.Slot("001"), live[:nMas], live[nMas:nMas+nSta], live[nMas+nSta:])
			if err != nil {
				t.Fatal(err)
			}
			g, err := game.New(sc, state.FactionID(captorFaction), 5, state.EditionBase)
			if err != nil {
				t.Fatal(err)
			}
			b := battle.New(battle.Setup{Field: field, FixedWeather: true})
			b.Units = nil
			toSide := [...]battle.Side{battle.MainDefender, battle.AidDefender, battle.MainAttacker, battle.AidAttacker}
			for tm, tr := range teams {
				if tr.n <= 0 {
					continue
				}
				u := &battle.Unit{Side: toSide[captorArmy], Formation: battle.DeployOrder()[tm],
					At: battle.FromOffset(tr.col, tr.row)}
				for _, idx := range tr.idx {
					u.Leaders = append(u.Leaders, battle.Leader{Index: idx, Soldiers: 100})
				}
				b.Units = append(b.Units, u)
			}
			b.Computer[battle.MainAttacker] = false
			b.Renown[battle.MainAttacker] = tc.setup.renown
			b.Host = g.CaptiveHost(at)
			rseed := seed
			var myNs []int
			b.UseRoll(func(n int) int {
				var out int
				rseed, out = game.MSCRand(rseed)
				myNs = append(myNs, n)
				return out % n
			})
			rec := live[nMas+nSta+captive*30:]
			x := &battle.Leader{Index: captive, Intel: rec[9], War: rec[10],
				Lord: rec[17] == 0, Loyalty: int(int8(rec[16]))}
			if bond := int(binary.LittleEndian.Uint16(rec[14:])); bond != captive && bond < 350 {
				x.BondAlly = live[nMas+nSta+bond*30+18] == rec[18]
			}
			answers := []byte(tc.keys)
			fates := map[byte]battle.Fate{'1': battle.Executed, '2': battle.Jailed, '3': battle.Released, '4': battle.Defected}
			b.PlayerCaptive = func(battle.Side, *battle.Leader) battle.Fate {
				if len(answers) == 0 {
					t.Fatal("remake 多問了一次")
				}
				f := fates[answers[0]]
				answers = answers[1:]
				return f
			}
			myCamp := 0
			b.PlayerCamp = func(*battle.Unit) battle.Hex {
				myCamp++
				return battle.FromOffset(campCol, campRow)
			}
			b.Capture(battle.MainAttacker, nil, x)
			if len(answers) != 0 {
				t.Errorf("remake 少問了：剩下 %q", answers)
			}

			if !reflect.DeepEqual(rndNs, myNs) || draws != len(myNs) {
				t.Errorf("擲骰：原版 RND %v（rand %d 次）、remake %v", rndNs, draws, myNs)
			} else {
				t.Logf("擲骰兩邊相同：%v", myNs)
			}
			if campAsked != myCamp {
				t.Errorf("中途紮寨：原版問 %d 次、remake %d 次", campAsked, myCamp)
			}

			g.ApplyCaptiveFate(at, *x, state.FactionID(captorFaction))
			_, rsta, rgen, err := g.Tables()
			if err != nil {
				t.Fatal(err)
			}
			origGen := after[nMas+nSta+captive*30:]
			myGen := rgen[captive*30:]
			for _, off := range []int{17, 18, 19} {
				if origGen[off] != myGen[off] {
					t.Errorf("人物 offset %d：原版 %d、remake %d", off, origGen[off], myGen[off])
				}
			}
			if x.Fate == battle.Defected || x.Fate == battle.Released {
				if origGen[16] != myGen[16] {
					t.Errorf("人物 offset 16（忠誠）：原版 %d、remake %d", origGen[16], myGen[16])
				}
			}
			if x.Fate == battle.Released && x.ReleasedTo > 0 {
				off := nMas + x.ReleasedTo*176 + 32
				if o32, m32 := binary.LittleEndian.Uint16(after[off:]), binary.LittleEndian.Uint16(rsta[x.ReleasedTo*176+32:]); o32 != m32 &&
					binary.LittleEndian.Uint16(live[off:]) != o32 {
					t.Errorf("去處郡 %d 的主事者：原版 %d、remake %d", x.ReleasedTo, o32, m32)
				}
			}
			if x.Fate == battle.Released && x.ReleasedTo > 0 {
				q := nMas + x.ReleasedTo*176
				t.Logf("去處郡 %d：現役數 %d → %d、主事者 %d → %d；remake 現役（人物表推導）%d、主事者 %d",
					x.ReleasedTo, live[q+22], after[q+22], binary.LittleEndian.Uint16(live[q+32:]), binary.LittleEndian.Uint16(after[q+32:]),
					g.ActiveGenerals(x.ReleasedTo), binary.LittleEndian.Uint16(rsta[x.ReleasedTo*176+32:]))
			}
			idle := func(tb []byte, o int) int { return int(tb[o+at*176+23]) }
			t.Logf("結果 %v（去處 %d）；戰場郡在野數 %d → %d；人物 17/18/19 ＝ %d/%d/%d",
				x.Fate, x.ReleasedTo, idle(live, nMas), idle(after, nMas), origGen[17], origGen[18], origGen[19])
			if x.Fate == battle.Defected {
				// 招降進的那一支：原版是五個槽裡最後一個未滿 10 的，比位置與將領數。
				for tm := battleTeams - 1; tm >= 0; tm-- {
					rec := battleUnitBase + (captorArmy*battleUnitPer+tm)*battleUnitSize
					if teams[tm].n >= 10 {
						continue
					}
					oc, or, on := w16(rec+unitCol), w16(rec+unitRow), w16(rec+unitLeaders)
					for _, u := range b.Units {
						if u.Formation != battle.DeployOrder()[tm] {
							continue
						}
						if u.At != battle.FromOffset(oc, or) || u.LeaderCount() != on {
							t.Errorf("招降進的%v：原版在 (%d,%d) %d 位、remake 在 %v %d 位", u.Formation, oc, or, on, u.At, u.LeaderCount())
						} else {
							t.Logf("招降進的%v：兩邊都在 (%d,%d)、%d 位", u.Formation, oc, or, on)
						}
					}
					break
				}
			}
		})
	}
}
