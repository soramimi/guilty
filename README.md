[![Ask DeepWiki](https://deepwiki.com/badge.svg)](https://deepwiki.com/soramimi/guilty)

# Guilty

**🚧 現在開発中です 🚧**

Guiltyは、WebベースのGitリポジトリ管理ツールです。直感的なWebインターフェースを通じて、シンプルなリポジトリ管理機能を提供します。「Guilty」という名前は、Gitをもじった遊び心のある呼び方で、コードの変更をバージョン管理から隠せないことを示唆しています。

## 機能

- **リポジトリ一覧**: 一元化されたダッシュボードですべてのGitリポジトリを確認
- **リポジトリのグループ化**: リポジトリを論理的なグループで整理
- **リポジトリの作成**: バリデーション付きで新しいbare Gitリポジトリを作成
- **リポジトリの削除**: 安全にリポジトリを削除（論理削除方式）
- **ファイルの閲覧**: リポジトリ内のファイルやディレクトリを移動
- **ファイルの表示**: テキスト／バイナリ判定付きでファイル内容を表示
- **クローンURLのサポート**: GitクローンURLを簡単にコピー
- **Git LFSサポート**: MinIO（S3互換オブジェクトストレージ）をバックエンドとしたGit Large File Storage（LFS）Batch API

## システム要件

- Go 1.24以降
- Gitコマンドラインツール
- systemd（サービスインストール用）
- ローカルの `git` ユーザーアカウント（下記の前提条件を参照）
- MinIOまたはその他のS3互換オブジェクトストレージ（Git LFSサポート用）

## 前提条件

### Gitユーザーアカウントの設定

Guiltyは、リポジトリへのアクセスを適切に処理するために、ローカルの `git` ユーザーアカウントを必要とします。

```bash
# gitユーザーアカウントが存在しない場合は作成
sudo useradd git
```

既に `git` アカウントが存在し、`useradd` で作成できない場合は、以下の手順を行ってください。

1. 必要に応じてgitユーザーのホームディレクトリを作成:
   ```bash
   sudo mkdir -p /home/git
   ```

2. `/etc/passwd` を編集し、gitユーザーのホームディレクトリを正しく設定。

3. リポジトリへの外部アクセスのため、`/home/git/git` から実際のリポジトリ場所へシンボリックリンクを作成:
   ```bash
   cd /home/git
   sudo ln -s /mnt/git git
   ```

この設定により、リポジトリは `git@hostname:group/repository.git` という形式でアクセスできるようになります。

## インストール

```bash
# リポジトリをクローン
git clone https://github.com/soramimi/guilty.git
cd guilty

# アプリケーションをビルド
make build

# アプリケーションをインストール（スーパーユーザー権限が必要）
sudo make install

# systemdサービスをセットアップ
sudo cp guilty.service /etc/systemd/system/
sudo systemctl daemon-reload
sudo systemctl enable guilty
sudo systemctl start guilty
```

## 設定

デフォルトでは、Guiltyは `/home/git` 以下のGitリポジトリを探します。保存場所を変更する必要がある場合は、ビルド前にソースコード内の `GitRepositoryHome` 定数を変更してください。

GitクローンURLに使用されるホスト名はデフォルトで `git` ですが、ソースコード内の `GitHostName` 変数を変更することでカスタマイズできます。

### Git LFS設定

Guiltyは、以下のエンドポイントで [Git LFS Batch API](https://github.com/git-lfs/git-lfs/blob/main/docs/api/batch.md) を実装しています。

```
POST /lfs/{group}/{reponame}/info/lfs/objects/batch
```

LFSオブジェクトはMinIO（S3互換）バケットに保存されます。以下の変数はソースコード内で調整可能です。

| 変数 | デフォルト値 | 説明 |
|---|---|---|
| `LFSStorageEndpoint` | `http://minio.example.com:9000` | MinIOサーバーのURL |
| `LFSBucketName` | `gitlfs` | LFSオブジェクト用バケット名 |
| `LFSAccessKeyID` | `minioadmin` | MinIOアクセスキー |
| `LFSSecretAccessKey` | `minioadmin` | MinIOシークレットキー |
| `LFSURLExpiry` | `600` | 署名付きURLの有効期限（秒） |

オブジェクトは以下のキーパスに保存されます。
```
{group}/{reponame}/{oid[0:2]}/{oid[2:4]}/{oid}
```

リポジトリでGuiltyをLFSサーバーとして使用するには、以下のようにLFS URLを設定してください。

```bash
git config lfs.url http://your-server:1080/lfs/group/reponame
```

## 使い方

起動後、Webインターフェースは http://localhost:1080 でアクセスできます。

Webインターフェースでは以下の操作が可能です。

- グループ別に整理された既存リポジトリの閲覧
- グループによるリポジトリの絞り込み
- 指定したグループ内への新規リポジトリ作成
- ファイル内容の表示
- リポジトリの削除

## リポジトリグループ

Guiltyはリポジトリをグループで整理します。

- グループは `GitRepositoryHome` ディレクトリ（デフォルト: `/home/git`）内のサブディレクトリで表されます
- デフォルトのグループは `git` です
- `-` と `_` を除く特殊文字を含むグループは除外されます
- グループ `git-shell-commands` は明示的に除外されます
- リポジトリURLは `git@hostname:group/repository.git` の形式になります

## 開発

```bash
# 開発モードで実行
make run

# アプリケーションをビルド
make build

# ビルド成果物を削除
make clean
```

## JavaScriptユーティリティ

Guiltyには、APIエンドポイントとの連携やページ間の移動に使用されるURL生成関数を一貫して提供するJavaScriptユーティリティライブラリ（`GuiltyUtils`）が含まれています。これにより、アプリケーション全体で適切なURLエンコーディングと一貫したURLパターンが保証されます。
