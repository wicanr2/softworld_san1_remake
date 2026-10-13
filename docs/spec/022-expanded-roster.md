# 022：參考 San5 擴充武將

狀態：**CONFORMED**，限本規格的擴充資料、規則、介面、存檔及素材組裝驗收。
使用者於 2026-10-12 確認具體配置、王朗例外、初始化、存檔格式 2 及缺圖回退方案。
六劇本資料及規則驗證、正常玩家路徑抽樣與 220 張肖像已完成，已納入 v.1.2.0 本機包。
後續 San5 原圖重繪先以獨立本機素材附包交付，現已納入 v.1.2.1-20261013 完整版，見下節。
原始證據見 [RE 14](../re/14-san5-roster.md)，現況入口為 [CONTEXT](../../CONTEXT.md)。

## 已確認範圍

下列為使用者在 2026-10-11、2026-10-12 對話的決定，適用本作原版與加強版 remake。
這是明示的 remake 擴充，不能宣稱為智冠原版名單或原版 parity。

- 參考 `/home/anr2/cht/san5`，只補本作缺少的人物。
- 所有新遊戲預設加入。舊存檔保留原有名單，不在讀檔時自動擴充。
- 左慈、于吉、華佗等特殊人物也作為一般武將，可登用及帶兵參戰。
- 新增人物體能統一 80。原有 346 人姓名與能力保持。
- San5 武力、智力、魅力分別對應戰力、謀略、魅力。政治保留在圖鑑。
- 依 San5 配置加入勢力。本作劇本不存在的來源勢力，使用者已於本輪決定連同勢力一起新增。
  2026-10-12 已選優先分配無主郡，保留既有領土及原有駐軍。具體分配、
  同一勢力的君主繼承及城郡對應已於本規格確認。
- 兩作年代不一致時，採用不晚於本作開局的最近一份 San5 資料。此項由使用者於本輪確認。
- 2026-10-12 已選保留所選 San5 名單，不另依史實生卒年刪除新增人物。
  新增人物年齡依本作開局年份換算。滿 15 歲才登場，未達年齡者保留為未登場。
- 2026-10-12 已選新增人物的老化門檻統一為 68 歲，體能仍為 80。
  沿用本作的年度老化判定及體能扣減，並非到 68 歲立即死亡或固定扣能力。
- 2026-10-12 已選加入本作既有繼承勢力，沿用本作現任君主。例如曹操來源勢力
  對應 220 年的曹丕，孫堅對應 195 年的孫策。確實沒有對應勢力時才另建。
  此項沒有授權把所有被兼併勢力自動歸給征服者；完整對照表見下方已確認配置。
- 原君主在本作已故且沒有繼承勢力時，新增部下改為在野，不復活原君主。
  使用者進一步確認「已故君主的部下在野；本作沒有的來源勢力仍新增」。
  因此張角、何進等本作沒有的人物及勢力仍在擴充範圍。
- 新增人物製作 HD 肖像。2026-10-13 使用者確認改以 San5 原圖重繪，依下節四張已選樣張製作；取代先前的原創外貌方向。

## San5 原圖 HD 重繪，2026-10-13

本節狀態：**CONFORMED**，限 220 張重繪、來源核對、總覽、兩版載入及正常人物卡抽樣。
使用者確認[張角、華佗及兩位馬忠四張樣張](../../workplace/asset-overviews/san5-hd-redraw-20261012-r1/san5-hd-comparison-r1.png)，
授權重繪全部 220 張新增人物並更新 HD 武將總覽。使用內建 image_gen，每張直接參考
該人物的 San5 原圖，保留臉型、年齡、表情、朝向、服飾與主要配色，完成高清手繪細節。
兩位馬忠以 san5:222、san5:452 分別識別。原有 346 人沿用現行肖像。

來源為已核對的 KAODATA.S5，SHA-256
`f942631ced0bc891f796aa6f884461f28aae0769312f0eef90b22b7d7c98e2d6`。
三平面解碼及外部調色盤參考沿用 [RE 14](../re/14-san5-roster.md#原版與-san5-武將原圖總覽)，
不宣稱原版執行期色盤或像素 parity。[全部來源、完整提示詞及採用版次](../../workplace/asset-overviews/san5-hd-redraw-20261012-r1/progress.json)只留本機；
先前四張樣張紀錄另保留於同目錄的 `provenance.json`。

逐批保存原圖、生成識別、參考與輸出雜湊，完整解碼並檢查對照縮圖。220 張已全部完成，
組裝本機素材後核對來源識別、兩版載入、原有肖像不變及 566 人總覽。
本次授權涵蓋重繪及本機總覽，不移動既有 tag／Release，也不覆寫 v.1.2.0 封包。

重繪逐張由 [匯入工具](../../tools/import-san5-redraw.py)保存進度；
全部完成後使用 [本機組裝工具](../../tools/package-san5-redraw.py)，
沿用 [總覽繪製工具](../../tools/render-general-catalog.py)的 566 人排列。
[正常人物卡驗證器](../../tools/verify-expanded-roster-inner.py)的 `--hd-assets` 可指定
重繪後完整素材包，並在收據記錄實際目錄與擴充肖像 manifest 雜湊。
[本機獨立核對腳本](../../workplace/asset-overviews/san5-hd-redraw-20261012-r1/verify-final.py)
另回讀生成原檔、San5 三平面來源、放大參考與總覽肖像格，與組裝工具分開執行。

| 驗證 | 結果與收據 |
|---|---|
| 生成與來源 | 220 張與 image_gen 原檔逐位元組相同；440 張原圖／放大參考及 566 格總覽通過。[獨立核對](../../workplace/asset-overviews/san5-hd-redraw-20261012-r1/final-verification.json) |
| 視覺檢查 | Codex 已查看全部 220 人的原圖／HD 對照，兩位馬忠分別保留；使用者確認的是四張畫風樣張。[逐批紀錄](../../workplace/asset-overviews/san5-hd-redraw-20261012-r1/visual-review.json) |
| 原有素材 | 原 manifest 904 筆全部雜湊保持，437 張舊 PNG 加 220 張重繪，共 657 張；原有 346 人的完整總覽格逐像素不變 |
| 正式載入 | 原版與加強版各 452 筆既有素材及 220 張重繪，零警告；440 張 HighImage 輸出完整解碼。[載入收據](../../workplace/asset-overviews/san5-hd-redraw-20261012-r1/loader-verification.json) |
| 正常玩家路徑 | 兩版 189 年難度 1 新局、張角人物卡、HD 切換、格式 2 存檔及重啟載入通過；47 項檢查、33 張唯一 PNG 回讀。[獨立收據](../../dist-all/v.1.2.0-20261012/smoke/san5-hd-redraw-20261013-r1/independent-verification.json) |
| 本機交付 | ZIP 1,119 個檔案的 CRC 與逐檔 SHA-256 回讀通過，含完整素材、原圖／HD 總覽、來源紀錄及驗證收據。[附包契約](../release/01-packaging.md#san5-重繪本機素材附包) |

新版 HD 總覽 SHA-256 為 `92af0d3ee0c56d53594bc35d7fe14b140a1e845900e2e42cbd773c782cecd7da`，
擴充素材 manifest 為 `d1e2c125fa503a2466cef03c466780a5b052405a44b7fc49e402101cfcca4701`。
生成工具未回報模型版本或 seed。重繪為依原圖製作的插畫，不宣稱史實容貌；正常 GUI
只抽測張角，不宣稱逐一操作 220 人或原生 Windows／macOS 驗收。

使用者確認本次 HD 後，已收入 [v.1.2.1 完整版](../release/01-packaging.md#v121-san5-重繪完整版)。
四平台直接收錄新重繪及 566 人總覽，README 圖片隨包提供；新版正式兩版人物卡與
存讀檔另驗通過。前一版素材附包與本節的生成、來源及視覺收據保留。

## 先前原創肖像樣張，已由 San5 重繪方向取代

[華佗原圖](../images/expanded-hua-tuo-approved-20261011.png)為上一輪生成且由使用者
在 2026-10-11 21:18 選定的樣張。它是新增肖像的樣式基準，已以 san5:492 納入正式肖像目錄。

| 項目 | 紀錄 |
|---|---|
| 生成工具 | 內建 `image_gen.imagegen`，未回報模型或 seed |
| 生成紀錄 | `exec-29d143fc-98b5-413e-8710-dc5c3cd4676e` |
| 來源 session | `01a1293e-6b29-77a2-a623-d87f3d91d39a` |
| 原圖尺寸 | 1122×1402，不透明 PNG |
| SHA-256 | `104090ece088f21a5285909cc477abbd6744868ded0266b0bd3dd3201cacebaf` |
| 參考 | 本作既有 `hd-assets/F005.png`、`F004.png` 的畫風與構圖 |
| 保存方式 | 從生成工具保存的原圖逐位元組複製，未重繪、裁切或縮放 |

灰白髮鬚、深色布帽、青綠袍、近景及粉紅筆觸背景是插畫設計，沒有宣稱為史實容貌。
不採用或複製 KOEI 原肖像。後續製作保留生成請求、參考雜湊及採用版次。

完整請求從前輪 session 的工具呼叫回讀，未從生成結果反推。兩張畫風參考檔的
目前雜湊為：

- `hd-assets/F005.png`：`a8ebf25dc85aad63884a3a9535f2a9ae36d29fe3fbe88cd0e2e560f24b7e436d`。
- `hd-assets/F004.png`：`1c93642fda3a35f791384c940aaeddee610cc62a1cf7bcfcde024e327f5f6884`。

```text
Use case: historical-scene.
Asset type: one new HD character portrait for the Traditional Chinese remake of Softworld's 1991 Romance of the Three Kingdoms, matching its EXISTING portrait art exactly.
Input images: F005.png and F004.png are style and framing references only. Do not reproduce their identities.
Primary request: create a NEW portrait of 華佗 (Hua Tuo), imagined as an elderly Han-era physician, to join this same portrait set. He has a slender thoughtful face, silver white eyebrows, a long flowing white beard and moustache, grey hair tied under a modest dark cloth scholar cap, and a simple muted teal robe with an ivory collar. Calm, perceptive expression, head turned slightly toward the viewer's left. These are illustrative design choices.
Style/medium: the same realistic hand-painted illustration as the references: textured opaque brushwork, crisp readable face and eyes, subtle impasto in hair and beard, warm ochre skin, restrained natural facial anatomy. Match the density, finish, lighting, close crop, and vivid pink/magenta mottled painted backdrop of the references. No photographic rendering or smooth 3D.
Composition/framing: vertical 4:5 portrait; one person, close head-and-shoulders crop, face fills the picture, top of the cap may approach the upper edge; shoulders extend beyond the bottom and sides. Same camera distance and painted background treatment as the two references. No frame or margins. Opaque background.
Text: none.
Constraints: a single finished portrait only; no contact sheet, no labels, no UI, no lettering, no watermarks, no weapons, no medical props, no other people. This is an original new portrait, not a reproduction of KOEI game artwork.
```

## 左慈肖像素材

[左慈第 2 版](../images/expanded-zuo-ci-v2-20261011.png)已以 san5:496 接入正式素材目錄。
使用者批准的是華佗畫風，左慈沿此方向製作並保留採用版次。

| 項目 | 紀錄 |
|---|---|
| 工具 | 內建 `image_gen.imagegen`，未回報模型或 seed |
| 第 1 版生成紀錄 | `exec-949b274c-5b1e-4299-8f8f-0a78725ef652` |
| 採用版生成紀錄 | `exec-5597a6c9-c507-4a3b-85c8-7539c1bf3db8` |
| 原圖 | 1122×1402，RGB 不透明 PNG，2,892,888 bytes |
| SHA-256 | `ec49ec3b412990e906e68cd65a6f6188e534c6632657415193a013c96c3ded71` |
| 畫風參考 | 本頁華佗樣張，SHA-256 `104090ece088f21a5285909cc477abbd6744868ded0266b0bd3dd3201cacebaf` |
| 保存 | 原圖直接複製；PNG 區塊 CRC、解壓、輸出雜湊與 UID/GID 1000:1000 均核對 |

面貌、髮髻、白袍與紫領為插畫設計，未宣稱為史實容貌。第 1 版僅供迭代，
未納入專案；第 2 版保留厚塗畫風並改變髮式及面貌。未複製 KOEI 肖像。

第 1 版完整生成請求：

```text
Use case: stylized-concept.
Asset type: standalone HD character portrait for a Three Kingdoms preservation remake.
Input image 1 is ONLY a reference for the approved painterly style, brush texture, close portrait framing, and overall finish. Create a distinct new character, Zuo Ci (左慈), instead of the physician in the reference. Leave the reference asset unchanged.
Subject: imagined elderly Chinese Daoist adept of the late Han era, thin angular face, high cheekbones, long flowing silver eyebrows, narrow white beard, an alert slightly playful expression. His facial identity must be clearly different from the reference. Simple cloth Daoist headwear and muted ivory robes with a dark plum collar; no armor.
Style/medium: rich hand-painted oil and gouache portrait with visible impasto, detailed brush-built eyes and skin, organic brush edges. Match the supplied approved portrait's traditional painted game-art quality, warm skin, and deep textured magenta background.
Composition: a single head and shoulders, close three-quarter portrait, readable at small game portrait size, headwear kept inside the image. Opaque full-bleed background. Vertical 4:5 aspect ratio, high definition.
Constraints: original imagined artwork, not a reproduction of any KOEI portrait or other game's art. No text, letters, watermark, frame, collage, extra people, weapons, magical effects, or props. This is an illustration, not a claim about the historical person's actual appearance.
```

第 2 版完整修訂請求，以第 1 版生成原圖為唯一輸入：

```text
Edit this Zuo Ci character portrait. Make a single targeted revision to the face and hair/headwear so he has an unmistakably different identity from the Hua Tuo reference. Replace the black cloth cap with silver-white hair gathered into a small Daoist topknot, exposing a high domed forehead. Give him a distinctly narrow elongated face, long fine eyes, exceptionally long white eyebrows sweeping downward, and a subtle knowing half-smile. Keep the white narrow beard. Keep the existing ivory robe, deep plum collar, shoulder framing, rich visible oil/gouache impasto, warm skin palette, opaque magenta painted background, vertical 4:5 canvas, and all other elements unchanged. No new props, text, armor, halos, or magic effects. This is original imagined character art, not a historical likeness.
```

## 于吉肖像素材

[于吉原圖](../images/expanded-yu-ji-20261012.png)由內建 image_gen 生成，已以 san5:495 接入正式素材目錄。
畫風沿用已選定華佗樣張；寬圓面、短髭鬚、髮髻、木簪及赭紅袍為插畫設計，
不宣稱史實容貌，不複製 KOEI 肖像。

| 項目 | 紀錄 |
|---|---|
| 工具 | 內建 `image_gen.imagegen`，未回報模型或 seed |
| 生成紀錄 | `exec-9b6cf1af-7eaf-4cbb-9984-d47218e26b68` |
| 來源 session | `01a12b5b-89d5-72e2-9615-2e89fe353370` |
| 原圖 | 1122×1402，RGB 不透明 PNG，2,977,301 bytes |
| SHA-256 | `83f5e801fb6220b80d41e1b3adedb9fa3031d713df7151e7837d7cea8d6b7964` |
| 畫風參考 | 本頁華佗樣張，SHA-256 `104090ece088f21a5285909cc477abbd6744868ded0266b0bd3dd3201cacebaf` |
| 保存 | 原圖逐位元組複製；PNG 區塊 CRC、解壓、輸出雜湊及 UID/GID 1000:1000 均通過 |

完整生成請求：

```text
Use case: stylized-concept.
Asset type: one new HD character portrait for the Traditional Chinese remake of Softworld's 1991 Romance of the Three Kingdoms.
Input image 1: style and framing reference ONLY, the user-approved Hua Tuo portrait. Leave this reference unchanged. Do not reproduce this person's facial identity.
Primary request: create a distinct original portrait of 于吉 (Yu Ji), imagined as an elderly late-Han Daoist. Give him a broad rounded face, a wide forehead, gently hooded eyes with a serene observant expression, a short full silver-grey beard and moustache, grey hair gathered into a simple topknot secured by a wooden hairpin, and no cap. His face and short beard must be unmistakably different from Hua Tuo and from a narrow-faced long-bearded Daoist. Muted rust-red cloth robe with a charcoal collar. These are illustrative design choices, not a claim of historical likeness.
Style/medium: match the supplied reference's realistic hand-painted oil/gouache game portrait, richly textured opaque brushwork, crisp readable eyes, warm ochre skin, organic visible paint in hair and clothing, subtle impasto. Preserve the same close portrait scale, natural anatomy and deep vivid pink/magenta mottled painted backdrop. No photographic rendering, glossy airbrush, smooth 3D, or anime.
Composition: one head and shoulders, close three-quarter portrait turned slightly toward the viewer's right. Topknot kept inside the canvas; shoulders extend beyond the bottom and sides. Vertical 4:5 canvas, full-bleed opaque background, no border or margins. Face remains readable at small in-game portrait size.
Text: none.
Constraints: one finished portrait only; no labels, lettering, watermark, frame, collage, other people, books, weapons, magical effects, halos, or props. Original imagined artwork, not a reproduction of KOEI portraits or other game's artwork.
```


## 實作與驗收契約

| 分支 | 已確認契約 | 完成條件 |
|---|---|---|
| 最終名單 | 九組異名以 L2 對應既有人物；候選 220 人、219 種姓名 | 保留兩位馬忠的獨立識別，原有姓名與能力不變 |
| 劇本 | 來源、15 歲登場、68 歲老化門檻及已故君主政策已定 | 年齡換算及年度登場接入本作既有事件路徑 |
| 勢力與城市 | 無主郡原則、具體分配、四組繼承及王朗例外均已確認 | 兩版六劇本按下方配置接入 |
| 人物識別與肖像 | 原始 350 槽及各劇本自創君主欄保留，追加人物採獨立來源鍵 | 原貌與 HD 的缺圖回退、四字姓名及同名人物均可測 |
| 存檔 | 下方已確認格式 2，格式 1 保持原名單 | 完整人物及勢力往返，拒絕破損引用，不靜默截斷 |
| 規則與介面 | 名單、登用、任命、帶兵、AI、事件、翻譯及圖鑑 | 資料到正常 UI、規則及存檔的垂直鏈抽測 |

2026-10-12 共同理解確認前只做研究與試作；目前已授權下方契約正式實作。
已定案的體能、能力對應、特殊人物及畫風不重開訪談。

## 六劇本來源選擇

下表只決定取用哪份來源。San5 欄位為容器內保存的日期，尚未宣稱為完成開局後的畫面日期。
日期載入端及位元組定位見 [RE 14](../re/14-san5-roster.md#劇本來源日期)。

| 本作劇本 | 開局 | San5 劇本 | 來源保存日期 |
|---|---|---:|---|
| 001 | 189 年 1 月 | 1 | 183 年 12 月 |
| 002 | 195 年 1 月 | 2 | 189 年 11 月 |
| 003 | 201 年 1 月 | 3 | 196 年 8 月 |
| 004 | 208 年 1 月 | 4 | 201 年 9 月 |
| 005 | 215 年 1 月 | 5 | 208 年 10 月 |
| 006 | 220 年 1 月 | 6 | 219 年 6 月 |

使用者已於 2026-10-12 選定保留所選 San5 名單，不另按史實剔除已故人物。
年齡依本作開局年份換算；候補登場與勢力對照另作明示契約。
這是採用來源快照的 remake 選擇，不宣稱為本作開局年的正確歷史配置。

## 驗收與停止線

### 已確認的領土原則

使用者已選新增來源勢力，並於 2026-10-12 選定優先分配無主郡：

| 選項 | 結果 |
|---|---|
| 優先分配無主郡，已採用 | 原有領土及駐軍保持；新增勢力所在地會偏離 San5，具體各郡配置及分配順序已確認 |
| 依 San5 重配領土，未採用 | 會取代原有領土，並須另行安置原駐軍 |

具體衝突：來源第一劇本何進位於洛陽；本作 001 洛陽為郡 15，所屬董卓勢力 5，
呂布、華雄、徐榮、賈詡、胡軫及王允六位在職人物都在該郡，另有宋憲、楊奉兩位在野人物。
本作原有 18 個無主郡。以上定位與完整清單在獨立回讀收據，不以歷史印象猜測開局。

原始勢力表只有 16 槽。審查後確認的第一劇本配置為原有 14 個在用勢力加 8 個，
共 22 個。人物、勢力與存檔不能截斷到原始 350／16 槽。

### 六劇本配置

本節為使用者於 2026-10-12 確認的 remake 配置，不宣稱原版或史實開局。
可重生資料在 `workplace/audio/san5-roster-independent-20261012-r3.json`，
來源及推論限制見 [RE 14](../re/14-san5-roster.md#異名與同名審查)。

| 劇本 | 新增人物 | 加入既有勢力 | 加入新增勢力 | 在野 | 未登場 | 新增勢力 |
|---|---:|---:|---:|---:|---:|---:|
| 001，189 年 | 220 | 3 | 31 | 40 | 146 | 8 |
| 002，195 年 | 207 | 8 | 8 | 57 | 134 | 5 |
| 003，201 年 | 198 | 14 | 2 | 58 | 124 | 1 |
| 004，208 年 | 180 | 14 | 0 | 54 | 112 | 0 |
| 005，215 年 | 175 | 32 | 0 | 44 | 99 | 0 |
| 006，220 年 | 168 | 56 | 6 | 21 | 85 | 1 |

各劇本只收所選來源快照中出現的候選，不把七劇本聯集全部塞入每一局。
來源候補滿 15 歲後先作在野，未滿 15 歲不預先加入勢力，從出身郡登場。
已有勢力的新增人物，優先放在該勢力持有的來源對應郡，否則放在現任君主所在郡。
來源君主已成為本作既有武將時，讓其新增部下加入他目前效力的勢力，
例如 201 年的王朗與張繡均對應曹操；不移動原有武將。

繼承表只列所選資料確實用到的四組，不從姓氏或征服關係自動推導：

| 本作劇本 | 來源君主 | 本作現任君主 |
|---|---|---|
| 002 | 孫堅 | 孫策 |
| 002 | 劉焉 | 劉璋 |
| 003 | 孫策 | 孫權 |
| 006 | 曹操 | 曹丕 |

城郡錨點的 47 筆完整對照保存在研究工具的 `CITY_ANCHOR_REVIEW`，收據亦逐筆保存。
它是已確認的 remake 地圖分組，不宣稱兩作行政邊界相同。
新增勢力先取無主的來源錨點；衝突依來源勢力 ID 決定先後。
剩餘勢力使用原有地圖座標，選擇平方距離總和最小的互異無主郡，同分按郡號固定結果。
既有領土、地圖相鄰關係及原有駐軍保持。

| 劇本 | 新增君主 → 分配無主郡 |
|---|---|
| 001 | 張角 → 太原；何進 → 漢中；丁原 → 武陵；龔景 → 遼東；嚴白虎 → 柴桑；韓馥 → 安定；喬玄 → 淮南；孔岫 → 譙郡 |
| 002 | 王朗 → 吳郡；嚴白虎 → 廬陵；韓馥 → 遼東；喬瑁 → 漢中；孔岫 → 譙郡 |
| 003 | 嚴白虎 → 廬陵 |
| 004、005 | 無新增勢力 |
| 006 | 徹里吉 → 酒泉 |

**已確認的王朗例外**：195 年王朗本來是洛陽在野武將，原人物槽 103、身分 8。
使用該原人物作新增勢力君主並移至無主吳郡，不再新增第二個王朗。
姓名與能力保持，會改變其身分及位置；這不屬於原有駐軍移防。

### 初始化

下列是 San5 沒有直接提供本作對應值時的明示 remake 預設，已獲共同理解確認。
體能、能力對應、登場及老化門檻已定案，不在本節重開。

| 欄位 | 已確認值 |
|---|---|
| 一般新增人物職位 | 最低武職 `RankJuniorGeneral`，未任命軍師或太守 |
| 兵力、兵種 | 一般人物 0 兵、兵種 0；新增君主沿用本作範本的 1,000 兵、兵種 6 |
| 訓練、武裝 | 均為 10，採本作自創君主人物範本值 |
| 忠誠、牽絆 | 在職 100，在野及未登場為無值；牽絆指向自己 |
| 新增君主 | 君主職位及身分；同勢力其他新增人物為一般武將 |
| 新增勢力 | 人望 50、無軍師；寶庫為本作範本的五項數量 `0,1,1,1,1`；AI 等級從範本 0 起，套用既有版本及難度初始化 |
| 新領土 | 保留該無主郡的原有資源、人口及地形，改所屬與主事者，再按正式刷新入口更新駐軍快照 |

原始範本事實見 [013](013-custom-lord.md)；實際六劇本欄位由研究收據記錄。
本輪逐版讀回結果在 `workplace/audio/san5-roster-review-validation-20261012-r3.json`
的 `original_template_facts`。
採用這些值是本擴充的明示選擇，不宣稱 San5 或智冠原版曾有此新增名單。

### 人物與存檔格式

| 邊界 | 已確認契約 |
|---|---|
| 原有索引 | 保留人物 0–349 及勢力 0–15，包含各劇本實際的自創君主欄，不挪作擴充欄位 |
| 新增人物 | 接在 350 起，依來源肖像編號固定排序。最多為 350＋220＝570 槽；每局只附所選來源人物 |
| 穩定識別 | 來源代號加肖像編號作鍵，例如 `san5:452` 與 `san5:222`。保存原始姓名及來源檔案雜湊；姓名不作唯一鍵 |
| 新增勢力 | 從 16 起，依來源勢力 ID 排序，保留 `255` 無勢力哨兵 |
| 姓名與肖像 | 追加 Unicode 全名、政治及獨立肖像鍵，保留原 `Portrait` 的原有用途；四字姓名不能截成三字 |
| 缺圖回退 | 新人物在原貌模式及缺少 HD 圖時使用同一個明確的通用人物框，顯示完整姓名，不冒用另一人的肖像 |
| 舊格式 1 | 讀取既有名單，不注入擴充。沒有擴充的局仍可寫格式 1 |
| 新格式 2 | `REMAKE.JSON` 增加擴充版本與完整型別快照；保存全部人物、勢力、郡、政治、來源及肖像識別。正式執行期快照是格式 2 的狀態權威 |
| 原始表 | 原三張表保持原長度作相容資料及未知 bytes 保存，不用它們承載超出容量的正式狀態。格式 2 的讀取不能依原表重建後丟棄新增狀態 |
| 讀取順序 | 驗版本及大小，讀原表，驗完整快照的索引、重複鍵、君主、軍師、牽絆、郡及玩家引用，再一次建成完整狀態 |
| 錯誤與重啟 | 未支援版本、缺少快照、重複人物或破損引用均回報錯誤；不得只載前 350 人或前 16 勢力。讀檔不依賴 San5 原始檔 |

快照須包含既有存檔的選項、亂數及回合進度，並保留人口及駐軍等原本的快照欄位，
不在存讀時重新計算。格式 2 寫出仍用既有暫存檔及替換流程。
具體 JSON 欄位及所有呼叫端依本契約實作，欄位完整性與破損引用列入驗收。

### 共同理解確認

使用者於 2026-10-12 選擇「採用草案，開始實作」，確認具體城郡分配、
王朗身分例外、初始化及格式 2／缺圖回退方案。規格轉 READY，依序接資料、
規則、介面、AI 與存檔。具體契約以本次確認為準。
來源未知的歷史容貌及 San5 完整候補規則不阻塞本擴充，也不宣稱已還原。

### 驗收契約

- 先驗原有 346 人姓名與能力逐欄不變，再驗每位新增人物來源及本作對應。
  原有領土及駐軍保持；新增勢力的無主郡配置依本規格驗證，不能混同為原版開局。
- 兩版正常開新局須能查閱新增人物；抽測登用、任命、內政、戰鬥與存讀檔。
- 新局與舊存檔分開驗。兩位馬忠分開驗。特殊人物按一般武將路徑驗。
- 原版 oracle 只驗原版保留行為；新增人物依本規格驗，不稱為原版對拍。
- 肖像逐筆核對索引、檔案形狀、載入與缺圖回退，再查看正常畫面。
- 未重包、建立 tag 或更新 Release。發行另依正式版號及交付契約處理。

## 正式實作與目前驗證

新遊戲由 [NewExpanded／NewExpandedPlayers](../../internal/game/expanded.go)載入
[六劇本資料](../../internal/game/expanded_roster.json)。原版對拍入口 New／NewPlayers 保持。
新增人物以來源鍵區分同名，登場事件只對新來源採滿 15 歲，原有人物門檻保持。
格式 2 使用 [完整快照](../../internal/game/snapshot.go)作狀態權威，保存全部索引、
駐軍快照、亂數及待處理選擇。相容三表只投影原有容量，讀檔不再注入新人物。
220 年新增徹里吉後只剩三個可安置自創君主名額，原始四個人物範本保持。

| 驗證範圍 | 結果與收據 |
|---|---|
| 合併回歸 | 1,891 個不重複通過測試事件、27 個略過；expanded-tests-20261012-summary.json |
| 特殊人物 | 華佗登用、軍師任命、徵兵、快速及戰術戰鬥、來源與快照保持；expanded-special-20261012-r1.jsonl |
| AI 與存檔 | 兩版六劇本固定 seed 36 個月，每月快照及存讀；另驗未控制月順序的新君主開局能返回玩家、待選繼承人或遊戲結束；expanded-session-20261012-r3.jsonl |
| 破損快照拒收 | 缺失快照、重複來源、越界君主、敵方君主、已故君主、缺失人物及未知版本均拒收；expanded-save-reference-regression-20261012-r1.jsonl |
| 正常 GUI | 兩版 001／002／006 完成新局、人物卡、存檔及重啟載入。001 採難度 1，002／006 採難度 5；r16、r11 及 r6 的三個原版成功樣本 |
| 高清人物卡 | 兩版張角、加強版徹里吉與正式載入器輸出逐像素相符；expanded-gui-20261012-r16 及 r11 |
| 姓名字型 | 新增人物的 323 個相異字，三套正式點陣字型均涵蓋；expanded-font-coverage-20261012.json |
| 原版保留行為 | 君主繼承、絕嗣及兩版玩家戰後表格共四項 oracle 通過；檔名、測試名及雜湊列於合併回歸收據 |
| 素材與組包接線 | 原 904 筆及全部 220 張新增肖像正式載入，兩版各 452＋220 筆、零警告；expanded-package-stage-20261012-r2.json、expanded-hd-pack-20261012-r3.txt |

上述收據均在 workplace/audio/。GUI 的完整檔名為 expanded-gui-20261012-r6、r11、r16
目錄下的 receipt.json；r6 的其他失敗樣本保留，不併入通過數。GUI 使用
[正常操作驗證器](../../tools/verify-expanded-roster-inner.py)，不注入狀態或 seed。
回歸中的 27 項略過不算通過，新增規則也不宣稱原版 oracle parity。

最後審查補入敵方及已故君主引用兩組負對照。原驗證只檢查人物索引範圍，
兩組破損存檔仍被接受；現在在用勢力的君主須同時具有君主身分及該勢力所屬。
修正前收據為 expanded-save-reference-negative-20261012.jsonl，修正後相關回歸
423 項通過、2 項既有略過。正常快照及兩版六劇本 36 個月存讀仍通過。

### 戰敗後繼承與首回合控制流修正

早期加強版 189 年張角、難度 5 的正常新局揭露既有戰後結算缺口：逐人處理死亡時
立即選繼承人，同場尚未處理的張寶隨後成為在野或敵方武將，留下錯誤君主引用。
原失敗收據 expanded-first-turn-20261012-r1.jsonl 及 expanded-gui-20261012-r7 保留。

修正依 [020 §1](020-player-battle-settlement.md) 的既有契約，
全部俘虜處置與勝方安置完成後才處理繼承。兩版各有／無其他繼承人共四組負對照
在修正前失敗，修正後通過；見 expanded-succession-negative-20261012.jsonl 與
expanded-succession-regression-20261012-r1.jsonl。
Session 遇到待選繼承人或太守即暫停推進；所有玩家絕嗣在月底正常結束。
介面先完成既有選擇流程再推進。未修改已確認的兵力、領土、難度或原有 AI。

原版 oracle 重跑 TestZZHeirMatchesTheOriginal、TestZZNoHeirReleasesTerritory、
TestZZPlayerSettlementTables、TestZZPlayerSettlementTablesPlus，四項通過。
原先合批的八分鐘逾時改為分批驗證，沒有把逾時當產品失敗或通過。
未控制月順序時，難度 5 的新增張角仍可能在首個玩家回合前戰敗；現在正確走繼承
或月底結束。兩版張角正常 GUI 的驗收難度為 1，不宣稱所有難度都能存活首回合。

### 新增肖像製作

220／220 張原圖已保存；每張有獨立來源識別、完整生成請求、生成識別及
SHA-256，統一入口為 [素材說明](../../hd-assets/README.md)、
[正式肖像目錄](../../hd-assets/expanded-portraits.json)與
[生成紀錄](../../hd-assets/expanded-portrait-provenance.json)。
[匯入工具](../../tools/import-expanded-portraits.py)在 Docker 內逐張驗證原圖並保存批次縮圖，
全部縮圖及最後補製的武安國原圖已目視檢查。兩位馬忠使用不同來源識別與不同肖像。
所有面貌均為原創插畫設計，沒有歷史容貌聲明。

expanded-art-independent-20261012.py 及同名 .json 獨立核對全部 220 張的來源鍵、姓名、PNG、
尺寸、擁有權及雜湊，與生成工具原檔逐位元組相同。
肖像目錄 SHA-256 為 ec1f74c91cb653183eb8185dd6624c5a854173b6995f1a73341f8b5eb215ca0e，
生成紀錄 SHA-256 為 6c7748fb1b3b2ff5545ef9712d986cb190497af2c5cbbbea623d6eab016b616e。
[新增人物圖鑑](../hd-generals.md#新增人物)已列出全數肖像、來源鍵及能力。

### 重跑入口與驗收界線

分析及驗證使用既有 Docker 映像檔 eob-remake-go:1.26.7-ebiten2.9.9-20261008-r2，
以 UID/GID 1000:1000 執行。原始資料唯讀掛載為 /orig，Go 快取使用專案
workplace/gocache 與 workplace/gomodcache；圖形測試在容器內使用 Xvfb。
資料可用 [匯出工具](../../tools/export-expanded-roster.py)重生，
核心專項位於 internal/game/expanded_test.go、internal/save/expanded_test.go、
internal/session/expanded_test.go 與 internal/ui/expanded_test.go。
正常介面使用上述驗證器；素材組裝呼叫 [stage_hd](../../tools/full-local-inner.py)，
再執行 [正式載入檢查](../../tools/hd-pack-check.go)。

最終正常 GUI 使用的執行檔為 workplace/audio/san1-expanded-20261012-r3，
SHA-256 為 c11fd2bf57f48d12733c86826596717e067da2b560361a4bfc21fe8f0839e1ab。
最終彙整為 workplace/audio/expanded-final-checks-20261012.json，另核對 33 張唯一 GUI PNG、
正常介面寫出的兩份存檔及 310 個本機文件連結；沒有 root-owned 檔案或誤建的 .md 目錄。
原有素材、舊存檔及兩版六劇本以規則測試核對，正常 GUI 抽樣範圍如上。
上述開發階段完成素材組裝及正式載入。後續 `v.1.2.0-20261012` 四平台本機包、
原版＋San5 與 HD 兩張 566 人總覽已完成，正式封包正常路徑及回讀收據見
[打包契約](../release/01-packaging.md#v120-擴充武將與總覽打包契約)。
既有套件、tag 及 Release 保持；未宣稱 Windows／macOS 原生驗收、全遊戲原版 parity
或 San5 未解候補邏輯已還原。
