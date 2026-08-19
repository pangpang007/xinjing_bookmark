# 心境书签 API

微信小程序「心境书签」后端：文学推荐、微信登录、分享图、历史记录。

对外地址：`https://api.soupcircle.xyz/bookmark`  
Swagger：https://api.soupcircle.xyz/bookmark/swagger

## 本地运行

```bash
cp .env.example .env
go run .
```

- 健康检查：`GET http://localhost:8080/health`
- Swagger：http://localhost:8080/swagger

## 接口

| 方法 | 路径 | 登录 |
| --- | --- | --- |
| GET | `/health` | 否 |
| POST | `/interpret` | 是。调模型前走微信内容安全 2.0；不通过返回 1005，不占次数。每天成功 3 次（`INTERPRET_DAILY_LIMIT`，0 表示不限制） |
| POST | `/login` | 否 |
| POST | `/share-image` | 是。multipart 上传小程序画好的图，返回本 API 域名 `image_url` |
| GET | `/share-images/{uuid}.jpg` | 否。供小程序 `wx.downloadFile` / `<image>` |
| GET | `/wxacode` | 是。返回 `wxacode_url`，码缓存到 R2 |
| GET | `/wxacode/share.png` | 否。小程序码 PNG，供 canvas 加载 |
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
- **R2**：bucket `xinjing-bookmark`，对象前缀 `bookmark/`。小程序只访问 `https://api.soupcircle.xyz/bookmark/...`，请把 `api.soupcircle.xyz` 分别配进 request、downloadFile、**uploadFile** 合法域名（三份名单是分开的）
- **小程序 AppSecret**：必填，否则登录 / 小程序码失败
- **小程序码**：未发布时 `WECHAT_ENV_VERSION=develop`；发布后改为 `release`。page 默认 `pages/index/index`
