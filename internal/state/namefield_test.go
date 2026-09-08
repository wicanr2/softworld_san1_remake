package state_test

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/wicanr2/softworld_san1_remake/internal/assets"
	"github.com/wicanr2/softworld_san1_remake/internal/state"
)

// 人物記錄的 offset 6 是姓名的結尾 `0`（`docs/spec/003` §2）。
//
// 姓名是 Big5 三個字、六個 byte，空白補齊——**三個字剛好填滿 0–5，
// 欄位裡沒有位置放結尾**，所以 offset 6 就是那個 `0`。原版把人物記錄的
// 遠指標直接推進 `%s`（例如 `%s主公, 有人在\n%s散佈謠言`，
// `docs/re/07` §5），沒有結尾就會一路印到下一個欄位去。
//
// 兩版六個劇本共 4200 個槽逐一驗過：offset 6 全部是 0，而姓名的
// 0–5 沒有一個 byte 是 0。
func TestGeneralNameFieldIsNulTerminated(t *testing.T) {
	root := os.Getenv("SAN1_ORIG")
	if root == "" {
		t.Skip("沒有原版素材")
	}
	slots := 0
	for _, ed := range []string{"三國演義", "三國演義1加強版"} {
	rd := func(ext string) []byte {
		b, err := os.ReadFile(filepath.Join(root, ed, "DATA2")+ext)
		if err != nil {
			t.Fatal(err)
		}
		return b
	}
	c, err := assets.OpenContainer(rd(".NAM"), rd(".IDX"), rd(".GRP"))
	if err != nil {
		t.Fatal(err)
	}
	for _, slot := range []string{"001", "002", "003", "004", "005", "006"} {
		sc, err := state.LoadScenario(c, state.Slot(slot))
		if err != nil {
			t.Fatal(err)
		}
		_, _, gen := sc.Tables()
		hist := map[byte]int{}
		nonzeroWithName := 0
		for i := 0; i+30 <= len(gen); i += 30 {
			hist[gen[i+6]]++
			if gen[i+6] != 0 && gen[i] != 0 {
				nonzeroWithName++
			}
		}
		if n := hist[0]; n != len(gen)/30 {
			t.Errorf("%s 劇本 %s：offset 6 只有 %d 個槽是 0，共 %d 個槽（分布 %v）",
				ed, slot, n, len(gen)/30, fmt.Sprint(hist))
		}
		if nonzeroWithName != 0 {
			t.Errorf("%s 劇本 %s：有 %d 筆有名字而 offset 6 非零", ed, slot, nonzeroWithName)
		}
		// 姓名到第一個 0 為止該是 6 個 byte——欄位裡沒有結尾的位置。
		lens := map[int]int{}
		for i := 0; i+30 <= len(gen); i += 30 {
			n := 0
			for n < 7 && gen[i+n] != 0 {
				n++
			}
			lens[n]++
		}
		slots += len(gen) / 30
		if lens[6] != len(gen)/30 {
			t.Errorf("%s 劇本 %s：姓名並非每一筆都填滿 0–5（長度分布 %v）",
				ed, slot, fmt.Sprint(lens))
		}
	}
}
	if slots == 0 {
		t.Fatal("一個槽都沒掃到——容器或劇本清單不對")
	}
	t.Logf("兩版六個劇本共 %d 個人物槽：offset 6 全部是 0，姓名都填滿 0–5", slots)
}
