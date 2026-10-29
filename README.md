# weilai —— 未来员工命令行

「未来员工」的命令行工具：在 WorkBuddy、Claude Code、Codex 等 AI 工作台里，用微信扫码领取 API key，然后查询所在行业的选题、对标账号、爆款视频与口播文案，生成朋友圈海报和小红书多图笔记。查询有结果、生成成功才按次扣积分，查余额免费。

本仓库只放命令行代码与安装脚本，不含任何密钥。

## 安装

macOS / Linux：

```bash
curl -fsSL https://raw.githubusercontent.com/yandaxia07/weilai-cli/main/install.sh | bash
```

Windows（PowerShell）：

```powershell
irm https://raw.githubusercontent.com/yandaxia07/weilai-cli/main/install.ps1 | iex
```

安装脚本读取 `releases/manifest.json`（读不到 GitHub 时自动改用国内镜像 luluyouai.com/downloads/weilai-cli/），下载本平台的发布包并强制校验 sha256，校验不过直接退出。支持 macOS（arm64、x64）、Linux（x64、arm64）、Windows（x64）。

## 获取 API key

```bash
weilai auth login        # 输出认证链接；在 WorkBuddy 里会直接弹出二维码
```

用微信扫码，在「未来员工」小程序里点「确认授权」（没注册的先用手机号注册，送 100 积分）。确认后：

```bash
weilai auth status       # {"logged_in":true,...}，key 已自动保存到 ~/.weilai/API_KEY（权限 600）
```

也可以网页登录后复制 key，再保存：`weilai auth <KEY> --save`。环境变量 `WEILAI_API_KEY` 优先于本机文件。

## 常用命令

| 命令 | 说明 |
|---|---|
| `weilai balance` | 积分余额（免费） |
| `weilai topics --module hot` | 今日选题榜 |
| `weilai topic <topic_id>` | 选题详情（口播稿、数据、依据） |
| `weilai accounts 法拍 --platform 视频号` | 找对标账号 |
| `weilai videos --days 30 --sort likes` | 近期爆款视频 |
| `weilai video <video_id>` | 视频详情与口播文案 |
| `weilai poster "中秋全场八折"` / `weilai poster-get <task_id>` | 朋友圈海报 |
| `weilai note @笔记.txt` / `weilai note-get <draft_id>` | 小红书多图笔记 |
| `weilai analyze --raw-query "原话" --context-intent "需求理解"` | 同步本轮需求（失败不阻塞） |
| `weilai tools` / `weilai call <工具> --参数 值` | 列出 / 直接调用任意工具 |

结果一律输出 JSON。退出码：0 成功，1 业务失败（看输出说明），2 参数错误，3 没有或无效的 API key（不会调用收费工具），4 网络或服务异常。

## 协议

使用前请阅读《未来员工 AI 开放服务协议》与《未来员工 AI 开放服务隐私政策》（上线后在 luluyouai.com 公布）。
