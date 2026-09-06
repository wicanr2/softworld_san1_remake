// san1assets 把原版的資料轉成通用格式：圖 → PNG、資料表 → JSON、
// 配樂 → MIDI。
//
// ⚠ **本儲存庫不含任何原版檔案。** 這個工具讀的是玩家自己那一份，
// 寫出來的東西也是玩家自己那一份的內容——**不隨本專案散布**。
// 輸出目錄預設在 `workplace/`（已經被 gitignore 擋掉）。
//
//	tools/go.sh run ./cmd/san1assets -root /path/to/三國演義 -out workplace/assets
package main

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"flag"
	"fmt"
	"image/png"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/wicanr2/softworld_san1_remake/internal/assets"
	"github.com/wicanr2/softworld_san1_remake/internal/music"
	"github.com/wicanr2/softworld_san1_remake/internal/state"
)

func main() {
	root := flag.String("root", "", "原版遊戲目錄（必填，玩家自備）")
	out := flag.String("out", "workplace/assets", "輸出目錄")
	what := flag.String("what", "all", "轉什麼：img／json／music／all")
	flag.Parse()
	if *root == "" {
		fmt.Fprintln(os.Stderr, "san1assets: 要用 -root 指到原版目錄（本儲存庫不含原版檔案）")
		flag.Usage()
		os.Exit(2)
	}
	if err := run(*root, *out, *what); err != nil {
		fmt.Fprintln(os.Stderr, "san1assets:", err)
		os.Exit(1)
	}
}

// manifest 是這一次轉出來的清單。
//
// **記來源的雜湊**：轉出來的東西日後對不上時，要先問「來源一不一樣」。
type manifest struct {
	Source string       `json:"source"`
	Files  []fileRecord `json:"files"`
}

type fileRecord struct {
	Path      string `json:"path"`
	From      string `json:"from"`
	Container string `json:"container"`
	Bytes     int    `json:"source_bytes"`
	SHA256    string `json:"source_sha256"`
	Note      string `json:"note,omitempty"`
}

func run(root, out, what string) error {
	if err := os.MkdirAll(out, 0o755); err != nil {
		return err
	}
	mf := manifest{Source: root}
	containers := map[string]*assets.Container{}
	for _, name := range []string{"DATA1", "DATA2", "DATA3"} {
		c, err := open(root, name)
		if err != nil {
			fmt.Printf("略過 %s：%v\n", name, err)
			continue
		}
		containers[name] = c
	}
	if len(containers) == 0 {
		return fmt.Errorf("一組容器都開不起來")
	}

	if what == "img" || what == "all" {
		n, err := exportImages(containers, out, &mf)
		if err != nil {
			return err
		}
		fmt.Printf("圖：%d 張 → %s/img/\n", n, out)
	}
	if what == "json" || what == "all" {
		n, err := exportScenarios(containers["DATA2"], out, &mf)
		if err != nil {
			return err
		}
		fmt.Printf("劇本：%d 份 → %s/scenario/\n", n, out)
	}
	if what == "music" || what == "all" {
		n, err := exportMusic(containers["DATA1"], out, &mf)
		if err != nil {
			return err
		}
		fmt.Printf("配樂：%d 首 → %s/music/\n", n, out)
	}

	sort.Slice(mf.Files, func(i, j int) bool { return mf.Files[i].Path < mf.Files[j].Path })
	b, err := json.MarshalIndent(mf, "", "  ")
	if err != nil {
		return err
	}
	if err := os.WriteFile(filepath.Join(out, "manifest.json"), append(b, '\n'), 0o644); err != nil {
		return err
	}
	fmt.Printf("\n清單：%s/manifest.json（%d 個檔案）\n", out, len(mf.Files))
	fmt.Println("⚠ 這些是你自己那一份原版的內容，只給你自己用，不要散布。")
	return nil
}

func open(root, name string) (*assets.Container, error) {
	base := filepath.Join(root, name)
	var p [3][]byte
	for i, ext := range []string{".NAM", ".IDX", ".GRP"} {
		b, err := os.ReadFile(base + ext)
		if err != nil {
			return nil, err
		}
		p[i] = b
	}
	return assets.OpenContainer(p[0], p[1], p[2])
}

func sum(b []byte) string {
	h := sha256.Sum256(b)
	return hex.EncodeToString(h[:])
}

// exportImages 把每一個 `.IMG`／`.FAC` 存成 PNG。
func exportImages(cs map[string]*assets.Container, out string, mf *manifest) (int, error) {
	n := 0
	names := make([]string, 0, len(cs))
	for k := range cs {
		names = append(names, k)
	}
	sort.Strings(names)
	for _, cn := range names {
		c := cs[cn]
		dir := filepath.Join(out, "img", cn)
		made := false
		for i := 0; i < c.Len(); i++ {
			name := c.Entry(i).Name
			if !strings.HasSuffix(name, ".IMG") && !strings.HasSuffix(name, ".FAC") {
				continue
			}
			raw := c.Data(i)
			im, err := assets.DecodeImage(raw)
			if err != nil {
				// **解不開就跳過並說出來**，不要硬存一張雜訊。
				fmt.Printf("  %s／%s 解不開：%v\n", cn, name, err)
				continue
			}
			if !made {
				if err := os.MkdirAll(dir, 0o755); err != nil {
					return n, err
				}
				made = true
			}
			base := strings.TrimSuffix(strings.TrimSuffix(name, ".IMG"), ".FAC")
			rel := filepath.Join("img", cn, base+".png")
			f, err := os.Create(filepath.Join(out, rel))
			if err != nil {
				return n, err
			}
			err = png.Encode(f, im.RGBA())
			f.Close()
			if err != nil {
				return n, err
			}
			mf.Files = append(mf.Files, fileRecord{
				Path: rel, From: name, Container: cn,
				Bytes: len(raw), SHA256: sum(raw),
				Note: fmt.Sprintf("%d×%d，EGA 16 色", im.W, im.H),
			})
			n++
		}
	}
	return n, nil
}

// scenarioJSON 是一份劇本轉成 JSON 的樣子。
type scenarioJSON struct {
	Slot        string          `json:"slot"`
	Prefectures []prefectureRow `json:"prefectures"`
	Generals    []generalRow    `json:"generals"`
}

type prefectureRow struct {
	ID         int    `json:"id"`
	Name       string `json:"name"`
	Owner      int    `json:"owner"`
	Population int    `json:"population"`
	Soldiers   int    `json:"soldiers"`
	Gold       int    `json:"gold"`
	Rice       int    `json:"rice"`
	Loyalty    int    `json:"public_loyalty"`
	LandValue  int    `json:"land_value"`
	FloodRate  int    `json:"flood_rate"`
	PriceLevel int    `json:"price_level"`
	Neighbours []int  `json:"neighbours"`
}

type generalRow struct {
	Index    int    `json:"index"`
	Name     string `json:"name"`
	Age      int    `json:"age"`
	Stamina  int    `json:"stamina"`
	Intel    int    `json:"intel"`
	War      int    `json:"war"`
	Charm    int    `json:"charm"`
	Rank     int    `json:"rank"`
	Origin   int    `json:"origin"`
	Loyalty  int    `json:"loyalty"`
	Status   int    `json:"status"`
	Faction  int    `json:"faction"`
	Location int    `json:"location"`
	Troop    int    `json:"troop"`
	Soldiers int    `json:"soldiers"`
	Training int    `json:"training"`
	Arms     int    `json:"arms"`
}

// exportScenarios 把六個劇本存成 JSON。
//
// **欄位名用英文**：JSON 是給程式與工具讀的，鍵用 ASCII 才不會在
// 各種工具鏈上出意外；內容（郡名、人名）維持原文。
func exportScenarios(c *assets.Container, out string, mf *manifest) (int, error) {
	if c == nil {
		return 0, fmt.Errorf("沒有 DATA2，讀不到劇本")
	}
	dir := filepath.Join(out, "scenario")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return 0, err
	}
	n := 0
	for _, slot := range []state.Slot{state.Scenario1, state.Scenario2, state.Scenario3,
		state.Scenario4, state.Scenario5, state.Scenario6} {
		sc, err := state.LoadScenario(c, slot)
		if err != nil {
			fmt.Printf("  劇本 %s：%v\n", slot, err)
			continue
		}
		j := scenarioJSON{Slot: string(slot)}
		for _, p := range sc.Prefectures() {
			j.Prefectures = append(j.Prefectures, prefectureRow{
				ID: p.ID, Name: p.Name, Owner: int(p.Owner),
				Population: p.People(), Soldiers: p.Troops(),
				Gold: int(p.Gold), Rice: int(p.Rice),
				Loyalty: int(p.PublicLoyalty), LandValue: int(p.LandValue),
				FloodRate: int(p.FloodRate), PriceLevel: int(p.PriceLevel),
				Neighbours: p.Neighbours,
			})
		}
		for _, g := range sc.Generals() {
			j.Generals = append(j.Generals, generalRow{
				Index: g.Index, Name: g.Name, Age: int(g.Age),
				Stamina: int(g.Stamina), Intel: int(g.Intel), War: int(g.War),
				Charm: int(g.Charm), Rank: int(g.Rank), Origin: int(g.Origin),
				Loyalty: int(g.Loyalty), Status: int(g.Status),
				Faction: int(g.Faction), Location: int(g.Location),
				Troop: int(g.Troop), Soldiers: int(g.Soldiers),
				Training: int(g.Training), Arms: int(g.Arms),
			})
		}
		b, err := json.MarshalIndent(j, "", "  ")
		if err != nil {
			return n, err
		}
		rel := filepath.Join("scenario", string(slot)+".json")
		if err := os.WriteFile(filepath.Join(out, rel), append(b, '\n'), 0o644); err != nil {
			return n, err
		}
		mas, sta, gen := sc.Tables()
		mf.Files = append(mf.Files, fileRecord{
			Path: rel, From: "BASEMAS/BASESTA/BASEGEN." + string(slot),
			Container: "DATA2", Bytes: len(mas) + len(sta) + len(gen),
			SHA256: sum(append(append(append([]byte{}, mas...), sta...), gen...)),
			Note:   "42 個郡 ＋ 350 個人物槽",
		})
		n++
	}
	return n, nil
}

// musicJSON 是配樂的目錄。
type musicJSON struct {
	Tracks []trackRow `json:"tracks"`
	Note   string     `json:"note"`
}

type trackRow struct {
	Index       int      `json:"index"`
	Name        string   `json:"name"`
	File        string   `json:"file"`
	Tempo       int      `json:"tempo_bpm"`
	Ticks       int      `json:"ticks"`
	Seconds     float64  `json:"seconds"`
	Events      int      `json:"events"`
	Instruments []string `json:"instruments"`
}

// exportMusic 把配樂存成標準 MIDI ＋ 一份目錄。
func exportMusic(c *assets.Container, out string, mf *manifest) (int, error) {
	if c == nil {
		return 0, fmt.Errorf("沒有 DATA1，讀不到配樂")
	}
	get := func(name string) []byte {
		i, ok := c.ByName(name)
		if !ok {
			return nil
		}
		return c.Data(i)
	}
	tracks, err := music.ParseAll(get(music.SongIndex), get(music.SongData))
	if err != nil {
		return 0, err
	}
	dir := filepath.Join(out, "music")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return 0, err
	}
	cat := musicJSON{Note: "音高與節奏是原版的；音色不是——原版走 AdLib（OPL2），" +
		"音色參數的對應還沒解（docs/formats/06 §6）"}
	for i, tr := range tracks {
		name := music.Name(i)
		rel := filepath.Join("music", fmt.Sprintf("%d-%s.mid", i+1, name))
		blob := tr.Song.MIDI()
		if err := os.WriteFile(filepath.Join(out, rel), blob, 0o644); err != nil {
			return i, err
		}
		var ins []string
		for _, in := range tr.Bank.Instruments {
			ins = append(ins, in.Name)
		}
		cat.Tracks = append(cat.Tracks, trackRow{
			Index: i + 1, Name: name, File: filepath.Base(rel),
			Tempo: tr.Song.Tempo, Ticks: tr.Song.Ticks,
			Seconds: tr.Song.Duration(), Events: len(tr.Song.Events),
			Instruments: ins,
		})
		mf.Files = append(mf.Files, fileRecord{
			Path: rel, From: "MUS.GRP 第 " + fmt.Sprint(i*2) + " 項",
			Container: "DATA1", Bytes: len(blob), SHA256: sum(blob),
			Note: "標準 MIDI；音色不是原版的 AdLib 音色",
		})
	}
	b, err := json.MarshalIndent(cat, "", "  ")
	if err != nil {
		return len(tracks), err
	}
	rel := filepath.Join("music", "index.json")
	if err := os.WriteFile(filepath.Join(out, rel), append(b, '\n'), 0o644); err != nil {
		return len(tracks), err
	}
	return len(tracks), nil
}
