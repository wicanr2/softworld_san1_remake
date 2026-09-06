package game

import "testing"

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
