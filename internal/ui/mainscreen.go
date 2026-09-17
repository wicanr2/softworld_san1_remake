package ui

import (
	"fmt"
	"image/color"
	"strings"

	"github.com/wicanr2/softworld_san1_remake/internal/cells"
	"github.com/wicanr2/softworld_san1_remake/internal/game"
	"github.com/wicanr2/softworld_san1_remake/internal/i18n"
	"github.com/wicanr2/softworld_san1_remake/internal/state"
)

// Command 是主畫面的一類指令。編號與原版相同（`docs/re/02`）。
type Command struct {
	Key  byte
	Name string
}

// t 取一句介面文字。語系是 `i18n.Current`。
//
// **繁體中文是原文不是譯文**（`internal/i18n`）；換語系只換譯文，
// 不會動到遊戲資料裡的專有名詞（郡名、人名）——那是玩家自己那一份
// 原版檔案的內容。
func t(key string) string { return i18n.S(key) }

// tf 取一句帶參數的介面文字。
func tf(key string, a ...any) string { return i18n.Sf(key, a...) }

// Commands 是主畫面右下的十類指令，順序與編號與原版相同。
func Commands() []Command {
	keys := []string{"cmd.status", "cmd.view", "cmd.military", "cmd.troops",
		"cmd.civil", "cmd.trade", "cmd.people", "cmd.lord", "cmd.plot", "cmd.other"}
	out := make([]Command, 0, len(keys))
	for i, k := range keys {
		out = append(out, Command{Key: byte('0' + i), Name: t(k)})
	}
	return out
}

// 版面（格）。原版把畫面分成左邊的時間、中間的地圖、右上的訊息欄與
// 右下的指令欄（說明書 p.16–17）。這裡照同一個分區，**但地圖是
// remake 自己畫的**——原版的地圖是美術素材，不重製也不散布。
const (
	timeCol  = 0
	timeW    = 4
	mapCol   = timeW
	panelCol = 52
	panelW   = Cols - panelCol
)

// View 是畫面要顯示的東西。**畫面不決定任何規則**——它只讀 Session。
type View struct {
	Sel int // 訊息欄要顯示哪一個郡

	// Menu 是目前展開的子選單；空字串表示在主選單。
	Menu string

	// Items 是子選單的項目（`Menu` 非空時才有意義）。
	Items []Command

	// Prompt 是提示列要顯示的一行字。
	Prompt string

	// Page 是覆蓋在地圖區的整頁內容（將軍列表、領土列表…）；
	// 非空時蓋掉訊息紀錄。
	PageTitle string
	Page      []string

	// Over 為真表示這一局結束了（勝、敗、或被消滅）。
	Over bool

	// Calendar 是年月的表示方式（原版「其他 → 年號」，手冊 p.26）。
	// 零值是中曆，與原版的預設相同。
	Calendar game.Calendar

	// PageTop 是分頁捲到第幾行（折行之後的行數，0 起算）。換頁用
	// `SetPage` 會歸零；捲動用 `ScrollPage`，上下界由畫面決定。
	PageTop int

	// Status 為真時，原版素材畫面的上面板畫**郡的資料**；否則畫十項
	// 指令表（`docs/spec/014` §3.1）。原版等玩家下令時上面板是指令表，
	// 按「0.狀態」才換成郡的資料——這一格就是那個開關。文字版的畫面
	// 兩樣都一直畫著，不看它。
	Status bool

	// HasCard 為真時，原版素材畫面的右側面板整塊換成 `Card` 這位人物的
	// 資料卡（原版「查看→武將」的 `0xf874`，`docs/spec/005` §9.2）。
	// 文字版的畫面走 `GeneralPage`，不看它。
	HasCard bool
	Card    int

	// Save 非 nil 時右側面板是存檔那一格（`DrawSaveScreen`）。
	Save *SaveScreen

	// Input 是下面板提示後面的輸入游標（`docs/spec/014` §4.1）。
	Input InputCursor

	// Roster 非 nil 時右側面板是「挑一位將軍」的清單（`DrawRosterPick`）。
	Roster *RosterPick
	// PrefPick 非 nil 時右側面板是「那一郡」的挑郡清單（`DrawPrefPick`）。
	PrefPick *PrefPick
	// Treasury 非 nil 時右側面板是君主物品表（`DrawTreasuryPanel`）。
	Treasury *TreasuryPanel

	// Atlas 不是 0 時，原版素材畫面整張換成那個郡的地理誌（場地圖與
	// 通道編號，`DrawArtAtlas`），按任意鍵回來；AtlasBubble 是主事者那一句。
	Atlas       int
	AtlasBubble *game.Bubble
}

// DrawMainScreen 畫遊戲主畫面。
func DrawMainScreen(c *Canvas, g *game.State, sel int) {
	DrawSession(c, g, nil, View{Sel: sel})
}

// DrawSession 畫一局進行中的遊戲。log 可以是 nil。
//
// v.Over 為真時在提示列說出結局——**被消滅之後畫面只是變空白的話，
// 玩家會以為是壞掉。**
func DrawSession(c *Canvas, g *game.State, log []string, v View) {
	c.Fill(ColBG)
	drawTimeColumn(c, g.Date, v.Calendar)
	drawMap(c, g, v.Sel)
	drawInfoPanel(c, g, v.Sel)
	if v.Menu == "" {
		drawCommandPanel(c, t("page.command"), Commands())
	} else {
		drawCommandPanel(c, v.Menu, v.Items)
	}
	if len(v.Page) > 0 {
		drawPage(c, v.PageTitle, v.Page, v.PageTop)
	} else {
		drawLog(c, log)
	}
	if v.Over {
		c.DrawText(mapCol+2, Rows-3, t("msg.over"), ColWarn)
	}
	if v.Prompt != "" {
		c.DrawText(mapCol+2, Rows-2, cells.Truncate(v.Prompt, panelCol-mapCol-4), ColSel)
	}
	if n := len(c.Missing); n > 0 {
		c.DrawText(mapCol+2, Rows-1, tf("msg.noGlyph", n), ColWarn)
	}
}

// drawPage 用整頁內容蓋掉地圖區與右側資料欄——列表型的指令（將軍列表、
// 領土列表）要的空間比訊息列多。
//
// **框延伸到畫面右緣**：領土列表中文就要 60 格、英文 67 格，停在地圖區
// （44 格）會把最後幾欄截掉。原版素材畫面的分頁也是蓋掉右側面板
// （`docs/spec/014` §3.3），兩個畫面同一個做法。
//
// 清底色要用 `FillRect`：畫空白字元沒有墨水，等於沒清，地圖會從底下透出來。
func drawPage(c *Canvas, title string, lines []string, top int) {
	w := c.Cols - mapCol
	c.FillRect((mapCol+1)*CellW, CellH, (mapCol+w-1)*CellW, (Rows-1)*CellH, ColBG)
	c.DrawBox(mapCol, 0, w, Rows, ColFrame)
	cols, rows := PageSize(false)
	body, head, hint, top := pageWindow(title, lines, top, cols, rows)
	c.DrawText(mapCol+2, 0, cells.Truncate(head, cols), ColSel)
	for i := 0; i < rows && top+i < len(body); i++ {
		c.DrawText(mapCol+2, 1+i, body[top+i], ColFG)
	}
	if hint != "" {
		c.DrawText(mapCol+2, Rows-1, cells.Truncate(hint, cols), ColDim)
	}
}

// SetPage 打開一頁，捲回最上面。
func (v *View) SetPage(title string, lines []string) {
	v.PageTitle, v.Page, v.PageTop = title, lines, 0
}

// ScrollPage 捲動分頁；art 為真表示原版素材畫面（分頁大小不同）。
func (v *View) ScrollPage(delta int, art bool) {
	cols, rows := PageSize(art)
	v.PageTop = clampTop(len(PageLines(v.Page, cols)), v.PageTop+delta, rows)
}

// PageSize 是分頁一頁放得下幾格寬、幾行（扣掉標題與提示那兩行）。
// art 為真是原版素材畫面的內容區（`docs/spec/014` §3.3），否則是文字版。
func PageSize(art bool) (cols, rows int) {
	if art {
		return (artPageX1-artPageX0)/CellW - 2, (artPageY1-artPageY0)/CellH - 2
	}
	return Cols - mapCol - 4, Rows - 2
}

// PageLines 把分頁的內容折成 cols 格一行：長的一行折下去、續行縮兩格。
//
// **不截字。** 分頁先前把超出的部分截掉，而戰報的長句（「誘敵成功」那一
// 句帶三個部隊名，中文就要七十幾格）截掉的正好是句尾的傷亡數字。表格
// 不會走到這裡——它們的欄寬依內容決定，本來就放得下。
func PageLines(lines []string, cols int) []string {
	out := make([]string, 0, len(lines))
	for _, l := range lines {
		if cells.Width(l) <= cols {
			out = append(out, l)
			continue
		}
		parts := cells.Wrap(l, cols)
		out = append(out, parts[0])
		rest := strings.TrimLeft(l[len(parts[0]):], " ")
		for _, p := range cells.Wrap(rest, cols-2) {
			out = append(out, "  "+p)
		}
	}
	return out
}

// pageWindow 決定這一頁畫哪幾行：折好的內容、標題（捲得動時帶位置）、
// 提示（捲得動時換成怎麼捲），以及夾好的起點。
func pageWindow(title string, lines []string, top, cols, rows int) ([]string, string, string, int) {
	body := PageLines(lines, cols)
	top = clampTop(len(body), top, rows)
	head, hint := title, ""
	if len(body) > rows {
		last := top + rows
		if last > len(body) {
			last = len(body)
		}
		head = tf("page.pos", title, top+1, last, len(body))
		hint = t("hint.pageScroll")
	}
	return body, head, hint, top
}

func clampTop(total, top, rows int) int {
	if top > total-rows {
		top = total - rows
	}
	if top < 0 {
		top = 0
	}
	return top
}

// drawLog 畫地圖區下緣的訊息。**失敗的命令也要看得見**——
// 靜靜地沒反應會讓玩家以為是按鍵沒進去。
func drawLog(c *Canvas, log []string) {
	const rows = 5
	top := Rows - rows - 3
	if len(log) > rows {
		log = log[len(log)-rows:]
	}
	for i, line := range log {
		// 失敗與警告用警示色。**不要只看第一個 byte**：`─`（U+2500）
		// 與 `✗`（U+2717）的 UTF-8 首位元組都是 0xE2，分月線會整排變紅。
		fg := ColDim
		if strings.HasPrefix(line, "✗") || strings.HasPrefix(line, "⚠") {
			fg = ColWarn
		}
		c.DrawText(mapCol+2, top+i, cells.Truncate(line, panelCol-mapCol-4), fg)
	}
}

// drawTimeColumn 畫左側直排的年月，與原版同一個形狀（「中平六年元月」）。
//
// 直排是**漢字才成立的版面**：一個字剛好填滿一格，往下排讀得下去。
// 譯成拼音之後同一個年號變成九個字母，四格寬的欄位一列放不下一個字，
// 拆開往下排就成了每列一個字母的長條。所以拉丁字母的語系改成橫排，
// 放在地圖區上緣——版面讓給可讀性。
func drawTimeColumn(c *Canvas, d game.Date, cal game.Calendar) {
	c.DrawBox(timeCol, 0, timeW, Rows, ColFrame)
	s := d.FormatWithSeason(cal)
	if !hasWide(s) {
		c.DrawText(mapCol+2, 0, s, ColFG)
		return
	}
	i := 0
	for _, ch := range s {
		if 2+i >= Rows-1 {
			break
		}
		c.DrawText(timeCol+2, 2+i, string(ch), ColFG)
		i++
	}
}

// hasWide 說一段文字裡有沒有全形字。
//
// 判準是「有沒有」不是「全部是不是」：西曆的中文寫法是 `189年1月`，
// 阿拉伯數字混著漢字，原版也是一位一列往下排。
func hasWide(s string) bool {
	for _, ch := range s {
		if cells.Width(string(ch)) == 2 {
			return true
		}
	}
	return false
}

// drawMap 畫地圖區。
//
// **這不是原版的地圖。** 原版是一張美術圖，remake 不重製也不散布它。
// 這裡用相鄰表（`docs/spec/003` §3.2）把 42 個郡排成格狀，每一格顯示
// 郡號與所屬勢力的代號——版面不同，但**拓樸與原版一致**，
// 而拓樸才是規則層要的東西。
func drawMap(c *Canvas, g *game.State, sel int) {
	c.DrawBox(mapCol, 0, panelCol-mapCol, Rows, ColFrame)
	prefs := g.Prefectures()

	// 格子的大小由**最長的郡名**決定，不寫死。
	//
	// 中文的郡名一律兩個字（四格），拼音是七到九個字母；照中文的格寬排，
	// `Xiangyang` 會被切成 `Xian`——而畫面上那看起來像資料壞掉，不像
	// 版面不夠寬。放不下就把列距從兩列縮成一列，換得下來的寬度。
	nameW := 0
	for _, p := range prefs {
		if w := cells.Width(PlaceName(p.Name)); w > nameW {
			nameW = w
		}
	}
	cellW := 3 + nameW
	perRow := (panelCol - mapCol - 3) / cellW
	if perRow < 1 {
		perRow = 1
	}
	step := 2
	if rows := (len(prefs) + perRow - 1) / perRow * step; rows > Rows-4 {
		step = 1
	}
	for i, p := range prefs {
		col := mapCol + 2 + (i%perRow)*cellW
		row := 2 + (i/perRow)*step
		if row >= Rows-2 {
			break
		}
		fg := ColDim
		if p.Owned() {
			fg = factionColour(p.Owner)
		}
		c.DrawText(col, row, fmt.Sprintf("%2d", p.ID), ColDim)
		name := cells.Truncate(PlaceName(p.Name), nameW)
		if p.ID == sel {
			// 選取中的郡用反白框標出來，不靠顏色——**顏色會與勢力衝突**。
			c.DrawText(col+2, row, name, ColSel)
			c.DrawBox(col+1, row, nameW+2, 1, ColSel)
		} else {
			c.DrawText(col+2, row, name, fg)
		}
	}
}

// drawInfoPanel 畫右上的訊息欄。
//
// 欄位與原版相同（說明書 p.17）：郡名與編號、所屬諸侯、主事者、
// 金、米、現役將、在野武將、兵士。
func drawInfoPanel(c *Canvas, g *game.State, sel int) {
	c.DrawBox(panelCol, 0, panelW, 13, ColFrame)
	p := g.Prefecture(sel)
	if p == nil {
		c.DrawText(panelCol+2, 2, t("msg.noPrefecture"), ColDim)
		return
	}
	_ = p
	line := func(row int, label, value string) {
		c.DrawText(panelCol+2, row, cells.Pad(label, 10), ColDim)
		c.DrawText(panelCol+12, row, value, ColFG)
	}
	c.DrawText(panelCol+2, 1, fmt.Sprintf("%2d %s", p.ID, PlaceName(p.Name)), ColSel)

	if !p.Owned() {
		c.DrawText(panelCol+2, 3, t("msg.blankPref"), ColDim)
	} else {
		lord := g.Lord(p.Owner)
		gov := g.Governor(p.ID)
		if lord != nil {
			line(3, t("fld.lord"), PersonName(lord.Name))
		}
		if gov != nil {
			line(4, t("fld.governor"), PersonName(gov.Name))
		}
	}
	line(6, t("fld.gold"), fmt.Sprintf("%d", p.Gold))
	line(7, t("fld.rice"), fmt.Sprintf("%d", p.Rice))
	line(8, t("fld.population"), fmt.Sprintf("%d", p.Population))
	line(9, t("fld.soldiers"), fmt.Sprintf("%d", g.Soldiers(p.ID)))

	line(10, t("fld.active"), tf("fld.activeFree",
		g.ActiveGenerals(p.ID), g.FreeGenerals(p.ID)))
	line(11, t("fld.landFlood"), fmt.Sprintf("%d / %d", p.LandValue, p.FloodRate))
}

// drawCommandPanel 畫右下的指令欄。原版是兩欄五列。
// commandPanelRows 是文字版指令欄一欄最多幾項（第 15–22 列；第 23 列
// 留給「Esc 返回」）；commandColW 是兩欄時一欄的寬。
const (
	commandPanelRows = 8
	commandColW      = 13
)

func drawCommandPanel(c *Canvas, title string, cmds []Command) {
	c.DrawBox(panelCol, 13, panelW, Rows-13, ColFrame)
	c.DrawText(panelCol+2, 13, title, ColSel)
	// **八項以內排一欄**（第 15–22 列，名字可以用到右緣），超過才兩欄。
	// 先前六項就排兩欄、每欄 13 格：英文的「Select Prefecture」會跟右欄的
	// 「6.」疊在一起，「Lord's Treasures」在右緣被截——前者連右緣的截字
	// 計數都量不到。兩欄時左欄的名字截在欄內，截掉的照樣記進 `Clipped`。
	per := len(cmds)
	if per > commandPanelRows {
		per = (len(cmds) + 1) / 2
	}
	for i, cmd := range cmds {
		col := panelCol + 2 + (i/per)*commandColW
		row := 15 + i%per
		if row >= Rows-1 {
			break
		}
		c.DrawText(col, row, fmt.Sprintf("%c.", cmd.Key), ColDim)
		// 名字的槽：一欄時到畫布右緣，兩欄時左欄停在右欄前。
		slot := c.Cols - (col + 3)
		if per < len(cmds) && i < per {
			slot = commandColW - 3
		}
		name := cmd.Name
		if cells.Width(name) > slot {
			// **放不下先用小字**（`fonts/README.md`：量得出放不下才用），
			// 小字也放不下或不是 ASCII 才截，截掉的記進 `Clipped`。
			if px := slot * CellW; c.FitsSmall(name) && len([]rune(name))*SmallW <= px {
				c.DrawSmallTextPx((col+3)*CellW, row*CellH+(CellH-SmallH)/2, name, ColFG)
				continue
			}
			c.Clipped += cells.Width(name) - cells.Width(cells.Truncate(name, slot))
			name = cells.Truncate(name, slot)
		}
		c.DrawText(col+3, row, name, ColFG)
	}
	if len(cmds) < len(Commands()) {
		c.DrawText(panelCol+2, Rows-2, t("msg.back"), ColDim)
	}
}

// SubMenu 回傳某一類指令底下的項目，編號與手冊相同
// （`docs/reference/01-manual-10-commands.md`）。
//
// **只列已經實作的。** 列出來卻按不動的項目比沒列更糟——
// 玩家會以為是自己按錯。沒實作的類別回 nil，呼叫端顯示理由。
// SubMenu 回傳一類指令的子選單。
//
// **文字一律照原版執行檔的字串表**（`docs/re/04` §2），不照手冊轉錄：
// 手冊是二手的，而且有出入——「其他」的第一項原版寫 `結束`，手冊寫
// 「＊結束」（`＊` 是手冊自己的記號）。原版的錯字也照原樣留著
// （「洪水防冶」「郡縣自冶」），理由同 `Floopy`（F29）。
//
// 還沒實作的項目**照樣列出來**：選單少一項，玩家看不出是沒做還是
// 原版沒有；列出來按下去會說還沒實作，那是可以理解的狀態。
func SubMenu(key byte) (string, []Command) {
	var title string
	var keys []string
	switch key {
	case '1':
		title, keys = "cmd.view", []string{"view.pick", "view.generals",
			"view.inspect", "view.territory", "view.terrain", "view.treasury"}
	case '2':
		title, keys = "cmd.military", []string{"mil.move", "mil.attack", "mil.transport"}
	case '3':
		title, keys = "cmd.troops", []string{"tro.train", "tro.conscript",
			"tro.arms", "tro.balance"}
	case '4':
		title, keys = "cmd.civil", []string{"civ.reclaim", "civ.flood",
			"civ.fort", "civ.rest"}
	case '5':
		title, keys = "cmd.trade", []string{"trd.buy", "trd.sell", "trd.relief"}
	case '6':
		title, keys = "cmd.people", []string{"ppl.search", "ppl.recruit",
			"ppl.reward", "ppl.dismiss"}
	case '7':
		title, keys = "cmd.lord", []string{"lord.chief", "lord.governor",
			"lord.autonomy", "lord.gift", "lord.headhunt"}
	case '8':
		title, keys = "cmd.plot", []string{"plot.tiger", "plot.distant",
			"plot.forge", "plot.incite", "plot.joint"}
	case '9':
		// 前八項是原版的（`docs/re/04` §3）。後兩項是 **remake 加的**：
		// 原版只有一套 AI，也沒有「電腦一次下幾道令」這回事
		//（`docs/design/02` §5）。加在最後是為了不動原版那八項的編號——
		// 玩家的手指記得「9-3 是音樂」。
		title, keys = "cmd.other", []string{"oth.quit", "oth.save", "oth.music",
			"oth.sound", "oth.delay", "oth.war", "oth.era", "oth.voice",
			"oth.ai", "oth.orders"}
	default:
		return "", nil
	}
	items := make([]Command, 0, len(keys))
	for i, k := range keys {
		items = append(items, Command{Key: MenuKey(i), Name: t(k)})
	}
	return t(title), items
}

// subMenuPrompt 是每一類子選單最後那一行的提示字（`docs/re/04` §2）。
// 謀略的「那一頂:」是原版字，照原樣留著（同「洪水防冶」）。
var subMenuPrompt = map[byte]string{
	'1': "sub.choose", '2': "sub.order", '3': "sub.order", '4': "sub.order",
	'5': "sub.order", '6': "sub.order", '7': "sub.order", '8': "sub.which",
	'9': "sub.choose",
}

// subMenuPerLine 是原版斷行的例外：「軍事」一行一項
// （`1.調動軍隊\n2.發動戰役\n3.運送錢糧\n請下命令:`），兩項其實塞得下。
// 其餘八類都是「塞得下就接在同一行」。
var subMenuPerLine = map[byte]int{'2': 1}

// SubMenuLines 把一類的子選單排成原版下面板上的樣子
// （`docs/spec/014` §2.2）：`編號.名稱`、項目之間一個半形空白、塞得下
// 就接在同一行，最後一行是提示字。cols 是一行幾格。
//
// **中文照這個規則排出來與原版字串逐行相同**（`TestSubMenuLinesMatchTheOriginal`），
// 所以英日文用同一個規則就是「原版會怎麼排」。項目多到四行放不下提示字
// 時，提示字擠到最後一行後面（「其他」有十項，`docs/spec/014` §3.4）。
func SubMenuLines(key byte, items []Command, cols, rows int) []string {
	var lines []string
	cur, n := "", 0
	per := subMenuPerLine[key]
	for _, it := range items {
		s := string(it.Key) + "." + it.Name
		if cur != "" && (cells.Width(cur)+1+cells.Width(s) > cols || (per > 0 && n >= per)) {
			lines = append(lines, cur)
			cur, n = "", 0
		}
		if cur != "" {
			cur += " "
		}
		cur += s
		n++
	}
	if cur != "" {
		lines = append(lines, cur)
	}
	prompt := t(subMenuPrompt[key])
	if k := len(lines); k >= rows && k > 0 &&
		cells.Width(lines[k-1])+1+cells.Width(prompt) <= cols {
		lines[k-1] += " " + prompt
	} else {
		lines = append(lines, prompt)
	}
	return lines
}

// MenuKey 是子選單第 i 項的按鍵。
//
// **第十項是 `0` 不是 `:`。** 鍵盤只送得進 `0`–`9`
// （`cmd/san1` 的 `press`），而 `byte('1'+i)` 到第十項會算出 `:`——
// 那一項就永遠按不動，而且畫面上看起來完全正常。
func MenuKey(i int) byte {
	if i >= 9 {
		return '0'
	}
	return byte('1' + i)
}

// ColSel 是選取中的顏色。
var ColSel = color.RGBA{0xFF, 0xD0, 0x60, 0xFF}

// factionColours 給每個勢力一個顏色。
//
// **這不是原版的色盤。** 原版用 `EGAFILL.PAL`／`HERCFILL.PAL`
// （`docs/formats/01` §3），那還沒解。這裡只求「相鄰的勢力看得出不同」，
// 解出來之後要換掉。
var factionColours = []color.RGBA{
	{0x60, 0xC0, 0x60, 0xFF}, {0x60, 0x90, 0xE0, 0xFF}, {0xE0, 0x80, 0x60, 0xFF},
	{0xE0, 0xD0, 0x60, 0xFF}, {0xC0, 0x70, 0xC0, 0xFF}, {0xA0, 0xA0, 0xA0, 0xFF},
	{0x60, 0xC0, 0xC0, 0xFF}, {0xC0, 0xC0, 0x80, 0xFF}, {0xE0, 0x60, 0x90, 0xFF},
	{0x80, 0xE0, 0xA0, 0xFF}, {0x90, 0x90, 0xE0, 0xFF}, {0xE0, 0xA0, 0x60, 0xFF},
	{0xB0, 0xE0, 0x60, 0xFF}, {0xE0, 0x60, 0x60, 0xFF},
}

func factionColour(f state.FactionID) color.RGBA {
	if int(f) < len(factionColours) {
		return factionColours[f]
	}
	return ColFG
}

// FactionLetter 給勢力一個單字元代號，方便在窄格子裡標示歸屬。
//
// **這是 remake 的顯示手段不是原版行為**——原版用顏色與紋飾區分
// （說明書 p.16）。顏色要等 `.PAL` 解出來才對得上。
func FactionLetter(f state.FactionID) byte {
	if f == state.NoFaction {
		return '.'
	}
	if f < 26 {
		return byte('A' + f)
	}
	return '?'
}
