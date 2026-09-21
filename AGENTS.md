# Guilty

## プロジェクト概要

Guiltyは、WebベースのGitリポジトリ管理ツールです。MinIO/S3互換のオブジェクトストレージをバックエンドとしたGit LFSサポート付きで、bare Gitリポジトリの閲覧、作成、削除を行うためのダッシュボードを提供します。

## コマンド

```bash
# 開発
make run       # go run main.go（ポート1080でサーバー起動）

# ビルド
make build     # go build -o guilty main.go

# クリーン
make clean

# インストール（gitユーザーとして /home/git/.guilty/ に配置）
make install

# サービス管理
make start / stop / restart   # systemctlのラッパー
```

現在、テストスイートは設定されていません。

## アーキテクチャ

このアプリケーションは、Vue.js 3フロントエンドを提供する単一バイナリのGoサーバー（`main.go`）です。

**バックエンド**（`main.go`）:
- JSON REST APIとHTMLテンプレートレンダリングを備えたHTTPサーバー
- すべてのリポジトリ操作は `git` CLIを呼び出して実行（Goのgitライブラリは使用しない）
- AWS SDK v2は、Git LFS用の署名付きS3/MinIO URLを生成する場合のみ使用
- `guilty.conf`（INI形式）から設定を読み込む。詳細は `guilty.conf.example` を参照

設定から読み込まれる主要なグローバル変数:
```go
var ServerPort = 1080
var GitRepositoryHome = "/home/git"
var GitHostName = "git"
const GitCloneURLTemplate = "ssh://git@%s/~/  %s/%s.git"
const LFSConfigTemplate = `git config lfs.url "http://%s/%s/%s/info/lfs"`
```

リポジトリの削除は論理削除です。ディレクトリは削除されるのではなく、`.deleted` という接尾辞を付けてリネームされます。

**フロントエンド**（`static/js/`）:
- `app.js` — リポジトリ一覧ページ（グループ絞り込み、検索）
- `repository.js` — リポジトリ詳細ページ（ファイルブラウザ、クローンURL、HEADブランチ）
- `create-repository.js` — リポジトリ作成フォーム
- `main.js` — 共有 `GuiltyUtils`（URL生成ヘルパー）

テンプレートは `templates/` にあり、Goの `html/template` を使用します。

**URLルーティング**:

`/-/` 接頭辞のつくパスはすべて内部ルート（API、静的ファイル、UIページ）です。接頭辞のないパスはリポジトリコンテンツを提供します。
- `GET /` — ホームページ（リポジトリ一覧）
- `GET /{group}/{repo}` — リポジトリ詳細ページ
- `POST /{group}/{repo}/info/lfs/objects/batch` — Git LFS Batch API（`homeHandler` 経由でルーティング）
- `GET /-/static/` — 静的ファイル
- `GET /-/create-repository` — リポジトリ作成ページ

**APIエンドポイント**:
- `GET /-/api/repositories` — グループ別のリポジトリ一覧
- `GET /-/api/groups` — グループ一覧
- `POST /-/api/repositories` — リポジトリ作成
- `GET /-/api/repository/{group}/{repo}` — リポジトリメタデータ
- `POST /-/api/repository/{group}/{repo}` — リポジトリ削除
- `GET /-/api/directory/{group}/{repo}/{path}` — ディレクトリ一覧
- `GET /-/api/file/{group}/{repo}/{filePath}` — ファイル内容
- `POST /-/api/head/{group}/{repo}` — HEADブランチの変更

## 設定

`guilty.conf.example` を `guilty.conf` にコピーして調整してください。

```ini
[server]
listen_port = 1080
host_name = git

[git]
repository_home = /home/git

[lfs]
storage_endpoint = http://minio-host:9000
storage_access_key = minioadmin
storage_secret_key = minioadmin
bucket_name = gitlfs
url_expiry = 600
```

## デプロイメント

このアプリは同梱のsystemdユニット（`guilty.service`）を通じて `git` ユーザーとして実行され、作業ディレクトリは `/home/git/.guilty/` です。

## キャッシュ

リポジトリ一覧の応答速度向上のため、`git log` の結果をグループごとに TTL キャッシュします。

- キャッシュファイル: `{GitRepositoryHome}/{group}/.guilty-cache.json`
- ディレクトリスキャンは毎回実行され、キャッシュされるのは各リポジトリの最終コミット情報のみ
- キャッシュキーは `repoPath + "#" + HEAD参照` の形式。HEAD ブランチ変更時は自動的にキャッシュミスとなる
- TTL は `guilty.conf` の `cache.ttl_seconds` で設定可能（デフォルト 60 秒）
- Web UI の「更新」ボタン、または `POST /-/api/cache/invalidate` で表示中グループのキャッシュを手動無効化可能
- `deleteRepository` 実行時には対象グループのキャッシュファイルが削除される
- 将来的にはカスタム git-shell による push/fetch 後の即時無効化も検討される

## カスタム git-shell（試作）

`my-git-shell/` には、C++17 で実装中の非対話型カスタム git-shell の試作版があります。

- 用途: SSH 経由での `git clone`/`push`/`fetch` を制限付きで許可するログインシェル
- 対応コマンド: `git-receive-pack`, `git-upload-pack`, `git-upload-archive`
- 動作:
  - `SSH_ORIGINAL_COMMAND` 環境変数、または `-c` オプション経由で渡されたコマンドを解析し、許可されたコマンドと安全なリポジトリパスのみ `execvp` で実際の git コマンドに委譲
  - `-c` オプションは sshd の `command="/path/to/my-git-shell -c"` 形式を想定し、`argv[2]` に含まれるコマンド文字列全体を `parse_command()` で解析する
- セキュリティ: 絶対パス、シェルメタ文字、`..` による traversal、ディレクトリ存在を検証
- ビルド: `cd my-git-shell && make`
- インストール例: `~/.ssh/authorized_keys` の `command=` に `my-git-shell` のパスを指定

### キャッシュ無効化連携（準備段階）

将来的な `push`/`fetch` 後の即時キャッシュ無効化に向けて、`my-git-shell/main.cpp` にグループキャッシュファイル（`.guilty-cache.json`）の JSON 読み書き関数を追加しました。

- `guilty::load_cache_file(path)` — キャッシュ JSON を読み込み、エントリ一覧を返す
- `guilty::save_cache_file(path, cache)` — キャッシュ JSON を一時ファイル（`.tmp`）へ書き込み、`rename()` でアトミックに置き換える
- これらの関数は現時点ではデバッグブロック内でのみ呼び出されており、実際の push/fetch 後の無効化ロジックは未実装です
- 実装にあたり、Go 側の `cacheKey` 形式（`repoPath + "#" + HEAD参照`）に合わせ、ブランチ名だけでなく detached HEAD の SHA をキーに持つエントリも読み書き対象としています
