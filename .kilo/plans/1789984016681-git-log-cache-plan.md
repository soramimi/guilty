# Git ログ結果の TTL キャッシュ計画

## 背景・目的

`GET /-/api/repositories?group={group}` は、毎回ディスクをスキャンし、さらにグループ内の各リポジトリで `git log` を実行している。リポジトリ数が増えるとこれがボトルネックになる。`git log` の結果を TTL 付きで JSON ファイルにキャッシュし、応答速度を向上させる。

## 採用方針

| 項目 | 決定事項 |
|---|---|
| キャッシュ対象 | `git log` の結果（`CommitInfo`）のみ。ディレクトリスキャンは毎回実行 |
| キャッシュ保存先 | グループディレクトリ内の `.guilty-cache.json`（例: `/mnt/git/{group}/.guilty-cache.json`） |
| 保存形式 | JSON |
| TTL | デフォルト 60 秒、`guilty.conf` で変更可能 |
| 手動更新 | 表示中のグループのみキャッシュをクリアするボタンを UI に追加 |
| 更新検知 | TTL + 手動更新。カスタム git-shell は将来検討、今回はスコープ外 |
| キャッシュキー | `repoPath + "#" + HEADシンボリック参照` |

## 影響範囲

- `main.go` の `getLastCommit` 関数
- `main.go` の `deleteRepository` 関数
- `guilty.conf.example`
- 新規 API エンドポイント `POST /-/api/cache/invalidate`
- フロントエンド `static/js/app.js` および `templates/index.html`
- `AGENTS.md`（設定項目の追記）

## 詳細設計

### 1. 設定項目

`guilty.conf.example` に `[cache]` セクションを追加。

```ini
[cache]
enabled = yes
ttl_seconds = 60
```

`loadConfig` に以下の処理を追加。

| キー | 型 | デフォルト | 説明 |
|---|---|---|---|
| `cache.enabled` | bool | `true` | キャッシュ機能の有効/無効 |
| `cache.ttl_seconds` | int | `60` | キャッシュの有効期限（秒） |

対応するグローバル変数。

```go
var CacheEnabled = true
var CacheTTLSeconds = 60
```

### 2. キャッシュファイルの構造

グループごとに1ファイル。パスは `filepath.Join(GitRepositoryHome, groupName, ".guilty-cache.json")`。

```json
{
  "entries": {
    "/mnt/git/group/repo.git#refs/heads/main": {
      "author": "taro",
      "date": "2026-09-21T10:00:00Z",
      "message": "initial commit",
      "cachedAt": 1695295200
    }
  }
}
```

- キーは `repoPath + "#" + headRef`
- `CommitInfo` のフィールドに加え、`cachedAt`（Unix秒）を保持
- ファイルが存在しなくても動作する（初回はキャッシュミス）
- ファイル破損時は無視して再生成

### 3. キャッシュモジュール

以下の関数を `main.go` に追加（または別ファイル `cache.go` を新設してもよい）。

```go
// グループごとのキャッシュをメモリに保持するマップ
type CacheEntry struct {
    Author  string    `json:"author"`
    Date    time.Time `json:"date"`
    Message string    `json:"message"`
    CachedAt int64    `json:"cachedAt"`
}

type GroupCache struct {
    Entries map[string]CacheEntry `json:"entries"`
}

var groupCaches = make(map[string]*GroupCache)
var groupCacheMutex sync.Mutex
```

関数。

| 関数 | 役割 |
|---|---|
| `loadGroupCache(groupName string) *GroupCache` | キャッシュファイルを読み込み、メモリに展開。存在しなければ空を返す |
| `saveGroupCache(groupName string, cache *GroupCache)` | メモリ上のキャッシュを JSON ファイルに書き出す（一時ファイル → rename でアトミック書き込み） |
| `getCachedLastCommit(groupName, repoPath, headRef string) *CommitInfo` | TTL 内のキャッシュがあれば返す。なければ nil |
| `setCachedLastCommit(groupName, repoPath, headRef string, commit *CommitInfo)` | キャッシュを更新し、ファイルに保存 |
| `invalidateGroupCache(groupName string)` | グループのキャッシュファイルを削除し、メモリ上のキャッシュもクリア |

### 4. `getLastCommit` への組み込み

```go
func getLastCommit(repoPath string) *CommitInfo {
    groupName := filepath.Base(filepath.Dir(repoPath)) // 簡易例、実際は呼び出し側から渡す
    headRef, _ := getCurrentHeadBranch(repoPath)

    if CacheEnabled {
        if commit := getCachedLastCommit(groupName, repoPath, headRef); commit != nil {
            return commit
        }
    }

    // 既存の git log 処理
    commit := runGitLog(repoPath)

    if CacheEnabled && commit != nil {
        setCachedLastCommit(groupName, repoPath, headRef, commit)
    }

    return commit
}
```

**注意**: `getLastCommit` は現在 `repoPath` のみを受け取る。グループ名を正しく取得するため、`getGitRepositories` 側からグループ名を渡す形にシグネチャを変更する。

```go
func getLastCommit(repoPath, groupName string) *CommitInfo
```

`repositoryDetailsHandler` からも `getLastCommit` を呼んでいるため、そちらもグループ名を渡す。

### 5. HEAD 参照の取得

`getCurrentHeadBranch` はすでに存在する（`changeHeadBranchHandler` 周辺）。これを再利用する。

```go
func getCurrentHeadBranch(repoPath string) (string, error)
```

HEAD が detached 状態や `refs/heads/...` 以外の場合は、可能な範囲で文字列化してキーに含める。

### 6. 自動無効化

- `deleteRepository(fullPath string)` 内で、対象リポジトリのグループ名を特定し、`invalidateGroupCache(groupName)` を呼ぶ。
- `createRepository` では無効化しない（新規リポジトリは通常コミットなしのため、次回スキャンで自然に追加される）。
- `changeRepositoryHead` はキャッシュキーに HEAD 参照を含めるため、無効化不要。

### 7. 手動更新 API

新規エンドポイントを追加。

```
POST /-/api/cache/invalidate
```

リクエストボディ。

```json
{ "group": "group-name" }
```

処理。

1. リクエストボディから `group` を取得
2. グループ名のバリデーション（`isValidGroupName` を使用）
3. `invalidateGroupCache(group)` を実行
4. 成功レスポンスを返す

CORS ヘッダーは既存の API と同様に設定する。

### 8. フロントエンド変更

#### `static/js/app.js`

- グループ選択 UI の近く（またはタイトル横）に「更新」ボタンを追加
- ボタンクリック時に `POST /-/api/cache/invalidate` を呼び出し、成功後にリポジトリ一覧を再取得
- 処理中はボタンを disabled にし、スピナーまたは「更新中...」の表示を行う

#### `templates/index.html`

- 更新ボタン用の要素を追加

### 9. エラーハンドリング

- キャッシュファイルの読み書きに失敗しても、本体機能は動作し続ける
- 読み込み失敗時：ログを出力し、空キャッシュとして扱う
- 書き込み失敗時：ログを出力し、メモリ上のキャッシュは維持
- ディスク容量不足などで書き込めない場合でも、次回の読み込みで再生成を試みる

### 10. セキュリティ

- キャッシュファイルパスは `GitRepositoryHome` 配下に限定され、グループ名のバリデーションを通過する
- `..` や特殊文字を含むグループ名は `isValidGroupName` で拒否される
- 手動無効化 API もグループ名をバリデーションする

## 実装タスク

1. `guilty.conf.example` に `[cache]` セクションを追加
2. `loadConfig` にキャッシュ設定の読み込み処理を追加
3. キャッシュモジュール（`cache.go` または `main.go` 内）を実装
4. `getLastCommit` のシグネチャを `getLastCommit(repoPath, groupName string)` に変更
5. `getGitRepositories` および `repositoryDetailsHandler` で変更後のシグネチャに対応
6. `deleteRepository` でグループキャッシュを無効化
7. `POST /-/api/cache/invalidate` エンドポイントを追加
8. `main()` でエンドポイントを登録
9. `static/js/app.js` に手動更新ボタンと API 呼び出しを追加
10. `templates/index.html` にボタン要素を追加
11. `AGENTS.md` にキャッシュ設定の説明を追記
12. 動作確認
    - リポジトリ一覧が初回だけ `git log` を実行し、TTL 内は高速に応答すること
    - TTL 経過後に `git log` が再実行されること
    - 手動更新ボタンでキャッシュがクリアされること
    - リポジトリ削除後にキャッシュファイルが削除されること
    - 設定 `cache.enabled = no` でキャッシュが無効になること

## 将来検討事項（今回はスコープ外）

- ディレクトリスキャン結果自体のキャッシュ
- カスタム git-shell による push/fetch 後の即時キャッシュ無効化
- グループ横断の一括キャッシュ無効化 API
