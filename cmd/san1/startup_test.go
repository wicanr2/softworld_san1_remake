package main

import (
	"fmt"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/wicanr2/softworld_san1_remake/internal/cells"
)

func TestStartupRemainsResponsiveDuringBlockedLoad(t *testing.T) {
	entered, release := make(chan struct{}), make(chan struct{})
	w := newStartupWindow("game/base", 2, func(report func(string)) (*preparedApp, error) {
		close(entered)
		report("讀取高清素材")
		<-release
		return nil, fmt.Errorf("缺少資料 DATA2.GRP")
	})
	w.drawn = true
	defer close(release)
	if err := w.Update(); err != nil {
		t.Fatal(err)
	}
	select {
	case <-entered:
	case <-time.After(time.Second):
		t.Fatal("未開始背景載入")
	}
	for range 100 {
		if err := w.Update(); err != nil {
			t.Fatal(err)
		}
	}
	if w.game != nil || w.err != nil || w.ticks != 101 {
		t.Fatalf("載入期間狀態錯誤: %+v", w)
	}
	w.drawStatus()
	if len(w.canvas.Missing) != 0 || w.canvas.Clipped != 0 {
		t.Fatalf("載入文字缺字或越界: %v", w.canvas.Missing)
	}
}

func TestStartupPathsAndBoundedErrors(t *testing.T) {
	exe := filepath.Join(t.TempDir(), "含中文的目錄", "san1")
	for _, edition := range []string{"base", "plus"} {
		p := defaultStartupPaths(exe, edition)
		if p.root != filepath.Join(filepath.Dir(exe), "game", edition) || !filepath.IsAbs(p.font) || !filepath.IsAbs(p.saves) || !filepath.IsAbs(p.hd) {
			t.Fatalf("預設路徑錯誤: %+v", p)
		}
	}
	for _, text := range []string{strings.Repeat("資料目錄/", 100), strings.Repeat("file/", 100), "line1\nline2\nline3"} {
		lines := startupLines(text, 70, 2)
		if len(lines) > 2 {
			t.Fatalf("過多行: %v", lines)
		}
		for _, line := range lines {
			if cells.Width(line) > 70 {
				t.Fatalf("文字越界: %s", line)
			}
		}
	}
}
