package main

// 行为测试：假的未来员工服务（httptest），覆盖登录出链接、status 领 key 只领一次、无 key 不调收费工具、需求同步失败不阻塞。
import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
)

const fakeKey = "spk_1_AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA"

type fake struct {
	confirmed bool
	claimed   bool
	paid      int
	reports   int
}

func (f *fake) handler(t *testing.T) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var body map[string]any
		_ = json.NewDecoder(r.Body).Decode(&body)
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/open/cli/login":
			io.WriteString(w, `{"code":200,"data":{"scene":"1.aaaaaaaaaaaaaaaaaaaaaaaa.secretsecretsecretsecret","auth_url":"https://luluyouai.com/api/v1/open/k/aaaaaaaaaaaaaaaaaaaaaaaa","web_url":"https://shangpai.luluyouai.com/get-key?source=WorkBuddy","industry":"shangpai"}}`)
		case "/open/cli/check":
			if f.confirmed && !f.claimed {
				f.claimed = true
				io.WriteString(w, `{"data":{"状态":"已确认","api_key":"`+fakeKey+`"}}`)
			} else if f.claimed {
				io.WriteString(w, `{"data":{"状态":"已过期"}}`)
			} else {
				io.WriteString(w, `{"data":{"状态":"待扫码"}}`)
			}
		case "/open/mcp", "/open/mcp-trial":
			params, _ := body["params"].(map[string]any)
			if body["method"] == "tools/list" {
				io.WriteString(w, `{"jsonrpc":"2.0","id":1,"result":{"tools":[{"name":"get_video_detail","inputSchema":{"type":"object","properties":{"video_id":{"type":"integer"}}}}]}}`)
				return
			}
			name, _ := params["name"].(string)
			if name == "report_need" {
				f.reports++
				io.WriteString(w, `{"jsonrpc":"2.0","id":1,"result":{"content":[{"type":"text","text":"{\"ok\": true}"}],"isError":false}}`)
				return
			}
			if r.URL.Path == "/open/mcp" && r.Header.Get("Authorization") == "Bearer "+fakeKey {
				f.paid++
				args, _ := json.Marshal(params["arguments"])
				io.WriteString(w, `{"jsonrpc":"2.0","id":1,"result":{"content":[{"type":"text","text":`+strconvQuote(string(args))+`}],"isError":false}}`)
				return
			}
			w.WriteHeader(401)
			io.WriteString(w, `{"jsonrpc":"2.0","id":1,"error":{"code":-32001,"message":"API key 无效"}}`)
		default:
			w.WriteHeader(404)
		}
	})
}

func strconvQuote(s string) string { b, _ := json.Marshal(s); return string(b) }

func capture(t *testing.T, f func() int) (string, int) {
	t.Helper()
	old := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w
	code := f()
	w.Close()
	os.Stdout = old
	var buf bytes.Buffer
	io.Copy(&buf, r)
	return buf.String(), code
}

func setup(t *testing.T) *fake {
	f := &fake{}
	srv := httptest.NewServer(f.handler(t))
	t.Cleanup(srv.Close)
	t.Setenv("WEILAI_API_BASE", srv.URL)
	t.Setenv("WEILAI_HOME", t.TempDir())
	t.Setenv("WEILAI_API_KEY", "")
	return f
}

func TestLoginStatusClaimOnce(t *testing.T) {
	f := setup(t)
	out, code := capture(t, func() int { return run([]string{"auth", "login"}) })
	if code != 0 || !strings.Contains(out, "\nhttps://luluyouai.com/api/v1/open/k/aaaaaaaaaaaaaaaaaaaaaaaa\n") {
		t.Fatalf("login 要单独一行输出认证链接: %q", out)
	}
	if first := strings.Index(out, "https://"); !strings.HasPrefix(out[first:], "https://luluyouai.com/api/v1/open/k/") {
		t.Fatal("第一个 https 链接必须是认证链接（WorkBuddy 取第一个）")
	}
	if _, err := os.Stat(pendingPath()); err != nil {
		t.Fatal("输出链接前要先存 scene")
	}
	out, _ = capture(t, func() int { return run([]string{"auth", "status"}) })
	if !strings.Contains(out, `"logged_in":false`) || !strings.Contains(out, "待扫码") {
		t.Fatalf("未确认应未登录: %s", out)
	}
	f.confirmed = true
	out, code = capture(t, func() int { return run([]string{"auth", "status"}) })
	if code != 0 || !strings.Contains(out, `"logged_in":true`) || strings.Contains(out, fakeKey) {
		t.Fatalf("确认后应登录且不输出 key: %s", out)
	}
	if keyOrEmpty() != fakeKey {
		t.Fatal("key 应存到本机")
	}
	if st, _ := os.Stat(keyPath()); st.Mode().Perm() != 0o600 {
		t.Fatalf("key 文件权限应为 600，现 %v", st.Mode().Perm())
	}
	if _, err := os.Stat(pendingPath()); err == nil {
		t.Fatal("领到后应删掉 scene")
	}
	out, _ = capture(t, func() int { return run([]string{"auth", "status"}) })
	if !strings.Contains(out, `"logged_in":true`) {
		t.Fatal("之后 status 直接读本机 key")
	}
	capture(t, func() int { return run([]string{"auth", "logout"}) })
	if keyOrEmpty() != "" {
		t.Fatal("logout 要删 key")
	}
}

func TestNoKeyNoPaidCall(t *testing.T) {
	f := setup(t)
	_, code := capture(t, func() int { return run([]string{"video", "123"}) })
	if code != 3 || f.paid != 0 {
		t.Fatalf("无 key 应退出码 3 且不调收费工具: code=%d paid=%d", code, f.paid)
	}
}

func TestTypedArgsAndSave(t *testing.T) {
	f := setup(t)
	if _, code := capture(t, func() int { return run([]string{"auth", "spk_1_short", "--save"}) }); code != 2 {
		t.Fatal("格式不对的 key 应拒绝")
	}
	capture(t, func() int { return run([]string{"auth", fakeKey, "--save"}) })
	out, code := capture(t, func() int { return run([]string{"video", "123"}) })
	if code != 0 || f.paid != 1 || !strings.Contains(out, `"video_id": 123`) {
		t.Fatalf("位置参数应按 schema 转整数: %s", out)
	}
}

func TestAnalyzeAnonymousAndNeverBlocks(t *testing.T) {
	f := setup(t)
	out, code := capture(t, func() int {
		return run([]string{"analyze", "--raw-query", "望春园呢", "--context-intent", "看小区"})
	})
	if code != 0 || f.reports != 1 || !strings.Contains(out, "ok") {
		t.Fatalf("无 key 也能同步需求: %s", out)
	}
	t.Setenv("WEILAI_API_BASE", "http://127.0.0.1:1")
	out, code = capture(t, func() int { return run([]string{"analyze", "--raw-query", "x"}) })
	if code != 0 || !strings.Contains(out, `"ok": false`) {
		t.Fatalf("同步失败不阻塞: %d %s", code, out)
	}
}
