package aider

import (
	"bufio"
	"crypto/sha256"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"

	"github.com/glory0216/taux/internal/model"
)

const historyFileName = ".aider.chat.history.md"
const maxScanDepth = 3

// timestampRe matches "# aider chat started at 2025-01-15 10:30:00"
var timestampRe = regexp.MustCompile(`^# aider chat started at (\d{4}-\d{2}-\d{2} \d{2}:\d{2}:\d{2})`)

// userMessageRe matches "#### <user message>"
var userMessageRe = regexp.MustCompile(`^####\s+(.+)`)

// ScanSession discovers aider history files and returns lightweight sessions.
func ScanSession(scanDirList []string) ([]model.Session, error) {
	fileList := discoverHistoryFileList(scanDirList)

	var sessionList []model.Session
	for _, path := range fileList {
		session, err := parseHistoryFile(path)
		if err != nil {
			continue
		}
		sessionList = append(sessionList, *session)
	}

	sort.Slice(sessionList, func(i, j int) bool {
		return sessionList[i].LastActive.After(sessionList[j].LastActive)
	})
	return sessionList, nil
}

// discoverHistoryFileList walks the scan directories looking for
// .aider.chat.history.md files with a depth limit.
func discoverHistoryFileList(scanDirList []string) []string {
	seen := make(map[string]bool)
	var result []string

	for _, baseDir := range scanDirList {
		baseDir, _ = filepath.Abs(baseDir)
		_ = filepath.WalkDir(baseDir, func(path string, d os.DirEntry, err error) error {
			if err != nil {
				return filepath.SkipDir
			}
			// Depth limit
			rel, _ := filepath.Rel(baseDir, path)
			depth := strings.Count(rel, string(filepath.Separator))
			if d.IsDir() && depth >= maxScanDepth {
				return filepath.SkipDir
			}
			// Skip hidden dirs (except current)
			if d.IsDir() && strings.HasPrefix(d.Name(), ".") && d.Name() != "." {
				return filepath.SkipDir
			}
			if d.Name() == historyFileName {
				absPath, _ := filepath.Abs(path)
				if !seen[absPath] {
					seen[absPath] = true
					result = append(result, absPath)
				}
			}
			return nil
		})
	}
	return result
}

// parseHistoryFile reads an aider chat history markdown file
// and extracts session metadata.
func parseHistoryFile(path string) (*model.Session, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	stat, err := f.Stat()
	if err != nil {
		return nil, err
	}

	var (
		firstTime    time.Time
		lastTime     time.Time
		messageCount int
		firstMsg     string
	)

	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := scanner.Text()

		// Parse timestamp headers
		if m := timestampRe.FindStringSubmatch(line); len(m) >= 2 {
			t, err := time.ParseInLocation("2006-01-02 15:04:05", m[1], time.Local)
			if err == nil {
				if firstTime.IsZero() {
					firstTime = t
				}
				lastTime = t
			}
		}

		// Count user messages
		if m := userMessageRe.FindStringSubmatch(line); len(m) >= 2 {
			messageCount++
			if firstMsg == "" {
				firstMsg = strings.TrimSpace(m[1])
				if len(firstMsg) > 80 {
					firstMsg = firstMsg[:77] + "..."
				}
			}
		}
	}

	if firstTime.IsZero() {
		// Use file modification time as fallback
		firstTime = stat.ModTime()
		lastTime = stat.ModTime()
	}
	if lastTime.IsZero() {
		lastTime = firstTime
	}

	// Generate deterministic session ID from file path
	id := sessionIDFromPath(path)
	shortID := id[:6]

	// Project info from parent directory
	dirPath := filepath.Dir(path)
	projectName := filepath.Base(dirPath)

	// Detect model from config file
	modelName := detectModel(dirPath)

	return &model.Session{
		ID:           id,
		ShortID:      shortID,
		Provider:     "aider",
		Status:       model.SessionDead, // Updated by caller with process info
		Project:      projectName,
		ProjectPath:  dirPath,
		Model:        modelName,
		Description:  firstMsg,
		Environment:  "cli",
		CWD:          dirPath,
		MessageCount: messageCount,
		StartedAt:    firstTime,
		LastActive:   lastTime,
		FilePath:     path,
		FileSize:     stat.Size(),
	}, nil
}

// sessionIDFromPath generates a deterministic ID from the file path.
func sessionIDFromPath(path string) string {
	hash := sha256.Sum256([]byte(path))
	return fmt.Sprintf("%x", hash)[:32]
}

// detectModel reads .aider.conf.yml to find the configured model.
func detectModel(dirPath string) string {
	confPath := filepath.Join(dirPath, ".aider.conf.yml")
	data, err := os.ReadFile(confPath)
	if err != nil {
		return ""
	}
	for _, line := range strings.Split(string(data), "\n") {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "model:") {
			return strings.TrimSpace(strings.TrimPrefix(line, "model:"))
		}
	}
	return ""
}
