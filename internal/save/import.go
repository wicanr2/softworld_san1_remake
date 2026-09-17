package save

import (
	"fmt"

	"github.com/wicanr2/softworld_san1_remake/internal/assets"
	"github.com/wicanr2/softworld_san1_remake/internal/game"
	"github.com/wicanr2/softworld_san1_remake/internal/state"
)

// 讀玩家自己的原版進度。
//
// 原版把六個進度放在 `DATA2.GRP` 容器裡（`docs/formats/05`），一個進度
// 六個項目。三張主表 remake 早就解得開，`BASEPRO`／`BASEPRE`／名稱表
// 是後來補上的（`docs/re/08`），所以整份現在讀得出來。
//
// ⚠ **只讀不寫。** 要寫就得改玩家自己的原版檔案，那是別人的東西，
// 而且一旦寫壞沒有第二份。存檔一律進 remake 自己的目錄。

// OriginalSlots 是原版的進度數。
const OriginalSlots = Slots

// originalSlot 把 1..6 換成容器項目的後綴。
func originalSlot(slot int) state.Slot { return state.Slot(fmt.Sprintf("SV%d", slot)) }

// ListOriginal 列出原版容器裡六個進度的概況。
//
// **讀不出來的槽照樣列出來**，只是標成空的——原版的讀檔畫面就是六格，
// 少一格會讓人以為存檔掉了。
func ListOriginal(c *assets.Container) []Info {
	names, err := state.LoadSaveNames(c)
	if err != nil {
		names = make([]string, OriginalSlots)
	}
	out := make([]Info, 0, OriginalSlots)
	for slot := 1; slot <= OriginalSlots; slot++ {
		info := Info{Slot: slot}
		if p, err := state.LoadProgress(c, originalSlot(slot)); err == nil {
			info.Exists = true
			info.Year, info.Month = p.Year, p.Month
		}
		if slot-1 < len(names) {
			info.Name = names[slot-1]
		}
		out = append(out, info)
	}
	return out
}

// ReadOriginal 從原版的 `DATA2` 容器讀一個進度。
//
// ed 是這份容器屬於哪一版——**容器本身沒說**，兩版的 `DATA2.GRP`
// 等長不同容（`docs/mechanics/90`），所以要由呼叫端從素材目錄決定。
func ReadOriginal(c *assets.Container, slot int, ed state.Edition) (*game.State, error) {
	if slot < 1 || slot > OriginalSlots {
		return nil, fmt.Errorf("save: 原版進度 %d 越界（有 %d 個）", slot, OriginalSlots)
	}
	name := originalSlot(slot)
	sc, err := state.LoadScenario(c, name)
	if err != nil {
		return nil, fmt.Errorf("save: 原版進度 %d：%w", slot, err)
	}
	p, err := state.LoadProgress(c, name)
	if err != nil {
		return nil, fmt.Errorf("save: 原版進度 %d：%w", slot, err)
	}
	if p.Difficulty < 1 || p.Difficulty > ed.MaxDifficulty() {
		// **不夾也不猜。** 原版的難度係數表只有 `MaxDifficulty()` 格
		// （原版 11、加強版 21，`docs/spec/004`），超出去的值在原版
		// 自己那裡也沒有對應的係數。出貨的第 2 個進度就是 15。
		return nil, fmt.Errorf(
			"save: 原版進度 %d 的難度是 %d，%s 的係數表只到 %d——"+
				"這份進度不是這一版的輸入畫面產生的",
			slot, p.Difficulty, ed, ed.MaxDifficulty())
	}

	e := game.Extra{
		Player:   state.NoFaction,
		Edition:  ed,
		Factions: map[state.FactionID]game.FactionExtra{},
	}
	// 玩家是哪些勢力：諸侯表 offset 0 等於 1 的那幾個（`L1`，`docs/spec/019`）。
	// 玩家序號不在三張表裡，照勢力槽號排。
	for _, p := range sc.Players() {
		e.Players = append(e.Players, state.FactionID(p))
	}
	if len(e.Players) > 0 {
		e.Player = e.Players[0]
	}
	for i := 0; i < state.MasterTableSize/state.MasterRecordSize; i++ {
		ctrl := sc.Controller(i)
		fx := game.FactionExtra{
			Alive: ctrl != state.ControlledByNobody,
			Chief: sc.ChiefIndex(i),
		}
		t := sc.TreasuryOf(i)
		copy(fx.Treasury[:], t[:])
		e.Factions[state.FactionID(i)] = fx
	}
	// 郡的補充資料：長度要與局面一致，內容從 `BASEPRO` 來。
	// 精確人口留 0——那一格是 remake 自己加的，原版沒有，
	// 給 0 表示「以州郡表為準」。
	for i := 0; i < state.PrefectureCount; i++ {
		e.Prefectures = append(e.Prefectures, game.PrefectureExtra{})
	}
	if err := applyProgress(&e, p); err != nil {
		return nil, fmt.Errorf("save: 原版進度 %d：%w", slot, err)
	}
	g, err := game.Restore(sc, e)
	if err != nil {
		return nil, fmt.Errorf("save: 原版進度 %d：%w", slot, err)
	}
	// **載入之後郡的所屬要重算一次**（`0x1e394`）。原版讀完存檔的記憶體
	// 與檔案裡的 offset 30 不一定相同——量到的一例是進度 1 的郡 13：
	// 檔案存 5，原版載入之後是 4（`TestOriginalSaveLoadsIdentically`）。
	// 所屬是從人物表推出來的，存檔裡那一格是寫檔當下的快照。
	g.RecomputeOwners()
	// 讀檔後原版立刻從 BASEPRO 的游標接回月內迴圈。若游標正停在玩家郡，
	// `0x17471` 會先呼叫 `0x1949e` 重整守將清單，然後才停進主命令；所以
	// 畫面出現時該郡的兵士與現役將已刷新。只在這個可證實的玩家停點套用：
	// 若游標指向電腦郡，原版還會跑完整分派器，不能只偷刷兩個欄位冒充。
	if p.Cursor >= 0 && p.Cursor < len(p.Order) {
		at := p.Order[p.Cursor]
		if at > 0 && at < len(p.Pending) && p.Pending[at] {
			if q := g.Prefecture(at); q != nil && g.IsHuman(q.Owner) {
				g.RefreshGarrison(at)
			}
		}
	}
	return g, nil
}
