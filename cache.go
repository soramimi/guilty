package main

import (
	"encoding/json"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

// CacheEntry は1件の git log 結果キャッシュを表す
type CacheEntry struct {
	Author   string    `json:"author"`
	Date     time.Time `json:"date"`
	Message  string    `json:"message"`
	CachedAt int64     `json:"cachedAt"`
}

// GroupCache は1グループ分のキャッシュを表す
type GroupCache struct {
	Entries map[string]CacheEntry `json:"entries"`
}

// groupCacheState は1グループのキャッシュ状態を排他制御付きで保持する
type groupCacheState struct {
	mu     sync.RWMutex
	cache  *GroupCache
	dirty  bool
}

var (
	cacheStatesMu sync.Mutex
	cacheStates   = make(map[string]*groupCacheState)
)

// cacheFilePath はグループに対応するキャッシュファイルパスを返す
func cacheFilePath(groupName string) string {
	return filepath.Join(GitRepositoryHome, groupName, ".guilty-cache.json")
}

// getCacheState はグループのキャッシュ状態を返す（存在しなければ初期化）
func getCacheState(groupName string) *groupCacheState {
	cacheStatesMu.Lock()
	defer cacheStatesMu.Unlock()
	if state, ok := cacheStates[groupName]; ok {
		return state
	}
	state := &groupCacheState{
		cache: &GroupCache{Entries: make(map[string]CacheEntry)},
	}
	cacheStates[groupName] = state
	return state
}

// loadCacheFromDisk はキャッシュファイルをディスクから読み込む
func loadCacheFromDisk(groupName string) *GroupCache {
	cache := &GroupCache{Entries: make(map[string]CacheEntry)}
	path := cacheFilePath(groupName)

	data, err := os.ReadFile(path)
	if err != nil {
		if !os.IsNotExist(err) {
			log.Printf("キャッシュファイルの読み込みに失敗しました (%s): %v", path, err)
		}
		return cache
	}

	if err := json.Unmarshal(data, cache); err != nil {
		log.Printf("キャッシュファイルの解析に失敗しました (%s): %v", path, err)
		cache.Entries = make(map[string]CacheEntry)
		return cache
	}

	if cache.Entries == nil {
		cache.Entries = make(map[string]CacheEntry)
	}

	return cache
}

// saveGroupCache はメモリ上のキャッシュを JSON ファイルに書き出す
func saveGroupCache(groupName string) {
	state := getCacheState(groupName)
	state.mu.Lock()
	defer state.mu.Unlock()

	if !state.dirty {
		return
	}

	path := cacheFilePath(groupName)
	dir := filepath.Dir(path)

	if err := os.MkdirAll(dir, 0755); err != nil {
		log.Printf("キャッシュディレクトリの作成に失敗しました (%s): %v", dir, err)
		return
	}

	data, err := json.MarshalIndent(state.cache, "", "  ")
	if err != nil {
		log.Printf("キャッシュデータのシリアライズに失敗しました (%s): %v", path, err)
		return
	}

	tmpPath := path + ".tmp"
	if err := os.WriteFile(tmpPath, data, 0644); err != nil {
		log.Printf("キャッシュファイルの書き込みに失敗しました (%s): %v", tmpPath, err)
		return
	}

	if err := os.Rename(tmpPath, path); err != nil {
		log.Printf("キャッシュファイルのリネームに失敗しました (%s -> %s): %v", tmpPath, path, err)
		_ = os.Remove(tmpPath)
		return
	}

	state.dirty = false
}

// flushScheduling によるバッチ書き込み制御用
var (
	flushMu      sync.Mutex
	flushPending = make(map[string]bool)
)

// scheduleFlush はグループのキャッシュを遅延書き込みするようスケジュールする
func scheduleFlush(groupName string) {
	flushMu.Lock()
	if flushPending[groupName] {
		flushMu.Unlock()
		return
	}
	flushPending[groupName] = true
	flushMu.Unlock()

	time.AfterFunc(100*time.Millisecond, func() {
		saveGroupCache(groupName)
		flushMu.Lock()
		delete(flushPending, groupName)
		flushMu.Unlock()
	})
}

// getHeadRef はリポジトリの HEAD ファイルの内容を返す
func getHeadRef(repoPath string) string {
	headFilePath := filepath.Join(repoPath, "HEAD")
	content, err := os.ReadFile(headFilePath)
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(content))
}

// cacheKey はリポジトリパスと HEAD 参照からキャッシュキーを生成する
func cacheKey(repoPath, headRef string) string {
	return fmt.Sprintf("%s#%s", repoPath, headRef)
}

// getCachedLastCommit は TTL 内のキャッシュがあれば CommitInfo として返す
func getCachedLastCommit(groupName, repoPath string) *CommitInfo {
	if !CacheEnabled || CacheTTLSeconds <= 0 {
		return nil
	}
	if !isValidGroupName(groupName) {
		return nil
	}

	headRef := getHeadRef(repoPath)
	state := getCacheState(groupName)

	state.mu.RLock()
	if state.cache == nil {
		state.mu.RUnlock()
		state.mu.Lock()
		if state.cache == nil {
			state.cache = loadCacheFromDisk(groupName)
		}
		state.mu.Unlock()
		state.mu.RLock()
	}
	defer state.mu.RUnlock()

	entry, ok := state.cache.Entries[cacheKey(repoPath, headRef)]
	if !ok {
		return nil
	}

	if time.Now().Unix()-entry.CachedAt > int64(CacheTTLSeconds) {
		return nil
	}

	return &CommitInfo{
		Author:  entry.Author,
		Date:    entry.Date,
		Message: entry.Message,
	}
}

// setCachedLastCommit は git log の結果をキャッシュする
func setCachedLastCommit(groupName, repoPath string, commit *CommitInfo) {
	if !CacheEnabled || CacheTTLSeconds <= 0 || commit == nil {
		return
	}
	if !isValidGroupName(groupName) {
		return
	}

	headRef := getHeadRef(repoPath)
	state := getCacheState(groupName)

	state.mu.Lock()
	if state.cache == nil {
		state.cache = loadCacheFromDisk(groupName)
	}
	state.cache.Entries[cacheKey(repoPath, headRef)] = CacheEntry{
		Author:   commit.Author,
		Date:     commit.Date,
		Message:  commit.Message,
		CachedAt: time.Now().Unix(),
	}
	state.dirty = true
	state.mu.Unlock()

	scheduleFlush(groupName)
}

// invalidateGroupCache はグループのキャッシュファイルを削除し、メモリ上のキャッシュもクリアする
func invalidateGroupCache(groupName string) {
	if !isValidGroupName(groupName) {
		return
	}

	state := getCacheState(groupName)
	state.mu.Lock()
	state.cache = &GroupCache{Entries: make(map[string]CacheEntry)}
	state.dirty = false
	state.mu.Unlock()

	path := cacheFilePath(groupName)
	if err := os.Remove(path); err != nil {
		if !os.IsNotExist(err) {
			log.Printf("キャッシュファイルの削除に失敗しました (%s): %v", path, err)
		}
	}
}
