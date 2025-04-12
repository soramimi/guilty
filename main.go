package main

import (
	"encoding/json"
	"fmt"
	"html/template"
	"io/fs"
	"log"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"
)

// GitRepositoryRoot はGitリポジトリのルートディレクトリを定義します
const GitRepositoryRoot = "/mnt/git"

// GitHostName はGitリポジトリのホスト名を定義します（git clone用）
var GitHostName = "localhost" // デフォルト値を設定

// GitCloneURLTemplate はクローンURLのテンプレートを定義します
const GitCloneURLTemplate = "git@%s:git/%s.git"

type PageData struct {
	Title        string
	Message      string
	HostName     string
	BuildVersion string // キャッシュ回避用のビルドバージョン
}

type GitRepository struct {
	Path       string      `json:"path"`
	Name       string      `json:"name"`
	Type       string      `json:"type"`
	CloneURL   string      `json:"cloneUrl"` // クローン用URLを追加
	LastCommit *CommitInfo `json:"lastCommit"`
}

type CommitInfo struct {
	Author  string    `json:"author"`
	Date    time.Time `json:"date"`
	Message string    `json:"message"`
}

// GitFile はリポジトリ内のファイル/ディレクトリを表す
type GitFile struct {
	Name         string    `json:"name"`
	Path         string    `json:"path"`
	Type         string    `json:"type"` // "file" または "dir"
	Size         int64     `json:"size"`
	LastModified time.Time `json:"lastModified"`
}

// RepositoryDetails はリポジトリの詳細情報を含む
type RepositoryDetails struct {
	Repository GitRepository `json:"repository"`
	Files      []GitFile     `json:"files"`
	Branches   []string      `json:"branches"`
	Tags       []string      `json:"tags"`
}

// リポジトリ作成リクエスト用の構造体
type CreateRepositoryRequest struct {
	Name string `json:"name"`
}

func main() {
	// 静的ファイルのルーティング
	fs := http.FileServer(http.Dir("static"))
	http.Handle("/static/", http.StripPrefix("/static/", fs))

	// ホームページのルーティング
	http.HandleFunc("/", homeHandler)

	// Gitリポジトリ一覧API
	http.HandleFunc("/api/repositories", repositoriesHandler)

	// リポジトリ詳細API
	http.HandleFunc("/api/repository/", repositoryDetailsHandler)

	// ディレクトリ内容取得API
	http.HandleFunc("/api/directory/", directoryContentsHandler)

	// ファイル内容取得API
	http.HandleFunc("/api/file/", fileContentsHandler)

	// リポジトリ詳細ページのルーティング
	http.HandleFunc("/repository/", repositoryPageHandler)

	// 新規リポジトリ作成ページのルーティング
	http.HandleFunc("/create-repository", createRepositoryPageHandler)

	// サーバー起動
	fmt.Println("サーバーを起動しています。http://localhost:8000 にアクセスしてください")
	log.Fatal(http.ListenAndServe(":8000", nil))
}

func homeHandler(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/" {
		http.NotFound(w, r)
		return
	}

	// ホームページのデータを準備
	data := PageData{
		Title:        "Git リポジトリ一覧",
		Message:      GitRepositoryRoot + " にあるGitリポジトリ一覧",
		HostName:     GitHostName,
		BuildVersion: fmt.Sprintf("%d", time.Now().Unix()), // Unixタイムスタンプをバージョンとして使用
	}

	// テンプレートを解析
	tmpl, err := template.ParseFiles("templates/index.html")
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	// テンプレートを実行
	err = tmpl.Execute(w, data)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}

func repositoryPageHandler(w http.ResponseWriter, r *http.Request) {
	// リポジトリ名をURLから取得（/repository/以降の部分）
	repoPath := strings.TrimPrefix(r.URL.Path, "/repository/")

	// ページデータの準備
	data := PageData{
		Title:        "リポジトリ詳細",
		Message:      "リポジトリ: " + repoPath,
		HostName:     GitHostName,
		BuildVersion: fmt.Sprintf("%d", time.Now().Unix()), // Unixタイムスタンプをバージョンとして使用
	}

	// テンプレートを解析
	tmpl, err := template.ParseFiles("templates/repository.html")
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	// テンプレートを実行
	err = tmpl.Execute(w, data)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}

// createRepositoryPageHandler はリポジトリ作成ページを表示するハンドラー
func createRepositoryPageHandler(w http.ResponseWriter, r *http.Request) {
	// ページデータの準備
	data := PageData{
		Title:        "新規リポジトリの作成",
		Message:      "新しいGitリポジトリを作成します",
		HostName:     GitHostName,
		BuildVersion: fmt.Sprintf("%d", time.Now().Unix()), // Unixタイムスタンプをバージョンとして使用
	}

	// テンプレートを解析
	tmpl, err := template.ParseFiles("templates/create-repository.html")
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	// テンプレートを実行
	err = tmpl.Execute(w, data)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}

func repositoriesHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	// CORSのためのヘッダーを追加
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.Header().Set("Access-Control-Allow-Methods", "GET, POST, OPTIONS")
	w.Header().Set("Access-Control-Allow-Headers", "Content-Type")

	// OPTIONSリクエスト（プリフライト）に対する応答
	if r.Method == http.MethodOptions {
		w.WriteHeader(http.StatusOK)
		return
	}

	// POSTリクエストの場合は新しいリポジトリを作成
	if r.Method == http.MethodPost {
		var req CreateRepositoryRequest

		// リクエストボディの解析
		err := json.NewDecoder(r.Body).Decode(&req)
		if err != nil {
			w.WriteHeader(http.StatusBadRequest)
			json.NewEncoder(w).Encode(map[string]string{"error": "無効なリクエスト形式です"})
			return
		}

		// リポジトリ名のバリデーション
		if err := validateRepositoryName(req.Name); err != nil {
			w.WriteHeader(http.StatusBadRequest)
			json.NewEncoder(w).Encode(map[string]string{"error": err.Error()})
			return
		}

		// リポジトリの作成
		err = createRepository(req.Name)
		if err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			json.NewEncoder(w).Encode(map[string]string{"error": err.Error()})
			return
		}

		// 成功レスポンス
		w.WriteHeader(http.StatusCreated)
		json.NewEncoder(w).Encode(map[string]string{"message": "リポジトリが作成されました"})
		return
	}

	// GETリクエストの場合はリポジトリ一覧を返す
	if r.Method == http.MethodGet {
		// Gitリポジトリを取得
		repos, err := getGitRepositories()
		if err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			json.NewEncoder(w).Encode(map[string]string{"error": err.Error()})
			return
		}

		// 結果をJSONとして返す
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(repos)
		return
	}

	// 未対応のHTTPメソッド
	w.WriteHeader(http.StatusMethodNotAllowed)
	json.NewEncoder(w).Encode(map[string]string{"error": "サポートされていないメソッドです"})
}

func repositoryDetailsHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.Header().Set("Access-Control-Allow-Methods", "GET, POST, OPTIONS")
	w.Header().Set("Access-Control-Allow-Headers", "Content-Type")

	// OPTIONSリクエスト（プリフライト）に対する応答
	if r.Method == http.MethodOptions {
		w.WriteHeader(http.StatusOK)
		return
	}

	// リポジトリパスを取得（/api/repository/以降の部分）
	encodedPath := strings.TrimPrefix(r.URL.Path, "/api/repository/")
	// URLエンコードされたパスをデコード
	decodedPath, err := url.PathUnescape(encodedPath)
	if err != nil {
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]string{"error": "無効なリポジトリパス"})
		return
	}

	// POSTリクエストの場合はリポジトリを削除する (DELETEからPOSTに変更)
	if r.Method == http.MethodPost {
		// リクエストボディから操作タイプを取得
		var requestBody map[string]string
		if err := json.NewDecoder(r.Body).Decode(&requestBody); err != nil {
			w.WriteHeader(http.StatusBadRequest)
			json.NewEncoder(w).Encode(map[string]string{"error": "不正なリクエスト形式"})
			return
		}
		
		// 操作タイプが "delete" の場合のみ削除を実行
		if requestBody["operation"] != "delete" {
			w.WriteHeader(http.StatusBadRequest)
			json.NewEncoder(w).Encode(map[string]string{"error": "不正な操作タイプ"})
			return
		}
		
		repoName := decodedPath
		err := deleteRepository(repoName)
		if err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			json.NewEncoder(w).Encode(map[string]string{"error": err.Error()})
			return
		}

		// 成功レスポンス
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(map[string]string{"message": "リポジトリが削除されました"})
		return
	}

	// GETリクエストの場合はリポジトリの詳細を返す
	if r.Method == http.MethodGet {
		repoPath, err := filepath.Abs(filepath.Join(GitRepositoryRoot, decodedPath))
		if err != nil {
			w.WriteHeader(http.StatusBadRequest)
			json.NewEncoder(w).Encode(map[string]string{"error": "無効なリポジトリパス"})
			return
		}

		// リポジトリの存在確認
		if _, err := os.Stat(repoPath); os.IsNotExist(err) {
			w.WriteHeader(http.StatusNotFound)
			json.NewEncoder(w).Encode(map[string]string{"error": "リポジトリが見つかりません"})
			return
		}

		// リポジトリのタイプを判定（通常 or ベア）
		isNormal := false
		isBare := false

		// 通常リポジトリのチェック
		if _, err := os.Stat(filepath.Join(repoPath, ".git")); err == nil {
			isNormal = true
		}

		// ベアリポジトリのチェック
		if _, err := os.Stat(filepath.Join(repoPath, "HEAD")); err == nil && !isNormal {
			isBare = true
		}

		if !isNormal && !isBare {
			w.WriteHeader(http.StatusBadRequest)
			json.NewEncoder(w).Encode(map[string]string{"error": "Gitリポジトリではありません"})
			return
		}

		// リポジトリ情報を取得
		repoName := filepath.Base(repoPath)
		// .git拡張子を削除（ベアリポジトリの場合）
		if strings.HasSuffix(repoName, ".git") {
			repoName = strings.TrimSuffix(repoName, ".git")
		}

		repo := GitRepository{
			Path: repoPath,
			Name: repoName,
			Type: "normal",
			// クローンURLを生成
			CloneURL: fmt.Sprintf(GitCloneURLTemplate, GitHostName, repoName),
		}

		if isBare {
			repo.Type = "bare"
		}

		// 最新のコミット情報を取得
		repo.LastCommit = getLastCommit(repoPath, isBare)

		// ファイル一覧を取得
		files, err := getRepositoryFiles(repoPath, isBare)
		if err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			json.NewEncoder(w).Encode(map[string]string{"error": "ファイル一覧の取得に失敗しました: " + err.Error()})
			return
		}

		// ブランチリストを取得
		branches, err := getRepositoryBranches(repoPath, isBare)
		if err != nil {
			branches = []string{}
		}

		// タグリストを取得
		tags, err := getRepositoryTags(repoPath, isBare)
		if err != nil {
			tags = []string{}
		}

		// リポジトリ詳細を組み立て
		details := RepositoryDetails{
			Repository: repo,
			Files:      files,
			Branches:   branches,
			Tags:       tags,
		}

		// 結果をJSONとして返す
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(details)
		return
	}

	// 未対応のHTTPメソッド
	w.WriteHeader(http.StatusMethodNotAllowed)
	json.NewEncoder(w).Encode(map[string]string{"error": "サポートされていないメソッドです"})
}

func getGitRepositories() ([]GitRepository, error) {
	gitDir := GitRepositoryRoot
	var repositories []GitRepository

	// ディレクトリ内の項目を確認
	err := filepath.WalkDir(gitDir, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			// エラーが発生しても処理を続行
			// この変更により、権限のないディレクトリがあってもスキップして続行する
			return nil
		}

		// ルートディレクトリはスキップ
		if path == gitDir {
			return nil
		}

		// ディレクトリのみ処理
		if !d.IsDir() {
			return nil
		}

		// ディレクトリの読み取り権限をチェック
		info, err := d.Info()
		if err != nil {
			// 情報が取得できない場合はスキップ（権限がない可能性）
			return nil
		}

		// 読み取り権限がない場合はスキップ
		if info.Mode().Perm()&0444 == 0 {
			return filepath.SkipDir
		}

		// ベアリポジトリのみを想定
		headPath := filepath.Join(path, "HEAD")
		_, err = os.Stat(headPath)
		if err == nil {
			// ベアリポジトリ
			repoName := filepath.Base(path)
			// .git拡張子を削除
			if strings.HasSuffix(repoName, ".git") {
				repoName = strings.TrimSuffix(repoName, ".git")
			}

			repo := GitRepository{
				Path: path,
				Name: repoName,
				Type: "bare",
				// クローンURLを生成
				CloneURL: fmt.Sprintf(GitCloneURLTemplate, GitHostName, repoName),
			}

			// 最新のコミット情報を取得
			repo.LastCommit = getLastCommit(path, true)
			repositories = append(repositories, repo)

			// サブディレクトリのスキャンをスキップ
			return filepath.SkipDir
		}

		return nil
	})

	// 処理中にエラーが発生してもリポジトリ一覧を返す
	// （読み取り権限のないディレクトリがあってもアクセス可能なリポジトリは一覧表示する）
	if err != nil && len(repositories) == 0 {
		return nil, err
	}

	// 最終コミット日時の降順でソート（新しい順）
	sort.Slice(repositories, func(i, j int) bool {
		// コミット情報がない場合は最後に表示
		if repositories[i].LastCommit == nil {
			return false
		}
		if repositories[j].LastCommit == nil {
			return true
		}
		// 日時を比較して降順にソート
		return repositories[i].LastCommit.Date.After(repositories[j].LastCommit.Date)
	})

	return repositories, nil
}

func getLastCommit(repoPath string, isBare bool) *CommitInfo {
	var cmd *exec.Cmd

	if isBare {
		cmd = exec.Command("git", "--git-dir="+repoPath, "log", "-1", "--format=%an|%at|%s")
	} else {
		cmd = exec.Command("git", "-C", repoPath, "log", "-1", "--format=%an|%at|%s")
	}

	output, err := cmd.Output()
	if err != nil {
		return nil
	}

	parts := strings.Split(strings.TrimSpace(string(output)), "|")
	if len(parts) != 3 {
		return nil
	}

	timestamp := parts[1]
	unixTime, err := strconv.ParseInt(timestamp, 10, 64)
	if err != nil {
		return nil
	}

	return &CommitInfo{
		Author:  parts[0],
		Date:    time.Unix(unixTime, 0),
		Message: parts[2],
	}
}

// hasCommits はリポジトリにコミットが1件以上あるか確認する
func hasCommits(repoPath string, isBare bool) bool {
	var cmd *exec.Cmd

	if isBare {
		cmd = exec.Command("git", "--git-dir="+repoPath, "rev-list", "--count", "HEAD")
	} else {
		cmd = exec.Command("git", "-C", repoPath, "rev-list", "--count", "HEAD")
	}

	output, err := cmd.Output()
	if err != nil {
		// エラーが発生した場合はコミットなしとみなす
		return false
	}

	// 出力を整数に変換
	count, err := strconv.ParseInt(strings.TrimSpace(string(output)), 10, 64)
	if err != nil {
		return false
	}

	return count > 0
}

// リポジトリ内のファイル一覧を取得（ルートディレクトリの1階層のみ）
func getRepositoryFiles(repoPath string, isBare bool) ([]GitFile, error) {
	// コミットが存在しない場合は特別な処理
	if !hasCommits(repoPath, isBare) {
		// コミットがない場合は、空の配列を返す
		// フロントエンド側で適切に表示する
		return []GitFile{}, nil
	}

	var files []GitFile
	var cmd *exec.Cmd

	if isBare {
		cmd = exec.Command("git", "--git-dir="+repoPath, "ls-tree", "HEAD")
	} else {
		cmd = exec.Command("git", "-C", repoPath, "ls-tree", "HEAD")
	}

	output, err := cmd.Output()
	if err != nil {
		// git ls-tree が失敗した場合でも、コミットがないという確認は済んでいるので
		// 空の配列を返す
		return []GitFile{}, nil
	}

	// git ls-tree の出力を解析
	// 各行の形式: <mode> <type> <object> <file>
	lines := strings.Split(strings.TrimSpace(string(output)), "\n")
	for _, line := range lines {
		if line == "" {
			continue
		}

		parts := strings.Fields(line)
		if len(parts) < 4 {
			continue
		}

		fileType := "file"
		if parts[1] == "tree" {
			fileType = "dir"
		}

		// ファイル名を取得（最後のフィールド、複数単語の場合もある）
		fileName := strings.Join(parts[3:], " ")

		var fileSize int64 = 0
		if fileType == "file" {
			// ファイルサイズを取得（blob の場合のみ）
			if isBare {
				fileSize = getGitObjectSize(repoPath, parts[2], true)
			} else {
				fileSize = getGitObjectSize(repoPath, parts[2], false)
			}
		}

		files = append(files, GitFile{
			Name:         fileName,
			Path:         fileName,
			Type:         fileType,
			Size:         fileSize,
			LastModified: getFileLastModified(repoPath, fileName, isBare),
		})
	}

	// ファイル一覧をソート
	// 1. ディレクトリを先に
	// 2. 大文字小文字を区別せずに名前順に
	sort.Slice(files, func(i, j int) bool {
		// タイプが異なる場合はディレクトリが先
		if files[i].Type != files[j].Type {
			return files[i].Type == "dir"
		}
		// タイプが同じ場合は名前の昇順（大文字小文字区別なし）
		return strings.ToLower(files[i].Name) < strings.ToLower(files[j].Name)
	})

	return files, nil
}

// ファイルシステムから直接ファイル一覧を取得（git ls-tree が使えない場合のフォールバック）
func getDirectoryFilesFromFilesystem(dirPath string) ([]GitFile, error) {
	var files []GitFile

	entries, err := os.ReadDir(dirPath)
	if err != nil {
		return nil, err
	}

	for _, entry := range entries {
		// .gitディレクトリはスキップ
		if entry.Name() == ".git" {
			continue
		}

		info, err := entry.Info()
		if err != nil {
			continue
		}

		fileType := "file"
		if entry.IsDir() {
			fileType = "dir"
		}

		files = append(files, GitFile{
			Name:         entry.Name(),
			Path:         entry.Name(),
			Type:         fileType,
			Size:         info.Size(),
			LastModified: info.ModTime(),
		})
	}

	// ファイル一覧をソート
	sort.Slice(files, func(i, j int) bool {
		// タイプが異なる場合はディレクトリが先
		if files[i].Type != files[j].Type {
			return files[i].Type == "dir"
		}
		// タイプが同じ場合は名前の昇順（大文字小文字区別なし）
		return strings.ToLower(files[i].Name) < strings.ToLower(files[j].Name)
	})

	return files, nil
}

// Gitオブジェクトのサイズを取得
func getGitObjectSize(repoPath, objectHash string, isBare bool) int64 {
	var cmd *exec.Cmd

	if isBare {
		cmd = exec.Command("git", "--git-dir="+repoPath, "cat-file", "-s", objectHash)
	} else {
		cmd = exec.Command("git", "-C", repoPath, "cat-file", "-s", objectHash)
	}

	output, err := cmd.Output()
	if err != nil {
		return 0
	}

	// 出力を整数に変換
	size, err := strconv.ParseInt(strings.TrimSpace(string(output)), 10, 64)
	if err != nil {
		return 0
	}

	return size
}

// リポジトリのブランチ一覧を取得
func getRepositoryBranches(repoPath string, isBare bool) ([]string, error) {
	var cmd *exec.Cmd

	if isBare {
		cmd = exec.Command("git", "--git-dir="+repoPath, "branch", "--list")
	} else {
		cmd = exec.Command("git", "-C", repoPath, "branch", "--list")
	}

	output, err := cmd.Output()
	if err != nil {
		return nil, err
	}

	var branches []string
	lines := strings.Split(strings.TrimSpace(string(output)), "\n")
	for _, line := range lines {
		if line == "" {
			continue
		}

		// '*'で始まる場合は現在のブランチ
		branch := strings.TrimSpace(line)
		if strings.HasPrefix(branch, "* ") {
			branch = strings.TrimPrefix(branch, "* ")
		}

		branches = append(branches, branch)
	}

	return branches, nil
}

// リポジトリのタグ一覧を取得
func getRepositoryTags(repoPath string, isBare bool) ([]string, error) {
	var cmd *exec.Cmd

	if isBare {
		cmd = exec.Command("git", "--git-dir="+repoPath, "tag", "--list")
	} else {
		cmd = exec.Command("git", "-C", repoPath, "tag", "--list")
	}

	output, err := cmd.Output()
	if err != nil {
		return nil, err
	}

	var tags []string
	lines := strings.Split(strings.TrimSpace(string(output)), "\n")
	for _, line := range lines {
		if line == "" {
			continue
		}

		tags = append(tags, strings.TrimSpace(line))
	}

	return tags, nil
}

// directoryContentsHandler はリポジトリ内の特定のディレクトリの内容を返す
func directoryContentsHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.Header().Set("Access-Control-Allow-Methods", "GET")

	// URLからパラメータを取得
	encodedRepoPath := strings.TrimPrefix(r.URL.Path, "/api/directory/")
	repoPathParts := strings.SplitN(encodedRepoPath, "/", 2)

	if len(repoPathParts) == 0 {
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]string{"error": "無効なリポジトリパス"})
		return
	}

	// リポジトリパスの解決
	repoName, err := url.PathUnescape(repoPathParts[0])
	if err != nil {
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]string{"error": "無効なリポジトリパス"})
		return
	}

	repoPath := filepath.Join(GitRepositoryRoot, repoName)

	// リポジトリの存在確認
	if _, err := os.Stat(repoPath); os.IsNotExist(err) {
		w.WriteHeader(http.StatusNotFound)
		json.NewEncoder(w).Encode(map[string]string{"error": "リポジトリが見つかりません"})
		return
	}

	// リポジトリのタイプを判定
	isNormal := false
	isBare := false

	// 通常リポジトリのチェック
	if _, err := os.Stat(filepath.Join(repoPath, ".git")); err == nil {
		isNormal = true
	}

	// ベアリポジトリのチェック
	if _, err := os.Stat(filepath.Join(repoPath, "HEAD")); err == nil && !isNormal {
		isBare = true
	}

	// ディレクトリパスの解決
	var dirPath string
	if len(repoPathParts) > 1 {
		dirPath, err = url.PathUnescape(repoPathParts[1])
		if err != nil {
			w.WriteHeader(http.StatusBadRequest)
			json.NewEncoder(w).Encode(map[string]string{"error": "無効なディレクトリパス"})
			return
		}
	} else {
		dirPath = ""
	}

	// ベアリポジトリの場合は、特別な処理
	if isBare && dirPath == "" {
		// ベアリポジトリのルートディレクトリは既に処理済み
		files, err := getRepositoryFiles(repoPath, true)
		if err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			json.NewEncoder(w).Encode(map[string]string{"error": "ディレクトリ内容の取得に失敗しました: " + err.Error()})
			return
		}

		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(files)
		return
	}

	// ディレクトリパスの作成
	fullDirPath := filepath.Join(repoPath, dirPath)

	// パスがリポジトリの外に出ていないか確認（パス走査攻撃の防止）
	absRepoPath, _ := filepath.Abs(repoPath)
	absDirPath, _ := filepath.Abs(fullDirPath)
	if !strings.HasPrefix(absDirPath, absRepoPath) {
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]string{"error": "無効なディレクトリパス"})
		return
	}

	// ディレクトリの内容を取得（git ls-treeを使用）
	files, err := getDirectoryContents(repoPath, dirPath, isNormal, isBare)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(map[string]string{"error": "ディレクトリ内容の取得に失敗しました: " + err.Error()})
		return
	}

	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(files)
}

// 特定のディレクトリ内のファイル一覧を取得する
func getDirectoryContents(repoPath, dirPath string, isNormal, isBare bool) ([]GitFile, error) {
	var files []GitFile
	var cmd *exec.Cmd

	if isBare {
		cmd = exec.Command("git", "--git-dir="+repoPath, "ls-tree", "HEAD:"+dirPath)
	} else {
		cmd = exec.Command("git", "-C", repoPath, "ls-tree", "HEAD:"+dirPath)
	}

	output, err := cmd.Output()
	if err != nil {
		return nil, err
	}

	// git ls-tree の出力を解析
	// 各行の形式: <mode> <type> <object> <file>
	lines := strings.Split(strings.TrimSpace(string(output)), "\n")
	for _, line := range lines {
		if line == "" {
			continue
		}

		parts := strings.Fields(line)
		if len(parts) < 4 {
			continue
		}

		fileType := "file"
		if parts[1] == "tree" {
			fileType = "dir"
		}

		// ファイル名を取得（最後のフィールド、複数単語の場合もある）
		fileName := strings.Join(parts[3:], " ")

		var fileSize int64 = 0
		if fileType == "file" {
			// ファイルサイズを取得（blob の場合のみ）
			if isBare {
				fileSize = getGitObjectSize(repoPath, parts[2], true)
			} else {
				fileSize = getGitObjectSize(repoPath, parts[2], false)
			}
		}

		files = append(files, GitFile{
			Name:         fileName,
			Path:         filepath.Join(dirPath, fileName),
			Type:         fileType,
			Size:         fileSize,
			LastModified: getFileLastModified(repoPath, filepath.Join(dirPath, fileName), isBare),
		})
	}

	// ファイル一覧をソート
	// 1. ディレクトリを先に
	// 2. 大文字小文字を区別せずに名前順に
	sort.Slice(files, func(i, j int) bool {
		// タイプが異なる場合はディレクトリが先
		if files[i].Type != files[j].Type {
			return files[i].Type == "dir"
		}
		// タイプが同じ場合は名前の昇順（大文字小文字区別なし）
		return strings.ToLower(files[i].Name) < strings.ToLower(files[j].Name)
	})

	return files, nil
}

// fileContentsHandler はGitリポジトリ内のファイル内容を返す
func fileContentsHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.Header().Set("Access-Control-Allow-Methods", "GET")

	// URLからパラメータを取得
	encodedPath := strings.TrimPrefix(r.URL.Path, "/api/file/")
	pathParts := strings.SplitN(encodedPath, "/", 2)

	if len(pathParts) < 2 {
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]string{"error": "無効なファイルパス"})
		return
	}

	// リポジトリ名とファイルパスの解決
	repoName, err := url.PathUnescape(pathParts[0])
	if err != nil {
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]string{"error": "無効なリポジトリ名"})
		return
	}

	filePath, err := url.PathUnescape(pathParts[1])
	if err != nil {
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]string{"error": "無効なファイルパス"})
		return
	}

	// リポジトリパスの構築
	repoPath := filepath.Join(GitRepositoryRoot, repoName)

	// リポジトリの存在確認
	if _, err := os.Stat(repoPath); os.IsNotExist(err) {
		w.WriteHeader(http.StatusNotFound)
		json.NewEncoder(w).Encode(map[string]string{"error": "リポジトリが見つかりません"})
		return
	}

	// リポジトリのタイプを判定
	isNormal := false
	isBare := false

	// 通常リポジトリのチェック
	if _, err := os.Stat(filepath.Join(repoPath, ".git")); err == nil {
		isNormal = true
	}

	// ベアリポジトリのチェック
	if _, err := os.Stat(filepath.Join(repoPath, "HEAD")); err == nil && !isNormal {
		isBare = true
	}

	if !isNormal && !isBare {
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]string{"error": "Gitリポジトリではありません"})
		return
	}

	// ファイル内容の取得
	content, isBinary, err := getFileContent(repoPath, filePath, isNormal, isBare)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(map[string]string{"error": "ファイル内容の取得に失敗しました: " + err.Error()})
		return
	}

	// バイナリファイルの場合は特別な処理
	if isBinary {
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"isBinary": true,
			"content":  "",
			"message":  "バイナリファイルのため表示できません",
		})
		return
	}

	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]interface{}{
		"isBinary": false,
		"content":  content,
	})
}

// ファイル内容を取得する
func getFileContent(repoPath, filePath string, isNormal, isBare bool) (string, bool, error) {
	var cmd *exec.Cmd
	var cmdCheck *exec.Cmd

	// ファイルタイプの確認（バイナリかどうか）
	if isBare {
		cmdCheck = exec.Command("git", "--git-dir="+repoPath, "check-attr", "binary", "HEAD:"+filePath)
	} else {
		cmdCheck = exec.Command("git", "-C", repoPath, "check-attr", "binary", "--", filePath)
	}

	checkOutput, err := cmdCheck.Output()
	if err != nil {
		return "", false, err
	}

	// バイナリファイルかどうかのチェック
	isBinary := strings.Contains(string(checkOutput), "binary: set")

	// バイナリファイルの場合は空を返す
	if isBinary {
		return "", true, nil
	}

	// ファイル内容の取得
	if isBare {
		cmd = exec.Command("git", "--git-dir="+repoPath, "show", "HEAD:"+filePath)
	} else {
		cmd = exec.Command("git", "-C", repoPath, "show", "HEAD:"+filePath)
	}

	output, err := cmd.Output()
	if err != nil {
		return "", false, err
	}

	return string(output), false, nil
}

// ファイルの最終更新日時を取得する
func getFileLastModified(repoPath string, filePath string, isBare bool) time.Time {
	var cmd *exec.Cmd

	// git logコマンドでファイルの最終更新日時を取得
	if isBare {
		cmd = exec.Command("git", "--git-dir="+repoPath, "log", "-1", "--format=%at", "--", filePath)
	} else {
		cmd = exec.Command("git", "-C", repoPath, "log", "-1", "--format=%at", "--", filePath)
	}

	output, err := cmd.Output()
	if err != nil {
		// エラーの場合は現在時刻を返す
		return time.Now()
	}

	timestamp := strings.TrimSpace(string(output))
	unixTime, err := strconv.ParseInt(timestamp, 10, 64)
	if err != nil {
		// 解析エラーの場合は現在時刻を返す
		return time.Now()
	}

	return time.Unix(unixTime, 0)
}

// validateRepositoryName は新規リポジトリ名のバリデーション
func validateRepositoryName(name string) error {
	// 空のチェック
	if name == "" {
		return fmt.Errorf("リポジトリ名が指定されていません")
	}

	// 不正な文字のチェック（英数字、ハイフン、アンダースコアのみ許可）
	validName := regexp.MustCompile(`^[a-zA-Z0-9_-]+$`)
	if !validName.MatchString(name) {
		return fmt.Errorf("リポジトリ名には英数字、ハイフン、アンダースコアのみ使用できます")
	}

	// 既存のリポジトリと名前が重複していないかチェック
	repoPath := filepath.Join(GitRepositoryRoot, name+".git")
	if _, err := os.Stat(repoPath); err == nil {
		return fmt.Errorf("リポジトリ '%s' は既に存在します", name)
	}

	return nil
}

// createRepository は新規ベアリポジトリを作成する
func createRepository(name string) error {
	// リポジトリのパスを構築
	repoPath := filepath.Join(GitRepositoryRoot, name+".git")

	// ディレクトリを作成
	err := os.MkdirAll(repoPath, 0755)
	if err != nil {
		return fmt.Errorf("ディレクトリの作成に失敗しました: %w", err)
	}

	// git init --bare コマンドを実行
	cmd := exec.Command("git", "init", "--bare", repoPath)
	err = cmd.Run()
	if err != nil {
		// 失敗した場合はディレクトリを削除してクリーンアップ
		os.RemoveAll(repoPath)
		return fmt.Errorf("リポジトリの初期化に失敗しました: %w", err)
	}

	return nil
}

// deleteRepository はリポジトリを削除する（実際には名前を変更して権限を変更する）
func deleteRepository(name string) error {
	// リポジトリ名から.gitを取り除く（既に含まれている場合）
	baseName := name
	if strings.HasSuffix(baseName, ".git") {
		baseName = strings.TrimSuffix(baseName, ".git")
	}

	// リポジトリのパスを構築
	repoPath := filepath.Join(GitRepositoryRoot, baseName+".git")

	// リポジトリの存在確認
	if _, err := os.Stat(repoPath); os.IsNotExist(err) {
		return fmt.Errorf("リポジトリ '%s' は存在しません", baseName)
	}

	// 移動先のパス（.deletedを追加）
	newPath := repoPath + ".deleted"

	// 既に削除済みのリポジトリがある場合は、それを先に完全に削除
	if _, err := os.Stat(newPath); err == nil {
		err := os.RemoveAll(newPath)
		if err != nil {
			return fmt.Errorf("既存の削除済みリポジトリの削除に失敗しました: %w", err)
		}
	}

	// リポジトリの名前を変更
	err := os.Rename(repoPath, newPath)
	if err != nil {
		return fmt.Errorf("リポジトリの名前変更に失敗しました: %w", err)
	}

	// 権限を変更（読み書き禁止: chmod 000）
	err = os.Chmod(newPath, 0000)
	if err != nil {
		// 権限変更に失敗した場合でも、名前の変更は成功しているので警告だけ出して続行
		log.Printf("警告: リポジトリのアクセス権限変更に失敗しました: %v", err)
	}

	return nil
}
