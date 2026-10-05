package ui

import (
	"crypto/sha256"
	"fmt"
	"image"

	"github.com/wicanr2/softworld_san1_remake/internal/assets"
	"github.com/wicanr2/softworld_san1_remake/internal/state"
)

const hdWorldMapSize = 336

// hdWorldMapCoverage 只接受 spec/021 §6.49 已驗的地圖及六組 typed 座標。
// 遮罩每包建立一次；勢力填色仍由 ArtScreen.Compose 提供。
func hdWorldMapCoverage(containers map[string]*assets.Container) ([]bool, error) {
	d2, d3 := containers["DATA2"], containers["DATA3"]
	if d2 == nil || d3 == nil {
		return nil, fmt.Errorf("大地圖缺少 DATA2／DATA3")
	}
	raw := &assets.Image{W: hdWorldMapSize, H: hdWorldMapSize, Pix: make([]byte, hdWorldMapSize*hdWorldMapSize)}
	for half, name := range []string{"MAINMAP4.IMG", "MAINMAP5.IMG"} {
		i, ok := d3.ByName(name)
		if !ok {
			return nil, fmt.Errorf("大地圖缺少 %s", name)
		}
		im, err := assets.DecodeImage(d3.Data(i))
		if err != nil || im.W != 168 || im.H != hdWorldMapSize {
			return nil, fmt.Errorf("大地圖來源尺寸不符")
		}
		raw.Blit(im, half*168, 0)
	}
	if fmt.Sprintf("%x", sha256.Sum256(raw.Pix)) != "0ea65e5c4c5f474adb237f075bde628de9edadb06c7255b4ee5ee036181533e5" {
		return nil, fmt.Errorf("大地圖語意來源未知")
	}
	var seeds []image.Point
	for _, slot := range []state.Slot{state.Scenario1, state.Scenario2, state.Scenario3, state.Scenario4, state.Scenario5, state.Scenario6} {
		sc, err := state.LoadScenario(d2, slot)
		if err != nil {
			return nil, fmt.Errorf("大地圖劇本 %s: %w", slot, err)
		}
		prefs := sc.Prefectures()
		if len(prefs) != state.PrefectureCount {
			return nil, fmt.Errorf("大地圖郡數不符")
		}
		for i, p := range prefs {
			pt := image.Pt(int(p.MapX)+assets.MapOriginX-72, int(p.MapY)+assets.MapOriginY-36)
			if !pt.In(image.Rect(0, 0, hdWorldMapSize, hdWorldMapSize)) || raw.At(pt.X, pt.Y) != 15 {
				return nil, fmt.Errorf("大地圖劇本 %s 郡 %d 種子不在白色區域", slot, i+1)
			}
			if slot == state.Scenario1 {
				seeds = append(seeds, pt)
			} else if pt != seeds[i] {
				return nil, fmt.Errorf("大地圖跨劇本座標不同")
			}
		}
	}
	return protectWorldMap(raw, seeds)
}

func protectWorldMap(raw *assets.Image, seeds []image.Point) ([]bool, error) {
	fill := raw.Clone()
	white := 0
	for _, pt := range seeds {
		if !pt.In(image.Rect(0, 0, raw.W, raw.H)) || fill.At(pt.X, pt.Y) != 15 {
			return nil, fmt.Errorf("大地圖郡種子重複或越界")
		}
		white += fill.FloodFill(pt.X, pt.Y, 254)
	}
	if white != 48494 {
		return nil, fmt.Errorf("大地圖郡區面積不符")
	}
	protected := make([]bool, len(raw.Pix))
	outside := make([]bool, len(raw.Pix))
	queue := make([]int, 0, len(raw.Pix))
	for i, code := range fill.Pix {
		protected[i] = code == 254
	}
	add := func(i int) {
		if !protected[i] && !outside[i] {
			outside[i] = true
			queue = append(queue, i)
		}
	}
	for x := 0; x < raw.W; x++ {
		add(x)
		add((raw.H-1)*raw.W + x)
	}
	for y := 0; y < raw.H; y++ {
		add(y * raw.W)
		add(y*raw.W + raw.W - 1)
	}
	for head := 0; head < len(queue); head++ {
		i := queue[head]
		x, y := i%raw.W, i/raw.W
		if x > 0 {
			add(i - 1)
		}
		if x+1 < raw.W {
			add(i + 1)
		}
		if y > 0 {
			add(i - raw.W)
		}
		if y+1 < raw.H {
			add(i + raw.W)
		}
	}
	core := 0
	for i := range protected {
		protected[i] = !outside[i]
		if protected[i] {
			core++
		}
	}
	if core != 52966 {
		return nil, fmt.Errorf("大地圖郡號保護面積不符")
	}
	rim := append([]bool(nil), protected...)
	for i, covered := range protected {
		if !covered {
			continue
		}
		x, y := i%raw.W, i/raw.W
		for dy := -1; dy <= 1; dy++ {
			for dx := -1; dx <= 1; dx++ {
				xx, yy := x+dx, y+dy
				if xx >= 0 && xx < raw.W && yy >= 0 && yy < raw.H {
					rim[yy*raw.W+xx] = true
				}
			}
		}
	}
	ink := 0
	for y := 31; y < 65; y++ {
		for x := 78; x < 178; x++ {
			code := raw.At(x, y)
			if code == 1 {
				continue
			}
			if code != 3 && code != 9 && code != 11 {
				return nil, fmt.Errorf("大地圖標題字色不符")
			}
			rim[y*raw.W+x] = true
			ink++
		}
	}
	count := 0
	for _, covered := range rim {
		if covered {
			count++
		}
	}
	if ink != 1375 || count != 61874 {
		return nil, fmt.Errorf("大地圖文字或完整保護面積不符")
	}
	return rim, nil
}
