package main

import (
	"bytes"
	"encoding/json"
	"io/fs"
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/wicanr2/softworld_san1_remake/internal/ai"
	"github.com/wicanr2/softworld_san1_remake/internal/assets"
	"github.com/wicanr2/softworld_san1_remake/internal/game"
	"github.com/wicanr2/softworld_san1_remake/internal/i18n"
	"github.com/wicanr2/softworld_san1_remake/internal/session"
	"github.com/wicanr2/softworld_san1_remake/internal/state"
)

func TestWindowMainBubbleBothQueuesAndSave(t *testing.T) {
	saved := i18n.Current
	defer func() { i18n.Current = saved }()
	root := os.Getenv("SAN1_ORIG")
	if root == "" {
		t.Skip("SAN1_ORIG 未設定，需要本機原版素材")
	}
	for _, edition := range []state.Edition{state.EditionBase, state.EditionPlus} {
		for _, from := range i18n.Locales() {
			t.Run(string(edition)+"/"+string(from), func(t *testing.T) {
				i18n.Current = from
				folder := "三國演義"
				if edition == state.EditionPlus {
					folder = "三國演義1加強版"
				}
				var raw [3][]byte
				for i, ext := range []string{".NAM", ".IDX", ".GRP"} {
					b, err := os.ReadFile(filepath.Join(root, folder, "DATA2"+ext))
					if err != nil {
						t.Fatal(err)
					}
					raw[i] = b
				}
				c, err := assets.OpenContainer(raw[0], raw[1], raw[2])
				if err != nil {
					t.Fatal(err)
				}
				sc, err := state.LoadScenario(c, state.Scenario1)
				if err != nil {
					t.Fatal(err)
				}
				g, err := game.New(sc, 0, 5, edition)
				if err != nil {
					t.Fatal(err)
				}
				g.SeedRand(0x13579bdf)
				mode := ai.ModeBase
				if edition == state.EditionPlus {
					mode = ai.ModePlus
				}
				brain, err := ai.New(mode)
				if err != nil {
					t.Fatal(err)
				}
				s := session.New(g, brain, 0)
				s.Queue(g.WarDeclaration(8, 11, 0))
				var target *game.General
				for _, x := range g.Garrison(8) {
					if x.Name == "關羽" {
						target = g.General(x.Index)
					}
				}
				if target == nil {
					t.Fatal("原始盤面沒有關羽")
				}
				if err := (game.AppointChiefOrder{At: 8, Target: target.Index}).Apply(g, 0); err != nil {
					t.Fatal(err)
				}
				before := append([]*game.Bubble(nil), s.Bubbles...)
				seed, draws := g.RandSeed(), g.RandDraws()
				name := s.SaveName(1, 8, "")
				dir := t.TempDir()
				if err := s.Save(dir, 1, name); err != nil {
					t.Fatal(err)
				}
				files := mainBubbleSaveFiles(t, dir)
				app := &app{s: s}
				old := from
				finish := i18n.Ja
				if from == i18n.Ja {
					finish = i18n.En
				}
				for _, to := range []i18n.Locale{i18n.En, i18n.Ja, i18n.ZhHant, from, finish} {
					i18n.Current = to
					app.relocalizeWindow(old)
					cao := map[i18n.Locale]string{i18n.En: "Cao Cao", i18n.Ja: "曹操", i18n.ZhHant: "曹操"}[to]
					liu := map[i18n.Locale]string{i18n.En: "Liu Bei", i18n.Ja: "劉備", i18n.ZhHant: "劉備"}[to]
					if s.Bubbles[0].Text != i18n.Tf(to, "bub.warDeclare", cao) || s.Bubbles[1].Text != i18n.Tf(to, "bub.warReply", liu) {
						t.Fatal("已移交佇列姓名未換語言")
					}
					if !reflect.DeepEqual(s.Bubbles, before) || g.RandSeed() != seed || g.RandDraws() != draws || s.SaveName(1, 8, "") != name {
						t.Fatal("換語言改變佇列、亂數或原始存檔名稱")
					}
					if err := s.Save(dir, 1, name); err != nil {
						t.Fatal(err)
					}
					after := mainBubbleSaveFiles(t, dir)
					if len(after) != len(files) {
						t.Fatal("換語言改變存檔檔案集合")
					}
					for file, b := range files {
						if !bytes.Equal(b, after[file]) {
							t.Fatalf("存檔 %s 隨顯示語言改變", file)
						}
					}
					old = to
				}
				pending := g.PendingEvents()
				if len(pending) != 3 || pending[0].Bubble.Scene != 5 || pending[1].Bubble.Text != i18n.Tf(finish, "bub.chiefOrder", i18n.PersonNameFor(finish, "關羽")) || pending[2].Bubble.Text != i18n.T(finish, "bub.chiefReply") {
					t.Fatal("未移交佇列或場景次序未保持")
				}
				if target.Name != "關羽" || g.Lord(0).Name != "劉備" {
					t.Fatal("顯示翻譯改寫原始姓名")
				}
			})
		}
	}
}

func mainBubbleSaveFiles(t *testing.T, dir string) map[string][]byte {
	t.Helper()
	files := map[string][]byte{}
	err := filepath.WalkDir(dir, func(path string, e fs.DirEntry, err error) error {
		if err != nil || e.IsDir() {
			return err
		}
		b, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		if e.Name() == "REMAKE.JSON" {
			// 寫檔時間本來就會更新；其餘所有額外存檔欄位仍完整比較。
			var m map[string]json.RawMessage
			if err := json.Unmarshal(b, &m); err != nil {
				return err
			}
			if _, ok := m["saved_at"]; !ok {
				t.Fatal("存檔時間欄位不存在")
			}
			delete(m, "saved_at")
			b, err = json.Marshal(m)
			if err != nil {
				return err
			}
		}
		name, err := filepath.Rel(dir, path)
		if err != nil {
			return err
		}
		files[name] = b
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(files) != 7 {
		t.Fatalf("存檔有 %d 檔，預期六個槽內檔案與共用名稱表", len(files))
	}
	return files
}
