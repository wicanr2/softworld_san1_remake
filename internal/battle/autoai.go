package battle

import "math/big"

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
	// 糧是**直接寫回兩個主軍的隨軍記錄**（`es:0x1792`／`es:0x1766`，
	// 加強版 `0x1d700`／`0x1d737`）：三天扣一次的量會留到收尾，戰場郡收
	// 進去的是扣過的數。存進區域變數再丟掉會讓戰後的米多出整場的消耗
	//（加強版月度對拍量到郡 30：原版 497、不寫回 905）。
	riceA, riceD := &b.Rice[MainAttacker], &b.Rice[MainDefender]

	// casualties 把「結束時的戰力 ÷ 開場戰力」乘回每一位將領的兵力。
	// epsilon 是原版加的（`DS:0xa812` ＝ 0.0001），為的是兩邊都沒兵時
	// 這個除法仍然除得下去。
	//
	// **比例先落地成 double 再乘**（`fstpl` 進區域變數，`0x1db27` 用
	// `fmull` 讀回來），乘積在擴充精度裡截尾（`x87MulTrunc`）——Go 的
	// `float64` 乘會把 2999.999… 捨入成 3000.0 再截。
	casualties := func() {
		ra, rd := x87Quo(att, att0+0.0001), x87Quo(def, def0+0.0001)
		for _, u := range b.Units {
			r := rd
			if u.Side.Attacking() {
				r = ra
			}
			for i := range u.Leaders {
				l := &u.Leaders[i]
				l.Soldiers = x87MulTrunc(float64(l.Soldiers), r)
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
			// `糧 − 戰力 × 0.01` 整段在 8087 的暫存器裡算完才截尾
			//（`0x1d6e0`–`0x1d6f7`：filds／fldl／fmull／fsubrp／ftol）。
			// 0.01 的 double 比 0.01 大一點，12500 × 0.01 在擴充精度是
			// 125.000…003，2125 減它是 1999.999… → 截成 1999；Go 的
			// `float64` 乘會先把乘積捨回 125.0，得 2000。每三天差一單位，
			// 加強版月度對拍量到郡 13 打完差 8。
			*riceA = x87ConsumeTrunc(*riceA, att)
			if *riceA <= 0 {
				win(false, "blog.aiAttRice")
				return
			}
			*riceD = x87ConsumeTrunc(*riceD, def)
			if *riceD <= 0 {
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

// ext 建一個 8087 擴充精度（64 位元有效位數）的暫存值。
func ext(v float64) *big.Float {
	return new(big.Float).SetPrec(64).SetMode(big.ToNearestEven).SetFloat64(v)
}

// x87Quo 是 a ÷ b 在擴充精度裡除完、再以 `fstpl` 落地成 double 的值。
func x87Quo(a, b float64) float64 {
	q, _ := new(big.Float).SetPrec(64).Quo(ext(a), ext(b)).Float64()
	return q
}

// x87MulTrunc 是 a × b 在擴充精度裡乘完直接 `ftol` 截尾的整數。
func x87MulTrunc(a, b float64) int {
	n, _ := new(big.Float).SetPrec(64).Mul(ext(a), ext(b)).Int64()
	return int(n)
}

// x87ConsumeTrunc 是 `rice − strength × 0.01` 的 8087 算法：0.01 取
// double 常數（`DS:0xa9dc`），乘與減都留在擴充精度，最後 `ftol` 截尾。
func x87ConsumeTrunc(rice int, strength float64) int {
	f := new(big.Float).SetPrec(64).Mul(ext(strength), ext(0.01))
	f.Sub(ext(float64(rice)), f)
	n, _ := f.Int64()
	return int(n)
}
