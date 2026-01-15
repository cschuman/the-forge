package scanner

import (
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

type FileInfo struct {
	Path    string
	Size    int64
	ModTime time.Time
	IsDir   bool
}

type ScanResult struct {
	Files       []FileInfo
	TotalSize   int64
	TotalFiles  int
	TotalDirs   int
	ScanTime    time.Duration
	Errors      []string
}

// Known cache/temp directories that are safe to clean
var CacheDirs = map[string]string{
	"node_modules":    "npm packages (can reinstall)",
	".npm":            "npm cache",
	".pnpm-store":     "pnpm cache",
	".yarn":           "yarn cache",
	"__pycache__":     "Python bytecode cache",
	".pytest_cache":   "pytest cache",
	".mypy_cache":     "mypy cache",
	".cache":          "generic cache",
	".gradle":         "Gradle cache",
	".m2":             "Maven cache",
	"target":          "Rust/Java build output",
	"build":           "build output",
	"dist":            "distribution output",
	".next":           "Next.js cache",
	".nuxt":           "Nuxt.js cache",
	".svelte-kit":     "SvelteKit cache",
	".turbo":          "Turborepo cache",
	"Pods":            "CocoaPods (iOS)",
	"DerivedData":     "Xcode derived data",
	".Trash":          "Trash",
	"Library/Caches":  "macOS app caches",
}

// File patterns that are often safe to clean
var CleanablePatterns = []string{
	"*.log",
	"*.tmp",
	"*.temp",
	"*.bak",
	"*.swp",
	"*.DS_Store",
	"Thumbs.db",
	"*.pyc",
	"*.pyo",
}

// Progress holds current scan progress info
type Progress struct {
	CurrentDir   string
	FilesScanned int
	DirsScanned  int
	BytesScanned int64
	Elapsed      time.Duration
}

// ProgressFunc is called periodically during scanning
type ProgressFunc func(Progress)

type Scanner struct {
	RootPath     string
	MinSize      int64 // Minimum file size to report
	MaxDepth     int   // Maximum directory depth (-1 for unlimited)
	SkipHidden   bool
	FollowLinks  bool
	OnProgress   ProgressFunc // Called during scan with progress updates
	mu           sync.Mutex
	errors       []string
}

func New(rootPath string) *Scanner {
	return &Scanner{
		RootPath:    rootPath,
		MinSize:     0,
		MaxDepth:    -1,
		SkipHidden:  false,
		FollowLinks: false,
	}
}

func (s *Scanner) Scan() (*ScanResult, error) {
	start := time.Now()
	result := &ScanResult{}

	root, err := filepath.Abs(s.RootPath)
	if err != nil {
		return nil, err
	}

	var lastProgress time.Time
	var currentDir string

	err = filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			s.mu.Lock()
			s.errors = append(s.errors, path+": "+err.Error())
			s.mu.Unlock()
			return nil // Continue walking
		}

		// Skip hidden files if configured
		name := info.Name()
		if s.SkipHidden && strings.HasPrefix(name, ".") && path != root {
			if info.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}

		// Check depth
		if s.MaxDepth >= 0 {
			relPath, _ := filepath.Rel(root, path)
			depth := strings.Count(relPath, string(os.PathSeparator))
			if depth > s.MaxDepth {
				if info.IsDir() {
					return filepath.SkipDir
				}
				return nil
			}
		}

		fileInfo := FileInfo{
			Path:    path,
			Size:    info.Size(),
			ModTime: info.ModTime(),
			IsDir:   info.IsDir(),
		}

		if info.IsDir() {
			result.TotalDirs++
			currentDir = path
		} else {
			result.TotalFiles++
			result.TotalSize += info.Size()
		}

		// Report progress every 100ms
		if s.OnProgress != nil && time.Since(lastProgress) > 100*time.Millisecond {
			lastProgress = time.Now()
			s.OnProgress(Progress{
				CurrentDir:   currentDir,
				FilesScanned: result.TotalFiles,
				DirsScanned:  result.TotalDirs,
				BytesScanned: result.TotalSize,
				Elapsed:      time.Since(start),
			})
		}

		// Only add files above min size, or all directories
		if info.IsDir() || info.Size() >= s.MinSize {
			result.Files = append(result.Files, fileInfo)
		}

		return nil
	})

	result.ScanTime = time.Since(start)
	result.Errors = s.errors

	return result, err
}

// IsCacheDir checks if a directory name is a known cache directory
func IsCacheDir(name string) (bool, string) {
	if desc, ok := CacheDirs[name]; ok {
		return true, desc
	}
	return false, ""
}

// GetDirSize calculates the total size of a directory
func GetDirSize(path string) (int64, error) {
	var size int64
	err := filepath.Walk(path, func(_ string, info os.FileInfo, err error) error {
		if err != nil {
			return nil
		}
		if !info.IsDir() {
			size += info.Size()
		}
		return nil
	})
	return size, err
}
