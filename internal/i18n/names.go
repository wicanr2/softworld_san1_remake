package i18n

import (
	"strings"
	"unicode"
)

// 人名與地名的英日對照。
//
// **繁中是原文不是譯文**：名字來自玩家自己那一份原版檔案，本套件不存
// 中文名，只存「怎麼把它換成英文或日文」的規則。
//
// 做法是**字元層**不是逐個名字：346 位人物 ＋ 42 個郡 ＋ 14 個州用到
// 439 個相異字，
// 一個字一條規則比 388 條名字短，而且加劇本、換版本都不必重寫。
// 涵蓋率由測試盯著——少一個字就會露出原文，那在畫面上看起來像沒翻到。
//
// 英文用漢語拼音（不帶聲調），姓與名之間空一格，名連寫：
// `劉備` → `Liu Bei`、`諸葛亮` → `Zhuge Liang`、`傅士仁` → `Fu Shiren`。
//
// 日文用同樣的漢字，只把繁體換成新字體：`關羽` → `関羽`、`黃忠` → `黄忠`。
// 沒有新字體的字原樣保留。

// compoundSurnames 是複姓。**不列的話 `諸葛亮` 會變成 `Zhu Geliang`**
// ——拼音本身沒錯，斷詞錯了。
var compoundSurnames = []string{
	"諸葛", "司馬", "夏侯", "公孫", "太史", "淳于",
}

// shinjitai 是繁體字對應的日文新字體。沒有對應的字不列，原樣保留。
var shinjitai = map[rune]rune{
	'關': '関', '黃': '黄', '權': '権', '濟': '済', '樂': '楽',
	'顏': '顔', '滿': '満', '勳': '勲', '繡': '繍', '覽': '覧',
	'肅': '粛', '澤': '沢', '溫': '温', '榮': '栄', '應': '応',
	'齡': '齢', '鐵': '鉄', '德': '徳', '橫': '横', '懷': '懐',
	'吳': '呉', '嚴': '厳', '觀': '観', '禪': '禅', '雙': '双',
	'豐': '豊', '敘': '叙', '觸': '触', '會': '会', '圖': '図',
	'續': '続', '齊': '斉', '靈': '霊', '黨': '党', '當': '当',
}

// pinyin 是每個字的漢語拼音（不帶聲調）。
//
// **多音字取這個遊戲裡用到的那個讀法**，理由標在旁邊：
// `樂進` 的 `樂` 讀 Yue、`沮授` 的 `沮` 讀 Ju、`逢紀` 的 `逢` 讀 Pang、
// `劉禪` 的 `禪` 讀 Shan、`鍾繇` 的 `繇` 讀 Yao、`張郃` 的 `郃` 讀 He、
// `琅邪` 的 `邪` 讀 Ya、`劉辟` 的 `辟` 讀 Bi。
var pinyin = map[rune]string{
	// 十四個州名用到、人名與郡名沒用到的十三個字。并、兗兩個字是原版
	// 「弁州」「充州」的**本字**，譯文換回本字再轉（`placeFix`）。
	'交': "Jiao", '兗': "Yan", '冀': "Ji", '州': "Zhou", '幽': "You",
	'并': "Bing", '揚': "Yang", '涼': "Liang", '益': "Yi", '荊': "Jing",
	'豫': "Yu", '隸': "Li", '青': "Qing",
	'丁': "Ding", '上': "Shang", '下': "Xia", '丕': "Pi", '中': "Zhong",
	'之': "Zhi", '乾': "Qian", '于': "Yu", '京': "Jing", '亮': "Liang",
	'仁': "Ren", '任': "Ren", '伉': "Kang", '伊': "Yi", '休': "Xiu",
	'侯': "Hou", '信': "Xin", '修': "Xiu", '倉': "Cang", '傅': "Fu",
	'傕': "Jue", '備': "Bei", '儀': "Yi", '儒': "Ru", '優': "You",
	'允': "Yun", '兆': "Zhao", '先': "Xian", '全': "Quan", '公': "Gong",
	'典': "Dian", '冷': "Leng", '凌': "Ling", '凱': "Kai", '刑': "Xing",
	'剛': "Gang", '劉': "Liu", '勳': "Xun", '化': "Hua", '北': "Bei",
	'匡': "Kuang", '卓': "Zhuo", '南': "Nan", '原': "Yuan", '叡': "Rui",
	'史': "Shi", '司': "Si", '同': "Tong", '向': "Xiang", '吳': "Wu",
	'呂': "Lu", '周': "Zhou", '和': "He", '嘉': "Jia", '嚴': "Yan",
	'圃': "Pu", '圖': "Tu", '堅': "Jian", '堪': "Kan", '士': "Shi",
	'夏': "Xia", '天': "Tian", '太': "Tai", '夷': "Yi", '奉': "Feng",
	'姜': "Jiang", '威': "Wei", '孔': "Kong", '孟': "Meng", '孫': "Sun",
	'安': "An", '宋': "Song", '宓': "Mi", '定': "Ding", '宜': "Yi",
	'宮': "Gong", '寧': "Ning", '審': "Shen", '寵': "Chong", '封': "Feng",
	'尚': "Shang", '就': "Jiu", '岑': "Cen", '岱': "Dai", '峻': "Jun",
	'崔': "Cui", '嶷': "Yi", '川': "Chuan", '巴': "Ba", '巽': "Xun",
	'布': "Bu", '師': "Shi", '平': "Ping", '幹': "Gan", '度': "Du",
	'庶': "Shu", '廉': "Lian", '廖': "Liao", '廬': "Lu", '延': "Yan",
	'建': "Jian", '式': "Shi", '弘': "Hong", '張': "Zhang", '彤': "Tong",
	'彧': "Yu", '彪': "Biao", '彭': "Peng", '彰': "Zhang", '徐': "Xu",
	'循': "Xun", '德': "De", '志': "Zhi", '忠': "Zhong", '性': "Xing",
	'恢': "Hui", '恪': "Ke", '惇': "Dun", '慈': "Ci", '憲': "Xian",
	'應': "Ying", '懷': "Huai", '懿': "Yi", '成': "Cheng", '授': "Shou",
	'操': "Cao", '攸': "You", '敘': "Xu", '文': "Wen", '旋': "Xuan",
	'旻': "Min", '昂': "Ang", '昌': "Chang", '昭': "Zhao", '昱': "Yu",
	'晃': "Huang", '普': "Pu", '曄': "Ye", '曠': "Kuang", '曹': "Cao",
	'會': "Hui", '朗': "Lang", '朱': "Zhu", '李': "Li", '東': "Dong",
	'松': "Song", '林': "Lin", '柏': "Bo", '柯': "Ke", '柴': "Chai",
	'桂': "Gui", '桑': "Sang", '桓': "Huan", '梁': "Liang", '植': "Zhi",
	'楊': "Yang", '楙': "Mao", '業': "Ye", '楷': "Kai", '榮': "Rong",
	'樂': "Yue", '樊': "Fan", '橋': "Qiao", '橫': "Heng", '權': "Quan",
	'欽': "Qin", '歆': "Xin", '正': "Zheng", '步': "Bu", '武': "Wu",
	'毓': "Yu", '毗': "Pi", '毛': "Mao", '水': "Shui", '永': "Yong",
	'氾': "Fan", '汜': "Si", '沙': "Sha", '沛': "Pei", '沮': "Ju",
	'治': "Zhi", '泉': "Quan", '法': "Fa", '泰': "Tai", '洛': "Luo",
	'洪': "Hong", '浩': "Hao", '海': "Hai", '涿': "Zhuo", '淮': "Huai",
	'淳': "Chun", '淵': "Yuan", '渤': "Bo", '溫': "Wen", '滿': "Man",
	'漢': "Han", '潁': "Ying", '潘': "Pan", '澤': "Ze", '濟': "Ji",
	'焉': "Yan", '焦': "Jiao", '然': "Ran", '煥': "Huan", '熊': "Xiong",
	'熙': "Xi", '燕': "Yan", '爽': "Shuang", '牂': "Zang", '牛': "Niu",
	'獲': "Huo", '玄': "Xuan", '王': "Wang", '玠': "Jie", '玩': "Wan",
	'珪': "Gui", '班': "Ban", '琅': "Lang", '琦': "Qi", '琬': "Wan",
	'琮': "Cong", '琰': "Yan", '琳': "Lin", '瑁': "Mao", '瑜': "Yu",
	'瑾': "Jin", '璋': "Zhang", '璜': "Huang", '瓊': "Qiong", '瓚': "Zan",
	'甘': "Gan", '生': "Sheng", '田': "Tian", '留': "Liu", '當': "Dang",
	'疆': "Jiang", '登': "Deng", '皎': "Jiao", '盛': "Sheng", '真': "Zhen",
	'矯': "Jiao", '磐': "Pan", '祖': "Zu", '禁': "Jin", '禪': "Shan",
	'秉': "Bing", '秋': "Qiu", '秦': "Qin", '程': "Cheng", '稠': "Chou",
	'竺': "Zhu", '策': "Ce", '範': "Fan", '簡': "Jian", '籍': "Ji",
	'粲': "Can", '紀': "Ji", '純': "Chun", '紘': "Hong", '索': "Suo",
	'累': "Lei", '紹': "Shao", '統': "Tong", '綜': "Zong", '維': "Wei",
	'綱': "Gang", '績': "Ji", '繇': "Yao", '繡': "Xiu", '續': "Xu",
	'羕': "Yang", '群': "Qun", '義': "Yi", '羽': "Yu", '翊': "Yi",
	'翔': "Xiang", '翻': "Fan", '翼': "Yi", '聘': "Pin", '肅': "Su",
	'胡': "Hu", '胤': "Yin", '臧': "Zang", '興': "Xing", '良': "Liang",
	'艾': "Ai", '芝': "Zhi", '芳': "Fang", '苞': "Bao", '茂': "Mao",
	'范': "Fan", '荀': "Xun", '華': "Hua", '萌': "Meng", '著': "Zhu",
	'葛': "Ge", '董': "Dong", '蒙': "Meng", '蒯': "Kuai", '蓋': "Gai",
	'蔡': "Cai", '蔣': "Jiang", '蕤': "Rui", '薄': "Bo", '薛': "Xue",
	'蘭': "Lan", '虎': "Hu", '虔': "Qian", '虞': "Yu", '融': "Rong",
	'術': "Shu", '衛': "Wei", '表': "Biao", '袁': "Yuan", '裔': "Yi",
	'褒': "Bao", '褘': "Hui", '褚': "Chu", '襄': "Xiang", '襲': "Xi",
	'覽': "Lan", '觀': "Guan", '觸': "Chu", '許': "Xu", '評': "Ping",
	'詡': "Xu", '詩': "Shi", '誕': "Dan", '諶': "Chen", '諸': "Zhu",
	'謖': "Su", '謙': "Qian", '譙': "Qiao", '譚': "Tan", '豐': "Feng",
	'豹': "Bao", '貴': "Gui", '費': "Fei", '賈': "Jia", '賢': "Xian",
	'超': "Chao", '越': "Yue", '趙': "Zhao", '軫': "Zhen", '輔': "Fu",
	'辛': "Xin", '辟': "Bi", '農': "Nong", '通': "Tong", '逢': "Pang",
	'進': "Jin", '逵': "Kui", '遂': "Sui", '道': "Dao", '達': "Da",
	'遜': "Xun", '遵': "Zun", '選': "Xuan", '遼': "Liao", '邈': "Miao",
	'邪': "Ya", '邳': "Pi", '郃': "He", '郝': "Hao", '郡': "Jun",
	'郭': "Guo", '都': "Du", '鄂': "E", '鄧': "Deng", '鄴': "Ye",
	'配': "Pei", '酒': "Jiu", '醜': "Chou", '金': "Jin", '銀': "Yin",
	'鍾': "Zhong", '鐵': "Tie", '長': "Chang", '閻': "Yan", '闓': "Kai",
	'關': "Guan", '闞': "Kan", '阜': "Fu", '陳': "Chen", '陵': "Ling",
	'陶': "Tao", '陸': "Lu", '陽': "Yang", '雄': "Xiong", '雍': "Yong",
	'雙': "Shuang", '雲': "Yun", '零': "Ling", '雷': "Lei", '霍': "Huo",
	'霸': "Ba", '靈': "Ling", '靖': "Jing", '鞏': "Gong", '韋': "Wei",
	'韓': "Han", '順': "Shun", '顏': "Yan", '顗': "Yi", '顧': "Gu",
	'飛': "Fei", '馬': "Ma", '騭': "Zhi", '騰': "Teng", '高': "Gao",
	'髦': "Mao", '鬱': "Yu", '魏': "Wei", '魯': "Lu", '魴': "Fang",
	'鮑': "Bao", '鴛': "Yuan", '麋': "Mi", '麴': "Qu", '黃': "Huang",
	'黨': "Dang", '齊': "Qi", '齡': "Ling", '龍': "Long", '龐': "Pang",
	'龔': "Gong",
}

// PersonName 把一個人名換成目前語系的寫法。
//
// 繁中原樣回傳；**有任何一個字沒有對照就整個原樣回傳**——半翻的名字
// （`Liu 備`）比不翻更糟，看起來像資料壞掉。
func PersonName(zh string) string {
	switch Current {
	case Ja:
		return toShinjitai(zh)
	case En:
		return romanisePerson(zh)
	}
	return zh
}

// placeFix 是原版地名的錯字在**譯文**裡換回的本字。
//
// 原版寫「弁州」「充州」，是并州、兗州寫錯的字（`docs/spec/003` §州名）。
// **中文照原版資料不動**——那是原文；英日文是譯文，照原意寫成
// Bingzhou／Yanzhou、并州／兗州（使用者裁定 2026-09-11）。
// 這兩個字在這個遊戲的資料裡只出現在這兩個州名，逐字換不會誤傷人名。
var placeFix = map[rune]rune{'弁': '并', '充': '兗'}

// PlaceName 把一個地名換成目前語系的寫法。地名不分姓名，整串連寫。
func PlaceName(zh string) string {
	if Current != ZhHant {
		zh = strings.Map(func(r rune) rune {
			if f, ok := placeFix[r]; ok {
				return f
			}
			return r
		}, zh)
	}
	switch Current {
	case Ja:
		return toShinjitai(zh)
	case En:
		s, ok := romanise(zh)
		if !ok {
			return zh
		}
		return s
	}
	return zh
}

// toShinjitai 逐字換成新字體；沒有對應的原樣保留。
func toShinjitai(zh string) string {
	var b strings.Builder
	for _, r := range zh {
		if n, ok := shinjitai[r]; ok {
			b.WriteRune(n)
			continue
		}
		b.WriteRune(r)
	}
	return b.String()
}

// romanisePerson 把人名拆成姓與名。
func romanisePerson(zh string) string {
	surname := ""
	for _, c := range compoundSurnames {
		if strings.HasPrefix(zh, c) {
			surname = c
			break
		}
	}
	if surname == "" {
		for _, r := range zh {
			surname = string(r)
			break
		}
	}
	given := zh[len(surname):]
	sr, ok := romanise(surname)
	if !ok {
		return zh
	}
	if given == "" {
		return sr
	}
	gr, ok := romanise(given)
	if !ok {
		return zh
	}
	return sr + " " + gr
}

// romanise 把一串漢字連寫成一個拼音詞，首字母大寫。
func romanise(zh string) (string, bool) {
	var b strings.Builder
	for _, r := range zh {
		s, ok := pinyin[r]
		if !ok {
			return "", false
		}
		if b.Len() == 0 {
			b.WriteString(s)
			continue
		}
		b.WriteString(strings.ToLower(s))
	}
	out := b.String()
	if out == "" {
		return "", false
	}
	r := []rune(out)
	r[0] = unicode.ToUpper(r[0])
	return string(r), true
}

// HasName 說一串漢字翻不翻得出來。給涵蓋率測試用。
func HasName(zh string) bool {
	for _, r := range zh {
		if _, ok := pinyin[r]; !ok {
			return false
		}
	}
	return zh != ""
}
