package battle

import (
	"strings"
	"testing"
)

// 自動作戰。玩家不看的戰役也要打得完，而且要打得出同一個結果。

// armies 造一批將領。
//
// **兩邊的 Index 不能撞號**：真的人物槽號是全局唯一的，撞號的話
// 「俘虜名單有沒有重複」這種以 Index 為鍵的檢查會誤報。守方從 100 起。
func armies(n int, war, intel uint8, soldiers int, prefix string) []Leader {
	base := 0
	if prefix != "攻" {
		base = 100
	}
	var out []Leader
	for i := 0; i < n; i++ {
		l := lead(prefix, war-uint8(i), intel, soldiers)
		l.Index = base + i
		out = append(out, l)
	}
	return out
}

func setup(seed uint32) Setup {
	return Setup{
		Field:        Generate(params(9)),
		Weather:      Windy,
		Seed:         seed,
		Attackers:    armies(8, 90, 70, 3000, "攻"),
		Defenders:    armies(6, 80, 85, 2500, "守"),
		FromGate:     10,
		AttackerGold: 3000, AttackerRice: 8000,
		DefenderGold: 2000, DefenderRice: 6000,
	}
}

// TestAutoFinishes 釘住自動作戰一定會結束，而且不超過卅天。
//
// **一場打不完的戰役會讓整局卡住**，而且是在無頭跑的時候卡住，
// 沒有畫面可看。所以這一條要有測試盯著。
func TestAutoFinishes(t *testing.T) {
	b := New(setup(12345))
	days := b.Auto()
	if !b.Over {
		t.Error("自動作戰結束了卻沒有分勝負")
	}
	if days > BattleDays+1 {
		t.Errorf("打了 %d 天，上限是 %d 天", days, BattleDays)
	}
	if len(b.Log) == 0 {
		t.Error("打完一場卻沒有任何戰報")
	}
}

// TestAutoIsDeterministic 釘住同一場戰役重跑得到同一個結果。
func TestAutoIsDeterministic(t *testing.T) {
	b1, b2 := New(setup(999)), New(setup(999))
	d1, d2 := b1.Auto(), b2.Auto()
	if d1 != d2 || b1.AttackerWon != b2.AttackerWon {
		t.Fatalf("重跑得到不同結果：%d 天 %v vs %d 天 %v",
			d1, b1.AttackerWon, d2, b2.AttackerWon)
	}
	if strings.Join(b1.Log, "\n") != strings.Join(b2.Log, "\n") {
		t.Error("兩次的戰報不一樣")
	}
}

// TestAutoAcrossManyBattles 釘住各種郡、各種天氣、各種兵力都打得完。
//
// 一張把城池封死的地圖、一個永遠選不出動作的狀態，都會讓 Auto 空轉；
// 掃過四十二個郡才問得到「有沒有哪一張地圖會卡住」。
func TestAutoAcrossManyBattles(t *testing.T) {
	attackerWins := 0
	for id := 1; id <= 42; id++ {
		for _, w := range []Weather{Clear, Windy, Rainy} {
			s := setup(uint32(id*7 + int(w)))
			s.Field = Generate(params(id))
			s.Weather = w
			s.FromGate = params(id).Neighbours[0]
			b := New(s)
			days := b.Auto()
			if !b.Over {
				t.Fatalf("第 %d 郡（%s）打了 %d 天沒有結束", id, w, days)
			}
			if days < 1 {
				t.Fatalf("第 %d 郡打了 %d 天", id, days)
			}
			if b.AttackerWon {
				attackerWins++
			}
		}
	}
	// 兩邊都要贏得到。一面倒表示某一方的自動作戰其實沒在動。
	total := 42 * 3
	if attackerWins == 0 {
		t.Error("跑了 126 場攻方一場都沒贏——攻方的自動作戰可能沒在動")
	}
	if attackerWins == total {
		t.Error("跑了 126 場守方一場都沒守住——守方的自動作戰可能沒在動")
	}
	t.Logf("126 場裡攻方贏 %d 場", attackerWins)
}

// TestAutoRespectsFormationOrder 釘住自動作戰照手冊 p.28 的順序輪到每一支部隊。
func TestAutoRespectsFormationOrder(t *testing.T) {
	b := New(setup(4242))
	order := b.Order()
	if len(order) == 0 {
		t.Fatal("開場就沒有部隊")
	}
	// 主守軍排在主攻軍前面。
	firstAtt, firstDef := -1, -1
	for i, u := range order {
		if firstDef < 0 && u.Side == MainDefender {
			firstDef = i
		}
		if firstAtt < 0 && u.Side == MainAttacker {
			firstAtt = i
		}
	}
	if firstDef < 0 || firstAtt < 0 {
		t.Fatal("開場少了一方")
	}
	if firstDef > firstAtt {
		t.Error("主守軍應該排在主攻軍前面（說明書 p.28）")
	}
}

// TestCaptivesAndSurvivors 釘住決勝之後點得出被擒與生還的人（說明書 p.35）。
func TestCaptivesAndSurvivors(t *testing.T) {
	b := New(setup(777))
	b.Auto()
	seen := map[int]bool{}
	for _, l := range b.Captives() {
		if seen[l.Index] {
			t.Errorf("%s 被點名兩次", l.Name)
		}
		seen[l.Index] = true
		if !l.Captured {
			t.Errorf("%s 出現在俘虜名單卻沒有被擒", l.Name)
		}
	}
	for _, side := range []bool{true, false} {
		for _, l := range b.Survivors(side) {
			if l.Dead || l.Captured {
				t.Errorf("%s 出現在生還名單卻已陣亡或被擒", l.Name)
			}
		}
	}
}

// TestDefenceIsWorthSomething 釘住守方的地利真的擋得住。
//
// 主戰場的所有加成——城池的防禦、關寨、地形、兵種適性——最後只表現在
// 一件事上：**兵力相當的攻方打不下郡**。如果打贏只看兵多，
// 上面那一整套地形與計謀就等於沒接上。
//
// 這裡量的是攻守兵力比與勝率的關係，不是某一場的結果。
func TestDefenceIsWorthSomething(t *testing.T) {
	winRate := func(ratio float64) int {
		wins := 0
		for id := 1; id <= 42; id++ {
			for _, w := range []Weather{Clear, Windy, Rainy} {
				s := Setup{
					Field:        Generate(params(id)),
					Weather:      w,
					Seed:         uint32(id*7 + int(w)),
					Attackers:    armies(6, 80, 70, int(2500*ratio), "攻"),
					Defenders:    armies(6, 80, 70, 2500, "守"),
					FromGate:     params(id).Neighbours[0],
					AttackerGold: 3000, AttackerRice: 20000,
					DefenderGold: 3000, DefenderRice: 20000,
				}
				if b := New(s); func() bool { b.Auto(); return b.AttackerWon }() {
					wins++
				}
			}
		}
		return wins
	}
	weak, even, strong := winRate(0.7), winRate(1.0), winRate(1.5)
	t.Logf("攻方勝場：兵力 0.7 倍 %d／等量 %d／1.5 倍 %d（各 126 場）", weak, even, strong)
	if weak > 126/4 {
		t.Errorf("兵力只有守方 0.7 倍卻贏了 %d 場——守方的地利沒有作用", weak)
	}
	// 門檻是 2/3 不是 3/4：換成量到的傷亡模型之後（雙方同時算、
	// 地形對守方的保護走守方自己那一份殺傷），城池的守值 40 對攻值 20
	// 讓守方明顯變硬。**要看的是單調性與幅度**——0.7 倍全敗、等量
	// 三成、1.5 倍七成——不是某一個百分比。
	if strong < 126*2/3 {
		t.Errorf("兵力 1.5 倍只贏 %d 場——兵力優勢應該打得下郡", strong)
	}
	if even == 0 || even == 126 {
		t.Errorf("兵力相當時攻方贏 %d 場——勝負應該由地形與用計決定，不是一面倒", even)
	}
}

// TestAutoUsesTheWholeRepertoire 釘住自動作戰真的用得到每一種手段。
//
// **一個永遠只會「走過去砍」的 AI 會讓整個戰術層看起來正常。**
// 單挑、計謀、弓箭、退兵各自都有測試證明「叫得動」，但沒有人叫
// 就等於沒接上——這一條掃過一批戰役，數戰報裡出現過哪些字。
func TestAutoUsesTheWholeRepertoire(t *testing.T) {
	seen := map[string]int{}
	watch := []string{"單挑", "撤退", "射了", "火攻", "水淹", "誘敵", "陷阱", "燒", "圍攻", "全滅"}
	for id := 1; id <= 42; id++ {
		for _, w := range []Weather{Clear, Windy, Rainy} {
			for _, c := range []struct {
				ratio float64
				war   uint8
			}{{0.8, 90}, {1.0, 90}, {1.4, 90}, {0.8, 99}} {
				s := setup(uint32(id*31 + int(w)))
				s.Field = Generate(params(id))
				s.Weather = w
				s.FromGate = params(id).Neighbours[0]
				// 最後一組是「猛將帶寡兵」：正面打不贏但戰力壓過對方，
				// 手冊說單挑「與率領軍力大小無關」，就是為這種局面設的。
				s.Attackers = armies(8, c.war, 85, int(3000*c.ratio), "攻")
				s.Defenders = armies(6, 60, 85, 2500, "守")
				b := New(s)
				b.Auto()
				for _, line := range b.Log {
					for _, k := range watch {
						if strings.Contains(line, k) {
							seen[k]++
						}
					}
				}
			}
		}
	}
	t.Logf("戰報裡出現的次數：%v", seen)
	for _, k := range watch {
		if seen[k] == 0 {
			t.Errorf("跑了一批戰役，戰報裡一次都沒出現「%s」——這一招沒被用到", k)
		}
	}
}
