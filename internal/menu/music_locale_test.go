package menu

import (
	"fmt"
	"github.com/wicanr2/softworld_san1_remake/internal/i18n"
	"testing"
)

func TestMusicPlateRelocalizesByStage(t *testing.T) {
	previous := i18n.Current
	defer func() { i18n.Current = previous }()
	for _, start := range i18n.Locales() {
		i18n.Current = start
		s := &Screen{stage: Music, title: i18n.S("title.musicPlate")}
		for k := 1; k <= 5; k++ {
			s.items = append(s.items, i18n.S(fmt.Sprintf("title.song%d", k)))
		}
		s.items = append(s.items, "")
		// 直接用穩定鍵取預期值，避免 Music 的同文回譯歧義。
		for _, target := range []i18n.Locale{i18n.En, i18n.Ja, i18n.ZhHant, i18n.Ja, i18n.En, start} {
			old := i18n.Current
			i18n.Current = target
			s.Relocalize(old)
			if s.Title() != i18n.T(target, "title.musicPlate") {
				t.Fatalf("%s→%s: %q，期望 %q", old, target, s.Title(), i18n.T(target, "title.musicPlate"))
			}
			if len(s.Items()) != 6 || s.Items()[5] != "" {
				t.Fatal("五曲及空白第六格改變")
			}
		}
	}
}
