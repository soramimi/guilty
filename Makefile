.PHONY: run build clean test install

# デフォルトターゲット
all: run

# アプリケーションを実行
run:
	go run main.go

# アプリケーションをビルド
build:
	go build -o guilty main.go

# インストール
install: build
	install -D -m 755 guilty /usr/local/bin/guilty

# バイナリとキャッシュファイルを削除
clean:
	rm -f guilty
	go clean

# テストを実行
test:
	go test ./...

# 静的ファイル用のフォルダを作成
setup:
	mkdir -p static/css static/js templates
	@if [ ! -f templates/index.html ]; then \
		echo '<!DOCTYPE html>' > templates/index.html; \
		echo '<html lang="ja">' >> templates/index.html; \
		echo '<head>' >> templates/index.html; \
		echo '    <meta charset="UTF-8">' >> templates/index.html; \
		echo '    <meta name="viewport" content="width=device-width, initial-scale=1.0">' >> templates/index.html; \
		echo '    <title>Guilty - {{.Title}}</title>' >> templates/index.html; \
		echo '    <link rel="stylesheet" href="/static/css/style.css">' >> templates/index.html; \
		echo '</head>' >> templates/index.html; \
		echo '<body>' >> templates/index.html; \
		echo '    <h1>{{.Message}}</h1>' >> templates/index.html; \
		echo '    <script src="/static/js/main.js"></script>' >> templates/index.html; \
		echo '</body>' >> templates/index.html; \
		echo '</html>' >> templates/index.html; \
		echo 'body { font-family: Arial, sans-serif; text-align: center; margin-top: 50px; }' > static/css/style.css; \
		echo 'console.log("Hello from Guilty!");' > static/js/main.js; \
	fi

# ヘルプを表示
help:
	@echo "使用方法:"
	@echo "  make run      - サーバーを起動します"
	@echo "  make build    - アプリケーションをビルドします"
	@echo "  make install  - アプリケーションをインストールします"
	@echo "  make clean    - ビルド成果物を削除します" 
	@echo "  make test     - テストを実行します"
	@echo "  make setup    - 必要なディレクトリとファイルを作成します"
	@echo "  make help     - このヘルプを表示します"
