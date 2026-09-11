package assets

import "testing"

// 遮罩只涵蓋山脊那一帶，界外要明確回答。
func TestCreditsSkyAtOutsideTheMask(t *testing.T) {
	cr := &Credits{Mask: &Image{W: 4, H: 2, Pix: []byte{1, 1, 0, 0, 1, 0, 0, 0}}}
	// 遮罩之上一律是天空——**包含負的 y**：字幕捲到畫面頂端時還在天空裡。
	for _, y := range []int{-100, -1, 0, CreditMaskY - 1} {
		if !cr.SkyAt(0, y) {
			t.Errorf("y=%d 應該是天空", y)
		}
	}
	// 遮罩之內看遮罩。
	if !cr.SkyAt(1, CreditMaskY) || cr.SkyAt(2, CreditMaskY) {
		t.Error("遮罩之內沒有照遮罩走")
	}
	if !cr.SkyAt(0, CreditMaskY+1) || cr.SkyAt(1, CreditMaskY+1) {
		t.Error("遮罩第二列沒有照遮罩走")
	}
	// 遮罩之下與左右界外都不是天空——字幕到那裡就該被擋住。
	for _, p := range [][2]int{{0, CreditMaskY + 2}, {-1, CreditMaskY}, {4, CreditMaskY}} {
		if cr.SkyAt(p[0], p[1]) {
			t.Errorf("(%d,%d) 不該是天空", p[0], p[1])
		}
	}
	// 沒有遮罩時一律不是天空（**不要變成全部可見**：那會讓缺素材
	// 看起來像「字幕壞掉」而不是「素材沒讀到」）。
	var none *Credits
	if none.SkyAt(0, 0) || (&Credits{}).SkyAt(0, 0) {
		t.Error("沒有遮罩卻回天空")
	}
}

// 整組素材缺一個就要報錯。
func TestLoadCreditsNeedsEverything(t *testing.T) {
	if _, err := LoadCredits(nil); err == nil {
		t.Error("沒有容器卻沒報錯")
	}
}

// 原版那一組：尺寸、條數，以及**遮罩與山景對不對得上**。
func TestCreditsAssetsMatchTheOriginal(t *testing.T) {
	cr, err := LoadCredits(container(t, "DATA2"))
	if err != nil {
		t.Fatal(err)
	}
	for i, b := range cr.Backdrop {
		if b.W != CreditBackdropW || b.H != CreditBackdropH {
			t.Errorf("山景 %d 是 %d×%d，想要 %d×%d",
				i, b.W, b.H, CreditBackdropW, CreditBackdropH)
		}
	}
	if cr.Hall.W != CreditBackdropW || cr.Hall.H != CreditBackdropH {
		t.Errorf("朝堂圖是 %d×%d", cr.Hall.W, cr.Hall.H)
	}
	if cr.Mask.W != 640 || cr.Mask.H != 151 {
		t.Errorf("遮罩是 %d×%d，想要 640×151", cr.Mask.W, cr.Mask.H)
	}
	if len(cr.Lines) != CreditLines {
		t.Fatalf("字幕有 %d 條，想要 %d", len(cr.Lines), CreditLines)
	}

	// **遮罩是 `REC10` 的天空遮罩**：把遮罩的 1 當天空、0 當山，
	// 與圖裡「這一格是不是天空色」比，位移 CreditMaskY 時逐點吻合。
	//
	// 這個數字是這一條的判準：素材換了、位移記錯了、平面順序解錯了，
	// 吻合率都會掉下來，而畫面上只會看起來「字被切得怪怪的」。
	sky := cr.Backdrop[0].Pix[0] // 最上面一列就是天空
	same, tot := 0, 0
	for y := 0; y < cr.Mask.H; y++ {
		for x := 0; x < cr.Mask.W; x++ {
			isSky := cr.Backdrop[0].At(x, y+CreditMaskY) == sky
			want := cr.Mask.Pix[y*cr.Mask.W+x] != 0
			tot++
			if isSky == want {
				same++
			}
		}
	}
	got := float64(same) / float64(tot)
	t.Logf("遮罩與 REC10 逐點吻合 %.2f%%（天空色 %d）", 100*got, sky)
	if got < 0.99 {
		t.Errorf("只吻合 %.2f%%——遮罩對的不是這張圖，或位移不是 %d",
			100*got, CreditMaskY)
	}
	// 正對照：**另一張山景不該對得上**，否則這個判準等於沒在看。
	same2, tot2 := 0, 0
	sky2 := cr.Backdrop[1].Pix[0]
	for y := 0; y < cr.Mask.H; y++ {
		for x := 0; x < cr.Mask.W; x++ {
			isSky := cr.Backdrop[1].At(x, y+CreditMaskY) == sky2
			want := cr.Mask.Pix[y*cr.Mask.W+x] != 0
			tot2++
			if isSky == want {
				same2++
			}
		}
	}
	if other := float64(same2) / float64(tot2); other > 0.9 {
		t.Errorf("REC11 也吻合 %.2f%%——這個判準分不出兩張圖", 100*other)
	} else {
		t.Logf("正對照：REC11 只吻合 %.2f%%", 100*other)
	}
}
