// HD 版面 prototype。只由既有 UI 重生靜態樣圖，不接入正式玩家路徑。
// tools/go.sh run ./tools/hd-preview.go -root /orig/三國演義 -edition base
package main

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"flag"
	"fmt"
	"image"
	"image/png"
	"os"
	"path/filepath"

	xdraw "golang.org/x/image/draw"

	"github.com/wicanr2/softworld_san1_remake/internal/assets"
	"github.com/wicanr2/softworld_san1_remake/internal/font"
	"github.com/wicanr2/softworld_san1_remake/internal/game"
	"github.com/wicanr2/softworld_san1_remake/internal/i18n"
	"github.com/wicanr2/softworld_san1_remake/internal/state"
	"github.com/wicanr2/softworld_san1_remake/internal/ui"
)

type previewReceipt struct {
	Status      string            `json:"status"`
	Edition     string            `json:"edition"`
	StateSHA256 string            `json:"state_table_sha256"`
	Inputs      map[string]string `json:"input_sha256"`
	Outputs     []previewOutput   `json:"outputs"`
	Prepared    []preparedArt     `json:"prepared_4x_assets"`
}
type preparedArt struct {
	Key          string `json:"source_key"`
	SourceSHA256 string `json:"source_sha256"`
	MasterSHA256 string `json:"master_sha256"`
	File         string `json:"file"`
	SHA256       string `json:"sha256"`
	Width        int    `json:"width"`
	Height       int    `json:"height"`
}
type previewOutput struct {
	File           string          `json:"file"`
	SHA256         string          `json:"sha256"`
	Width          int             `json:"width"`
	Height         int             `json:"height"`
	Locale         string          `json:"locale"`
	Scale          int             `json:"scale"`
	ArtRect        image.Rectangle `json:"art_rectangle"`
	OutsideChanges int             `json:"changed_pixels_outside_art"`
}

func main() {
	root := flag.String("root", "", "原版資料目錄")
	edition := flag.String("edition", "base", "base／plus")
	artDir := flag.String("art-dir", "workplace", "B 試作原圖所在目錄")
	out := flag.String("out", "workplace/hd-preview", "本機樣圖目錄")
	flag.Parse()
	if err := preview(*root, *edition, *artDir, *out); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func preview(root, edition, artDir, out string) error {
	if root == "" || (edition != "base" && edition != "plus") {
		return fmt.Errorf("需要 -root 及有效 -edition")
	}
	r := previewReceipt{Status: "DRAFT：B 畫風與 4× 定案；正式顯示模式待確認；2× 保留作比較；靜態 prototype，不是玩家路徑或原版對拍", Edition: edition, Inputs: map[string]string{}}
	load := func(cn string) (*assets.Container, error) {
		var b [3][]byte
		for i, ext := range []string{".NAM", ".IDX", ".GRP"} {
			name := cn + ext
			v, err := os.ReadFile(filepath.Join(root, name))
			if err != nil {
				return nil, err
			}
			b[i] = v
			r.Inputs[name] = previewSum(v)
		}
		return assets.OpenContainer(b[0], b[1], b[2])
	}
	data1, err := load("DATA1")
	if err != nil {
		return err
	}
	data2, err := load("DATA2")
	if err != nil {
		return err
	}
	data3, err := load("DATA3")
	if err != nil {
		return err
	}
	sc, err := state.LoadScenario(data2, state.Scenario1)
	if err != nil {
		return err
	}
	player := state.FactionID(state.NoFaction)
	for _, p := range sc.Generals() {
		if p.Name == "曹操" {
			player = state.FactionID(p.Faction)
			break
		}
	}
	if player == state.NoFaction {
		return fmt.Errorf("資料中找不到曹操勢力")
	}
	ed := state.EditionBase
	if edition == "plus" {
		ed = state.EditionPlus
	}
	g, err := game.New(sc, player, 5, ed)
	if err != nil {
		return err
	}
	stateHash := func() (string, error) {
		a, b, c, err := g.Tables()
		if err != nil {
			return "", err
		}
		return previewSum(append(append(append([]byte{}, a...), b...), c...)), nil
	}
	r.StateSHA256, err = stateHash()
	if err != nil {
		return err
	}
	a, err := ui.NewArtScreen(data3, data1)
	if err != nil {
		return err
	}
	loadFace := func(name string, h int) (*font.Face, error) {
		f, e := os.Open(filepath.Join("fonts", name))
		if e != nil {
			return nil, e
		}
		defer f.Close()
		return font.ParseHexGz(f, h)
	}
	face, err := loadFace("unifont.hex.gz", 16)
	if err != nil {
		return err
	}
	small, err := loadFace("ascii6x10.hex.gz", ui.SmallH)
	if err != nil {
		return err
	}
	masters := map[string]image.Image{}
	for _, key := range []string{"F000", "F005", "F228", "SCG01"} {
		name := "hd-b-" + key + "-v1.png"
		b, e := os.ReadFile(filepath.Join(artDir, name))
		if e != nil {
			return e
		}
		r.Inputs[name] = previewSum(b)
		f, e := os.Open(filepath.Join(artDir, name))
		if e != nil {
			return e
		}
		im, e := png.Decode(f)
		f.Close()
		if e != nil {
			return e
		}
		masters[key] = im
		bounds := im.Bounds()
		w, h := 64, 80
		if key == "SCG01" {
			w, h = assets.SceneW, assets.SceneH
		}
		// image_gen 的尺寸會四捨五入；只容許不超過一個來源像素的比例誤差。
		errPx := bounds.Dx()*h - bounds.Dy()*w
		if errPx < 0 {
			errPx = -errPx
		}
		if errPx > max(w, h) {
			return fmt.Errorf("%s 比例與原槽不符：%v", name, bounds)
		}
	}
	if err = os.MkdirAll(out, 0o755); err != nil {
		return err
	}
	for _, key := range []string{"F000", "F005", "F228", "SCG01"} {
		name := key + ".FAC"
		w, h := 64*4, 80*4
		if key == "SCG01" {
			name = key + ".IMG"
			w, h = assets.SceneW*4, assets.SceneH*4
		}
		i, ok := data3.ByName(name)
		if !ok {
			return fmt.Errorf("缺少來源 %s", name)
		}
		im := image.NewRGBA(image.Rect(0, 0, w, h))
		xdraw.CatmullRom.Scale(im, im.Bounds(), masters[key], masters[key].Bounds(), xdraw.Src, nil)
		file := edition + "-" + key + "-prepared-4x.png"
		path := filepath.Join(out, file)
		if err = previewPNG(path, im); err != nil {
			return err
		}
		b, e := os.ReadFile(path)
		if e != nil {
			return e
		}
		r.Prepared = append(r.Prepared, preparedArt{"DATA3/" + name, previewSum(data3.Data(i)), r.Inputs["hd-b-"+key+"-v1.png"], file, previewSum(b), w, h})
	}
	territory := g.Territory(player)
	mainPref := 0
	for _, id := range territory {
		if who := g.Governor(id); who != nil && who.Name == "曹操" && who.Portrait == 0 {
			mainPref = id
			break
		}
	}
	if mainPref == 0 {
		return fmt.Errorf("找不到由曹操 F000 主事的正常州郡狀態")
	}
	for _, locale := range i18n.Locales() {
		i18n.Current = locale
		for _, key := range []string{"main", "F000", "F005", "F228", "SCG01"} {
			c := ui.NewCanvasPx(assets.ScreenW, assets.ScreenH, face)
			c.SetSmallFace(small)
			ui.DrawArtSession(c, a, g, nil, ui.View{Sel: mainPref, Status: true})
			var rect image.Rectangle
			assetKey := key
			if key == "main" {
				rect = image.Rect(536, 116, 600, 196)
				assetKey = "F000"
			}
			if key == "SCG01" {
				ui.DrawScene(c, a, 1, ui.WipeKind(0), assets.SceneMainX, assets.SceneMainY, 1<<20)
				rect = ui.SceneRect(assets.SceneMainX, assets.SceneMainY)
			}
			if key != "main" && key != "SCG01" {
				n := -1
				for _, p := range sc.Generals() {
					if fmt.Sprintf("F%03d", p.Portrait) == key {
						n = p.Index
						break
					}
				}
				if n < 0 {
					return fmt.Errorf("找不到 %s 的人物引用", key)
				}
				ui.DrawPersonCard(c, a, g, n)
				rect = image.Rect(536, 68, 600, 148)
			}
			if len(c.Missing) > 0 {
				return fmt.Errorf("%s/%s 缺字：%v", locale, key, c.Missing)
			}
			if err = previewPNG(filepath.Join(out, edition+"-"+string(locale)+"-"+key+"-original.png"), c.Img); err != nil {
				return err
			}
			for _, scale := range []int{2, 4} {
				b := image.Rect(0, 0, assets.ScreenW*scale, assets.ScreenH*scale)
				im := image.NewRGBA(b)
				xdraw.NearestNeighbor.Scale(im, b, c.Img, c.Img.Bounds(), xdraw.Src, nil)
				hdRect := image.Rect(rect.Min.X*scale, rect.Min.Y*scale, rect.Max.X*scale, rect.Max.Y*scale)
				xdraw.CatmullRom.Scale(im, hdRect, masters[assetKey], masters[assetKey].Bounds(), xdraw.Src, nil)
				changes := 0
				for y := 0; y < b.Dy(); y++ {
					for x := 0; x < b.Dx(); x++ {
						if !image.Pt(x, y).In(hdRect) && im.RGBAAt(x, y) != c.Img.RGBAAt(x/scale, y/scale) {
							changes++
						}
					}
				}
				if changes != 0 {
					return fmt.Errorf("%s/%s 美術越界 %d", locale, key, changes)
				}
				name := fmt.Sprintf("%s-%s-%s-%dx.png", edition, locale, key, scale)
				path := filepath.Join(out, name)
				if err = previewPNG(path, im); err != nil {
					return err
				}
				blob, e := os.ReadFile(path)
				if e != nil {
					return e
				}
				r.Outputs = append(r.Outputs, previewOutput{name, previewSum(blob), b.Dx(), b.Dy(), string(locale), scale, hdRect, changes})
			}
		}
	}
	afterHash, err := stateHash()
	if err != nil {
		return err
	}
	if afterHash != r.StateSHA256 {
		return fmt.Errorf("畫圖改變遊戲表格")
	}
	b, err := json.MarshalIndent(r, "", "  ")
	if err != nil {
		return err
	}
	if err = os.WriteFile(filepath.Join(out, edition+"-receipt.json"), append(b, '\n'), 0o644); err != nil {
		return err
	}
	fmt.Printf("%s：%d 張 HD 版面樣圖；素材區外 0 差；三表雜湊 %s\n", edition, len(r.Outputs), r.StateSHA256)
	return nil
}

func previewPNG(path string, im image.Image) error {
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	err = png.Encode(f, im)
	closeErr := f.Close()
	if err != nil {
		return err
	}
	return closeErr
}
func previewSum(b []byte) string { h := sha256.Sum256(b); return hex.EncodeToString(h[:]) }
