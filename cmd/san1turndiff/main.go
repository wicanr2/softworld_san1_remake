// san1turndiff 讀兩份原版執行期的盤面，說出中間那一個月發生了什麼。
//
// 輸入是 `internal/parity` 倒出來的 `.bin`：三張表首尾相接的 19,220 個
// 位元組（諸侯 1,152 ＋ 州郡 7,568 ＋ 人物 10,500，`docs/formats/03`）。
//
// 輸出分兩層，**分開標**：
//
//	量到的   哪一個郡、哪一個人、哪一個欄位、差多少
//	推出來的 那組差異看起來像哪一道命令
//
// 第二層是 `L3`。同一個欄位可以有好幾個來源——訓練度上升可能是「訓練
// 兵士」，也可能是新兵加入把平均拉低之後又被別的動作抬回來。**不要把
// 推論寫成觀測。**
package main

import (
	"flag"
	"fmt"
	"os"
	"sort"

	"github.com/wicanr2/softworld_san1_remake/internal/ai"
	"github.com/wicanr2/softworld_san1_remake/internal/game"
	"github.com/wicanr2/softworld_san1_remake/internal/state"
)

func main() {
	quiet := flag.Bool("quiet", false, "只印推論，不印逐欄位的差異")
	price := flag.Bool("price", false, "印每個郡的物價時間序列（吃任意多個檔）")
	lords := flag.Bool("lords", false, "印十六個諸侯槽的君主與領地")
	plan := flag.Bool("plan", false, "從盤面跑 remake 的一個月，逐道印電腦諸侯發出的命令")
	out := flag.String("out", "", "-plan 跑完之後把盤面寫成 .bin")
	month := flag.Int("month", 9, "-plan 的起始月（年月不在三張表裡）")
	year := flag.Int("year", 197, "-plan 的起始年")
	flag.Parse()
	if *plan {
		if flag.NArg() != 1 {
			fmt.Fprintln(os.Stderr, "用法：san1turndiff -plan 盤面.bin [-out 走完.bin]")
			os.Exit(2)
		}
		sc, err := load(flag.Arg(0))
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
		if err := planMonth(sc, *year, *month, *out); err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
		return
	}
	if *price {
		if flag.NArg() < 2 {
			fmt.Fprintln(os.Stderr, "用法：san1turndiff -price 盤面1.bin 盤面2.bin …")
			os.Exit(2)
		}
		priceSeries(flag.Args())
		return
	}
	if *lords {
		if flag.NArg() != 1 {
			fmt.Fprintln(os.Stderr, "用法：san1turndiff -lords 盤面.bin")
			os.Exit(2)
		}
		sc, err := load(flag.Arg(0))
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
		showLords(sc)
		return
	}
	if flag.NArg() != 2 {
		fmt.Fprintln(os.Stderr, "用法：san1turndiff [-quiet] 前.bin 後.bin")
		os.Exit(2)
	}
	a, err := load(flag.Arg(0))
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	b, err := load(flag.Arg(1))
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	report(a, b, *quiet)
}

func load(path string) (*state.Scenario, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	const nMas, nSta = state.MasterTableSize, state.PrefectureTableSize
	want := nMas + nSta + state.GeneralTableSize
	if len(raw) != want {
		return nil, fmt.Errorf("%s 是 %d 個位元組，三張表應該是 %d 個",
			path, len(raw), want)
	}
	return state.DecodeTables(state.Scenario1, raw[:nMas], raw[nMas:nMas+nSta], raw[nMas+nSta:])
}

// prefDelta 是一個郡這個月的變化。
type prefDelta struct {
	id                                 int
	name                               string
	owner                              uint8
	gold, rice, people, soldiers       int
	land, flood, price, loyalty        int
	activeGen, freeGen                 int
	trainUp, armsUp, troopsUp, movedIn int
	statUp, loyaltyUp, joined, left    int

	// 交易要看絕對值不只看差值：米換金的比率由物價決定，
	// 而物價每個月都在動——只印差值就把公式的自變數丟掉了。
	priceBefore, priceAfter int
	goldAfter, riceAfter    int
	peopleAfter             int
}

func (d prefDelta) quiet() bool {
	return d.gold == 0 && d.rice == 0 && d.people == 0 && d.soldiers == 0 &&
		d.land == 0 && d.flood == 0 && d.loyalty == 0 &&
		d.trainUp == 0 && d.armsUp == 0 && d.troopsUp == 0 && d.movedIn == 0 &&
		d.statUp == 0 && d.joined == 0 && d.left == 0
}

func report(a, b *state.Scenario, quiet bool) {
	deltas := map[int]*prefDelta{}
	ap, bp := a.Prefectures(), b.Prefectures()
	for i := range ap {
		x, y := ap[i], bp[i]
		d := &prefDelta{id: y.ID, name: y.Name, owner: y.Owner,
			gold: int(y.Gold) - int(x.Gold), rice: int(y.Rice) - int(x.Rice),
			people:      int(y.Population) - int(x.Population),
			soldiers:    int(y.Soldiers) - int(x.Soldiers),
			land:        int(y.LandValue) - int(x.LandValue),
			flood:       int(y.FloodRate) - int(x.FloodRate),
			price:       int(y.PriceLevel) - int(x.PriceLevel),
			loyalty:     int(y.PublicLoyalty) - int(x.PublicLoyalty),
			activeGen:   int(y.ActiveGenerals) - int(x.ActiveGenerals),
			freeGen:     int(y.FreeGenerals) - int(x.FreeGenerals),
			priceBefore: int(x.PriceLevel), priceAfter: int(y.PriceLevel),
			goldAfter: int(y.Gold), riceAfter: int(y.Rice),
			peopleAfter: int(y.Population),
		}
		deltas[y.ID] = d
	}

	ag, bg := a.Generals(), b.Generals()
	for i := range ag {
		x, y := ag[i], bg[i]
		at := int(y.Location)
		d := deltas[at]
		if d == nil {
			continue
		}
		if y.Training > x.Training {
			d.trainUp++
		}
		if y.Arms > x.Arms {
			d.armsUp++
		}
		if y.Soldiers > x.Soldiers {
			d.troopsUp++
		}
		if y.Location != x.Location {
			d.movedIn++
		}
		if y.Intel != x.Intel || y.War != x.War || y.Charm != x.Charm {
			d.statUp++
		}
		if y.Loyalty != x.Loyalty {
			d.loyaltyUp++
		}
		if y.Faction != x.Faction {
			if y.Employed() {
				d.joined++
			} else {
				d.left++
			}
		}
	}

	ids := make([]int, 0, len(deltas))
	for id := range deltas {
		ids = append(ids, id)
	}
	sort.Ints(ids)

	acted := 0
	for _, id := range ids {
		d := deltas[id]
		if d.quiet() {
			continue
		}
		acted++
		if !quiet {
			fmt.Printf("郡 %2d %s（勢力 %d）%s\n", d.id, d.name, d.owner, fields(*d))
		}
		if t := trade(*d); t != "" {
			fmt.Printf("    %s\n", t)
		}
		if g := guess(*d); g != "" {
			fmt.Printf("    看起來像：%s（L3）\n", g)
		}
	}
	fmt.Printf("\n有動靜的郡：%d／%d\n", acted, len(ids))
}

func fields(d prefDelta) string {
	out := ""
	for _, f := range []struct {
		name string
		v    int
	}{
		{"金", d.gold}, {"米", d.rice}, {"兵士", d.soldiers},
		{"地力", d.land}, {"水利", d.flood}, {"物價", d.price}, {"民忠", d.loyalty},
		{"在職將", d.activeGen}, {"在野將", d.freeGen},
	} {
		if f.v != 0 {
			out += fmt.Sprintf(" %s%+d", f.name, f.v)
		}
	}
	if d.people != 0 {
		// **人口存的是實際值 ÷ 100**（`docs/formats/03`）——只印差值會
		// 讓成長率看起來像另一個數量級，所以把成長後的絕對值一起印。
		out += fmt.Sprintf(" 人口%+d(→%d,%+.1f%%)", d.people, d.peopleAfter,
			100*float64(d.people)/float64(d.peopleAfter-d.people))
	}
	for _, f := range []struct {
		name string
		v    int
	}{
		{"人訓練↑", d.trainUp}, {"人武裝↑", d.armsUp}, {"人兵力↑", d.troopsUp},
		{"人移入", d.movedIn}, {"人能力變", d.statUp}, {"人忠誠變", d.loyaltyUp},
		{"人入仕", d.joined}, {"人離職", d.left},
	} {
		if f.v != 0 {
			out += fmt.Sprintf(" %s%d", f.name, f.v)
		}
	}
	return out
}

// trade 把米金交易的比率印出來，配上當月的物價。
//
// 「依物價購米入倉」「依物價以米換金」（說明書 p.22）——**公式沒給**。
//
// ⚠ **這個比率不是交易的匯率。** 同一個月裡金與米還被每月結算動過
// （兵糧消耗、稅收），所以差值是「交易 ＋ 結算」的合。要解出匯率得用
// 受控盤面：只讓一個郡動、其他欄位固定，再看單獨一次交易換到多少。
// 這裡列出來是為了讓量級與物價的關係看得見，不是結論。
func trade(d prefDelta) string {
	if d.gold >= 0 || d.rice <= 0 {
		if d.gold <= 0 || d.rice >= 0 {
			return ""
		}
	}
	g, r := d.gold, d.rice
	if g < 0 {
		g = -g
	}
	if r < 0 {
		r = -r
	}
	if g == 0 {
		return ""
	}
	dir := "買米"
	if d.gold > 0 {
		dir = "賣米"
	}
	return fmt.Sprintf("%s：Δ米/Δ金 ＝ %.2f，物價 %d→%d（米後 %d、金後 %d）",
		dir, float64(r)/float64(g), d.priceBefore, d.priceAfter,
		d.riceAfter, d.goldAfter)
}

// guess 猜這個郡下了哪一道命令。
//
// ⚠ **這是 `L3`。** 判準是「花費 ＋ 效果」的組合，來源是說明書的數值
// （`docs/reference/01-manual-10-commands.md`）。同一個欄位可以有好幾個
// 來源，所以列出來的是**候選**不是結論。
func guess(d prefDelta) string {
	var out []string
	switch {
	case d.gold < 0 && d.troopsUp > 0 && d.people < 0:
		out = append(out, "徵兵（每人 1 金，人口跟著少）")
	case d.gold < 0 && d.armsUp > 0:
		out = append(out, "購買武器（每 100 單位 1 金）")
	case d.trainUp > 0 && d.gold == 0:
		out = append(out, "訓練兵士（不花錢）")
	}
	if d.gold == -10 && d.land > 0 {
		out = append(out, "土地開發（10 金）")
	}
	if d.gold == -10 && d.flood < 0 {
		out = append(out, "洪水防治（10 金）")
	}
	if d.gold < 0 && d.rice > 0 {
		out = append(out, "買入米糧")
	}
	if d.gold > 0 && d.rice < 0 {
		out = append(out, "賣出米糧")
	}
	if d.rice < 0 && d.loyalty > 0 {
		out = append(out, "開倉賑民")
	}
	if d.gold <= -30 && d.joined > 0 {
		out = append(out, "登用人才（30 金）")
	}
	if d.gold == -5 && d.freeGen > 0 {
		out = append(out, "尋訪人才（5 金）")
	}
	if d.left > 0 {
		out = append(out, "撤職（10 金遣散費）")
	}
	if d.loyaltyUp > 0 && d.gold < 0 {
		out = append(out, "賞賜金帛（最高 100 金）")
	}
	if d.movedIn > 0 {
		out = append(out, "調動軍隊")
	}
	if len(out) == 0 {
		return ""
	}
	s := out[0]
	for _, x := range out[1:] {
		s += "／" + x
	}
	return s
}

// priceSeries 印每個郡的物價隨月份的變化。
//
// **remake 目前完全不動物價**，而原版每個月幾乎每一個郡都在動
// （`docs/mechanics/70-ai` §2.2）。要還原那條規則得先看清楚它怎麼走：
// 有沒有上下界、步幅多大、是不是均值回歸。
func priceSeries(paths []string) {
	var rows [][]int
	var names []string
	for i, p := range paths {
		sc, err := load(p)
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
		var row []int
		for _, x := range sc.Prefectures() {
			row = append(row, int(x.PriceLevel))
			if i == 0 {
				names = append(names, x.Name)
			}
		}
		rows = append(rows, row)
	}

	lo, hi := 255, 0
	var steps []int
	for j := range names {
		fmt.Printf("%-6s", names[j])
		for i := range rows {
			v := rows[i][j]
			fmt.Printf(" %3d", v)
			if v < lo {
				lo = v
			}
			if v > hi {
				hi = v
			}
			if i > 0 {
				steps = append(steps, v-rows[i-1][j])
			}
		}
		fmt.Println()
	}

	// 步幅的分布比平均值有用：均值回歸與隨機漫步的平均都是零。
	hist := map[int]int{}
	for _, d := range steps {
		hist[d]++
	}
	ks := make([]int, 0, len(hist))
	for k := range hist {
		ks = append(ks, k)
	}
	sort.Ints(ks)
	// 值的分布比步幅更能分辨「每月重抽」與「隨機漫步」：
	// 重抽會是平的，漫步會在中間隆起。
	vals := map[int]int{}
	for i := range rows {
		for j := range rows[i] {
			vals[rows[i][j]]++
		}
	}
	vk := make([]int, 0, len(vals))
	for k := range vals {
		vk = append(vk, k)
	}
	sort.Ints(vk)
	fmt.Print("\n值的分布：")
	for _, k := range vk {
		fmt.Printf(" %d×%d", k, vals[k])
	}
	fmt.Println()
	fmt.Printf("物價範圍 %d–%d，%d 次變動\n", lo, hi, len(steps))
	fmt.Print("步幅分布：")
	for _, k := range ks {
		fmt.Printf(" %+d×%d", k, hist[k])
	}
	fmt.Println()
}

// showLords 印十六個諸侯槽：君主是誰、認不認得出是人、有幾個郡。
//
// 存檔可能帶著**自創君主**，那種勢力不在劇本檔裡。判斷「這個槽有沒有
// 在用」不能拿劇本檔比對，得看盤面自己說什麼。
func showLords(sc *state.Scenario) {
	mas, _, _ := sc.Tables()
	owned := map[int]int{}
	for _, p := range sc.Prefectures() {
		if p.Owned() {
			owned[int(p.Owner)]++
		}
	}
	for i := 0; i < state.MasterTableSize/state.MasterRecordSize; i++ {
		g, err := sc.Lord(i)
		switch {
		case err != nil:
			fmt.Printf("槽 %2d：查不到君主（%v）　領地 %d\n", i, err, owned[i])
		default:
			// offset 4 是 AI 等級（`docs/re/03` §1.4，`L0`）。
			lv := int(mas[i*state.MasterRecordSize+4]) |
				int(mas[i*state.MasterRecordSize+5])<<8
			fmt.Printf("槽 %2d：君主槽號 %3d %q　是人 %v　領地 %2d　AI 等級 %d\n",
				i, g.Index, g.Name, g.IsPerson, owned[i], lv)
		}
	}
}

// planMonth 從一份盤面跑 remake 的一個月，把電腦諸侯發出的每一道命令
// 印出來。
//
// 這是**對拍的快速迴圈**：出發局面是 `internal/parity` 倒出來的 `.bin`，
// remake 這一邊不需要 oracle 就能重跑，所以「remake 為什麼多做了這件事」
// 可以在幾秒內問一次，不必每次都等四分鐘的對拍。
//
// ⚠ 它只跑 remake 那一半，**證明不了原版做了什麼**。要比對還是得跑
// `TestZZMonthParity`。
func planMonth(sc *state.Scenario, year, month int, outPath string) error {
	players := sc.Players()
	if len(players) == 0 {
		return fmt.Errorf("這個盤面沒有玩家控制的勢力")
	}
	player := state.FactionID(players[0])
	g, err := game.New(sc, player, 5, state.EditionBase)
	if err != nil {
		return err
	}
	g.Date = game.Date{Year: year, Month: month}
	brain, err := ai.New(ai.ModeBase)
	if err != nil {
		return err
	}
	fmt.Printf("玩家勢力 %d，從 %d 年 %d 月走一個月\n", player, year, month)
	for _, f := range g.Factions() {
		if !f.Alive || f.ID == player {
			continue
		}
		orders, n, err := brain.Act(g, f.ID)
		if len(orders) == 0 {
			continue
		}
		fmt.Printf("勢力 %d（%d 道，套上 %d 道）\n", f.ID, len(orders), n)
		for i, o := range orders {
			fmt.Printf("  %2d 郡 %2d  %s\n", i+1, o.Prefecture(), o.Describe(g))
		}
		if err != nil {
			fmt.Printf("  ⚠ %v\n", err)
		}
	}
	g.EndMonth()
	if outPath == "" {
		return nil
	}
	m, st, gn, err := g.Tables()
	if err != nil {
		return err
	}
	return os.WriteFile(outPath, append(append(append([]byte{}, m...), st...), gn...), 0o644)
}
