//go:build oracle

package parity

import (
	"crypto/sha256"
	"path/filepath"
	"testing"

	"github.com/wicanr2/dosgolem/oracle"
)

// 直接觀測兩版開機讀取的曲庫，不以 remake 的選曲作原版證據。
func TestOpeningLoadsLongMusic(t *testing.T) {
	for _, ed := range []struct{ name, root, exe string }{
		{"base", origRoot(t), "AA.EXE"}, {"plus", plusRoot(t), "ASV.EXE"},
	} {
		t.Run(ed.name, func(t *testing.T) {
			o, err := oracle.Load(filepath.Join(ed.root, ed.exe), ed.root)
			if err != nil {
				t.Fatal(err)
			}
			defer o.Close()
			o.AdLib(true)
			o.TraceFiles()
			o.Type("222")
			if err := o.Run(160_000_000); err != nil {
				t.Fatal(err)
			}
			c := openContainer(t, filepath.Join(ed.root, "DATA1"))
			seen := map[string]bool{}
			for _, op := range o.FileOps() {
				if op.Name != "DATA1.GRP" || op.Op != "seek" {
					continue
				}
				for i := 0; i < c.Len(); i++ {
					e := c.Entry(i)
					if int64(e.Start) == op.Pos && (e.Name == "MUSV.IDX" || e.Name == "MUSV.GRP" || e.Name == "MUS.GRP") {
						seen[e.Name] = true
						t.Logf("original file offset=%d %s SHA256=%x", op.Pos, e.Name, sha256.Sum256(c.Data(i)))
					}
				}
			}
			if !seen["MUSV.IDX"] || !seen["MUSV.GRP"] {
				t.Fatalf("片頭沒有讀到長曲庫：%v", seen)
			}
		})
	}
}
