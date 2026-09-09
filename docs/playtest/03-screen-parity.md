# 03：畫面的對拍

## 主選單：兩個實作逐點相同（2026-09-09）

`workplace/shots/open/` 那幾張基準畫面是 dosgolem 畫的，而主選單五張圖塊
的位置是從那幾張比對出來的。**同一個實作既當被驗的對象又當標準答案，
驗不出東西**（`CLAUDE.md` §4）——所以另外拿 DOSBox-X 跑一次原版。

| | 產生方式 |
|---|---|
| dosgolem | `internal/parity` 的 `TestZZOriginalOpeningScreens`（`-tags oracle`），`workplace/shots/open/open-06.png` |
| DOSBox-X | `tools/dosboxx.sh`，`workplace/shots/dosboxx/menu.png` |

DOSBox-X 2026.06.02、`machine=svga_s3`、`core=normal`、`cputype=386`、
**`cycles=fixed 60000`**（`cycles=auto` 是可重現性的敵人，兩份 bundle 附的
`dosbox.conf` 都是 auto，不要直接拿來用）。裝置三題送 `1`／`2`／`2`，
之後一路按 Enter 續行開場。

結果：**640×350 裡 223,916 點逐點相同**，剩下的 84 點全部落在
`x 592..599, y 329..343`——右下角小飾框 `MENU3` 裡的一格，
那裡原版放了一段動畫，同一台原版連拍兩張就會不同。

閘門是 `internal/assets` 的 `TestDosgolemMenuMatchesDosboxX`，
兩張圖都在時才跑（都在 gitignore 的 `workplace/` 底下）。

### 抓 DOSBox-X 畫面的三個坑

- **截圖快捷鍵預設是 host key ＋ F5，而 host key 在非 Windows 上是 F12**，
  不是 `Ctrl+F5`。按 `Ctrl+F5` 不會有任何錯誤訊息，`captures` 目錄就是空的。
  繞過去的方法是不用它：`import -window` 直接抓視窗。
- **容器裡沒有視窗管理員，焦點要自己設**（`xdotool windowfocus --sync`）。
  沒設的話按鍵送不進去，而開場動畫照樣在動——看起來像遊戲卡住而不像沒收到鍵。
- **視窗是 640×408，上下各一條黑邊，畫面本體在左上角**。
  `+0+29` 那種「上下對半」的裁法會整張錯開，比出來只有四成相符，
  看起來像兩個實作真的不一樣。

## 主選單的版面

位置全部拿原版的畫面比對出來（主選單跑在開機鏈的第二層 `DATA0.GRP`，
主程式的碼段 dump 涵蓋不到，`docs/re/02` §1）：

| 圖 | 位置 | 大小 | 相符 |
|---|---|---|---|
| `MENU0A.IMG` | (40, 27) | 280×180 | 100% |
| `MENU0B.IMG` | (320, 27) | 288×180 | 100% |
| `MENU1.IMG` | (56, 215) | 96×151 | 95.3%（字寫在上面）|
| `MENU2.IMG` | x ∈ {152, 376}、y ∈ {215, 267, 320} | 200×46 | 93–97%（字寫在上面）|
| `MENU3.IMG` | (576, 320) | 40×41 | 91%（裡面那格會動）|

**三列的間距是 52、53 不是兩個 52**：第三列在 `y=320`。直牌與第三列按鈕
的下緣都超出 350 被裁掉。

文字（`assets.MenuText*`）：

- 按鈕上一行字從按鈕左緣 ＋24 起排，先是編號的半形「數字 ＋ 句點」，
  **每個全形字前面空一個半形格**——所以中文的字距是 24 不是 16。
  照 16 排，五個字只佔 80 像素，擠在 200 寬的按鈕左半邊。
- 文字格上緣是按鈕上緣 ＋9，格高 16。
- 左側直牌上「主選擇單」四個字是**橫向拉成兩倍寬**畫的（16×16 的字模
  畫成 32×16），格左緣 x=80、第一格上緣 y=242、行距 24。

remake 這一側的閘門是 `internal/ui` 的 `TestMenuItemLayoutMatchesTheOriginal`
（每個字落在原版的哪一格）、`TestMenuLabelIsDoubleWidth`、
`TestTitleScreenMatchesTheOriginal`（扣掉字與動畫那一格之後 **208,050 點
逐點相同**）。

### remake 差異

| 差異 | 為什麼 |
|---|---|
| 字模不同 | remake 自建字庫，不內嵌任何原版字模（`CLAUDE.md` §3.3）|
| 選到的那一項畫白色 | 原版沒有選取記號（六項同一個黃，靠按數字鍵選）；remake 支援上下鍵移動，沒有記號看不出停在哪一項 |
| 小飾框裡不畫動畫 | 那段動畫是什麼還沒解出來 |
| 疊清單時直牌仍寫「主選擇單」 | 原版會換成那一層的名字（實測按下「載入舊進度」之後變成「載入進度」）；remake 的清單標題是多語系的，直排放不下英日 |
| 最上面兩個角落 | 原版 (0,0) 與 (639,0) 是黑的（兩個實作一致），remake 那兩點畫底色 |
