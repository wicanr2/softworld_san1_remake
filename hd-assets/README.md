# HD 素材包

本目錄是遊戲目前使用的 B 寫實手繪 4× 素材包，包含 437 張 PNG 與
`manifest.json`。原版及加強版各對應 452 筆資源，共 904 筆；部分資源共用圖片。
人物、場景、地形、天候、旗幟、面板、圖框、選單、片頭與製作群均收錄於此。

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
素材製作及玩家路徑的驗收範圍見 [021 高清規格](../docs/spec/021-hd-art.md)，
入庫範圍見 [§6.62](../docs/spec/021-hd-art.md#662-hd-素材入庫與總覽)。

如需重生總覽，在持有兩版原版資料及既有工具映像檔的環境執行：

```sh
bash tools/asset-overviews.sh
```

此工具只在 Docker 匯出及合成。預設讀取專案 `org_game/`，可用 `SAN1_ORIG`
指定包含兩版目錄的其他路徑。原版單張 PNG 保存在被 Git 忽略的
`workplace/asset-overviews/`；公開輸出是 `docs/images/` 的兩張總覽與 JSON 索引。
映像檔預設為 `eob-remake-go:1.26.7-ebiten2.9.9-20261008-r2` 及
`eob-audio-capture:20261009-r3`，可用 `SAN1_GO_IMAGE`、`SAN1_ART_IMAGE` 指定既有相容映像檔。

專案授權見 [LICENSE](../LICENSE)。原版素材著作權仍屬智冠科技，原版參照縮圖
不納入本專案授權。生成母圖、未採用候選與製作暫存未收錄於本目錄。
