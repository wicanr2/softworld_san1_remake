//go:build ignore

// 私人實際素材的完整圖層檢查；不是正常玩家路徑或原版 oracle。
// 在 Docker 內以 go run ./tools/hd-wipe-check.go -root ... -pack ... -out ... 執行。
package main

import (
	"bytes"
	"crypto/sha256"
	"encoding/json"
	"flag"
	"fmt"
	"image"
	"image/color"
	"image/draw"
	"os"
	"path/filepath"

	"github.com/wicanr2/softworld_san1_remake/internal/assets"
	"github.com/wicanr2/softworld_san1_remake/internal/ui"
)

func sum(b []byte) string { return fmt.Sprintf("%x", sha256.Sum256(b)) }

func read(path string) []byte {
	b, err := os.ReadFile(path)
	if err != nil {
		panic(err)
	}
	return b
}

func main() {
	root := flag.String("root", "", "原版資料目錄")
	packDir := flag.String("pack", "", "私人高清包")
	edition := flag.String("edition", "base", "base／plus")
	out := flag.String("out", "", "既有私人目錄內的收據")
	flag.Parse()
	if *root == "" || *packDir == "" || *out == "" || (*edition != "base" && *edition != "plus") {
		panic("參數不符")
	}
	inputs := map[string]string{}
	containers := map[string]*assets.Container{}
	for _, name := range []string{"DATA2", "DATA3"} {
		var b [3][]byte
		for i, ext := range []string{".NAM", ".IDX", ".GRP"} {
			b[i] = read(filepath.Join(*root, name+ext))
			inputs[name+ext] = sum(b[i])
		}
		c, err := assets.OpenContainer(b[0], b[1], b[2])
		if err != nil {
			panic(err)
		}
		containers[name] = c
	}
	manifestBytes := read(filepath.Join(*packDir, "manifest.json"))
	var manifest ui.HDManifest
	if err := json.Unmarshal(manifestBytes, &manifest); err != nil {
		panic(err)
	}
	pack, err := ui.LoadHDPack(*packDir, *edition, containers)
	if err != nil || pack.Count == 0 || len(pack.Warnings) != 0 {
		panic(fmt.Sprintf("素材包不符 %v %v", pack, err))
	}
	background := color.RGBA{17, 31, 53, 255}
	steps := []map[string]any{}
	for _, e := range manifest.Entries {
		if e.Edition != *edition || len(e.Name) < 3 || e.Name[:3] != "SCG" {
			continue
		}
		container := containers[e.Container]
		i, ok := container.ByName(e.Name)
		if !ok {
			panic(e.Name)
		}
		source, err := assets.DecodeImage(container.Data(i))
		if err != nil || source.W != 176 || source.H != 96 {
			panic("來源尺寸不符")
		}
		for kind := 0; kind < 4; kind++ {
			c := ui.NewCanvasPx(200, 110, nil)
			c.HD = pack
			c.Fill(background)
			high := c.HighImage(source)
			if high == nil || high.Bounds() != image.Rect(0, 0, 704, 384) {
				panic("正式圖層缺圖")
			}
			wipe := ui.NewSceneWipe(c, source, ui.WipeKind(kind), 8, 8)
			total := 24
			if kind >= 2 {
				total = 22
			}
			if wipe.Steps() != total {
				panic("步數不符")
			}
			for n := 1; n <= total; n++ {
				if !wipe.Advance(c.Img) {
					panic("提早結束")
				}
				// 幾何取自 spec/010 §3，未使用實作的 Reveal／Source 作預期。
				dst := image.Rect(8, 8, 184, 104)
				src := image.Point{}
				switch kind {
				case 0:
					dst.Max.Y = 8 + n*4
					src.Y = 96 - n*4
				case 1:
					dst.Min.Y = 104 - n*4
				case 2:
					dst.Max.X = 8 + n*8
					src.X = 176 - n*8
				case 3:
					dst.Min.X = 184 - n*8
				}
				nativeRect := image.Rect(dst.Min.X*4, dst.Min.Y*4, dst.Max.X*4, dst.Max.Y*4)
				want := image.NewRGBA(image.Rect(0, 0, 800, 440))
				draw.Draw(want, want.Bounds(), image.NewUniform(background), image.Point{}, draw.Src)
				draw.Draw(want, nativeRect, high, src.Mul(4), draw.Src)
				got := c.Output(true)
				if !bytes.Equal(want.Pix, got.Pix) {
					panic(fmt.Sprintf("高清整幀不符 %s 方向%d 步%d", e.Name, kind, n))
				}
				cpu := image.NewRGBA(c.Img.Bounds())
				draw.Draw(cpu, cpu.Bounds(), image.NewUniform(background), image.Point{}, draw.Src)
				for y := dst.Min.Y; y < dst.Max.Y; y++ {
					for x := dst.Min.X; x < dst.Max.X; x++ {
						sx, sy := src.X+x-dst.Min.X, src.Y+y-dst.Min.Y
						cpu.SetRGBA(x, y, assets.EGAPalette[source.Pix[sy*176+sx]&15])
					}
				}
				if c.Output(false) != c.Img || !bytes.Equal(cpu.Pix, c.Img.Pix) {
					panic("高清改寫原貌畫布")
				}
				steps = append(steps, map[string]any{"key": e.Container + "/" + e.Name, "kind": kind, "step": n,
					"destination": nativeRect, "source": src.Mul(4), "rgba_sha256": sum(got.Pix),
					"source_sha256": e.SourceSHA256, "image_sha256": e.SHA256})
			}
			if wipe.Advance(c.Img) {
				panic("走完後仍搬移")
			}
		}
	}
	if len(steps) == 0 {
		panic("沒有場景檢查")
	}
	report := map[string]any{"scope": "實際高清素材、四方向全部步數、完整圖層與塊外像素；不是正常 GUI 或原版 oracle",
		"edition": *edition, "pack_sha256": sum(manifestBytes), "input_sha256": inputs,
		"tool_sha256": sum(read("tools/hd-wipe-check.go")), "passed": true, "steps": steps}
	b, err := json.MarshalIndent(report, "", "  ")
	if err != nil {
		panic(err)
	}
	if err := os.WriteFile(*out, append(b, '\n'), 0644); err != nil {
		panic(err)
	}
	fmt.Printf("%s：%d 個完整中間幀與原貌畫布通過；SHA-256 %s\n", *edition, len(steps), sum(append(b, '\n')))
}
