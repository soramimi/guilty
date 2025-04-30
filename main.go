package main

import (
	"encoding/json"
	"fmt"
	"html/template"
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

const ServerPort = 1080

// GitRepositoryHome はGitリポジトリのホームディレクトリを定義します
const GitRepositoryHome= "/home/git"

// GitHostName はGitリポジトリのホスト名を定義します（git clone用）
var GitHostName = "git"

// GitCloneURLTemplate はクローンURLのテンプレートを定義します
const GitCloneURLTemplate = "git@%s:%s/%s.git"

// 除外すべきグループ名のパターンを定義
var GroupNameBlacklist = []*regexp.Regexp{
	regexp.MustCompile(`^git-shell-commands$`), // git-shell-commands を除外
}

type PageData struct {
	Title        string
	Message      string
	HostName     string
	BuildVersion string // キャッシュ回避用のビルドバージョン
}

type GitRepository struct {
	Path       string      `json:"path"`
	Group      string      `json:"group"`
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
	Name  string `json:"name"`
	Group string `json:"group"`
}

func main() {
	// 静的ファイルのルーティング
	fs := http.FileServer(http.Dir("static"))
	http.Handle("/static/", http.StripPrefix("/static/", fs))

	// ホームページのルーティング
	http.HandleFunc("/", homeHandler)

	// Gitリポジトリ一覧API
	http.HandleFunc("/api/repositories", repositoriesHandler)

	// グループ一覧API
	http.HandleFunc("/api/groups", groupsHandler)

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
	fmt.Printf("サーバーを起動しています。http://localhost:%d にアクセスしてください\n", ServerPort)
	log.Fatal(http.ListenAndServe(fmt.Sprintf(":%d", ServerPort), nil))
}

func homeHandler(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/" {
		http.NotFound(w, r)
		return
	}

	// クエリパラメータからグループ名を取得（デフォルトは"git"）
	groupName := r.URL.Query().Get("group")
	if groupName == "" {
		groupName = "git"
	}

	// ホームページのデータを準備
	data := PageData{
		Title:        "Gitリポジトリ一覧",
		Message:      groupName + " グループにあるGitリポジトリ一覧",
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
		if err := validateRepositoryName(req.Name, req.Group); err != nil {
			w.WriteHeader(http.StatusBadRequest)
			json.NewEncoder(w).Encode(map[string]string{"error": err.Error()})
			return
		}

		// リポジトリの作成
		err = createRepository(req.Name, req.Group)
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
		 // URLクエリパラメータからグループ名を取得
		groupName := r.URL.Query().Get("group")
		if groupName == "" {
			// グループ名が指定されていない場合はデフォルトの "git" を使用
			groupName = "git"
		}

		// Gitリポジトリを取得
		repos, err := getGitRepositories(groupName)
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

// groupsHandler はグループ一覧を返すハンドラー
func groupsHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.Header().Set("Access-Control-Allow-Methods", "GET")

	if r.Method != http.MethodGet {
		w.WriteHeader(http.StatusMethodNotAllowed)
		json.NewEncoder(w).Encode(map[string]string{"error": "サポートされていないメソッドです"})
		return
	}

	// グループリストを取得
	groups, err := getGroupList()
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(map[string]string{"error": "グループ一覧の取得に失敗しました: " + err.Error()})
		return
	}

	// 結果をJSONとして返す
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(groups)
}

func splitRepositoryName(path string) (group string, name string) {
	group = "git"
	name = path
	i := strings.LastIndex(path, "/")
	if i != -1 {
		group = path[:i]
		name = path[i+1:]
	}
	return group, name
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

	groupName, repoName := splitRepositoryName(decodedPath)

	// POSTリクエストの場合はリポジトリを削除する
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
		repoPath, err := filepath.Abs(filepath.Join(GitRepositoryHome, groupName, repoName) + ".git")
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

		repo := GitRepository{
			//Path: repoPath,
			Path: filepath.Join(groupName, repoName),
			Name: repoName,
			// クローンURLを生成
			CloneURL: fmt.Sprintf(GitCloneURLTemplate, GitHostName, groupName, repoName),
		}

		// 最新のコミット情報を取得
		repo.LastCommit = getLastCommit(repoPath)

		// ファイル一覧を取得
		files, err := getRepositoryFiles(repoPath)
		if err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			json.NewEncoder(w).Encode(map[string]string{"error": "ファイル一覧の取得に失敗しました: " + err.Error()})
			return
		}

		// ブランチリストを取得
		branches, err := getRepositoryBranches(repoPath)
		if err != nil {
			branches = []string{}
		}

		// タグリストを取得
		tags, err := getRepositoryTags(repoPath)
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

// getDirectories はディレクトリエントリを取得し、シンボリックリンクも解決する
// ディレクトリのみを返し、ファイルは返さない
func getDirectories(path string) ([]string, error) {
	var entries []string

	// ディレクトリエントリを取得
	dirEntries, err := os.ReadDir(path)
	if err != nil {
		return nil, err
	}

	for _, entry := range dirEntries {
		// ディレクトリでない場合はスキップ
		if !entry.IsDir() && entry.Type()&os.ModeSymlink == 0 {
			continue
		}

		// エントリのパスを構築
		entryPath := filepath.Join(path, entry.Name())

		// シンボリックリンクの場合、実際の対象を確認
		if entry.Type()&os.ModeSymlink != 0 {
			// シンボリックリンクの実際のターゲットを取得
			realPath, err := os.Readlink(entryPath)
			if err != nil {
				// シンボリックリンクが読めない場合はスキップ
				continue
			}

			// 相対パスの場合は絶対パスに変換
			if !filepath.IsAbs(realPath) {
				realPath = filepath.Join(path, realPath)
			}

			// リンク先の情報を取得
			info, err := os.Stat(realPath)
			if err != nil {
				// リンク先情報が取得できない場合はスキップ
				continue
			}

			// リンク先がディレクトリの場合のみ追加
			if info.IsDir() {
				// リンク先パスを使用
				//entryPath = realPath
			} else {
				// ディレクトリでない場合はスキップ
				continue
			}
		}

		entries = append(entries, entryPath)
	}

	return entries, nil
}

func getGitRepositories(groupName string) ([]GitRepository, error) {
	if groupName == "" {
		return nil, fmt.Errorf("グループ名を空にすることはできません")
	}
	gitDir := filepath.Join(GitRepositoryHome, groupName)
	var repositories []GitRepository

	// ディレクトリエントリを取得
	entries, err := getDirectories(gitDir)
	if err != nil {
		return nil, err
	}

	for _, path := range entries {
		// ファイル情報を取得してアクセス権を確認
		info, err := os.Stat(path)
		if err != nil {
			// 情報が取得できない場合はスキップ
			continue
		}

		// 読み取り権限がない場合はスキップ
		if info.Mode().Perm()&0444 == 0 {
			continue
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
				Group: groupName, // 選択されたグループ名を使用
				Name: repoName,
				Type: "bare",
				// クローンURLを生成
				CloneURL: fmt.Sprintf(GitCloneURLTemplate, GitHostName, groupName, repoName),
			}

			// 最新のコミット情報を取得
			repo.LastCommit = getLastCommit(path)
			repositories = append(repositories, repo)
		}
	}

	// リポジトリが見つからなかった場合
	if len(repositories) == 0 {
		// エラーがある場合だけエラーを返す
		// エラーがなければ空配列を返す
		if err != nil {
			return nil, err
		}
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

// グループ名が有効かどうかをチェックする関数
func isValidGroupName(name string) bool {
	// 不正な文字のチェック（英数字、ハイフン、アンダースコアのみ許可）
	validName := regexp.MustCompile(`^[a-zA-Z0-9_-]+$`)
	if !validName.MatchString(name) {
		return false
	}

	// ブラックリストに一致するものは除外
	for _, pattern := range GroupNameBlacklist {
		if pattern.MatchString(name) {
			return false
		}
	}

	return true
}

// getGroupList はGitRepositoryHome内のサブディレクトリ（グループ）をスキャンします
func getGroupList() ([]string, error) {
	var groups []string

	// getDirectories関数を使用してGitRepositoryHome内のディレクトリを取得
	entries, err := getDirectories(GitRepositoryHome)
	if err != nil {
		return nil, fmt.Errorf("GitRepositoryHomeのディレクトリ読み取りに失敗しました: %w", err)
	}

	// 常に'git'グループはデフォルトとして含める
	hasGitGroup := false

	for _, entryPath := range entries {
		// パスからグループ名（ディレクトリ名）を取得
		groupName := filepath.Base(entryPath)

		// グループ名のバリデーション
		if !isValidGroupName(groupName) {
			continue
		}
		
		if groupName == "git" {
			hasGitGroup = true
		}

		// 読み取り権限がないディレクトリはスキップ
		info, err := os.Stat(entryPath)
		if err != nil || info.Mode().Perm()&0444 == 0 {
			continue
		}

		groups = append(groups, groupName)
	}

	// デフォルトの'git'グループが見つからなかった場合は追加
	if !hasGitGroup {
		groups = append(groups, "git")
	}

	// グループ名をアルファベット順にソート
	sort.Strings(groups)

	return groups, nil
}

func getLastCommit(repoPath string) *CommitInfo {
	var cmd *exec.Cmd

	cmd = exec.Command("git", "--git-dir="+repoPath, "log", "-1", "--format=%an|%at|%s")

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
func hasCommits(repoPath string) bool {
	var cmd *exec.Cmd

	cmd = exec.Command("git", "--git-dir="+repoPath, "rev-list", "--count", "HEAD")

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
func getRepositoryFiles(repoPath string) ([]GitFile, error) {
	// コミットが存在しない場合は特別な処理
	if !hasCommits(repoPath) {
		// コミットがない場合は、空の配列を返す
		// フロントエンド側で適切に表示する
		return []GitFile{}, nil
	}

	var files []GitFile
	var cmd *exec.Cmd

	cmd = exec.Command("git", "--git-dir="+repoPath, "ls-tree", "HEAD")

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
			fileSize = getGitObjectSize(repoPath, parts[2], true)
		}

		files = append(files, GitFile{
			Name:         fileName,
			Path:         fileName,
			Type:         fileType,
			Size:         fileSize,
			LastModified: getFileLastModified(repoPath, fileName),
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

// 特定のディレクトリ内のファイル一覧を取得する
func getDirectoryContents(repoPath, dirPath string) ([]GitFile, error) {
	var files []GitFile
	var cmd *exec.Cmd

	cmd = exec.Command("git", "--git-dir="+repoPath, "ls-tree", "HEAD:"+dirPath)

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
			fileSize = getGitObjectSize(repoPath, parts[2], true)
		}

		files = append(files, GitFile{
			Name:         fileName,
			Path:         filepath.Join(dirPath, fileName),
			Type:         fileType,
			Size:         fileSize,
			LastModified: getFileLastModified(repoPath, filepath.Join(dirPath, fileName)),
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
func getRepositoryBranches(repoPath string) ([]string, error) {
	var cmd *exec.Cmd

	cmd = exec.Command("git", "--git-dir="+repoPath, "branch", "--list")

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
func getRepositoryTags(repoPath string) ([]string, error) {
	var cmd *exec.Cmd

	cmd = exec.Command("git", "--git-dir="+repoPath, "tag", "--list")

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
	encodedPath := strings.TrimPrefix(r.URL.Path, "/api/directory/")
	
	// 最初の2つのスラッシュの位置を特定
	firstSlashPos := strings.Index(encodedPath, "/")
	if firstSlashPos < 0 {
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]string{"error": "無効なパス形式です（グループ名がありません）"})
		return
	}
	
	// リポジトリ名のスラッシュ位置を特定
	secondSlashPos := strings.Index(encodedPath[firstSlashPos+1:], "/")
	
	// グループ名とリポジトリ名を取得
	encodedGroupName := encodedPath[:firstSlashPos]
	var encodedRepoName, encodedDirPath string
	
	if secondSlashPos < 0 {
		// ディレクトリパスが指定されていない場合
		encodedRepoName = encodedPath[firstSlashPos+1:]
		encodedDirPath = ""
	} else {
		// ディレクトリパスが指定されている場合
		secondSlashPos += firstSlashPos + 1 // path全体の中での位置に調整
		encodedRepoName = encodedPath[firstSlashPos+1:secondSlashPos]
		encodedDirPath = encodedPath[secondSlashPos+1:]
	}
	
	// デコード
	groupName, err := url.PathUnescape(encodedGroupName)
	if err != nil {
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]string{"error": "無効なグループ名"})
		return
	}
	
	repoName, err := url.PathUnescape(encodedRepoName)
	if err != nil {
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]string{"error": "無効なリポジトリ名"})
		return
	}
	
	// ディレクトリパス部分のデコード - %2Fもデコードされるように
	var dirPath string
	if encodedDirPath != "" {
		dirPath, err = url.PathUnescape(encodedDirPath)
		if err != nil {
			w.WriteHeader(http.StatusBadRequest)
			json.NewEncoder(w).Encode(map[string]string{"error": "無効なディレクトリパス"})
			return
		}
	} else {
		dirPath = ""
	}

	// リポジトリの完全パスを構築
	fullRepoPath := filepath.Join(filepath.Join(GitRepositoryHome, groupName), repoName+".git")

	// リポジトリの存在確認
	if _, err := os.Stat(fullRepoPath); os.IsNotExist(err) {
		w.WriteHeader(http.StatusNotFound)
		json.NewEncoder(w).Encode(map[string]string{"error": "リポジトリが見つかりません"})
		return
	}

	// ベアリポジトリの場合は、特別な処理
	if dirPath == "" {
		// ベアリポジトリのルートディレクトリは既に処理済み
		files, err := getRepositoryFiles(fullRepoPath)
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
	fullDirPath := filepath.Join(fullRepoPath, dirPath)

	// パスがリポジトリの外に出ていないか確認（パス走査攻撃の防止）
	absRepoPath, _ := filepath.Abs(fullRepoPath)
	absDirPath, _ := filepath.Abs(fullDirPath)
	if !strings.HasPrefix(absDirPath, absRepoPath) {
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]string{"error": "無効なディレクトリパス"})
		return
	}

	// ディレクトリの内容を取得（git ls-treeを使用）
	files, err := getDirectoryContents(fullRepoPath, dirPath)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(map[string]string{"error": "ディレクトリ内容の取得に失敗しました: " + err.Error()})
		return
	}

	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(files)
}

// fileContentsHandler はGitリポジトリ内のファイル内容を返す
func fileContentsHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.Header().Set("Access-Control-Allow-Methods", "GET")

	// URLからパラメータを取得
	encodedPath := strings.TrimPrefix(r.URL.Path, "/api/file/")
	
	// 最初の2つのスラッシュの位置を特定
	firstSlashPos := strings.Index(encodedPath, "/")
	if firstSlashPos < 0 {
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]string{"error": "無効なパス形式です（グループ名がありません）"})
		return
	}
	
	secondSlashPos := strings.Index(encodedPath[firstSlashPos+1:], "/")
	if secondSlashPos < 0 {
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]string{"error": "無効なパス形式です（リポジトリ名がありません）"})
		return
	}
	secondSlashPos += firstSlashPos + 1 // path全体の中での位置に調整
	
	// グループ名とリポジトリ名部分を取得
	encodedGroupName := encodedPath[:firstSlashPos]
	encodedRepoName := encodedPath[firstSlashPos+1:secondSlashPos]
	encodedFilePath := encodedPath[secondSlashPos+1:]
	
	// デコード
	groupName, err := url.PathUnescape(encodedGroupName)
	if err != nil {
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]string{"error": "無効なグループ名"})
		return
	}
	
	repoName, err := url.PathUnescape(encodedRepoName)
	if err != nil {
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]string{"error": "無効なリポジトリ名"})
		return
	}
	
	// ファイルパス部分のデコード - %2Fもデコードされるように
	filePath, err := url.PathUnescape(encodedFilePath)
	if err != nil {
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]string{"error": "無効なファイルパス"})
		return
	}
	
	// リポジトリの完全パスを構築
	fullRepoPath := filepath.Join(filepath.Join(GitRepositoryHome, groupName), repoName+".git")

	// リポジトリの存在確認
	if _, err := os.Stat(fullRepoPath); os.IsNotExist(err) {
		w.WriteHeader(http.StatusNotFound)
		json.NewEncoder(w).Encode(map[string]string{"error": "リポジトリが見つかりません"})
		return
	}

	// リポジトリのタイプを判定
	isNormal := false
	isBare := false

	// 通常リポジトリのチェック
	if _, err := os.Stat(filepath.Join(fullRepoPath, ".git")); err == nil {
		isNormal = true
	}

	// ベアリポジトリのチェック
	if _, err := os.Stat(filepath.Join(fullRepoPath, "HEAD")); err == nil && !isNormal {
		isBare = true
	}

	if !isNormal && !isBare {
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]string{"error": "Gitリポジトリではありません"})
		return
	}

	// ファイル内容の取得
	content, isBinary, err := getFileContent(fullRepoPath, filePath, isNormal, isBare)
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
func getFileLastModified(repoPath string, filePath string) time.Time {
	var cmd *exec.Cmd

	// git logコマンドでファイルの最終更新日時を取得
	cmd = exec.Command("git", "--git-dir="+repoPath, "log", "-1", "--format=%at", "--", filePath)

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
func validateRepositoryName(name string, group string) error {
	// 空のチェック
	if name == "" {
		return fmt.Errorf("リポジトリ名が指定されていません")
	}

	// 不正な文字のチェック（英数字、ハイフン、アンダースコアのみ許可）
	validName := regexp.MustCompile(`^[a-zA-Z0-9_-]+$`)
	if !validName.MatchString(name) {
		return fmt.Errorf("リポジトリ名には英数字、ハイフン、アンダースコアのみ使用できます")
	}
	
	// グループ名が指定されていない場合はデフォルトの "git" を使用
	if group == "" {
		group = "git"
	}
	
	// 既存のリポジトリと名前が重複していないかチェック
	repoPath := filepath.Join(filepath.Join(GitRepositoryHome, group), name+".git")
	if _, err := os.Stat(repoPath); err == nil {
		return fmt.Errorf("リポジトリ '%s' は既に存在します", name)
	}

	return nil
}

// createRepository は新規ベアリポジトリを作成する
func createRepository(name string, group string) error {
	// グループ名が指定されていない場合はsplitRepositoryNameでグループ名を取得してみる
	// これは後方互換性のためと、name内にグループパスが含まれている場合の対応
	var groupName, baseName string
	if group == "" {
		groupName, baseName = splitRepositoryName(name)
	} else {
		groupName = group
		baseName = name
	}

	// リポジトリのパスを構築
	repoPath := filepath.Join(filepath.Join(GitRepositoryHome, groupName), baseName+".git")

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
	groupName, baseName := splitRepositoryName(name);

	// リポジトリのパスを構築
	repoPath := filepath.Join(filepath.Join(GitRepositoryHome, groupName), baseName+".git")

	// リポジトリの存在確認
	if _, err := os.Stat(repoPath); os.IsNotExist(err) {
		return fmt.Errorf("リポジトリ '%s' は存在しません", baseName)
	}

	// 移動先のパス（.deletedを追加）
	newPath := repoPath + ".deleted"

	// 既に削除済みのリポジトリがある場合は、それを先に完全に削除
	if _, statErr := os.Stat(newPath); statErr == nil {
		// 削除する前にアクセス権を変更（chmod 755）して読み書き可能にする
		chmodErr := os.Chmod(newPath, 0755)
		if chmodErr != nil {
			log.Printf("警告: 既存の削除済みリポジトリの権限変更に失敗しました: %v", chmodErr)
			// 権限変更に失敗してもディレクトリ削除を試みる
		}
		
		removeErr := os.RemoveAll(newPath)
		if removeErr != nil {
			return fmt.Errorf("既存の削除済みリポジトリの削除に失敗しました: %w", removeErr)
		}
	}

	// リポジトリの名前を変更
	renameErr := os.Rename(repoPath, newPath)
	if renameErr != nil {
		return fmt.Errorf("リポジトリの名前変更に失敗しました: %w", renameErr)
	}

	// 権限を変更（読み書き禁止: chmod 000）
	chmodErr := os.Chmod(newPath, 0000)
	if chmodErr != nil {
		// 権限変更に失敗した場合でも、名前の変更は成功しているので警告だけ出して続行
		log.Printf("警告: リポジトリのアクセス権限変更に失敗しました: %v", chmodErr)
	}

	return nil
}
