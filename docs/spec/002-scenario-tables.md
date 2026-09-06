# 002：劇本／存檔資料表讀取

狀態：**READY**（範圍極窄，見 §2）
日期：2026-09-06
證據：`docs/formats/03-scenario-tables.md`

## 1. 範圍

從 `DATA2` 容器讀出六個劇本與六個存檔槽的三張表，**只解已驗過的欄位**。

## 2. `[HARD]` 這一版只解姓名與郡名，其餘一律當不透明位元組

已驗的只有記錄大小與名稱欄（`docs/formats/03` §3、§4）。
**其餘 bytes 一律以 `Raw` 原樣交出，不給欄位名、不給型別。**

手冊列了 15 項郡屬性與 15 項將領屬性，那是**該去對的清單，不是版面**。
照它去猜欄位順序，猜出來的結構會自洽、會通過型別檢查、而且不會報錯——
錯誤要到「某個郡的糧草數字很怪」才浮現，那時已經有一堆程式建在上面。

欄位要一個一個進來，每一個都要有 `docs/formats/` 的證據與 dosgolem 對拍。

## 3. 定位方式

**用容器項目名，不用絕對位移。** 位移會隨版本漂移（`DATA2.GRP` 兩版等長
但內容不同），項目名不會。

```
BASEMAS.<slot>   16 × 72
BASESTA.<slot>   43 × 176   ← 筆 0 是啞元
BASEGEN.<slot>   350 × 30
```

`<slot>` ＝ `001`–`006`（劇本）或 `SV1`–`SV6`（存檔）。

## 4. 索引

**州郡索引是 1-based**：筆 0 是啞元（名稱欄 `....`、帶 `FFFF` 哨兵），
郡編號 1–42 直接對應筆號 1–42，與手冊一致。

`PrefectureByID(id)` 只接受 1–42；`0` 與越界回錯誤。
**不要把啞元當成第 43 個郡交出去。**

## 5. 名稱解碼

| 表 | 欄位 | 版面 |
|---|---|---|
| 州郡 | 郡名 | offset 0，4 byte Big5 ＋ 1 byte `00` |
| 人物 | 姓名 | offset 0，**6 byte，空白補齊**（2–3 個漢字）|

一律用 **`cp950`** 解，不用 `big5`。
人物姓名要 `TrimSpace` 之後再解——欄位是空白補齊的，兩字名前後各一個空白。

解不出來的**不要當成錯誤**：350 筆裡有 4 筆是空槽（筆號 346–349）。
空槽的姓名回空字串，並標 `Empty`。**不要 skip 掉**——
索引必須與原版的槽號一致，中間少一筆會讓後面全部錯位。

## 6. 哨兵值

`FFFF` 是原版的「沒有」，**不是** Go 的零值（`CLAUDE.md` §7 第 11 條）。

這一版不解任何數值欄位，所以還不需要正規化。但**開始解欄位時，
正規化要放在唯一入口**，不要讓 `0xFFFF` 當成 65535 流進規則層。

## 7. 介面

```go
package state

type Slot string // "001".."006"、"SV1".."SV6"

type Prefecture struct {
    ID   int      // 1..42
    Name string   // "遼東"
    Raw  [176]byte
}

type General struct {
    Index int      // 0..349，原版槽號
    Name  string   // "劉備"；空槽是 ""
    Empty bool
    Raw   [30]byte
}

type Master struct {
    Index int      // 0..15
    Raw   [72]byte
}

type Scenario struct { … }

func LoadScenario(c *assets.Container, slot Slot) (*Scenario, error)
func (s *Scenario) Prefecture(id int) (Prefecture, error) // 1-based
func (s *Scenario) Generals() []General                   // 350 筆，含空槽
func (s *Scenario) Masters() []Master                     // 16 筆
```

## 8. 不在這一版的

- 任何數值欄位（郡屬性、將領能力值、諸侯資料）。
- `BASEPRO`／`BASEPRE`（只在存檔槽，用途未解）。
- 寫回存檔。
- 加強版的差異——`DATA2.GRP` 兩版內容不同，這一版兩版都用同一個讀法，
  **但每次讀出來的東西要標版本**（`CLAUDE.md` §3.4）。
