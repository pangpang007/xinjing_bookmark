# 心境书签 API

微信小程序「心境书签」后端：文学推荐、微信登录、分享图、历史记录。

对外域名按现有 Tunnel 约定走 `https://api.soupcircle.xyz/bookjie/*`，容器映射 `8082 -> 8080`。

## 本地运行

```bash
cp .env.example .env
# 填 DATABASE_URL、JWT_SECRET、DEEPSEEK_API_KEY、WECHAT_APP_SECRET、R2_*
bash scripts/download-font.sh
go run .
```

健康检查：`GET http://localhost:8080/health`

## 接口

| 方法 | 路径 | 登录 |
| --- | --- | --- |
| POST | `/api/v1/interpret` | 可选。带 JWT 时写入历史 |
| POST | `/api/v1/login` | 否 |
| POST | `/api/v1/share-image` | 是 |
| GET | `/api/v1/history?page=1&page_size=20` | 是 |

Cloudflare Tunnel 会把完整路径转进来，因此以上接口同时挂在 `/bookjie` 前缀下，例如：

`https://api.soupcircle.xyz/bookjie/api/v1/interpret`

## NAS Docker

```bash
docker compose up -d --build
```

- 监听 `8082`，复用现有 Tunnel：`api.soupcircle.xyz` + `path: /bookjie/*` → `http://localhost:8082`
- Postgres 用 `DATABASE_URL`（Supabase）
- Redis 由 compose 内部提供，不暴露端口

## 待确认项

- **DeepSeek API Key**：必填，否则 `/interpret` 会走朱自清《匆匆》兜底
- **R2 bucket**：建议共用现有 bucket，对象前缀 `bookjie/`，自定义域保持 `https://r2.soupcircle.xyz`
- **小程序 AppSecret**：必填，否则登录失败
- **字体**：`scripts/download-font.sh` 会下载 Noto Sans SC（与思源黑体同族）。Docker 构建时也会尝试下载
- **小程序码**：未发布时把 `WECHAT_ENV_VERSION=develop`；发布后改为 `release`
