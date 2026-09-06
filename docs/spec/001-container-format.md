# 001：`.GRP` ／ `.IDX` ／ `.NAM` 容器讀取

狀態：**READY**
日期：2026-09-06
證據：`docs/formats/01-grp-idx-nam.md`（round-trip 已驗）

## 1. 範圍

只管**讀**。寫回原版格式不在範圍內——這是保存專案，不改原版檔案。

## 2. 資料模型

一組容器由三個同名不同副檔名的檔案組成：

```
DATA<n>.NAM   項目名表
DATA<n>.IDX   結束位移表
DATA<n>.GRP   資料本體
```

| 檔 | 版面 |
|---|---|
| `.NAM` | 每項 **16 byte**：8.3 檔名（名 8 格空白補齊 ＋ `.` ＋ 副檔名 3 格）＝ 12 byte，其後 4 個 `00` |
| `.IDX` | 每項 **4 byte 小端序 u32**，值 ＝ 該項的**結束位移** |
| `.GRP` | 各項首尾相接 |

第 `i` 項的位元組範圍是 `[end(i-1), end(i))`，其中 `end(-1) = 0`。

## 3. `[HARD]` 讀取時必須做的檢查

這四項是**驗證條件不是建議**。任何一項不過就回錯誤，不要「盡力而為」地讀下去——
解錯一個位元組不會報錯，只會讓多數項目碰巧是對的（`CLAUDE.md` §7 第 18 條）。

1. **`.GRP` 開頭不得是 `MZ`。** `DATA0`／`DATA4`／`DATA5` 的 `.GRP` 是執行檔，
   不是容器本體（`docs/formats/01` §4）。看到 `MZ` 要回一個講清楚的錯誤，
   不是硬切下去。
2. **項數兩邊都算並比對**：`len(.NAM)/16` 必須等於 `len(.IDX)/4`。
   `DATA0` 就是靠這一步露餡的（43 vs 604）。
3. **`.IDX` 必須非遞減**。出現逆序代表它不是位移表。
4. **`.IDX` 末值必須等於 `len(.GRP)`**。這是最強的一項：
   三個獨立檔案同時滿足，不會是巧合。

## 4. 名稱正規化

`.NAM` 的 12 byte 是**空白補齊**的 8.3 版面（例：`ZHONG   .COD`）。
對外的名稱要去掉補齊空白再拼：`ZHONG.COD`。

- 名稱段取前 8 byte，去尾空白
- 副檔名取第 10–12 byte，去尾空白
- 副檔名為空就不加 `.`
- **保留原始 12 byte** 供比對；正規化後的名稱只是方便用的

名稱是 ASCII（實測全部落在 `0x20`–`0x7E`）。**不要對名稱做 Big5 解碼**——
Big5 是資料內容的編碼，不是檔名的。

## 5. 介面

```go
package assets

type Entry struct {
    Name    string // 正規化後，例 "ZHONG.COD"
    RawName [12]byte
    Start   uint32 // 含
    End     uint32 // 不含
}

type Container struct { … }

func OpenContainer(nam, idx, grp []byte) (*Container, error)
func (c *Container) Len() int
func (c *Container) Entry(i int) Entry
func (c *Container) Data(i int) []byte
func (c *Container) ByName(name string) (int, bool)
```

`Data` 回傳的是 `.GRP` 的**子切片，不複製**。呼叫端不得寫入。

`ByName` 用正規化後的名稱查，**大小寫敏感**——原版全大寫，
容忍大小寫會把「名字打錯」變成「安靜地拿到別的東西」。

## 6. 不在這一版的

- `.GRP` 內每一項的**內容**怎麼解（`.PAT` 字模、`.IMG` 圖、`.FAC` 臉譜、`.PAL` 調色盤）。
  那是各自的 spec。
- `DATA0`／`DATA4`／`DATA5` 的解包（LZEXE 假說還沒驗，`docs/formats/01` §4）。
- 寫回。
