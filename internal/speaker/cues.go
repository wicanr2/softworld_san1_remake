package speaker

import (
	_ "embed"
	"encoding/json"
)

//go:embed voice_catalog.json
var voiceCatalogJSON []byte

var voiceCatalog struct {
	Cues map[string][3]int `json:"cues"`
}

func init() {
	if err := json.Unmarshal(voiceCatalogJSON, &voiceCatalog); err != nil {
		panic("invalid embedded voice catalog: " + err.Error())
	}
}

// VoiceClipsFor resolves a semantic dialogue key using its original three slots.
// person is an original general-table index, never a portrait or translated name.
// Evidence and snapshot rules: spec/008 §10, re/09 §10, and re/12.
func VoiceClipsFor(key string, person int) ([3]int, bool) {
	indices, ok := voiceCatalog.Cues[key]
	if !ok {
		return [3]int{}, false
	}
	for i, n := range indices {
		if n == -1 {
			if person < 0 || person >= 350 {
				return [3]int{}, false
			}
			indices[i] = person
		} else if n < 0 || n > 499 {
			return [3]int{}, false
		}
	}
	return indices, true
}
