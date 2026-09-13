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
//	  天數 > RND(11) + 20：兩邊同時互相削減戰力
//	      新守方 ＝ max(0, 守方 − 攻方 × 攻方品質)
//	      新攻方 ＝ max(0, 攻方 − 守方 × 守方品質)
//	  守方戰力歸零 → 攻方勝；攻方戰力歸零 → 守方勝
//	  天數 >= 30 → 守方勝（**不看城池那一格，也不看統帥在不在**）
//
// 傷亡在打完之後一次寫回（`0x1f6fe` 頭段算比例、`0x1f9fe` 逐人乘上去）：
//
//	存活比例 ＝ 結束時的戰力 ÷ (開場戰力 ＋ 0.0001)
//	每一位將領：兵力 ← trunc(兵力 × 他那一方的存活比例)
//
// **四個軍力都乘**，贏的那一方也會折損；同一方的所有部隊共用一個比例。
// 原版接著把每個人的所在郡清成 0 再重新安置，remake 這一邊的安置在
// `game.settle`，所以這裡不動所在郡。
func (b *Battle) AutoResolveAI() { b.autoResolveAI(b.roll) }

// AutoResolveAIWithRoll 讓可重播的 oracle 明示提供每次 RND(n) 的結果。
// 正式遊戲使用 AutoResolveAI；這個入口只隔離亂數器差異，不另做一套規則。
func (b *Battle) AutoResolveAIWithRoll(roll func(int) int) {
	if roll == nil {
		roll = b.roll
	}
	b.autoResolveAI(roll)
}

func (b *Battle) autoResolveAI(roll func(int) int) {
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
	att0, def0 := att, def
	riceA, riceD := b.Rice[MainAttacker], b.Rice[MainDefender]

	// casualties 把「結束時的戰力 ÷ 開場戰力」乘回每一位將領的兵力。
	// epsilon 是原版加的（`DS:0xa812` ＝ 0.0001），為的是兩邊都沒兵時
	// 這個除法仍然除得下去。
	casualties := func() {
		ra, rd := att/(att0+0.0001), def/(def0+0.0001)
		for _, u := range b.Units {
			r := rd
			if u.Side.Attacking() {
				r = ra
			}
			for i := range u.Leaders {
				l := &u.Leaders[i]
				l.Soldiers = int(float64(l.Soldiers) * r)
			}
		}
	}
	win := func(attacker bool, key string) {
		casualties()
		b.Over, b.AttackerWon = true, attacker
		b.note(key)
	}
	for b.Day = 1; ; b.Day++ {
		if b.Day%3 == 0 {
			riceA = int(float64(riceA) - att*0.01)
			if riceA <= 0 {
				win(false, "blog.aiAttRice")
				return
			}
			riceD = int(float64(riceD) - def*0.01)
			if riceD <= 0 {
				win(true, "blog.aiDefRice")
				return
			}
		}
		// 每天重擲一次「今天是否開始互相削兵」的門檻（`0x1f5d8`）：
		// 門檻落在第 20–30 天，只有當日已超過門檻才進傷亡段。
		if b.Day > roll(11)+20 {
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
			win(true, "blog.aiDefRout")
			return
		}
		if att <= 0 {
			win(false, "blog.aiAttRout")
			return
		}
		if b.Day >= BattleDays {
			win(false, "blog.aiTime")
			return
		}
	}
}
