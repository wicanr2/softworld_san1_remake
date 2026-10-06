//go:build ignore

// 由玩家自備 DATA1 產生 remake 播放器的 PCM 參考，輸出只留本機。
package main

import (
	"encoding/binary"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/wicanr2/softworld_san1_remake/internal/assets"
	"github.com/wicanr2/softworld_san1_remake/internal/music"
	"github.com/wicanr2/softworld_san1_remake/internal/speaker"
)

func main() {
	root := flag.String("root", "", "玩家自備原版目錄")
	out := flag.String("out", "", "既有本機輸出目錄")
	cursorsOnly := flag.Bool("cursors-only", false, "只重生游標來源")
	seconds := flag.Int("seconds", 240, "配樂參考秒數")
	flag.Parse()
	if err := reference(*root, *out, *cursorsOnly, *seconds); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func reference(root, out string, cursorsOnly bool, seconds int) error {
	if seconds < 1 || seconds > 600 {
		return fmt.Errorf("秒數須在 1–600")
	}
	var parts [3][]byte
	for i, ext := range []string{".NAM", ".IDX", ".GRP"} {
		b, err := os.ReadFile(filepath.Join(root, "DATA1"+ext))
		if err != nil {
			return err
		}
		parts[i] = b
	}
	c, err := assets.OpenContainer(parts[0], parts[1], parts[2])
	if err != nil {
		return err
	}
	frames, err := assets.CursorFrames(c, assets.CursorMain)
	if err != nil {
		return err
	}
	type cursor struct {
		Name          string
		Width, Height int
		Sprite, Mask  []byte
	}
	var records []cursor
	for _, f := range frames {
		rec := cursor{Name: f.Name, Width: f.Sprite.W, Height: f.Sprite.H}
		for y := 0; y < f.Sprite.H; y++ {
			for x := 0; x < f.Sprite.W; x++ {
				rec.Sprite = append(rec.Sprite, f.Sprite.At(x, y))
				rec.Mask = append(rec.Mask, f.Mask.At(x, y))
			}
		}
		records = append(records, rec)
	}
	b, err := json.MarshalIndent(records, "", "  ")
	if err != nil {
		return err
	}
	if err := os.WriteFile(filepath.Join(out, "main-cursors.json"), b, 0644); err != nil {
		return err
	}
	if cursorsOnly {
		return nil
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
		return err
	}
	if len(tracks) != 5 {
		return fmt.Errorf("曲數不是五首：%d", len(tracks))
	}
	f, err := os.OpenFile(filepath.Join(out, "music-reference.pcm"), os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0644)
	if err != nil {
		return err
	}
	_, err = io.CopyN(f, music.NewStream(tracks[0].Song, tracks[0].Bank, 48000), int64(seconds)*48000*4)
	closeErr := f.Close()
	if err != nil {
		return err
	}
	if closeErr != nil {
		return closeErr
	}
	bank := &speaker.Bank{}
	if err := bank.Load(speaker.SFXSlot, get(speaker.SFXName)); err != nil {
		return err
	}
	mono := speaker.Render(bank.Clip(speaker.SFXSlot), speaker.Rate(speaker.SFXDivisor), 48000)
	pcm := make([]byte, len(mono)*4)
	for i, v := range mono {
		binary.LittleEndian.PutUint16(pcm[i*4:], uint16(v))
		binary.LittleEndian.PutUint16(pcm[i*4+2:], uint16(v))
	}
	return os.WriteFile(filepath.Join(out, "sfx-reference.pcm"), pcm, 0644)
}
