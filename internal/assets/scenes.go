package assets

// 場景圖 `SCG01`–`SCG31.IMG`（176×96，`docs/formats/07` §3）與它們的用途。
//
// 原版在四十八處先把一張載進來、再叫特效常式 `0x32dfa(x, y)` 把它拉進
// 畫面（`docs/spec/010` §1 的表，`L0`、`[base]`）。名字照原版檔名的編號，
// 這裡的常數名是從呼叫端讀出來的用途，不是原版有的名字。
const (
	SceneQuake       = 1  // `0x1633b` 地震
	SceneFlood       = 2  // `0x165f6` 洪水
	SceneFloodTactic = 3  // `0x2b219` 水淹
	SceneJoin        = 4  // `0x1c1ba` 登用來歸、`0x1db5d` 挖角來歸、`0x25cae` 被擒願降
	SceneAppoint     = 5  // `0x15224`／`0x1729c` 新君主即位、`0x1c9e5` 指定軍師、`0x1cc77` 指定太守
	SceneWar         = 6  // `0x18b82` 發動戰役
	SceneRecruit     = 7  // `0x1c00e` 登用
	SceneJail        = 8  // `0x261a1` 囚禁
	SceneArms        = 9  // `0x199c8` 徵兵、`0x19d66` 武裝
	SceneLocust      = 10 // `0x16c5b` 蝗害
	SceneRetreat     = 12 // `0x24395` 撤軍
	ScenePlotSow     = 13 // `0x1da72` 挖角、`0x2d338` 離間君臣
	ScenePlague      = 14 // `0x16871` 瘟疫
	SceneTrain       = 15 // `0x197f6` 訓練、`0x1a075` 調整兵力
	SceneDeath       = 16 // `0x159b2` 玩家全部絕嗣、`0x15e8e` 元月老死
	SceneArrows      = 17 // `0x2a8c8` 弓箭
	ScenePlotFarNear = 18 // `0x2c8de` 遠交近攻
	SceneFire        = 19 // `0x2aeab` 火攻、`0x2b951` 燒糧
	SceneMove        = 20 // `0x18f99` 調動軍隊、`0x19273` 運送錢糧
	SceneFort        = 21 // `0x1ab84` 築關
	ScenePlotRevolt  = 22 // `0x2d879` 策反人民
	ScenePlotTiger   = 23 // `0x2cd6e` 驅虎吞狼
	SceneReward      = 24 // `0x1c501` 賞賜、`0x1d264` 賜物
	SceneDebut       = 25 // `0x16152` 出頭
	SceneDismiss     = 26 // `0x1c6ac` 撤職、`0x26383` 釋放
	SceneDuelDeath   = 27 // `0x31c96`／`0x31fba` 單挑戰死
	SceneDuelCapture = 28 // `0x31b51`／`0x31e66` 單挑被擒
	SceneDuel        = 29 // `0x30b40` 叫陣、`0x3190f` 平手
	SceneRice        = 30 // `0x1b2c7` 買米、`0x1b5b2` 賣米
	SceneLand        = 31 // `0x1a721` 開墾、`0x1a932` 治水
)

// 場景圖的尺寸（`docs/formats/07` §3）與落點：主畫面右側面板、計略那
// 兩則、主戰場第三塊面板。
const (
	SceneW, SceneH = 176, 96

	SceneMainX, SceneMainY     = 432, 80
	ScenePlotX, ScenePlotY     = 432, 120
	SceneBattleX, SceneBattleY = 448, 268
)
