// san1music 讀原版的配樂資料，列出來或匯出成標準 MIDI 檔。
//
// ⚠ **本儲存庫不含任何原版檔案**，`-root` 由玩家自備；
// 匯出的檔案是玩家自己那一份原版的內容，不隨本專案散布。
//
//	tools/go.sh run ./cmd/san1music -root /path/to/三國演義
//	tools/go.sh run ./cmd/san1music -root /path/to/三國演義 -out /tmp/mus
package main

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"

	"github.com/wicanr2/softworld_san1_remake/internal/assets"
	"github.com/wicanr2/softworld_san1_remake/internal/music"
)

func main() {
	root := flag.String("root", "", "原版遊戲目錄（必填，玩家自備）")
	out := flag.String("out", "", "把每一首匯出成 .mid 到這個目錄；空的就只列出來")
	which := flag.String("set", "MUS", "哪一組：MUS（主選單的五首）／MUSV（長曲）")
	flag.Parse()
	if *root == "" {
		fmt.Fprintln(os.Stderr, "san1music: 要用 -root 指到原版目錄（本儲存庫不含原版檔案）")
		flag.Usage()
		os.Exit(2)
	}
	items, err := openData1(*root)
	if err != nil {
		die(err)
	}
	idx, grp := music.SongIndex, music.SongData
	if *which == "MUSV" {
		idx, grp = music.LongIndex, music.LongData
	}
	tracks, err := music.ParseAll(items[idx], items[grp])
	if err != nil {
		die(err)
	}
	for i, tr := range tracks {
		name := music.Name(i)
		if *which == "MUSV" {
			name = fmt.Sprintf("MUSV-%d", i)
		}
		fmt.Printf("%d. %-6s %5d 事件　%3d BPM　%6.1f 秒　%2d 個音色：",
			i+1, name, len(tr.Song.Events), tr.Song.Tempo,
			tr.Song.Duration(), len(tr.Bank.Instruments))
		for _, in := range tr.Bank.Instruments {
			fmt.Printf(" %s", in.Name)
		}
		fmt.Println()
		if *out == "" {
			continue
		}
		if err := os.MkdirAll(*out, 0o755); err != nil {
			die(err)
		}
		path := filepath.Join(*out, fmt.Sprintf("%d-%s.mid", i+1, name))
		if err := os.WriteFile(path, tr.Song.MIDI(), 0o644); err != nil {
			die(err)
		}
		fmt.Printf("   → %s\n", path)
	}
	if *out != "" {
		fmt.Println("\n⚠ 匯出的是原版的內容，只給你自己用；音色不是原版的 AdLib 音色。")
	}
}

// openData1 讀 DATA1 容器，回傳項目名 → 內容。
func openData1(root string) (map[string][]byte, error) {
	base := filepath.Join(root, "DATA1")
	var parts [3][]byte
	for i, ext := range []string{".NAM", ".IDX", ".GRP"} {
		b, err := os.ReadFile(base + ext)
		if err != nil {
			return nil, fmt.Errorf("讀 DATA1%s：%w", ext, err)
		}
		parts[i] = b
	}
	c, err := assets.OpenContainer(parts[0], parts[1], parts[2])
	if err != nil {
		return nil, err
	}
	out := map[string][]byte{}
	for i := 0; i < c.Len(); i++ {
		out[c.Entry(i).Name] = c.Data(i)
	}
	return out, nil
}

func die(err error) {
	fmt.Fprintln(os.Stderr, "san1music:", err)
	os.Exit(1)
}
