package runner

import (
	"crypto/sha256"
	"encoding/hex"
	"os"
	"path/filepath"
	"testing"
)

func TestExtractWorker(t *testing.T) {
	bytes := []byte("#!/bin/sh\necho ok\n")
	sum := sha256.Sum256(bytes)
	worker, err := ExtractWorker(ExtractRequest{
		Name:      "worker.sh",
		Bytes:     bytes,
		SHA256Hex: hex.EncodeToString(sum[:]),
		TmpDir:    t.TempDir(),
	})
	if err != nil {
		t.Fatal(err)
	}
	defer worker.Cleanup()
	info, err := os.Stat(worker.Path)
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode().Perm()&0o100 == 0 {
		t.Fatalf("worker is not executable: %s", info.Mode())
	}
	if filepath.Base(worker.Path) != "worker.sh" {
		t.Fatalf("worker path = %s", worker.Path)
	}
}

func TestExtractWorkerHashMismatch(t *testing.T) {
	_, err := ExtractWorker(ExtractRequest{
		Name:      "worker.sh",
		Bytes:     []byte("abc"),
		SHA256Hex: "deadbeef",
		TmpDir:    t.TempDir(),
	})
	if err == nil {
		t.Fatal("expected hash mismatch")
	}
}
