package main

import (
	"encoding/binary"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/wicanr2/softworld_san1_remake/internal/assets"
)

func inventoryFixture(t *testing.T, names []string, blobs [][]byte) *assets.Container {
	t.Helper()
	nam, idx := make([]byte, 16*len(names)), make([]byte, 4*len(names))
	var grp []byte
	for i, name := range names {
		parts := strings.Split(name, ".")
		copy(nam[i*16:i*16+8], parts[0])
		copy(nam[i*16+9:i*16+12], parts[1])
		grp = append(grp, blobs[i]...)
		binary.LittleEndian.PutUint32(idx[i*4:], uint32(len(grp)))
	}
	c, err := assets.OpenContainer(nam, idx, grp)
	if err != nil {
		t.Fatal(err)
	}
	return c
}

func portraitFixture(w, h int) []byte {
	b := make([]byte, 4+((w+7)/8)*h*4)
	binary.LittleEndian.PutUint16(b, uint16(h))
	binary.LittleEndian.PutUint16(b[2:], uint16(w))
	return b
}

func TestInventoryRejectsDuplicateAndInvalidImages(t *testing.T) {
	for _, tc := range []struct {
		name  string
		names []string
		blobs [][]byte
		want  string
	}{
		{"duplicate", []string{"F000.FAC", "F000.FAC"}, [][]byte{portraitFixture(64, 80), portraitFixture(64, 80)}, "重複資源鍵"},
		{"swapped_dimensions", []string{"F000.FAC"}, [][]byte{portraitFixture(80, 64)}, "肖像尺寸錯誤"},
		{"truncated", []string{"SCG01.IMG"}, [][]byte{portraitFixture(176, 96)[:100]}, "解碼失敗"},
		{"invalid_mask", []string{"ENDO4.MSK"}, [][]byte{portraitFixture(640, 151)}, "解碼失敗"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			_, err := inventoryContainer("DATA3", inventoryFixture(t, tc.names, tc.blobs))
			if err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("err=%v，預期 %s", err, tc.want)
			}
		})
	}
}

func TestInventoryComparisonDoesNotConflateMissingAndChanged(t *testing.T) {
	es := []inventoryEdition{
		{Assets: []inventoryAsset{{Key: "same", SHA256: "a"}, {Key: "changed", SHA256: "b"}, {Key: "missing", SHA256: "c"}}},
		{Assets: []inventoryAsset{{Key: "same", SHA256: "a"}, {Key: "changed", SHA256: "d"}}},
	}
	compareInventory(es)
	if es[0].Assets[0].Shared == nil || !*es[0].Assets[0].Shared {
		t.Fatal("相同項目未標為共用")
	}
	if es[0].Assets[1].Shared == nil || *es[0].Assets[1].Shared {
		t.Fatal("改變項目誤標為共用")
	}
	if es[0].Assets[2].Shared != nil {
		t.Fatal("缺少項目誤標成已完成跨版比較")
	}
}

func TestInventoryRootRejectsMissingPortrait(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "AA.EXE"), []byte("fixture"), 0o644); err != nil {
		t.Fatal(err)
	}
	for _, cn := range []string{"DATA1", "DATA2", "DATA3"} {
		var nam, idx, grp []byte
		names := []string{"DUMMY.DAT"}
		blobs := [][]byte{{0}}
		if cn == "DATA3" {
			names = nil
			blobs = nil
			for n := 0; n < 256; n++ {
				if n != 127 {
					names = append(names, fmt.Sprintf("F%03d.FAC", n))
					blobs = append(blobs, portraitFixture(64, 80))
				}
			}
		}
		for i, name := range names {
			r := make([]byte, 16)
			parts := strings.Split(name, ".")
			copy(r[:8], parts[0])
			copy(r[9:12], parts[1])
			nam = append(nam, r...)
			grp = append(grp, blobs[i]...)
			end := make([]byte, 4)
			binary.LittleEndian.PutUint32(end, uint32(len(grp)))
			idx = append(idx, end...)
		}
		for ext, b := range map[string][]byte{".NAM": nam, ".IDX": idx, ".GRP": grp} {
			if err := os.WriteFile(filepath.Join(root, cn+ext), b, 0o644); err != nil {
				t.Fatal(err)
			}
		}
	}
	_, err := inventoryRoot(root)
	if err == nil || !strings.Contains(err.Error(), "缺少肖像 DATA3/F127.FAC") {
		t.Fatalf("err=%v，預期缺少 F127", err)
	}
}
