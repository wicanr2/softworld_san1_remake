// Package parity 是對拍：拿 remake 的結果與**原版自己的行為**逐項比對。
//
// 判準不是「remake 看起來像原版」，是「原版在同一個狀態下說了什麼」。
// 原版跑在 dosgolem（`~/cht/dosgolem-san`）裡，程序內執行、直接讀它的
// 主控台、開檔紀錄與記憶體。
//
// # 為什麼要 build tag
//
// 這裡的測試需要兩樣本機才有的東西：
//
//   - **原版素材**（`SAN1_ORIG`）——本儲存庫不含任何原版檔案
//   - **dosgolem 的原始碼**（go workspace 掛在 `/dosgolem`）
//
// 所以全部放在 `//go:build oracle` 之下。沒有這兩樣的人跑
// `go test ./...` 會直接跳過，**不會紅**——但也不會假裝通過：
// 缺素材時是 skip 並說明缺什麼，不用自製代用品。
//
//	SAN1_ORIG=$PWD/org_game tools/go.sh test -tags oracle ./internal/parity/
package parity
