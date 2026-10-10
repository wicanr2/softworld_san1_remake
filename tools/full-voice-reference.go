//go:build ignore

// Additional normal-player voice references; raw assets and PCM remain local.
package main

import (
	"crypto/sha256"
	"encoding/binary"
	"encoding/json"
	"flag"
	"fmt"
	"image/png"
	"os"
	"path/filepath"

	"github.com/wicanr2/softworld_san1_remake/internal/assets"
	"github.com/wicanr2/softworld_san1_remake/internal/font"
	"github.com/wicanr2/softworld_san1_remake/internal/game"
	"github.com/wicanr2/softworld_san1_remake/internal/i18n"
	"github.com/wicanr2/softworld_san1_remake/internal/speaker"
	"github.com/wicanr2/softworld_san1_remake/internal/state"
	"github.com/wicanr2/softworld_san1_remake/internal/ui"
)

func open(root, name string) *assets.Container {
	var files [3][]byte
	for i, ext := range []string{".NAM", ".IDX", ".GRP"} {
		var err error
		files[i], err = os.ReadFile(filepath.Join(root, name+ext))
		if err != nil {
			panic(err)
		}
	}
	c, err := assets.OpenContainer(files[0], files[1], files[2])
	if err != nil {
		panic(err)
	}
	return c
}

func main() {
	root := flag.String("root", "/orig", "original edition directories")
	out := flag.String("out", "", "existing private output directory")
	flag.Parse()
	if st, err := os.Stat(*out); err != nil || !st.IsDir() {
		panic("output directory required")
	}
	result := map[string]any{"method": "remake sample-rate model; original clip indices; normal GUI references only"}
	for _, edition := range []state.Edition{state.EditionBase, state.EditionPlus} {
		folder := "三國演義"
		if edition == state.EditionPlus {
			folder = "三國演義1加強版"
		}
		c2, c3 := open(filepath.Join(*root, folder), "DATA2"), open(filepath.Join(*root, folder), "DATA3")
		scenario, err := state.LoadScenario(c2, state.Scenario1)
		if err != nil {
			panic(err)
		}
		var g *game.State
		var chief *game.General
		position, chosen, at := 0, state.FactionID(0), 0
		for f := state.FactionID(0); f < 16; f++ {
			candidate, err := game.New(scenario, f, 5, edition)
			if err != nil {
				continue
			}
			territory := candidate.Territory(f)
			if len(territory) != 1 {
				continue
			}
			lord := candidate.Lord(f)
			if lord == nil || lord.Location != territory[0] {
				continue
			}
			for i, x := range candidate.PickRoster(territory[0], game.PickWiseSub, game.PickByIntel) {
				if x.Index != candidate.Faction(f).Chief {
					g, chief, position, chosen, at = candidate, x, i+1, f, territory[0]
					break
				}
			}
			if chief != nil {
				break
			}
		}
		if chief == nil {
			panic("no normal chief appointment fixture")
		}
		people := map[string]*game.General{}
		for i := 0; i < 350; i++ {
			if x := g.General(i); x != nil && x.Name != "" {
				people[x.Name] = x
			}
		}
		lu, chen := people["呂布"], people["陳宮"]
		if lu == nil || chen == nil {
			panic("duel data names missing")
		}
		lord, gov := g.Lord(chosen), g.Governor(at)
		fontFile, err := os.Open("fonts/unifont.hex.gz")
		if err != nil {
			panic(err)
		}
		face, err := font.ParseHexGz(fontFile, 16)
		fontFile.Close()
		if err != nil {
			panic(err)
		}
		canvas := ui.NewCanvasPx(176, 96, face)
		canvas.Fill(assets.EGAPalette[2])
		for row, line := range ui.MessageLines(i18n.Sf("ask.main", ui.NameField(lord.Name), at, g.Prefecture(at).Name), 22) {
			canvas.DrawTextPx(0, row*16, line, assets.EGAPalette[14])
		}
		readyFile := string(edition) + "-main-ready.png"
		ready, err := os.Create(filepath.Join(*out, readyFile))
		if err != nil {
			panic(err)
		}
		if err := png.Encode(ready, canvas.Img); err != nil {
			panic(err)
		}
		ready.Close()
		rows := map[string]any{}
		for _, row := range []struct {
			tag, key string
			indices  [3]int
			who      *game.General
		}{
			{"chief-order", "bub.chiefOrder", [3]int{chief.Index, 399, 499}, lord},
			{"chief-reply", "bub.chiefReply", [3]int{400, 499, 499}, chief},
			{"atlas", "bub.atlas", [3]int{356, 499, 499}, gov},
			{"camp", "bub.camp", [3]int{430, 499, 499}, lu},
			{"duel-challenge", "bub.duelChallenge", [3]int{chen.Index, 433, 499}, lu},
			{"duel-accept", "bub.duelAccept", [3]int{lu.Index, 434, 499}, chen},
		} {
			// Original call-site tuples from spec/005 §9.6–9.8 and RE/12.
			// Do not ask the production cue dispatcher for its own expected output.
			indices := row.indices
			var pcm []byte
			for _, n := range indices {
				var data []byte
				name := speaker.VoiceName(n)
				for _, c := range []*assets.Container{c2, c3} {
					if i, ok := c.ByName(name); ok {
						data = c.Data(i)
						break
					}
				}
				if len(data) == 0 || len(data) > speaker.MaxVoiceBytes {
					panic(name)
				}
				for _, sample := range speaker.Render(speaker.NewClip(data), speaker.Rate(speaker.VoiceDivisor), 48000) {
					var frame [4]byte
					binary.LittleEndian.PutUint16(frame[:2], uint16(sample))
					binary.LittleEndian.PutUint16(frame[2:], uint16(sample))
					pcm = append(pcm, frame[:]...)
				}
			}
			file := fmt.Sprintf("%s-%s.pcm", edition, row.tag)
			if err := os.WriteFile(filepath.Join(*out, file), pcm, 0644); err != nil {
				panic(err)
			}
			rows[row.tag] = map[string]any{"key": row.key, "indices": indices, "file": file, "frames": len(pcm) / 4,
				"sha256": fmt.Sprintf("%x", sha256.Sum256(pcm)), "speaker": row.who.Name, "speaker_index": row.who.Index, "portrait": fmt.Sprintf("F%03d.png", row.who.Portrait)}
		}
		result[string(edition)] = map[string]any{"lord_menu": int(chosen) + 1, "prefecture": at, "chief_position": position,
			"chief_index": chief.Index, "lord": lord.Name, "main_ready": readyFile,
			"scene": fmt.Sprintf("SCG%02d.png", assets.SceneAppoint), "samples": rows}
	}
	data, err := json.MarshalIndent(result, "", "  ")
	if err != nil {
		panic(err)
	}
	if err := os.WriteFile(filepath.Join(*out, "full-reference.json"), append(data, '\n'), 0644); err != nil {
		panic(err)
	}
}
