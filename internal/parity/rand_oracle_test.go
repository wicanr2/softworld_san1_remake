//go:build oracle

package parity

import (
	"fmt"
	"path/filepath"
	"testing"

	"github.com/wicanr2/dosgolem/oracle"

	"github.com/wicanr2/softworld_san1_remake/internal/game"
	"github.com/wicanr2/softworld_san1_remake/internal/state"
)

// 亂數是 MSC 6.0 的 `rand()`（`0x5c4:0x2cb0`），`RND(n)` 是它的餘數。
//
// `0x10b0c`（＝ `0x1058:0x058c`）：
//
//	n <= 0 → 回 0
//	ax = rand(); cwtd; idiv n
//	回 dx（餘數）
//
// 這一支把 `rand()` 剛回來的原始值收下來（`0x10b27`，`idiv` 之前），
// 反推 32 位元的狀態，驗常數是不是 `seed = seed*214013 + 2531011`、
// 輸出是不是 `(seed >> 16) & 0x7fff`。
//
// **這是亂數對齊的地基**：remake 要能從原版的狀態接著抽，兩邊的序列
// 才有可能一致（`game.SeedRand`）。
func TestRandIsTheMSCLCG(t *testing.T) {
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
	bootToGame(t, o, seedMas)

	// 種子在 `DS:0xa3ae`（低字）與 `DS:0xa3b0`（高字），從 `rand()` 的碼
	// 直接讀出來的：
	//
	//	mov ax,0x43fd / mov dx,3        ; 乘數 0x000343FD ＝ 214013
	//	push [0xa3b0] / push [0xa3ae]   ; 目前的種子
	//	call 32 位元乘法
	//	add ax,0x9ec3 / adc dx,0x26     ; 加數 0x269EC3 ＝ 2531011
	//	mov [0xa3ae],ax / mov [0xa3b0],dx
	//	mov ax,dx / and ah,0x7f         ; 回 (seed >> 16) & 0x7fff
	//
	// ⚠ **回傳值遮掉了第 31 位**，所以從輸出反推不出高字——要驗就讀
	// 種子本身。
	const want = 400
	seedAt := func(o *oracle.Oracle) uint32 {
		ds := uint32(o.DSReg()) * 16
		return uint32(o.Word(addr(ds+0xa3ae))) | uint32(o.Word(addr(ds+0xa3b0)))<<16
	}
	var before uint32
	have := false
	checked, bad := 0, 0
	var firstBad string
	o.OnCall(addr(0x10b22), func(o *oracle.Oracle) {
		before, have = seedAt(o), true
	})
	o.OnCall(addr(0x10b27), func(o *oracle.Oracle) {
		if !have || checked >= want {
			return
		}
		have = false
		checked++
		wantSeed, wantOut := game.MSCRand(before)
		gotSeed, gotOut := seedAt(o), int(o.AX())
		if gotSeed != wantSeed || gotOut != wantOut {
			if bad == 0 {
				firstBad = fmt.Sprintf(
					"第 %d 次：種子 0x%08x → 原版 0x%08x／算出 0x%08x，回傳 %d／算出 %d",
					checked, before, gotSeed, wantSeed, gotOut, wantOut)
			}
			bad++
		}
	})

	const settle = 40_000_000
	for _, k := range []string{"4\r", "4\r", "Y"} {
		o.Drain()
		o.PressScan(k)
		if err := o.Run(settle * 3); err != nil {
			t.Fatalf("送 %q 時停止：%v", k, err)
		}
	}
	if checked < 50 {
		t.Fatalf("只核對到 %d 次——hook 沒掛上，或者按鍵序列沒推動月份", checked)
	}
	if bad > 0 {
		t.Fatalf("核對 %d 次，%d 次對不上。%s", checked, bad, firstBad)
	}
	t.Logf("核對 %d 次逐次相同：seed = seed*214013 + 2531011（`DS:0xa3ae`），"+
		"RND(n) 取 idiv 的餘數", checked)
}
