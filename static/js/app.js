Vue.component('repository-row', {
  props: ['repository'],
  template: `
    <tr class="repo-row" @click="openRepository" style="cursor: pointer;">
      <td class="repo-name">{{ repository.name }}</td>
      <td class="repo-path">{{ repository.path }}</td>
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
      // ベースパスを削除してリポジトリ名を取得
      const basePath = "/mnt/git/";
      let relativePath = this.repository.path;
      if (relativePath.startsWith(basePath)) {
        relativePath = relativePath.substring(basePath.length);
      }
      // リポジトリ詳細ページに遷移
      window.location.href = `/repository/${encodeURIComponent(relativePath)}`;
    }
  }
});

new Vue({
  el: '#app',
  data: {
    repositories: [],
    loading: true,
    error: null,
    searchQuery: ''
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
        <div class="form-group flex-grow-1 mr-2">
          <input 
            type="text" 
            class="form-control"
            v-model="searchQuery"
            placeholder="リポジトリを検索..."
          />
        </div>
        <div>
          <a href="/create-repository" class="btn btn-primary">
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
                <th>パス</th>
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
    this.fetchRepositories();
  },
  methods: {
    fetchRepositories() {
      // APIエンドポイントからリポジトリを取得
      axios.get('/api/repositories')
        .then(response => {
          this.repositories = response.data;
          this.loading = false;
        })
        .catch(error => {
          this.error = `リポジトリ一覧の取得に失敗しました: ${error.message}`;
          this.loading = false;
        });
    }
  }
});
