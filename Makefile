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
	mkdir -p /home/git/.guilty
	install -m 755 guilty /home/git/.guilty/
	cp -a static /home/git/.guilty/
	cp -a templates /home/git/.guilty/
	chown -R git:git /home/git/.guilty
	#install guilty.service /etc/systemd/system/

# バイナリとキャッシュファイルを削除
clean:
	rm -f guilty
	go clean

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


start:
	systemctl start guilty

stop:
	systemctl stop guilty
