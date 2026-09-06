//go:build oracle

package parity

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"testing"

	"github.com/wicanr2/dosgolem/oracle"

	"github.com/wicanr2/softworld_san1_remake/internal/ai"
	"github.com/wicanr2/softworld_san1_remake/internal/game"
	"github.com/wicanr2/softworld_san1_remake/internal/state"
)

// 一個月的狀態轉移對拍。
//
// 原版與 remake 從**同一個局面**出發，各走一個月，比三張表。
// 局面是原版自己記憶體裡的那 19,220 個位元組，所以「同一個」不是假設
// ——remake 這一邊是拿那份位元組解出來的。
//
// 玩家兩邊都休息（不做任何事），所以差異只可能來自：
//
//   - 電腦諸侯的決策（`docs/mechanics/70-ai`，還沒解）
//   - 每月結算（收成、洪水、物價、忠誠……）
//
// 這一條**不是回歸測試**，是觀測工具：`base` AI 還沒解出來之前它必然
// 有差，差在哪裡就是接下來要解的東西。所以它只報告不判定。

// TestZZMonthParity 原版走一個月，remake 走同一個月，逐欄位比。
func TestZZMonthParity(t *testing.T) {
	root := origRoot(t)
	c := openContainer(t, filepath.Join(root, "DATA2"))
	sc0, err := state.LoadScenario(c, state.Slot("001"))
	if err != nil {
		t.Fatal(err)
	}
	seedMas, _, _ := sc0.Tables()

	o, err := oracle.Load(filepath.Join(root, "AA.EXE"), root)
	if err != nil {
		t.Fatal(err)
	}
	defer o.Close()

	base := bootToGame(t, o, seedMas)
	nMas, nSta, nGen := state.MasterTableSize, state.PrefectureTableSize, state.GeneralTableSize
	total := nMas + nSta + nGen

	before := o.Bytes(addr(base), total)
	dumpTables(t, before, "parity-00-出發")

	// remake 這一邊從同一份位元組建局面。
	sc, err := state.DecodeTables(state.Slot("001"),
		before[:nMas], before[nMas:nMas+nSta], before[nMas+nSta:])
	if err != nil {
		t.Fatalf("原版記憶體裡的三張表解不開：%v", err)
	}

	// 玩家是誰：主畫面問的是 (41)南海，所以 41 郡的所屬就是玩家。
	// **不要預設 0**——「玩家其實在控制別人」在畫面上看不出來。
	player := state.FactionID(before[nMas+41*state.PrefectureRecordSize+30])
	t.Logf("玩家勢力槽號 %d", player)

	g, err := game.New(sc, player, 5)
	if err != nil {
		t.Fatalf("remake 這邊開不了局：%v", err)
	}

	// 原版：一路「內政 → 休息 → Y」，把每個郡的指令用掉。
	seq := strings.Split(envOr("SAN1_TURNKEY", "4\r|4\r|Y"), "|")
	turns := 20
	if v, err := strconv.Atoi(os.Getenv("SAN1_TURNS")); err == nil && v > 0 {
		turns = v
	}
	const settle = 40_000_000
	for i := 0; i < turns; i++ {
		for _, keys := range seq {
			o.Drain()
			o.PressScan(keys)
			if err := o.Run(settle * 3); err != nil {
				t.Fatalf("原版停止：%v", err)
			}
		}
	}
	after := o.Bytes(addr(base), total)
	dumpTables(t, after, "parity-01-原版走完")
	t.Logf("原版：三張表動了 %d 個位元組%s",
		differs8(before, after), where(before, after, nMas, nSta))

	// remake：玩家休息，電腦諸侯各自出手，然後結算。
	brain, err := ai.New(ai.ModeBase)
	if err != nil {
		t.Fatal(err)
	}
	if !brain.Derived() {
		t.Logf("⚠ %s 還沒解出來（`Plan` 回空）——下面的差異包含「電腦完全沒動」這一項",
			brain.Name())
	}
	for _, f := range g.Factions() {
		if !f.Alive || f.ID == player {
			continue
		}
		if n, err := g.ApplyAll(brain.Plan(g, f.ID), f.ID); err != nil {
			t.Logf("勢力 %d 的命令有 %d 道成立，然後：%v", f.ID, n, err)
		}
	}
	g.EndMonth()

	rm, rs, rg, err := g.Tables()
	if err != nil {
		t.Fatalf("remake 的盤面寫不回三張表：%v", err)
	}
	mine := append(append(append([]byte{}, rm...), rs...), rg...)
	dumpTables(t, mine, "parity-02-remake走完")

	t.Logf("原版 vs remake：差 %d 個位元組%s",
		differs8(after, mine), where(after, mine, nMas, nSta))
	t.Log(byPrefecture(after, mine, nMas, nSta))
}

// byPrefecture 把州郡表的差異逐郡列出來，欄位印名字。
//
// **總數看不出東西。** 「差 400 個位元組」與「40 個郡各差一個物價」
// 是同一個數字，而後者才是線索。
func byPrefecture(a, b []byte, nMas, nSta int) string {
	rec := state.PrefectureRecordSize
	var lines []string
	for i := 0; i*rec < nSta; i++ {
		lo := nMas + i*rec
		fields := map[string]bool{}
		for j := 0; j < rec && lo+j < len(b); j++ {
			if a[lo+j] != b[lo+j] {
				fields[fieldName(prefField, j)] = true
			}
		}
		if len(fields) == 0 {
			continue
		}
		names := make([]string, 0, len(fields))
		for n := range fields {
			names = append(names, n)
		}
		sort.Strings(names)
		// **所屬要一起印。** 「哪個欄位變了」不接上「那是誰的郡」，
		// 就分不出是電腦諸侯下的令還是每月結算。
		lines = append(lines, fmt.Sprintf("    郡 %2d（勢力 %d）：%s",
			i, b[lo+30], strings.Join(names, " ")))
	}
	if len(lines) == 0 {
		return "    州郡表逐郡相同"
	}
	if len(lines) > 20 {
		lines = append(lines[:20], fmt.Sprintf("    …（還有 %d 個郡）", len(lines)-20))
	}
	return "州郡表逐郡：\n" + strings.Join(lines, "\n")
}

// byGeneral 把人物表的差異逐人列出來。
//
// 訓練度、武裝度、忠誠、所在——電腦諸侯下了什麼令，多半是從這裡看出來的。
func byGeneral(a, b []byte, nMas, nSta int) string {
	rec := state.GeneralRecordSize
	lo0 := nMas + nSta
	var lines []string
	for i := 0; lo0+i*rec+rec <= len(a) && lo0+i*rec+rec <= len(b); i++ {
		lo := lo0 + i*rec
		var fields []string
		for j := 0; j < rec; j++ {
			if a[lo+j] != b[lo+j] {
				n := fieldName(genField, j)
				if len(fields) == 0 || fields[len(fields)-1] != n {
					fields = append(fields, n)
				}
			}
		}
		if len(fields) == 0 {
			continue
		}
		lines = append(lines, fmt.Sprintf("    人物 %3d（勢力 %d、所在 %d）：%s",
			i, b[lo+18], b[lo+19], strings.Join(fields, " ")))
	}
	if len(lines) == 0 {
		return "    人物表逐筆相同"
	}
	if len(lines) > 24 {
		lines = append(lines[:24], fmt.Sprintf("    …（還有 %d 人）", len(lines)-24))
	}
	return "人物表逐筆：\n" + strings.Join(lines, "\n")
}
