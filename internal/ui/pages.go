package ui

import (
	"fmt"
	"sort"
	"strings"

	"github.com/wicanr2/softworld_san1_remake/internal/battle"
	"github.com/wicanr2/softworld_san1_remake/internal/cells"
	"github.com/wicanr2/softworld_san1_remake/internal/game"
	"github.com/wicanr2/softworld_san1_remake/internal/i18n"
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
		out = append(out, cells.Pad(PersonName(x.Name), 8)+cells.Pad(RankName(x.Rank), 6)+
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
		out = append(out, "", t("msg.freeList"))
		line := "  "
		for _, x := range free {
			line += PersonName(x.Name) + "　"
			if cells.Width(line) > 40 {
				out = append(out, line)
				line = "  "
			}
		}
		if cells.Width(line) > 2 {
			out = append(out, line)
		}
	}
	return tf("page.generalsAt", p.ID, PlaceName(p.Name)), out
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
			gov = PersonName(x.Name)
		}
		out = append(out, cells.Pad(fmt.Sprintf("%d", id), 4)+cells.Pad(PlaceName(p.Name), 6)+
			cells.Pad(gov, 8)+cells.Pad(fmt.Sprintf("%d", p.Gold), 7)+
			cells.Pad(fmt.Sprintf("%d", p.Rice), 7)+
			cells.Pad(fmt.Sprintf("%d", p.Population), 8)+
			cells.Pad(fmt.Sprintf("%d", g.Soldiers(id)), 7)+
			cells.Pad(fmt.Sprintf("%d", p.LandValue), 4)+
			cells.Pad(fmt.Sprintf("%d", p.FloodRate), 4)+
			fmt.Sprintf("%d", p.PublicLoyalty))
	}
	lord := g.Lord(f)
	name := tf("fld.factionN", f)
	if lord != nil {
		name = PersonName(lord.Name)
	}
	return tf("page.territoryOf", name, len(ids)), out
}

// TreasuryList 是「君主物品」：查看君主寶庫（說明書 p.19）。
func TreasuryList(g *game.State, f state.FactionID) (string, []string) {
	x := g.Faction(f)
	if x == nil {
		return t("page.treasury"), []string{t("msg.none")}
	}
	var out []string
	total := 0
	for i := game.Treasure(0); i < 5; i++ {
		n := x.Treasury[i]
		total += n
		out = append(out, fmt.Sprintf("%s　%d", game.TreasureName(i), n))
	}
	if total == 0 {
		out = append(out, "", t("msg.treasuryEmpty"))
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

// PersonName／PlaceName 把遊戲資料裡的專有名詞換成目前語系的寫法。
//
// **繁中是原文不是譯文**：這兩個函式在繁中原樣回傳，換語系才動。
func PersonName(s string) string { return i18n.PersonName(s) }
func PlaceName(s string) string  { return i18n.PlaceName(s) }

// TroopName 是兵種的名稱。
func TroopName(k state.TroopType) string {
	keys := []string{"troop.land", "troop.mtn", "troop.water", "troop.mtnLand",
		"troop.waterLand", "troop.mtnWater", "troop.mighty"}
	if int(k) < len(keys) {
		return t(keys[k])
	}
	return "?"
}

// 領域層的型別**自己印中文**：那是遊戲的原文，測試與文件都靠它。
// 畫面要的是譯文，所以由這裡按編號查一次表——編號是資料的一部分，
// 不會因為語系而變。

// TroopKindName 是主戰場上一支部隊的兵種名稱。編號與 state.TroopType 相同。
func TroopKindName(k battle.TroopKind) string {
	return TroopName(state.TroopType(k))
}

// WeatherName 是天候的名稱。
func WeatherName(w battle.Weather) string {
	keys := []string{"weather.clear", "weather.windy", "weather.rainy"}
	if int(w) < len(keys) {
		return t(keys[w])
	}
	return "?"
}

// SideName 是四支軍隊的名稱（說明書 p.28）。
func SideName(s battle.Side) string {
	keys := []string{"side.mainAtt", "side.aidAtt", "side.mainDef", "side.aidDef"}
	if int(s) < len(keys) {
		return t(keys[s])
	}
	return "?"
}

// FormationName 是五種戰鬥隊伍的名稱。
func FormationName(f battle.Formation) string {
	keys := []string{"form.vanguard", "form.left", "form.right",
		"form.centre", "form.rear"}
	if int(f) < len(keys) {
		return t(keys[f])
	}
	return "?"
}

// CommandName 是主戰場一個指令的名稱。編號與原版相同。
func CommandName(c battle.Command) string {
	keys := map[battle.Command]string{
		battle.CmdRest: "bat.rest", battle.CmdMove: "bat.move",
		battle.CmdEngage: "bat.engage", battle.CmdQuick: "bat.quick",
		battle.CmdDeath: "bat.death", battle.CmdArchery: "bat.archery",
		battle.CmdPlot: "bat.plot", battle.CmdInspect: "bat.inspect",
		battle.CmdRetreat: "bat.retreat",
	}
	if k, ok := keys[c]; ok {
		return t(k)
	}
	return "?"
}

// StratagemName 是六種計謀的名稱。編號與原版相同（`docs/design/03` §6）。
func StratagemName(s battle.Stratagem) string {
	keys := []string{"", "strat.fire", "strat.flood", "strat.trap",
		"strat.lure", "strat.burn", "strat.siege"}
	if int(s) < len(keys) && keys[s] != "" {
		return t(keys[s])
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
	side := t("rep.held")
	if r.AttackerWon {
		side = t("rep.won")
	}
	out := []string{
		tf("rep.head", prefName(g, r.From), prefName(g, r.To), side, r.Days),
		tf("rep.losses", r.AttackerLost, r.DefenderLost),
	}
	if len(r.Captives) > 0 {
		names := make([]string, 0, len(r.Captives))
		for _, c := range r.Captives {
			names = append(names, PersonName(c.Name))
		}
		out = append(out, t("rep.captives")+strings.Join(names, "、"))
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
		return PlaceName(p.Name)
	}
	return tf("fld.prefN", id)
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
		origin = tf("gen.origin", PlaceName(p.Name))
	}
	role := t("status.free")
	if x.Employed() {
		role = StatusName(x.Status) + "／" + RankName(x.Rank)
	}
	loyal := "—"
	if x.HasLoyalty() {
		loyal = fmt.Sprintf("%d", x.Loyalty)
	}
	where := "—"
	if p := g.Prefecture(x.Location); p != nil {
		where = PlaceName(p.Name)
	}
	return t("page.inspect"), []string{
		fmt.Sprintf("%s　%s", PersonName(x.Name), origin),
		tf("gen.where", role, where),
		tf("gen.loyalAge", loyal, x.Age),
		"",
		tf("gen.body", x.Stamina, TroopName(x.Troop)),
		tf("gen.intel", x.Intel, x.Soldiers),
		tf("gen.war", x.War, x.Training),
		tf("gen.charm", x.Charm, x.Arms),
		"",
		tf("gen.cap", x.TroopCap(), RankName(x.Rank)),
	}
}
