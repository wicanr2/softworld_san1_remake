//go:build ignore

// 已知宣戰三段的完整 PCM 參考。原版素材與輸出只留本機。
package main

import (
	"crypto/sha256"
	"encoding/binary"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"path/filepath"

	"github.com/wicanr2/softworld_san1_remake/internal/assets"
	"github.com/wicanr2/softworld_san1_remake/internal/game"
	"github.com/wicanr2/softworld_san1_remake/internal/speaker"
	"github.com/wicanr2/softworld_san1_remake/internal/state"
)

func container(root, name string) *assets.Container {
	var raw [3][]byte
	for i, ext := range []string{".NAM", ".IDX", ".GRP"} {
		b, err := os.ReadFile(filepath.Join(root, name+ext))
		if err != nil {
			panic(err)
		}
		raw[i] = b
	}
	c, err := assets.OpenContainer(raw[0], raw[1], raw[2])
	if err != nil {
		panic(err)
	}
	return c
}

func main() {
	root := flag.String("root", "/orig", "原始兩版資料的父目錄")
	out := flag.String("out", "", "新輸出目錄，不覆寫")
	flag.Parse()
	if *out == "" {
		panic("需要 -out")
	}
	if err := os.Mkdir(*out, 0755); err != nil {
		panic(err)
	}
	meta := map[string]any{"rate": 48000, "channels": 2, "sample_bytes": 2, "voice_divisor": speaker.VoiceDivisor, "method": "既有 remake 取樣率模型；不宣稱原版牆鐘或波形一致"}
	for _, edition := range []state.Edition{state.EditionBase, state.EditionPlus} {
		folder := "三國演義"
		if edition == state.EditionPlus {
			folder = "三國演義1加強版"
		}
		c2, c3 := container(filepath.Join(*root, folder), "DATA2"), container(filepath.Join(*root, folder), "DATA3")
		sc, err := state.LoadScenario(c2, state.Scenario1)
		if err != nil {
			panic(err)
		}
		g, err := game.New(sc, 0, 5, edition)
		if err != nil {
			panic(err)
		}
		rows := map[string]any{}
		for j, cue := range [][3]int{{32, 456, 499}, {0, 457, 499}} {
			var pcm []byte
			var clips []map[string]any
			for _, n := range cue {
				name := speaker.VoiceName(n)
				var raw []byte
				for _, c := range []*assets.Container{c2, c3} {
					if i, ok := c.ByName(name); ok {
						raw = c.Data(i)
						break
					}
				}
				if len(raw) == 0 || len(raw) > speaker.MaxVoiceBytes {
					panic(name + " 缺漏或越界")
				}
				clips = append(clips, map[string]any{"name": name, "bytes": len(raw), "sha256": fmt.Sprintf("%x", sha256.Sum256(raw))})
				for _, x := range speaker.Render(speaker.NewClip(raw), speaker.Rate(speaker.VoiceDivisor), 48000) {
					var f [4]byte
					binary.LittleEndian.PutUint16(f[:2], uint16(x))
					binary.LittleEndian.PutUint16(f[2:], uint16(x))
					pcm = append(pcm, f[:]...)
				}
			}
			file := fmt.Sprintf("%s-%d.pcm", edition, j+1)
			if err := os.WriteFile(filepath.Join(*out, file), pcm, 0644); err != nil {
				panic(err)
			}
			x := g.Lord(0)
			if j == 1 {
				x = g.Lord(g.Prefecture(7).Owner)
			}
			rows[fmt.Sprint(j+1)] = map[string]any{"file": file, "frames": len(pcm) / 4, "sha256": fmt.Sprintf("%x", sha256.Sum256(pcm)), "clips": clips, "speaker": x.Name, "portrait": fmt.Sprintf("F%03d.png", x.Portrait)}
		}
		meta[string(edition)] = rows
	}
	b, err := json.MarshalIndent(meta, "", "  ")
	if err != nil {
		panic(err)
	}
	if err := os.WriteFile(filepath.Join(*out, "reference.json"), b, 0644); err != nil {
		panic(err)
	}
}
