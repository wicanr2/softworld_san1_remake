package main

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/hajimehoshi/ebiten/v2/audio"

	"github.com/wicanr2/softworld_san1_remake/internal/assets"
	"github.com/wicanr2/softworld_san1_remake/internal/music"
)

// 配樂。
//
// 曲子與音色都從玩家自己那一份原版讀，**邊播邊合成**：一首曲子算完要
// 好幾秒 CPU，開遊戲不能停在那裡等。`music.Stream` 是拉的，音訊裝置
// 要多少就合多少。
//
// 「其他 → 音樂狀態」關掉的時候曲子照走，只是不出聲——回來的時候接得上
// 原本的位置，而不是從頭開始。

// audioRate 是輸出的取樣率。OPL2 自己是 49,716 Hz，`music.Stream` 內插過來。
const audioRate = 48000

// jukebox 管一首正在播的曲子。
type jukebox struct {
	ctx    *audio.Context
	tracks []music.Track
	player *audio.Player
	stream *music.Stream
	cur    int
}

// newJukebox 從 DATA1 讀出配樂。讀不到就回 nil——沒有音樂不該擋著開遊戲。
func newJukebox(root string) *jukebox {
	c, err := openContainer(root, "DATA1")
	if err != nil {
		fmt.Fprintln(os.Stderr, "san1: 沒有配樂：", err)
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
		fmt.Fprintln(os.Stderr, "san1: 配樂解不開：", err)
		return nil
	}
	return &jukebox{ctx: audio.NewContext(audioRate), tracks: tracks, cur: -1}
}

// Play 換一首。編號超出範圍就取模，所以呼叫端不必自己算。
func (j *jukebox) Play(i int) {
	if j == nil || len(j.tracks) == 0 {
		return
	}
	i = ((i % len(j.tracks)) + len(j.tracks)) % len(j.tracks)
	if i == j.cur && j.player != nil {
		return
	}
	j.Stop()
	tr := j.tracks[i]
	s := music.NewStream(tr.Song, tr.Bank, audioRate)
	if s == nil {
		return
	}
	p, err := j.ctx.NewPlayer(s)
	if err != nil {
		fmt.Fprintln(os.Stderr, "san1: 配樂放不出來：", err)
		return
	}
	j.stream, j.player, j.cur = s, p, i
	p.Play()
}

// Stop 停掉正在播的。
func (j *jukebox) Stop() {
	if j == nil || j.player == nil {
		return
	}
	_ = j.player.Close()
	j.player, j.stream, j.cur = nil, nil, -1
}

// SetSilent 靜音或恢復。曲子照走。
func (j *jukebox) SetSilent(v bool) {
	if j == nil || j.stream == nil {
		return
	}
	j.stream.SetSilent(v)
}

// Next 是下一首的編號。
func (j *jukebox) Next() int {
	if j == nil {
		return 0
	}
	return j.cur + 1
}

// Name 是正在播的曲名。
func (j *jukebox) Name() string {
	if j == nil || j.cur < 0 {
		return ""
	}
	return music.Name(j.cur)
}

// openContainer 開一組 `.NAM`／`.IDX`／`.GRP`。
func openContainer(root, name string) (*assets.Container, error) {
	base := filepath.Join(root, name)
	var parts [3][]byte
	for i, ext := range []string{".NAM", ".IDX", ".GRP"} {
		b, err := os.ReadFile(base + ext)
		if err != nil {
			return nil, fmt.Errorf("讀 %s%s：%w", name, ext, err)
		}
		parts[i] = b
	}
	return assets.OpenContainer(parts[0], parts[1], parts[2])
}

// Len 是有幾首曲子。
func (j *jukebox) Len() int {
	if j == nil {
		return 0
	}
	return len(j.tracks)
}
