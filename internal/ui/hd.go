package ui

import (
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"image"
	"image/color"
	"image/draw"
	"image/png"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"

	xdraw "golang.org/x/image/draw"

	"github.com/wicanr2/softworld_san1_remake/internal/assets"
	"github.com/wicanr2/softworld_san1_remake/internal/font"
	"github.com/wicanr2/softworld_san1_remake/internal/game"
)

// HDManifest 是 spec/021 的顯示素材包；不參與遊戲或存檔資料。
type HDManifest struct {
	Schema  int       `json:"schema"`
	Style   string    `json:"style"`
	Scale   int       `json:"scale"`
	Entries []HDEntry `json:"entries"`
}

type HDEntry struct {
	Edition      string `json:"edition"`
	Container    string `json:"container"`
	Name         string `json:"name"`
	SourceSHA256 string `json:"source_sha256"`
	File         string `json:"file"`
	SHA256       string `json:"sha256"`
	Width        int    `json:"width"`
	Height       int    `json:"height"`
}

type HDPack struct {
	creditFigures    map[[32]byte][]image.Rectangle
	marchCache       map[hdMarchKey]*image.RGBA
	images           map[[32]byte]*image.RGBA
	flags            map[[32]byte]highFlag
	flagText         map[highFlagTextKey]*image.RGBA
	panels           map[string]hdPanel
	sidePanels       map[hdSideKey]map[byte]string
	sideBorders      map[hdSideKey]string
	panelCache       map[hdPanelSize]*image.RGBA
	panelCacheBytes  int
	battleBackground *image.RGBA
	Count            int
	Warnings         []string
}

var hdResource = regexp.MustCompile(`^(F[0-9]{3}\.FAC|SCG[0-9]{2}\.IMG|WEATHER[0-2]\.IMG|CVSC(?:0[0-9]|1[0-5])\.IMG|CMARK[LR]\.IMG|SANT(?:[LR]|BB|BM[12]|BS)\.IMG|TITL[0-3]\.IMG|EICON\.GRP#(?:0[0-9]|1[0-4]|3[2-5])|WFLAG[DA][01][0-4]\.IMG|MENU(?:0[AB]|[1-3])\.IMG|MAINMAP[12378]\.IMG|8x8PAT0\.IMG|FBR[A-D][0-3]\.IMG|PANEL\.SIDE#(?:A[13]|B3|C[157]|D[12]|E1)|PANEL\.BEVEL#[13]|ENDO[0-3]\.IMG|REC1[01][LR]\.IMG|UPR(?:[01][0-9]|2[01])\.IMG)$`)

type highFlag struct {
	image    *image.RGBA
	label    rune
	selected bool
}

type highFlagTextKey struct {
	flag highFlag
	face *font.Face
}

// LoadHDPack 逐項驗證玩家自己的來源，錯項回退原圖。
func LoadHDPack(dir, edition string, containers map[string]*assets.Container) (*HDPack, error) {
	f, err := os.Open(filepath.Join(dir, "manifest.json"))
	if err != nil {
		return nil, err
	}
	defer f.Close()
	info, err := f.Stat()
	if err != nil {
		return nil, err
	}
	if info.Size() > 2<<20 {
		return nil, fmt.Errorf("HD manifest 過大")
	}
	var m HDManifest
	if err := json.NewDecoder(f).Decode(&m); err != nil {
		return nil, err
	}
	if m.Schema != 1 || m.Style != "b" || m.Scale != 4 || len(m.Entries) > 850 {
		return nil, fmt.Errorf("HD 素材包規格不符")
	}
	p := &HDPack{creditFigures: map[[32]byte][]image.Rectangle{}, images: make(map[[32]byte]*image.RGBA), flags: make(map[[32]byte]highFlag), flagText: make(map[highFlagTextKey]*image.RGBA), panels: map[string]hdPanel{}, sidePanels: map[hdSideKey]map[byte]string{}, sideBorders: map[hdSideKey]string{}, panelCache: map[hdPanelSize]*image.RGBA{}}
	counts := map[string]int{}
	for _, e := range m.Entries {
		counts[e.Edition+"/"+e.Container+"/"+e.Name]++
	}
	for _, e := range m.Entries {
		if e.Edition != edition {
			continue
		}
		key := e.Container + "/" + e.Name
		if counts[e.Edition+"/"+key] != 1 {
			p.Warnings = append(p.Warnings, key+" 重複")
			continue
		}
		im, high, err := loadHDEntry(dir, e, containers)
		if err != nil {
			p.Warnings = append(p.Warnings, key+": "+err.Error())
			continue
		}
		if strings.HasPrefix(e.Name, "PANEL.") {
			p.registerPanel(containers[e.Container], e, high)
		} else {
			p.images[hdImageKey(im)] = high
		}
		if strings.HasPrefix(e.Name, "UPR") {
			p.creditFigures[hdImageKey(im)] = hdCreditFigures(e.Name, im.W)
		}
		if e.Name == assets.BattleBGTile {
			p.battleBackground = tileBattleBackground(high)
		}
		if strings.HasPrefix(e.Name, "WFLAG") {
			label := []rune("帥先左右後")[int(e.Name[7]-'0')]
			p.flags[hdImageKey(im)] = highFlag{image: high, label: label}
			p.flags[hdImageKey(im.Complement())] = highFlag{image: high, label: label, selected: true}
		}
		if strings.HasSuffix(e.Name, ".FAC") {
			if key := hdImageKey(im.Mirror()); key != hdImageKey(im) {
				p.images[key] = mirrorRGBA(high)
			}
		}
		p.Count++
	}
	return p, nil
}

func loadHDEntry(dir string, e HDEntry, containers map[string]*assets.Container) (*assets.Image, *image.RGBA, error) {
	if !hdResource.MatchString(e.Name) {
		return nil, nil, fmt.Errorf("未知資源鍵")
	}
	if strings.HasSuffix(e.Name, ".FAC") && e.Name > "F255.FAC" {
		return nil, nil, fmt.Errorf("肖像資源越界")
	}
	if (strings.HasSuffix(e.Name, ".FAC") || strings.HasPrefix(e.Name, "MENU") || strings.HasPrefix(e.Name, "CVSC") || strings.HasPrefix(e.Name, "MAINMAP") || (strings.HasPrefix(e.Name, "SCG") && e.Name < "SCG30.IMG")) && e.Container != "DATA3" {
		return nil, nil, fmt.Errorf("來源容器不符")
	}
	credit := strings.HasPrefix(e.Name, "ENDO") || strings.HasPrefix(e.Name, "REC") || strings.HasPrefix(e.Name, "UPR")
	if credit && e.Container != "DATA2" {
		return nil, nil, fmt.Errorf("製作群來源容器不符")
	}
	march := strings.HasPrefix(e.Name, "CVSC")
	terrain := strings.HasPrefix(e.Name, "EICON.")
	flag := strings.HasPrefix(e.Name, "WFLAG")
	panel := strings.HasPrefix(e.Name, "PANEL.")
	frame := strings.HasPrefix(e.Name, "FBR")
	menu := strings.HasPrefix(e.Name, "MENU")
	menuTitle := strings.HasPrefix(e.Name, "MENU0")
	decoration := strings.HasPrefix(e.Name, "MAINMAP")
	pattern := e.Name == assets.BattleBGTile
	openingArt := strings.HasPrefix(e.Name, "CMARK") || strings.HasPrefix(e.Name, "SANT") || strings.HasPrefix(e.Name, "TITL")
	if (strings.HasPrefix(e.Name, "WEATHER") || terrain || flag || panel || frame || pattern || openingArt) && e.Container != "DATA1" {
		return nil, nil, fmt.Errorf("來源容器不符")
	}
	if strings.HasPrefix(e.Name, "SCG") && e.Name >= "SCG30.IMG" && (e.Container != "DATA2" || e.Name > "SCG31.IMG") {
		return nil, nil, fmt.Errorf("場景資源越界")
	}
	c := containers[e.Container]
	if c == nil {
		return nil, nil, fmt.Errorf("缺少來源容器")
	}
	var im *assets.Image
	var err error
	if panel {
		var raw []byte
		im, raw, err = hdPanelSource(c, e.Name)
		if err != nil {
			return nil, nil, err
		}
		if panelRecipeHash(raw) != e.SourceSHA256 {
			return nil, nil, fmt.Errorf("來源雜湊不符")
		}
	} else {
		name, fragment, _ := strings.Cut(e.Name, "#")
		i, ok := c.ByName(name)
		if !ok {
			return nil, nil, fmt.Errorf("來源資源不存在")
		}
		raw := c.Data(i)
		if terrain {
			// 實際檔案形狀由正式解碼器驗證，不能只憑來源雜湊或版本宣稱。
			if _, err := assets.BattleTiles(c); err != nil {
				return nil, nil, err
			}
			n, err := strconv.Atoi(fragment)
			if err != nil || n < 0 || (n > assets.SkirmishMaxTerrain && (n < 32 || n > 35)) {
				return nil, nil, fmt.Errorf("地形資源越界")
			}
			const stride = assets.ImageHeader + assets.TileW/8*assets.TileH*4
			raw = raw[n*stride : (n+1)*stride]
		}
		if fmt.Sprintf("%x", sha256.Sum256(raw)) != e.SourceSHA256 {
			return nil, nil, fmt.Errorf("來源雜湊不符")
		}
		im, err = assets.DecodeImage(raw)
		if err != nil {
			return nil, nil, err
		}
	}
	if (strings.HasSuffix(e.Name, ".FAC") && (im.W != 64 || im.H != 80)) ||
		(strings.HasPrefix(e.Name, "SCG") && (im.W != 176 || im.H != 96 || e.Name == "SCG00.IMG")) ||
		(strings.HasPrefix(e.Name, "WEATHER") && (im.W != 32 || im.H != 32)) ||
		(march && (im.W != 32 || im.H != 32)) ||
		(flag && (im.W != assets.FlagW || im.H != assets.FlagH)) ||
		(terrain && (im.W != assets.TileW || im.H != assets.TileH)) {
		return nil, nil, fmt.Errorf("來源尺寸不符")
	}
	if credit {
		w, h := 160, assets.CreditBackdropH
		if strings.HasPrefix(e.Name, "REC") {
			w = 320
		}
		if strings.HasPrefix(e.Name, "UPR") {
			n, _ := strconv.Atoi(e.Name[3:5])
			w, h = hdCreditWidths[n], assets.CreditLineH
		}
		if im.W != w || im.H != h {
			return nil, nil, fmt.Errorf("製作群來源尺寸不符")
		}
	}
	if decoration || pattern {
		w, h := 640, 36
		switch e.Name {
		case "MAINMAP3.IMG":
			w, h = 72, 336
		case "MAINMAP7.IMG":
			w, h = 8, 336
		case assets.BattleBGTile:
			w, h = 8, 8
		}
		if im.W != w || im.H != h {
			return nil, nil, fmt.Errorf("外框或底紋來源尺寸不符")
		}
	}
	if openingArt {
		sizes := map[string][2]int{"CMARKL.IMG": {320, 290}, "CMARKR.IMG": {320, 290}, "SANTL.IMG": {320, 295}, "SANTR.IMG": {320, 295}, "SANTBB.IMG": {40, 64}, "SANTBM1.IMG": {24, 32}, "SANTBM2.IMG": {16, 20}, "SANTBS.IMG": {16, 16}, "TITL0.IMG": {160, 400}, "TITL1.IMG": {160, 400}, "TITL2.IMG": {160, 400}, "TITL3.IMG": {160, 400}}
		want := sizes[e.Name]
		if im.W != want[0] || im.H != want[1] {
			return nil, nil, fmt.Errorf("片頭來源尺寸不符")
		}
		if strings.HasPrefix(e.Name, "SANTB") {
			for _, v := range im.Pix {
				if v != 0 && v != 15 {
					return nil, nil, fmt.Errorf("船隊遮罩色號不符")
				}
			}
		}
	}
	if menu {
		sizes := map[string][2]int{"MENU0A.IMG": {280, 180}, "MENU0B.IMG": {288, 180}, "MENU1.IMG": {96, 151}, "MENU2.IMG": {200, 46}, "MENU3.IMG": {40, 41}}
		want := sizes[e.Name]
		if im.W != want[0] || im.H != want[1] {
			return nil, nil, fmt.Errorf("主選單素材尺寸不符")
		}
		if !menuTitle && hdMenuFrameBounds(im).Empty() {
			return nil, nil, fmt.Errorf("主選單素材缺少框面")
		}
		for _, code := range im.Pix {
			if !menuTitle && code != 0 && code != 3 && code != 9 && code != 15 {
				return nil, nil, fmt.Errorf("主選單素材色號不符")
			}
		}
	}
	if frame {
		w, h := 80, 8
		if e.Name[4] >= '2' {
			w, h = 8, 80
		}
		if im.W != w || im.H != h {
			return nil, nil, fmt.Errorf("肖像框尺寸不符")
		}
	}
	if flag {
		for _, code := range im.Pix {
			a, b := assets.EGAPalette[code], assets.EGAPalette[code^15]
			if a.R^255 != b.R || a.G^255 != b.G || a.B^255 != b.B {
				return nil, nil, fmt.Errorf("旗幟反白色號不符")
			}
		}
	}
	if e.Width != im.W*4 || e.Height != im.H*4 {
		return nil, nil, fmt.Errorf("高清尺寸不符")
	}
	if !filepath.IsLocal(e.File) {
		return nil, nil, fmt.Errorf("素材路徑不在包內")
	}
	base, err := filepath.EvalSymlinks(dir)
	if err != nil {
		return nil, nil, err
	}
	path, err := filepath.EvalSymlinks(filepath.Join(dir, e.File))
	if err != nil {
		return nil, nil, err
	}
	rel, err := filepath.Rel(base, path)
	if err != nil || !filepath.IsLocal(rel) {
		return nil, nil, fmt.Errorf("素材連結越出包目錄")
	}
	file, err := os.Open(path)
	if err != nil {
		return nil, nil, err
	}
	defer file.Close()
	stat, err := file.Stat()
	if err != nil || !stat.Mode().IsRegular() || stat.Size() > 16<<20 {
		return nil, nil, fmt.Errorf("素材檔案形態或大小不符")
	}
	config, err := png.DecodeConfig(file)
	if err != nil || config.Width != e.Width || config.Height != e.Height {
		return nil, nil, fmt.Errorf("PNG 表頭尺寸不符")
	}
	if _, err := file.Seek(0, 0); err != nil {
		return nil, nil, err
	}
	h := sha256.New()
	if _, err := io.Copy(h, file); err != nil {
		return nil, nil, err
	}
	if hex.EncodeToString(h.Sum(nil)) != e.SHA256 {
		return nil, nil, fmt.Errorf("PNG 雜湊不符")
	}
	if _, err := file.Seek(0, 0); err != nil {
		return nil, nil, err
	}
	decoded, err := png.Decode(file)
	if err != nil {
		return nil, nil, err
	}
	high := image.NewRGBA(image.Rect(0, 0, e.Width, e.Height))
	draw.Draw(high, high.Bounds(), decoded, decoded.Bounds().Min, draw.Src)
	if terrain || flag || panel || frame || menu || decoration || pattern || march || openingArt || (credit && !strings.HasPrefix(e.Name, "UPR")) {
		for i := 3; i < len(high.Pix); i += 4 {
			if high.Pix[i] != 255 {
				return nil, nil, fmt.Errorf("地形、旗幟、面板、框材與底紋必須不透明")
			}
		}
	}
	if menuTitle {
		if err := validateTitleInk(im, high, e.Name); err != nil {
			return nil, nil, err
		}
	}
	if strings.HasPrefix(e.Name, "UPR") {
		if err := validateCreditText(im, high, e.Name); err != nil {
			return nil, nil, err
		}
	}
	return im, high, nil
}

func hdImageKey(im *assets.Image) [32]byte {
	h := sha256.New()
	var wh [8]byte
	binary.LittleEndian.PutUint32(wh[:4], uint32(im.W))
	binary.LittleEndian.PutUint32(wh[4:], uint32(im.H))
	h.Write(wh[:])
	h.Write(im.Pix)
	var key [32]byte
	copy(key[:], h.Sum(nil))
	return key
}

func mirrorRGBA(src *image.RGBA) *image.RGBA {
	out := image.NewRGBA(src.Bounds())
	for y := 0; y < out.Bounds().Dy(); y++ {
		for x := 0; x < out.Bounds().Dx(); x++ {
			out.SetRGBA(x, y, src.RGBAAt(out.Bounds().Dx()-1-x, y))
		}
	}
	return out
}

type highOp struct {
	image   *image.RGBA
	rect    image.Rectangle
	source  image.Point // 高清像素座標
	covered []bool      // 後畫的原 UI 具有覆蓋權
	over    bool        // 行軍透明空隙保留先畫圖層
}

func (c *Canvas) HighImage(im *assets.Image) *image.RGBA {
	if c.HD == nil || im == nil {
		return nil
	}
	key := hdImageKey(im)
	if flag, ok := c.HD.flags[key]; ok {
		return c.highFlagImage(flag)
	}
	return c.HD.images[key]
}

// highFlagImage 的隊伍字走目前字型，原色與反白分開快取。
func (c *Canvas) highFlagImage(flag highFlag) *image.RGBA {
	if c.face == nil {
		return nil
	}
	glyph, ok := c.face.Glyph(flag.label)
	if !ok || glyph.W != 16 || glyph.H != 16 {
		return nil
	}
	key := highFlagTextKey{flag: flag, face: c.face}
	if cached := c.HD.flagText[key]; cached != nil {
		return cached
	}
	out := image.NewRGBA(flag.image.Bounds())
	draw.Draw(out, out.Bounds(), flag.image, flag.image.Bounds().Min, draw.Src)
	for y := 0; y < glyph.H; y++ {
		for x := 0; x < glyph.W; x++ {
			if glyph.At(x, y) {
				draw.Draw(out, image.Rect(10+x*3, 6+y*3, 13+x*3, 9+y*3), image.Black, image.Point{}, draw.Src)
			}
		}
	}
	if flag.selected {
		for i := 0; i < len(out.Pix); i += 4 {
			out.Pix[i] ^= 255
			out.Pix[i+1] ^= 255
			out.Pix[i+2] ^= 255
		}
	}
	c.HD.flagText[key] = out
	return out
}

func (c *Canvas) drawHigh(im *assets.Image, x, y int) {
	if high := c.HighImage(im); high != nil {
		c.addHigh(high, image.Rect(x, y, x+im.W, y+im.H), image.Point{})
	}
}

// drawHighField 先保留該格的原圖覆蓋權，再疊可用高清圖；缺子圖也能蓋住主圖。
func (c *Canvas) drawHighField(tiles []*assets.Image, field []byte, max int) {
	assets.ForEachFieldTile(tiles, field, max, func(im *assets.Image, x, y int) {
		c.trackRect(image.Rect(x, y, x+im.W, y+im.H))
		c.drawHigh(im, x, y)
	})
}

// indexedCoverage 用同一套索引繪圖操作記錄前景，不比較來源與前景的顏色。
func (c *Canvas) indexedCoverage() *assets.Image {
	b := c.Img.Bounds()
	if c.highCoverage == nil || c.highCoverage.W != b.Dx() || c.highCoverage.H != b.Dy() {
		c.highCoverage = &assets.Image{W: b.Dx(), H: b.Dy(), Pix: make([]byte, b.Dx()*b.Dy())}
	}
	for i := range c.highCoverage.Pix {
		c.highCoverage.Pix[i] = 255 // 所有正式索引圖只使用 0–15。
	}
	return c.highCoverage
}

func (c *Canvas) coverIndexed(mask *assets.Image) {
	for _, op := range c.highOps {
		for y := op.rect.Min.Y; y < op.rect.Max.Y; y++ {
			for x := op.rect.Min.X; x < op.rect.Max.X; x++ {
				if mask.At(x, y) != 255 {
					op.covered[(y-op.rect.Min.Y)*op.rect.Dx()+x-op.rect.Min.X] = true
				}
			}
		}
	}
}

func (c *Canvas) addHigh(im *image.RGBA, rect image.Rectangle, source image.Point) {
	r := rect.Intersect(c.Img.Bounds())
	if r.Empty() {
		return
	}
	source = source.Add(r.Min.Sub(rect.Min).Mul(4))
	c.highOps = append(c.highOps, &highOp{image: im, rect: r, source: source, covered: make([]bool, r.Dx()*r.Dy())})
}

func (c *Canvas) trackPixel(x, y int) {
	for _, op := range c.highOps {
		if image.Pt(x, y).In(op.rect) {
			op.covered[(y-op.rect.Min.Y)*op.rect.Dx()+x-op.rect.Min.X] = true
		}
	}
}

func (c *Canvas) trackRect(r image.Rectangle) {
	for _, op := range c.highOps {
		sub := r.Intersect(op.rect)
		for y := sub.Min.Y; y < sub.Max.Y; y++ {
			for x := sub.Min.X; x < sub.Max.X; x++ {
				op.covered[(y-op.rect.Min.Y)*op.rect.Dx()+x-op.rect.Min.X] = true
			}
		}
	}
}

func (c *Canvas) drawRGBA(r image.Rectangle, src image.Image, pt image.Point) {
	draw.Draw(c.Img, r, src, pt, draw.Src)
	if c.Img.Bounds().In(r) {
		c.highOps = nil
	} else {
		c.trackRect(r)
	}
}

// Output 保留原畫布；高清合成依繪製順序保留前景。
func (c *Canvas) Output(high bool) *image.RGBA {
	if !high || c.HD == nil {
		return c.Img
	}
	b := c.Img.Bounds()
	if c.highOutput == nil {
		c.highOutput = image.NewRGBA(image.Rect(0, 0, b.Dx()*4, b.Dy()*4))
	}
	scaleRGBA4(c.highOutput, c.Img)
	for _, op := range c.highOps {
		r := image.Rectangle{Min: op.rect.Min.Mul(4), Max: op.rect.Max.Mul(4)}
		mode := draw.Src
		if op.over {
			mode = draw.Over
		}
		draw.Draw(c.highOutput, r, op.image, op.source, mode)
		for y := op.rect.Min.Y; y < op.rect.Max.Y; y++ {
			for x := op.rect.Min.X; x < op.rect.Max.X; x++ {
				if !op.covered[(y-op.rect.Min.Y)*op.rect.Dx()+x-op.rect.Min.X] {
					continue
				}
				col := c.Img.RGBAAt(x, y)
				for sy := 0; sy < 4; sy++ {
					for sx := 0; sx < 4; sx++ {
						c.highOutput.SetRGBA(x*4+sx, y*4+sy, col)
					}
				}
			}
		}
	}
	return c.highOutput
}

// scaleRGBA4 將每個 RGBA 像素複製為 4×4；目的尺寸須為來源的四倍。
func scaleRGBA4(dst, src *image.RGBA) {
	b, d := src.Bounds(), dst.Bounds()
	rowBytes := b.Dx() * 16
	for y := 0; y < b.Dy(); y++ {
		so := src.PixOffset(b.Min.X, b.Min.Y+y)
		source := src.Pix[so : so+b.Dx()*4]
		do := dst.PixOffset(d.Min.X, d.Min.Y+y*4)
		row := dst.Pix[do : do+rowBytes]
		for x := 0; x < len(source); x += 4 {
			i := x * 4
			copy(row[i:i+4], source[x:x+4])
			copy(row[i+4:i+8], row[i:i+4])
			copy(row[i+8:i+16], row[i:i+8])
		}
		for sy := 1; sy < 4; sy++ {
			off := do + sy*dst.Stride
			copy(dst.Pix[off:off+rowBytes], row)
		}
	}
}

// SearchHighScene 裁取完成畫面的同一塊藍底紙，避免拉幕結尾更換紋理。
func (c *Canvas) SearchHighScene(a *ArtScreen, portrait int) *image.RGBA {
	if a == nil || c.HD == nil {
		return nil
	}
	pn := assets.MainPanels()[0]
	r := clearPanelRects(pn.X, pn.Y, pn.X+pn.W-1, pn.Y+pn.H-1)[1]
	paper := c.highPanel("PANEL.BEVEL#1", r, true)
	face := a.Portrait(portrait)
	high := c.HighImage(face)
	if paper == nil && high == nil {
		return nil
	}
	out := image.NewRGBA(image.Rect(0, 0, assets.SceneW*4, assets.SceneH*4))
	if paper != nil {
		origin := image.Pt(assets.SceneMainX, assets.SceneMainY).Sub(r.Min).Mul(4)
		draw.Draw(out, out.Bounds(), paper, origin, draw.Src)
	} else {
		draw.Draw(out, out.Bounds(), image.NewUniform(assets.EGAPalette[1]), image.Point{}, draw.Src)
	}
	pt := image.Pt(game.SearchFaceX-assets.SceneMainX, game.SearchFaceY-assets.SceneMainY).Mul(4)
	if high != nil {
		draw.Draw(out, high.Bounds().Add(pt), high, image.Point{}, draw.Src)
	} else if face != nil {
		original := face.RGBA()
		xdraw.NearestNeighbor.Scale(out, image.Rect(0, 0, face.W*4, face.H*4).Add(pt), original, original.Bounds(), draw.Src, nil)
	}
	return out
}

// hdMenuFrameBounds 由原始高光及青色框面量出可重繪範圍。
func hdMenuFrameBounds(im *assets.Image) image.Rectangle {
	r := image.Rectangle{Min: image.Pt(im.W, im.H)}
	for y := 0; y < im.H; y++ {
		for x := 0; x < im.W; x++ {
			if p := im.At(x, y); p == 3 || p == 15 {
				r.Min.X = min(r.Min.X, x)
				r.Min.Y = min(r.Min.Y, y)
				r.Max.X = max(r.Max.X, x+1)
				r.Max.Y = max(r.Max.Y, y+1)
			}
		}
	}
	return r
}

// drawHighMenu 保留框外背景及投影，框內黑色線條與框面一併重繪。
func (c *Canvas) drawHighMenu(im *assets.Image, x, y int) {
	high := c.HighImage(im)
	if high == nil {
		return
	}
	before := len(c.highOps)
	c.addHigh(high, image.Rect(x, y, x+im.W, y+im.H), image.Point{})
	if len(c.highOps) == before {
		return
	}
	op := c.highOps[len(c.highOps)-1]
	frame := hdMenuFrameBounds(im).Add(image.Pt(x, y))
	for py := op.rect.Min.Y; py < op.rect.Max.Y; py++ {
		for px := op.rect.Min.X; px < op.rect.Max.X; px++ {
			if !image.Pt(px, py).In(frame) {
				op.covered[(py-op.rect.Min.Y)*op.rect.Dx()+px-op.rect.Min.X] = true
			}
		}
	}
}

// titleInk 保留來源的書法格點，交錯填色改為 B 的象牙白與青黑投影。
func titleInk(im *assets.Image, name string, x, y int) (color.RGBA, bool) {
	rects := [2]image.Rectangle{image.Rect(56, 40, 168, 134), image.Rect(184, 40, 264, 134)}
	if name == "MENU0B.IMG" {
		rects = [2]image.Rectangle{image.Rect(0, 40, 104, 134), image.Rect(104, 40, 208, 134)}
	}
	if !image.Pt(x, y).In(rects[0]) && !image.Pt(x, y).In(rects[1]) {
		return color.RGBA{}, false
	}
	ink := color.RGBA{238, 242, 226, 255}
	switch im.At(x, y) {
	case 13:
		return ink, true
	case 15:
		return color.RGBA{255, 255, 242, 255}, true
	case 0:
		return color.RGBA{18, 36, 38, 255}, true
	case 3:
		neighbours := 0
		for _, d := range [...]image.Point{{-1, 0}, {1, 0}, {0, -1}, {0, 1}} {
			if im.At(x+d.X, y+d.Y) == 13 {
				neighbours++
			}
		}
		return ink, neighbours >= 2
	}
	return color.RGBA{}, false
}

// validateTitleInk 拒絕改寫已知書法的素材，錯項仍走原圖回退。
func validateTitleInk(im *assets.Image, high *image.RGBA, name string) error {
	glyphs := 0
	for y := 0; y < im.H; y++ {
		for x := 0; x < im.W; x++ {
			expected, protected := titleInk(im, name, x, y)
			if im.At(x, y) == 13 {
				glyphs++
				if !protected {
					return fmt.Errorf("書法來源超出保護範圍")
				}
			}
			if !protected {
				continue
			}
			for dy := 0; dy < 4; dy++ {
				for dx := 0; dx < 4; dx++ {
					if high.RGBAAt(x*4+dx, y*4+dy) != expected {
						return fmt.Errorf("書法筆畫或填色不符")
					}
				}
			}
		}
	}
	if glyphs == 0 {
		return fmt.Errorf("書法來源缺少字面")
	}
	return nil
}
