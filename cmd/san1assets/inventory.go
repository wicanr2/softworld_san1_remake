package main

// HD 盤點只輸出中繼資料，不改引擎素材或存檔格式。
import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"sort"
	"strings"

	"github.com/wicanr2/softworld_san1_remake/internal/assets"
	"github.com/wicanr2/softworld_san1_remake/internal/state"
)

type inventory struct {
	Schema      int                `json:"schema_version"`
	Go          string             `json:"go_version"`
	Rights      string             `json:"rights"`
	ToolSources map[string]string  `json:"tool_source_sha256"`
	Editions    []inventoryEdition `json:"editions"`
}

type inventoryEdition struct {
	Edition string            `json:"edition"`
	Sources map[string]string `json:"source_file_sha256"`
	Counts  map[string]int    `json:"container_counts"`
	Assets  []inventoryAsset  `json:"assets"`
	Persons []inventoryPerson `json:"portrait_references"`
}

type inventoryAsset struct {
	Key         string `json:"key"`
	Container   string `json:"container"`
	Name        string `json:"name"`
	Start       uint32 `json:"file_offset_start"`
	End         uint32 `json:"file_offset_end"`
	SHA256      string `json:"source_sha256"`
	PixelSHA256 string `json:"decoded_index_sha256,omitempty"`
	Width       int    `json:"width,omitempty"`
	Height      int    `json:"height,omitempty"`
	Category    string `json:"category"`
	Usage       string `json:"usage"`
	Geometry    string `json:"geometry"`
	Compositing string `json:"compositing"`
	Evidence    string `json:"evidence"`
	Policy      string `json:"hd_policy"`
	Alias       string `json:"identical_cache_of,omitempty"`
	Shared      *bool  `json:"identical_across_editions,omitempty"`
}

type inventoryPerson struct {
	Scenario string `json:"scenario"`
	Index    int    `json:"person_index"`
	Name     string `json:"name"`
	Age      int    `json:"age"`
	Portrait string `json:"portrait_key"`
	Template bool   `json:"custom_lord_template"`
}

var portraitName = regexp.MustCompile(`^F[0-9]{3}\.FAC$`)

func exportInventory(root, peer, out string) error {
	r := inventory{Schema: 1, Go: runtime.Version(),
		Rights:      "原版輸入、解包圖與 AI 衍生圖僅留本機；公開再散布權未知。此清單不含原始 bytes。",
		ToolSources: map[string]string{}}
	for _, name := range []string{"cmd/san1assets/main.go", "cmd/san1assets/inventory.go", "internal/assets/container.go", "internal/assets/image.go", "internal/assets/battlefield.go", "internal/assets/battlescreen.go", "internal/assets/mainscreen.go", "internal/assets/credits.go", "internal/assets/panel.go", "internal/state/scenario.go", "internal/state/customlord.go", "internal/ui/artscreen.go", "internal/ui/artbattle.go", "internal/ui/bubble.go", "internal/ui/card.go", "internal/ui/lordpick.go", "internal/ui/scene.go", "internal/ui/march.go", "internal/ui/credits.go", "internal/opening/script.go"} {
		b, err := os.ReadFile(name)
		if err != nil {
			return fmt.Errorf("盤點請從專案根目錄執行：%w", err)
		}
		r.ToolSources[name] = sum(b)
	}
	for _, dir := range []string{root, peer} {
		if dir == "" {
			continue
		}
		e, err := inventoryRoot(dir)
		if err != nil {
			return err
		}
		r.Editions = append(r.Editions, e)
	}
	if len(r.Editions) == 2 {
		if r.Editions[0].Edition == r.Editions[1].Edition {
			return fmt.Errorf("peer-root 必須是另一版本")
		}
		compareInventory(r.Editions)
	}
	b, err := json.MarshalIndent(r, "", "  ")
	if err != nil {
		return err
	}
	if err = os.MkdirAll(out, 0o755); err != nil {
		return err
	}
	path := filepath.Join(out, "inventory.json")
	if err = os.WriteFile(path, append(b, '\n'), 0o644); err != nil {
		return err
	}
	for _, e := range r.Editions {
		n, cache, visual := 0, 0, 0
		for _, a := range e.Assets {
			if portraitName.MatchString(a.Name) {
				if a.Container == "DATA3" {
					n++
				} else if a.Alias != "" {
					cache++
				}
			}
			if a.Width > 0 {
				visual++
			}
		}
		fmt.Printf("%s：%v；視覺項目 %d，獨立肖像 %d，同內容快取 %d，人物引用 %d\n", e.Edition, e.Counts, visual, n, cache, len(e.Persons))
	}
	fmt.Println("盤點：" + path)
	return nil
}

func inventoryRoot(root string) (inventoryEdition, error) {
	e := inventoryEdition{Sources: map[string]string{}, Counts: map[string]int{}}
	files, err := os.ReadDir(root)
	if err != nil {
		return e, err
	}
	for _, f := range files {
		if !f.Type().IsRegular() {
			continue
		}
		b, err := os.ReadFile(filepath.Join(root, f.Name()))
		if err != nil {
			return e, err
		}
		e.Sources[f.Name()] = sum(b)
	}
	_, base := e.Sources["AA.EXE"]
	_, plus := e.Sources["ASV.EXE"]
	if base == plus {
		return e, fmt.Errorf("%s 必須包含且只包含 AA.EXE 或 ASV.EXE", root)
	}
	e.Edition = "base"
	if plus {
		e.Edition = "plus"
	}
	cs := map[string]*assets.Container{}
	for _, cn := range []string{"DATA1", "DATA2", "DATA3"} {
		c, err := open(root, cn)
		if err != nil {
			return e, err
		}
		cs[cn] = c
		e.Counts[cn] = c.Len()
		aa, err := inventoryContainer(cn, c)
		if err != nil {
			return e, err
		}
		e.Assets = append(e.Assets, aa...)
	}
	byKey := map[string]int{}
	for i, a := range e.Assets {
		byKey[a.Key] = i
	}
	for n := 0; n < 256; n++ {
		key := fmt.Sprintf("DATA3/F%03d.FAC", n)
		if _, ok := byKey[key]; !ok {
			return e, fmt.Errorf("缺少肖像 %s", key)
		}
	}
	for i := range e.Assets {
		a := &e.Assets[i]
		if a.Container == "DATA1" && portraitName.MatchString(a.Name) {
			key := "DATA3/" + a.Name
			j, ok := byKey[key]
			if !ok {
				return e, fmt.Errorf("快取 %s 在 DATA3 沒有本體", a.Key)
			}
			if a.SHA256 == e.Assets[j].SHA256 {
				a.Alias = key
			}
		}
	}
	for _, slot := range []state.Slot{state.Scenario1, state.Scenario2, state.Scenario3, state.Scenario4, state.Scenario5, state.Scenario6} {
		sc, err := state.LoadScenario(cs["DATA2"], slot)
		if err != nil {
			return e, err
		}
		for _, g := range sc.Generals() {
			key := fmt.Sprintf("DATA3/F%03d.FAC", g.Portrait)
			if _, ok := byKey[key]; !ok {
				return e, fmt.Errorf("劇本 %s 人物 %d 引用缺圖 %s", slot, g.Index, key)
			}
			e.Persons = append(e.Persons, inventoryPerson{string(slot), g.Index, g.Name, int(g.Age), key, g.Index >= 346})
		}
	}
	sort.Slice(e.Assets, func(i, j int) bool { return e.Assets[i].Key < e.Assets[j].Key })
	return e, nil
}

func inventoryContainer(cn string, c *assets.Container) ([]inventoryAsset, error) {
	var out []inventoryAsset
	seen := map[string]bool{}
	for i := 0; i < c.Len(); i++ {
		en, raw := c.Entry(i), c.Data(i)
		if seen[en.Name] {
			return nil, fmt.Errorf("重複資源鍵 %s/%s", cn, en.Name)
		}
		seen[en.Name] = true
		a := inventoryAsset{Key: cn + "/" + en.Name, Container: cn, Name: en.Name, Start: en.Start, End: en.End, SHA256: sum(raw)}
		classifyInventory(&a)
		var im *assets.Image
		var err error
		switch filepath.Ext(en.Name) {
		case ".IMG", ".FAC":
			im, err = assets.DecodeImage(raw)
		case ".MSK":
			im, err = assets.DecodeMask(raw)
		}
		if err != nil {
			return nil, fmt.Errorf("%s 解碼失敗：%w", a.Key, err)
		}
		if im != nil {
			a.Width, a.Height, a.PixelSHA256 = im.W, im.H, sum(im.Pix)
			if portraitName.MatchString(en.Name) && (im.W != 64 || im.H != 80) {
				return nil, fmt.Errorf("%s 肖像尺寸錯誤：%d×%d", a.Key, im.W, im.H)
			}
		}
		out = append(out, a)
		if cn == "DATA1" && en.Name == "EICON.GRP" {
			tiles, err := assets.BattleTiles(c)
			if err != nil {
				return nil, err
			}
			for n, tile := range tiles {
				t := a
				t.Key = fmt.Sprintf("%s#%02d", a.Key, n)
				t.Start = en.Start + uint32(n*(assets.ImageHeader+assets.TileW/8*assets.TileH*4))
				t.End = t.Start + uint32(assets.ImageHeader+assets.TileW/8*assets.TileH*4)
				t.SHA256 = sum(raw[t.Start-en.Start : t.End-en.Start])
				t.Width, t.Height, t.PixelSHA256 = tile.W, tile.H, sum(tile.Pix)
				out = append(out, t)
			}
		}
	}
	return out, nil
}

func compareInventory(editions []inventoryEdition) {
	for i := range editions {
		other := map[string]string{}
		for _, a := range editions[1-i].Assets {
			other[a.Key] = a.SHA256
		}
		for j := range editions[i].Assets {
			a := &editions[i].Assets[j]
			if hash, ok := other[a.Key]; ok {
				same := hash == a.SHA256
				a.Shared = &same
			}
		}
	}
}

func classifyInventory(a *inventoryAsset) {
	n := a.Name
	a.Category, a.Usage, a.Geometry, a.Compositing, a.Evidence, a.Policy = "非視覺資料", "未逐項解出用途；見既有 formats 目錄", "不適用", "不適用", "L0 格式及雜湊；用途 unknown", "不交給 AI"
	visual := strings.HasSuffix(n, ".IMG") || strings.HasSuffix(n, ".FAC") || strings.HasSuffix(n, ".MSK")
	if visual {
		a.Category, a.Geometry, a.Compositing, a.Policy = "待定位圖像", "unknown；不授權正式替換", "unknown", "待用途查證"
	}
	set := func(category, usage, geometry, compose, policy string) {
		a.Category, a.Usage, a.Geometry, a.Compositing, a.Policy = category, usage, geometry, compose, policy
		a.Evidence = "L0 尺寸與雜湊；目前 remake 使用端已查證；不外推原版未驗用途"
	}
	switch {
	case portraitName.MatchString(n):
		set("肖像", "ui.ArtScreen.Portrait / ui.ArtBattle.face / opening.LoadArt；state.General.Portrait offset 27", "主畫面 (536,116)；人物卡 (536,68)；選君主 ui/lordpick.go；戰場面板 assets/battlescreen.go；對白左右由框算；自創君主 state.CustomLordPortrait", "矩形不透明；對白左側／戰場攻方須左右鏡像", "B 寫實手繪；保留 4:5、頭部輪廓、服飾、背景與視線")
	case strings.HasPrefix(n, "SCG"):
		set("事件場景", "ui.ArtScreen.Scene / ui.DrawScene；spec/010；SCG30/31 在 DATA2，現行讀取端未接 DATA2", "(432,80)、(432,120)、(448,268)，按事件呼叫端選取", "176×96 不透明；四向拉幕，須依原揭露區裁切", "B 寫實手繪；保持原構圖")
	case strings.HasPrefix(n, "MAINMAP"):
		set("大地圖及邊框", "assets.MainScreen / ui.ArtScreen.Compose；MAINMAP8 為戰場底框", "逐片座標 assets/mainscreen.go MainScreen、spec/006 §2.3；8 像素接縫不可漂移", "不透明；地圖填色及州郡邊界由原版索引圖決定", "邊框可新繪；地圖拓樸及填色遮罩保留")
	case strings.HasPrefix(n, "MENU"):
		set("主選單底圖與按鈕", "assets.MenuScreen / ui.DrawTitleLayer", "assets/mainscreen.go MenuScreen 與 MenuButtons；文字獨立於按鈕", "不透明；MENU3 飾框受游標動畫覆寫", "可新繪美術；按鈕字、命中區及格位由程式保留")
	case strings.HasPrefix(n, "FBR") || strings.HasPrefix(n, "SIDE"):
		set("拼接邊框", "assets.PortraitFrame / assets.LoadSideFrame；ui/card.go、artscreen.go、artbattle.go", "依面板／肖像槽拼接；四角、邊及接縫固定", "不透明；平鋪邊條不得露縫", "可新繪邊框，保留拼接幾何")
	case strings.HasPrefix(n, "CUR") || strings.Contains(n, "CUR"):
		set("游標與遮罩", "assets.CursorFrames / ui/cursor.go；MAPCUR1 ui/artbattle.go；其餘用途見原鍵，不假定共用", "跟隨既有邏輯游標與 CursorFrame；命中區不由圖像推導", "CUR?0M 等為配對遮罩；沿用 AND／OR 語意；MAPCUR 依原控制流", "保留幾何及動畫，禁用 AI 自動改輪廓／遮罩")
	case strings.HasPrefix(n, "CVSC"):
		set("行軍圖及遮罩", "ui.MarchArt / ui.NewMarchLayout", "依兩郡座標及方向決定 32×32 圖和配對遮罩；ui/march.go", "CVSC00–15 圖；16–23 遮罩；模式 5 與模式 2 合成", "美術可新繪；配對遮罩及動畫幾何另驗")
	case strings.HasPrefix(n, "MV"):
		set("地圖動畫圖示", "目前 ui 未直接引用；舊 formats/04 用途不升格", "unknown", "unknown", "待定位，暫不替換")
	case strings.HasPrefix(n, "WFLAG"):
		set("戰場旗幟", "assets.FlagName / assets.UnitFlags；ui.DrawArtField", "assets.FlagCell；欄×48+56、列×32+36、奇欄+16；下方兵力牌獨立", "矩形；選取幀取色號補數", "旗幟可新繪；陣型、陣營、兵力牌及選取保留")
	case n == "EICON.GRP":
		set("戰場圖塊與特效", "assets.BattleTiles / assets.BattleField；ui/lure.go / skirmish.go", "36 張 48×32；assets.FieldOriginX/Y、FieldStagger；特效由既有編號及步序控制", "矩形；圖塊拼接及旗幟疊層固定", "可新繪地形與特效；地形碼、遮罩與邏輯邊界保留")
	case strings.HasPrefix(n, "8x8"):
		set("底紋與遮罩", "assets.Masks；8x8PAT0 為戰場底紋，其餘用途不外推", "以原 8×8 週期鋪排", "AND 底紋為遮罩；不轉成任意 alpha", "保留遮罩；可新繪不改辨識度的底紋")
	case strings.HasPrefix(n, "WEATHER"):
		set("天候圖示", "ui.NewArtBattle / ui.DrawArtBattle", "面板落點由 assets.BattleLayout.Panel / ui/artbattle.go 決定", "32×32 不透明", "可依 B 重繪，保留天候語意")
	case strings.HasPrefix(n, "CP"):
		set("待定位圖塊", "CP128–171；目前 remake 無直接使用端；formats/04 的歷史假說不升格", "24×24；未解出顯示位置", "unknown", "待定位；保留原圖，不進首批 HD")
		a.Evidence = "L0 尺寸及雜湊；用途 unknown"
	case strings.HasPrefix(n, "UPR") || strings.HasPrefix(n, "PRV") || n == "TITFONT.IMG" || n == "LOADS.IMG" || strings.HasPrefix(n, "TZUE"):
		set("圖中字幕及文字", "assets.LoadCredits / opening.LoadArt；TZUE1 是 TZUE 遮罩；spec/012 與 re/13", "沿用字幕、標題、載入文字及片頭位置；opening/script.go", "TZUE/TZUE1 配對；UPR／PRV 已含字，不當一般美術", "禁用 AI 改字；文字與配對遮罩由程式及字型重建")
	case strings.HasPrefix(n, "REC") || strings.HasPrefix(n, "ENDO"):
		set("製作群背景與遮罩", "assets.LoadCredits / ui/credits.go；UPR22/23 未由 LoadCredits 採用", "四片或兩片並排 640×336；ENDO4 對齊 y=54，spec/012", "ENDO4 單平面，1 天空／0 山；其他不透明", "可依 B 重繪背景；天空遮罩、字幕及分片接縫保留")
	case strings.HasPrefix(n, "TITL") || strings.HasPrefix(n, "SANT") || strings.HasPrefix(n, "CMARK"):
		set("片頭美術", "opening.LoadArt / opening.Script；assets.TitleArt / TrademarkScreen / PoemScreen", "商標 (0,64) 與 (320,64)；片頭海景／船／三英圖由 opening/script.go 原步序決定", "矩形與原模式合成；商標／題字不可任意改造", "B 美術可重繪；商標、文字與動畫定位另驗")
	}
}
