//go:build ignore

// 匯出圖鑑所需的原始姓名、肖像對照與劇本能力，不匯出原版圖像或 binary。
package main

import (
	"crypto/sha256"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"path/filepath"

	"github.com/wicanr2/softworld_san1_remake/internal/assets"
	"github.com/wicanr2/softworld_san1_remake/internal/state"
)

func main() {
	root := flag.String("root", "/orig", "兩版原始資料的父目錄")
	out := flag.String("out", "", "JSON 輸出路徑")
	flag.Parse()
	if *out == "" {
		panic("需要 -out")
	}
	editions := map[string]any{}
	for _, edition := range []string{"base", "plus"} {
		folder := "三國演義"
		if edition == "plus" {
			folder = "三國演義1加強版"
		}
		var raw [3][]byte
		hashes := map[string]string{}
		for i, ext := range []string{".NAM", ".IDX", ".GRP"} {
			b, err := os.ReadFile(filepath.Join(*root, folder, "DATA2"+ext))
			if err != nil {
				panic(err)
			}
			raw[i] = b
			hashes["DATA2"+ext] = fmt.Sprintf("%x", sha256.Sum256(b))
		}
		c, err := assets.OpenContainer(raw[0], raw[1], raw[2])
		if err != nil {
			panic(err)
		}
		sc, err := state.LoadScenario(c, state.Scenario1)
		if err != nil {
			panic(err)
		}
		var people []map[string]any
		for _, g := range sc.Generals() {
			if !g.IsPerson {
				continue
			}
			people = append(people, map[string]any{
				"index": g.Index, "name": g.Name, "portrait": g.Portrait,
				"stamina": g.Stamina, "intel": g.Intel, "war": g.War, "charm": g.Charm,
				"troop": state.TroopName(g.Troop),
			})
		}
		editions[edition] = map[string]any{"scenario": "001", "inputs": hashes, "people": people}
	}
	v := map[string]any{"schema": 1, "source": "state.LoadScenario；劇本 001 BASEGEN；姓名採 cp950 原文", "editions": editions}
	b, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		panic(err)
	}
	if err := os.WriteFile(*out, append(b, '\n'), 0644); err != nil {
		panic(err)
	}
}
