//go:build ignore

// 可丟棄 Go overlay 計時；不加入正式玩家路徑。
package main

import (
	"encoding/json"
	"fmt"
	"os"
	"runtime"
	"time"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/wicanr2/softworld_san1_remake/internal/ui"
)

type hdUploadPhase struct {
	Step, Tile, Speed, Draws int
	Seconds                  float64
}

type hdUploadTrace struct {
	start        time.Time
	path         string
	high, done   bool
	metrics      map[string][]int64
	phases       []hdUploadPhase
	bytes, calls int64
}

func (a *app) hdUploadMeasure(stage string) func() {
	start := time.Now()
	return func() {
		end := time.Now()
		active := a.lure.of != nil && a.lure.step >= 0
		if stage == "update" && active && a.hdUploadTrace == nil {
			path := os.Getenv("SAN1_HD_UPLOAD_TRACE")
			if path == "" {
				return
			}
			a.hdUploadTrace = &hdUploadTrace{start: end, path: path, high: a.hdTheme, metrics: map[string][]int64{}}
		}
		p := a.hdUploadTrace
		if p == nil || p.done {
			return
		}
		p.metrics[stage] = append(p.metrics[stage], end.Sub(start).Nanoseconds())
		if stage == "draw" && active {
			step := a.lure.step
			if len(p.phases) == 0 || p.phases[len(p.phases)-1].Step != step {
				spec := ui.LureFlashSteps()[step]
				p.phases = append(p.phases, hdUploadPhase{Step: step, Tile: spec.Tile, Speed: spec.Speed, Seconds: end.Sub(p.start).Seconds()})
			}
			p.phases[len(p.phases)-1].Draws++
		}
		if stage == "update" && !active {
			p.done = true
			metrics := map[string]any{}
			for key, values := range p.metrics {
				var total, maxTime int64
				for _, n := range values {
					total += n
					maxTime = max(maxTime, n)
				}
				metrics[key] = map[string]any{"count": len(values), "mean_ms": float64(total) / float64(len(values)) / 1e6, "max_ms": float64(maxTime) / 1e6}
			}
			report := map[string]any{"method": "正常玩家誘敵，無錄影，Go overlay 計時", "high": p.high, "phases": p.phases, "metrics": metrics, "duration_seconds": end.Sub(p.start).Seconds(), "upload_calls": p.calls, "upload_bytes": p.bytes, "actual_fps": ebiten.ActualFPS(), "actual_tps": ebiten.ActualTPS(), "go_version": runtime.Version(), "scope": "呼叫內計時與整段牆鐘，不宣稱 GPU 呼叫時間或跨平台效能"}
			blob, err := json.MarshalIndent(report, "", "  ")
			if err != nil {
				panic(err)
			}
			f, err := os.OpenFile(p.path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0644)
			if err != nil {
				panic(err)
			}
			if _, err = f.Write(append(blob, '\n')); err != nil {
				panic(err)
			}
			f.Close()
			fmt.Println("HD_UPLOAD_DONE", p.path)
		}
	}
}

func (a *app) hdUploadPacket(n int) {
	if p := a.hdUploadTrace; p != nil && !p.done {
		p.bytes += int64(n)
		if n > 0 {
			p.calls++
		}
	}
}
