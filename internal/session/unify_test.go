package session

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/wicanr2/softworld_san1_remake/internal/ai"
	"github.com/wicanr2/softworld_san1_remake/internal/state"
)

// unifySeeds 是統一年份分布用的八顆種子（Issue #9）。第一顆是對拍一直在用的
// `0x13579BDF`（`TestZZSeedScan` 挑的），其餘七顆是它逐次加上一個奇數常數，
// 沒有挑過——分布要的是「隨便八顆」，不是「好看的八顆」。
var unifySeeds = []uint32{
	0x13579BDF, 0x2468ACE1, 0x3579BDF3, 0x468ACE05,
	0x579BDF17, 0x68ACE129, 0x79BDF13B, 0x8ACE124D,
}

// unifyResult 是 remake 全電腦跑一局的結果，欄位與原版那一側
// （`internal/parity` 的 `unifyRun`）對齊，方便並排。
type unifyResult struct {
	Seed       uint32 `json:"seed"`
	AI         string `json:"ai"`
	Difficulty int    `json:"difficulty"`
	StartYear  int    `json:"start_year"`
	Unified    bool   `json:"unified"`
	Year       int    `json:"year"`
	Month      int    `json:"month"`
	Winner     int    `json:"winner"`
	WinnerLord string `json:"winner_lord"`
	Months     int    `json:"months"`
	Alive      []int  `json:"alive"`    // 每年元月還有幾個勢力持郡（與原版那一側同一個判準）
	Employed   []int  `json:"employed"` // 每年元月在職的人物數
	Fallen     []int  `json:"fallen"`   // 每年元月已故（身分 12）的人物數
	// Status 是每年元月身分欄的分布（索引＝身分碼），與原版那一側同一格式。
	Status [][13]int `json:"status"`
	// Deaths 是每一種死法的次數（`game.State.DeathLog`）。
	Deaths map[string]int `json:"deaths"`
}

// TestUnifyYearDistribution 讓 remake 用原版對拍過的 AI（`ai.ModeBase`）、
// 原版的亂數（`SeedRand`）、難度 5、劇本一、沒有玩家，八顆種子各跑到統一，
// 記下年份與統一者。與原版示範模式的同一組數字並排在
// `docs/playtest/05-unify-year.md`；**只報分布，不設 pass/fail**——
// 相同種子不等於相同骰序，兩邊不可能逐局相同（Issue #9 的停止線）。
func TestUnifyYearDistribution(t *testing.T) {
	const maxYears = 150
	const difficulty = 5
	var results []unifyResult
	for _, seed := range unifySeeds {
		s := newSession(t, ai.ModeBase, state.NoFaction)
		s.G.SeedRand(seed)
		r := unifyResult{Seed: seed, AI: string(ai.ModeBase), Difficulty: difficulty,
			StartYear: s.G.Date.Year}
		lastYear := 0
		for i := 0; i < maxYears*12; i++ {
			s.EndMonth()
			r.Months++
			// **判準與原版那一側相同**：數「持有郡的勢力」，不看 `Faction.Alive`
			// ——那個旗標只在戰敗與君主死亡時更新，郡被搬空之後不會跟著變，
			// 拿它數會把沒有領地的殭屍算進去（`docs/playtest/05` §4）。
			owning := 0
			for _, f := range s.G.Factions() {
				if len(s.G.Territory(f.ID)) > 0 {
					owning++
				}
			}
			if s.G.Date.Year != lastYear {
				r.Alive = append(r.Alive, owning)
				emp, fallen := 0, 0
				var st [13]int
				for _, x := range s.G.AllGenerals() {
					if x.Employed() {
						emp++
					}
					if x.Status == state.StatusFallen {
						fallen++
					}
					if int(x.Status) < len(st) {
						st[x.Status]++
					}
				}
				r.Employed, r.Fallen = append(r.Employed, emp), append(r.Fallen, fallen)
				r.Status = append(r.Status, st)
				lastYear = s.G.Date.Year
			}
			// 統一判定用 remake 自己的 `Winner()`（`0x15852` 的條件：所有有主的
			// 郡同屬一方）。
			if w, done := s.G.Winner(); done {
				r.Unified, r.Year, r.Month, r.Winner = true, s.G.Date.Year, s.G.Date.Month, int(w)
				if f := s.G.Faction(w); f != nil {
					if g := s.G.General(f.Lord); g != nil {
						r.WinnerLord = g.Name
					}
				}
				break
			}
			if owning == 0 {
				break // 全部死光，沒有人統一
			}
		}
		if r.Unified {
			t.Logf("seed %#x：%d 年 %d 月由勢力 %d（%s）統一，%d 個月", seed, r.Year, r.Month,
				r.Winner, r.WinnerLord, r.Months)
		} else {
			t.Logf("seed %#x：%d 個月沒統一，還剩 %d 個勢力", seed, r.Months, r.Alive[len(r.Alive)-1])
		}
		r.Deaths = s.G.DeathLog
		results = append(results, r)
	}
	if dir := os.Getenv("SAN1_DUMP"); dir != "" {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			t.Fatal(err)
		}
		b, _ := json.MarshalIndent(results, "", "  ")
		path := filepath.Join(dir, "unify-remake.json")
		if err := os.WriteFile(path, b, 0o644); err != nil {
			t.Fatal(err)
		}
		t.Logf("結果寫到 %s", path)
	}
	// 正對照：八局裡至少要有一局收斂，否則是規則接錯了不是分布。
	n := 0
	for _, r := range results {
		if r.Unified {
			n++
		}
	}
	if n == 0 {
		t.Errorf("八顆種子在 %d 年內一局都沒統一", maxYears)
	}
}
