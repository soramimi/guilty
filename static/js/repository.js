// コンポーネントの定義
const FileRow = {
  props: ['file', 'repoName'],
  template: `
    <tr>
      <td>
        <span v-if="file.type === 'dir'" class="text-primary" @click="openDirectory" style="cursor: pointer;">📁 {{ file.name }}/</span>
        <span v-if="file.type === 'file'" @click="openFile" style="cursor: pointer;">📄 {{ file.name }}</span>
      </td>
      <td>{{ formatFileType(file.type) }}</td>
      <td v-if="file.type === 'file'">{{ formatFileSize(file.size) }}</td>
      <td v-else>-</td>
      <td class="datetime-cell">{{ formatDate(file.lastModified) }}</td>
    </tr>
  `,
  methods: {
    formatFileType(type) {
      return type === 'dir' ? 'ディレクトリ' : 'ファイル';
    },
    formatFileSize(size) {
      if (size < 1024) return size + ' B';
      if (size < 1024 * 1024) return (size / 1024).toFixed(2) + ' KB';
      if (size < 1024 * 1024 * 1024) return (size / (1024 * 1024)).toFixed(2) + ' MB';
      return (size / (1024 * 1024 * 1024)).toFixed(2) + ' GB';
    },
    formatDate(dateString) {
      if (!dateString) return '-';
      const date = new Date(dateString);
      
      // 年月日の区切りに-を使用し、時刻はそのまま
      const year = date.getFullYear();
      const month = String(date.getMonth() + 1).padStart(2, '0');
      const day = String(date.getDate()).padStart(2, '0');
      const hours = String(date.getHours()).padStart(2, '0');
      const minutes = String(date.getMinutes()).padStart(2, '0');
      const seconds = String(date.getSeconds()).padStart(2, '0');
      
      return `${year}-${month}-${day} ${hours}:${minutes}:${seconds}`;
    },
    openDirectory() {
      this.$emit('open-directory', this.file);
    },
    openFile() {
      this.$emit('open-file', this.file);
    }
  }
};

// アプリケーションの作成
const repositoryApp = Vue.createApp({
  data() {
    return {
      repository: null,
      files: [],
      loading: true,
      error: null,
      searchQuery: '',
      currentPath: '',
      pathHistory: [],
      directoryStack: [],
      selectedFile: null,
      fileContent: '',
      fileLoading: false,
      fileError: null,
      isBinaryFile: false,
      showFileModal: false,
      modalJustOpened: false,
      showDeleteModal: false, // 削除確認モーダル表示フラグ
      deleteInProgress: false, // 削除処理中フラグ
      deleteError: null, // 削除エラーメッセージ
      hostName: document.querySelector('meta[name="git-host"]')?.content || 'localhost'
    };
  },
  computed: {
    repoPath() {
      const path = window.location.pathname;
      return path.substring('/repository/'.length);
    },
    groupName() {
      const parts = this.repoPath.split('/');
      if (parts.length >= 2) {
        return parts[0];
      }
      return 'git'; // デフォルトグループ
    },
    repoName() {
      const parts = this.repoPath.split('/');
      if (parts.length >= 2) {
        return parts[1];
      }
      return parts[0]; // グループが指定されていない場合
    },
    currentViewPath() {
      return this.currentPath ? this.currentPath : 'ルートディレクトリ';
    },
    filteredFiles() {
      if (!this.searchQuery) {
        return this.files;
      }
      const query = this.searchQuery.toLowerCase();
      return this.files.filter(file => 
        file.name.toLowerCase().includes(query)
      );
    },
    fullRepoPath() {
      // グループ名とリポジトリ名を含む完全パス
      return `${this.groupName}/${this.repoName}`;
    }
  },
  template: `
    <div>
      <div class="mb-3">
        <a :href="getRepositoriesPageUrl(groupName)" class="btn btn-outline-secondary">← リポジトリ一覧に戻る</a>
      </div>
      
      <div v-if="loading" class="loading-spinner">
        <div class="spinner-border text-primary" role="status">
          <span class="sr-only">読み込み中...</span>
        </div>
      </div>
      
      <div v-else-if="error" class="error-message">
        {{ error }}
      </div>
      
      <div v-else>
        <!-- リポジトリの基本情報 -->
        <div class="card mb-4">
          <div class="card-header bg-light">
            <h3>リポジトリ情報</h3>
          </div>
          <div class="card-body">
            <dl class="row">
              <dt class="col-sm-2 text-left">名前</dt>
              <dd class="col-sm-10 text-left">{{ repository.name }}</dd>
              
              <dt class="col-sm-2 text-left">最終コミット</dt>
              <dd v-if="repository.lastCommit" class="col-sm-10 text-left">
                {{ formatDate(repository.lastCommit.date) }} by {{ repository.lastCommit.author }}
              </dd>
              <dd v-else class="col-sm-10 text-left">コミット情報なし</dd>
              
              <dt class="col-sm-2 text-left">クローンURL</dt>
              <dd class="col-sm-10">
                <div class="input-group">
                  <input type="text" class="form-control" readonly :value="repository.cloneUrl || ''" id="cloneUrlInput">
                  <div class="input-group-append">
                    <button class="btn btn-outline-secondary" type="button" @click="copyCloneUrl" title="URLをコピー">
                      <span>コピー</span>
                    </button>
                  </div>
                </div>
                <small class="text-muted mt-1 d-block">{{ repository.cloneUrl ? '' : 'クローンURLが取得できませんでした' }}</small>
              </dd>
            </dl>
            
            <!-- 削除ボタン追加 -->
            <div class="mt-3 text-right">
              <button class="btn btn-danger" @click="confirmDelete">
                <span>リポジトリの削除</span>
              </button>
            </div>
          </div>
        </div>
        
        <!-- ファイル一覧 -->
        <div class="card">
          <div class="card-header bg-light">
            <div class="d-flex justify-content-between align-items-center">
              <h3 class="mb-0">ファイル一覧 ({{ currentViewPath }})</h3>
              <div class="form-inline">
                <input 
                  type="text" 
                  class="form-control form-control-sm"
                  v-model="searchQuery"
                  placeholder="ファイルを検索..."
                />
              </div>
            </div>
          </div>
          <div class="card-body">
            <!-- パンくずリスト -->
            <div v-if="directoryStack.length > 0" class="mb-3">
              <nav aria-label="breadcrumb">
                <ol class="breadcrumb">
                  <li class="breadcrumb-item">
                    <a href="#" @click.prevent="navigateToRoot">ルート</a>
                  </li>
                  <li v-for="(dir, index) in directoryStack" 
                      :key="index" 
                      class="breadcrumb-item" 
                      :class="{ active: index === directoryStack.length - 1 }">
                    <a href="#" 
                       v-if="index < directoryStack.length - 1" 
                       @click.prevent="navigateToPath(index)">
                      {{ dir.name }}
                    </a>
                    <span v-else>{{ dir.name }}</span>
                  </li>
                </ol>
              </nav>
            </div>
            
            <div v-if="filteredFiles.length === 0" class="text-center my-3">
              <div class="alert alert-info" role="alert">
                <i class="fa fa-info-circle mr-1"></i> このリポジトリにはまだコミットがありません。<br>
                最初のコミットをプッシュするには、以下のコマンドを実行してください：
                <pre class="mt-2 mb-0 bg-light p-2 rounded text-left">
git clone {{ repository.cloneUrl }}
cd {{ repository.name }}
touch README.md
git add README.md
git commit -m "Initial commit"
git push origin master</pre>
              </div>
            </div>
            
            <div v-else class="table-responsive">
              <table class="table table-striped">
                <thead class="thead-light">
                  <tr>
                    <th>名前</th>
                    <th>種類</th>
                    <th>サイズ</th>
                    <th>最終更新日時</th>
                  </tr>
                </thead>
                <tbody>
                  <file-row 
                    v-for="(file, index) in filteredFiles" 
                    :key="index" 
                    :file="file"
                    :repo-name="repoName"
                    @open-directory="openDirectory"
                    @open-file="openFile"
                  ></file-row>
                </tbody>
              </table>
            </div>
          </div>
        </div>
      </div>
      
      <!-- ファイル内容を表示するモーダル -->
      <div v-if="showFileModal" class="modal-wrapper">
        <div class="modal-backdrop" @click="closeFileModal"></div>
        <div class="modal file-modal" tabindex="-1" role="dialog">
          <div class="modal-dialog modal-lg" role="document">
            <div class="modal-content">
              <div class="modal-header">
                <h5 class="modal-title">{{ selectedFile && selectedFile.name }}</h5>
                <button type="button" class="close" @click="closeFileModal" aria-label="閉じる">
                  <span aria-hidden="true">&times;</span>
                </button>
              </div>
              <div class="modal-body">
                <div v-if="fileLoading" class="text-center p-3">
                  <div class="spinner-border text-primary" role="status">
                    <span class="sr-only">ファイル読み込み中...</span>
                  </div>
                </div>
                <div v-else-if="fileError" class="alert alert-danger">
                  {{ fileError }}
                </div>
                <div v-else-if="isBinaryFile" class="alert alert-warning">
                  このファイルはバイナリファイルのため表示できません。
                </div>
                <pre v-else class="file-content">{{ fileContent }}</pre>
              </div>
              <div class="modal-footer">
                <button type="button" class="btn btn-secondary" @click="closeFileModal">閉じる</button>
              </div>
            </div>
          </div>
        </div>
      </div>
      
      <!-- リポジトリ削除確認モーダル -->
      <div v-if="showDeleteModal" class="modal-wrapper delete-modal-wrapper" style="display: block !important;">
        <div class="modal-backdrop delete-modal-backdrop" @click="cancelDelete" style="display: block; z-index: 1999;"></div>
        <div class="modal delete-modal" tabindex="-1" role="dialog" aria-labelledby="deleteModalLabel" aria-hidden="false" style="display: block; z-index: 2000;">
          <div class="modal-dialog" role="document">
            <div class="modal-content">
              <div class="modal-header">
                <h5 class="modal-title text-danger" id="deleteModalLabel">リポジトリの削除確認</h5>
                <button type="button" class="close" @click="cancelDelete" aria-label="閉じる">
                  <span aria-hidden="true">&times;</span>
                </button>
              </div>
              <div class="modal-body">
                <p><strong>{{ repository.name }}</strong> リポジトリを削除しますか？</p>
                <div class="alert alert-warning">
                  <strong>注意:</strong> この操作はリポジトリをアクセスできなくします。削除されたリポジトリは復元できません。
                </div>
                
                <div v-if="deleteError" class="alert alert-danger mt-3">
                  {{ deleteError }}
                </div>
              </div>
              <div class="modal-footer">
                <button type="button" class="btn btn-secondary" @click="cancelDelete">キャンセル</button>
                <button type="button" class="btn btn-danger" @click="deleteRepository" :disabled="deleteInProgress">
                  <span v-if="deleteInProgress" class="spinner-border spinner-border-sm mr-2" role="status"></span>
                  削除する
                </button>
              </div>
            </div>
          </div>
        </div>
      </div>
    </div>
  `,
  created() {
    this.fetchRepositoryDetails();
    // キーボードイベントリスナーを登録
    document.addEventListener('keydown', this.handleKeyDown);
    // モーダル外クリック検出のためのイベントリスナー登録
    document.addEventListener('click', this.handleOutsideClick);
  },
  unmounted() {
    // コンポーネント破棄時にイベントリスナーを削除
    document.removeEventListener('keydown', this.handleKeyDown);
    document.removeEventListener('click', this.handleOutsideClick);
  },
  methods: {
    formatDate(dateString) {
      if (!dateString) return '-';
      const date = new Date(dateString);
      
      // 年月日の区切りに-を使用し、時刻はそのまま
      const year = date.getFullYear();
      const month = String(date.getMonth() + 1).padStart(2, '0');
      const day = String(date.getDate()).padStart(2, '0');
      const hours = String(date.getHours()).padStart(2, '0');
      const minutes = String(date.getMinutes()).padStart(2, '0');
      const seconds = String(date.getSeconds()).padStart(2, '0');
      
      return `${year}-${month}-${day} ${hours}:${minutes}:${seconds}`;
    },
    fetchRepositoryDetails() {
      axios.get(GuiltyUtils.getApiRepositoryPath(this.groupName, this.repoName))
        .then(response => {
          const details = response.data;
          this.repository = details.repository;
          this.files = details.files;
          this.loading = false;
        })
        .catch(error => {
          this.error = `リポジトリ詳細の取得に失敗しました: ${error.message}`;
          this.loading = false;
        });
    },
    copyCloneUrl() {
      const cloneUrlInput = document.getElementById('cloneUrlInput');
      if (cloneUrlInput) {
        cloneUrlInput.select();
        document.execCommand('copy');
        // コピー成功を通知するためにボタンテキストを一時的に変更
        const button = document.querySelector('#cloneUrlInput + .input-group-append button');
        if (button) {
          const originalText = button.textContent;
          button.textContent = 'コピーしました！';
          setTimeout(() => {
            button.textContent = originalText;
          }, 1500);
        }
      }
    },
    openDirectory(directory) {
      this.loading = true;
      this.directoryStack.push({
        name: directory.name,
        path: directory.path
      });
      this.currentPath = directory.path;
      
      axios.get(GuiltyUtils.getApiDirectoryPath(this.groupName, this.repoName, directory.path))
        .then(response => {
          this.files = response.data;
          this.loading = false;
        })
        .catch(error => {
          this.error = `ディレクトリの内容を取得できませんでした: ${error.message}`;
          this.loading = false;
        });
    },
    openFile(file) {
      // モーダルを開くときにフラグをリセット
      this.modalJustOpened = true;
      
      this.selectedFile = file;
      this.fileLoading = true;
      this.fileError = null;
      this.fileContent = '';
      this.showFileModal = true;
      document.body.classList.add('modal-open');
      
      // イベントの伝播を防ぐために少し遅延させる
      setTimeout(() => {
        this.modalJustOpened = false;
      }, 10);
      
      axios.get(GuiltyUtils.getApiFilePath(this.groupName, this.repoName, file.path))
        .then(response => {
          this.fileContent = response.data.content;
          this.isBinaryFile = response.data.isBinary;
          this.fileLoading = false;
        })
        .catch(error => {
          this.fileError = `ファイルの内容を取得できませんでした: ${error.message}`;
          this.fileLoading = false;
        });
    },
    closeFileModal() {
      this.showFileModal = false;
      document.body.classList.remove('modal-open');
      // データクリーンアップ
      setTimeout(() => {
        if (!this.showFileModal) {
          this.selectedFile = null;
          this.fileContent = '';
        }
      }, 300);
    },
    confirmDelete() {
      // 一度ファイルモーダルが開いていたら閉じる
      if (this.showFileModal) {
        this.closeFileModal();
      }
      
      // 削除確認モーダル表示前に少し遅延
      setTimeout(() => {
        // 削除確認モーダルを表示
        this.showDeleteModal = true;
        this.deleteError = null;
        document.body.classList.add('modal-open');
        document.body.classList.add('has-delete-modal'); // 削除モーダル専用クラスを追加
        
        // 強制的にDOM更新をトリガーする
        this.$nextTick(() => {
          // モーダルを確実に表示させるため、モーダル要素のスタイルを直接操作
          const modal = document.querySelector('.delete-modal');
          if (modal) {
            modal.style.display = 'block';
            modal.style.zIndex = '2000';
            modal.style.visibility = 'visible';
            modal.style.opacity = '1';
          }
          
          // モーダルのbackdropも確実に表示
          const backdrop = document.querySelector('.modal-wrapper:last-child .modal-backdrop');
          if (backdrop) {
            backdrop.style.display = 'block';
            backdrop.style.zIndex = '1999';
          }
          
          // すべてのモーダルラッパーを最前面に
          const wrappers = document.querySelectorAll('.modal-wrapper');
          if (wrappers.length > 0) {
            const lastWrapper = wrappers[wrappers.length - 1];
            lastWrapper.style.zIndex = '1990';
          }
        });
      }, 100);
    },
    cancelDelete() {
      // 削除確認モーダルを閉じる
      this.showDeleteModal = false;
      document.body.classList.remove('modal-open');
      document.body.classList.remove('has-delete-modal'); // 削除モーダル専用クラスを削除
      this.deleteError = null;
    },
    deleteRepository() {
      // 削除処理中フラグをセット
      this.deleteInProgress = true;
      this.deleteError = null;
      
      // リポジトリ名から.git拡張子を削除（既に含まれている場合）
      let repoNameToDelete = this.repoName;
      if (repoNameToDelete.endsWith('.git')) {
        repoNameToDelete = repoNameToDelete.substring(0, repoNameToDelete.length - 4);
      }
      
      // リポジトリ削除APIを呼び出し
      axios({
        method: 'post',
        url: GuiltyUtils.getApiRepositoryPath(this.groupName, repoNameToDelete),
        data: {
          operation: "delete" // 操作タイプを指定
        },
        headers: {
          'Content-Type': 'application/json'
        }
      })
        .then(response => {
          // 削除成功時の処理
          this.deleteInProgress = false;
          this.showDeleteModal = false;
          document.body.classList.remove('modal-open');
          document.body.classList.remove('has-delete-modal'); // 削除モーダル専用クラスを削除
          // ホームページ（リポジトリ一覧）にリダイレクトする際、現在のグループ名を維持
          window.location.href = GuiltyUtils.getRepositoriesPageUrl(this.groupName);
        })
        .catch(error => {
          // エラー処理
          this.deleteInProgress = false;
          this.deleteError = `リポジトリの削除に失敗しました: ${error.response?.data?.error || error.message || '不明なエラー'}`;
        });
    },
    handleKeyDown(event) {
      // ESCキーでモーダルを閉じる
      if (event.key === 'Escape') {
        if (this.showFileModal) {
          this.closeFileModal();
        }
        if (this.showDeleteModal) {
          this.cancelDelete();
        }
      }
    },
    handleOutsideClick(event) {
      // ファイルモーダル処理
      if (this.showFileModal && !this.modalJustOpened) {
        const modalContent = document.querySelector('.file-modal .modal-content');
        if (modalContent && !modalContent.contains(event.target) && 
            !event.target.classList.contains('close')) {
          this.closeFileModal();
        }
      }
      
      // 削除モーダル処理
      if (this.showDeleteModal) {
        const modalContent = document.querySelector('.delete-modal .modal-content');
        if (modalContent && !modalContent.contains(event.target) && 
            !event.target.classList.contains('close')) {
          this.cancelDelete();
        }
      }
    },
    navigateToRoot() {
      this.loading = true;
      this.directoryStack = [];
      this.currentPath = '';
      
      this.fetchRepositoryDetails();
    },
    navigateToPath(index) {
      // index番目までのパスに移動
      if (index < 0 || index >= this.directoryStack.length - 1) {
        return;
      }
      
      this.loading = true;
      const targetDir = this.directoryStack[index];
      this.directoryStack = this.directoryStack.slice(0, index + 1);
      this.currentPath = targetDir.path;
      
      axios.get(GuiltyUtils.getApiDirectoryPath(this.groupName, this.repoName, targetDir.path))
        .then(response => {
          this.files = response.data;
          this.loading = false;
        })
        .catch(error => {
          this.error = `ディレクトリの内容を取得できませんでした: ${error.message}`;
          this.loading = false;
        });
    },
    getRepositoriesPageUrl(group) {
      return GuiltyUtils.getRepositoriesPageUrl(group);
    }
  }
});

// コンポーネントを登録
repositoryApp.component('file-row', FileRow);

// アプリケーションをマウント
repositoryApp.mount('#repository-app');
