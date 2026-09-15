//go:build oracle

package parity

import (
	"path/filepath"
	"testing"

	"github.com/wicanr2/dosgolem/oracle"

	"github.com/wicanr2/softworld_san1_remake/internal/save"
	"github.com/wicanr2/softworld_san1_remake/internal/state"
)

// TestZZNoHeirReleasesTerritory 對拍「君主老死而無人繼承」之後那個勢力的
// 下場（Issue #19）：操縱方 ← `0xFFFF`、君主的記錄變已故、它的郡變無主。
//
// 盤面自己擺：挑一個只有一個郡、有兩位以上武將的電腦勢力，把君主以外的
// 人都清成在野（勢力 `0xFF`），君主擺成過壽五年、體能 1——元月必死、
// 而候選清單是空的（`0x14a84` 掃勢力欄，`docs/mechanics/80` §2.5）。
// 走到三月讀三件事，兩邊比**事實**不比位元組：remake 從同一份進度
// （`save.ReadOriginal`）做同樣的改動、走同樣的月份。
//
// 原版**沒有**另一段釋出郡的碼：絕嗣代表勢力裡一個武將都不剩，而郡的
// 歸屬是從人物表重算的（`0x1e394`），下一次郡回合入口就重算成無主。
// 這支測試就是在驗這件事——如果原版還有別的處置（例如把郡直接清掉、
// 或把在野的人也動了），三件事會對不上。
func TestZZNoHeirReleasesTerritory(t *testing.T) {
	root := origRoot(t)
	c := openContainer(t, filepath.Join(root, "DATA2"))
	sc0, err := state.LoadScenario(c, state.Slot("001"))
	if err != nil {
		t.Fatal(err)
	}
	seedMas, _, _ := sc0.Tables()

	o, err := oracle.Load(filepath.Join(root, "AA.EXE"), root)
	if err != nil {
		t.Fatal(err)
	}
	defer o.Close()
	base := bootToGame(t, o, seedMas)
	staBase := base + uint32(state.MasterTableSize)
	genBase := staBase + uint32(state.PrefectureTableSize)

	// 挑勢力：電腦操縱、剛好一個郡、至少兩位武將。
	owners := map[int][]int{}
	for id := 1; id <= state.PrefectureCount; id++ {
		v := int(o.Byte(addr(staBase + uint32(id*state.PrefectureRecordSize+30))))
		if v != 0xFF {
			owners[v] = append(owners[v], id)
		}
	}
	members := map[int][]int{}
	for i := 0; i < 350; i++ {
		f := int(o.Byte(addr(genBase + uint32(i*30+18))))
		if f != 0xFF {
			members[f] = append(members[f], i)
		}
	}
	pick := -1
	for f := 0; f < 16; f++ {
		if o.Word(addr(base+uint32(f*72))) == 2 && len(owners[f]) == 1 && len(members[f]) >= 2 {
			pick = f
			break
		}
	}
	if pick < 0 {
		t.Fatal("找不到「電腦操縱、一個郡、兩位以上武將」的勢力")
	}
	lord := int(o.Word(addr(base + uint32(pick*72+2))))
	life := o.Byte(addr(genBase + uint32(lord*30+28)))
	if life == 0xFF || life == 0 || life > 250-5 {
		t.Fatalf("勢力 %d 的君主槽 %d 壽命 %d 擺不了", pick, lord, life)
	}
	stripped := 0
	for _, i := range members[pick] {
		if i == lord {
			continue
		}
		o.SetByte(addr(genBase+uint32(i*30+18)), 0xFF) // 勢力 ← 在野
		o.SetByte(addr(genBase+uint32(i*30+17)), 9)    // 身分 ← 在野
		o.SetByte(addr(genBase+uint32(i*30+16)), 0xFF) // 忠誠 ← 哨兵
		o.SetByte(addr(genBase+uint32(i*30+19)), 0xFF) // 領地 ← 無
		stripped++
	}
	o.SetByte(addr(genBase+uint32(lord*30+7)), life+5) // 年齡
	o.SetByte(addr(genBase+uint32(lord*30+8)), 1)      // 體能
	prefecture := owners[pick][0]
	t.Logf("勢力 %d（君主槽 %d、郡 %d）：清掉 %d 位部下，君主過壽五年", pick, lord, prefecture, stripped)

	// 走到三月。載入的進度在八月；元月老死、二月的郡回合重算歸屬，
	// 三月讀最穩。
	const settle = 40_000_000
	for m := 0; m < 14; m++ {
		for _, key := range []string{"4\r", "4\r", "Y"} {
			o.Drain()
			o.PressScan(key)
			if err := o.Run(settle * 3); err != nil {
				t.Fatalf("第 %d 個月送 %q 時停止：%v", m+1, key, err)
			}
		}
		if month := int(o.Word(addr(workSeg(o) + unifyMonthOff))); month == 3 {
			break
		}
	}
	year, month := int(o.Word(addr(workSeg(o)+unifyYearOff))), int(o.Word(addr(workSeg(o)+unifyMonthOff)))
	t.Logf("原版走到 %d 年 %d 月", year, month)

	origCtl := o.Word(addr(base + uint32(pick*72)))
	origStatus := o.Byte(addr(genBase + uint32(lord*30+17)))
	origFaction := o.Byte(addr(genBase + uint32(lord*30+18)))
	origOwner := o.Byte(addr(staBase + uint32(prefecture*state.PrefectureRecordSize+30)))
	origHeld := 0
	for id := 1; id <= state.PrefectureCount; id++ {
		if int(o.Byte(addr(staBase+uint32(id*state.PrefectureRecordSize+30)))) == pick {
			origHeld++
		}
	}
	t.Logf("原版：操縱方 %#x、君主身分 %d、君主勢力 %#x、郡 %d 的所屬 %#x、還持 %d 個郡",
		origCtl, origStatus, origFaction, prefecture, origOwner, origHeld)

	// remake：同一份進度、同樣的改動、走到同一個月。
	g, err := save.ReadOriginal(c, 1, state.EditionBase)
	if err != nil {
		t.Fatal(err)
	}
	fid := state.FactionID(pick)
	for _, i := range members[pick] {
		if i == lord {
			continue
		}
		x := g.General(i)
		x.Faction, x.Status, x.Loyalty, x.Location = state.NoFaction, state.StatusIdle, state.NoValue, 0
	}
	lx := g.General(lord)
	lx.Age, lx.Stamina = life+5, 1
	for i := 0; i < 14 && !(g.Date.Month == 3 && g.Date.Year == year); i++ {
		g.EndMonth()
	}
	f := g.Faction(fid)
	rmHeld := len(g.Territory(fid))
	t.Logf("remake：%d 年 %d 月，Lord %d、Alive %v、君主身分 %d、君主勢力 %d、還持 %d 個郡",
		g.Date.Year, g.Date.Month, f.Lord, f.Alive, lx.Status, lx.Faction, rmHeld)

	if origCtl != 0xFFFF {
		t.Errorf("原版的操縱方應為 0xFFFF（絕嗣），得到 %#x", origCtl)
	}
	if origStatus != 12 || origFaction != 0xFF {
		t.Errorf("原版的君主記錄應為身分 12、勢力 0xFF，得到 %d／%#x", origStatus, origFaction)
	}
	if origHeld != 0 {
		t.Errorf("原版絕嗣之後該勢力還持 %d 個郡", origHeld)
	}
	if f.Lord != -1 || f.Alive {
		t.Errorf("remake 絕嗣之後 Lord ＝ %d、Alive ＝ %v，應為 −1／false", f.Lord, f.Alive)
	}
	if lx.Status != state.StatusFallen || lx.Faction != state.NoFaction {
		t.Errorf("remake 的君主記錄應為已故、無勢力，得到 %d／%d", lx.Status, lx.Faction)
	}
	if rmHeld != origHeld {
		t.Errorf("持郡數：原版 %d、remake %d", origHeld, rmHeld)
	}
}
