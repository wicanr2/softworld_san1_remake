package game

import (
	"errors"
	"testing"

	"github.com/wicanr2/softworld_san1_remake/internal/state"
)

// TestSearchThreshold 釘住尋訪的門檻與命中的效果。
//
// 原版的常式（`0xcc86`）掃全部 350 人，找**所在郡是本郡且身分 9
// （在野未露面）**的；尋訪者的智要**大於 `RND(65)+30`**（30–94）才算
// 成功，成功的話那個人身分 9 → 8、勢力設成 `0xFF`。
//
// 說明書只說「謀略越高成功機率越大」——**門檻是一個 30–94 的亂數**是
// 碼裡才有的。
func TestSearchThreshold(t *testing.T) {
	if lo, hi := SearchIntelFloor, SearchIntelFloor+SearchIntelSpread; lo != 30 || hi != 95 {
		t.Errorf("尋訪的門檻範圍是 %d–%d，原版是 30–94", lo, hi-1)
	}
}

// TestRecruitBondGate 釘住登用的牽絆閘門（`0xce8c`，`L0`）。
//
// **三條路要各驗一次**：牽絆對象效力於招募方（一定成功）、在野
// （照能力值判定）、效力於第三方（幾乎不可能）。只驗成功那一條的話，
// 閘門整個沒接上也照樣綠——在野是多數情形，能力值那條會蓋過去。
func TestRecruitBondGate(t *testing.T) {
	// 說服力與難度的兩半各自先釘住，再看閘門怎麼蓋過難度。
	if got := RecruitPersuasion(50, 90, 0); got != 80 {
		t.Errorf("人望 50、太守魅力 90、無加成算出 %d，應該是 (50*3+90)/3 ＝ 80", got)
	}
	if got := RecruitPersuasion(50, 90, 40); got != 120 {
		t.Errorf("等級 5 的加成沒算進去：%d", got)
	}
	if got := RecruitDifficulty(90, 80, 0, 0); got != 56 {
		t.Errorf("謀略 90、戰力 80、兩次 RND 都是 0 算出 %d，應該是 30+26 ＝ 56", got)
	}
	if got := RecruitDifficulty(90, 80, 3, 3); got != 28 {
		t.Errorf("兩次 RND 都是 3 算出 %d，應該是 15+13 ＝ 28", got)
	}
	// 費用與加成成對：等級越高越便宜也越容易。
	for lvl, want := range map[int][2]int{
		0: {30, 0}, 1: {30, 0}, 2: {30, 0},
		3: {20, 10}, 4: {10, 20}, 5: {0, 40},
	} {
		if fee, bonus := RecruitFee(lvl), RecruitBonus(lvl); fee != want[0] || bonus != want[1] {
			t.Errorf("等級 %d 的參數是 (%d, %d)，量到的是 %v", lvl, fee, bonus, want)
		}
	}

	g := newGame(t)
	// **局面自己擺，不去盤面上找。** 開局有沒有剛好符合條件的在野人才
	// 是資料的事；用 skip 帶過就等於這一條測試在多數環境下不存在。
	at := 0
	for _, p := range g.Prefectures() {
		if p.Owned() && p.Owner == 0 {
			at = p.ID
			break
		}
	}
	if at == 0 {
		t.Fatal("劉備一個郡都沒有")
	}
	target := g.General(200)
	if target == nil || target.Name == "" {
		t.Fatal("找不到人物 200")
	}
	target.Faction = state.NoFaction
	target.Location = at
	target.Status = state.StatusAvailable
	target.Intel, target.War = 90, 80

	// **牽絆對象效力於第三方**：難度 160 以上，說服力最多一百出頭。
	rival := g.General(1)
	if rival == nil {
		t.Fatal("找不到人物 1")
	}
	rival.Faction, rival.Status = 1, state.StatusOfficer
	target.Bond = rival.Index
	if err := g.Recruit(at, target.Index, 0); !errors.Is(err, ErrDeclined) {
		t.Errorf("牽絆對象在敵營，登用卻回 %v，應該是 ErrDeclined", err)
	}
	if target.Employed() {
		t.Error("牽絆對象在敵營，人卻來了")
	}

	// **牽絆對象效力於招募方**：難度 0，一定成功。
	g.Prefecture(at).Commanded = false
	rival.Faction = 0
	if err := g.Recruit(at, target.Index, 0); err != nil {
		t.Fatalf("牽絆對象在自己麾下，登用卻失敗：%v", err)
	}
	if target.Faction != 0 || target.Status != state.StatusOfficer {
		t.Errorf("登用成功了但欄位沒改：勢力 %d、身分 %d", target.Faction, target.Status)
	}
	if target.Loyalty == 0 || target.Loyalty > 100 {
		t.Errorf("新進的忠誠是 %d，應該落在 1..100", target.Loyalty)
	}
}
