package main

import (
	"testing"

	"github.com/wicanr2/softworld_san1_remake/internal/battle"
	"github.com/wicanr2/softworld_san1_remake/internal/ui"
)

func TestLureRedrawKeepsEveryUpdateAndPhase(t *testing.T) {
	a := &app{artBattle: &ui.ArtBattle{}, fight: &fight{speeches: []battle.Speech{{LureFlash: true}}}}
	control := &app{}
	sp := &battle.Speech{LureFlash: true}
	var phases []int
	waitUpdates := 0
	for tick := 0; tick < 1000; tick++ {
		before := a.lure.step
		a.dirty = false
		done := control.updateLureFlash(sp)
		if err := a.updateBattle(); err != nil {
			t.Fatal(err)
		}
		if a.lure.step != control.lure.step || a.lure.left != control.lure.left || done != (len(a.fight.speeches) == 0) {
			t.Fatal("重畫最佳化改變動畫更新或完成時間")
		}
		if done {
			if len(phases) != 22 || waitUpdates == 0 || !a.dirty {
				t.Fatal("相位、等待或完成畫面不完整")
			}
			return
		}
		if tick == 0 || a.lure.step != before {
			if !a.dirty || a.lure.step != len(phases) {
				t.Fatal("新相位漏畫或跳步")
			}
			phases = append(phases, a.lure.step)
		} else {
			waitUpdates++
			if a.dirty {
				t.Fatal("等待中的同一相位仍重畫")
			}
		}
	}
	t.Fatal("動畫未完成")
}
