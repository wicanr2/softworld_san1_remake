//go:build oracle

package parity

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/wicanr2/dosgolem/oracle"

	"github.com/wicanr2/softworld_san1_remake/internal/ai"
	"github.com/wicanr2/softworld_san1_remake/internal/save"
	"github.com/wicanr2/softworld_san1_remake/internal/session"
	"github.com/wicanr2/softworld_san1_remake/internal/state"
)

// TestRemakeSaveLoadsInOriginal 是存讀檔對拍的**寫出方向**：remake 存出來
// 的進度，讓原版讀進去，盤面逐格相同。
//
// 載入方向（原版存的檔 remake 讀得對）由 `TestOriginalSaveLoadsIdentically`
// 顧。兩個方向要分開驗——只驗載入的話，「remake 寫出來的位元組原版看不看
// 得懂」完全沒被問到，而那正是玩家存了檔之後拿回原版開的那條路。
//
// `internal/save` **不寫回原版的容器**（那是玩家自己的 `DATA2.GRP`，寫壞
// 沒有第二份），所以這裡的做法是：把整份原版目錄複製到暫存目錄，在**複製
// 品**的 `DATA2.GRP` 上就地換掉那一格的六個項目，再讓 dosgolem 從複製品開機。
// 原版目錄本身唯讀掛載，一個位元組都沒動到。
//
// 就地換成立的前提是**長度相同**：六個項目的長度由原版的版面決定
// （`docs/formats/05`），remake 寫出來的長度不一樣就代表版面寫錯了，
// 那本身就是要抓的錯，所以下面長度不符直接 fail 而不是重建容器。
func TestRemakeSaveLoadsInOriginal(t *testing.T) {
	root := origRoot(t)
	c := openContainer(t, filepath.Join(root, "DATA2"))

	// 起點取原版自己的第 1 個進度，再讓 remake 走一個月。
	//
	// **一定要動過盤面**：不動的話寫出去的位元組與那一格本來就幾乎一樣，
	// 原版讀回來相同也證明不了是「讀到我們寫的」還是「根本沒換到」。
	g, err := save.ReadOriginal(c, 1, state.EditionBase)
	if err != nil {
		t.Fatalf("remake 讀不了原版的第 1 個進度：%v", err)
	}
	from := g.Date
	// **整個月都要走**，不只月底結算：`State.EndMonth` 只做換月、物價、
	// 洗牌與四季，人物表一個位元組都不會動——那樣諸侯表與人物表的
	// 寫出路徑就只驗到「原封不動搬一遍」。`session.EndMonth` 會先讓
	// 各郡的電腦諸侯行動，徵兵、訓練、賞賜都會落到人物表上。
	brain, err := ai.New(ai.ModeBase)
	if err != nil {
		t.Fatal(err)
	}
	session.New(g, brain, g.Player).EndMonth()
	t.Logf("remake 從 %d 年 %d 月走完一個月到 %d 年 %d 月，再存進第 1 格",
		from.Year, from.Month, g.Date.Year, g.Date.Month)

	out := t.TempDir()
	if err := save.Write(out, 1, g, "對拍"); err != nil {
		t.Fatalf("remake 存不出去：%v", err)
	}

	// remake 寫出去的六個項目 → 容器裡對應的項目名。
	written := map[string]string{
		"BASEMAS.SV1":  filepath.Join(out, "SV1", "BASEMAS.SV1"),
		"BASESTA.SV1":  filepath.Join(out, "SV1", "BASESTA.SV1"),
		"BASEGEN.SV1":  filepath.Join(out, "SV1", "BASEGEN.SV1"),
		"BASEPRO.SV1":  filepath.Join(out, "SV1", "BASEPRO.SV1"),
		"BASEPRE.SV1":  filepath.Join(out, "SV1", "BASEPRE.SV1"),
		"SAVENAME.SVP": filepath.Join(out, "SAVENAME.SVP"),
	}

	dir := copyGameDir(t, root)
	grp, err := os.ReadFile(filepath.Join(dir, "DATA2.GRP"))
	if err != nil {
		t.Fatal(err)
	}
	changed := 0
	for name, path := range written {
		b, err := os.ReadFile(path)
		if err != nil {
			t.Fatalf("remake 沒寫出 %s：%v", name, err)
		}
		i, ok := c.ByName(name)
		if !ok {
			t.Fatalf("原版容器裡沒有 %s", name)
		}
		e := c.Entry(i)
		if int(e.Size()) != len(b) {
			t.Fatalf("%s：原版 %d 個位元組，remake 寫了 %d 個——版面對不上",
				name, e.Size(), len(b))
		}
		// **負對照**：換進去的內容要與原版那一格本來的不同。
		// 兩邊本來就一樣的話，「原版讀回來相同」證明不了是讀到我們寫的
		// ——也可能是根本沒換到。
		was := 0
		for i, x := range b {
			if grp[int(e.Start)+i] != x {
				was++
			}
		}
		changed += was
		copy(grp[e.Start:e.End], b)
		t.Logf("換掉 %-12s %#08x 起 %5d 個位元組（與原本那一格差 %d 個）",
			name, e.Start, e.Size(), was)
	}
	if changed == 0 {
		t.Fatal("remake 寫出去的六個項目與原版那一格逐位元組相同——" +
			"這樣比不出「原版讀到的是我們寫的」")
	}
	if err := os.WriteFile(filepath.Join(dir, "DATA2.GRP"), grp, 0o644); err != nil {
		t.Fatal(err)
	}

	mas, sta, gen, err := g.Tables()
	if err != nil {
		t.Fatalf("remake 的盤面寫不回三張表：%v", err)
	}
	mine := append(append(append([]byte{}, mas...), sta...), gen...)

	o, err := oracle.Load(filepath.Join(dir, "AA.EXE"), dir)
	if err != nil {
		t.Fatalf("換過存檔的原版載不起來：%v", err)
	}
	defer o.Close()

	snaps := bootLoadedSave(t, o, len(mine))
	dumpScreen(t, o, "remake存檔載進原版")

	nMas, nSta := state.MasterTableSize, state.PrefectureTableSize
	for i, s := range snaps {
		t.Logf("載入序列第 %d 個檢查點：與 remake 寫出去的差 %d 個位元組",
			i, differs8(s, mine))
	}
	atLoad := snaps[0]
	if n := differs8(atLoad, mine); n != 0 {
		t.Errorf("原版讀進去的盤面與 remake 寫出去的差 %d 個位元組%s\n%s",
			n, where(atLoad, mine, nMas, nSta), byPrefecture(atLoad, mine, nMas, nSta))
	} else {
		t.Logf("三張表 %d 個位元組：原版讀進去的與 remake 寫出去的完全相同", len(mine))
	}
}

// boardBase 是原版把三張表載到哪裡（線性位址）。
//
// **走「載入舊進度」時搜不到**：搜尋樣本是劇本的位元組，而存檔的盤面
// 與劇本不同。這個值是量出來的（`bootToMain` 的退路用的是同一個）。
// 拿諸侯表去搜會搜到檔案緩衝區——量到的一次在 `0x85fe6`，那份只是
// 剛讀進來的檔案內容，往後 19,220 個位元組並不是盤面。
const boardBase = 0x399b0

// bootLoadedSave 開機、選「載入舊進度 → 第 1 格」，並在載入流程的每一步
// 各取一次盤面快照。
//
// 與 `bootToMain` 的差別是**送完鍵不強求畫面會動**：換過存檔之後流程會
// 提早走到防拷密碼關，固定序列的最後一下就落在等文字輸入的提示上，
// 畫面當然不動——那不是載入失敗。
func bootLoadedSave(t *testing.T, o *oracle.Oracle, total int) [][]byte {
	t.Helper()
	o.Press("122")
	send := map[int]string{4: "\r", 17: "2", 23: "1"}
	for i := 0; i < 27; i++ {
		if err := o.Run(50_000_000); err != nil {
			t.Fatalf("停止：%v", err)
		}
		if k, ok := send[i]; ok {
			o.Press(k)
		}
	}
	snaps := [][]byte{append([]byte(nil), o.Bytes(addr(boardBase), total)...)}

	diff := "5"
	if v := os.Getenv("SAN1_DIFFICULTY"); v != "" {
		diff = v
	}
	for _, keys := range []string{"1\r", "\r", diff + "\r", "\r", "\r"} {
		if err := o.Run(150_000_000); err != nil {
			t.Fatalf("沉澱時停止：%v", err)
		}
		o.Drain()
		o.PressScan(keys)
		if err := o.Run(150_000_000); err != nil {
			t.Fatalf("停止：%v", err)
		}
		snaps = append(snaps, append([]byte(nil), o.Bytes(addr(boardBase), total)...))
	}
	return snaps
}

// copyGameDir 把原版目錄複製一份到暫存目錄，回傳複製品的路徑。
//
// 只複製最上層的檔案：`.jsdos/` 是 js-dos 的設定，DOS 那一側用不到。
func copyGameDir(t *testing.T, src string) string {
	t.Helper()
	dst := t.TempDir()
	ents, err := os.ReadDir(src)
	if err != nil {
		t.Fatal(err)
	}
	n := 0
	for _, e := range ents {
		if e.IsDir() {
			continue
		}
		b, err := os.ReadFile(filepath.Join(src, e.Name()))
		if err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(dst, e.Name()), b, 0o644); err != nil {
			t.Fatal(err)
		}
		n++
	}
	t.Logf("原版目錄複製了 %d 個檔到 %s（原目錄唯讀，不動它）", n, dst)
	return dst
}
