# Sub2API Fork 精简维护手册

> 用途：同步上游、保护本 fork 定制、备份/发布/回滚。历史细节以 Git 记录为准，本文只保留可执行规则。

## 1. 当前状态

| 项目 | 当前值 |
| --- | --- |
| 仓库 | `/home/aihub/Peter_ws/sub2api` |
| 生产分支 | `custom/gallery` |
| 上游 | `upstream/main` = `e316ebf52838` (`v0.1.151-17-ge316ebf5`) |
| 定制源码提交 | `e1da51f13363` |
| 当前仓库 HEAD | 以 `git rev-parse --short=12 HEAD` 为准 |
| 生产镜像 | `sub2api-custom:20260712-upstream-v0151-e1da51f1` |
| Compose 项目 | `peter-sub2api` |
| 本机入口 | `http://127.0.0.1:18080` |
| 公网入口 | `https://api.peterai.cc.cd` |
| 数据 volume | `peter-sub2api_sub2api_data` |
| 最近发布备份 | `/home/aihub/Peter_ws/sub2api-backups/20260712_011100` |

快速确认：

```bash
cd /home/aihub/Peter_ws/sub2api
git status --short --branch
git log -3 --oneline --decorate
grep -E '^(COMPOSE_PROJECT_NAME|SUB2API_IMAGE|BIND_HOST|SERVER_PORT|GATEWAY_IMAGE_STREAM_DATA_INTERVAL_TIMEOUT)=' deploy/.env
sg docker -c 'docker compose -f deploy/docker-compose.yml --env-file deploy/.env ps'
curl -sS http://127.0.0.1:18080/health
curl -sS https://api.peterai.cc.cd/health
```

## 2. 分支与上游同步

### 2.1 分支规则

- `custom/gallery`：唯一生产源码分支，直接 merge `upstream/main`。
- `main`：可作为干净上游镜像，不承载定制，不用于生产。
- `gpt55-defaults`：历史分支，不是发布来源。
- 生产分支使用 merge，不 rebase，避免改写已发布历史。

远端：

```text
upstream  https://github.com/Wei-Shaw/sub2api.git
origin    git@github.com:Bingtao-Wang/sub2api.git
```

### 2.2 标准同步流程

```bash
cd /home/aihub/Peter_ws/sub2api
git fetch upstream --prune
git fetch origin --prune
git checkout custom/gallery
git status --short --branch

# 合并前备份分支，名称按实际上游版本调整
git branch custom/gallery-backup-YYYYMMDD-before-vXXXX custom/gallery
git merge --no-edit upstream/main
```

冲突处理原则：

1. 优先吸收上游安全修复、API 变更和迁移。
2. 保留第 3 节列出的所有定制不变量。
3. 同一段逻辑两边都修改时，以上游新结构为基础，重新套入定制逻辑。
4. 不要为了解决冲突删除自定义迁移、路由、计费或 i18n 完整性测试。

必须中止时：

```bash
git merge --abort
```

验证后推送：

```bash
git diff --check
git status --short --branch
git log --oneline --left-right origin/custom/gallery...custom/gallery | head -120
git push origin custom/gallery
```

## 3. 定制功能不变量

每次同步上游后，至少核对下表。

| 定制 | 不能回归的行为 | 主要位置 |
| --- | --- | --- |
| GPT-5.5 默认模型 | Codex `model` / `review_model` 默认 `gpt-5.5`；白名单、教程、价格都可识别 | `frontend/src/utils/clientConfig.ts`、`UseKeyModal.vue`、`UsageTutorialView.vue`、`useModelWhitelist.ts`、模型价格 JSON |
| PeterAI 多模型生图 | 同 Key/同分组多选模型；任务独立 `n:1`；最多 4 并发；可分模型提示词 | `deploy/static/image-generator/` |
| PeterAI 独立画布 | 外部 Canvas 服务和源码不并入 Sub2API；菜单、iframe、认证参数与 CSP 不能回归 | `settings.custom_menu_items`、`CustomPageView.vue`、外部 `peterai-canvas` fork |
| PeterAI 托管媒体 | Audio/Seedance 只调度显式能力 APIKey 账号；无明确价格 fail-closed；创建请求进入站内 usage 和扣费 | `openai_managed_media.go`、`gateway.go`、账号 `openai_capabilities` |
| 图片画廊 | 可发布/下载/管理；默认永久保留；原图尽量不降质 | `backend/migrations/150_image_gallery_items.sql`、`gallery*.go`、`GalleryView.vue` |
| 图片计费 | 价格来自 `prices_by_model`；前端不写死；只有真实图片输出才计费 | `api_key_service.go`、`billing_service.go`、`openai_images*.go` |
| Images failover | 网络错误、无图、可切换的上游拒绝会换账号；内容安全/参数错误不盲目重试 | `openai_images.go`、`openai_images_responses.go` |
| 多级代理 | 差价返利、代理授权、管理/用户树视图、树结构排序 | `9001_custom_affiliate_hierarchy.sql`、`affiliate_service.go`、`affiliate_repo.go` |
| 易支付增强 | 支持可选 `queryUrl`，`findorder` 先 type=2 再 type=1，然后回退 `/api.php` | `easypay.go`、`providerConfig.ts` |
| 顶栏问候 | 根据本地时间切换；夜深文案保留；窄屏隐藏 | `AppHeader.vue`、locale `common.ts` |
| 自定义 i18n | Gallery、教程、代理层级、问候、支付 key 不得直接显示 | `customFeatureLocaleKeys.spec.ts`、`headerGreetingLocales.spec.ts` |

### 3.1 GPT-5.5 默认值

必须保持：

```text
model = "gpt-5.5"
review_model = "gpt-5.5"
CLAUDE_CODE_ATTRIBUTION_HEADER=0
```

关键文件：

```text
frontend/src/utils/clientConfig.ts
frontend/src/components/keys/UseKeyModal.vue
frontend/src/views/user/UsageTutorialView.vue
frontend/src/composables/useModelWhitelist.ts
backend/resources/model-pricing/model_prices_and_context_window.json
backend/internal/pkg/openai/instructions_gpt5_5.txt
```

### 3.2 PeterAI 画图页与画廊

入口：

```text
用户页：https://api.peterai.cc.cd/custom/6768ebe29836ec72
iframe： https://api.peterai.cc.cd/image-generator/
```

版本化静态文件：

```text
deploy/static/image-generator/index.html
deploy/static/image-generator/main.js
deploy/static/image-generator/styles.css
deploy/static/image-generator/peterai.svg
```

生产实际目录：

```text
/app/data/public/image-generator/
```

修改静态页的最短流程：

```bash
node --check deploy/static/image-generator/main.js
# 修改 index.html 中 main.js?v=... 的版本参数
RESTART=1 deploy/publish-image-generator.sh
deploy/verify-production.sh
```

必须保持：

- 主提示词只通过 `textarea.prompt-textarea` 获取，不要误取模型覆盖输入框。
- 价格从 `/api/v1/user/image-generation/options` 的 `prices_by_model` 读取。
- 价格无法估算时显示“以实际扣费为准”。
- 失败任务不保存历史、不发布画廊、不计入成功费用。
- 历史记录存在 IndexedDB，不再人为限制 5 条；真实上限受浏览器配额影响。
- `GATEWAY_IMAGE_STREAM_DATA_INTERVAL_TIMEOUT=90`，浏览器单次生图超时也为 90 秒。

画廊 API 边界：

```text
POST   /api/v1/gallery/items
GET    /api/v1/gallery/my
DELETE /api/v1/gallery/items/:id
GET    /api/v1/gallery/items
GET    /api/v1/gallery/media/*path
       /api/v1/admin/gallery/*
```

### 3.3 图片计费与失败请求

- 用户组基础图片价格当前为 `0.1` / 张，但前端不得写死。
- 估算要综合分组价格、渠道定价、用户组倍率、图片倍率和峰值倍率。
- `imageCount` 初始值必须是 `0`；只有解析到 `b64_json`、`url` 或其他可用图片时才累加。
- 上游 HTTP 200 但无图片输出，必须触发 failover 或失败，不能进入成功计费。
- 非内容策略类 `request_rejected` 可切换同组账号；内容安全拒绝不切换。

### 3.3.1 PeterAI 独立画布

画布源码和发布链位于独立仓库：

```text
/home/aihub/Peter_ws/peterai-canvas
公开源码：https://github.com/Bingtao-Wang/PeterAI_canvas
生产分支：custom/peterai
上游：    https://github.com/basketikun/infinite-canvas.git
公网：    https://canvas.peterai.cc.cd
本机：    http://127.0.0.1:13000
菜单 ID： 51be877493a8929d
```

Sub2API 侧只保存数据库菜单配置，不复制 Canvas 源码、不新增迁移、不修改 Dockerfile。必须保留原 PeterAI 画图菜单，并新增：

```json
{
  "id": "51be877493a8929d",
  "label": "PeterAI 画布",
  "url": "https://canvas.peterai.cc.cd/canvas?mode=recent",
  "visibility": "user",
  "sort_order": 1
}
```

Canvas 通过自身 `/peter-api` 白名单代理访问本机 Sub2API，Sub2API 不为它开放全局 CORS。每次上游同步后确认：

```bash
curl -fsS https://canvas.peterai.cc.cd/healthz
curl -fsS https://api.peterai.cc.cd/api/v1/settings/public \
  | jq -e '.data.custom_menu_items[] | select(.id == "51be877493a8929d")'
curl -fsS https://api.peterai.cc.cd/custom/51be877493a8929d \
  | grep -q 'canvas.peterai.cc.cd/canvas'
```

Canvas 自动导入非图片模型依赖管理设置 `available_channels_enabled=true`；关闭时 `/api/v1/channels/available` 会按设计返回空数组。Sub2API 真实网关除 Models、Responses、Images 和 Grok 视频外，还提供：

```text
POST /v1/audio/speech
POST /v1/contents/generations/tasks
GET  /v1/contents/generations/tasks/:task_id
```

这两个托管媒体能力采用 fail-closed：OpenAI APIKey 账号必须在后台显式勾选 `audio_speech` / `seedance`，Audio 模型必须有独立按次默认价格，Seedance 必须有渠道按次分辨率价格或分组视频价格；自动时长只能走按次价，不能按默认秒数猜测扣费。普通 Codex 账号、未定价模型和 OAuth 账号不得进入这两个调度池。生产启用步骤：

1. 配置真实支持对应端点的 APIKey 账号和正确 Base URL；Seedance 官方 Ark Plan Base URL应以 `/api/plan/v3` 结尾。
2. 仅给已通过只读能力核对或人工验收的账号勾选媒体能力，不通过付费生成请求批量探测。
3. 把账号加入目标 PeterAI 分组，并为公开模型配置独立价格。
4. 用测试 Key 验证创建、轮询、响应内容、usage log、余额/订阅扣费和 failover 后，再让模型进入可用渠道元数据。

视频 `/content` 路由仍未实现，Canvas 托管适配器依赖轮询响应直接返回下载 URL。不得把 Nginx 白名单误当成上游账号和价格已配置；当前生产数据库尚未配置可启用的 Seedance 账号/价格，禁止自动启用旧 `Ark` 账号。

Canvas 的同步、发布、AGPL 源码和回滚规则以其 `PETER_FORK_MAINTENANCE.md` 为准。两个服务独立发版；Canvas 故障时只隐藏新菜单，不重建或回滚 Sub2API。

### 3.4 多级代理

数据源和边界：

- 邀请树唯一来源：`user_affiliates.inviter_id`。
- 自定义迁移：`backend/migrations/9001_custom_affiliate_hierarchy.sql`，不要改回普通连续编号。
- 用户权限表：`affiliate_agent_access`。
- 用户侧根节点必须强制为当前 JWT 用户，不接受前端伪造的 `root_user_id`。
- 未授权用户直接访问返回 `403`，不返回团队数据。
- 邀请链最多 20 层并带循环保护。
- 差价制：下级代理拿自己比例，上级拿与下级的差额。
- 下级比例不能超过上级；父级不能降到直属子级最高比例以下。
- 只扩展支付订单返利；历史订单不回算。

入口：

```text
管理员：/admin/affiliates/hierarchy
用户：  /affiliate/hierarchy
```

关键 API：

```text
GET /api/v1/admin/affiliates/hierarchy/roots
GET /api/v1/admin/affiliates/hierarchy
PUT /api/v1/admin/affiliates/hierarchy/users/:user_id/rate
PUT /api/v1/admin/affiliates/hierarchy/users/:user_id/access
GET /api/v1/user/aff/hierarchy/access
GET /api/v1/user/aff/hierarchy
```

### 3.5 支付、问候与 i18n

易支付：

- 推荐基础地址：`https://www.ezfpy.cn/`。
- 表单入口：`submit.php`；API 入口：`mapi.php`。
- 支付结果页：`https://api.peterai.cc.cd/payment/result`。
- 非标准查单接口通过可选 `queryUrl` 配置。

顶栏问候：

```text
{name} 夜深了，辛苦了。喝口水，早点休息！加油！
```

i18n 已目录化，定制 key 主要位于：

```text
frontend/src/i18n/locales/{zh,en}/common.ts
frontend/src/i18n/locales/{zh,en}/dashboard.ts
frontend/src/i18n/locales/{zh,en}/admin/overview.ts
frontend/src/i18n/locales/{zh,en}/admin/settings.ts
```

上游同步后必须运行：

```bash
npm -C frontend run test:run -- \
  src/i18n/__tests__/customFeatureLocaleKeys.spec.ts \
  src/i18n/__tests__/headerGreetingLocales.spec.ts \
  src/i18n/__tests__/localesNoKeyCollision.spec.ts
```

## 4. 运行时、备份与恢复

### 4.1 运行时真相源

生产行为共同由以下内容决定：

```text
deploy/.env
deploy/docker-compose.yml
Postgres 中的 settings / 业务数据
peter-sub2api_sub2api_data volume
Cloudflare Tunnel
```

关键约定：

```text
COMPOSE_PROJECT_NAME=peter-sub2api
BIND_HOST=127.0.0.1
SERVER_PORT=18080
GATEWAY_IMAGE_STREAM_DATA_INTERVAL_TIMEOUT=90
```

注意：

- `deploy/.env` 可能包含数据库密码、JWT secret、支付密钥等，不得提交或公开。
- Compose 默认值可能与生产 `.env` 不同，排障时以容器实际配置为准。
- 不要固定 `container_name`；继续使用 Compose 项目前缀隔离容器与 volume。

### 4.2 本地备份

备份脚本：

```text
/home/aihub/Peter_ws/sub2api/deploy/local-backup.sh
```

备份目录：

```text
/home/aihub/Peter_ws/sub2api-backups
```

内容：Postgres `pg_dump -Fc`、应用数据 volume、manifest、SHA-256 校验文件。

定时任务：

```cron
0 2 * * * /home/aihub/Peter_ws/sub2api/deploy/local-backup.sh >> /home/aihub/Peter_ws/sub2api-backups/cron.log 2>&1
```

保留策略：`RETAIN_DAYS=0`、`RETAIN_COUNT=7`。

手动备份与校验：

```bash
/home/aihub/Peter_ws/sub2api/deploy/local-backup.sh
cd /home/aihub/Peter_ws/sub2api-backups/<timestamp>
sha256sum -c sha256sums.txt
sg docker -c 'docker run --rm -v "$PWD:/backup" postgres:18-alpine pg_restore -l /backup/sub2api_pg_<timestamp>.dump | head'
```

内置 S3/R2 调度已启用，但当前未配置 `backup_s3_config`，不会产生远程备份。本地备份不能防整机磁盘损坏，仍建议配置 R2/S3。

### 4.3 恢复原则

- 恢复前先停止写入或进入维护窗口。
- 优先核对 `sha256sums.txt` 和 `pg_restore -l`。
- 数据库 dump、app data volume 和 `deploy/.env` 必须作为一组恢复材料。
- 回滚应用镜像时通常不需要恢复数据库；除非新迁移与旧版本不兼容。

## 5. 测试、发布与回滚

### 5.1 源码验证

前端：

```bash
npm -C frontend run typecheck -- --pretty false
npm -C frontend run test:run
npm -C frontend run build
node --check deploy/static/image-generator/main.js
```

后端（宿主机无 Go 时使用 Docker）：

```bash
sg docker -c "docker run --rm \
  -v '$PWD/backend:/app' -w /app \
  -e GOPROXY='https://goproxy.cn,direct' \
  -e GOSUMDB='sum.golang.google.cn' \
  golang:1.26.5 go test ./... -count=1"
```

关键定制快速测试：

```bash
deploy/test-with-docker.sh
```

### 5.2 标准发布

1. 确认 `custom/gallery` 已推送，工作区干净。
2. 运行本地备份并校验。
3. 用日期+上游版本+提交号构建新镜像。
4. 只重建 `sub2api` 应用容器，不重建 Postgres/Redis。
5. 运行完整生产验收。

```bash
cd /home/aihub/Peter_ws/sub2api
/home/aihub/Peter_ws/sub2api/deploy/local-backup.sh

commit="$(git rev-parse --short=12 HEAD)"
tag="sub2api-custom:YYYYMMDD-upstream-vXXXX-$(git rev-parse --short=8 HEAD)"

sg docker -c "docker build -t $tag \
  --build-arg GOPROXY=https://goproxy.cn,direct \
  --build-arg GOSUMDB=sum.golang.google.cn \
  --build-arg COMMIT=$commit \
  -f Dockerfile ."
```

将 `deploy/.env` 的 `SUB2API_IMAGE` 改为新 tag，然后：

```bash
sg docker -c 'docker compose -f deploy/docker-compose.yml --env-file deploy/.env up -d --force-recreate sub2api'
sg docker -c 'docker compose -f deploy/docker-compose.yml --env-file deploy/.env ps'
deploy/verify-production.sh
```

发布后至少确认：

- 容器 `healthy`，实际镜像 tag 正确。
- 本机与公网 `/health` 都返回 `{"status":"ok"}`。
- `/custom/<id>` 注入正确的 iframe URL。
- 公网 `main.js` 语法正确，`single_dollar_forEach = 0`。
- 图片价格与数据库配置相符。
- 仓库静态文件与 volume 中文件 hash 相同。
- Gallery、代理层级、教程、问候、支付不直接显示 i18n key。
- 日志没有迁移失败、panic 或持续启动错误。

### 5.3 回滚

优先只回滚应用镜像：

```bash
sg docker -c 'docker images | grep sub2api-custom'
# 将 deploy/.env 的 SUB2API_IMAGE 改回上一个已验证 tag
sg docker -c 'docker compose -f deploy/docker-compose.yml --env-file deploy/.env up -d --force-recreate sub2api'
deploy/verify-production.sh
```

不要删除 Postgres、Redis 或 app data volume。

## 6. 快速排障

### 6.1 页面仍显示旧内容

按真实访问链路排查：

1. `/custom/<id>` 注入的 iframe URL 是否带当前版本参数。
2. `/image-generator/` 是否引用当前 `main.js?v=...`。
3. 公网 `main.js` 是否真的包含新代码。
4. 数据库 `settings.custom_menu_items` 是否仍为旧 URL。
5. 修改菜单后是否重启应用以清理 `HTMLCache`。

常用验证：

```bash
curl -sS https://api.peterai.cc.cd/custom/6768ebe29836ec72 | grep -o 'image-generator[^"\\]*'
curl -sS https://api.peterai.cc.cd/image-generator/ | grep -n 'main.js'
deploy/verify-production.sh
```

用户端还需强制刷新：Windows/Linux `Ctrl+F5`，macOS `Cmd+Shift+R`。

### 6.2 页面显示 `admin.xxx` / `nav.xxx` / `common.xxx`

这是 i18n key 缺失，不是正常英文。

```bash
npm -C frontend run test:run -- src/i18n/__tests__/customFeatureLocaleKeys.spec.ts
```

修复时必须同时补齐 `zh` 和 `en`，并确认 key 放在正确的目录化 locale 模块中。

### 6.3 生图 502 / 长时间无响应

先看后端真实上游、账号和错误：

```bash
sg docker -c 'docker compose -f deploy/docker-compose.yml --env-file deploy/.env logs --since=2h sub2api' \
  | rg -n -C 6 'images/generations|openai.images.forward_failed|upstream request failed|status_code":502'
```

判断顺序：

1. 当前分组是否有其他 `schedulable=true` 的兼容账号。
2. transport/network 错误是否被包装为 `UpstreamFailoverError`。
3. 无图片输出是否触发 failover，而不是计费成功。
4. 后端图片流空闲超时和浏览器超时是否均为 90 秒。
5. 内容策略/参数错误不应盲目切账号。

## 7. 公网与 Cloudflare Tunnel

当前 Tunnel：

```text
名称：peter-sub2api
ID：  2ec22032-75ae-4588-b9d0-fddbaa1976cb
目标：http://localhost:18080
目录：/home/aihub/Peter_ws/sub2api-cloudflare-tunnel
```

已用路由：

```text
api.peterai.cc.cd -> http://localhost:18080
api.peteraix.com  -> http://localhost:18080
```

注意：

- Tunnel token 是敏感凭证，不写入文档或 Git。
- `api.peteraix.com` 的 DNS 权威托管仍需单独确认，不要在新路由验收前停旧服务。

## 8. 里程碑记录

- `2026-07-12`：新增 PeterAI 托管 `POST /v1/audio/speech`、Seedance 创建/轮询路由；仅调度管理员显式授权的 OpenAI APIKey 账号，无明确价格时 fail-closed；生产镜像 `sub2api-custom:20260712-upstream-v0151-e1da51f1`，发布前备份 `20260712_011100`，本机/公网验收通过。
- `2026-07-10`：同步至 `v0.1.151-17-ge316ebf5`；生产镜像 `sub2api-custom:20260710-upstream-v0151-92bce245`；修复目录化 i18n 遗漏；前端 146 文件/936 测试通过，Go 1.26.5 `go test ./...` 通过。
- `2026-07-07`：同步 `v0.1.146`，保留图片计费、Gallery、代理层级、问候和支付增强。
- `2026-06-30`：完成 Docker 迁移、本地备份、多级代理、使用教程和 PeterAI 多模型生图的主要定制。
- 更早的发布、热修、数据修复和排障过程不再在正文重复，通过 `git log -- FORK_MAINTENANCE.md` 查看。
