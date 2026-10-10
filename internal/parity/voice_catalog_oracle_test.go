//go:build oracle

package parity

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/wicanr2/dosgolem/oracle"
)

// TestVoiceCatalogSnapshotPlus captures an independent plus runtime for static
// catalog analysis. Normal new-game inputs; only RNG is controlled before use.
// No gameplay tables or audio gates are injected. Raw output stays local.
func TestVoiceCatalogSnapshotPlus(t *testing.T) {
	out := os.Getenv("SAN1_VOICE_CATALOG_OUT")
	if out == "" {
		t.Skip("set SAN1_VOICE_CATALOG_OUT to a private evidence directory")
	}
	root := plusRoot(t)
	o, err := oracle.Load(filepath.Join(root, "ASV.EXE"), root)
	if err != nil {
		t.Fatal(err)
	}
	defer o.Close()
	const seed = uint32(0x13579bdf)
	var seededAt uint64
	o.OnCall(addr(plusRndFn), func(o *oracle.Oracle) {
		if seededAt != 0 {
			return
		}
		ds := uint32(o.DSReg()) * 16
		o.SetWord(addr(ds+0xa566), uint16(seed&0xffff))
		o.SetWord(addr(ds+0xa568), uint16(seed>>16))
		seededAt = o.Steps()
	})
	bootToNewGamePlus(t, o, 1, 5)
	if seededAt == 0 {
		t.Fatal("initial RNG hook was not hit")
	}
	const base = uint32(0x1100)
	raw := o.Bytes(addr(base), 0xa0000-int(base))
	digest := sha256.Sum256(raw)
	path := filepath.Join(out, "plus-runtime.bin")
	file, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o644)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := file.Write(raw); err != nil {
		t.Fatal(err)
	}
	if err := file.Close(); err != nil {
		t.Fatal(err)
	}
	metadata := map[string]any{
		"edition": "plus", "input": "ASV.EXE", "input_sha256": sha256File(t, filepath.Join(root, "ASV.EXE")),
		"data5_sha256":   sha256File(t, filepath.Join(root, "DATA5.GRP")),
		"runtime_sha256": hex.EncodeToString(digest[:]), "runtime_base": base,
		"ds_base": uint32(o.DSReg()) * 16, "seed": seed, "seeded_at_step": seededAt,
		"steps": o.Steps(), "normal_new_game": true, "state_injection": false,
		"rights": "local_only_original_runtime", "dosgolem_commit": os.Getenv("SAN1_DOSGOLEM_COMMIT"),
	}
	encoded, err := json.MarshalIndent(metadata, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(out, "plus-runtime.json"), append(encoded, '\n'), 0o644); err != nil {
		t.Fatal(err)
	}
	t.Logf("plus catalog runtime SHA-256=%x, seed=%#x; normal new game", digest, seed)
}
