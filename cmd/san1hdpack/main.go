// san1hdpack 依 spec/021 準備私人 B 高清包，不加入發行產物。
package main

import (
	"bytes"
	"crypto/sha256"
	"encoding/json"
	"flag"
	"fmt"
	"image"
	"image/png"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"

	xdraw "golang.org/x/image/draw"

	"github.com/wicanr2/softworld_san1_remake/internal/assets"
	"github.com/wicanr2/softworld_san1_remake/internal/ui"
)

type preparation struct {
	Edition      string `json:"edition"`
	Key          string `json:"key"`
	Input        string `json:"input"`
	InputSHA256  string `json:"input_sha256"`
	SourceSHA256 string `json:"source_sha256"`
	File         string `json:"file"`
	SHA256       string `json:"sha256"`
	Width        int    `json:"width"`
	Height       int    `json:"height"`
}

func main() {
	base := flag.String("root", "", "原版資料目錄")
	plus := flag.String("peer-root", "", "加強版資料目錄")
	input := flag.String("input", "workplace/hd-preview", "已準備的 4× PNG 目錄")
	masters := flag.String("master-dir", "", "已審查的 B 原圖目錄；整圖縮放至 4×")
	revisions := flag.String("master-revisions", "", "指定採用的原圖版次，例如 F020=2,F236=2；其餘為 v1")
	selected := flag.String("assets", "F000,F005,F228,SCG01", "本批已審查的資源鍵，逗號分隔")
	out := flag.String("out", "workplace/hd-assets", "私人輸出目錄")
	flag.Parse()
	if err := prepare(*base, *plus, *input, *masters, *revisions, *selected, *out); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func prepare(base, plus, input, masters, revisions, selected, out string) error {
	if base == "" || plus == "" {
		return fmt.Errorf("需提供兩版來源")
	}
	versions := map[string]string{}
	if revisions != "" {
		if masters == "" {
			return fmt.Errorf("指定原圖版次需要 -master-dir")
		}
		for _, item := range strings.Split(revisions, ",") {
			key, version, ok := strings.Cut(item, "=")
			if !ok || !regexp.MustCompile(`^[1-9][0-9]*$`).MatchString(version) {
				return fmt.Errorf("原圖版次格式不符：%q", item)
			}
			if _, _, err := resource(key); err != nil {
				return err
			}
			if versions[key] != "" {
				return fmt.Errorf("重複原圖版次鍵 %s", key)
			}
			versions[key] = version
		}
	}
	editions := []struct{ name, root string }{{"base", base}, {"plus", plus}}
	containers := map[string]map[string]*assets.Container{}
	for _, ed := range editions {
		containers[ed.name] = map[string]*assets.Container{}
		for _, cn := range []string{"DATA2", "DATA3"} {
			c, err := container(ed.root, cn)
			if err != nil {
				return err
			}
			containers[ed.name][cn] = c
		}
	}
	m := ui.HDManifest{Schema: 1, Style: "b", Scale: 4}
	files := map[string][]byte{}
	var receipt []preparation
	keys := strings.Split(selected, ",")
	seen := map[string]bool{}
	for _, key := range keys {
		cn, name, err := resource(key)
		if err != nil {
			return err
		}
		if seen[key] {
			return fmt.Errorf("重複資源鍵 %s", key)
		}
		seen[key] = true
		originals := map[string][]byte{}
		for _, ed := range editions {
			c := containers[ed.name][cn]
			i, ok := c.ByName(name)
			if !ok {
				return fmt.Errorf("%s/%s/%s 來源缺失", ed.name, cn, name)
			}
			originals[ed.name] = c.Data(i)
		}
		shared := bytes.Equal(originals["base"], originals["plus"])
		for _, ed := range editions {
			source := originals[ed.name]
			original, err := assets.DecodeImage(source)
			if err != nil {
				return fmt.Errorf("%s/%s: %w", ed.name, name, err)
			}
			w, h := original.W*4, original.H*4
			file, prefix := key+".png", "base"
			if !shared {
				file, prefix = ed.name+"-"+file, ed.name
			}
			path := filepath.Join(input, prefix+"-"+key+"-prepared-4x.png")
			if masters != "" {
				version := versions[key]
				if version == "" {
					version = "1"
				}
				master := "hd-b-" + key + "-v" + version + ".png"
				if !shared {
					master = "hd-b-" + ed.name + "-" + key + "-v" + version + ".png"
				}
				path = filepath.Join(masters, master)
			}
			b, err := os.ReadFile(path)
			if err != nil {
				return err
			}
			im, err := png.Decode(bytes.NewReader(b))
			if err != nil {
				return fmt.Errorf("%s: %w", path, err)
			}
			data := b
			if masters != "" {
				bounds := im.Bounds()
				ratioError := bounds.Dx()*original.H - bounds.Dy()*original.W
				if ratioError < 0 {
					ratioError = -ratioError
				}
				if ratioError > max(original.W, original.H) {
					return fmt.Errorf("%s 比例與來源槽不符：%v", path, bounds)
				}
				scaled := image.NewRGBA(image.Rect(0, 0, w, h))
				xdraw.CatmullRom.Scale(scaled, scaled.Bounds(), im, bounds, xdraw.Src, nil)
				var encoded bytes.Buffer
				if err := png.Encode(&encoded, scaled); err != nil {
					return err
				}
				data = encoded.Bytes()
			} else if im.Bounds().Dx() != w || im.Bounds().Dy() != h {
				return fmt.Errorf("%s 尺寸須為 %d×%d", path, w, h)
			}
			files[file] = data
			entry := ui.HDEntry{Edition: ed.name, Container: cn, Name: name,
				SourceSHA256: sum(source), File: file, SHA256: sum(data), Width: w, Height: h}
			m.Entries = append(m.Entries, entry)
			receipt = append(receipt, preparation{Edition: ed.name, Key: cn + "/" + name,
				Input: path, InputSHA256: sum(b), SourceSHA256: entry.SourceSHA256,
				File: file, SHA256: entry.SHA256, Width: w, Height: h})
		}
	}
	for key := range versions {
		if !seen[key] {
			return fmt.Errorf("指定版次的資源 %s 未列入 -assets", key)
		}
	}
	if err := os.MkdirAll(out, 0755); err != nil {
		return err
	}
	for file, b := range files {
		if err := os.WriteFile(filepath.Join(out, file), b, 0644); err != nil {
			return err
		}
	}
	for file, value := range map[string]any{"manifest.json": m, "preparation.json": receipt} {
		b, err := json.MarshalIndent(value, "", "  ")
		if err != nil {
			return err
		}
		if err := os.WriteFile(filepath.Join(out, file), append(b, '\n'), 0644); err != nil {
			return err
		}
	}
	for _, ed := range editions {
		pack, err := ui.LoadHDPack(out, ed.name, containers[ed.name])
		if err != nil {
			return err
		}
		if pack.Count != len(keys) || len(pack.Warnings) != 0 {
			return fmt.Errorf("%s 素材包驗證失敗：%+v", ed.name, pack)
		}
		fmt.Printf("%s: %d 張高清圖驗證通過\n", ed.name, pack.Count)
	}
	return nil
}

var keyPattern = regexp.MustCompile(`^(F[0-9]{3}|SCG[0-9]{2})$`)

func resource(key string) (string, string, error) {
	if keyPattern.MatchString(key) {
		if strings.HasPrefix(key, "F") {
			n, _ := strconv.Atoi(key[1:])
			if n < 256 {
				return "DATA3", key + ".FAC", nil
			}
		} else {
			n, _ := strconv.Atoi(key[3:])
			if n >= 1 && n <= 31 {
				cn := "DATA3"
				if n >= 30 {
					cn = "DATA2"
				}
				return cn, key + ".IMG", nil
			}
		}
	}
	return "", "", fmt.Errorf("不支援資源鍵 %q", key)
}

func container(root, key string) (*assets.Container, error) {
	var b [3][]byte
	for i, ext := range []string{".NAM", ".IDX", ".GRP"} {
		data, err := os.ReadFile(filepath.Join(root, key+ext))
		if err != nil {
			return nil, err
		}
		b[i] = data
	}
	return assets.OpenContainer(b[0], b[1], b[2])
}

func sum(b []byte) string { return fmt.Sprintf("%x", sha256.Sum256(b)) }
