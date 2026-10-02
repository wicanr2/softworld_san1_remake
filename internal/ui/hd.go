package ui

import (
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"image"
	"image/draw"
	"image/png"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	xdraw "golang.org/x/image/draw"

	"github.com/wicanr2/softworld_san1_remake/internal/assets"
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
	images   map[[32]byte]*image.RGBA
	Count    int
	Warnings []string
}

var hdResource = regexp.MustCompile(`^(F[0-9]{3}\.FAC|SCG[0-9]{2}\.IMG)$`)

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
	if m.Schema != 1 || m.Style != "b" || m.Scale != 4 || len(m.Entries) > 574 {
		return nil, fmt.Errorf("HD 素材包規格不符")
	}
	p := &HDPack{images: make(map[[32]byte]*image.RGBA)}
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
		p.images[hdImageKey(im)] = high
		if strings.HasPrefix(e.Name, "F") {
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
	if strings.HasPrefix(e.Name, "F") && e.Name > "F255.FAC" {
		return nil, nil, fmt.Errorf("肖像資源越界")
	}
	if (strings.HasPrefix(e.Name, "F") || e.Name < "SCG30.IMG") && e.Container != "DATA3" {
		return nil, nil, fmt.Errorf("來源容器不符")
	}
	if strings.HasPrefix(e.Name, "SCG") && e.Name >= "SCG30.IMG" && (e.Container != "DATA2" || e.Name > "SCG31.IMG") {
		return nil, nil, fmt.Errorf("場景資源越界")
	}
	c := containers[e.Container]
	if c == nil {
		return nil, nil, fmt.Errorf("缺少來源容器")
	}
	i, ok := c.ByName(e.Name)
	if !ok {
		return nil, nil, fmt.Errorf("來源資源不存在")
	}
	raw := c.Data(i)
	if fmt.Sprintf("%x", sha256.Sum256(raw)) != e.SourceSHA256 {
		return nil, nil, fmt.Errorf("來源雜湊不符")
	}
	im, err := assets.DecodeImage(raw)
	if err != nil {
		return nil, nil, err
	}
	if (strings.HasPrefix(e.Name, "F") && (im.W != 64 || im.H != 80)) ||
		(strings.HasPrefix(e.Name, "SCG") && (im.W != 176 || im.H != 96 || e.Name == "SCG00.IMG")) {
		return nil, nil, fmt.Errorf("來源尺寸不符")
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
}

func (c *Canvas) HighImage(im *assets.Image) *image.RGBA {
	if c.HD == nil || im == nil {
		return nil
	}
	return c.HD.images[hdImageKey(im)]
}

func (c *Canvas) drawHigh(im *assets.Image, x, y int) {
	if high := c.HighImage(im); high != nil {
		c.addHigh(high, image.Rect(x, y, x+im.W, y+im.H), image.Point{})
	}
}

func (c *Canvas) addHigh(im *image.RGBA, rect image.Rectangle, source image.Point) {
	r := rect.Intersect(c.Img.Bounds())
	if r.Empty() {
		return
	}
	source = source.Add(r.Min.Sub(rect.Min).Mul(4))
	c.highOps = append(c.highOps, &highOp{im, r, source, make([]bool, r.Dx()*r.Dy())})
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
	xdraw.NearestNeighbor.Scale(c.highOutput, c.highOutput.Bounds(), c.Img, b, draw.Src, nil)
	for _, op := range c.highOps {
		r := image.Rectangle{Min: op.rect.Min.Mul(4), Max: op.rect.Max.Mul(4)}
		draw.Draw(c.highOutput, r, op.image, op.source, draw.Src)
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

// SearchHighScene 保留尋訪的藍底及原版肖像定位。
func (c *Canvas) SearchHighScene(a *ArtScreen, portrait int) *image.RGBA {
	if a == nil || c.HD == nil {
		return nil
	}
	scene := SearchPanel(a, portrait)
	high := c.HighImage(a.Portrait(portrait))
	if high == nil {
		return nil
	}
	out := image.NewRGBA(image.Rect(0, 0, assets.SceneW*4, assets.SceneH*4))
	xdraw.NearestNeighbor.Scale(out, out.Bounds(), scene.RGBA(), scene.RGBA().Bounds(), draw.Src, nil)
	pt := image.Pt(56*4, 8*4)
	draw.Draw(out, high.Bounds().Add(pt), high, image.Point{}, draw.Src)
	return out
}
