package battle

// 電腦對電腦的戰役結算（原版 `0x1f538`，`docs/re/05` §7.1）。
//
// **與 `Auto()` 不是同一回事。** `Auto()` 是把戰術層自動打完——那是原版
// 「其他 → 戰役」開著、玩家不想看時走的路。這一支是原版在**四個郡都沒有
// 玩家**時走的另一條（`0x20471` 的分岔）：根本不進戰術層，沒有格子、
// 沒有計謀、沒有一騎討，整場只用兩個數。
//
//	戰力 ＝ Σ 部隊的兵士數
//	品質 ＝ Σ(部隊的綜合能力 × 0.01) ÷ 部隊數
//
// 量到的證據：三個月裡電腦進攻 5 次，戰役入口與「天數 ← 1」各 5 次，
// 而每天判一次的統帥條件（`0x24f8c`）與三十天期滿（`0x250d4`）
// **一次都沒跑**（`internal/parity/battleover_oracle_test.go`）。

// AutoResolveAI 照原版電腦對電腦那條路把勝負定出來。
//
//	每天：
//	  天數 % 3 == 0：兩邊各按自己的戰力吃糧（戰力 × 0.01），糧盡的一方敗
//	  天數 <= RND(11) + 20：兩邊同時互相削減戰力
//	      新守方 ＝ max(0, 守方 − 攻方 × 攻方品質)
//	      新攻方 ＝ max(0, 攻方 − 守方 × 守方品質)
//	  守方戰力歸零 → 攻方勝；攻方戰力歸零 → 守方勝
//	  天數 >= 30 → 守方勝（**不看城池那一格，也不看統帥在不在**）
//
// ⚠ **傷亡怎麼寫回去還沒解**：`0x1f538` 只動隨軍的米與兩個戰力，
// 部隊的兵士數整場沒被碰過；寫回去那一段（`0x1fb26` 之後）還沒讀。
// 所以這一支**不扣任何人的兵**，勝負以外的狀態一律不動。
func (b *Battle) AutoResolveAI() {
	if b.Over {
		return
	}
	side := func(main, aid Side) (strength, quality float64) {
		men, ability, n := 0, 0, 0
		for _, u := range b.Units {
			if u.Side != main && u.Side != aid {
				continue
			}
			if !u.Alive() {
				continue
			}
			men += u.Soldiers()
			ability += u.Ability()
			n++
		}
		if n == 0 {
			return float64(men), 0
		}
		return float64(men), float64(ability) * 0.01 / float64(n)
	}
	att, qa := side(MainAttacker, AidAttacker)
	def, qd := side(MainDefender, AidDefender)
	riceA, riceD := b.Rice[MainAttacker], b.Rice[MainDefender]

	win := func(attacker bool, msg string) {
		b.Over, b.AttackerWon = true, attacker
		b.note("%s", msg)
	}
	for b.Day = 1; ; b.Day++ {
		if b.Day%3 == 0 {
			riceA = int(float64(riceA) - att*0.01)
			if riceA <= 0 {
				win(false, "攻方糧盡，守軍獲勝")
				return
			}
			riceD = int(float64(riceD) - def*0.01)
			if riceD <= 0 {
				win(true, "守方糧盡，攻軍獲勝")
				return
			}
		}
		// 每天重擲一次「這一仗還撐不撐得住」的門檻（`0x1f5d8`）：
		// 打過 20–30 天之間的一個亂數之後就不再互相削減了。
		if b.Day <= b.roll(11)+20 {
			nd, na := def-att*qa, att-def*qd
			if nd < 0 {
				nd = 0
			}
			if na < 0 {
				na = 0
			}
			def, att = nd, na
		}
		if def <= 0 {
			win(true, "守方潰散，攻軍獲勝")
			return
		}
		if att <= 0 {
			win(false, "攻方潰散，守軍獲勝")
			return
		}
		if b.Day >= BattleDays {
			win(false, "卅天期滿，守軍獲勝")
			return
		}
	}
}
