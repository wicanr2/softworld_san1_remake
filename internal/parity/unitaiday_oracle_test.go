//go:build oracle

package parity

import (
	"fmt"
	"path/filepath"
	"strings"
	"testing"

	"github.com/wicanr2/dosgolem/oracle"

	"github.com/wicanr2/softworld_san1_remake/internal/assets"
	"github.com/wicanr2/softworld_san1_remake/internal/battle"
	"github.com/wicanr2/softworld_san1_remake/internal/game"
	"github.com/wicanr2/softworld_san1_remake/internal/state"
	"github.com/wicanr2/softworld_san1_remake/internal/ui"
)

// TestZZUnitAIDayParity 對拍原版部隊 AI 的九支判斷式（Issue #22）：玩家
// 親征，盤面直寫記憶體（`stageABattle`），每一支電腦部隊每一天的決策
// ——**哪一支定案、對誰、走到哪**——與 remake 的 `DecideBase` 逐次比。
//
// 比的方法：攔決策鏈入口 `0x29014`，把原版當下的盤面（四個軍力的部隊
// 記錄、軍力記錄、地圖、天候、難度）拍下來，並記下這條鏈裡每一次
// `RND(n)` 擲出的值；鏈結束（`0x29132`）時讀原版定案的選項、目標與
// 部隊的落點。remake 從同一份盤面出發、用同一串骰值走 `DecideBase`，
// 三件事逐一相同才算對。
//
// **盤面每一次決策都從原版重拍**，所以比的是判斷式本身。
//
// `SAN1_NORESYNC=1` 多跑一段**不重拍**（Issue #24）：只拿第一條鏈的盤面，
// 之後 remake 自己走一整場——每支部隊的決策、後果常式（交戰結算、弓箭、
// 計謀、退兵、被擒處置）、回合結束的投敵判定與回填、玩家那幾支的休息、
// 日結算與天候——骰用 MSC 的 LCG 從原版的種子接，每一條鏈進來時比種子、
// 這支部隊的狀態與決策。原版讀鍵時會重新播種（`docs/re/03` §1.45），
// 那幾個點照原版的值接。
func TestZZUnitAIDayParity(t *testing.T) {
	runUnitAIDayBoards(t, baseDayRig())
}

// TestZZUnitAIDayParityPlus 是加強版的同一支（Issue #28）：`plusDayRig`
// 出發（開新局選曹操、難度由盤面給），路標全換成 `ASV.EXE` 的，remake
// 走 `battle.AIPlus`。九支的差異在 `docs/re/05` §12.5。
func TestZZUnitAIDayParityPlus(t *testing.T) {
	runUnitAIDayBoards(t, plusDayRig())
}

// dayBoard 是一張盤面：玩家一支部隊帶多少兵、敵方幾位各多少兵、敵將的
// 謀略與戰力（0 ＝ 照劇本）、玩家這一邊的智（0 ＝ 不改）、難度。
type dayBoard struct {
	name                                             string
	soldiers, enemies, enemySoldiers, stats, myIntel int
	difficulty                                       int
	// weak 把玩家那一位的戰力、武裝、訓練壓到底（綜合能力 0，打不痛任何人）
	// ——玩家一旦打光對方的將領，原版就停在「請主公裁決」四選一，這條
	// 鍵序（0／Y）答不了，那一場就到此為止。要看更多天就別讓玩家殺人。
	weak bool
	// days 是最多跑幾天（0 ＝ 32）。
	days int
	// clearTarget 把目標郡原本駐守的人搬走，守方只剩盤面擺進去的那幾位
	// （加強版）；apart 把玩家那支擺在守軍帥隊同一直線隔一格的位置而不是
	// 貼著——弓箭那一支只射「同方向連走兩步」的目標。
	clearTarget, apart bool
	// enemyWeak 把守方的訓練與武裝壓到 0（綜合能力只剩戰力那一項）：
	// 配 weak 的玩家，兩邊都打不死人，死戰才有得看而且不會停在四選一。
	// enemyLoyal 把守方的忠誠擺成 100——在野出身的人忠誠是 −1，每回合
	// RND(5)==0 就投奔玩家，守方一支一位的盤面幾天就散了。
	// enemySoldiersBy 照目標郡裡的人物編號順序逐位改兵數（整編把編號最小的
	// 編成帥隊，之後先鋒、左、右、後）；沒列到的照 enemySoldiers。帥隊比
	// 鄰敵弱，其他部隊才會離開帥隊旁邊來打玩家；只有夠大的那一支能死戰。
	enemyWeak, enemyLoyal bool
	enemySoldiersBy       []int
	// allStats 把目標郡**原本駐守的人**的智與武也壓成 stats（stats 本身只
	// 改擺進去的那幾位）：原版的守將會用計、快戰也打得痛，要看的是對戰
	// 子畫面時，那兩位會先把玩家那支打光。
	// solid 把玩家那支擺在走得進去的地形上（見 placeNearDefenderRig）：
	// 對戰子畫面照那一格的地形碼挑版型，大山會索引到表外。
	allStats, solid bool
	// zeroWar 把玩家那一位的戰力壓成 0 而不是 weak 的 1：子畫面裡敵將的
	// 單挑門是 `戰力 > 玩家戰力 + RND(20)`，敵將戰力壓在 4（戰力值才會是 0、
	// 快戰打不痛人）時，0 比 1 多一格機會。
	zeroWar bool
	// short 表示這張盤面本來就幾天就打完（要看的是一場單挑），不套
	// 「至少十次決策」的樣本門。
	short bool
	// enemyWar 把守方每一位的戰力另外壓成這個值（0 ＝ 不動）：stats 把智與
	// 戰力一起設，要「智 99 會用計、戰力 1 打不痛人」就靠它。
	enemyWar int
	// weatherDays 把每一天的天候直寫進原版（0 晴、1 雨、2 風；索引是
	// 天數 − 1，超出就從頭循環）——火攻要風、水淹要雨、燒糧不能雨。
	// 每天第一條鏈進來之前寫，remake 那一邊在同一個時點跟著設。
	weatherDays []int
	// shallow 把玩家那支旁邊的一個空格改成淺水（地形碼 3）：水淹要目標的
	// 鄰格有淺水。
	shallow bool
	// enemyGold 把目標郡的金改成這個數（0 ＝ 照 stageABattleWith 的 500）：
	// 守方軍力的金從它來，火攻 600、水淹 500，只有五百一次都用不了。
	enemyGold int
}

// dayRig 是一版的路標：哪些位址攔、工作區的哪幾格讀。兩版的戰場工作區
// 版面只差幾個純量的位移（`docs/re/05` §8.2），部隊記錄、軍力記錄、
// 地圖、城池格、天候、佔位圖的相對版面相同。
type dayRig struct {
	name    string
	exe     string
	root    func(*testing.T) string
	edition state.Edition
	ai      battle.AI
	// boot 把原版帶到能發動戰役的局面，回傳三張表的基底。
	boot func(t *testing.T, o *oracle.Oracle, bd dayBoard) uint32
	// boards 是這一版要跑的盤面。
	boards []dayBoard
	// seedLo／seedHi 是 MSC `rand()` 種子的兩格（DS 位移）。
	seedLo, seedHi uint16
	// rnd 是 `RND(n)` 包裝；rand／srand 是 MSC 的兩支；msg 是對白常式。
	rnd         uint32
	rand, srand oracle.Addr
	msg         uint32
	// enter 是進主戰場時攔一次記 DGROUP 的位址；cmdRead 是玩家每日命令
	// 讀到鍵之後的那一條指令（紮寨走完的判準）。
	enter, cmdRead uint32
	// chain／chainExit 是決策鏈的入口與出口；options 是九支的入口；
	// archery 是射箭常式（目標從它的參數讀）。
	chain, chainExit uint32
	options          map[uint32]int
	archery          uint32
	// workSegPtr 是戰場工作區的段值放在 DS 的哪一格；unitBase 是部隊記錄
	// 的基底；day／difficulty／occ 是工作區裡天數、難度、佔位圖的位移；
	// colTable／rowTable 是六方向位移表（DS）。
	workSegPtr           uint16
	unitBase             int
	day, difficulty, occ int
	colTable, rowTable   uint16
	// keyGap 是進戰場那串鍵同一段裡兩個鍵之間跑的指令數（0 ＝ 一段一送）。
	keyGap uint64
	// braveRet 是單挑常式裡「戰力 ≥ RND(5) + 90」那一擲的回傳位址：
	// 玩家答了 N 原版才走到它，答 Y 直接進打鬥——用來從骰序回推玩家
	// 在「接受嗎(Y/N)」的答案（`duelAnswerFrom`）。
	braveRet string
	// lure 是誘敵常式的入口（特效在它裡面）；0 ＝ 這一版不比特效。
	lure uint32
}

// baseDayRig 是原版的路標（`docs/re/05` §12、`docs/re/03` §1.45）。
func baseDayRig() dayRig {
	return dayRig{
		name: "base", exe: "AA.EXE", root: origRoot,
		edition: state.EditionBase, ai: battle.AIBase,
		boot: func(t *testing.T, o *oracle.Oracle, _ dayBoard) uint32 {
			c := openContainer(t, filepath.Join(origRoot(t), "DATA2"))
			sc0, err := state.LoadScenario(c, state.Slot("001"))
			if err != nil {
				t.Fatal(err)
			}
			seedMas, _, _ := sc0.Tables()
			return bootToGame(t, o, seedMas)
		},
		// 四張盤面，讓九支都輪得到（選項 1 是評估前置，每次都跑）：
		//   甲 玩家兩萬兵對五支各一千五 → 移動、弓箭、策略、快戰、休息
		//   乙 玩家三萬兵對五支各一千   → 退兵（相鄰敵軍四倍以上、總兵力比 ≥ 3）
		//   丙 玩家六千對五支各兩萬七、敵將謀略戰力 5 → 死戰（目標兵力比
		//      ≤ 0.23）；敵將能力壓低是讓玩家那支多撐幾天，決策才夠多
		//   丁 玩家兩千四（武 1，打不痛人）對四支：郡裡原有的兩位（武智壓成 1）
		//      與擺進去的兩位各五千 → 對戰（RND(16)==0 且 RND(100) ＋ 兵÷2 ≥
		//      目標兵，只有五千那兩支過得了；兵力比 0.48 > 0.23 擋掉死戰）。
		//      將領的兵不能超過五千——對戰子畫面的攻擊把「扣完 > 5000」的兵
		//      當成 0（`0x30628`），八千兵的將領打一下就被抓；雙方戰力值都是
		//      0，子畫面裡沒有人掉兵，打滿十三個時刻回主戰場（Issue #30）。
		//      RND(16)==0 這張盤面第 24 天才出現；跑滿 30 天，第 30 天原版
		//      還打一整天、動完才判期滿（Issue #36）
		//   戊 玩家兩萬兵、智 10 對五支各一千五、敵將智武 99 → 計謀**成功**的
		//      那幾條路（火攻／水淹／陷阱／誘敵／燒糧／圍攻的效果與骰序，
		//      Issue #24）；前四張盤面玩家的智是 99，電腦的計謀一次都不會成
		//   辛 玩家兩萬、智 10 對五支各一千五、敵將智 99 戰力 1、兩邊都打不痛
		//      人、天候單日風雙日雨、玩家旁邊擺一格淺水、守方金九千 →
		//      電腦用計**成功**的那幾條路（火攻要風、水淹要雨加淺水、
		//      圍攻要兩支貼著、誘敵無條件；`RND(6)` 挑計，三十天約八十次；
		//      Issue #40）
		//   己 同丁但四支五千、敵將武 4、玩家戰力 0 → 子畫面裡的**單挑**
		//      （敵將貼上玩家那一位時 `4 > 0 + RND(20)` 就叫陣，每個時刻
		//      兩成；武 4 是戰力值還是 0 的上限——城池攻值 27 ＋ 猛將 8，
		//      4×7×35 < 1000——再高快戰就打得痛人，玩家撐不到對戰；
		//      玩家的答案從鍵序來，Issue #38）
		// 難度是存檔裡的（5），這一版的 boot 不改它。
		boards: []dayBoard{
			{name: "甲", soldiers: 20000, enemies: 5, enemySoldiers: 1500, difficulty: 5},
			{name: "乙", soldiers: 30000, enemies: 5, enemySoldiers: 1000, difficulty: 5},
			{name: "丙", soldiers: 6000, enemies: 5, enemySoldiers: 27000, stats: 5, difficulty: 5},
			{name: "丁", soldiers: 2400, enemies: 2, enemySoldiers: 5000, stats: 1, difficulty: 5,
				weak: true, enemyWeak: true, enemyLoyal: true, allStats: true, solid: true, days: 30},
			{name: "戊", soldiers: 20000, enemies: 5, enemySoldiers: 1500, stats: 99, myIntel: 10, difficulty: 5},
			{name: "己", soldiers: 2400, enemies: 4, enemySoldiers: 5000, stats: 4, difficulty: 5,
				weak: true, zeroWar: true, enemyWeak: true, enemyLoyal: true, allStats: true, solid: true, days: 30},
			{name: "辛", soldiers: 20000, enemies: 5, enemySoldiers: 1500, stats: 99, enemyWar: 1, myIntel: 10, difficulty: 5,
				weak: true, enemyWeak: true, enemyLoyal: true, allStats: true, solid: true, shallow: true, enemyGold: 9000, days: 30,
				weatherDays: []int{2, 1}},
		},
		seedLo: 0xa3ae, seedHi: 0xa3b0,
		rnd: 0x10b0c, rand: oracle.Addr{Seg: 0x5c4, Off: 0x2cb0}, srand: oracle.Addr{Seg: 0x5c4, Off: 0x2c9e},
		msg:      0x3273e,
		braveRet: "30d6f",
		enter:    0x2053c, cmdRead: 0x27a68,
		chain: 0x29014, chainExit: 0x29132,
		options: map[uint32]int{
			0x29e78: 1, 0x29138: 2, 0x29344: 3, 0x2985c: 4, 0x29784: 5,
			0x29c56: 6, 0x29b82: 7, 0x29ade: 8, 0x29e2e: 9,
		},
		archery:    0x2a80a,
		workSegPtr: battleWorkSeg, unitBase: battleUnitBase,
		day: 0x2100, difficulty: 0x30fe, occ: 0x2532,
		colTable: 0x7c6a, rowTable: 0x7c82,
		lure: 0x2b6aa,
	}
}

// plusDayRig 是加強版的路標（`docs/re/05` §12.5、`docs/spec/015` §8.5）。
// 決策鏈與九支是從 `ASV.EXE` 的碼讀出來的：鏈 `0x2644a`–`0x26590`，
// 回合常式 `0x226b6`、日迴圈 `0x21542`、主戰場常式 `0x1e574`；工作區的純量往後移了幾格
//（天數 `0x2102`、難度 `0x310a`、佔位圖 `0x2536`、部隊記錄 `0x350e`）。
func plusDayRig() dayRig {
	return dayRig{
		name: "plus", exe: "ASV.EXE", root: plusRoot,
		edition: state.EditionPlus, ai: battle.AIPlus,
		boot: func(t *testing.T, o *oracle.Oracle, bd dayBoard) uint32 {
			base, _ := bootToNewGamePlus(t, o, caoCaoPick, bd.difficulty)
			// 整編的固定按鍵（`driveIntoBattle`）是照「攻方一位將」掃出來的
			//（`docs/re/05` §7）；開新局的曹操身邊有好幾位，只留君主。
			keepOnlyLord(t, o, base, bd.clearTarget)
			return base
		},
		// 六張盤面（難度 5 是聚在帥隊旁邊那一套）：
		//   甲 玩家兩萬對五位各一千五（目標郡原有的人留著，守方十位五支）
		//      → 移動、策略、快戰、休息
		//   乙 難度 15、玩家三萬對五位各一千、玩家打不痛人（免得第二天就停在
		//      「請主公裁決」）→ 退兵（RND(3)+3 < 總兵力比）、三十天判定的兩條
		//      加強版規則、`難度 mod 11` 的模式
		//   丙 玩家三百對五位（右軍一千一、其餘各一百）、敵將 5、守方只有這
		//      五位、忠誠 100、兩邊都打不死人 → 帥隊壓不住貼上來的玩家，
		//      其他部隊過來圍；只有第四天才准動的右軍能死戰（目標兵力比
		//      0.27 ≤ 0.4，一場 38 回合，每回合殺傷 8），一百人的部隊目標
		//      兵力比 3.0 要 RND(3)==1 才對戰、快戰打不到人
		//   丁 同上但帥隊三千、左軍八千、其餘四千 → 對戰（門 RND(8)、
		//      目標兵力比 < 2 誰都能）；子畫面的骰序是 Issue #30，不重拍到
		//      第一次對戰為止
		//   戊 玩家智 10 對五位智武 99 → 計謀成功的那幾條路
		//   己 玩家擺在帥隊同一直線隔一格 → 弓箭（沒有相鄰敵軍時只剩移動、
		//      弓箭、休息三支）
		//   庚 同丁但敵將武 99、難度 10（子畫面的行動門 92%）→ 子畫面裡的
		//      **單挑**：敵將貼上玩家那一位就叫陣（`戰力 > 1 + RND(5)`），
		//      玩家的答案從鍵序來（Issue #37／#38）
		boards: []dayBoard{
			{name: "甲", soldiers: 20000, enemies: 5, enemySoldiers: 1500, difficulty: 5},
			{name: "乙", soldiers: 30000, enemies: 5, enemySoldiers: 1000, difficulty: 15, weak: true, days: 14},
			{name: "丙", soldiers: 300, enemies: 5, enemySoldiers: 100, stats: 5, difficulty: 5, clearTarget: true,
				weak: true, enemyWeak: true, enemyLoyal: true, enemySoldiersBy: []int{100, 100, 100, 1100, 100}, days: 12},
			{name: "丁", soldiers: 2000, enemies: 5, enemySoldiers: 4000, stats: 5, difficulty: 5, clearTarget: true,
				weak: true, enemyWeak: true, enemyLoyal: true, enemySoldiersBy: []int{3000, 4000, 8000, 4000, 4000}, days: 12},
			{name: "戊", soldiers: 20000, enemies: 5, enemySoldiers: 1500, stats: 99, myIntel: 10, difficulty: 5, clearTarget: true},
			{name: "己", soldiers: 20000, enemies: 5, enemySoldiers: 1500, difficulty: 5, clearTarget: true, apart: true, days: 10},
			{name: "庚", soldiers: 2000, enemies: 5, enemySoldiers: 4000, stats: 99, difficulty: 10, clearTarget: true,
				weak: true, enemyWeak: true, enemyLoyal: true, enemySoldiersBy: []int{3000, 4000, 8000, 4000, 4000}, days: 12, short: true},
		},
		seedLo: 0xa566, seedHi: 0xa568,
		rnd: plusRndFn, rand: oracle.Addr{Seg: 0x5b9, Off: 0x2cb2}, srand: oracle.Addr{Seg: 0x5b9, Off: 0x2ca0},
		msg:      0x2f366,
		braveRet: "2db31",
		enter:    0x1e574, cmdRead: 0x25028,
		chain: 0x2644a, chainExit: 0x26590,
		options: map[uint32]int{
			0x27396: 1, 0x26594: 2, 0x26748: 3, 0x26d64: 4, 0x26ca4: 5,
			0x2718e: 6, 0x2709a: 7, 0x26fca: 8, 0x27354: 9,
		},
		archery:    0x27c74,
		workSegPtr: 0xaa6c, unitBase: 0x350e,
		day: 0x2102, difficulty: 0x310a, occ: 0x2536,
		colTable: 0x7dd8, rowTable: 0x7df0,
		keyGap: keyGap,
	}
}

// keepOnlyLord 把玩家的郡裡君主以外的人、以及目標郡（`stageABattleWith`
// 會挑的第一個鄰郡）原有的人全部放成在野搬去郡 42，郡的現役數跟著改。
// 目標郡清空是讓守方只有盤面擺進去的那五位——劇本裡原本駐在那裡的
// 小部隊一被玩家打光，原版就停在「請主公裁決」，那一場就到此為止。
func keepOnlyLord(t *testing.T, o *oracle.Oracle, base uint32, clearTarget bool) {
	t.Helper()
	nMas, nSta, nGen := state.MasterTableSize, state.PrefectureTableSize, state.GeneralTableSize
	raw := o.Bytes(addr(base), nMas+nSta+nGen)
	mas, sta, gen := raw[:nMas], raw[nMas:nMas+nSta], raw[nMas+nSta:]
	sc, err := state.DecodeTables(state.Slot("001"), mas, sta, gen)
	if err != nil {
		t.Fatal(err)
	}
	me := sc.Players()[0]
	at := 0
	for id := 1; id <= 42; id++ {
		if int(sta[id*176+30]) == me {
			at = id
			break
		}
	}
	dropped, lord := 0, -1
	for i := 0; i < 350; i++ {
		r := gen[i*30:]
		if int(r[18]) != me || int(r[19]) != at {
			continue
		}
		if r[17] == 0 {
			lord = i
			continue
		}
		// **所在郡也要搬走。** 整編的「分配那一位將軍(1-n)」列的是所在郡
		// 等於出兵郡的人，不看勢力——只把勢力改成在野，整編仍然列出七位
		//（實測）。搬去一個不相干的郡，在野的人不會把那一郡換旗。
		r[18], r[19] = 0xFF, 42
		dropped++
	}
	if lord < 0 {
		t.Fatalf("郡 %d 裡沒有君主", at)
	}
	sta[at*176+22] = 1
	to := 0
	for k := 45; k <= 54; k++ {
		if n := int(sta[at*176+k]); n != 0xFF && n != 0 {
			to = n
			break
		}
	}
	cleared := 0
	if !clearTarget {
		to = 0
	}
	for i := 0; i < 350 && to != 0; i++ {
		r := gen[i*30:]
		if int(r[19]) != to || r[18] == 0xFF {
			continue
		}
		r[18], r[19] = 0xFF, 42
		cleared++
	}
	if to != 0 {
		sta[to*176+22] = 0
	}
	o.SetBytes(addr(base), raw)
	t.Logf("郡 %d 只留君主（人物 %d），%d 位放成在野搬去郡 42；目標郡 %d 原有的 %d 位也搬走", at, lord, dropped, to, cleared)
}

// runUnitAIDayBoards 跑一版的所有盤面，合計原版各選項定案的次數。
func runUnitAIDayBoards(t *testing.T, rig dayRig) {
	seen := map[int]int{}
	for _, bd := range rig.boards {
		t.Run(bd.name, func(t *testing.T) {
			for k, v := range runUnitAIDayParity(t, rig, bd) {
				seen[k] += v
			}
		})
	}
	t.Logf("%s %d 張盤面合計，原版各選項定案：%v", rig.name, len(rig.boards), seen)
	for opt := 2; opt <= 9; opt++ {
		if seen[opt] == 0 {
			t.Errorf("%s 的盤面裡選項 %d 一次都沒定案——盤面要再調", rig.name, opt)
		}
	}
}

// restInSkirmish 是對戰子畫面裡玩家那一方的答案：每一位每一時刻都休息
// ——與送給原版的鍵序（「0」再「Y」）相同。
func restInSkirmish(*battle.Skirmish, *battle.SkirmishGeneral) battle.SkirmishCommand {
	return battle.SkirmishCommand{Kind: battle.SkirmishRest}
}

// duelAnswerFrom 從原版的骰序回推玩家在「接受嗎(Y/N)」的答案：答 N 之後
// 原版緊接著擲「戰力 ≥ RND(5) + 90」那一道（`braveRet`），答 Y 則接著是
// 接受那一句對白的 `RND(8)`。next 是 remake 問答案那一刻原版的下一擲。
func duelAnswerFrom(next, braveRet string) bool {
	return !strings.HasSuffix(next, "@"+braveRet)
}

// compactDraws 把連續的 `srand=` 折成一格（讀鍵的迴圈一次會播種幾十萬次）。
func compactDraws(in []string) []string {
	var out []string
	run, last := 0, ""
	flush := func() {
		if run > 0 {
			out = append(out, fmt.Sprintf("srand×%d→%s", run, strings.TrimPrefix(last, "srand=")))
			run = 0
		}
	}
	for _, e := range in {
		if strings.HasPrefix(e, "srand=") {
			run++
			last = e
			continue
		}
		flush()
		out = append(out, e)
	}
	flush()
	return out
}

// runUnitAIDayParity 跑一張盤面，回傳原版各選項定案的次數。
func runUnitAIDayParity(t *testing.T, rig dayRig, bd dayBoard) map[int]int {
	soldiers, enemies, enemySoldiers, enemyStats, myIntel := bd.soldiers, bd.enemies, bd.enemySoldiers, bd.stats, bd.myIntel
	root := rig.root(t)
	o, err := oracle.Load(filepath.Join(root, rig.exe), root)
	if err != nil {
		t.Fatal(err)
	}
	defer o.Close()

	base := rig.boot(t, o, bd)
	staBase := base + uint32(state.MasterTableSize)
	genBase := staBase + uint32(state.PrefectureTableSize)
	// 玩家一支部隊帶兩萬兵撐場、敵方五位（五支部隊）——決策多、而且
	// 打得完一場（守方五支輪流快戰，三十天內分得出勝負）。
	at, to := stageABattleWith(t, o, base, soldiers, enemies, enemySoldiers, enemyStats)
	if myIntel > 0 || bd.weak {
		// 把玩家這一邊的智壓低，電腦的計謀才過得了成功判定
		// （`RND(表) + 目標領隊的智 < 施法者領隊的智`，`docs/re/05` §4.1）。
		me := int(o.Byte(addr(staBase + uint32(at*176+30))))
		for i := 0; i < 350; i++ {
			rec := genBase + uint32(i*30)
			if int(o.Byte(addr(rec+18))) != me || int(o.Byte(addr(rec+19))) != at {
				continue
			}
			if myIntel > 0 {
				o.SetByte(addr(rec+9), byte(myIntel))
			}
			if bd.weak {
				o.SetByte(addr(rec+10), 1)
				if bd.zeroWar {
					o.SetByte(addr(rec+10), 0)
				}
				o.SetByte(addr(rec+24), 0)
				o.SetByte(addr(rec+25), 0)
			}
		}
	}
	if bd.enemyGold > 0 {
		o.SetWord(addr(staBase+uint32(to*176+18)), uint16(bd.enemyGold))
	}
	if bd.enemyWeak || bd.enemyLoyal || bd.allStats || len(bd.enemySoldiersBy) > 0 {
		nth := 0
		for i := 0; i < 350; i++ {
			rec := genBase + uint32(i*30)
			if int(o.Byte(addr(rec+19))) != to || o.Byte(addr(rec+18)) == 0xFF {
				continue
			}
			if bd.enemyWeak {
				o.SetByte(addr(rec+24), 0)
				o.SetByte(addr(rec+25), 0)
			}
			if bd.allStats && bd.stats > 0 {
				o.SetByte(addr(rec+9), byte(bd.stats))
				o.SetByte(addr(rec+10), byte(bd.stats))
			}
			if bd.enemyWar > 0 {
				o.SetByte(addr(rec+10), byte(bd.enemyWar))
			}
			if bd.enemyLoyal {
				o.SetByte(addr(rec+16), 100)
			}
			if nth < len(bd.enemySoldiersBy) {
				o.SetWord(addr(rec+22), uint16(bd.enemySoldiersBy[nth]))
			}
			nth++
		}
	}

	lureChecked, lureBad := checkLureFlash(t, o, rig, root)
	defer func() {
		if *lureBad != "" {
			t.Errorf("誘敵特效：%s", *lureBad)
		} else if *lureChecked > 0 {
			t.Logf("誘敵特效 %d 次，每一次二十二步那一格 48×32 與圖塊逐格相同、速度序列相同", *lureChecked)
		}
	}()

	var dgroup uint16
	o.OnCall(addr(rig.enter), func(o *oracle.Oracle) {
		if dgroup == 0 {
			dgroup = o.DSReg()
		}
	})
	cmdReads := 0
	o.OnCall(addr(rig.cmdRead), func(*oracle.Oracle) { cmdReads++ })

	work := func() uint16 { return o.Word(oracle.Addr{Seg: dgroup, Off: rig.workSegPtr}) }
	w16 := func(off int) int { return int(o.Word(oracle.Addr{Seg: work(), Off: uint16(off)})) }
	// `SAN1_SKIRMISH=1`：把對戰子畫面的每一步倒出來（Issue #30／#37）。
	var skirmish *[]string
	if envOr("SAN1_SKIRMISH", "") != "" {
		sites := baseSkirmishSites
		if rig.edition == state.EditionPlus {
			sites = plusSkirmishSites
		}
		skirmish = attachSkirmishTrace(t, o, work, w16, genBase, sites)
	}

	// 一次決策的紀錄。
	type decision struct {
		day, army, team int
		option          int
		tArmy, tTeam    int
		col, row        int // 鏈結束時部隊的落點
		rolls           []int
		rollNs          []int
		callers         []string
		around          []string
		model           *battle.Battle
		unit            *battle.Unit
		escapes         int
		// seed 是進決策鏈時原版的亂數種子，gap 是這條鏈結束到下一條
		// 鏈開始之間原版擲的骰（呼叫端），不重拍模式要接這些。
		// entry 是進鏈時這支部隊的樣子（複本，重拍那一段跑 DecideBase
		// 會改到 unit），others 是同一刻其餘每一支的複本（重拍那一段的
		// 快戰、計謀、圍攻也改到 model 裡被打的那幾支），fresh 是第一條鏈
		// 另拍的一份完整盤面，給不重拍模式從頭走。
		seed   uint32
		gap    []string
		entry  *battle.Unit
		others []*battle.Unit
		fresh  *battle.Battle
	}
	cloneUnit := func(u *battle.Unit) *battle.Unit {
		if u == nil {
			return nil
		}
		c := *u
		c.Leaders = append([]battle.Leader(nil), u.Leaders...)
		return &c
	}
	var decisions []*decision
	var cur *decision
	noresync := envOr("SAN1_NORESYNC", "") != ""
	curOpt := 0
	options := rig.options
	toSide := [...]battle.Side{battle.MainDefender, battle.AidDefender, battle.MainAttacker, battle.AidAttacker}
	teamForm := battle.DeployOrder()

	snapshot := func(army, team int) *decision {
		d := &decision{army: army, team: team, day: w16(rig.day)}
		field, err := battle.Load(o.Bytes(oracle.Addr{Seg: work(), Off: 0x163a}, 120), nil)
		if err != nil {
			t.Fatalf("戰場地圖讀不出來：%v", err)
		}
		if field.CityAt == battle.NoHex {
			field.CityAt = battle.FromOffset(w16(0x584), w16(0x586))
		}
		weather := battle.Clear
		switch w16(0x17bc) {
		case 1:
			weather = battle.Rainy
		case 2:
			weather = battle.Windy
		}
		b := battle.New(battle.Setup{Field: field, Weather: weather, FixedWeather: true, Difficulty: w16(rig.difficulty),
			AI: rig.ai, Rules: battle.RulesFor(rig.edition, w16(rig.difficulty))})
		b.Day = d.day
		b.Units = nil
		for a := 0; a < 4; a++ {
			arec := 0x175e + a*22
			b.Gold[toSide[a]] = w16(arec + 6)
			b.Rice[toSide[a]] = w16(arec + 8)
			// 統帥（軍力記錄 offset 0）：投敵判定跳過他。
			b.Commander[toSide[a]] = int(int16(w16(arec)))
			// 五個隊伍槽全掃，活著的判準是將領數 > 0：軍力記錄的部隊數
			// （offset 10）在一支被打光之後會少一，但槽號不會往前補。
			for tm := 0; tm < 5; tm++ {
				rec := rig.unitBase + (a*battleUnitPer+tm)*battleUnitSize
				if w16(rec+unitLeaders) <= 0 {
					continue
				}
				u := &battle.Unit{Side: toSide[a], Formation: teamForm[tm],
					At:      battle.FromOffset(w16(rec+unitCol), w16(rec+unitRow)),
					Move:    w16(rec + unitMove),
					Arrows:  w16(rec + 20),
					Trapped: w16(rec + unitTrapped),
					Quality: w16(rec + unitAbility),
					Cap:     w16(rec + unitCap),
				}
				// 槽號要保留：對戰子畫面照槽號擺起點、看第 0 槽在不在
				// （`0x2e68a`），被抓走留下的洞不能往前補。洞用一位「不在
				// 陣中」的佔位將領頂著（只到最後一位真的將領為止）。
				last := -1
				for pos := 0; pos < 10; pos++ {
					if w16(rec+pos*2) != 0xFFFF {
						last = pos
					}
				}
				for pos := 0; pos <= last; pos++ {
					idx := w16(rec + pos*2)
					if idx == 0xFFFF {
						u.Leaders = append(u.Leaders, battle.Leader{Index: -1, Dead: true})
						continue
					}
					g := genBase + uint32(idx*30)
					l := battle.Leader{
						Index: idx, Intel: o.Byte(addr(g + 9)), War: o.Byte(addr(g + 10)),
						Stamina:  o.Byte(addr(g + 8)),
						Soldiers: int(o.Word(addr(g + 22))), Training: o.Byte(addr(g + 24)),
						Arms: o.Byte(addr(g + 25)), Troop: battle.TroopKind(o.Byte(addr(g + 21))),
						Lord: o.Byte(addr(g+17)) == 0,
					}
					l.Loyalty = int(int8(o.Byte(addr(g + 16)))) // 在野的 0xFF 讀成 −1（cbw）
					if bond := int(o.Word(addr(g + 14))); bond != idx && bond < 350 {
						l.BondAlly = o.Byte(addr(genBase+uint32(bond*30)+18)) == o.Byte(addr(g+18))
					}
					u.Leaders = append(u.Leaders, l)
				}
				u.Started = u.Soldiers()
				if got, want := u.Soldiers(), w16(rec+unitSoldiers); got != want {
					t.Errorf("第 %d 天 軍力 %d 隊伍 %d：將領兵力和 %d，部隊記錄的兵士數 %d", d.day, a, tm, got, want)
				}
				b.Units = append(b.Units, u)
				if a == army && tm == team {
					d.unit = u
				}
			}
		}
		// 退兵逃得去的鄰郡（`0x23e34`–`0x23ef2`）：戰場所在郡的鄰郡裡
		// 無主或自己勢力的，扣掉對方助軍出兵的那一郡。四個軍力都算，
		// 不重拍模式要用；順便填誰是電腦、人望多少（被擒處置要看）。
		pref := w16(0x1bf8)
		for a := 0; a < 4; a++ {
			faction := w16(0x175e + a*22 + 16)
			if faction == 0xFFFF || faction >= 16 {
				continue
			}
			mrec := base + uint32(faction*72)
			b.Computer[toSide[a]] = o.Word(addr(mrec)) == 2
			b.Renown[toSide[a]] = int(o.Word(addr(mrec + 8)))
			other := 3
			if a >= 2 {
				other = 1
			}
			exclude := w16(0x175e + other*22 + 18)
			for k := 45; k <= 54; k++ {
				n := int(o.Byte(addr(staBase + uint32(pref*176+k))))
				if n == 0xFF || n == 0 || n == exclude {
					continue
				}
				owner := int(o.Byte(addr(staBase + uint32(n*176+30))))
				if owner != 0xFF && owner != faction {
					continue
				}
				b.Escapes[toSide[a]] = append(b.Escapes[toSide[a]],
					battle.Escape{Prefecture: n, Active: int(o.Byte(addr(staBase + uint32(n*176+22))))})
			}
		}
		d.escapes = len(b.Escapes[toSide[army]])
		d.model = b
		// 診斷：這支部隊六個鄰格的佔位（原版 `es:0x2532`）與那些部隊的兵士數。
		if d.unit != nil {
			rec := rig.unitBase + (army*battleUnitPer+team)*battleUnitSize
			c, r := w16(rec+unitCol), w16(rec+unitRow)
			for dir := 0; dir < 6; dir++ {
				i := ((c%2)*6 + dir) * 2
				dc := int(int16(o.Word(oracle.Addr{Seg: dgroup, Off: rig.colTable + uint16(i)})))
				dr := int(int16(o.Word(oracle.Addr{Seg: dgroup, Off: rig.rowTable + uint16(i)})))
				nc, nr := c+dc, r+dr
				if nc < 0 || nc >= 12 || nr < 0 || nr >= 10 {
					continue
				}
				occ := w16(rig.occ + (nr*12+nc)*2)
				if occ == 0xFFFF {
					continue
				}
				orec := rig.unitBase + ((occ/10)*battleUnitPer+occ%10)*battleUnitSize
				d.around = append(d.around, fmt.Sprintf("(%d,%d)=%d/%d 兵 %d 將 %d", nc, nr, occ/10, occ%10, w16(orec+unitSoldiers), w16(orec+unitLeaders)))
			}
		}
		if d.unit == nil {
			rec := rig.unitBase + (army*battleUnitPer+team)*battleUnitSize
			t.Logf("第 %d 天 軍力 %d 隊伍 %d：軍力記錄部隊數 %d、將領數 %d、兵士 %d、槽 %04x %04x、位置 (%d,%d)",
				d.day, army, team, w16(0x175e+army*22+10), w16(rec+unitLeaders), w16(rec+unitSoldiers),
				w16(rec), w16(rec+2), w16(rec+unitCol), w16(rec+unitRow))
		}
		return d
	}

	seedNow := func(o *oracle.Oracle) uint32 {
		ds := o.DSReg()
		return uint32(o.Word(oracle.Addr{Seg: ds, Off: rig.seedLo})) | uint32(o.Word(oracle.Addr{Seg: ds, Off: rig.seedHi}))<<16
	}
	// 鏈外的骰（玩家那支的命令、投敵判定、日結算、天候）記在上一條鏈
	// 上，值一樣從下一次讀到的種子回推。
	gapN, gapAt := 0, ""
	flushGap := func(o *oracle.Oracle) {
		if gapN > 0 && len(decisions) > 0 {
			last := decisions[len(decisions)-1]
			last.gap = append(last.gap, fmt.Sprintf("%d=%d@%s", gapN, int((seedNow(o)>>16)&0x7fff)%gapN, gapAt))
		}
		gapN = 0
	}
	forcedWeather := func(day int) int {
		if len(bd.weatherDays) == 0 {
			return -1
		}
		i := day - 1
		if i < 0 {
			i = 0
		}
		return bd.weatherDays[i%len(bd.weatherDays)]
	}
	o.OnCall(addr(rig.chain), func(o *oracle.Oracle) {
		if dgroup == 0 {
			return
		}
		flushGap(o)
		if wx := forcedWeather(w16(rig.day)); wx >= 0 {
			o.SetWord(oracle.Addr{Seg: work(), Off: 0x17bc}, uint16(wx))
		}
		cur = snapshot(int(int16(o.Arg(0))), int(int16(o.Arg(1))))
		cur.seed = seedNow(o)
		if noresync {
			cur.entry = cloneUnit(cur.unit)
			for _, ou := range cur.model.Units {
				if ou != cur.unit {
					cur.others = append(cur.others, cloneUnit(ou))
				}
			}
			if len(decisions) == 0 {
				cur.fresh = snapshot(cur.army, cur.team).model
			}
		}
		curOpt = 0
	})
	for a, n := range options {
		n := n
		o.OnCall(addr(a), func(*oracle.Oracle) {
			if cur != nil {
				curOpt = n
			}
		})
	}
	// `RND(n)` 擲出什麼：**從下一次讀到的種子回推**。`rand()` 的輸出就是
	// 更新後種子的第 16..30 位（`game.MSCRand`），而種子只有 `rand()` 會動，
	// 所以下一次進 `RND` 時（或鏈結束時）讀到的種子，就是上一擲的結果。
	// 不從進入時的種子往前算——那要假設這一份 `rand()` 的算式，回推不必。
	pendingN := 0
	flushRoll := func(o *oracle.Oracle) {
		if pendingN > 0 && cur != nil {
			cur.rolls = append(cur.rolls, int((seedNow(o)>>16)&0x7fff)%pendingN)
			cur.rollNs = append(cur.rollNs, pendingN)
		}
		pendingN = 0
	}
	// `srand()`（`0x5c4:0x2c9e`）：原版讀鍵的迴圈每等一輪就把計數器
	// `es:0x2172` 加一，鍵到了拿它重新播種（`0x10bb0`／`0x11832`／`0x11c2c`）。
	// 玩家每按一個鍵，種子就跳到那個計數值——記成 `srand=值@位址`，
	// 不重拍模式在同一個位置把 remake 的種子也跳過去。
	o.OnCall(rig.srand, func(o *oracle.Oracle) {
		if dgroup == 0 {
			return
		}
		flushGap(o)
		flushRoll(o)
		at := fmt.Sprintf("srand=%d@%05x", o.Arg(0), o.Caller().Linear())
		if cur != nil {
			cur.callers = append(cur.callers, at)
		} else if len(decisions) > 0 {
			last := decisions[len(decisions)-1]
			last.gap = append(last.gap, at)
		}
	})
	// 對白常式（`0x3273e`）的呼叫端：記成 `msg@位址`，看每一道 `RND(8)`
	// 是誰印的。
	o.OnCall(addr(rig.msg), func(o *oracle.Oracle) {
		if dgroup == 0 {
			return
		}
		at := fmt.Sprintf("msg@%05x", o.Caller().Linear())
		if cur != nil {
			cur.callers = append(cur.callers, at)
		} else if len(decisions) > 0 {
			last := decisions[len(decisions)-1]
			last.gap = append(last.gap, at)
		}
	})
	// 直接叫 `rand()`（`0x5c4:0x2cb0`）而不經 `RND(n)` 的呼叫端：記成
	// `rand@位址`。有這種呼叫，從種子回推的骰值就會錯位。
	viaWrapper := false
	o.OnCall(rig.rand, func(o *oracle.Oracle) {
		if viaWrapper {
			viaWrapper = false
			return
		}
		if dgroup == 0 {
			return
		}
		at := fmt.Sprintf("rand@%05x", o.Caller().Linear())
		if cur != nil {
			cur.callers = append(cur.callers, at)
		} else if len(decisions) > 0 {
			last := decisions[len(decisions)-1]
			last.gap = append(last.gap, at)
		}
	})
	o.OnCall(addr(rig.rnd), func(o *oracle.Oracle) {
		if n := int(int16(o.Arg(0))); n > 0 {
			viaWrapper = true
		}
		if cur == nil {
			flushGap(o)
			if n := int(int16(o.Arg(0))); n > 0 && dgroup != 0 {
				gapN, gapAt = n, fmt.Sprintf("%05x", o.Caller().Linear())
			}
			return
		}
		flushRoll(o)
		// `RND(0)`（移動那一支的 `push 0`）回 0 而且不動種子，remake 沒有
		// 對應的一擲，不記。
		if n := int(int16(o.Arg(0))); n > 0 {
			pendingN = n
			cur.callers = append(cur.callers, fmt.Sprintf("%d@%05x", n, o.Caller().Linear()))
		}
	})
	// 弓箭的目標不進 `es:0x31a8`，從射箭常式的參數讀。
	o.OnCall(addr(rig.archery), func(o *oracle.Oracle) {
		if cur != nil && curOpt == 4 {
			cur.tArmy, cur.tTeam = int(int16(o.Arg(2))), int(int16(o.Arg(3)))
		}
	})
	o.OnCall(addr(rig.chainExit), func(o *oracle.Oracle) {
		if cur == nil {
			return
		}
		flushRoll(o)
		// 定案槽、目標軍力、目標隊伍（原版 `es:0x31c0`／`0x31a8`／`0x1604`，
		// 加強版 `0x31cc`／`0x31b4`／`0x1604`）都在工作區。
		done, tArmy, tTeam := 0x31c0, 0x31a8, 0x1604
		if rig.edition == state.EditionPlus {
			done, tArmy = 0x31cc, 0x31b4
		}
		if w16(done) != 0 {
			cur.option = 0 // 沒有任何一支定案（不該發生：選項 9 無條件）
		} else {
			cur.option = curOpt
		}
		if cur.option >= 5 && cur.option <= 8 {
			cur.tArmy = int(int16(w16(tArmy)))
			cur.tTeam = int(int16(w16(tTeam)))
		}
		rec := rig.unitBase + (cur.army*battleUnitPer+cur.team)*battleUnitSize
		cur.col, cur.row = w16(rec+unitCol), w16(rec+unitRow)
		decisions = append(decisions, cur)
		cur = nil
	})

	driveIntoBattleGap(t, o, at, to, rig.keyGap)
	if dgroup == 0 {
		dumpScreen(t, o, "unitaiday-"+rig.name+"-noentry")
		t.Fatal("沒有進到主戰場")
	}
	for step := 1; step <= 12 && cmdReads == 0; step++ {
		o.Drain()
		o.PressScan("0")
		if err := o.Run(60_000_000); err != nil {
			t.Fatalf("紮寨第 %d 步停止：%v", step, err)
		}
	}
	// 玩家那支是在第一天的電腦部隊都動完之後才被搬到守軍旁邊的
	// （紮寨那幾步已經讓守方走完第一天）；不重拍模式要在同一個時點
	// 把 remake 的那支也搬過去。
	meRec, _ := placeNearDefenderRig(t, o, dgroup, rig, bd.apart, bd.solid)
	placedAt := battle.NoHex
	if meRec >= 0 {
		placedAt = battle.FromOffset(w16(meRec+unitCol), w16(meRec+unitRow))
	}
	shallowAt := battle.NoHex
	if bd.shallow && meRec >= 0 {
		// 玩家那支的六個鄰格裡挑第一個空格改成淺水（地形碼 3）。
		c, r := w16(meRec+unitCol), w16(meRec+unitRow)
		for dir := 0; dir < 6; dir++ {
			i := ((c%2)*6 + dir) * 2
			dc := int(int16(o.Word(oracle.Addr{Seg: dgroup, Off: rig.colTable + uint16(i)})))
			dr := int(int16(o.Word(oracle.Addr{Seg: dgroup, Off: rig.rowTable + uint16(i)})))
			nc, nr := c+dc, r+dr
			if nc < 0 || nc >= 12 || nr < 0 || nr >= 10 {
				continue
			}
			if o.Word(oracle.Addr{Seg: work(), Off: uint16(rig.occ + (nr*12+nc)*2)}) != 0xFFFF {
				continue
			}
			at := oracle.Addr{Seg: work(), Off: uint16(0x163a + nr*12 + nc)}
			o.SetByte(at, o.Byte(at)&0xf0|3)
			shallowAt = battle.FromOffset(nc, nr)
			t.Logf("把玩家那支旁邊的 (%d,%d) 改成淺水", nc, nr)
			break
		}
	}

	days := 32
	if bd.days > 0 {
		days = bd.days
	}
	if v := envOr("SAN1_DAYS", ""); v != "" {
		fmt.Sscan(v, &days)
	}
	// 鍵不照天數送，照「還有沒有新決策」送：玩家那支每天「0」休息，
	// 「Y」答掉沿路的確認；電腦選了對戰（選項 7）會進對戰子畫面，
	// 那裡的每一位將領也吃「0」休息，子畫面打完才回到主戰場——所以
	// 連續十五輪沒有新決策才當作這一場結束。
	quiet := 0
	o.Drain()
	o.PressScan("Y")
	if err := o.Run(120_000_000); err != nil {
		t.Fatalf("開戰確認停止：%v", err)
	}
	for i := 0; i < days*12 && quiet < 15; i++ {
		before := len(decisions)
		for _, k := range []string{"0", "Y"} {
			o.Drain()
			o.PressScan(k)
			if err := o.Run(60_000_000); err != nil {
				t.Fatalf("第 %d 輪送 %q 停止：%v", i+1, k, err)
			}
		}
		if len(decisions) == before {
			quiet++
			if quiet == 14 {
				dumpScreen(t, o, fmt.Sprintf("unitaiday-%d-quiet", soldiers))
			}
		} else {
			quiet = 0
		}
		if w16(rig.day) > days {
			break
		}
	}

	if skirmish != nil {
		for _, l := range *skirmish {
			t.Log(l)
		}
	}
	// remake 這一邊：同一份盤面、同一串骰值。
	sideName := func(a int) string { return toSide[a].String() }
	bad := 0
	byOpt := map[int]int{}
	for _, d := range decisions {
		if d.unit == nil {
			t.Errorf("第 %d 天 %s 隊伍 %d：原版在替一支 remake 認不出來的部隊決策", d.day, sideName(d.army), d.team)
			bad++
			continue
		}
		i := 0
		var asked []int
		d.model.PlayerSkirmish = restInSkirmish
		d.model.PlayerDuelAnswer = func(*battle.Skirmish, *battle.SkirmishGeneral, *battle.SkirmishGeneral) bool {
			if i < len(d.callers) {
				return duelAnswerFrom(d.callers[i], rig.braveRet)
			}
			return true
		}
		d.model.UseRoll(func(n int) int {
			asked = append(asked, n)
			if i < len(d.rolls) {
				v := d.rolls[i]
				i++
				if n > 0 {
					return v % n
				}
				return 0
			}
			return 0
		})
		got := d.model.DecideBase(d.unit)
		byOpt[d.option]++
		ok := got.Option == d.option
		want := fmt.Sprintf("選項 %d", d.option)
		have := fmt.Sprintf("選項 %d", got.Option)
		if d.option >= 4 && d.option <= 8 {
			want += fmt.Sprintf(" 對 %s 隊伍 %d", sideName(d.tArmy), d.tTeam)
			if got.Target != nil {
				have += fmt.Sprintf(" 對 %s %s", got.Target.Side, got.Target.Formation)
				if got.Target.Side != toSide[d.tArmy] || got.Target.Formation != teamForm[d.tTeam] {
					ok = false
				}
			} else {
				ok = false
			}
		}
		if d.option == 3 || got.Option == 3 {
			x, y := battle.ToOffset(d.unit.At)
			want += fmt.Sprintf(" 落點 (%d,%d)", d.col, d.row)
			have += fmt.Sprintf(" 落點 (%d,%d)", x, y)
			if x != d.col || y != d.row {
				ok = false
			}
		}
		line := fmt.Sprintf("第 %2d 天 %s 隊伍 %d：原版 %s；remake %s；骰 %v（n=%v）remake 問了 %v；可逃鄰郡 %d",
			d.day, sideName(d.army), d.team, want, have, d.rolls, d.rollNs, asked, d.escapes)
		if ok {
			t.Logf("✓ %s；RND 的呼叫端 %v", line, compactDraws(d.callers))
		} else {
			t.Errorf("✗ %s；RND 的呼叫端 %v；鄰格 %v；本隊兵 %d", line, compactDraws(d.callers), d.around, d.unit.Soldiers())
			bad++
		}
	}
	t.Logf("決策 %d 次，原版各選項定案：%v，不同 %d 次", len(decisions), byOpt, bad)
	if len(decisions) < 10 && !bd.short {
		t.Errorf("只比到 %d 次決策——樣本太少", len(decisions))
	}
	if !noresync || len(decisions) == 0 {
		return byOpt
	}

	// 不重拍模式（Issue #24）：只拿第一條鏈的盤面，之後 remake 自己走，
	// 骰用 MSC 的 LCG 從原版當時的種子接。每一條鏈進來時比三件事：
	// 種子（兩邊擲的次數一樣多）、這支部隊的狀態（兵、將領數、位置、
	// 移動力、箭、陷阱）、決策（選項／目標／落點）。種子岔開就把 remake
	// 的種子接回原版的，讓後面的鏈還比得下去，但算一次「岔開」。
	first := decisions[0]
	model := first.fresh
	seed := first.seed
	// 電腦選了對戰（選項 7）就進對戰子畫面，玩家那一位每一時刻吃「0」
	// 休息（上面的鍵序）；remake 這一邊用同一個答案。
	model.PlayerSkirmish = restInSkirmish
	// 玩家那一邊：原版每天在電腦的部隊之後輪到它們，測試每一支都送「0」
	// 休息；休息印一句對白，回合結束一樣判投敵、回填移動力。投敵或招降
	// 的人進了空槽位會多出一支（後軍），所以照行動順序逐支來。
	playerUnits := func() []*battle.Unit {
		var out []*battle.Unit
		for _, f := range battle.ActionOrder() {
			for _, u := range model.Units {
				if u.Side == battle.MainAttacker && u.Formation == f && u.Alive() {
					out = append(out, u)
				}
			}
		}
		return out
	}
	// 中途生出來的部隊原版是問玩家紮在哪（`0x2731a`）；這裡拿下一條鏈
	// 拍到的位置當那個答案。
	placeNew := func(snap *battle.Battle) {
		for _, u := range model.Units {
			if !u.Unplaced {
				continue
			}
			for _, v := range snap.Units {
				if v.Side == u.Side && v.Formation == u.Formation {
					u.At = v.At
					u.Unplaced = false
					t.Logf("中途生出的 %s%s 照原版擺在 %v", u.Side, u.Formation, u.At)
				}
			}
		}
	}
	// 原版讀鍵與等待的迴圈會不斷 `srand(計數器)`（見上面的 hook），
	// 玩家每按一個鍵種子就跳一次。remake 擲骰時照原版同一段的紀錄
	// （鏈內 callers ＋ 鏈外 gap）走：擲第 k 次之前，先把排在原版第 k
	// 次擲骰前面的 `srand=` 套上去。骰的**順序與次數**還是 remake 自己的，
	// 種子相不相同由下一條鏈進來時的比對決定。
	var stream []string
	cursor := 0
	isDraw := func(e string) bool { return e != "" && e[0] >= '0' && e[0] <= '9' }
	applySrands := func() {
		for cursor < len(stream) && !isDraw(stream[cursor]) {
			if strings.HasPrefix(stream[cursor], "srand=") {
				var v uint32
				fmt.Sscanf(stream[cursor], "srand=%d@", &v)
				seed = v
			}
			cursor++
		}
	}
	var asked []string
	model.PlayerDuelAnswer = func(*battle.Skirmish, *battle.SkirmishGeneral, *battle.SkirmishGeneral) bool {
		applySrands()
		if cursor < len(stream) {
			return duelAnswerFrom(stream[cursor], rig.braveRet)
		}
		return true
	}
	model.UseRoll(func(n int) int {
		applySrands()
		if cursor < len(stream) {
			cursor++
		}
		var out int
		seed, out = game.MSCRand(seed)
		asked = append(asked, fmt.Sprintf("%d=%d", n, out%n))
		return out % n
	})
	findUnit := func(b *battle.Battle, army, team int) *battle.Unit {
		for _, u := range b.Units {
			if u.Side == toSide[army] && u.Formation == teamForm[team] {
				return u
			}
		}
		return nil
	}
	unitState := func(u *battle.Unit) string {
		if u == nil {
			return "（沒有這支）"
		}
		x, y := battle.ToOffset(u.At)
		return fmt.Sprintf("兵 %d 將 %d 落點 (%d,%d) 移動 %d 箭 %d 陷阱 %d 綜合 %d 在場 %v",
			u.Soldiers(), u.LeaderCount(), x, y, u.Move, u.Arrows, u.Trapped, u.Quality, u.Alive())
	}
	// leaderRoster 是一支部隊逐槽的「人物槽號:兵」（火攻、水淹逐將領扣兵
	// 與除名，只比部隊的合計看不出是哪一位掉了兵）。
	leaderRoster := func(u *battle.Unit) string {
		var parts []string
		for i := range u.Leaders {
			if l := &u.Leaders[i]; l.InUnit() {
				parts = append(parts, fmt.Sprintf("%d:%d", l.Index, l.Soldiers))
			}
		}
		return fmt.Sprint(parts)
	}
	layout := func(b *battle.Battle) string {
		var parts []string
		for _, u := range b.Units {
			x, y := battle.ToOffset(u.At)
			var idx []int
			for i := range u.Leaders {
				if u.Leaders[i].InUnit() {
					idx = append(idx, u.Leaders[i].Index)
				}
			}
			parts = append(parts, fmt.Sprintf("%s%s@(%d,%d)兵%d將%v移%d", u.Side, u.Formation, x, y, u.Soldiers(), idx, u.Move))
		}
		return fmt.Sprint(parts)
	}
	t.Logf("不重拍：起點 第 %d 天 種子 %08x 統帥 %v 天候 %v；%s", first.day, seed, model.Commander, model.Weather, layout(model))
	diverged, stateBad, decideBad := 0, 0, 0
	for i, d := range decisions {
		// 上一段尾巴的 `srand=`（等鍵的迴圈在下一條鏈之前又播了種）先套上。
		applySrands()
		stream, cursor = append(append([]string(nil), d.callers...), d.gap...), 0
		tag := fmt.Sprintf("第 %2d 天 %s 隊伍 %d", d.day, sideName(d.army), d.team)
		if seed != d.seed {
			steps := lcgStepsBetween(seed, d.seed, 64)
			t.Errorf("✗ %s：進鏈時骰岔開——remake 種子 %08x、原版 %08x（原版比 remake 多 %d 步）；上一條鏈之後原版鏈外的骰 %v，remake 問了 %v",
				tag, seed, d.seed, steps, compactDraws(decisions[i-1].gap), asked)
			diverged++
			seed = d.seed
		}
		asked = nil
		placeNew(d.model)
		u := findUnit(model, d.army, d.team)
		if u == nil || d.entry == nil {
			t.Errorf("✗ %s：remake 找不到這支部隊（原版 %s）", tag, unitState(d.entry))
			stateBad++
			continue
		}
		model.RefreshQuality(u)
		if got, want := unitState(u), unitState(d.entry); got != want {
			t.Errorf("✗ %s：進鏈時部隊狀態不同——remake %s；原版 %s", tag, got, want)
			stateBad++
		}
		// **被打的那一邊也要比**：火攻、水淹、圍攻、快戰的殺傷落在別支
		// 部隊上，只比決策的那一支看不到。原版那一刻的其餘每一支
		// （d.others 是進鏈時拍的複本；d.model 裡的那幾支已經被重拍那一段
		// 的 DecideBase 打過）逐支對 remake 的兵、將領數、落點。
		for _, ou := range d.others {
			ru := findUnit(model, int(ou.Side.OriginalIndex()), int(ou.Formation.OriginalIndex()))
			if ru == nil {
				if ou.Alive() {
					t.Errorf("✗ %s：remake 少了 %s%s（原版 %s）", tag, ou.Side, ou.Formation, unitState(ou))
					stateBad++
				}
				continue
			}
			if ru.Soldiers() != ou.Soldiers() || ru.LeaderCount() != ou.LeaderCount() || ru.At != ou.At ||
				leaderRoster(ru) != leaderRoster(ou) {
				t.Errorf("✗ %s：%s%s 進鏈時不同——remake %s %s；原版 %s %s", tag, ou.Side, ou.Formation,
					unitState(ru), leaderRoster(ru), unitState(ou), leaderRoster(ou))
				stateBad++
			}
		}
		got := model.DecideBase(u)
		model.EndTurn(u)
		if i+1 < len(decisions) && decisions[i+1].day != d.day {
			// 換日：原版在最後一支電腦部隊之後輪到玩家那支（休息、
			// 判投敵、回填），再做日結算與天候；remake 在這裡做同一串。
			next := decisions[i+1]
			for day := d.day; day < next.day; day++ {
				if day == first.day && placedAt != battle.NoHex {
					if p := findUnit(model, 2, 0); p != nil {
						p.At = placedAt
					}
					// 玩家那支搬過去的同一個時點，那一格淺水也是那時改的。
					if shallowAt != battle.NoHex {
						model.Field.Set(shallowAt, battle.Shallow)
					}
				}
				placeNew(next.model)
				for _, p := range playerUnits() {
					model.RefreshQuality(p)
					if p.Trapped > 0 {
						model.SkipTrappedTurn(p)
						continue
					}
					_ = model.Rest(p)
					model.EndTurn(p)
				}
				model.EndDay()
			}
			if wx := forcedWeather(next.day); wx >= 0 {
				// 原版那一邊在每天第一條鏈之前把天候直寫掉了，remake 跟著設
				// （每天重擲那一擲兩邊都擲過了，只是值被蓋掉）。
				model.Weather = [...]battle.Weather{battle.Clear, battle.Rainy, battle.Windy}[wx%3]
			}
			applySrands()
			if model.Weather != next.model.Weather {
				t.Errorf("第 %d 天：remake 擲出的天候是 %v，原版 %v", next.day, model.Weather, next.model.Weather)
				model.Weather = next.model.Weather
			}
		}
		ok := got.Option == d.option
		want := fmt.Sprintf("選項 %d", d.option)
		have := fmt.Sprintf("選項 %d", got.Option)
		if d.option >= 4 && d.option <= 8 {
			want += fmt.Sprintf(" 對 %s 隊伍 %d", sideName(d.tArmy), d.tTeam)
			if got.Target != nil {
				have += fmt.Sprintf(" 對 %s %s", got.Target.Side, got.Target.Formation)
				if got.Target.Side != toSide[d.tArmy] || got.Target.Formation != teamForm[d.tTeam] {
					ok = false
				}
			} else {
				ok = false
			}
		}
		x, y := battle.ToOffset(u.At)
		// 退了兵的部隊原版會走到出口才消失，記錄裡留的是那一格；remake
		// 不走那段路，落點不比。
		if d.option != 2 && (x != d.col || y != d.row) {
			ok = false
		}
		want += fmt.Sprintf(" 落點 (%d,%d)", d.col, d.row)
		have += fmt.Sprintf(" 落點 (%d,%d)", x, y)
		if ok {
			t.Logf("✓ %s：%s；種子 %08x；鏈內骰 %v；鏈外骰 %v；remake 問了 %v", tag, want, d.seed, compactDraws(d.callers), compactDraws(d.gap), asked)
		} else {
			t.Errorf("✗ %s：原版 %s；remake %s（%v）；鏈內骰 %v；鏈外骰 %v；remake 鏈內問了 %v；remake 盤面 %s；原版盤面 %s",
				tag, want, have, got.Why, compactDraws(d.callers), compactDraws(d.gap), asked, layout(model), layout(d.model))
			decideBad++
		}
		asked = nil
	}
	t.Logf("不重拍：%d 條鏈，骰岔開 %d 次、狀態不同 %d 次、決策不同 %d 次", len(decisions), diverged, stateBad, decideBad)
	return byOpt
}

// checkLureFlash 攔誘敵常式（`0x2b6aa`）：讀施法者那一支的格子，每一聲
// `speak`（`0x5b80`，呼叫端在特效那一段）時比那一格與 `ui.LureFlashSteps`
// 那一步的圖塊逐格相同、速度相同（Issue #63）。回傳比完幾次、第一個錯誤。
func checkLureFlash(t *testing.T, o *oracle.Oracle, rig dayRig, root string) (*int, *string) {
	done, bad := new(int), new(string)
	if rig.lure == 0 {
		return done, bad
	}
	tiles, err := assets.BattleTiles(openContainer(t, filepath.Join(root, "DATA1")))
	if err != nil {
		t.Fatal(err)
	}
	steps := ui.LureFlashSteps()
	var at battle.Hex
	step := -1
	o.OnCall(addr(rig.lure), func(oo *oracle.Oracle) {
		ds := uint32(oo.DSReg()) * 16
		seg := uint32(oo.Word(addr(ds + 0xa984)))
		rec := seg*16 + uint32(0x3502+(int(oo.Arg(0))*10+int(oo.Arg(1)))*42)
		at = battle.FromOffset(int(int16(oo.Word(addr(rec+22)))), int(int16(oo.Word(addr(rec+24)))))
		step = 0
	})
	o.OnCall(addr(0x5b80), func(oo *oracle.Oracle) {
		if step < 0 || *bad != "" {
			return
		}
		if c := oo.Caller().Linear(); c < 0x2b783 || c >= 0x2b7f6 {
			return
		}
		if step >= len(steps) {
			*bad = fmt.Sprintf("原版第 %d 步還在閃，remake 只有 %d 步", step+1, len(steps))
			return
		}
		want := steps[step]
		if got := int(oo.Arg(1)); got != want.Speed {
			*bad = fmt.Sprintf("第 %d 步速度原版 %d remake %d", step, got, want.Speed)
			return
		}
		x, y, w, h := ui.LureFlashCell(at)
		scr := oo.IndexedEGASize(scrW, scrH)
		im := tiles[want.Tile]
		for yy := 0; yy < h; yy++ {
			for xx := 0; xx < w; xx++ {
				if g, r := scr[(y+yy)*scrW+x+xx]&15, im.Pix[yy*im.W+xx]&15; g != r {
					*bad = fmt.Sprintf("第 %d 步 (%d,%d) 那一格的 (%d,%d) 原版 %d remake 圖塊 %d 是 %d", step, x, y, xx, yy, g, want.Tile, r)
					return
				}
			}
		}
		step++
		if step == len(steps) {
			*done++
			step = -1
		}
	})
	return done, bad
}
