Vue.component('file-row', {
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
});

new Vue({
  el: '#repository-app',
  data: {
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
    hostName: document.querySelector('meta[name="git-host"]')?.content || 'localhost'
  },
  computed: {
    repoPath() {
      const path = window.location.pathname;
      return path.substring('/repository/'.length);
    },
    repoName() {
      const parts = this.repoPath.split('/');
      return parts[0];
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
    }
  },
  template: `
    <div>
      <div class="mb-3">
        <a href="/" class="btn btn-outline-secondary">← リポジトリ一覧に戻る</a>
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
    </div>
  `,
  created() {
    this.fetchRepositoryDetails();
    // キーボードイベントリスナーを登録
    document.addEventListener('keydown', this.handleKeyDown);
    // モーダル外クリック検出のためのイベントリスナー登録
    document.addEventListener('click', this.handleOutsideClick);
  },
  destroyed() {
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
      axios.get(`/api/repository/${this.repoPath}`)
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
      
      axios.get(`/api/directory/${encodeURIComponent(this.repoName)}/${encodeURIComponent(directory.path)}`)
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
      
      axios.get(`/api/file/${encodeURIComponent(this.repoName)}/${encodeURIComponent(file.path)}`)
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
    handleKeyDown(event) {
      // ESCキーでモーダルを閉じる
      if (event.key === 'Escape') {
        if (this.showFileModal) {
          this.closeFileModal();
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
      
      axios.get(`/api/directory/${encodeURIComponent(this.repoName)}/${encodeURIComponent(targetDir.path)}`)
        .then(response => {
          this.files = response.data;
          this.loading = false;
        })
        .catch(error => {
          this.error = `ディレクトリの内容を取得できませんでした: ${error.message}`;
          this.loading = false;
        });
    }
  }
});