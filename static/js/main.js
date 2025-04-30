/**
 * Guilty共通ユーティリティ関数
 */

// グローバル名前空間汚染を避けるためにオブジェクトにまとめる
const GuiltyUtils = {
  /**
   * グループ名とリポジトリ名をエンコードしたパスを生成する内部ヘルパー関数
   * @param {string} groupName - グループ名
   * @param {string} repoName - リポジトリ名
   * @returns {string} エンコード済みのパス
   * @private
   */
  _getEncodedPath(groupName, repoName) {
    return `${encodeURIComponent(groupName)}/${encodeURIComponent(repoName)}`;
  },

  /**
   * グループ名とリポジトリ名からリポジトリ詳細ページのURLを生成
   * @param {string} groupName - グループ名
   * @param {string} repoName - リポジトリ名
   * @returns {string} リポジトリ詳細ページのURL
   */
  getRepositoryUrl(groupName, repoName) {
    return `/repository/${this._getEncodedPath(groupName, repoName)}`;
  },

  /**
   * グループ名とリポジトリ名からAPI用のリポジトリパスを生成
   * @param {string} groupName - グループ名
   * @param {string} repoName - リポジトリ名
   * @returns {string} APIで使用するリポジトリパス
   */
  getApiRepositoryPath(groupName, repoName) {
    return `/api/repository/${this._getEncodedPath(groupName, repoName)}`;
  },

  /**
   * グループ名、リポジトリ名、ファイルパスからAPI用のファイルパスを生成
   * @param {string} groupName - グループ名
   * @param {string} repoName - リポジトリ名
   * @param {string} filePath - ファイルパス（オプション）
   * @returns {string} APIで使用するファイルパス
   */
  getApiFilePath(groupName, repoName, filePath) {
    const basePath = `/api/file/${this._getEncodedPath(groupName, repoName)}`;
    if (!filePath) return basePath;
    
    // パスの各部分を保持したままURLを構築
    const parts = filePath.split('/');
    const urlPath = parts.map(part => encodeURIComponent(part)).join('/');
    return `${basePath}/${urlPath}`;
  },

  /**
   * グループ名、リポジトリ名、ディレクトリパスからAPI用のディレクトリパスを生成
   * @param {string} groupName - グループ名
   * @param {string} repoName - リポジトリ名
   * @param {string} dirPath - ディレクトリパス（オプション）
   * @returns {string} APIで使用するディレクトリパス
   */
  getApiDirectoryPath(groupName, repoName, dirPath) {
    const basePath = `/api/directory/${this._getEncodedPath(groupName, repoName)}`;
    if (!dirPath) return basePath;
    
    // パスの各部分を保持したままURLを構築
    const parts = dirPath.split('/');
    const urlPath = parts.map(part => encodeURIComponent(part)).join('/');
    return `${basePath}/${urlPath}`;
  },

  /**
   * グループ名を指定してリポジトリ一覧APIのURLを生成
   * @param {string} groupName - グループ名
   * @returns {string} リポジトリ一覧APIのURL
   */
  getRepositoriesApiUrl(groupName) {
    return `/api/repositories?group=${encodeURIComponent(groupName)}`;
  },

  /**
   * グループ名を指定してリポジトリ一覧ページのURLを生成
   * @param {string} groupName - グループ名
   * @returns {string} リポジトリ一覧ページのURL
   */
  getRepositoriesPageUrl(groupName) {
    return `/?group=${encodeURIComponent(groupName)}`;
  },

  /**
   * グループ名を指定して新規リポジトリ作成ページのURLを生成
   * @param {string} groupName - グループ名
   * @returns {string} 新規リポジトリ作成ページのURL
   */
  getCreateRepositoryUrl(groupName) {
    return `/create-repository?group=${encodeURIComponent(groupName)}`;
  }
};

// グローバルスコープに公開（他のスクリプトから利用可能に）
window.GuiltyUtils = GuiltyUtils;