package game

import (
	"fmt"

	"github.com/wicanr2/softworld_san1_remake/internal/state"
)

// 從存檔還原一局。
//
// 三張表（`tables.go`）裝得下原版存的東西，但**裝不下 remake 自己加的**：
// 年月、玩家是誰、難度、城寨數、郡縣自治、這個月下過令沒、賞賜過沒、
// 君主寶庫。那些走 Extra，由 `internal/save` 存成一份 JSON。
//
// ⚠ **不要把 Extra 的東西「猜回來」。** 例如城寨數猜成 0、
// 自治猜成正常——讀檔之後畫面一切正常，只是玩家蓋的關寨不見了。

// Extra 是三張表放不下的局面狀態。
type Extra struct {
	Year, Month int
	Player      state.FactionID
	Difficulty  int

	// Prefectures 依郡編號 1..42，索引 0 對應郡 1。
	Prefectures []PrefectureExtra

	// Rewarded 是這個月已經受賞的人物槽號。
	Rewarded []int

	// Factions 依勢力槽號索引。
	Factions map[state.FactionID]FactionExtra

	// Options 是「其他」底下的開關。存了才不會每次讀檔都要重設一遍。
	Options Options
}

// PrefectureExtra 是一個郡在三張表以外的狀態。
type PrefectureExtra struct {
	Forts     int
	Autonomy  Autonomy
	Commanded bool

	// Population 是**精確**人口。
	//
	// 原版的州郡表把人口存成實際值 ÷ 100（格式字串是 `人口 %5d00`），
	// 所以那一欄只放得下百位。remake 的四季事件用百分比縮放人口，
	// 算出來的數字不是百的倍數；只靠原版那一欄的話，存讀一輪就會
	// 掉最多 99 人，而**每存一次就再掉一次**。
	//
	// 三張表照原版的版面寫（對拍要用），精確值放這裡。
	Population int
}

// FactionExtra 是一個勢力在三張表以外的狀態。
type FactionExtra struct {
	Alive    bool
	Chief    int
	Treasury [treasureCount]int
}

// CaptureExtra 把目前局面裡三張表放不下的部分抄出來。
func (g *State) CaptureExtra() Extra {
	e := Extra{
		Year: g.Date.Year, Month: g.Date.Month,
		Player: g.Player, Difficulty: g.Difficulty,
		Options:  g.Options,
		Factions: map[state.FactionID]FactionExtra{},
	}
	for i := range g.prefectures {
		p := &g.prefectures[i]
		e.Prefectures = append(e.Prefectures, PrefectureExtra{
			Forts: p.Forts, Autonomy: p.Autonomy, Commanded: p.Commanded,
			Population: p.Population,
		})
	}
	for i := range g.generals {
		if g.generals[i].Rewarded {
			e.Rewarded = append(e.Rewarded, g.generals[i].Index)
		}
	}
	for _, f := range g.factions {
		e.Factions[f.ID] = FactionExtra{
			Alive: f.Alive, Chief: f.Chief, Treasury: f.Treasury,
		}
	}
	return e
}

// Restore 從三張表加上 Extra 還原一局。
func Restore(sc *state.Scenario, e Extra) (*State, error) {
	if e.Month < 1 || e.Month > 12 {
		return nil, fmt.Errorf("game: 存檔的月份是 %d", e.Month)
	}
	if e.Difficulty < 1 || e.Difficulty > 10 {
		return nil, fmt.Errorf("game: 存檔的難度是 %d", e.Difficulty)
	}
	// 借 New 把三張表解出來。難度先給合法值，年月與玩家馬上蓋掉——
	// New 會擋「玩家控制的勢力沒在用」，而存檔裡的玩家可能已經被消滅，
	// 那不是錯誤，是輸掉了。
	g, err := New(sc, state.NoFaction, e.Difficulty)
	if err != nil {
		return nil, err
	}
	g.Date = Date{Year: e.Year, Month: e.Month}
	g.Player = e.Player
	g.Options = e.Options
	// `New` 是拿 `NoFaction` 叫的，所以它把每一個勢力都標成電腦。
	// 玩家蓋回去之後要重算——**這個旗標會改規則**（「每郡每月一道令」
	// 只擋玩家），漏掉的話讀檔之後玩家就能一個月下九道令。
	for i := range g.factions {
		g.factions[i].ByComputer = g.factions[i].ID != g.Player
	}

	if n := len(e.Prefectures); n != len(g.prefectures) {
		return nil, fmt.Errorf("game: 存檔有 %d 個郡的補充資料，這一局有 %d 個",
			n, len(g.prefectures))
	}
	for i := range g.prefectures {
		p := &g.prefectures[i]
		x := e.Prefectures[i]
		p.Forts, p.Autonomy, p.Commanded = x.Forts, x.Autonomy, x.Commanded
		if x.Population > 0 {
			p.Population = x.Population
		}
	}
	for _, idx := range e.Rewarded {
		if x := g.General(idx); x != nil {
			x.Rewarded = true
		}
	}
	for i := range g.factions {
		f := &g.factions[i]
		x, ok := e.Factions[f.ID]
		if !ok {
			// 存檔沒提到這個勢力：那是開局時在用、存檔時已經被消滅的。
			f.Alive = false
			continue
		}
		f.Alive, f.Chief, f.Treasury = x.Alive, x.Chief, x.Treasury
	}
	return g, nil
}
