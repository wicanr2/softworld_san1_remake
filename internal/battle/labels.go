package battle

import "github.com/wicanr2/softworld_san1_remake/internal/i18n"

// 戰場上給玩家看的名字走譯文（`internal/i18n`）。
//
// **`String()` 留著不動**：它們是固定的中文，給除錯輸出與測試用；
// 畫面、提示與逐日戰報一律用 `Label()`。先前戰報直接拿 `String()` 拼句子，
// 英日文玩家看到的戰報內文是中文——三十三則紀錄一則都沒譯
//（`docs/spec/014` §8）。
//
// 鍵與 `internal/ui` 的名稱函式共用同一組（`weather.*`、`side.*`、
// `form.*`、`strat.*`），`ui` 那邊回頭呼叫這裡，**一個名字只留一份對照**。

var (
	weatherKeys   = [...]string{"weather.clear", "weather.windy", "weather.rainy"}
	sideKeys      = [...]string{"side.mainAtt", "side.aidAtt", "side.mainDef", "side.aidDef"}
	sideUnitKeys  = [...]string{"side.unit.mainAtt", "side.unit.aidAtt", "side.unit.mainDef", "side.unit.aidDef"}
	formationKeys = [...]string{"form.vanguard", "form.left", "form.right", "form.centre", "form.rear"}
	stratagemKeys = [...]string{"", "strat.fire", "strat.flood", "strat.trap", "strat.lure", "strat.burn", "strat.siege"}
	terrainKeys   = map[Terrain]string{
		Plain: "terrain.plain", Desert: "terrain.desert", Hill: "terrain.hill",
		Forest: "terrain.forest", Shallow: "terrain.shallow", Deep: "terrain.deep",
		City: "terrain.city", Fort: "terrain.fort", Mountain: "terrain.mountain",
	}
)

func label(keys []string, i int) string {
	if i >= 0 && i < len(keys) && keys[i] != "" {
		return i18n.S(keys[i])
	}
	return "?"
}

// Label 是天候給人看的名字。
func (w Weather) Label() string { return label(weatherKeys[:], int(w)) }

// Label 是四支軍隊給人看的名字（說明書 p.28）。
func (s Side) Label() string { return label(sideKeys[:], int(s)) }

// unitLabel 是部隊名裡的軍別。英文用短的（`Att`），完整的「Main
// Attackers」接上隊形與統帥名要三十幾格，戰報一行就撞邊；中日文與
// `Label()` 相同。
func (s Side) unitLabel() string { return label(sideUnitKeys[:], int(s)) }

// Label 是五種戰鬥隊伍給人看的名字。
func (f Formation) Label() string { return label(formationKeys[:], int(f)) }

// Label 是六種計謀給人看的名字。編號與原版相同。
func (s Stratagem) Label() string { return label(stratagemKeys[:], int(s)) }

// Label 是地形給人看的名字。
func (t Terrain) Label() string {
	if k, ok := terrainKeys[t]; ok {
		return i18n.S(k)
	}
	return "?"
}
