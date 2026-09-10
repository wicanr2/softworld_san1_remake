//go:build oracle

package parity

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/wicanr2/dosgolem/oracle"

	"github.com/wicanr2/softworld_san1_remake/internal/state"
)

// 誰把郡 6 的金從 30000 扣成 22000？
//
// 月度對拍在新 base 上差 145，第一個岔開在順序表第 0 格（郡 6）。而兩邊的
// 亂數狀態在起點與進貢前都相同，開月那整段也同步——**岔開的是盤面不是
// 序列**：
//
//	出發盤面（共同起點）  金 30000  米 11421  兵(百) 75
//	原版 郡 6 回合開始    金 22000  米  8376  兵(百) 55
//	remake 郡 6 回合開始  金 30000  米 11421  兵(百) 75   ← 沒動
//
// 差 8000 金、3045 米、2000 兵，而同一段也動了「在職將」與「主事者」
// ——那是出兵的形狀，可是出兵該在郡回合裡，不在開月階段。
//
// **推測到此為止。** 靜態 xref 只證明位址被寫進程式碼裡，不證明那條路走得到
// （`CONTEXT.md` R22）；「誰寫這個變數」唯一直接的答案是攔寫入
// （`CLAUDE.md` §4.1）。這一支就攔那四個位元組，把寫入者的位址印出來。
func TestZZWhoTakesGoldFromPrefecture6(t *testing.T) {
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
	seq := strings.Split(envOr("SAN1_TURNKEY", "4\r|4\r|Y"), "|")
	total := state.MasterTableSize + state.PrefectureTableSize + state.GeneralTableSize
	before := driveToMonthStart(t, o, seq, base, total, 0x13579BDF)

	const watched = 6
	at := state.MasterTableSize + watched*176
	t.Logf("出發：郡 %d 金 %d 米 %d 兵(百) %d",
		watched,
		int(before[at+18])|int(before[at+19])<<8,
		int(before[at+20])|int(before[at+21])<<8,
		int(before[at+16])|int(before[at+17])<<8)

	// 金（offset 18）、米（20）、兵士（16）各兩個位元組，一起攔。
	lo := base + uint32(at) + 16
	hi := base + uint32(at) + 21
	writes := o.WatchWritesAt(lo, hi)

	// 跑到第一個郡的回合入口就停——問題發生在那之前。
	got := false
	o.OnCall(addr(0x1746e), func(*oracle.Oracle) { got = true })
	for i := 0; i < 12 && !got; i++ {
		if err := o.Run(40_000_000); err != nil {
			t.Fatalf("跑到第一個郡的回合入口時：%v", err)
		}
	}
	if !got {
		t.Fatal("沒走到第一個郡的回合入口")
	}

	t.Logf("郡 %d 的金／米／兵士在「結算 → 第一個郡」之間被寫 %d 次：",
		watched, len(*writes))
	// 同一個 IP 連續寫多個位元組是一次 16 位元寫入，收攏起來看。
	last := oracle.Addr{}
	n := 0
	for _, w := range *writes {
		if w.IP == last {
			continue
		}
		last = w.IP
		n++
		if n > 30 {
			t.Logf("  …（還有更多）")
			break
		}
		// **兩個位址都印**：`IDA` 那一欄只能拿去 IDA 查，
		// 攔截點（`o.OnCall`）與 `objdump` 要的是**線性位址**，
		// 兩者差 `0xEF00`。只印一個就會被拿去用在另一個地方，
		// 而反組譯落在別的函式上照樣讀得通（`CONTEXT.md` R50）。
		t.Logf("  [%d] 位移 %04X：%02X → %02X　寫入者 %s（線性 %05X／IDA %05X）",
			w.Step, w.Off, w.Old, w.New, w.IP, w.IP.Linear(), o.ToIDA(w.IP))
	}
}
