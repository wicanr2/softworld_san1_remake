package i18n

import "testing"

// TestPinyinCoversEveryCharacter 釘住拼音表沒有漏字。
//
// 用的是**這個遊戲真的會出現的字**，不是一份通用字表：漏一個字在畫面上
// 是整個名字退回原文，看起來像資料壞掉而不像沒翻到。
//
// 字表本身在 `internal/i18n/names.go`；要重新盤點用
// `tools/go.sh run ./cmd/san1names -root <原版目錄>`。
func TestPinyinCoversEveryCharacter(t *testing.T) {
	// 346 位人物 ＋ 42 個郡用到的 426 個相異字，加上十四個州名多用到的 13 個
	//（州名先前沒算進來，英文的郡資料面板上「豫州」就一直是中文）。
	// 并、兗是原版「弁州」「充州」的本字——譯文換回本字再轉（`placeFix`）。
	const chars = "丁上下丕中之乾于京亮仁任伉伊休侯信修倉傅傕備儀儒優允兆先全公典冷凌凱刑剛劉勳化北" +
		"匡卓南原叡史司同向吳呂周和嘉嚴圃圖堅堪士夏天太夷奉姜威孔孟孫安宋宓定宜宮寧審寵封" +
		"尚就岑岱峻崔嶷川巴巽布師平幹度庶廉廖廬延建式弘張彤彧彪彭彰徐循德志忠性恢恪惇慈憲" +
		"應懷懿成授操攸敘文旋旻昂昌昭昱晃普曄曠曹會朗朱李東松林柏柯柴桂桑桓梁植楊楙業楷榮" +
		"樂樊橋橫權欽歆正步武毓毗毛水永氾汜沙沛沮治泉法泰洛洪浩海涿淮淳淵渤溫滿漢潁潘澤濟" +
		"焉焦然煥熊熙燕爽牂牛獲玄王玠玩珪班琅琦琬琮琰琳瑁瑜瑾璋璜瓊瓚甘生田留當疆登皎盛真" +
		"矯磐祖禁禪秉秋秦程稠竺策範簡籍粲紀純紘索累紹統綜維綱績繇繡續羕群義羽翊翔翻翼聘肅" +
		"胡胤臧興良艾芝芳苞茂范荀華萌著葛董蒙蒯蓋蔡蔣蕤薄薛蘭虎虔虞融術衛表袁裔褒褘褚襄襲" +
		"覽觀觸許評詡詩誕諶諸謖謙譙譚豐豹貴費賈賢超越趙軫輔辛辟農通逢進逵遂道達遜遵選遼邈" +
		"邪邳郃郝郡郭都鄂鄧鄴配酒醜金銀鍾鐵長閻闓關闞阜陳陵陶陸陽雄雍雙雲零雷霍霸靈靖鞏韋" +
		"韓順顏顗顧飛馬騭騰高髦鬱魏魯魴鮑鴛麋麴黃黨齊齡龍龐龔" +
		"交兗冀州幽并揚涼益荊豫隸青"

	n := 0
	for _, r := range chars {
		n++
		if _, ok := pinyin[r]; !ok {
			t.Errorf("拼音表少了 %q", string(r))
		}
	}
	if n != 439 {
		t.Errorf("字表有 %d 個字，盤點時是 439 個", n)
	}
	// 反向：表裡不該有用不到的字。多出來的字是死資料，改的時候不會有人記得。
	seen := map[rune]bool{}
	for _, r := range chars {
		seen[r] = true
	}
	for r := range pinyin {
		if !seen[r] {
			t.Errorf("拼音表多了 %q，這個遊戲用不到", string(r))
		}
	}
	if len(pinyin) != n {
		t.Errorf("拼音表有 %d 條，字表有 %d 個字", len(pinyin), n)
	}
}

// TestPersonNames 釘住幾個會踩到斷詞與多音字的名字。
func TestPersonNames(t *testing.T) {
	old := Current
	defer func() { Current = old }()

	for _, c := range []struct{ zh, en, ja string }{
		{"劉備", "Liu Bei", "劉備"},
		{"關羽", "Guan Yu", "関羽"},
		{"諸葛亮", "Zhuge Liang", "諸葛亮"},  // 複姓：不斷詞會變成 Zhu Geliang
		{"司馬懿", "Sima Yi", "司馬懿"},      // 複姓
		{"夏侯惇", "Xiahou Dun", "夏侯惇"},   // 複姓
		{"淳于瓊", "Chunyu Qiong", "淳于瓊"}, // 複姓
		{"太史慈", "Taishi Ci", "太史慈"},    // 複姓
		{"公孫瓚", "Gongsun Zan", "公孫瓚"},  // 複姓
		{"樂進", "Yue Jin", "楽進"},        // 多音字：不是 Le
		{"沮授", "Ju Shou", "沮授"},        // 多音字：不是 Zu
		{"逢紀", "Pang Ji", "逢紀"},        // 多音字：不是 Feng
		{"劉禪", "Liu Shan", "劉禅"},       // 多音字：不是 Chan
		{"鍾繇", "Zhong Yao", "鍾繇"},      // 多音字：不是 You
		{"張郃", "Zhang He", "張郃"},       // 多音字
		{"傅士仁", "Fu Shiren", "傅士仁"},    // 兩個字的名連寫
		{"黃忠", "Huang Zhong", "黄忠"},
		{"孫權", "Sun Quan", "孫権"},
		{"嚴顏", "Yan Yan", "厳顔"},
		{"魯肅", "Lu Su", "魯粛"},
		{"吳懿", "Wu Yi", "呉懿"},
		{"龐德", "Pang De", "龐徳"},
	} {
		Current = En
		if got := PersonName(c.zh); got != c.en {
			t.Errorf("%s 的英文是 %q，應該是 %q", c.zh, got, c.en)
		}
		Current = Ja
		if got := PersonName(c.zh); got != c.ja {
			t.Errorf("%s 的日文是 %q，應該是 %q", c.zh, got, c.ja)
		}
		Current = ZhHant
		if got := PersonName(c.zh); got != c.zh {
			t.Errorf("%s 的繁中被改成 %q——繁中是原文不是譯文", c.zh, got)
		}
	}
}

// TestPlaceNames 釘住地名連寫，不拆姓名。
func TestPlaceNames(t *testing.T) {
	old := Current
	defer func() { Current = old }()

	for _, c := range []struct{ zh, en, ja string }{
		{"遼東", "Liaodong", "遼東"},
		{"涿郡", "Zhuojun", "涿郡"},
		{"潁川", "Yingchuan", "潁川"},
		{"上黨", "Shangdang", "上党"},
		{"齊郡", "Qijun", "斉郡"},
		{"吳郡", "Wujun", "呉郡"},
		{"琅邪", "Langya", "琅邪"},
		{"鬱林", "Yulin", "鬱林"},
		{"牂柯", "Zangke", "牂柯"},
	} {
		Current = En
		if got := PlaceName(c.zh); got != c.en {
			t.Errorf("%s 的英文是 %q，應該是 %q", c.zh, got, c.en)
		}
		Current = Ja
		if got := PlaceName(c.zh); got != c.ja {
			t.Errorf("%s 的日文是 %q，應該是 %q", c.zh, got, c.ja)
		}
	}
}

// TestUnknownNameStaysWhole 釘住翻不出來的名字整個保留原文。
//
// 半翻的名字（`Liu 備`）比不翻更糟：它看起來像資料壞掉，
// 而不像「這個字還沒收進表裡」。
func TestUnknownNameStaysWhole(t *testing.T) {
	old := Current
	defer func() { Current = old }()
	Current = En
	if got := PersonName("劉甲"); got != "劉甲" {
		t.Errorf("翻不出來的名字變成 %q，應該整個保留原文", got)
	}
	if got := PlaceName("甲郡"); got != "甲郡" {
		t.Errorf("翻不出來的地名變成 %q，應該整個保留原文", got)
	}
}

// TestProvinceTyposAreFixedOnlyInTranslations 釘住原版的兩個州名錯字：
// 中文照原版資料（「弁州」「充州」），譯文換回本字（并州、兗州）。
func TestProvinceTyposAreFixedOnlyInTranslations(t *testing.T) {
	saved := Current
	defer func() { Current = saved }()
	for _, c := range []struct {
		l          Locale
		bian, chong string
	}{
		{ZhHant, "弁州", "充州"},
		{En, "Bingzhou", "Yanzhou"},
		{Ja, "并州", "兗州"},
	} {
		Current = c.l
		if got := PlaceName("弁州"); got != c.bian {
			t.Errorf("%s：弁州 → %q，想要 %q", c.l, got, c.bian)
		}
		if got := PlaceName("充州"); got != c.chong {
			t.Errorf("%s：充州 → %q，想要 %q", c.l, got, c.chong)
		}
	}
	// 面板放不下時會拿掉「州」字再轉，單字也要換回本字。
	Current = En
	if got := PlaceName("弁"); got != "Bing" {
		t.Errorf("英文的「弁」→ %q，想要 Bing", got)
	}
}
