
#include <cctype>
#include <cerrno>
#include <cstdio>
#include <cstdlib>
#include <cstring>
#include <string>
#include <sys/stat.h>
#include <unistd.h>
#include <vector>
#include <optional>
#include "jstream.h" // https://github.com/soramimi/jstream

namespace guilty {

struct CacheFile {
	struct Item {
		std::string key;
		std::string author;
		std::string date;
		std::string message;
		uint64_t cachedAt;
	};
	std::vector<Item> items;
};

std::optional<CacheFile> load_cache_file(char const *path)
{
	std::vector<char> buf;
	{
		FILE *fp = fopen(path, "rb");
		if (!fp) {
			std::fprintf(stderr, "failed to open %s: %s\n", path, std::strerror(errno));
			return {};
		}
		fseek(fp, 0, SEEK_END);
		long size = ftell(fp);
		if (size < 0) {
			std::fprintf(stderr, "failed to get size of %s: %s\n", path, std::strerror(errno));
			fclose(fp);
			return {};
		}
		buf.reserve(size + 1);
		buf.resize(size);
		fseek(fp, 0, SEEK_SET);
		if (fread(buf.data(), 1, size, fp) != static_cast<size_t>(size)) {
			std::fprintf(stderr, "failed to read %s: %s\n", path, std::strerror(errno));
			fclose(fp);
			return {};
		}
		fclose(fp);
	}
	if (buf.empty()) {
		std::fprintf(stderr, "file %s is empty\n", path);
		return {};
	}
	buf.push_back(0);
	
	std::vector<CacheFile::Item> items;
	jstream::Reader r(buf.data());
	while (r.next()) {
		if (r.match_start_object("{entries{*{**")) {
			CacheFile::Item item;
			item.key = r.key();
			r.nest([&](){
				if (r.match("@author")) {
					item.author = r.string();
				} else if (r.match("@date")) {
					item.date = r.string();
				} else if (r.match("@message")) {
					item.message = r.string();
				} else if (r.match("@cachedAt")) {
					item.cachedAt = (uint64_t)r.number();
				}
			});
			items.push_back(item);
		}
	}
	
	CacheFile ret;
	ret.items = std::move(items);
	return ret;
}

bool save_cache_file(char const *path, CacheFile const &cache)
{
	jstream::Writer w;

	w.object({}, [&](){
		w.object("entries", [&](){
			for (CacheFile::Item const &item : cache.items) {
				w.object(item.key, [&](){
					w.string("author", item.author);
					w.string("date", item.date);
					w.string("message", item.message);
					w.number("cachedAt", item.cachedAt);
				});
			}
		});
	});

	std::string str = w;

	std::string tmp = std::string(path) + ".tmp";
	FILE *fp = fopen(tmp.c_str(), "wb");
	if (!fp) {
		std::fprintf(stderr, "failed to open %s for writing: %s\n", tmp.c_str(), std::strerror(errno));
		return false;
	}
	if (fwrite(str.data(), 1, str.size(), fp) != str.size()) {
		std::fprintf(stderr, "failed to write %s: %s\n", tmp.c_str(), std::strerror(errno));
		fclose(fp);
		unlink(tmp.c_str());
		return false;
	}
	if (fclose(fp) != 0) {
		std::fprintf(stderr, "failed to close %s: %s\n", tmp.c_str(), std::strerror(errno));
		unlink(tmp.c_str());
		return false;
	}
	if (std::rename(tmp.c_str(), path) != 0) {
		std::fprintf(stderr, "failed to rename %s to %s: %s\n", tmp.c_str(), path, std::strerror(errno));
		unlink(tmp.c_str());
		return false;
	}

	return true;
}

} // namespace guilty

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
	if (0) { // debug
		char const *path = "/mnt/git/git/.guilty-cache.json";
		auto opt = guilty::load_cache_file(path);
		if (opt) {
			guilty::CacheFile const &cache = *opt;
			guilty::save_cache_file("/tmp/guilty-cache.json", cache);
		}
		return 0;
	}
	
	std::string cmd, arg;

	if (0) { // debug
		std::string s;
		for (int i = 0; i < argc; i++) {
			if (!s.empty()) {
				s += ' ';
			}
			s += argv[i];
		}
		fprintf(stderr, "--- %s\n", s.c_str());
	}
	
	if (argc == 3 && strcmp(argv[1], "-c") == 0) {
		if (!parse_command(argv[2], cmd, arg)) {
			std::fprintf(stderr, "fatal: invalid command format\n");
			return 1;
		}
	} else {
		const char *cmd_env = std::getenv("SSH_ORIGINAL_COMMAND");
		if (!cmd_env || !*cmd_env) {
			std::fprintf(stderr, "fatal: invalid command specified\n");
			return 1;
		}
	
		if (!parse_command(cmd_env, cmd, arg)) {
			std::fprintf(stderr, "fatal: invalid command format\n");
			return 1;
		}
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

