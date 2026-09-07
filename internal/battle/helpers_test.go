package battle

// 測試用的組裝工具。
//
// 戰場自己生成的版面是隨機的，測規則時要的是**可預測的地形**，
// 所以這裡直接鋪一張單一地形的場，再逐格改要測的那幾格。

// flat 鋪一張整片同一種地形的戰場，城池照樣放在中央。
func flat(t Terrain) *Field {
	f := &Field{W: FieldW, H: FieldH, cell: make([]Terrain, FieldW*FieldH),
		Gates: map[int][]Hex{}}
	for i := range f.cell {
		f.cell[i] = t
	}
	f.CityAt = FromOffset(FieldW/2, FieldH/2)
	f.Set(f.CityAt, City)
	return f
}

// lead 造一位中庸的將領。要測什麼就改哪一項。
func lead(name string, war, intel uint8, soldiers int) Leader {
	return Leader{
		Name: name, War: war, Intel: intel, Stamina: 100, Charm: 50,
		Soldiers: soldiers, Training: 50, Arms: 50, Troop: TroopLand,
	}
}

// arena 開一場空的戰役，天氣預設晴。
func arena(f *Field) *Battle {
	return &Battle{Field: f, Day: 1, CityHeld: MainDefender, rng: newRand(1)}
}

// place 把一支部隊放到指定的格子上。
func place(b *Battle, s Side, form Formation, at Hex, ls ...Leader) *Unit {
	u := &Unit{Side: s, Formation: form, Leaders: ls, At: at}
	u.Move = u.MovePoints()
	b.Units = append(b.Units, u)
	return u
}
