package ui

import (
	"fmt"
	"sort"

	"github.com/wicanr2/softworld_san1_remake/internal/cells"
	"github.com/wicanr2/softworld_san1_remake/internal/game"
	"github.com/wicanr2/softworld_san1_remake/internal/state"
)

// 「查看」底下的列表頁。欄位照手冊 p.18–19。

// GeneralList 是「將軍列表」：顯示當地文武官員資料（說明書 p.18）。
func GeneralList(g *game.State, prefectureID int) (string, []string) {
	p := g.Prefecture(prefectureID)
	if p == nil {
		return "將軍列表", []string{"（沒有這個郡）"}
	}
	out := []string{
		cells.Pad("姓名", 8) + cells.Pad("職位", 6) + cells.Pad("忠", 4) +
			cells.Pad("齡", 4) + cells.Pad("體", 4) + cells.Pad("謀", 4) +
			cells.Pad("戰", 4) + cells.Pad("魅", 4) + cells.Pad("兵士", 7) +
			cells.Pad("訓", 4) + "武",
	}
	for _, x := range g.Garrison(prefectureID) {
		if x.Faction != p.Owner {
			continue
		}
		loyal := "—"
		if x.HasLoyalty() {
			loyal = fmt.Sprintf("%d", x.Loyalty)
		}
		out = append(out, cells.Pad(x.Name, 8)+cells.Pad(RankName(x.Rank), 6)+
			cells.Pad(loyal, 4)+cells.Pad(fmt.Sprintf("%d", x.Age), 4)+
			cells.Pad(fmt.Sprintf("%d", x.Stamina), 4)+
			cells.Pad(fmt.Sprintf("%d", x.Intel), 4)+
			cells.Pad(fmt.Sprintf("%d", x.War), 4)+
			cells.Pad(fmt.Sprintf("%d", x.Charm), 4)+
			cells.Pad(fmt.Sprintf("%d", x.Soldiers), 7)+
			cells.Pad(fmt.Sprintf("%d", x.Training), 4)+
			fmt.Sprintf("%d", x.Arms))
	}
	free := g.Free(prefectureID)
	if len(free) > 0 {
		out = append(out, "", "在野：")
		line := "  "
		for _, x := range free {
			line += x.Name + "　"
			if cells.Width(line) > 40 {
				out = append(out, line)
				line = "  "
			}
		}
		if cells.Width(line) > 2 {
			out = append(out, line)
		}
	}
	return fmt.Sprintf("將軍列表　%d %s", p.ID, p.Name), out
}

// TerritoryList 是「領土列表」：列出該軍所有領土基本資料（說明書 p.19）。
func TerritoryList(g *game.State, f state.FactionID) (string, []string) {
	ids := g.Territory(f)
	sort.Ints(ids)
	out := []string{
		cells.Pad("郡", 4) + cells.Pad("名稱", 6) + cells.Pad("太守", 8) +
			cells.Pad("金", 7) + cells.Pad("米", 7) + cells.Pad("人口", 8) +
			cells.Pad("兵士", 7) + cells.Pad("地", 4) + cells.Pad("洪", 4) + "民",
	}
	for _, id := range ids {
		p := g.Prefecture(id)
		gov := "—"
		if x := g.Governor(id); x != nil {
			gov = x.Name
		}
		out = append(out, cells.Pad(fmt.Sprintf("%d", id), 4)+cells.Pad(p.Name, 6)+
			cells.Pad(gov, 8)+cells.Pad(fmt.Sprintf("%d", p.Gold), 7)+
			cells.Pad(fmt.Sprintf("%d", p.Rice), 7)+
			cells.Pad(fmt.Sprintf("%d", p.Population), 8)+
			cells.Pad(fmt.Sprintf("%d", g.Soldiers(id)), 7)+
			cells.Pad(fmt.Sprintf("%d", p.LandValue), 4)+
			cells.Pad(fmt.Sprintf("%d", p.FloodRate), 4)+
			fmt.Sprintf("%d", p.PublicLoyalty))
	}
	lord := g.Lord(f)
	name := fmt.Sprintf("勢力 %d", f)
	if lord != nil {
		name = lord.Name
	}
	return fmt.Sprintf("領土列表　%s　共 %d 郡", name, len(ids)), out
}

// TreasuryList 是「君主物品」：查看君主寶庫（說明書 p.19）。
func TreasuryList(g *game.State, f state.FactionID) (string, []string) {
	x := g.Faction(f)
	if x == nil {
		return "君主物品", []string{"（沒有這個勢力）"}
	}
	var out []string
	total := 0
	for t := game.Treasure(0); t < 5; t++ {
		n := x.Treasury[t]
		total += n
		out = append(out, fmt.Sprintf("%s　%d", t, n))
	}
	if total == 0 {
		out = append(out, "", "寶庫是空的。冬季各州郡會進貢，領地越多貢品越多。")
	}
	return "君主物品", out
}

// RankName 是職位的名稱。索引與原版的字串表相同（`docs/spec/003` §2.1）。
func RankName(r state.Rank) string {
	names := []string{"君主", "軍師", "參軍", "主簿", "謀士",
		"大將", "副將", "裨將", "牙將"}
	if int(r) < len(names) {
		return names[r]
	}
	return "?"
}

// TroopName 是兵種的名稱。
func TroopName(t state.TroopType) string {
	names := []string{"陸", "山", "水", "山陸", "水陸", "山水", "強力"}
	if int(t) < len(names) {
		return names[t]
	}
	return "?"
}
