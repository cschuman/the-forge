package analyzer

import (
	"crypto/md5"
	"encoding/hex"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"forge-dust/scanner"
)

type Analysis struct {
	LargeFiles      []FileReport
	OldFiles        []FileReport
	CacheDirs       []CacheReport
	DuplicateGroups []DuplicateGroup
	Downloads       []FileReport
	TotalReclaimable int64
	ScanStats       ScanStats
}

type FileReport struct {
	Path        string
	Size        int64
	ModTime     time.Time
	Age         time.Duration
	Description string
}

type CacheReport struct {
	Path        string
	Size        int64
	Type        string
	Description string
}

type DuplicateGroup struct {
	Hash  string
	Size  int64
	Files []string
}

type ScanStats struct {
	TotalFiles  int
	TotalDirs   int
	TotalSize   int64
	ScanTime    time.Duration
}

type Analyzer struct {
	MinLargeFile   int64         // Minimum size to consider "large" (default 100MB)
	OldFileAge     time.Duration // Age threshold for "old" files (default 1 year)
	DownloadsPath  string
	CheckDuplicates bool
}

func New() *Analyzer {
	home, _ := os.UserHomeDir()
	return &Analyzer{
		MinLargeFile:    100 * 1024 * 1024,  // 100MB
		OldFileAge:      365 * 24 * time.Hour, // 1 year
		DownloadsPath:   filepath.Join(home, "Downloads"),
		CheckDuplicates: false, // Disabled by default (slow)
	}
}

func (a *Analyzer) Analyze(result *scanner.ScanResult) *Analysis {
	analysis := &Analysis{
		ScanStats: ScanStats{
			TotalFiles: result.TotalFiles,
			TotalDirs:  result.TotalDirs,
			TotalSize:  result.TotalSize,
			ScanTime:   result.ScanTime,
		},
	}

	now := time.Now()

	// Maps for deduplication
	sizeMap := make(map[int64][]string) // For potential duplicates

	for _, file := range result.Files {
		// Skip directories for file analysis
		if file.IsDir {
			// Check if it's a cache directory
			name := filepath.Base(file.Path)
			if isCache, desc := scanner.IsCacheDir(name); isCache {
				size, _ := scanner.GetDirSize(file.Path)
				if size > 1024*1024 { // Only report if > 1MB
					analysis.CacheDirs = append(analysis.CacheDirs, CacheReport{
						Path:        file.Path,
						Size:        size,
						Type:        name,
						Description: desc,
					})
					analysis.TotalReclaimable += size
				}
			}
			continue
		}

		age := now.Sub(file.ModTime)

		// Large files
		if file.Size >= a.MinLargeFile {
			analysis.LargeFiles = append(analysis.LargeFiles, FileReport{
				Path:    file.Path,
				Size:    file.Size,
				ModTime: file.ModTime,
				Age:     age,
			})
		}

		// Old files (> 1 year old and > 10MB)
		if age > a.OldFileAge && file.Size > 10*1024*1024 {
			analysis.OldFiles = append(analysis.OldFiles, FileReport{
				Path:    file.Path,
				Size:    file.Size,
				ModTime: file.ModTime,
				Age:     age,
			})
		}

		// Track for duplicates
		if a.CheckDuplicates && file.Size > 1024*1024 { // Only check files > 1MB
			sizeMap[file.Size] = append(sizeMap[file.Size], file.Path)
		}

		// Downloads folder analysis
		if strings.HasPrefix(file.Path, a.DownloadsPath) && file.Size > 50*1024*1024 {
			analysis.Downloads = append(analysis.Downloads, FileReport{
				Path:    file.Path,
				Size:    file.Size,
				ModTime: file.ModTime,
				Age:     age,
			})
		}
	}

	// Find duplicates (only if enabled)
	if a.CheckDuplicates {
		analysis.DuplicateGroups = findDuplicates(sizeMap)
		for _, group := range analysis.DuplicateGroups {
			// Can reclaim all but one copy
			analysis.TotalReclaimable += group.Size * int64(len(group.Files)-1)
		}
	}

	// Add large files to reclaimable (user's choice)
	for _, f := range analysis.LargeFiles {
		analysis.TotalReclaimable += f.Size
	}

	// Sort results by size
	sort.Slice(analysis.LargeFiles, func(i, j int) bool {
		return analysis.LargeFiles[i].Size > analysis.LargeFiles[j].Size
	})
	sort.Slice(analysis.OldFiles, func(i, j int) bool {
		return analysis.OldFiles[i].Size > analysis.OldFiles[j].Size
	})
	sort.Slice(analysis.CacheDirs, func(i, j int) bool {
		return analysis.CacheDirs[i].Size > analysis.CacheDirs[j].Size
	})
	sort.Slice(analysis.Downloads, func(i, j int) bool {
		return analysis.Downloads[i].Size > analysis.Downloads[j].Size
	})

	// Limit results
	if len(analysis.LargeFiles) > 20 {
		analysis.LargeFiles = analysis.LargeFiles[:20]
	}
	if len(analysis.OldFiles) > 15 {
		analysis.OldFiles = analysis.OldFiles[:15]
	}
	if len(analysis.CacheDirs) > 15 {
		analysis.CacheDirs = analysis.CacheDirs[:15]
	}
	if len(analysis.Downloads) > 15 {
		analysis.Downloads = analysis.Downloads[:15]
	}

	return analysis
}

func findDuplicates(sizeMap map[int64][]string) []DuplicateGroup {
	var groups []DuplicateGroup

	for size, files := range sizeMap {
		if len(files) < 2 {
			continue
		}

		// Hash files with same size
		hashMap := make(map[string][]string)
		for _, path := range files {
			hash, err := hashFile(path)
			if err != nil {
				continue
			}
			hashMap[hash] = append(hashMap[hash], path)
		}

		// Find actual duplicates
		for hash, paths := range hashMap {
			if len(paths) > 1 {
				groups = append(groups, DuplicateGroup{
					Hash:  hash,
					Size:  size,
					Files: paths,
				})
			}
		}
	}

	sort.Slice(groups, func(i, j int) bool {
		return groups[i].Size*int64(len(groups[i].Files)) > groups[j].Size*int64(len(groups[j].Files))
	})

	if len(groups) > 10 {
		groups = groups[:10]
	}

	return groups
}

func hashFile(path string) (string, error) {
	file, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer file.Close()

	// Only hash first 1MB for speed
	hash := md5.New()
	if _, err := io.CopyN(hash, file, 1024*1024); err != nil && err != io.EOF {
		return "", err
	}

	return hex.EncodeToString(hash.Sum(nil)), nil
}
