// san1diff 比原版與加強版的劇本資料。
//
// `DATA2.GRP` 兩版**等長但內容不同**，而版面已知（三張表），
// 所以 diff 出來的每一格都對得回欄位——這是不必反組譯就做得到的
// 版本差異盤點（`docs/mechanics/90` §4）。
//
// ⚠ **本儲存庫不含任何原版檔案。** 這個工具讀的是玩家自己那兩份。
//
//	tools/go.sh run ./cmd/san1diff -root /path/to/org_game
package main

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/wicanr2/softworld_san1_remake/internal/assets"
	"github.com/wicanr2/softworld_san1_remake/internal/state"
)

func main() {
	root := flag.String("root", "", "放著兩個版本目錄的上層（必填）")
	base := flag.String("base", "三國演義", "原版的目錄名")
	plus := flag.String("plus", "三國演義1加強版", "加強版的目錄名")
	flag.Parse()
	if *root == "" {
		fmt.Fprintln(os.Stderr, "san1diff: 要用 -root 指到放著兩版的目錄")
		os.Exit(2)
	}
	if err := run(*root, *base, *plus); err != nil {
		fmt.Fprintln(os.Stderr, "san1diff:", err)
		os.Exit(1)
	}
}

func open(root, ver string) (*assets.Container, error) {
	b := filepath.Join(root, ver, "DATA2")
	var p [3][]byte
	for i, ext := range []string{".NAM", ".IDX", ".GRP"} {
		x, err := os.ReadFile(b + ext)
		if err != nil {
			return nil, err
		}
		p[i] = x
	}
	return assets.OpenContainer(p[0], p[1], p[2])
}

func run(root, base, plus string) error {
	cb, err := open(root, base)
	if err != nil {
		return err
	}
	cp, err := open(root, plus)
	if err != nil {
		return err
	}
	diffItems(cb, cp)
	for _, slot := range []string{"001", "002", "003", "004", "005", "006"} {
		sb, err := state.LoadScenario(cb, state.Slot(slot))
		if err != nil {
			return fmt.Errorf("原版 %s：%w", slot, err)
		}
		sp, err := state.LoadScenario(cp, state.Slot(slot))
		if err != nil {
			return fmt.Errorf("加強版 %s：%w", slot, err)
		}
		diffSlot(slot, sb, sp)
	}
	return nil
}

// diffItems 逐項比兩版的容器內容。
//
// 兩版的 `.NAM` 與 `.IDX` 完全相同（`docs/mechanics/90` §2），
// 所以項目清單與邊界一樣，可以按名字一對一比。
func diffItems(b, p *assets.Container) {
	fmt.Printf("=== DATA2 逐項 ===\n")
	same, diff, sized, saves := 0, 0, 0, 0
	var names []string
	for i := 0; i < b.Len(); i++ {
		e := b.Entry(i)
		j, ok := p.ByName(e.Name)
		if !ok {
			fmt.Printf("  加強版沒有 %s\n", e.Name)
			continue
		}
		x, y := b.Data(i), p.Data(j)
		if len(x) != len(y) {
			sized++
			fmt.Printf("  %s：長度 %d → %d\n", e.Name, len(x), len(y))
			continue
		}
		n := 0
		for k := range x {
			if x[k] != y[k] {
				n++
			}
		}
		if n == 0 {
			same++
			continue
		}
		// 存檔槽是各自那一份 bundle 裡存的進度，不是版本差異。
		if isSave(e.Name) {
			saves++
			continue
		}
		diff++
		names = append(names, fmt.Sprintf("%s(%d/%d)", e.Name, n, len(x)))
	}
	fmt.Printf("  相同 %d 項、不同 %d 項、長度不同 %d 項、"+
		"存檔槽 %d 項（各自的進度，不算版本差異）（共 %d 項）\n",
		same, diff, sized, saves, b.Len())
	sort.Strings(names)
	for i, n := range names {
		if i%4 == 0 {
			fmt.Print("   ")
		}
		fmt.Printf(" %-22s", n)
		if i%4 == 3 {
			fmt.Println()
		}
	}
	fmt.Println()
}

// isSave 說一個項目是不是存檔槽。存檔是玩家自己的進度，
// 兩版的 bundle 各自附了不同的存檔，那不是版本差異。
func isSave(name string) bool {
	return strings.Contains(name, ".SV") || strings.HasSuffix(name, ".SVP")
}

// diffSlot 比一個劇本的三張表。
func diffSlot(slot string, b, p *state.Scenario) {
	mb, sb, gb := b.Tables()
	mp, sp, gp := p.Tables()

	fmt.Printf("\n=== 劇本 %s ===\n", slot)
	for _, x := range []struct {
		name   string
		rec    int
		lo, hi []byte
	}{
		{"BASEMAS 諸侯", state.MasterRecordSize, mb, mp},
		{"BASESTA 州郡", state.PrefectureRecordSize, sb, sp},
		{"BASEGEN 人物", state.GeneralRecordSize, gb, gp},
	} {
		if len(x.lo) != len(x.hi) {
			fmt.Printf("  %s：長度不同 %d／%d\n", x.name, len(x.lo), len(x.hi))
			continue
		}
		// 差在哪一筆、哪一個位移。
		recs := map[int]bool{}
		offs := map[int]int{}
		total := 0
		for i := range x.lo {
			if x.lo[i] == x.hi[i] {
				continue
			}
			total++
			recs[i/x.rec] = true
			offs[i%x.rec]++
		}
		if total == 0 {
			fmt.Printf("  %s：完全相同\n", x.name)
			continue
		}
		var os_ []int
		for o := range offs {
			os_ = append(os_, o)
		}
		sort.Ints(os_)
		fmt.Printf("  %s：%d 個位元組不同，散在 %d 筆記錄；位移 ", x.name, total, len(recs))
		for i, o := range os_ {
			if i > 0 {
				fmt.Print("、")
			}
			fmt.Printf("%d(×%d)", o, offs[o])
			if i == 11 && len(os_) > 12 {
				fmt.Printf("… 共 %d 個位移", len(os_))
				break
			}
		}
		fmt.Println()
	}
}
