package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestPopulateOutputMetadata(t *testing.T) {
	out := t.TempDir()
	if err := os.Mkdir(filepath.Join(out, "img"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(out, "img", "a.bin"), []byte("abc"), 0o644); err != nil {
		t.Fatal(err)
	}
	mf := manifest{Files: []fileRecord{{Path: filepath.Join("img", "a.bin")}}}
	if err := populateOutputMetadata(out, &mf); err != nil {
		t.Fatal(err)
	}
	got := mf.Files[0]
	if got.OutputBytes != 3 {
		t.Fatalf("output_bytes = %d，預期 3", got.OutputBytes)
	}
	const want = "ba7816bf8f01cfea414140de5dae2223b00361a396177a9cb410ff61f20015ad"
	if got.OutputSHA256 != want {
		t.Fatalf("output_sha256 = %q，預期 %q", got.OutputSHA256, want)
	}
}

func TestPopulateOutputMetadataRejectsUntrackedOutput(t *testing.T) {
	out := t.TempDir()
	if err := os.WriteFile(filepath.Join(out, "untracked.bin"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	err := populateOutputMetadata(out, &manifest{})
	if err == nil || !strings.Contains(err.Error(), "輸出未登錄 manifest") {
		t.Fatalf("err = %v，預期未登錄輸出錯誤", err)
	}
}

func TestPopulateOutputMetadataRejectsDuplicatePath(t *testing.T) {
	out := t.TempDir()
	if err := os.WriteFile(filepath.Join(out, "a.bin"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	mf := manifest{Files: []fileRecord{{Path: "a.bin"}, {Path: "a.bin"}}}
	err := populateOutputMetadata(out, &mf)
	if err == nil || !strings.Contains(err.Error(), "路徑重複") {
		t.Fatalf("err = %v，預期重複路徑錯誤", err)
	}
}
