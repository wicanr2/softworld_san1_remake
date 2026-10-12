# HD 素材包

本目錄是遊戲目前使用的 B 寫實手繪 4× 素材包，既有素材包含 437 張 PNG 與
`manifest.json`。原版及加強版各對應 452 筆資源，共 904 筆；部分資源共用圖片。
人物、場景、地形、天候、旗幟、面板、圖框、選單、片頭與製作群均收錄於此。
另收錄 220 張新增人物肖像，使用下方的獨立肖像目錄。

從專案根目錄啟動時，加上 `-hd-assets hd-assets`。使用下載的引擎包時，
把整個 `hd-assets/` 複製到執行檔旁，或用 `-hd-assets <完整路徑>` 指定：

```sh
./san1 -root /path/to/三國演義 -edition base -hd-assets /path/to/hd-assets
./san1 -root /path/to/三國演義1加強版 -edition plus -hd-assets /path/to/hd-assets
```

Windows 執行 `san1.exe`。啟動後按 Esc 展開選項列，將 Theme 切成 B 高清。
每次啟動預設仍是原貌。遊玩需要自備合法原版資料；這個素材包不能取代遊戲資料。

完整預覽見 [README 素材總覽](../README.md#素材總覽)，逐張來源與輸出雜湊見
[manifest](manifest.json)。[圖錄索引](../docs/images/asset-overviews.json) 另列縮圖的位置與版本。
[獨立武將圖鑑](../docs/hd-generals.md)列出 346 位人物的頭像、姓名、表字與遊戲能力值；
[武將姓名總覽](../docs/images/hd-generals-overview.png)在各張頭像下方標示姓名。
素材製作及玩家路徑的驗收範圍見 [021 高清規格](../docs/spec/021-hd-art.md)，
入庫範圍見 [§6.62](../docs/spec/021-hd-art.md#662-hd-素材入庫與總覽)。

如需重生總覽，在持有兩版原版資料及既有工具映像檔的環境執行：

```sh
bash tools/asset-overviews.sh
```

此工具只在 Docker 匯出及合成。預設讀取專案 `org_game/`，可用 `SAN1_ORIG`
指定包含兩版目錄的其他路徑。原版單張 PNG 保存在被 Git 忽略的
`workplace/asset-overviews/`；公開輸出包含 `docs/images/` 的三張總覽與 JSON 索引，
以及 `docs/hd-generals.md`。武將圖鑑讀取已核對的
[`docs/reference/hd-generals.json`](../docs/reference/hd-generals.json)，表字不寫入遊戲。
映像檔預設為 `eob-remake-go:1.26.7-ebiten2.9.9-20261008-r2` 及
`eob-audio-capture:20261009-r3`，可用 `SAN1_GO_IMAGE`、`SAN1_ART_IMAGE` 指定既有相容映像檔。

規格 022 的新增人物使用獨立[肖像目錄](expanded-portraits.json)，220／220 張已完成。
五位特殊人物、新增君主及其餘新增人物均已納入。使用者提供的素材包若缺圖或圖檔
驗證失敗，會使用通用人物像。
原版的 904 筆目錄保持。
[生成紀錄](expanded-portrait-provenance.json)保存完整請求、參考及輸出雜湊，
[圖鑑新增人物表](../docs/hd-generals.md#新增人物)列出來源識別及政治值。
這些面貌是原創插畫設計，沒有宣稱史實容貌。驗收及限制見
[規格 022](../docs/spec/022-expanded-roster.md)。

後續原圖以 [匯入工具](../tools/import-expanded-portraits.py)在既有 Go／Ebiten Docker
映像檔內保存。輸入批次 JSON 包含人物識別、姓名、完整提示詞及生成原圖路徑；
工具核對 PNG、雜湊及擁有權後，追加肖像目錄與生成紀錄，並輸出驗證縮圖。
本機完整版的 [組包工具](../tools/full-local-inner.py)會把此目錄的新增肖像疊入原素材包，
再以正式載入器驗證。此流程尚未重建既有發行包。

專案授權見 [LICENSE](../LICENSE)。原版素材著作權仍屬智冠科技，原版參照縮圖
不納入本專案授權。生成母圖、未採用候選與製作暫存未收錄於本目錄。
