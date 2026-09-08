package ui

import (
	"image/color"
	imgpng "image/png"
	"os"
	"path/filepath"
	"testing"

	"github.com/wicanr2/softworld_san1_remake/internal/assets"
	"github.com/wicanr2/softworld_san1_remake/internal/state"
)

// TestPrefectureFillsMatchTheOriginal 把州郡的填色對回原版。
//
// 基準畫面是 `internal/parity` 的 `TestZZOriginalLoadedScreen` 存的
// `orig-loaded.png`：原版在主選單送 `2` 再送 `1`、**剛載完第一個進度**
// 的主畫面。
//
// ⚠ 不能用 `orig-main.png`——那一張走的是 `bootToGame`，
// 觸發防拷密碼的那道指令已經把玩家的第一個月用掉了，郡會易主
// （實測長沙就從第 2 個勢力換到第 0 個），而那看起來與
// 「填色的對應表錯了」一模一樣。
//
// 判準是**每一個郡的種子點**（地圖座標加原點）上的顏色：原版的填色
// 是 8×8 的網點（`EGAFILL.PAL`），拿單一顏色比會在十二個勢力上失敗。
func TestPrefectureFillsMatchTheOriginal(t *testing.T) {
	f, err := os.Open("../../workplace/shots/orig-loaded.png")
	if err != nil {
		t.Skipf("沒有主畫面的基準畫面：%v", err)
	}
	shot, err := imgpng.Decode(f)
	f.Close()
	if err != nil {
		t.Fatal(err)
	}
	root := os.Getenv("SAN1_ORIG")
	if root == "" {
		t.Skip("沒設 SAN1_ORIG，跳過（本儲存庫不含原版檔案）")
	}
	dir := filepath.Join(root, "三國演義")
	open := func(name string) *assets.Container {
		read := func(ext string) []byte {
			b, err := os.ReadFile(filepath.Join(dir, name+"."+ext))
			if err != nil {
				t.Skipf("讀不到 %s.%s：%v", name, ext, err)
			}
			return b
		}
		c, err := assets.OpenContainer(read("NAM"), read("IDX"), read("GRP"))
		if err != nil {
			t.Fatal(err)
		}
		return c
	}
	c2, c3, c1 := open("DATA2"), open("DATA3"), open("DATA1")
	// **所屬要讀存檔裡的那一份，不是載入之後重算的。**
	// 地圖是「載完就畫」、之後不重畫，而原版載完存檔會把郡的所屬重算
	// 一次（`0x1e394`，`save.ReadOriginal` 照做）——重算是在畫完之後，
	// 所以畫面上的顏色是存檔的值。潁川（存檔 5、記憶體 4）與南海
	// （存檔無主、記憶體 14）兩格會因此看起來像「填色的對應表錯了」。
	g, err := state.LoadScenario(c2, state.Slot("SV1"))
	if err != nil {
		t.Fatalf("讀原版第一個進度：%v", err)
	}
	a, err := NewArtScreen(c3, c1)
	if err != nil {
		t.Fatal(err)
	}
	if a.fills == nil {
		t.Fatal("EGAFILL.PAL 沒讀進來")
	}

	// **逐格比整塊，不是只比種子點**：網點只有兩個顏色，單看一格
	// 有一半的圖樣都對得上，比出來的「相符」不算數。
	//
	// 一個郡一份底圖的副本，只填它自己——這樣「這一格是誰填的」不會
	// 被別的郡剛好同色的格子混進來。
	owned := 0
	var badPref []int
	for _, p := range g.Prefectures() {
		if p.Owner == state.NoFaction {
			continue
		}
		owned++
		one := a.base.Clone()
		x := int(p.MapX) + assets.MapOriginX
		y := int(p.MapY) + assets.MapOriginY
		one.FloodFillPattern(x, y, &a.fills[int(p.Owner)%len(a.fills)])
		bad, n := 0, 0
		for py := 0; py < assets.ScreenH; py++ {
			for px := 0; px < assets.ScreenW; px++ {
				if one.At(px, py) == a.base.At(px, py) {
					continue
				}
				n++
				want := color.RGBAModel.Convert(shot.At(px, py)).(color.RGBA)
				if assets.EGAPalette[one.At(px, py)&15] != want {
					bad++
				}
			}
		}
		if n == 0 {
			t.Errorf("郡 %d（%s）一格都沒填到", p.ID, p.Name)
			continue
		}
		// 容 5%：郡的編號寫在填色上面，那幾格本來就不同。
		if bad*20 > n {
			badPref = append(badPref, p.ID)
			t.Logf("郡 %d（%s，勢力 %d）：填了 %d 格，其中 %d 格與原版不同",
				p.ID, p.Name, p.Owner, n, bad)
		}
	}
	t.Logf("有主的郡 %d 個，填色對不上 %d 個", owned, len(badPref))
	if owned < 20 {
		t.Errorf("只有 %d 個郡有主，進度大概沒讀對", owned)
	}
	// **例外要列名，不能只數個數**：只寫「最多錯一個」的話，換成別的郡
	// 錯了照樣綠燈。
	want := []int{knownFillMismatch}
	if len(badPref) != len(want) || (len(badPref) == 1 && badPref[0] != want[0]) {
		t.Errorf("對不上的是 %v，已知的例外只有 %v", badPref, want)
	}
}

// knownFillMismatch 是唯一一個填色對不上的郡。
//
// 長沙（勢力 2 的本據，而且是那個勢力唯一的郡）在原版畫面上是**圖樣 0**
// ——純淺紅，而它的所屬寫在州郡記錄 offset 30 是 2，圖樣 2 是純淺綠。
// 803 格逐格都不同，不是網點相位的問題。
//
// 排除過的解釋：不是玩家的顏色（這個進度的玩家是勢力 14，沒有領地）、
// 不是本據的顏色（另外十一個本據都畫自己勢力的圖樣）、
// 不是「只有一個郡的勢力」（那樣的勢力有七個，其餘六個都對）、
// 也不是太守與君主的所屬不一致（逐郡比過，沒有不一致）；
// **原版執行期記憶體裡的所屬也是 2**（`internal/parity` 的
// `TestLoadedPrefectureOwnersMatchTheSave`），所以不是 remake 讀錯欄位；
// 州郡記錄位移 0–54 沒有任何一格滿足「34 個郡等於所屬、長沙等於 0」，
// 諸侯記錄前 24 個位元組也沒有一格等於自己的槽號；
// **不是「後灌蓋先灌」**（`TestPrefectureFillsInOrderMatchTheOriginal`）。
// 原因未解。
const knownFillMismatch = 31
