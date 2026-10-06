//go:build ignore

// 回讀正常 GUI 的六槽存檔與完整 PNG。參考圖只產生於本機記憶體。
package main

import (
	"bytes"
	"crypto/sha256"
	"encoding/json"
	"flag"
	"fmt"
	"image"
	_ "image/png"
	"os"
	"path/filepath"
	"reflect"
	"runtime"
	"strings"

	"github.com/wicanr2/softworld_san1_remake/internal/ai"
	"github.com/wicanr2/softworld_san1_remake/internal/assets"
	"github.com/wicanr2/softworld_san1_remake/internal/font"
	"github.com/wicanr2/softworld_san1_remake/internal/game"
	"github.com/wicanr2/softworld_san1_remake/internal/save"
	"github.com/wicanr2/softworld_san1_remake/internal/session"
	"github.com/wicanr2/softworld_san1_remake/internal/ui"
)

func must(err error) {
	if err != nil {
		panic(err)
	}
}
func check(ok bool, label string) {
	if !ok {
		panic(label)
	}
}
func data(path string) []byte   { b, err := os.ReadFile(path); must(err); return b }
func digest(path string) string { return fmt.Sprintf("%x", sha256.Sum256(data(path))) }
func metadata(path string) map[string]any {
	var m map[string]any
	must(json.Unmarshal(data(path), &m))
	return m
}
func openContainer(root, name string) *assets.Container {
	c, err := assets.OpenContainer(data(filepath.Join(root, name+".NAM")), data(filepath.Join(root, name+".IDX")), data(filepath.Join(root, name+".GRP")))
	must(err)
	return c
}
func picture(path string) (image.Image, []byte) {
	im, _, err := image.Decode(bytes.NewReader(data(path)))
	must(err)
	b := im.Bounds()
	check(b.Min == image.Pt(0, 0), "PNG 起點")
	rgb := make([]byte, 0, b.Dx()*b.Dy()*3)
	for y := 0; y < b.Dy(); y++ {
		for x := 0; x < b.Dx(); x++ {
			r, g, bl, a := im.At(x, y).RGBA()
			check(a == 65535, "PNG 透明")
			rgb = append(rgb, byte(r>>8), byte(g>>8), byte(bl>>8))
		}
	}
	return im, rgb
}
func crop(rgb []byte, width int, r image.Rectangle) []byte {
	b := make([]byte, 0, r.Dx()*r.Dy()*3)
	for y := r.Min.Y; y < r.Max.Y; y++ {
		b = append(b, rgb[(y*width+r.Min.X)*3:(y*width+r.Max.X)*3]...)
	}
	return b
}
func rgbaRGB(im *image.RGBA) []byte {
	b := im.Bounds()
	rgb := make([]byte, 0, b.Dx()*b.Dy()*3)
	for y := 0; y < b.Dy(); y++ {
		for x := 0; x < b.Dx(); x++ {
			p := im.PixOffset(x, y)
			rgb = append(rgb, im.Pix[p:p+3]...)
		}
	}
	return rgb
}

type savedFile struct {
	Meta  map[string]any `json:"meta"`
	SHA   string         `json:"sha256"`
	Bytes int            `json:"bytes"`
}
type snapshot map[string]savedFile

func readSnapshot(dir string) snapshot {
	m := snapshot{}
	must(filepath.WalkDir(dir, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			return nil
		}
		name, err := filepath.Rel(dir, path)
		if err != nil {
			return err
		}
		v := savedFile{SHA: digest(path)}
		if d.Name() == "REMAKE.JSON" {
			v.Meta = metadata(path)
			delete(v.Meta, "saved_at")
		} else {
			v.Bytes = len(data(path))
		}
		m[name] = v
		return nil
	}))
	return m
}
func canonical(m snapshot) snapshot {
	n := snapshot{}
	for k, v := range m {
		if v.Meta != nil {
			v.SHA = ""
		}
		n[k] = v
	}
	return n
}

func functional(m snapshot) map[string]map[string]any {
	n := map[string]map[string]any{}
	for k, v := range m {
		if v.Meta != nil {
			n[k] = v.Meta
		}
	}
	return n
}

type capture struct {
	File          string
	Width, Height int
	SHA           string `json:"sha256"`
}
type phase struct {
	Error      string
	Identical  bool
	Rect       []int
	Background int
	Old        int `json:"original_phase"`
	New        int `json:"restored_phase"`
	Complete   int `json:"complete_pixels"`
	Excluded   int `json:"excluded_pixels"`
}
type pair struct {
	Tag, Original, High, Restored string
	Phase                         phase
}
type panel struct {
	Tag, File, Memo string
	Slot            int
	High            bool
	Before          snapshot
}
type roundtrip struct {
	Tag, Edition  string
	Slot          int
	High          bool
	SaveDir       string `json:"save_dir"`
	Before, After snapshot
}
type selection struct {
	File                         string
	Field, Expected, Rows, Scale int
}
type receipt struct {
	Passed     bool
	Binary     string `json:"binary_sha256"`
	Pack       string `json:"pack_sha256"`
	Captures   []capture
	Pairs      []pair
	Panels     []panel `json:"save_panels"`
	Roundtrips []roundtrip
	Baselines  map[string]snapshot
	Selections []selection
	Checks     []struct {
		Name   string
		Passed bool
	}
	Closes []struct {
		Tag  string
		Code int `json:"returncode"`
	} `json:"normal_closes"`
}

func main() {
	name := flag.String("out", "", "既有 workplace/hd-save-* 目錄")
	flag.Parse()
	check(strings.HasPrefix(*name, "workplace/hd-save-") && !strings.Contains(*name, ".."), "輸出路徑")
	out := filepath.Join("/src", *name)
	var r receipt
	diagnostics := map[string]bool{}
	diagnose := func(ok bool, label string) { diagnostics[label] = ok }
	must(json.Unmarshal(data(filepath.Join(out, "receipt.json")), &r))
	check(r.Passed, "GUI 收據未通過")
	check(len(r.Roundtrips) == 6 && len(r.Pairs) == 6 && len(r.Panels) == 18 && len(r.Closes) == 8, "正常路徑數量")
	for _, c := range r.Checks {
		check(c.Passed, c.Name)
	}
	for _, c := range r.Closes {
		check(c.Code == 0, "正常視窗關閉")
	}
	check(digest(filepath.Join(out, "san1-window-check")) == r.Binary, "執行檔雜湊")
	packDir := "/src/workplace/hd-assets-mapcursor-v50-r1"
	check(digest(filepath.Join(packDir, "manifest.json")) == r.Pack, "HD 包雜湊")
	for _, c := range r.Captures {
		path := filepath.Join(out, c.File)
		check(digest(path) == c.SHA, "PNG 雜湊 "+c.File)
		im, _, err := image.Decode(bytes.NewReader(data(path)))
		must(err)
		check(im.Bounds().Dx() == c.Width && im.Bounds().Dy() == c.Height, "完整 PNG 尺寸")
	}
	containers := map[string]map[string]*assets.Container{}
	arts := map[string]*ui.ArtScreen{}
	packs := map[string]*ui.HDPack{}
	for _, ed := range []string{"base", "plus"} {
		folder := "三國演義"
		if ed == "plus" {
			folder = "三國演義1加強版"
		}
		cs := map[string]*assets.Container{}
		for _, n := range []string{"DATA1", "DATA2", "DATA3"} {
			cs[n] = openContainer(filepath.Join("/orig", folder), n)
		}
		containers[ed] = cs
		var err error
		arts[ed], err = ui.NewArtScreen(cs["DATA3"], cs["DATA1"], cs["DATA2"])
		must(err)
		packs[ed], err = ui.LoadHDPack(packDir, ed, cs)
		must(err)
		check(len(packs[ed].Warnings) == 0, "HD 包警告")
		actual := readSnapshot(filepath.Join(out, "saves-"+ed))
		check(reflect.DeepEqual(actual, r.Baselines[ed]), "基準存檔保持")
		check(len(actual) == 37, "六槽完整檔案")
		for slot := 1; slot <= 6; slot++ {
			ss, err := session.Load(filepath.Join(out, "saves-"+ed), slot, ai.ModeEnhanced)
			must(err)
			mode := ai.ModeEnhanced
			if slot == 1 {
				mode = ai.Mode(ed)
			}
			check(ss.Brain.Mode() == mode, "六槽已保存的 AI")
			if slot > 1 {
				check(ss.G.Options.AIOrders() == slot-1, "六槽 AI 強度")
			}
		}
	}
	for _, p := range r.Panels {
		ed := strings.Split(p.Tag, "-")[0]
		base := r.Baselines[ed]
		meta := base[fmt.Sprintf("SV%d/REMAKE.JSON", p.Slot)].Meta
		kind, _ := meta["options"].(map[string]any)["font"].(float64)
		fontName := game.FontFile(int(kind))
		if fontName == "" {
			fontName = "unifont.hex.gz"
		}
		f, err := os.Open(filepath.Join("/src/fonts", fontName))
		must(err)
		face, err := font.ParseHexGz(f, 16)
		must(err)
		must(f.Close())
		c := ui.NewCanvasPx(640, 408, face)
		c.HD = packs[ed]
		s := &ui.SaveScreen{Slot: p.Slot}
		for k := 1; k <= 6; k++ {
			v, ok := p.Before[fmt.Sprintf("SV%d/REMAKE.JSON", k)]
			s.Names[k-1] = fmt.Sprintf("%d.", k)
			if ok {
				s.Names[k-1] = v.Meta["name"].(string)
			}
		}
		s.Names[p.Slot-1] = meta["name"].(string)
		check(strings.HasSuffix(s.Names[p.Slot-1], p.Memo), "六格備註")
		ui.DrawSaveScreen(c, arts[ed], s)
		im, rgb := picture(filepath.Join(out, p.File))
		scale := 1
		if p.High {
			scale = 4
		}
		check(im.Bounds().Dx() == 640*scale && im.Bounds().Dy() == 408*scale, "存檔面板尺寸")
		rect := image.Rect(408*scale, 36*scale, 632*scale, 292*scale)
		expected := rgbaRGB(c.Output(p.High))
		diagnose(bytes.Equal(crop(rgb, 640*scale, rect), crop(expected, 640*scale, rect)), "完整存檔面板、六槽文字及備註 "+p.Tag)
		bad := crop(expected, 640*scale, rect)
		bad[0] ^= 1
		check(!bytes.Equal(crop(rgb, 640*scale, rect), bad), "面板錯誤像素負例")
	}
	_, portrait := picture(filepath.Join(packDir, "F005.png"))
	for _, p := range r.Pairs {
		ed := strings.Split(p.Tag, "-")[0]
		_, before := picture(filepath.Join(out, p.Original))
		_, after := picture(filepath.Join(out, p.Restored))
		check(len(before) == 640*408*3 && len(after) == len(before), "原貌完整畫布")
		ph := p.Phase
		if ph.Error != "" {
			diagnose(false, "整張原貌恢復 "+p.Tag)
			continue
		}
		check(ph.Complete == 640*408 && ph.Excluded == 0, "不排除畫面像素")
		expected := append([]byte(nil), before...)
		if !ph.Identical {
			frames, err := assets.CursorFrames(containers[ed]["DATA1"], assets.CursorMain)
			must(err)
			check(len(frames) == 6 && len(ph.Rect) == 4 && ph.Rect[2] == 8 && ph.Rect[3] == 16, "原版游標相位")
			check(ph.Old >= 0 && ph.Old < 6 && ph.New >= 0 && ph.New < 6 && ph.Background >= 0 && ph.Background < 16, "相位索引")
			for y := 0; y < 16; y++ {
				for x := 0; x < 8; x++ {
					pos := ((ph.Rect[1]+y)*640 + ph.Rect[0] + x) * 3
					for j, n := range []int{ph.Old, ph.New} {
						f := frames[n]
						v := assets.EGAPalette[(byte(ph.Background)&f.Mask.At(x, y))|f.Sprite.At(x, y)]
						if j == 0 {
							check(bytes.Equal(before[pos:pos+3], []byte{v.R, v.G, v.B}), "原游標來源")
						} else {
							copy(expected[pos:pos+3], []byte{v.R, v.G, v.B})
						}
					}
				}
			}
		}
		diagnose(bytes.Equal(expected, after), "整張原貌恢復 "+p.Tag)
		expected[0] ^= 1
		check(!bytes.Equal(expected, after), "游標外錯誤像素負例")
		im, high := picture(filepath.Join(out, p.High))
		check(im.Bounds().Dx() == 2560 && im.Bounds().Dy() == 1632, "高清完整畫布")
		check(bytes.Equal(crop(high, 2560, image.Rect(2144, 464, 2400, 784)), portrait), "原生高清肖像")
	}
	for _, s := range r.Selections {
		_, rgb := picture(filepath.Join(out, s.File))
		x := []int{10, 202, 378}[s.Field] * s.Scale
		for row := 0; row < s.Rows; row++ {
			want := []byte{44, 52, 65}
			if row == s.Expected {
				want = []byte{44, 92, 112}
			}
			pos := ((34+24*row)*s.Scale*640*s.Scale + x) * 3
			check(bytes.Equal(rgb[pos:pos+3], want), "GUI 選取列")
		}
	}
	for _, rt := range r.Roundtrips {
		dir := filepath.Join(out, rt.SaveDir)
		actual := readSnapshot(dir)
		check(reflect.DeepEqual(actual, rt.After), "再存檔實際 bytes")
		key := fmt.Sprintf("SV%d/REMAKE.JSON", rt.Slot)
		check(actual[key].SHA != r.Baselines[rt.Edition][key].SHA, "正常再存確實產生新寫入")
		check(reflect.DeepEqual(functional(actual), functional(r.Baselines[rt.Edition])), "局面與選項功能值保持")
		diagnose(reflect.DeepEqual(canonical(actual), canonical(r.Baselines[rt.Edition])), "完整存檔 bytes "+rt.Tag)
		ss, err := session.Load(dir, rt.Slot, ai.ModeEnhanced)
		must(err)
		mode := ai.ModeEnhanced
		if rt.Slot == 1 {
			mode = ai.Mode(rt.Edition)
		}
		check(ss.Brain.Mode() == mode && string(ss.G.Edition) == rt.Edition, "typed 讀檔版本及 AI")
		if rt.Slot > 1 {
			check(ss.G.Options.AIOrders() == rt.Slot-1, "typed AI 強度")
		}
		check(ss.G.Options.AIMode == string(mode), "typed AI 儲存設定")
		g, err := save.Read(dir, rt.Slot)
		must(err)
		a, b, c, err := g.Tables()
		must(err)
		for i, raw := range [][]byte{a, b, c} {
			stem := []string{"BASEMAS", "BASESTA", "BASEGEN"}[i]
			diagnose(bytes.Equal(raw, data(filepath.Join(dir, fmt.Sprintf("SV%d/%s.SV%d", rt.Slot, stem, rt.Slot)))), "typed 三表完整 round trip "+rt.Tag+stem)
		}
	}
	result := map[string]any{"passed": true, "go_version": runtime.Version(), "receipt_sha256": digest(filepath.Join(out, "receipt.json")), "diagnostics": diagnostics,
		"captures_decoded": len(r.Captures), "normal_closes": len(r.Closes), "save_panels_complete": len(r.Panels), "normal_roundtrips": len(r.Roundtrips),
		"original_restores": len(r.Pairs), "excluded_pixels": 0, "canonical_metadata_only_omits": "saved_at", "negative_pixels_rejected": true,
		"scope": "Linux 正常六槽存讀檔、AI 恢復及完整存檔面板；不表示原版 oracle、音訊、人耳或原生平台已驗"}
	b, err := json.MarshalIndent(result, "", "  ")
	must(err)
	f, err := os.OpenFile(filepath.Join(out, "readback.json"), os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0644)
	must(err)
	_, err = f.Write(append(b, '\n'))
	must(err)
	must(f.Close())
	fmt.Printf("PNG=%d panels=%d roundtrips=%d closes=%d restores=%d\n", len(r.Captures), len(r.Panels), len(r.Roundtrips), len(r.Closes), len(r.Pairs))
}
