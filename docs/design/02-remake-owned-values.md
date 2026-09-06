# remake 自己定的數值

`internal/game/tuning.go`、`events.go`、`battle.go`、`plot.go` 與
`internal/battle/tuning.go` 裡所有 `Tune` 開頭的常數。
**這些不是原版的數字。**

說明書給了方向（「謀略越高，土地價值增加越多」）卻沒給係數；
原版的公式還沒反組譯到。集中列出來，是為了讓「哪些是還原的、
哪些是我們選的」一眼看得出來。

## 1. 為什麼要集中

猜出來的公式會自洽、會通過測試、玩起來「差不多」。散在各處的話，
反組譯出真正的公式之後就得逐一去找——而且會漏。

命名一律 `Tune` 開頭，用到的地方也就標示出來了：
`grep -rn Tune internal/game internal/battle` 得到的就是全部的清單。

## 2. 有出處的數字（**不在**這一份裡）

以下有手冊或資料背書，不是 remake 選的：

| 數字 | 出處 |
|---|---|
| 金／米上限 30000 | 說明書 p.22 |
| 徵兵人口下限 3000 | 說明書 p.20、p.37 |
| 城寨上限每郡 5 座 | 說明書 p.21 |
| 建寨費用 ＝ 物價 × 100 | 說明書 p.21 |
| 軍師謀略下限 80 | 說明書 p.23 |
| 各項花費（徵兵 1／武器 100 單位 1／開墾 10／防洪 10／尋訪 5／登用 30／撤職 10／挖角 100／賞賜上限 100）| 說明書 p.20–24 |
| 帶兵上限（君主 5000／軍師·大將 3000／參軍·副將 2500／主簿·裨將 2000／謀士·牙將 1500）| 說明書 p.18 **＋ 原版資料**（346 人零超標，九個職位裡八個頂到上限）|
| 寶物效果（兵書 +2 謀略／寶刀 +3 戰力／美女 +5 魅力／駿馬 +2 戰力 +3 魅力）、賞賜上限 90 | 說明書 p.24 |
| 戰場計謀的費用與智力門檻（火攻 600/80、水淹 500/75、誘敵 400/60、燒糧 300/70、圍攻 200/65、陷阱 100/60）| 說明書 p.32–34 |
| 弓箭可射次數 ＝ 各單武裝度**算術**平均 ÷ 20 取整 | 說明書 p.32（含算例：(75+80+50+100)÷4＝76.25 → 三次）|
| 戰役上限三十天、守方滿卅天且城池未失即衛郡成功 | 說明書 p.35 |
| 休息一次增加移動力 2 | 說明書 p.29、p.30 |
| 中陷阱九日內無法活動 | 說明書 p.33 |
| 每個戰鬥組最多十名將領 | 說明書 p.27 |
| 編隊、紮營、作戰的四張順序表 | 說明書 p.27–28 |
| 九種地形攻防效應的**方向**（誰高誰低）、火攻與水淹的地形殺傷**排序** | 說明書 p.31–33 |
| 洪水後洪水率立刻升到 100 | 說明書 p.21、p.36 |
| 秋收後土地價值略降 | 說明書 p.21、p.37 |

## 3. remake 選的數字

### 內政與經濟

| 常數 | 值 | 手冊怎麼說 |
|---|---|---|
| `TuneReclaimBase` / `TuneReclaimIntel` | 1／25 | 「負責開墾的將領謀略越高，土地價值增加越多」|
| `TuneFloodBase` / `TuneFloodIntel` | 1／25 | 「負責治水的將領謀略越高，洪水發生機率下降越多」|
| `TuneTrainBase` / `TuneTrainIntel` | 2／25 | 「各將的能力影響其麾下的訓練度提升」|
| `TuneReliefRice` / `TuneReliefLoyalty` | 500／3 | 「太守魅力越高，效果越好」|
| `TuneTransportLoss` | 20 | 「太守魅力值越高，途中損耗越少」|
| 米價換算（一單位米 ＝ 物價 ÷ 100 金）| — | 「依物價購米入倉」，沒給比率 |

### 人事

| 常數 | 值 | 手冊怎麼說 |
|---|---|---|
| `TuneSearchIntel` | 1 | 「謀略越高，成功的機率越大」|
| `TuneRecruitCharm` | 1 | 「太守的魅力越高，成功的機會越大」|
| `TuneRewardLoyalty` | 10 | 只給了賞金上限 100 |
| `TuneHeadhuntBase` | 60 | 只給了費用 100 金 |
| `TuneNewSoldierTraining` / `TuneNewSoldierArms` | 0／0 | 「新兵毫無訓練，加入時會把部隊的訓練度拉低」|

### 季節事件

| 常數 | 值 | 手冊怎麼說 |
|---|---|---|
| `TuneDisasterBase` | 8（每月）| 「天災多因人怨引起，民眾忠誠最好不要太低」|
| `TuneFloodWeight` | 50 | 洪水率的作用 |
| `TuneQuakeLoss` | 10 | 「地震造成人口減少，財物和米糧的損失，及部隊兵力傷亡」|
| `TuneFloodPopLoss` / `TuneFloodLandLoss` | 8／5 | 「人口和兵力都會減少，土地價值也會流失」|
| `TunePlagueLoss` / `TunePlagueStamina` | 12／5 | 「人口和兵力銳減，將領的體能也不正常地下降」|
| `TuneHarvestRicePerLand` / `TuneHarvestGoldPerLand` / `TuneHarvestLandDrop` | 2／1／2 | 「稅金入庫、米糧進倉」|
| `TuneLocustRiceLoss` / `TuneLocustLandLoss` | 30／5 | 「米糧減少，土地價值也會下降」|
| `TuneWinterGrowth` | 95（千分比，**一年一次**）| 「人口增加」|
| `harvestMonth` | 9 | 說明書只說秋收在秋天，沒說哪個月 |
| `tributeMonth` | 12 | 說明書只說每年進貢一次 |
| `TuneAgingStamina` | 1 | 「年齡增長，體能隨之逐漸減退」|

`agingMonth`（元月）與 `growthMonth`（十月）**不在這張表裡**——
那兩個是從原版量出來的，不是 remake 挑的（`docs/mechanics/50-events` §1）。
| `TuneTributePerPrefecture` | 3 | 「領地越多，貢品越多」|

### 戰役

#### 戰略層（`internal/game/battle.go`）

這一組只用來給 AI **估算**要不要出兵，不決定任何一場戰役的結果。

| 常數 | 值 | 手冊怎麼說 |
|---|---|---|
| `TuneTrainingWeight` / `TuneArmsWeight` / `TuneWarWeight` | 60／40／50 | 「影響戰力的因素：訓練度、武裝度、兵數、地形、兵種及有無用計」|
| `TuneDefenceBonus` / `TuneFortBonus` | 30／5 | 「城池能夠發揮部隊最大戰力，以及一流防禦工事」「關寨提供少許攻擊優勢，及簡陋的防禦工事」|

#### 戰術層（`internal/battle/tuning.go`）

| 常數 | 值 | 手冊怎麼說 |
|---|---|---|
| `TuneMoveBase` / `TuneMoveTraining` / `TuneMoveArmsPenalty` / `TuneMoveMin` | 4／25／50／2 | 「移動力來源是訓練度和兵種能否適應地形」「全副武裝將稍減移動力」，沒給公式 |
| `TuneHitTraining` / `TuneHitArms` / `TuneHitWar` / `TuneHitBase` | 60／40／50／12 | 列了影響戰力的因素，沒給公式 |
| `TuneArrowDamage` | 6 | 只給了**次數**公式，沒給單次殺傷 |
| `TuneFireBase` / `TuneFloodBase` | 30／30 | 只給了地形之間的**排序**，沒給幅度 |
| `TuneBurnLoss` | 40 | 「燒毀敵軍的糧食」，沒給比例 |
| `TuneSiegeBonus` | 25 | 「聯合友軍圍攻」，沒給加成 |
| `TuneEnragedPenalty` | 30 | 「來犯敵軍攻擊力暫時下降」，沒給幅度 |
| `TuneDuelDamage` | 12 | 「體力降到 0 即告落敗」，沒給每回合消耗 |
| `TuneRefuseDuelLoss` | 10 | 「麾下士兵將有部份逃跑」，沒給比例 |
| `TuneCaptureOnDuel` | 60 | 「可能被擒，或死於刀下」，沒給機率 |
| `TuneDeathBattleEdge` / `TuneDuelWarEdge` / `TuneRetreatShare` / `TuneStratagemRange` | 140／20／30／3 | 自動作戰什麼時候該死戰、叫陣、退兵、用計——手冊是寫給玩家看的，沒有這一層 |

地形的攻防修正幅度（`attackMod`／`defenceMod`）、地形的移動花費
（`moveCost`）、戰場尺寸（21×15）同樣是 remake 選的；
手冊只給了它們之間的相對關係。

### 謀略

| 常數 | 值 | 手冊怎麼說 |
|---|---|---|
| `TuneChiefWeight` / `TuneEnvoyWeight` / `TunePrestigeWeight` / `TuneEnemyChiefBonus` | 40／25／15／20 | 「成功率取決於四項：我方軍師智力、派遣使者魅力、我方君主人望、對方軍師智力」|
| `PlotCost` | 100–300 | 平時的五種計謀手冊**沒給費用**；照戰場那一組的量級 |
| `TuneForgeryLoyalty` / `TuneInciteLoss` | 15／20 | 「降低其部將忠誠」「減少米、金和人民忠誠」|

### 災害與成長是一對

`TuneDisasterBase` 與 `TuneWinterGrowth` 不能各自調——災害吃人口、
冬季補回來，兩者的比例決定世界會不會慢慢死掉。

判準寫成測試（`session.TestEconomyStaysSane`）：**二十年後的總人口要落在
開局的一半到兩倍之間**。這是 remake 自己的設計意圖，手冊沒給成長率。

> 量到過：第一版（災害 30／月、成長 20‰）跑二十年之後總人口從
> 2,538,000 掉到 173,991——剩百分之七。而每一條規則單獨看都「照手冊做」，
> 只有整局跑過才看得出來。同一輪也發現 AI 徵兵是 1:1 減人口卻不節制，
> 一次抽到下限，下一次還會再抽。

## 4. 亂數

`State.roll` 是**決定性**的：以年、月與呼叫端給的鹽做雜湊。
同一個局面同一個決定永遠得到同一個結果。

⚠ **這不是原版的亂數產生器**（還沒反組譯到）。換掉它的時候，
上層的介面不用動——所有需要機率的地方都只呼叫 `roll`。

決定性是刻意的：帶系統亂數的話「這一次為什麼失敗」無法回答，
對拍也無從下手。
