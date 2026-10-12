package save_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/wicanr2/softworld_san1_remake/internal/game"
	"github.com/wicanr2/softworld_san1_remake/internal/save"
	"github.com/wicanr2/softworld_san1_remake/internal/state"
)

func TestExpandedSaveRoundTripAndLegacyIsolation(t *testing.T) {
	sc := loadScenario(t)
	g, err := game.NewExpanded(sc, 16, 5, state.EditionBase)
	if err != nil {
		t.Fatal(err)
	}
	lord := g.Lord(16)
	lord.Soldiers = 777
	lord.Stamina = 73
	g.Faction(16).Prestige = 83
	g.Prefecture(5).Population += 17
	g.Prefecture(5).Commanded = true
	g.Options.Font = 2
	if err := g.Options.SetDelay(47); err != nil {
		t.Fatal(err)
	}
	if err := g.Options.SetAIOrders(4); err != nil {
		t.Fatal(err)
	}
	before, err := g.CaptureSnapshot()
	if err != nil {
		t.Fatal(err)
	}
	before.RandOn, before.RandSeed, before.RandDraws = true, 12345, 0
	g, err = game.RestoreSnapshot(sc, g.CaptureExtra(), before)
	if err != nil {
		t.Fatal(err)
	}
	root := t.TempDir()
	order := make([]int, 43)
	for i := range order {
		order[i] = 42 - i
	}
	if err = save.WriteWithProgress(root, 1, g, "張角的擴充進度", order, 19); err != nil {
		t.Fatal(err)
	}
	loaded, err := save.Read(root, 1)
	if err != nil {
		t.Fatal(err)
	}
	after, err := loaded.CaptureSnapshot()
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(before, after) {
		t.Fatal("完整擴充快照在存讀中改變")
	}
	if loaded.Player != 16 || !loaded.IsHuman(16) || loaded.Options.Font != 2 || loaded.Options.Delay() != 47 || loaded.Options.AIOrders() != 4 {
		t.Fatal("玩家或選項未保存")
	}
	p, err := save.ReadProgress(root, 1)
	if err != nil {
		t.Fatal(err)
	}
	if p.Cursor != 19 || !reflect.DeepEqual(p.Order[:], order) {
		t.Fatal("月內游標未保存")
	}
	if !save.List(root)[0].Exists {
		t.Fatal("格式 2 不在讀檔選單")
	}
	if save.List(root)[0].Name != "張角的擴充進度" {
		t.Fatal("格式 2 名稱被原表截斷")
	}
	for i := 0; i < 10; i++ {
		if g.Roll(100, i) != loaded.Roll(100, i) {
			t.Fatal("亂數狀態未保存")
		}
	}
	legacy, err := game.New(sc, 0, 5, state.EditionBase)
	if err != nil {
		t.Fatal(err)
	}
	if err = save.Write(root, 2, legacy, "舊名單"); err != nil {
		t.Fatal(err)
	}
	old, err := save.Read(root, 2)
	if err != nil {
		t.Fatal(err)
	}
	if old.Expanded() || len(old.AllGenerals()) != 350 || len(old.Factions()) != 14 {
		t.Fatal("舊存檔自動擴充")
	}
}

func TestExpandedSaveRejectsBrokenReferences(t *testing.T) {
	g, err := game.NewExpanded(loadScenario(t), 0, 5, state.EditionBase)
	if err != nil {
		t.Fatal(err)
	}
	for _, kind := range []string{"missing_snapshot", "duplicate_source", "bad_lord", "enemy_lord", "fallen_lord", "missing_general", "unsupported_version"} {
		t.Run(kind, func(t *testing.T) {
			root := t.TempDir()
			if err := save.Write(root, 1, g, "錯誤對照"); err != nil {
				t.Fatal(err)
			}
			path := filepath.Join(root, "SV1", "REMAKE.JSON")
			blob, err := os.ReadFile(path)
			if err != nil {
				t.Fatal(err)
			}
			var m map[string]any
			if err = json.Unmarshal(blob, &m); err != nil {
				t.Fatal(err)
			}
			snapshot := m["snapshot"].(map[string]any)
			switch kind {
			case "missing_snapshot":
				delete(m, "snapshot")
			case "duplicate_source":
				people := snapshot["generals"].([]any)
				people[351].(map[string]any)["source"] = people[350].(map[string]any)["source"]
			case "bad_lord":
				snapshot["factions"].([]any)[0].(map[string]any)["Lord"] = 9999
			case "enemy_lord":
				factions := snapshot["factions"].([]any)
				factions[0].(map[string]any)["Lord"] = factions[1].(map[string]any)["Lord"]
			case "fallen_lord":
				lord := int(snapshot["factions"].([]any)[0].(map[string]any)["Lord"].(float64))
				snapshot["generals"].([]any)[lord].(map[string]any)["Status"] = state.StatusFallen
			case "missing_general":
				people := snapshot["generals"].([]any)
				snapshot["generals"] = people[:len(people)-1]
			case "unsupported_version":
				m["version"] = 99
			}
			blob, err = json.Marshal(m)
			if err != nil {
				t.Fatal(err)
			}
			if err = os.WriteFile(path, blob, 0644); err != nil {
				t.Fatal(err)
			}
			if _, err = save.Read(root, 1); err == nil {
				t.Fatal("破損擴充存檔被接受")
			}
		})
	}
}
