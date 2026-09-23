package save

import (
	"fmt"

	"github.com/wicanr2/softworld_san1_remake/internal/game"
	"github.com/wicanr2/softworld_san1_remake/internal/state"
)

// 原版存檔的第四、五、六個項目（`docs/formats/05`）。
//
// `BASEPRO.SVn` 裝的是年月、難度、選項、這個月走到哪一個郡；
// `BASEPRE.SVn` 裝自創君主的字模；`SAVENAME.SVP` 裝六個進度的名稱。
// 三張主表以外的局面狀態原本全在 `REMAKE.JSON` 裡，解出這三個之後
// 就搬回原版的版面——**能照原版存的就不要自己發明格式**，那樣才比得了。
//
// 搬不回去的還留在 JSON：精確人口、受賞名單、君主寶庫、現任軍師、
// 版本（原版沒有加強版這回事）。

// buildProgress 把一局的狀態填進 `BASEPRO` 的版面。
//
// prev 是這個槽上一次寫出去的內容，用來保住兩樣東西：還沒解出用途的
// 位元組，以及**郡的處理順序**。順序是原版每個月洗出來的狀態，
// remake 沒有它，寫成 0..42 的恆等排列即可——但既然上一次寫過就照抄，
// 存讀一輪不要無故變動別人的資料。
func buildProgress(e game.Extra, prev *state.Progress) *state.Progress {
	p := &state.Progress{}
	if prev != nil {
		*p = *prev
	} else {
		for i := range p.Order {
			p.Order[i] = i
		}
	}
	p.Year, p.Month = e.Year, e.Month
	p.Difficulty = e.Difficulty
	p.InMonth = true
	p.MusicOff = e.Options.MusicOff
	p.SoundOff = e.Options.SoundOff
	p.VoiceOff = e.Options.VoiceOff
	p.SkipAIWar = e.Options.SkipAIWar
	p.Delay = e.Options.Delay()
	p.Calendar = int(e.Options.Calendar)

	// Pending 的索引是**郡號**，而 Extra.Prefectures 的索引 0 對應郡 1。
	// 差一格的話讀回來每個郡的「這個月下過令沒」會整批位移一格。
	p.Pending[0] = false
	for i, pe := range e.Prefectures {
		if id := i + 1; id < len(p.Pending) {
			p.Pending[id] = !pe.Commanded
		}
	}
	// 游標要與旗標一致：原版的月內迴圈拿它當進度。remake 沒有這個概念
	// （它逐郡檢查 Commanded），所以照旗標反推一個相容的值。
	p.Cursor = 0
	for i, pref := range p.Order {
		if pref > 0 && pref < len(p.Pending) && p.Pending[pref] {
			p.Cursor = i
			break
		}
		p.Cursor = i + 1
	}

	p.Seal = -1
	if e.SealAppeared != nil {
		if *e.SealAppeared {
			p.Seal = 0
		}
	} else {
		// 舊補充存檔沒有這個旗標，依當時寶庫作相容推斷。
		for _, f := range e.Factions {
			if f.Treasury[game.TreasureSeal] > 0 {
				p.Seal = 0
				break
			}
		}
	}
	return p
}

// applyProgress 把 `BASEPRO` 的內容套回 Extra。
//
// **只覆蓋這張表真的裝得下的欄位**：其餘（精確人口、受賞、寶庫）留給
// `REMAKE.JSON`。玉璽的已現世旗標是獨立狀態，照原版欄位套用。
func applyProgress(e *game.Extra, p *state.Progress) error {
	if p.Month < 1 || p.Month > 12 {
		return fmt.Errorf("save: BASEPRO 的月份是 %d", p.Month)
	}
	e.Year, e.Month = p.Year, p.Month
	sealAppeared := p.Seal >= 0
	e.SealAppeared = &sealAppeared
	e.Difficulty = p.Difficulty
	e.Options.MusicOff = p.MusicOff
	e.Options.SoundOff = p.SoundOff
	e.Options.VoiceOff = p.VoiceOff
	e.Options.SkipAIWar = p.SkipAIWar
	e.Options.Calendar = game.Calendar(p.Calendar)
	if err := e.Options.SetDelay(p.Delay); err != nil {
		return fmt.Errorf("save: BASEPRO 的延時：%w", err)
	}
	for i := range e.Prefectures {
		if id := i + 1; id < len(p.Pending) {
			e.Prefectures[i].Commanded = !p.Pending[id]
		}
	}
	return nil
}
