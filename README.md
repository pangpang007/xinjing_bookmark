# 心境书签 API

微信小程序「心境书签」后端：文学推荐、微信登录、分享图、历史记录。

对外地址：`https://api.soupcircle.xyz/bookmark`  
Swagger：https://api.soupcircle.xyz/bookmark/swagger

## 本地运行

```bash
cp .env.example .env
bash scripts/download-font.sh
go run .
```

- 健康检查：`GET http://localhost:8080/health`
- Swagger：http://localhost:8080/swagger

## 接口

| 方法 | 路径 | 登录 |
| --- | --- | --- |
| GET | `/health` | 否 |
| POST | `/interpret` | 是。按登录用户每天 3 次（`INTERPRET_DAILY_LIMIT`，0 表示不限制） |
| POST | `/login` | 否 |
| POST | `/share-image` | 是。`image_url` 为本 API 域名，不暴露 R2 |
| GET | `/share-images/{uuid}.jpg` | 否。供小程序 `wx.downloadFile` / `<image>` |
| GET | `/history?page=1&page_size=20` | 是 |

以上接口同时挂在 `/bookmark` 前缀下。小程序请调用：

`https://api.soupcircle.xyz/bookmark/interpret`

## NAS Docker

```bash
docker compose up -d --build
```

容器名是 `bookmark-api`，内部端口 `8080`，宿主机映射 `8082`。网关 `soupcircle-gateway` 会把 `/bookmark*` 转到这个容器，需要和网关在同一 Docker 网络：

```bash
docker inspect bookmark-api --format '{{range $k,$v := .NetworkSettings.Networks}}{{$k}} {{end}}'
docker network connect <书签网络名> soupcircle-gateway
# 或
docker network connect soupcircle-edge bookmark-api
```

验证：

```bash
curl -sS https://api.soupcircle.xyz/bookmark/health
```

## 待确认项

- **DeepSeek API Key**：必填，否则 `/interpret` 会走朱自清《匆匆》兜底
- **R2**：bucket `xinjing-bookmark`，对象前缀 `bookmark/`。小程序只访问 `https://api.soupcircle.xyz/bookmark/share-images/...`，请把 `api.soupcircle.xyz` 配进 downloadFile 合法域名
- **小程序 AppSecret**：必填，否则登录失败
- **字体**：`scripts/download-font.sh` 或 Docker 构建时自动下载
- **小程序码**：未发布时 `WECHAT_ENV_VERSION=develop`；发布后改为 `release`
