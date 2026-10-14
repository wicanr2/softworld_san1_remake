# HD 重新配音樣音

這裡保存三段既有 AI 配音，供試聽與後續製作。全遊戲 HD 配音尚未完成，
聲線與人耳驗收仍待確認；樣音尚未接入正式遊戲。

| 樣音 | 內容 | 時長 |
|---|---|---|
| [軍師 A](adviser-a.ogg) | 兵者貴神速。 | 1.84 秒 |
| [武將 B](commander-b.ogg) | 兵者貴神速。 | 1.60 秒 |
| [卷首詞](poem-narration.ogg) | 滾滾長江東逝水，浪花淘盡英雄。 | 6.16 秒 |

OGG 為 24 kHz 單聲道 Vorbis，quality 6。同名 WAV 是未壓縮生成來源。
這些檔案沒有混入原版錄音、配樂或影片。

[生成收據](generation-receipt.json)保存模型修訂、seed、完整文字與表演提示詞。
[轉檔收據](voice-samples-QA.json)保存 OGG 與來源 WAV 的 SHA-256。
[保存驗證](archive.json)保存每檔大小、雜湊、完整解碼、時長、峰值與音量檢查。

生成工具與 Docker 用法見[重新配音入口](../../docs/release/01-packaging.md#高清重新配音樣音)。
本目錄由使用者於 2026-10-14 要求保存至 GitHub；原版配樂與混剪影片仍只留本機。
