package battle
// 對戰子畫面的十六張子地圖版型（`AA.EXE` 資料段 `DS:0x88c8`，檔案位移
// `0x47c22` 起 1920 個位元組，`L0`、`[base]`；`docs/re/05` §10.1）。
//
// 每張 10 列 × 12 欄、一格一個位元組：低四位是地形碼，高四位小於 10 的
// 是守方第 n 位將領的起點，`0xff` 是圖外。版型的挑法是
// `(守方部隊所在格的地形碼 − 2) × 2`，主戰場是 8 欄的窄圖再加一
// （`0x2e20d`–`0x2e225`）——所以每種地形一寬一窄，順序山丘、淺水、深水、
// 城池、關寨、平原、樹林、沙漠。寬圖只有 0–6 列（第 6 列奇數欄是圖外），
// 窄圖只有 0–7 欄。
//
// 這是**版面資料不是美術**：與州郡記錄裡的 42 張戰場圖同一種東西
// （`docs/mechanics/40` §3），圖塊本身仍不重製、不散布。
// `TestSkirmishTemplatesMatchTheExe` 有素材時逐位元組核對。
var skirmishTemplates = [16][SkirmishRows]string{
	// 0：山丘・寬
	{
		"f2f2f2f2f2f2f2f2f2f2f272",
		"f2f8f2f8f2f8f2f1f1f25222",
		"f2f2f2f2f1f2f1f2f2923802",
		"f2f2f2f1f2f2f2f1f2f24212",
		"f2f2f2f2f1f2f2f1f1f26282",
		"f2f2f8f2f2f2f1f2f1f2f8f2",
		"f2fff2fff8fff2fff8fff2ff",
		"ffffffffffffffffffffffff",
		"ffffffffffffffffffffffff",
		"ffffffffffffffffffffffff",
	},
	// 1：山丘・窄
	{
		"f2f2f2220232f2f2ffffffff",
		"f2f16242125272f2ffffffff",
		"f2f8f1f18292f8f2ffffffff",
		"f2f2f2f2f2f1f1f2ffffffff",
		"f2f1f8f2f2f2f2f2ffffffff",
		"f2f2f1f1f2f2f2f2ffffffff",
		"f2f2f2f2f2f2f8f2ffffffff",
		"f2f2f8f2f2f1f1f2ffffffff",
		"f8f2f2f2f2f2f2f2ffffffff",
		"f2f2f2f2f2f2f2f2ffffffff",
	},
	// 2：淺水・寬
	{
		"f3f3f4f4f4f4f3f3f3f3f3f3",
		"f3f3f3f4f4f3f3f3f3836333",
		"f3f3f3f3f3f3f3f3f3531303",
		"f3f3f3f3f3f3f3f3f3932343",
		"f3f3f3f4f3f4f3f3f3f373f3",
		"f3f4f4f4f4f4f4f4f3f3f3f3",
		"f3fff4fff4fff4fff3fff3ff",
		"ffffffffffffffffffffffff",
		"ffffffffffffffffffffffff",
		"ffffffffffffffffffffffff",
	},
	// 3：淺水・窄
	{
		"f3f3130323f3f3f3ffffffff",
		"f37343335363f3f3ffffffff",
		"f3f3f383f3f3f3f3ffffffff",
		"f3f3f393f3f3f3f4ffffffff",
		"f4f3f3f3f3f3f4f4ffffffff",
		"f4f4f3f3f3f4f4f4ffffffff",
		"f4f4f3f3f3f3f4f4ffffffff",
		"f4f3f3f3f3f3f3f4ffffffff",
		"f3f3f3f3f3f3f3f3ffffffff",
		"f3f3f3f3f3f3f3f3ffffffff",
	},
	// 4：深水・寬
	{
		"f4f4f4f4f4f4f4f4f4f4f4f4",
		"f4f4f4f4f4f4f4f4f4846434",
		"f4f4f4f4f4f4f4f4f4541404",
		"f4f4f4f4f4f4f4f4f4942444",
		"f4f4f4f4f4f4f4f4f4f474f4",
		"f4f4f4f4f4f4f4f4f4f4f4f4",
		"f4fff4fff4fff4fff4fff4ff",
		"ffffffffffffffffffffffff",
		"ffffffffffffffffffffffff",
		"ffffffffffffffffffffffff",
	},
	// 5：深水・窄
	{
		"f4f4140424f4f4f4ffffffff",
		"f47444345464f4f4ffffffff",
		"f4f4f484f4f4f4f4ffffffff",
		"f4f4f494f4f4f4f4ffffffff",
		"f4f4f4f4f4f4f4f4ffffffff",
		"f4f4f4f4f4f4f4f4ffffffff",
		"f4f4f4f4f4f4f4f4ffffffff",
		"f4f4f4f4f4f4f4f4ffffffff",
		"f4f4f4f4f4f4f4f4ffffffff",
		"f4f4f4f4f4f4f4f4ffffffff",
	},
	// 6：城池・寬
	{
		"f7f7f7f0f0fdfdfdfdfdfdfd",
		"f7f8f7f0f0fdfd6bfbfbfbfb",
		"f7f7f7faf0aafd1b3b8bfbfb",
		"f7f7f7aaaafaaa2b5b9b0bfb",
		"f7f8f7f0f0fdfd7b4bfbfbfb",
		"f7f7f7f0f0fdfdfdfbfdfbfd",
		"f7fff7fff0fffdfffdfffdff",
		"ffffffffffffffffffffffff",
		"ffffffffffffffffffffffff",
		"ffffffffffffffffffffffff",
	},
	// 7：城池・窄
	{
		"fbfbfb0bfbfbfbfbffffffff",
		"fbfbfb9bfbfbfbfbffffffff",
		"fbfb7b6b8bfbfbfbffffffff",
		"fbfd4b1b5bfdfbfdffffffff",
		"fdfd2baa3bfdfdfdffffffff",
		"fdf0fdaafdf0fdf0ffffffff",
		"f0f0f0aaf0f0f0f0ffffffff",
		"f0f8f0f7f0f7f0f7ffffffff",
		"f7f7f7f7f7f7f8f7ffffffff",
		"f7f7f7f7f7f7f7f7ffffffff",
	},
	// 8：關寨・寬
	{
		"f7f7f8f8fefefefefefefefe",
		"f7f7f8f7fefefc6cfcfcfcfe",
		"f7f7f7a7fe1c3c8cfcfcfcfe",
		"f7f7f7a7aa2c5c9c0cfcfcfe",
		"f7f7f7f7fefe4c7cfcfcfcfe",
		"f7f7f8f8fefefcfefcfefcfe",
		"f7fff8fffefffefffefffeff",
		"ffffffffffffffffffffffff",
		"ffffffffffffffffffffffff",
		"ffffffffffffffffffffffff",
	},
	// 9：關寨・窄
	{
		"fefefefefefefefeffffffff",
		"fefcfc0cfcfcfcfeffffffff",
		"fefcfc9cfcfcfcfeffffffff",
		"fefc7c6c8cfcfcfeffffffff",
		"fefe4c1c5cfefefeffffffff",
		"fefe2caa3cfefefeffffffff",
		"fef8fea7fef8f8f8ffffffff",
		"f8f7f8f7f7f7f7f7ffffffff",
		"f7f7f7f7f7f7f7f7ffffffff",
		"f7f7f7f7f7f7f7f7ffffffff",
	},
	// 10：平原・寬
	{
		"f7f7f7f7f7f0f0f7f7f7f7f7",
		"f7f7f7f7f7f0f7f8f7776737",
		"f7f7f8f7f7f7f7f7f7971707",
		"f7f7f7f7f7f7f7f7f7872847",
		"f7f8f7f7f7f7f7f7f7f757f7",
		"f7f7f7f0f0f7f8f7f0f0f7f7",
		"f7fff0fff7fff7fff7fff7ff",
		"ffffffffffffffffffffffff",
		"ffffffffffffffffffffffff",
		"ffffffffffffffffffffffff",
	},
	// 11：平原・窄
	{
		"f7f7f7270737f7f7ffffffff",
		"f7f76747175777f0ffffffff",
		"f7f0f7f787f7f0f7ffffffff",
		"f7f7f8f797f8f7f7ffffffff",
		"f7f7f7f7f7f7f7f7ffffffff",
		"f7f0f7f7f7f7f7f7ffffffff",
		"f0f7f7f7f8f7f7f0ffffffff",
		"f7f8f7f7f7f7f0f7ffffffff",
		"f7f7f7f7f7f7f8f7ffffffff",
		"f7f7f7f7f7f7f7f7ffffffff",
	},
	// 12：樹林・寬
	{
		"f7f7f8f7f8f0f0f8f8f7f8f7",
		"f8f8f7f8f8f8f8f8f8f8f8f8",
		"f8f8f8f7f7f7f7f7f7785727",
		"f7f7f7f8f8f7f8f8f8973808",
		"f8f8f8f8f7f8f8f7f7884818",
		"f8f7f7f0f8f7f8f8f8f868f7",
		"f7fff0fff7fff7fff8fff7ff",
		"ffffffffffffffffffffffff",
		"ffffffffffffffffffffffff",
		"ffffffffffffffffffffffff",
	},
	// 13：樹林・窄
	{
		"f8f7f7170728f8f8ffffffff",
		"f8f87848385867f7ffffffff",
		"f8f7f888f898f8f8ffffffff",
		"f7f8f8f8f8f7f8f0ffffffff",
		"f8f0f8f8f7f8f0f8ffffffff",
		"f0f8f8f7f7f8f8f8ffffffff",
		"f8f7f8f8f8f8f8f8ffffffff",
		"f7f8f8f7f7f7f7f8ffffffff",
		"f7f8f8f7f8f8f0f0ffffffff",
		"f8f8f7f8f8f7f8f8ffffffff",
	},
	// 14：沙漠・寬
	{
		"f9f9f9f9f9f9f9f9f9f9f9f9",
		"f9f9f9f9f9f9f9f9f9795919",
		"f9f9f9f9f9f9f9f9f9993909",
		"f9f9f9f9f9f9f9f9f9894929",
		"f9f9f9f9f9f9f9f9f9f969f9",
		"f9f9f9f9f9f9f9f9f9f9f9f9",
		"f9fff9fff9fff9fff9fff9ff",
		"ffffffffffffffffffffffff",
		"ffffffffffffffffffffffff",
		"ffffffffffffffffffffffff",
	},
	// 15：沙漠・窄
	{
		"f9f9190929f9f9f9ffffffff",
		"f96949395979f9f9ffffffff",
		"f9f9f989f9f9f9f9ffffffff",
		"f9f9f999f9f9f9f9ffffffff",
		"f9f9f9f9f9f9f9f9ffffffff",
		"f9f9f9f9f9f9f9f9ffffffff",
		"f9f9f9f9f9f9f9f9ffffffff",
		"f9f9f9f9f9f9f9f9ffffffff",
		"f9f9f9f9f9f9f9f9ffffffff",
		"f9f9f9f9f9f9f9f9ffffffff",
	},
}
