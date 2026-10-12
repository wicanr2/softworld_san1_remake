package ui

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"image"
	"image/png"
	"os"
	"path/filepath"

	"github.com/wicanr2/softworld_san1_remake/internal/assets"
	xdraw "golang.org/x/image/draw"
)

type expandedArtManifest struct {
	Schema  int                `json:"schema"`
	Entries []expandedArtEntry `json:"entries"`
}
type expandedArtEntry struct {
	Portrait int    `json:"portrait"`
	File     string `json:"file"`
	SHA256   string `json:"sha256"`
	Width    int    `json:"width"`
	Height   int    `json:"height"`
}

// 新肖像有獨立目錄，不假造原版容器項目或 source_sha256。
func (p *HDPack) loadExpandedPortraits(dir string) {
	b, err := os.ReadFile(filepath.Join(dir, "expanded-portraits.json"))
	if os.IsNotExist(err) {
		return
	}
	warn := func(err error) { p.Warnings = append(p.Warnings, "新增肖像："+err.Error()) }
	if err != nil {
		warn(err)
		return
	}
	var m expandedArtManifest
	if len(b) > 256<<10 {
		warn(fmt.Errorf("目錄過大"))
		return
	}
	if err = json.Unmarshal(b, &m); err != nil {
		warn(err)
		return
	}
	if m.Schema != 1 || len(m.Entries) > 220 {
		warn(fmt.Errorf("目錄版本或數量不符"))
		return
	}
	counts := map[int]int{}
	for _, e := range m.Entries {
		counts[e.Portrait]++
	}
	for _, e := range m.Entries {
		if counts[e.Portrait] != 1 {
			warn(fmt.Errorf("來源 %d 重複", e.Portrait))
			continue
		}
		im, err := loadExpandedArt(dir, e)
		if err != nil {
			warn(err)
			continue
		}
		native := assets.ExpandedPortrait(e.Portrait)
		p.images[hdImageKey(native)] = im
		if key := hdImageKey(native.Mirror()); key != hdImageKey(native) {
			p.images[key] = mirrorRGBA(im)
		}
		p.ExpandedCount++
	}
}

func loadExpandedArt(dir string, e expandedArtEntry) (*image.RGBA, error) {
	bad := func(s string) (*image.RGBA, error) { return nil, fmt.Errorf("來源 %d：%s", e.Portrait, s) }
	if e.Portrait < 0 || e.Portrait > 999 || e.File == "" || filepath.Base(e.File) != e.File || e.Width < 1 || e.Height < 1 || e.Width > 4096 || e.Height > 4096 {
		return bad("識別、路徑或尺寸不符")
	}
	path := filepath.Join(dir, e.File)
	info, err := os.Stat(path)
	if err != nil {
		return nil, err
	}
	if !info.Mode().IsRegular() || info.Size() > 24<<20 {
		return bad("檔案形狀不符")
	}
	b, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	digest := sha256.Sum256(b)
	if hex.EncodeToString(digest[:]) != e.SHA256 {
		return bad("SHA-256 不符")
	}
	config, err := png.DecodeConfig(bytes.NewReader(b))
	if err != nil {
		return nil, err
	}
	if config.Width != e.Width || config.Height != e.Height {
		return bad("PNG 尺寸不符")
	}
	im, err := png.Decode(bytes.NewReader(b))
	if err != nil {
		return nil, err
	}
	out := image.NewRGBA(image.Rect(0, 0, 256, 320))
	xdraw.CatmullRom.Scale(out, out.Bounds(), im, im.Bounds(), xdraw.Src, nil)
	return out, nil
}
