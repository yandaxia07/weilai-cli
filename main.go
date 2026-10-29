// weilai：未来员工命令行（WorkBuddy CLI 连接器用，也可单独在终端里用）。
//
// 登录（与 WorkBuddy cli.json 对应）：
//
//	weilai auth login    出认证链接（WorkBuddy 用 authQrModal 画成二维码，微信扫码在小程序里确认）
//	weilai auth status   输出 {"logged_in": true|false}；有待确认的扫码就去服务端领 key，领到存本机
//	weilai auth logout   删掉本机 key
//	weilai auth <KEY> --save   手动保存 key（与贝壳 beike auth --save 同一用法）
//
// 业务：每个子命令对应未来员工 MCP 的一个工具，参数写成 --名字 值（值以 @ 开头表示读文件），结果原样输出 JSON。
// 凭证只存本机 ~/.weilai/API_KEY（权限 600），不进安装目录；本程序不含任何密钥。
package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"
)

var version = "dev" // 构建时 -ldflags "-X main.version=..." 注入

const defaultBase = "https://luluyouai.com/api/v1"

var keyRe = regexp.MustCompile(`^spk_\d{1,4}_[A-Za-z0-9]{32}$`)

// 子命令 → MCP 工具；第一个位置参数填到哪个参数名（没有则为空）
var shortcuts = map[string][2]string{
	"balance":  {"get_balance", ""},
	"topics":   {"list_topics", ""},
	"topic":    {"get_topic_detail", "topic_id"},
	"accounts": {"search_benchmark_accounts", "keyword"},
	"videos":   {"list_benchmark_videos", "keyword"},
	"video":    {"get_video_detail", "video_id"},
	"poster":   {"generate_moment_poster", "title"},
	"poster-get": {"get_moment_poster", "task_id"},
	"note":     {"generate_note_images", "copy"},
	"note-get": {"get_note_images", "draft_id"},
}

func main() {
	os.Exit(run(os.Args[1:]))
}

func run(args []string) int {
	if len(args) == 0 || args[0] == "-h" || args[0] == "--help" || args[0] == "help" {
		usage()
		return 0
	}
	switch args[0] {
	case "--version", "-v", "version":
		fmt.Println(version)
		return 0
	case "login":
		return authLogin(args[1:])
	case "auth":
		return auth(args[1:])
	case "analyze":
		return analyze(args[1:])
	case "tools":
		return printResult(rpc(keyOrEmpty(), "tools/list", map[string]any{}))
	case "call":
		if len(args) < 2 {
			return fail(2, "用法：weilai call <工具名> [--参数 值 ...]")
		}
		return callTool(args[1], "", args[2:])
	case "doctor":
		return doctor()
	}
	if sc, ok := shortcuts[args[0]]; ok {
		return callTool(sc[0], sc[1], args[1:])
	}
	return fail(2, "不认识的命令："+args[0]+"（weilai --help 看用法）")
}

func usage() {
	fmt.Print(`weilai ` + version + ` —— 未来员工命令行

登录：
  weilai auth login [--industry 商拍] [--source WorkBuddy]   输出认证链接，微信扫码在小程序确认
  weilai auth status                                         {"logged_in": true|false}
  weilai auth logout                                         删除本机 key
  weilai auth <KEY> --save                                   手动保存 key
需求同步：
  weilai analyze --raw-query "用户原话" --context-intent "需求理解" [--session-id ID] [--industry 商拍]
查询与生成（结果为 JSON；有结果、生成成功才扣积分）：
  weilai balance                      积分余额（免费）
  weilai topics [--module hot|industry] [--limit 10]
  weilai topic <topic_id>
  weilai accounts [关键词] [--platform 视频号] [--limit 10]
  weilai videos [关键词] [--platform 视频号] [--days 30] [--sort likes] [--account-id N]
  weilai video <video_id>
  weilai poster <标题> [--body 文案] [--size wechat|xhs|square|banner]
  weilai poster-get <task_id>
  weilai note <文案或 @文件> [--knowledge @资料.txt] [--page-count 4] [--theme classic|klein]
  weilai note-get <draft_id>
  weilai tools                        列出全部工具
  weilai call <工具名> [--参数 值]     直接调用任意工具
其它：weilai doctor 检查连接与 key；weilai --version
`)
}

// ==================== 本机存储 ====================

func home() string {
	if h := os.Getenv("WEILAI_HOME"); h != "" {
		return h
	}
	d, err := os.UserHomeDir()
	if err != nil {
		d = "."
	}
	return filepath.Join(d, ".weilai")
}

func keyPath() string     { return filepath.Join(home(), "API_KEY") }
func pendingPath() string { return filepath.Join(home(), "pending.json") }

func base() string {
	if b := os.Getenv("WEILAI_API_BASE"); b != "" {
		return strings.TrimRight(b, "/")
	}
	return defaultBase
}

// 环境变量优先（与贝壳 BEIKE_MCP_API_KEY 同一顺序），其次本机文件
func keyOrEmpty() string {
	if k := strings.TrimSpace(os.Getenv("WEILAI_API_KEY")); keyRe.MatchString(k) {
		return k
	}
	b, err := os.ReadFile(keyPath())
	if err != nil {
		return ""
	}
	k := strings.TrimSpace(string(b))
	if keyRe.MatchString(k) {
		return k
	}
	return ""
}

func saveKey(k string) error {
	if err := os.MkdirAll(home(), 0o700); err != nil {
		return err
	}
	tmp := keyPath() + ".tmp"
	if err := os.WriteFile(tmp, []byte(k+"\n"), 0o600); err != nil {
		return err
	}
	return os.Rename(tmp, keyPath())
}

// ==================== HTTP ====================

var client = &http.Client{Timeout: 60 * time.Second}

func postJSON(url string, body any, key string) (map[string]any, int, error) {
	b, _ := json.Marshal(body)
	req, _ := http.NewRequest("POST", url, bytes.NewReader(b))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("User-Agent", "weilai-cli/"+version)
	if key != "" {
		req.Header.Set("Authorization", "Bearer "+key)
	}
	resp, err := client.Do(req)
	if err != nil {
		return nil, 0, err
	}
	defer resp.Body.Close()
	raw, _ := io.ReadAll(io.LimitReader(resp.Body, 8<<20))
	out := map[string]any{}
	if len(raw) > 0 {
		if err := json.Unmarshal(raw, &out); err != nil {
			return nil, resp.StatusCode, fmt.Errorf("服务返回不是 JSON（%d）", resp.StatusCode)
		}
	}
	return out, resp.StatusCode, nil
}

type rpcResult struct {
	data  any
	isErr bool
	err   error
	code  int
}

// rpc：有 key 走 /open/mcp，没有走匿名的 /open/mcp-trial（只开放扫码、上报与免费试用）
func rpc(key, method string, params map[string]any) rpcResult {
	url := base() + "/open/mcp"
	if key == "" {
		url = base() + "/open/mcp-trial"
	}
	out, status, err := postJSON(url, map[string]any{"jsonrpc": "2.0", "id": 1, "method": method, "params": params}, key)
	if err != nil {
		return rpcResult{err: err, code: 4}
	}
	if e, ok := out["error"].(map[string]any); ok {
		msg, _ := e["message"].(string)
		code := 4
		if status == 401 {
			code = 3
		}
		return rpcResult{err: errors.New(msg), code: code}
	}
	res, _ := out["result"].(map[string]any)
	if method != "tools/call" {
		return rpcResult{data: res}
	}
	isErr, _ := res["isError"].(bool)
	var data any = res
	if content, ok := res["content"].([]any); ok && len(content) > 0 {
		if c, ok := content[0].(map[string]any); ok {
			txt, _ := c["text"].(string)
			var v any
			if json.Unmarshal([]byte(txt), &v) == nil {
				data = v
			} else {
				data = map[string]any{"message": txt}
			}
		}
	}
	return rpcResult{data: data, isErr: isErr}
}

func printResult(r rpcResult) int {
	if r.err != nil {
		return fail(r.code, r.err.Error())
	}
	enc := json.NewEncoder(os.Stdout)
	enc.SetEscapeHTML(false)
	enc.SetIndent("", "  ")
	_ = enc.Encode(r.data)
	if r.isErr {
		return 1
	}
	return 0
}

func fail(code int, msg string) int {
	fmt.Fprintln(os.Stderr, "weilai: "+msg)
	return code
}

// ==================== 参数 ====================

// --a-b 值 → a_b；值以 @ 开头读文件；第一个不带 -- 的参数填到 positional 指定的参数名
func parseArgs(args []string, positional string) (map[string]string, error) {
	out := map[string]string{}
	for i := 0; i < len(args); i++ {
		a := args[i]
		if strings.HasPrefix(a, "--") {
			name := strings.ReplaceAll(strings.TrimPrefix(a, "--"), "-", "_")
			if eq := strings.Index(name, "="); eq > 0 {
				out[name[:eq]] = name[eq+1:]
				continue
			}
			if i+1 >= len(args) {
				return nil, fmt.Errorf("--%s 缺少值", name)
			}
			out[name] = args[i+1]
			i++
			continue
		}
		if positional == "" {
			return nil, fmt.Errorf("多余的参数：%s", a)
		}
		if _, ok := out[positional]; ok {
			return nil, fmt.Errorf("多余的参数：%s", a)
		}
		out[positional] = a
	}
	for k, v := range out {
		if strings.HasPrefix(v, "@") && len(v) > 1 {
			b, err := os.ReadFile(v[1:])
			if err != nil {
				return nil, fmt.Errorf("读不到文件 %s", v[1:])
			}
			out[k] = string(b)
		}
	}
	return out, nil
}

// 按工具的 inputSchema 把字符串参数转成整数 / 布尔
func typed(key, tool string, raw map[string]string) map[string]any {
	types := map[string]string{}
	r := rpc(key, "tools/list", map[string]any{})
	if m, ok := r.data.(map[string]any); ok {
		if list, ok := m["tools"].([]any); ok {
			for _, t := range list {
				tm, _ := t.(map[string]any)
				if tm["name"] != tool {
					continue
				}
				schema, _ := tm["inputSchema"].(map[string]any)
				props, _ := schema["properties"].(map[string]any)
				for name, p := range props {
					pm, _ := p.(map[string]any)
					if s, ok := pm["type"].(string); ok {
						types[name] = s
					}
				}
			}
		}
	}
	out := map[string]any{}
	for k, v := range raw {
		switch types[k] {
		case "integer":
			if n, err := strconv.Atoi(v); err == nil {
				out[k] = n
				continue
			}
		case "boolean":
			out[k] = v == "true" || v == "1"
			continue
		}
		out[k] = v
	}
	return out
}

func callTool(tool, positional string, args []string) int {
	raw, err := parseArgs(args, positional)
	if err != nil {
		return fail(2, err.Error())
	}
	key := keyOrEmpty()
	if key == "" && tool != "get_login_qr" && tool != "check_login_qr" && tool != "report_need" && !strings.HasPrefix(tool, "trial_") && !strings.HasPrefix(tool, "get_trial_") {
		return fail(3, "还没有 API key，本次没有调用、不扣积分。运行 weilai auth login 扫码获取，或网页登录后 weilai auth <KEY> --save")
	}
	return printResult(rpc(key, "tools/call", map[string]any{"name": tool, "arguments": typed(key, tool, raw)}))
}

// ==================== 登录 ====================

func auth(args []string) int {
	if len(args) == 0 {
		return fail(2, "用法：weilai auth login|status|logout|<KEY> --save")
	}
	switch args[0] {
	case "login":
		return authLogin(args[1:])
	case "status":
		return authStatus()
	case "logout":
		_ = os.Remove(keyPath())
		_ = os.Remove(pendingPath())
		fmt.Println(`{"logged_in": false}`)
		return 0
	}
	// weilai auth <KEY> --save
	k := strings.TrimSpace(args[0])
	if !keyRe.MatchString(k) {
		return fail(2, "这串 key 好像不完整，应以 spk_ 开头（不会显示你输入的内容）")
	}
	if err := saveKey(k); err != nil {
		return fail(4, "保存失败："+err.Error())
	}
	fmt.Println("已保存好，可以开始了")
	return 0
}

func authLogin(args []string) int {
	raw, err := parseArgs(args, "")
	if err != nil {
		return fail(2, err.Error())
	}
	src := raw["source"]
	if src == "" {
		src = "WorkBuddy"
	}
	out, _, err := postJSON(base()+"/open/cli/login", map[string]any{"industry": raw["industry"], "source": src}, "")
	if err != nil {
		return fail(4, "连不上未来员工服务："+err.Error())
	}
	data, _ := out["data"].(map[string]any)
	scene, _ := data["scene"].(string)
	url, _ := data["auth_url"].(string)
	if scene == "" || url == "" {
		msg, _ := out["message"].(string)
		return fail(4, "出码失败："+msg)
	}
	// 先存 scene 再输出链接：WorkBuddy 拿到链接会立刻结束本进程
	if err := os.MkdirAll(home(), 0o700); err != nil {
		return fail(4, err.Error())
	}
	p, _ := json.Marshal(map[string]any{"scene": scene, "created": time.Now().Unix(), "industry": data["industry"]})
	if err := os.WriteFile(pendingPath(), p, 0o600); err != nil {
		return fail(4, err.Error())
	}
	fmt.Println("请用微信扫码，在小程序里点「确认授权」：")
	fmt.Println(url)
	if web, _ := data["web_url"].(string); web != "" {
		fmt.Println("（也可以网页登录后复制 key：" + strings.TrimPrefix(web, "https://") + "）")
	}
	return 0
}

// status：只读状态；有待确认的扫码时去服务端领一次 key（官方「服务端 Token 存储」方案：由 status 拉取到本地）
func authStatus() int {
	say := func(ok bool, state string) int {
		b, _ := json.Marshal(map[string]any{"logged_in": ok, "state": state})
		fmt.Println(string(b))
		return 0
	}
	if keyOrEmpty() != "" {
		return say(true, "已登录")
	}
	b, err := os.ReadFile(pendingPath())
	if err != nil {
		return say(false, "未登录")
	}
	var p struct {
		Scene   string `json:"scene"`
		Created int64  `json:"created"`
	}
	if json.Unmarshal(b, &p) != nil || p.Scene == "" || time.Now().Unix()-p.Created > 900 {
		_ = os.Remove(pendingPath())
		return say(false, "已过期")
	}
	out, _, err := postJSON(base()+"/open/cli/check", map[string]any{"scene": p.Scene}, "")
	if err != nil {
		return say(false, "网络异常")
	}
	data, _ := out["data"].(map[string]any)
	state, _ := data["状态"].(string)
	if k, _ := data["api_key"].(string); keyRe.MatchString(k) {
		if err := saveKey(k); err != nil {
			return say(false, "保存失败")
		}
		_ = os.Remove(pendingPath())
		return say(true, "已登录")
	}
	if state == "已过期" || state == "已取消" {
		_ = os.Remove(pendingPath())
	}
	return say(false, state)
}

// ==================== 需求同步与检查 ====================

func analyze(args []string) int {
	raw, err := parseArgs(args, "")
	if err != nil {
		return fail(2, err.Error())
	}
	if raw["raw_query"] == "" {
		return fail(2, "--raw-query 必填：用户本轮原话")
	}
	params := map[string]any{"raw_query": raw["raw_query"], "context_intent": raw["context_intent"],
		"session_id": raw["session_id"], "industry": raw["industry"], "source": "WorkBuddy"}
	if s := raw["source"]; s != "" {
		params["source"] = s
	}
	// 失败不阻塞回答：最多重试一次，仍失败输出 ok:false、退出码 0
	for i := 0; i < 2; i++ {
		r := rpc(keyOrEmpty(), "tools/call", map[string]any{"name": "report_need", "arguments": params})
		if r.err == nil && !r.isErr {
			return printResult(r)
		}
	}
	fmt.Println(`{"ok": false}`)
	return 0
}

func doctor() int {
	key := keyOrEmpty()
	out := map[string]any{"version": version, "api": base(), "has_key": key != ""}
	if key != "" {
		r := rpc(key, "tools/call", map[string]any{"name": "get_balance", "arguments": map[string]any{}})
		out["key_ok"] = r.err == nil && !r.isErr
	}
	enc := json.NewEncoder(os.Stdout)
	enc.SetEscapeHTML(false)
	enc.SetIndent("", "  ")
	_ = enc.Encode(out)
	return 0
}
