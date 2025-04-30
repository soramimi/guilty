// グローバルコンポーネントの定義をcreateAppの前に行う
const RepositoryRow = {
  props: ['repository'],
  template: `
    <tr class="repo-row" @click="openRepository" style="cursor: pointer;">
      <td class="repo-name">{{ repository.name }}</td>
      <td class="repo-commit" v-if="repository.lastCommit">
        {{ formatDate(repository.lastCommit.date) }} by {{ repository.lastCommit.author }}<br>
        <small>{{ repository.lastCommit.message }}</small>
      </td>
      <td class="repo-commit" v-else>
        <small>コミット情報なし</small>
      </td>
    </tr>
  `,
  methods: {
    formatDate(dateString) {
      const date = new Date(dateString);
      return date.toLocaleString('ja-JP');
    },
    openRepository() {
      // リポジトリ詳細ページに遷移
      window.location.href = `/repository/${encodeURIComponent(this.repository.group)}/${encodeURIComponent(this.repository.name)}`;
    }
  }
};

// アプリケーションインスタンスを作成
const app = Vue.createApp({
  data() {
    return {
      repositories: [],
      loading: true,
      error: null,
      searchQuery: '',
      groups: [],
      selectedGroup: 'git',
      loadingGroups: true,
      pageTitle: document.querySelector('h1'),
      pageMessage: document.querySelector('p')
    };
  },
  computed: {
    filteredRepositories() {
      if (!this.searchQuery) {
        return this.repositories;
      }
      const query = this.searchQuery.toLowerCase();
      return this.repositories.filter(repo => 
        repo.name.toLowerCase().includes(query) || 
        repo.path.toLowerCase().includes(query) ||
        (repo.lastCommit && repo.lastCommit.message.toLowerCase().includes(query))
      );
    }
  },
  template: `
    <div>
      <div class="d-flex justify-content-between mb-3">
        <div class="form-group mr-2" style="min-width: 200px;">
          <select 
            class="form-control" 
            v-model="selectedGroup" 
            @change="onGroupChange"
            :disabled="loadingGroups"
          >
            <option v-for="group in groups" :key="group" :value="group">
              {{ group }}
            </option>
          </select>
        </div>
        <div class="form-group flex-grow-1 mr-2">
          <input 
            type="text" 
            class="form-control"
            v-model="searchQuery"
            placeholder="リポジトリを検索..."
          />
        </div>
        <div>
          <a :href="'/create-repository?group=' + encodeURIComponent(selectedGroup)" class="btn btn-primary">
            <i class="fa fa-plus-circle"></i> 新規リポジトリ
          </a>
        </div>
      </div>
      
      <div v-if="loading" class="loading-spinner">
        <div class="spinner-border text-primary" role="status">
          <span class="sr-only">読み込み中...</span>
        </div>
      </div>
      
      <div v-else-if="error" class="error-message">
        {{ error }}
      </div>
      
      <div v-else class="repo-list">
        <div v-if="filteredRepositories.length === 0" class="text-center my-5">
          <p>表示するリポジトリがありません</p>
        </div>
        
        <div v-else class="table-responsive">
          <table class="table table-striped table-hover">
            <thead class="thead-light">
              <tr>
                <th>リポジトリ名</th>
                <th>最終コミット</th>
              </tr>
            </thead>
            <tbody>
              <repository-row 
                v-for="(repo, index) in filteredRepositories" 
                :key="index" 
                :repository="repo"
              ></repository-row>
            </tbody>
          </table>
        </div>
      </div>
    </div>
  `,
  created() {
    // URLからグループ名を取得（もしあれば）
    const urlParams = new URLSearchParams(window.location.search);
    const groupParam = urlParams.get('group');
    if (groupParam) {
      this.selectedGroup = groupParam;
    }
    
    this.fetchGroups();
  },
  methods: {
    fetchGroups() {
      // グループ一覧を取得
      this.loadingGroups = true;
      axios.get('/api/groups')
        .then(response => {
          this.groups = response.data;
          this.loadingGroups = false;
          // グループを取得した後にリポジトリを取得
          this.fetchRepositories();
        })
        .catch(error => {
          this.error = `グループ一覧の取得に失敗しました: ${error.message}`;
          this.loadingGroups = false;
          this.loading = false;
        });
    },
    fetchRepositories() {
      // APIエンドポイントからリポジトリを取得
      this.loading = true;
      axios.get(`/api/repositories?group=${encodeURIComponent(this.selectedGroup)}`)
        .then(response => {
          this.repositories = response.data;
          this.loading = false;
          
          // タイトルとメッセージを更新
          this.updatePageTitle();
        })
        .catch(error => {
          this.error = `リポジトリ一覧の取得に失敗しました: ${error.message}`;
          this.loading = false;
        });
    },
    onGroupChange() {
      // URLを更新（ブラウザの履歴に追加）
      const url = new URL(window.location);
      url.searchParams.set('group', this.selectedGroup);
      window.history.pushState({}, '', url);
      
      // グループが変更されたときにリポジトリ一覧を更新
      this.fetchRepositories();
    },
    updatePageTitle() {
      // ページのタイトルとメッセージを更新
      if (this.pageTitle && this.pageMessage) {
        this.pageMessage.textContent = `${this.selectedGroup} グループにあるGitリポジトリ一覧`;
        
        // ブラウザのタイトルも更新
        document.title = `Guilty - ${this.selectedGroup} グループのリポジトリ一覧`;
      }
    }
  }
});

// コンポーネントを登録
app.component('repository-row', RepositoryRow);

// アプリケーションをマウント
app.mount('#app');
