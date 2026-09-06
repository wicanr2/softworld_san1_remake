// san1names 列出人名與郡名用到的字，給英日對照表當母本。
//
// ⚠ **本儲存庫不含任何原版檔案。** 這個工具讀的是玩家自己那一份。
//
//	tools/go.sh run ./cmd/san1names -root /path/to/三國演義
package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"sort"

	"github.com/wicanr2/softworld_san1_remake/internal/assets"
	"github.com/wicanr2/softworld_san1_remake/internal/state"
)

func main() {
	root := flag.String("root", "", "原版遊戲目錄（必填，玩家自備）")
	out := flag.String("out", "", "把字表寫成 JSON（空的就只印統計）")
	flag.Parse()
	if *root == "" {
		fmt.Fprintln(os.Stderr, "san1names: 要用 -root 指到原版目錄")
		os.Exit(2)
	}
	if err := run(*root, *out); err != nil {
		fmt.Fprintln(os.Stderr, "san1names:", err)
		os.Exit(1)
	}
}

func run(root, out string) error {
	base := filepath.Join(root, "DATA2")
	rd := func(ext string) []byte {
		b, _ := os.ReadFile(base + ext)
		return b
	}
	c, err := assets.OpenContainer(rd(".NAM"), rd(".IDX"), rd(".GRP"))
	if err != nil {
		return err
	}
	// 六個劇本的名字全收：不同劇本的人物不完全一樣。
	people := map[string]bool{}
	prefs := map[string]bool{}
	for _, slot := range []string{"001", "002", "003", "004", "005", "006"} {
		sc, err := state.LoadScenario(c, state.Slot(slot))
		if err != nil {
			continue
		}
		for _, g := range sc.People() {
			if g.Name != "" {
				people[g.Name] = true
			}
		}
		for _, p := range sc.Prefectures() {
			prefs[p.Name] = true
		}
	}
	runes := map[string]int{}
	for s := range people {
		for _, r := range s {
			runes[string(r)]++
		}
	}
	for s := range prefs {
		for _, r := range s {
			runes[string(r)]++
		}
	}
	fmt.Printf("人名 %d 個、郡名 %d 個、用到 %d 個相異字\n",
		len(people), len(prefs), len(runes))

	// 槽號在六個劇本之間穩不穩定？穩定的話對照表就能用槽號當鍵。
	byIndex := map[int]map[string]bool{}
	for _, slot := range []string{"001", "002", "003", "004", "005", "006"} {
		sc, err := state.LoadScenario(c, state.Slot(slot))
		if err != nil {
			continue
		}
		for i, g := range sc.People() {
			if g.Name == "" {
				continue
			}
			if byIndex[i] == nil {
				byIndex[i] = map[string]bool{}
			}
			byIndex[i][g.Name] = true
		}
	}
	shifted := 0
	for i, names := range byIndex {
		if len(names) > 1 {
			shifted++
			if shifted <= 5 {
				fmt.Printf("  槽 %d 在不同劇本是不同人：%v\n", i, sortedKeys(names))
			}
		}
	}
	fmt.Printf("槽號穩定性：%d 個槽在六個劇本之間名字不一致（共 %d 個用到的槽）\n",
		shifted, len(byIndex))

	// 姓（人名的第一個字）另外列：多音字多半出在姓上。
	surnames := map[string]int{}
	for s := range people {
		for _, r := range s {
			surnames[string(r)]++
			break
		}
	}
	var sn []string
	for k := range surnames {
		sn = append(sn, k)
	}
	sort.Strings(sn)
	fmt.Printf("姓 %d 個\n", len(sn))

	if out == "" {
		return nil
	}
	type row struct {
		Char  string `json:"char"`
		Count int    `json:"count"`
		Head  int    `json:"as_surname"`
	}
	var rows []row
	for k, n := range runes {
		rows = append(rows, row{k, n, surnames[k]})
	}
	sort.Slice(rows, func(i, j int) bool { return rows[i].Char < rows[j].Char })
	// 槽號 → 名字，給對照表當工作底稿（放 workplace/，不進版控）。
	idx := map[string]string{}
	pidx := map[string]string{}
	if sc, err := state.LoadScenario(c, state.Slot("001")); err == nil {
		for i, g := range sc.People() {
			if g.Name != "" {
				idx[fmt.Sprint(i)] = g.Name
			}
		}
		for _, p := range sc.Prefectures() {
			pidx[fmt.Sprint(p.ID)] = p.Name
		}
	}
	b, err := json.MarshalIndent(map[string]any{
		"people":      sortedKeys(people),
		"prefectures": sortedKeys(prefs),
		"chars":       rows,
		"by_slot":     idx,
		"by_pref_id":  pidx,
	}, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(out, append(b, '\n'), 0o644)
}

func sortedKeys(m map[string]bool) []string {
	var out []string
	for k := range m {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}
