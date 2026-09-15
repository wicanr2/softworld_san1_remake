package session

import (
	"strings"
	"testing"

	"github.com/wicanr2/softworld_san1_remake/internal/ai"
	"github.com/wicanr2/softworld_san1_remake/internal/game"
	"github.com/wicanr2/softworld_san1_remake/internal/state"
)

// TestGameReachesAConclusion 讓全部十四個勢力都由強化 AI 操盤，跑到分出勝負。
//
// **這是「這個 remake 跑不跑得完一局」的判準。** 個別規則各自正確，
// 不代表接起來會收斂——經濟可能通膨、戰爭可能永遠打不下城、
// 勢力可能永遠不死。那些只有整局跑過才看得出來。
func TestGameReachesAConclusion(t *testing.T) {
	s := newSession(t, ai.ModeEnhanced, state.NoFaction) // 沒有玩家，全電腦
	// **年數是量出來的，不是挑的。** 訓練度、武裝度與土地價值都會逐年
	// 自然衰減（`game.AnnualDecay`，`L0`），所以軍隊不會一路變強，
	// 統一因此慢下來——六十年只收斂到剩兩家。這是規則本身的結果，
	// 不是 AI 不會打。
	const maxYears = 90
	start := len(s.G.Factions())
	alive := start
	for i := 0; i < maxYears*12; i++ {
		s.EndMonth()
		// **持郡才算存活**：`Faction.Alive` 是絕嗣旗標（君主欄非空），
		// 沒了領地、君主還在的勢力原版照樣算活著（`docs/mechanics/80` §1.1）；
		// 這裡要看的是局面收斂，判準與統一判定一樣看郡。
		alive = 0
		for _, f := range s.G.Factions() {
			if len(s.G.Territory(f.ID)) > 0 {
				alive++
			}
		}
		if alive <= 1 {
			break
		}
	}
	t.Logf("跑到 %d 年 %d 月，剩下 %d/%d 個勢力",
		s.G.Date.Year, s.G.Date.Month, alive, start)
	if alive >= start {
		t.Errorf("跑了 %d 年一個勢力都沒被消滅——戰爭沒有推進局面", maxYears)
	}
	if alive > 1 {
		t.Errorf("跑了 %d 年還剩 %d 個勢力——局面沒有收斂", maxYears, alive)
	}
}

// TestEconomyStaysSane 釘住經濟不會炸掉也不會歸零。
//
// 上限是手冊給的（金米各 30000，p.22）；下限是常識——
// **一個所有郡都零人口的世界不是「難度高」，是規則接錯了**。
func TestEconomyStaysSane(t *testing.T) {
	s := newSession(t, ai.ModeEnhanced, state.NoFaction)
	for i := 0; i < 20*12; i++ {
		s.EndMonth()
	}
	pop, gold := 0, 0
	for _, p := range s.G.Prefectures() {
		if p.Gold > 30000 || p.Rice > 30000 {
			t.Errorf("%s 的金 %d 米 %d 超過上限", p.Name, p.Gold, p.Rice)
		}
		if p.Gold < 0 || p.Rice < 0 || p.Population < 0 {
			t.Errorf("%s 出現負值：金 %d 米 %d 人口 %d",
				p.Name, p.Gold, p.Rice, p.Population)
		}
		pop += p.Population
		gold += p.Gold
	}
	// **世界不該崩潰。** 下界是 remake 自己的護欄：沒有它的話，災害與
	// 徵兵會把人口打到剩百分之七，而每一條規則單獨看都「照手冊做」，
	// 只有整局跑過才看得出來。
	//
	// **上界用原版的上限，不用倍數。** 成長率是量到的
	// （`game.GrowPopulation`，倍率跟著土地價值與忠誠走，最快 15%），
	// 所以人口本來就會往每郡 `PopulationCap` 爬——拿「不得超過開局的
	// 三倍」當上界，等於拿 remake 早期猜的災害頻率當基準，那個基準
	// 已經被量到的判定取代了。
	const start = 2538000 // 劇本 001 的開局總人口
	if pop < start/2 {
		t.Errorf("二十年後總人口 %d，開局是 %d——世界崩潰了", pop, start)
	}
	if cap := game.PopulationCap * state.PrefectureCount; pop > cap {
		t.Errorf("二十年後總人口 %d，超過每郡上限的總和 %d", pop, cap)
	}
	t.Logf("二十年後：總人口 %d（開局 %d）、總庫銀 %d", pop, start, gold)
}

// TestTroopCapNeverExceeded 釘住整局跑下來沒有人超過帶兵上限。
//
// 上限是**兩個獨立來源同意的數字**，所以它是最硬的不變式之一。
// 任何一條規則（徵兵、重編、戰役、繼承）算錯都會在這裡露餡。
func TestTroopCapNeverExceeded(t *testing.T) {
	s := newSession(t, ai.ModeEnhanced, state.NoFaction)
	for i := 0; i < 10*12; i++ {
		s.EndMonth()
		for _, p := range s.G.Prefectures() {
			for _, x := range s.G.Garrison(p.ID) {
				if x.Soldiers > x.TroopCap() {
					t.Fatalf("%d 年 %d 月：%s 帶 %d 兵，上限 %d",
						s.G.Date.Year, s.G.Date.Month, x.Name, x.Soldiers, x.TroopCap())
				}
			}
		}
	}
}

// TestEveryOwnedPrefectureHasAGovernor 釘住整局跑下來
// 「每個有主的郡都有主事者」——這條在開局是資料保證的，
// 之後要靠規則維持（移防、戰役、老死都可能打破它）。
func TestEveryOwnedPrefectureHasAGovernor(t *testing.T) {
	s := newSession(t, ai.ModeEnhanced, state.NoFaction)
	for i := 0; i < 10*12; i++ {
		s.EndMonth()
		for _, p := range s.G.Prefectures() {
			if !p.Owned() {
				continue
			}
			if s.G.Governor(p.ID) == nil {
				t.Fatalf("%d 年 %d 月：%s 有主（勢力 %d）卻沒有主事者",
					s.G.Date.Year, s.G.Date.Month, p.Name, p.Owner)
			}
		}
	}
}

// TestSoldierTotalsMatch 釘住「郡的總兵力 ＝ 駐軍加總」。
//
// 手冊說總兵力是「所有現役將麾下的兵力總合」（p.17），所以它是導出值。
// 存一份副本就會有兩個真相——這一條抓的就是它們分家。
func TestSoldierTotalsMatch(t *testing.T) {
	s := newSession(t, ai.ModeEnhanced, state.NoFaction)
	for i := 0; i < 5*12; i++ {
		s.EndMonth()
	}
	for _, p := range s.G.Prefectures() {
		sum := 0
		for _, x := range s.G.Garrison(p.ID) {
			sum += x.Soldiers
		}
		if s.G.Soldiers(p.ID) != sum {
			t.Errorf("%s 的總兵力 %d，駐軍加總 %d", p.Name, s.G.Soldiers(p.ID), sum)
		}
	}
}

// TestLogMentionsWar 釘住戰爭真的有打起來。
func TestLogMentionsWar(t *testing.T) {
	s := newSession(t, ai.ModeEnhanced, state.NoFaction)
	s.MaxLog = 100000
	for i := 0; i < 5*12; i++ {
		s.EndMonth()
	}
	// **釘的是戰報，不是命令的描述。** `AttackOrder.Describe`（「某某出兵攻
	// 某某」）只會在命令**被擋下**時隨錯誤訊息進紀錄——拿它當判準等於在
	// 釘一個 bug 的症狀：命令改成套得上去之後，那一行就消失了。
	// 戰役真的打起來的證據是戰報（`BattleResult.Summary`）。
	for _, line := range s.Log {
		if strings.Contains(line, "攻") && strings.Contains(line, "折損") {
			return
		}
	}
	t.Error("五年之內沒有任何一次出兵——AI 不會打仗")
}

// TestBattleReportReachesTheLog 釘住主戰場打出來的東西會被玩家看見。
//
// 命令層的 `Apply` 只回錯誤，電腦諸侯的戰役玩家更是完全沒經手——
// **戰報沒有被取走的話，一場三十天的戰役在紀錄裡只剩一行「出兵攻」**，
// 而那一行在功能正常與戰術層根本沒跑起來的時候長得一模一樣。
func TestBattleReportReachesTheLog(t *testing.T) {
	s := newSession(t, ai.ModeEnhanced, state.NoFaction)
	s.MaxLog = 100000
	for i := 0; i < 5*12 && len(s.Battles()) == 0; i++ {
		s.EndMonth()
	}
	if len(s.Battles()) == 0 {
		t.Fatal("五年之內一場戰役都沒打起來")
	}
	found := false
	for _, line := range s.Log {
		if strings.HasPrefix(line, "⚔") {
			found = true
			break
		}
	}
	if !found {
		t.Error("打過戰役，紀錄裡卻沒有任何一行戰報")
	}
	r := s.Battles()[0]
	if len(r.Log) == 0 {
		t.Error("留下來的戰役沒有逐日戰報")
	}
	if r.Days < 1 {
		t.Errorf("戰役打了 %d 天", r.Days)
	}
}

// TestBattleHistoryIsBounded 釘住逐日戰報不會無限累積。
//
// 一場三十天的主戰場動輒上百行；全部留著，跑完一局的記憶體會
// 跟局面本身一樣大。
func TestBattleHistoryIsBounded(t *testing.T) {
	s := newSession(t, ai.ModeEnhanced, state.NoFaction)
	for i := 0; i < 20*12; i++ {
		s.EndMonth()
	}
	if n := len(s.Battles()); n > MaxBattles {
		t.Errorf("留了 %d 場戰役的逐日戰報，上限是 %d", n, MaxBattles)
	}
}
