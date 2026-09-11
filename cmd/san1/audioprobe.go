package main

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"time"

	"github.com/ebitengine/oto/v3"
)

// 音訊裝置的探測。
//
// **Ebiten 把音訊驅動開不起來當成致命錯誤**：`audio.Context` 一旦建立，
// 驅動的錯誤就從遊戲迴圈的 hook 回傳（ebiten v2.9.9 `audio/audio.go`
// 的 `AppendHookOnBeforeUpdate`），`RunGame` 直接結束。沒有音效卡的機器
// （無頭、容器、ALSA 沒設好）因此連開都開不起來——而配樂與音效本來的
// 原則是「放不出聲音不該擋著開遊戲」（`newJukebox`／`newVoicebox`）。
//
// **在同一個行程裡先試一次不行**：oto 一個行程只准開一個環境，而且
// 「開過了」的旗標在嘗試**之前**就立起來（oto v3.4.0 `context.go` 的
// `contextCreated`）——試一次就把 Ebiten 要用的那個名額用掉了。所以
// 探測放在子行程：開一次、等它就緒、回報結果、結束。

// probeAudioArg 是子行程的第一個參數。不走 `flag`：它不是給玩家用的，
// 也不該出現在 `-h` 裡。
const probeAudioArg = "-probe-audio"

// probeAudioTimeout 是等裝置就緒的上限。**remake 自選**：裝置正常時
// 就緒是毫秒級的，這個數字只是讓卡住的驅動不要拖著開遊戲。
const probeAudioTimeout = 3 * time.Second

// probeAudioChild 是子行程那一邊：開得起來回 0，開不起來把理由印到
// stderr 回 1。
func probeAudioChild() int {
	ctx, ready, err := oto.NewContext(&oto.NewContextOptions{
		SampleRate:   audioRate,
		ChannelCount: 2,
		Format:       oto.FormatSignedInt16LE,
	})
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	select {
	case <-ready:
	case <-time.After(probeAudioTimeout):
		fmt.Fprintln(os.Stderr, "音訊裝置沒有在時限內就緒")
		return 1
	}
	if err := ctx.Err(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	return 0
}

// audioAvailable 是主行程那一邊：跑一次子行程看結果。回傳能不能開聲音，
// 不能的時候附上理由（子行程印出來的最後一行）。
//
// **找不到自己的執行檔時照舊開聲音**：那是探測本身出了問題，不是裝置
// 出了問題——把聲音關掉等於用一個不相干的錯誤拿走玩家的配樂。
func audioAvailable() (bool, string) {
	exe, err := os.Executable()
	if err != nil {
		return true, ""
	}
	ctx, cancel := context.WithTimeout(context.Background(), probeAudioTimeout+2*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, exe, probeAudioArg)
	var errOut bytes.Buffer
	cmd.Stderr = &errOut
	if err := cmd.Run(); err != nil {
		why := strings.TrimSpace(errOut.String())
		if i := strings.LastIndexByte(why, '\n'); i >= 0 {
			why = why[i+1:]
		}
		if why == "" {
			why = err.Error()
		}
		return false, why
	}
	return true, ""
}
