// filepath: /home/soramimi/develop/guilty/static/js/create-repository.js

const createRepoApp = Vue.createApp({
  data() {
    return {
      repositoryName: '',
      isSubmitting: false,
      error: null,
      success: null,
      validationError: null,
      groups: [],
      selectedGroup: 'git',
      loadingGroups: true
    };
  },
  computed: {
    isNameValid() {
      return this.repositoryName.trim() !== '';
    }
  },
  template: `
    <div>
      <div class="mb-3">
        <a href="/" class="btn btn-outline-secondary">← リポジトリ一覧に戻る</a>
      </div>

      <div v-if="success" class="alert alert-success">
        {{ success }}
        <div class="mt-3">
          <a :href="'/repository/' + encodeURIComponent(selectedGroup) + '/' + encodeURIComponent(repositoryName)" class="btn btn-primary">リポジトリを表示する</a>
        </div>
      </div>

      <div v-else>
        <div v-if="error" class="alert alert-danger">
          {{ error }}
        </div>

        <div class="card">
          <div class="card-header bg-light">
            <h3>新規リポジトリの作成</h3>
          </div>
          <div class="card-body">
            <form @submit.prevent="createRepository">
              <div class="form-group mb-3">
                <label for="groupSelect">グループ</label>
                <select 
                  id="groupSelect" 
                  class="form-control" 
                  v-model="selectedGroup"
                  :disabled="loadingGroups || isSubmitting"
                >
                  <option v-for="group in groups" :key="group" :value="group">
                    {{ group }}
                  </option>
                </select>
                <small class="form-text text-muted">
                  リポジトリを作成するグループを選択してください。
                </small>
              </div>
              
              <div class="form-group mb-3">
                <label for="repositoryName">リポジトリ名</label>
                <input 
                  type="text" 
                  class="form-control" 
                  id="repositoryName" 
                  v-model="repositoryName"
                  :class="{'is-invalid': validationError}"
                  placeholder="例: my-project"
                  required
                >
                <div v-if="validationError" class="invalid-feedback">
                  {{ validationError }}
                </div>
                <small class="form-text text-muted">
                  リポジトリ名は英数字、ハイフン、アンダースコアのみ使用できます。
                </small>
              </div>
              <button type="submit" class="btn btn-primary" :disabled="!isNameValid || isSubmitting">
                <span v-if="isSubmitting">
                  <span class="spinner-border spinner-border-sm" role="status" aria-hidden="true"></span>
                  作成中...
                </span>
                <span v-else>作成</span>
              </button>
            </form>
          </div>
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
        })
        .catch(error => {
          this.error = `グループ一覧の取得に失敗しました: ${error.message}`;
          this.loadingGroups = false;
        });
    },
    createRepository() {
      // 入力値の検証
      if (!this.repositoryName.trim()) {
        this.validationError = 'リポジトリ名を入力してください';
        return;
      }
      
      // 不正な文字をチェック
      const nameRegex = /^[a-zA-Z0-9_-]+$/;
      if (!nameRegex.test(this.repositoryName)) {
        this.validationError = 'リポジトリ名には英数字、ハイフン、アンダースコアのみ使用できます';
        return;
      }
      
      this.validationError = null;
      this.isSubmitting = true;
      this.error = null;
      
      // APIリクエストを送信
      axios.post('/api/repositories', {
        name: this.repositoryName,
        group: this.selectedGroup
      })
        .then(response => {
          this.isSubmitting = false;
          this.success = `リポジトリ ${this.selectedGroup}/${this.repositoryName} を作成しました！`;
        })
        .catch(error => {
          this.isSubmitting = false;
          if (error.response && error.response.data && error.response.data.error) {
            this.error = error.response.data.error;
          } else {
            this.error = 'リポジトリの作成中にエラーが発生しました: ' + error.message;
          }
        });
    }
  }
});

// アプリケーションをマウント
createRepoApp.mount('#create-repo-app');
