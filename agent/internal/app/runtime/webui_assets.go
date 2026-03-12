package runtime

import (
	"os"
	"path/filepath"
)

func (r *Runtime) webUIAssetsDir() string {
	candidates := make([]string, 0, 5)
	if v := filepath.Clean(r.opts.WebUIAssetsDir); v != "." && v != "" {
		candidates = append(candidates, v)
	}
	if r.opts.BinaryPath != "" {
		binaryDir := filepath.Dir(r.opts.BinaryPath)
		candidates = append(candidates,
			filepath.Join(binaryDir, "webui"),
			filepath.Clean(filepath.Join(binaryDir, "..", "web", "dist")),
		)
	}
	if cwd, err := os.Getwd(); err == nil {
		candidates = append(candidates,
			filepath.Join(cwd, "web", "dist"),
			filepath.Clean(filepath.Join(cwd, "..", "web", "dist")),
		)
	}

	for _, candidate := range candidates {
		if hasBundledWebUI(candidate) {
			return candidate
		}
	}
	return ""
}

func hasBundledWebUI(dir string) bool {
	if dir == "" {
		return false
	}
	info, err := os.Stat(filepath.Join(dir, "index.html"))
	if err != nil {
		return false
	}
	return !info.IsDir()
}
