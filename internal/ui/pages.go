package ui

import (
	"fmt"
	"sort"
	"strings"

	"github.com/wicanr2/softworld_san1_remake/internal/cells"
	"github.com/wicanr2/softworld_san1_remake/internal/game"
	"github.com/wicanr2/softworld_san1_remake/internal/state"
)

// 「查看」底下的列表頁。欄位照手冊 p.18–19。

// GeneralList 是「將軍列表」：顯示當地文武官員資料（說明書 p.18）。
func GeneralList(g *game.State, prefectureID int) (string, []string) {
	p := g.Prefecture(prefectureID)
	if p == nil {
		return t("page.generals"), []string{t("msg.none")}
	}
	out := []string{
		cells.Pad(t("fld.name"), 8) + cells.Pad(t("fld.rank"), 6) +
			cells.Pad(t("fld.loyalty"), 4) + cells.Pad(t("fld.age"), 4) +
			cells.Pad(t("fld.stamina"), 4) + cells.Pad(t("fld.intel"), 4) +
			cells.Pad(t("fld.war"), 4) + cells.Pad(t("fld.charm"), 4) +
			cells.Pad(t("fld.soldiers"), 7) + cells.Pad(t("fld.training"), 4) +
			t("fld.arms"),
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
		cells.Pad(t("fld.prefecture"), 4) + cells.Pad(t("fld.name"), 6) +
			cells.Pad(t("fld.governor"), 8) + cells.Pad(t("fld.gold"), 7) +
			cells.Pad(t("fld.rice"), 7) + cells.Pad(t("fld.population"), 8) +
			cells.Pad(t("fld.soldiers"), 7) + cells.Pad(t("fld.landValue"), 4) +
			cells.Pad(t("fld.floodRate"), 4) + t("fld.loyalty"),
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
		return t("page.treasury"), []string{t("msg.none")}
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
	return t("page.treasury"), out
}

// RankName 是職位的名稱。索引與原版的字串表相同（`docs/spec/003` §2.1）。
func RankName(r state.Rank) string {
	keys := []string{"rank.lord", "rank.strategist", "rank.staff", "rank.clerk",
		"rank.advisor", "rank.general", "rank.vice", "rank.sub", "rank.junior"}
	if int(r) < len(keys) {
		return t(keys[r])
	}
	return "?"
}

// StatusName 是身分的名稱（`BASEGEN` offset 17，`docs/spec/003` §2.2）。
func StatusName(s state.Status) string {
	switch s {
	case state.StatusLord:
		return t("status.lord")
	case state.StatusChief:
		return t("status.chief")
	case state.StatusGovernor:
		return t("status.governor")
	case state.StatusOfficer:
		return t("status.officer")
	case state.StatusAvailable, state.StatusIdle:
		return t("status.free")
	case state.StatusUnborn:
		return t("status.unborn")
	}
	return "?"
}

// TroopName 是兵種的名稱。
func TroopName(k state.TroopType) string {
	keys := []string{"troop.land", "troop.mtn", "troop.water", "troop.mtnLand",
		"troop.waterLand", "troop.mtnWater", "troop.mighty"}
	if int(k) < len(keys) {
		return t(keys[k])
	}
	return "?"
}

// BattleReport 是一場戰役的逐日戰報。
//
// **戰役是遊戲裡最花時間的一件事**——三十天、數十支部隊、單挑與計謀。
// 只給一行結果等於把過程丟掉；原版有「查看電腦戰役」這個開關
// （`docs/re/04` §3），就是因為過程本身是內容。
func BattleReport(g *game.State, r *game.BattleResult) (string, []string) {
	if r == nil {
		return t("page.report"), []string{t("msg.none")}
	}
	side := "守方衛郡成功"
	if r.AttackerWon {
		side = "攻方獲勝"
	}
	out := []string{
		fmt.Sprintf("%s 攻 %s　%s　共 %d 日",
			prefName(g, r.From), prefName(g, r.To), side, r.Days),
		fmt.Sprintf("攻方折損 %d　守方折損 %d", r.AttackerLost, r.DefenderLost),
	}
	if len(r.Captives) > 0 {
		names := make([]string, 0, len(r.Captives))
		for _, c := range r.Captives {
			names = append(names, c.Name)
		}
		out = append(out, "被擒："+strings.Join(names, "、"))
	}
	out = append(out, "")
	out = append(out, r.Log...)
	return t("page.report"), out
}

// BattleList 是最近幾場戰役的一覽，給玩家挑一場來看。
func BattleList(g *game.State, rs []*game.BattleResult) (string, []string) {
	if len(rs) == 0 {
		return t("page.battles"), []string{t("msg.none")}
	}
	out := make([]string, 0, len(rs))
	// 新的排前面：剛打完的那一場最可能是玩家要看的。
	for i := len(rs) - 1; i >= 0; i-- {
		out = append(out, fmt.Sprintf("%d. %s", len(rs)-i, rs[i].Summary(g)))
	}
	return t("page.battles"), out
}

// prefName 取郡名；沒有這個郡就印編號。
func prefName(g *game.State, id int) string {
	if p := g.Prefecture(id); p != nil {
		return p.Name
	}
	return fmt.Sprintf("郡%d", id)
}

// GeneralPage 是「檢視將軍」：一位人物的完整資料（說明書 p.18）。
//
// 欄位與原版的檢視畫面對齊（`docs/re/04` §7）：
// `%s%s人氏`、`忠心度 %3d`、`現年%2d歲`、`体能／謀略／戰力／魅力`、
// `兵士數`、`訓練度`、`武裝度`、兵種。
func GeneralPage(g *game.State, index int) (string, []string) {
	x := g.General(index)
	if x == nil {
		return t("page.inspect"), []string{t("msg.none")}
	}
	origin := "—"
	if p := g.Prefecture(x.Origin); p != nil {
		origin = p.Name + "人氏"
	}
	role := "在野"
	if x.Employed() {
		role = StatusName(x.Status) + "／" + RankName(x.Rank)
	}
	loyal := "—"
	if x.HasLoyalty() {
		loyal = fmt.Sprintf("%d", x.Loyalty)
	}
	where := "—"
	if p := g.Prefecture(x.Location); p != nil {
		where = p.Name
	}
	return t("page.inspect"), []string{
		fmt.Sprintf("%s　%s", x.Name, origin),
		fmt.Sprintf("%s　現在 %s", role, where),
		fmt.Sprintf("忠心度 %3s　現年 %2d 歲", loyal, x.Age),
		"",
		fmt.Sprintf("体能 %3d    兵種   %s", x.Stamina, TroopName(x.Troop)),
		fmt.Sprintf("謀略 %3d    兵士數 %5d", x.Intel, x.Soldiers),
		fmt.Sprintf("戰力 %3d    訓練度 %3d", x.War, x.Training),
		fmt.Sprintf("魅力 %3d    武裝度 %3d", x.Charm, x.Arms),
		"",
		fmt.Sprintf("帶兵上限 %d（%s）", x.TroopCap(), RankName(x.Rank)),
	}
}
