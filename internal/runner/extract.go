package runner

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"os"
	"path/filepath"
)

type ExtractRequest struct {
	Name      string
	Bytes     []byte
	SHA256Hex string
	TmpDir    string
	NoCleanup bool
}

type ExtractedWorker struct {
	Path    string
	Cleanup func() error
}

func ExtractWorker(req ExtractRequest) (ExtractedWorker, error) {
	if req.Name == "" {
		return ExtractedWorker{}, errors.New("worker name is required")
	}
	if len(req.Bytes) == 0 {
		return ExtractedWorker{}, errors.New("worker bytes are empty")
	}
	if req.SHA256Hex != "" {
		sum := sha256.Sum256(req.Bytes)
		if hex.EncodeToString(sum[:]) != req.SHA256Hex {
			return ExtractedWorker{}, fmt.Errorf("worker %s sha256 mismatch", req.Name)
		}
	}

	root, err := createExtractionRoot(req.TmpDir)
	if err != nil {
		return ExtractedWorker{}, err
	}
	cleanup := func() error { return os.RemoveAll(root) }
	if req.NoCleanup {
		cleanup = func() error { return nil }
	}

	path := filepath.Join(root, filepath.Base(req.Name))
	if err := os.WriteFile(path, req.Bytes, 0o700); err != nil {
		_ = os.RemoveAll(root)
		return ExtractedWorker{}, err
	}
	if err := os.Chmod(path, 0o700); err != nil {
		_ = os.RemoveAll(root)
		return ExtractedWorker{}, err
	}
	return ExtractedWorker{Path: path, Cleanup: cleanup}, nil
}

func createExtractionRoot(override string) (string, error) {
	var candidates []string
	if override != "" {
		candidates = append(candidates, override)
	}
	if xdg := os.Getenv("XDG_RUNTIME_DIR"); xdg != "" {
		candidates = append(candidates, filepath.Join(xdg, "nerdbench"))
	}
	if home, err := os.UserHomeDir(); err == nil && home != "" {
		candidates = append(candidates, filepath.Join(home, ".cache", "nerdbench"))
	}
	candidates = append(candidates, filepath.Join(os.TempDir(), "nerdbench"))

	var errs []error
	for _, base := range candidates {
		if err := os.MkdirAll(base, 0o700); err != nil {
			errs = append(errs, err)
			continue
		}
		root, err := os.MkdirTemp(base, "run-*")
		if err != nil {
			errs = append(errs, err)
			continue
		}
		return root, nil
	}
	return "", errors.Join(errs...)
}
