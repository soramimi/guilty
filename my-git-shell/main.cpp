
#include <cctype>
#include <cerrno>
#include <cstdio>
#include <cstdlib>
#include <cstring>
#include <string>
#include <sys/stat.h>
#include <unistd.h>
#include <vector>

static bool is_allowed_command(const std::string &cmd)
{
	static const char *allowed[] = {
		"git-receive-pack",
		"git-upload-pack",
		"git-upload-archive",
	};
	for (const char *a : allowed) {
		if (cmd == a) return true;
	}
	return false;
}

static std::string trim(const std::string &s)
{
	size_t i = 0;
	while (i < s.size() && std::isspace(static_cast<unsigned char>(s[i]))) i++;
	size_t j = s.size();
	while (j > i && std::isspace(static_cast<unsigned char>(s[j - 1]))) j--;
	return s.substr(i, j - i);
}

static bool is_safe_path(const std::string &path)
{
	if (path.empty()) return false;
	if (path[0] != '/') return false;
	for (char c : path) {
		unsigned char u = static_cast<unsigned char>(c);
		if (std::iscntrl(u)) return false;
		if (c == ';' || c == '|' || c == '&' || c == '$' || c == '`' ||
		    c == '<' || c == '>' || c == '(' || c == ')' || c == '{' || c == '}' ||
		    c == '*' || c == '?' || c == '[' || c == ']' || c == '\\' || c == '"') {
			return false;
		}
	}
	// パスコンポーネントが ".." なら拒否（traversal 防止）
	size_t start = 0;
	while (start <= path.size()) {
		size_t end = path.find('/', start);
		if (end == std::string::npos) end = path.size();
		if (end - start == 2 && path[start] == '.' && path[start + 1] == '.') {
			return false;
		}
		start = end + 1;
	}
	return true;
}

static bool directory_exists(const std::string &path)
{
	struct stat st;
	return stat(path.c_str(), &st) == 0 && S_ISDIR(st.st_mode);
}

static bool parse_command(const std::string &input, std::string &cmd, std::string &arg)
{
	std::string s = trim(input);
	if (s.empty()) return false;

	size_t pos = s.find(' ');
	if (pos == std::string::npos) {
		cmd = s;
		arg.clear();
		return true;
	}

	cmd = s.substr(0, pos);
	arg = trim(s.substr(pos + 1));

	// シングルクォートを取り除く
	if (arg.size() >= 2 && arg.front() == '\'' && arg.back() == '\'') {
		arg = arg.substr(1, arg.size() - 2);
	}

	return true;
}

int main(int argc, char **argv)
{
	const char *cmd_env = std::getenv("SSH_ORIGINAL_COMMAND");
	if (!cmd_env || !*cmd_env) {
		std::fprintf(stderr, "fatal: SSH_ORIGINAL_COMMAND is not set\n");
		return 1;
	}

	std::string cmd, arg;
	if (!parse_command(cmd_env, cmd, arg)) {
		std::fprintf(stderr, "fatal: invalid command format\n");
		return 1;
	}

	if (!is_allowed_command(cmd)) {
		std::fprintf(stderr, "fatal: command not allowed: %s\n", cmd.c_str());
		return 1;
	}

	if (arg.empty()) {
		std::fprintf(stderr, "fatal: missing repository argument\n");
		return 1;
	}

	if (!is_safe_path(arg)) {
		std::fprintf(stderr, "fatal: unsafe repository path: %s\n", arg.c_str());
		return 1;
	}

	if (!directory_exists(arg)) {
		std::fprintf(stderr, "fatal: repository does not exist: %s\n", arg.c_str());
		return 1;
	}

	std::vector<char *> exec_args;
	exec_args.push_back(const_cast<char *>(cmd.c_str()));
	exec_args.push_back(const_cast<char *>(arg.c_str()));
	exec_args.push_back(nullptr);

	execvp(cmd.c_str(), exec_args.data());

	std::fprintf(stderr, "fatal: failed to execute %s: %s\n", cmd.c_str(), std::strerror(errno));
	return 127;
}

