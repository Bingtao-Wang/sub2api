# Sub2API Fork 维护手册

这份文档记录当前 fork 的分支策略、部署方式、自定义功能、迁移状态和关键运维操作。原则是：`main` 保持干净同步上游，自定义能力集中维护在 `custom/gallery`，线上镜像从 `custom/gallery` 构建。

## 目录

- [一、分支与上游同步](#一分支与上游同步)
  - [1.1 当前分支模型](#11-当前分支模型)
  - [1.2 日常同步流程](#12-日常同步流程)
  - [1.3 冲突处理原则](#13-冲突处理原则)
  - [1.4 当前远端差异与灾备优先级](#14-当前远端差异与灾备优先级)
- [二、Docker 迁移部署](#二docker-迁移部署)
  - [2.1 当前部署状态](#21-当前部署状态)
  - [2.2 镜像构建策略](#22-镜像构建策略)
  - [2.3 Compose 隔离约定](#23-compose-隔离约定)
  - [2.4 数据恢复与备份](#24-数据恢复与备份)
  - [2.5 运行时配置真相源](#25-运行时配置真相源)
- [三、公网访问与域名](#三公网访问与域名)
  - [3.1 本机访问入口](#31-本机访问入口)
  - [3.2 Cloudflare Tunnel](#32-cloudflare-tunnel)
  - [3.3 域名切换注意事项](#33-域名切换注意事项)
- [四、自定义功能维护](#四自定义功能维护)
  - [4.1 GPT-5.5 默认模型](#41-gpt-55-默认模型)
  - [4.2 图片画廊](#42-图片画廊)
  - [4.3 PeterAI 画图页面](#43-peterai-画图页面)
  - [4.4 自定义菜单 URL](#44-自定义菜单-url)
  - [4.5 支付配置](#45-支付配置)
- [五、计费与数据修复](#五计费与数据修复)
  - [5.1 图片生成价格](#51-图片生成价格)
  - [5.2 失败请求不扣费](#52-失败请求不扣费)
  - [5.3 管理员账号](#53-管理员账号)
- [六、验证、发布与回滚](#六验证发布与回滚)
  - [6.1 常用验证命令](#61-常用验证命令)
  - [6.2 发布流程](#62-发布流程)
  - [6.3 回滚流程](#63-回滚流程)
  - [6.4 宿主机缺少构建工具时的替代验证](#64-宿主机缺少构建工具时的替代验证)
- [七、更新记录](#七更新记录)
  - [2026-06-30](#2026-06-30)
  - [2026-06-28](#2026-06-28)
  - [2026-06-27](#2026-06-27)
  - [2026-06-23](#2026-06-23)
  - [2026-06-22](#2026-06-22)

## 一、分支与上游同步

### 1.1 当前分支模型

```text
upstream/main
    |
origin/main
    |
custom/gallery
```

各分支职责：

- `main`：干净同步 `upstream/main`，不要放自定义功能。
- `gpt55-defaults`：最新 `main` + GPT-5.5 默认模型改动。
- `custom/gallery`：最新 `main` + GPT-5.5 默认模型改动 + 图片画廊 + PeterAI 画图相关定制。

当前远端：

- `upstream`：`https://github.com/Wei-Shaw/sub2api.git`
- `origin`：`https://github.com/Bingtao-Wang/sub2api.git`

当前生产分支：

- `custom/gallery`

### 1.2 日常同步流程

在仓库根目录执行：

```bash
cd /home/aihub/Peter_ws/sub2api

git fetch upstream --prune
git fetch origin --prune

git checkout main
git reset --hard upstream/main
git push origin main

git checkout custom/gallery
git rebase main
git push --force-with-lease origin custom/gallery
```

如果仍保留单独的 `gpt55-defaults` 分支，也同步更新：

```bash
git checkout gpt55-defaults
git rebase main
git push --force-with-lease origin gpt55-defaults
```

`rebase` 会改写功能分支历史，推送必须使用 `--force-with-lease`，不要直接使用 `--force`。

### 1.3 冲突处理原则

出现冲突后先查看状态：

```bash
git status
```

处理原则：

- 优先保留上游新增的安全修复、接口变化和迁移代码。
- 保留本 fork 的自定义能力：GPT-5.5 默认模型、画廊、PeterAI 画图、支付和部署适配。
- 如果同一段逻辑两边都改了，先读上游新逻辑，再把自定义逻辑重新套上去。

解决后继续：

```bash
git add <resolved-files>
GIT_EDITOR=true git rebase --continue
```

需要中止时：

```bash
git rebase --abort
```

重要更新前建议创建备份分支：

```bash
git branch custom/gallery-backup-$(date +%Y%m%d) custom/gallery
```

### 1.4 当前远端差异与灾备优先级

2026-06-30 检查结果：

```text
当前分支：custom/gallery
本地 HEAD：135c1537
upstream/main：c2754222
origin/custom/gallery：1af7573a
状态：custom/gallery...origin/custom/gallery [ahead 83, behind 7]
```

这表示本机 `custom/gallery` 已包含 2026-06-28 上游同步和本 fork 修复，但 GitHub fork 的 `origin/custom/gallery` 仍不是可靠灾备来源。恢复生产时不能只依赖 `origin/custom/gallery`。

已创建离线灾备：

```text
/home/aihub/Peter_ws/migration/sub2api-git-20260630-123303.bundle
```

优先处理顺序：

1. 优先恢复 GitHub HTTPS/SSH 凭据，把 `main`、`custom/gallery`、`gpt55-defaults` 推送到 `origin`。
2. 如果暂时不能推送，先创建本地 Git bundle 灾备：

```bash
mkdir -p /home/aihub/Peter_ws/migration
bundle=/home/aihub/Peter_ws/migration/sub2api-git-$(date +%Y%m%d-%H%M%S).bundle
git -C /home/aihub/Peter_ws/sub2api bundle create "$bundle" --all
git -C /home/aihub/Peter_ws/sub2api bundle verify "$bundle"
```

3. 画图页静态文件已纳入仓库 `deploy/static/image-generator/`，后续以该目录为版本化真相源，见 [4.3.1](#431-真实入口与文件位置)。

推送前必须重新确认：

```bash
git -C /home/aihub/Peter_ws/sub2api status --short --branch
git -C /home/aihub/Peter_ws/sub2api log --oneline --left-right origin/custom/gallery...custom/gallery | head -120
```

## 二、Docker 迁移部署

### 2.1 当前部署状态

当前服务器部署位置：

```text
/home/aihub/Peter_ws/sub2api
```

当前 Compose 配置目录：

```text
/home/aihub/Peter_ws/sub2api/deploy
```

当前运行约定：

- Compose 项目名：`peter-sub2api`
- 应用镜像：`sub2api-custom:20260628`
- 本机监听：`127.0.0.1:18080`
- 容器服务端口：`8080`
- Postgres：Compose 内部服务 `postgres`
- Redis：Compose 内部服务 `redis`

当前服务检查：

```bash
sg docker -c 'docker compose -f /home/aihub/Peter_ws/sub2api/deploy/docker-compose.yml --env-file /home/aihub/Peter_ws/sub2api/deploy/.env ps'
curl -sS http://127.0.0.1:18080/health
curl -sS https://api.peterai.cc.cd/health
```

### 2.2 镜像构建策略

生产镜像必须从 `custom/gallery` 构建：

```bash
cd /home/aihub/Peter_ws/sub2api
git checkout custom/gallery
sg docker -c 'docker build -t sub2api-custom:YYYYMMDD .'
```

`deploy/.env` 中通过变量指定镜像：

```env
SUB2API_IMAGE=sub2api-custom:YYYYMMDD
```

不建议生产长期使用 `latest`，带日期或版本号的镜像更容易回滚。

### 2.3 Compose 隔离约定

因为多人共用同一个 Linux 用户 `aihub`，隔离重点放在 Docker 和目录边界：

- 固定使用 `COMPOSE_PROJECT_NAME=peter-sub2api`
- 端口绑定到 `127.0.0.1:18080`
- 避免固定 `container_name`
- volume 使用 Compose 项目前缀，例如 `peter-sub2api_sub2api_data`
- 隧道单独放在 `/home/aihub/Peter_ws/sub2api-cloudflare-tunnel`

### 2.4 数据恢复与备份

迁移数据来源目录：

```text
/home/aihub/Peter_ws/migration
```

已使用过的迁移文件：

```text
sub2api_pg_20260622_163747.dump
sub2api_appdata_20260622_163819.tar.gz
```

数据库恢复示例：

```bash
cd /home/aihub/Peter_ws/sub2api/deploy
sg docker -c 'docker compose cp /home/aihub/Peter_ws/migration/sub2api_pg_20260622_163747.dump postgres:/tmp/sub2api.dump'
sg docker -c 'docker compose exec postgres sh -lc '\''PGPASSWORD="$POSTGRES_PASSWORD" pg_restore --clean --if-exists -U "$POSTGRES_USER" -d "$POSTGRES_DB" /tmp/sub2api.dump'\'''
```

`/app/data` 恢复示例：

```bash
sg docker -c 'docker run --rm -v peter-sub2api_sub2api_data:/data -v /home/aihub/Peter_ws/migration:/backup alpine sh -lc '\''cd /data && tar -xzf /backup/sub2api_appdata_20260622_163819.tar.gz'\'''
```

### 2.5 运行时配置真相源

生产运行时以 `deploy/.env` + `deploy/docker-compose.yml` + Docker volume + 数据库设置共同决定，不能只看源码默认值。

当前关键 `.env` 覆盖：

```text
COMPOSE_PROJECT_NAME=peter-sub2api
SUB2API_IMAGE=sub2api-custom:20260628
BIND_HOST=127.0.0.1
SERVER_PORT=18080
GATEWAY_IMAGE_STREAM_DATA_INTERVAL_TIMEOUT=90
```

注意：

- `deploy/docker-compose.yml` 里 `GATEWAY_IMAGE_STREAM_DATA_INTERVAL_TIMEOUT` 默认仍是 `900`，生产依赖 `.env` 覆盖为 `90`。
- `deploy/.env` 可能包含数据库密码、JWT secret、支付密钥等敏感信息，不要提交到 Git。
- 迁移或重装时，必须安全迁移 `.env`，否则图片生成超时、登录态、TOTP、支付等行为可能变化。

只查看非敏感运行关键项：

```bash
grep -E '^(COMPOSE_PROJECT_NAME|SUB2API_IMAGE|BIND_HOST|SERVER_PORT|GATEWAY_IMAGE_STREAM_DATA_INTERVAL_TIMEOUT)=' /home/aihub/Peter_ws/sub2api/deploy/.env
```

完整运行态确认：

```bash
sg docker -c 'docker inspect peter-sub2api-sub2api-1 --format "{{.Config.Image}}"'
sg docker -c 'docker compose -f /home/aihub/Peter_ws/sub2api/deploy/docker-compose.yml --env-file /home/aihub/Peter_ws/sub2api/deploy/.env ps'
```

## 三、公网访问与域名

### 3.1 本机访问入口

本机验收入口：

```text
http://127.0.0.1:18080
```

公网入口：

```text
https://api.peterai.cc.cd
```

当前 `api.peteraix.com` 因 DNS 权威服务器仍在阿里云 `dns19.hichina.com / dns20.hichina.com`，Cloudflare Tunnel 的自动 DNS 不会直接接管该域名。后续要迁移 `api.peteraix.com`，需要先明确 DNS 托管方案。

### 3.2 Cloudflare Tunnel

当前隧道：

- 隧道名：`peter-sub2api`
- 隧道 ID：`2ec22032-75ae-4588-b9d0-fddbaa1976cb`
- Connector 平台：`linux_amd64`
- Connector 状态：已连接

当前已发布应用程序路由：

```text
api.peteraix.com    -> http://localhost:18080
api.peterai.cc.cd   -> http://localhost:18080
```

注意：

- 隧道 token 是敏感凭证，不要写入文档、提交到 Git 或发到公开渠道。
- Cloudflare 路由服务类型使用 `HTTP`。
- 服务地址填写 `localhost:18080`。
- 路径留空，表示匹配所有路径。

### 3.3 域名切换注意事项

正式切换 `api.peteraix.com` 前：

- 先确认新服务器本机和 `api.peterai.cc.cd` 都正常。
- 确认登录、API Key、后台、画廊、支付、图片生成都能正常使用。
- 如果继续使用阿里云 DNS，需要手动维护 CNAME/A 记录。
- 如果迁到 Cloudflare DNS，需要把域名 NS 改成 Cloudflare 提供的 nameserver。
- 不要在新服务未验收前停旧服务。

## 四、自定义功能维护

### 4.1 GPT-5.5 默认模型

`gpt55-defaults` 和 `custom/gallery` 都包含 GPT-5.5 默认模型相关提交。上游同步后，如果默认模型相关文件冲突，需要确认 UI 默认值、后端模型默认值和文案一致。

当前关键代码位置：

```text
frontend/src/utils/clientConfig.ts
frontend/src/components/keys/UseKeyModal.vue
frontend/src/views/user/UsageTutorialView.vue
frontend/src/composables/useModelWhitelist.ts
backend/resources/model-pricing/model_prices_and_context_window.json
backend/internal/pkg/openai/instructions_gpt5_5.txt
```

核心验收点：

- Codex 配置生成器默认 `model = "gpt-5.5"` 和 `review_model = "gpt-5.5"`。
- Claude Code 模板保留上游新增的 `CLAUDE_CODE_ATTRIBUTION_HEADER=0`。
- 后台模型白名单、使用教程、价格资源里都能识别 `gpt-5.5`。

### 4.2 图片画廊

画廊功能在 `custom/gallery` 中维护。当前要求：

- 生成后的图片可以发布到画廊。
- 最近生成和画廊展示应使用清晰图片。
- 下载应尽量使用原图，不使用压缩预览图。
- 静态页面位于 Docker volume 的 `/app/data/public/image-generator/`。

当前真实实现边界：

```text
backend/migrations/150_image_gallery_items.sql
backend/internal/service/gallery.go
backend/internal/repository/gallery_repo.go
backend/internal/handler/gallery_handler.go
backend/internal/handler/admin/gallery_handler.go
backend/internal/server/routes/user.go
backend/internal/server/routes/admin.go
frontend/src/api/admin/gallery.ts
frontend/src/views/admin/GalleryView.vue
```

路由边界：

- 用户登录后：`POST /api/v1/gallery/items`、`GET /api/v1/gallery/my`、`DELETE /api/v1/gallery/items/:id`
- 公开画廊：`GET /api/v1/gallery/items`
- 公开媒体：`GET /api/v1/gallery/media/*path`
- 管理后台：`/api/v1/admin/gallery/*`

画廊文件默认落在应用数据目录下，随 `sub2api_data` volume 迁移；数据库只保存相对路径和元信息。

线上静态文件可通过 volume 检查：

```bash
sg docker -c 'docker run --rm -v peter-sub2api_sub2api_data:/data alpine ls -la /data/public/image-generator'
```

### 4.3 PeterAI 画图页面

#### 4.3.1 真实入口与文件位置

用户实际访问入口通常是自定义菜单页：

```text
https://api.peterai.cc.cd/custom/6768ebe29836ec72
```

该页面是主 Vue 页面，它会从 `window.__APP_CONFIG__.custom_menu_items` 读取菜单 URL，再嵌入画图 iframe。

画图 iframe 入口：

```text
https://api.peterai.cc.cd/image-generator/
```

重要：画图页不是 Vue 前端编译产物，不需要 `pnpm build`。后端代码会优先从 `data/public/<path>` 服务本地覆盖文件，再回退到嵌入的 Vue SPA。相关代码：

```text
backend/internal/web/embed_on.go
backend/internal/server/middleware/security_headers.go
frontend/src/utils/embedded-url.ts
frontend/src/views/user/CustomPageView.vue
```

生产优先生效路径是 Docker volume 中的静态覆盖目录：

```text
/app/data/public/image-generator/
```

版本化静态文件目录：

```text
/home/aihub/Peter_ws/sub2api/deploy/static/image-generator/
```

当前包含：

```text
index.html
main.js
styles.css
peterai.svg
```

历史宿主机源文件仍存在：

```text
/home/aihub/Peter_ws/image-generator-index.html
/home/aihub/Peter_ws/image-generator-main.js
```

这两个历史文件不再作为唯一真相源。修改画图页时优先改 `deploy/static/image-generator/`，再用发布脚本覆盖到 Docker volume。

宿主机当前对应 Docker volume：

```text
peter-sub2api_sub2api_data
```

由于该 volume 在宿主机 `/var/lib/docker/volumes/...` 可能没有当前用户权限，推荐通过容器或临时 alpine 容器检查：

```bash
sg docker -c 'docker exec peter-sub2api-sub2api-1 sh -lc "ls -la /app/data/public/image-generator && wc -c /app/data/public/image-generator/index.html /app/data/public/image-generator/main.js"'
sg docker -c 'docker run --rm -v peter-sub2api_sub2api_data:/data alpine sh -lc "ls -la /data/public/image-generator"'
```

#### 4.3.2 正确修改流程

修改画图页时按以下顺序做，缺任何一步都可能导致用户仍加载旧代码：

1. 修改版本化源文件：
   - `/home/aihub/Peter_ws/sub2api/deploy/static/image-generator/index.html`
   - `/home/aihub/Peter_ws/sub2api/deploy/static/image-generator/main.js`
   - `/home/aihub/Peter_ws/sub2api/deploy/static/image-generator/styles.css`
   - `/home/aihub/Peter_ws/sub2api/deploy/static/image-generator/peterai.svg`
2. 修改 `index.html` 里的 `main.js?v=...` 版本号。
3. 语法检查：

```bash
node -c /home/aihub/Peter_ws/sub2api/deploy/static/image-generator/main.js
```

4. 发布到正在运行的容器覆盖目录：

```bash
/home/aihub/Peter_ws/sub2api/deploy/publish-image-generator.sh
```

需要同时清理 `/custom/<id>` 注入配置缓存时：

```bash
RESTART=1 /home/aihub/Peter_ws/sub2api/deploy/publish-image-generator.sh
```

5. 对比版本化文件和容器实际文件：

```bash
sha256sum /home/aihub/Peter_ws/sub2api/deploy/static/image-generator/*
sg docker -c 'docker exec peter-sub2api-sub2api-1 sh -lc "sha256sum /app/data/public/image-generator/index.html /app/data/public/image-generator/main.js"'
```

6. 如果 `/custom/<id>` 的 iframe URL 需要强制刷新缓存，同步更新数据库 `settings.custom_menu_items` 中该菜单 URL 的查询参数，例如 `?v=image-502-failover-20260627`。
7. 重启应用容器，清掉后端 `HTMLCache`，否则 `/custom/<id>` HTML 里可能仍注入旧菜单 URL：

```bash
sg docker -c 'docker restart peter-sub2api-sub2api-1'
curl -sS http://127.0.0.1:18080/health
```

8. 使用公网 URL 验证，不能只看本地文件：

```bash
/home/aihub/Peter_ws/sub2api/deploy/verify-production.sh
```

#### 4.3.3 生图 HTTP 502 排查要点

`生成失败 HTTP 502` 不一定是前端问题。必须先看后端日志确认真实上游、账号和错误：

```bash
docker logs --since 2h peter-sub2api-sub2api-1 2>&1 \
  | rg -n -C 6 'images/generations|openai.images.forward_failed|upstream request failed|status_code":502'
```

2026-06-27 的真实原因：

```text
/v1/images/generations
api_key_id=31 FaXian_api
group_id=25 生图模型
account_id=4562 FX_image2
upstream=https://www.findcg.com/v1/images/generations
error=dial tcp ...:443: connect: connection refused
```

检查生图组当前可调度账号：

```bash
docker exec peter-sub2api-postgres-1 psql -U sub2api -d sub2api -P pager=off -F $'\t' -Atc "
select ag.account_id, a.name, a.status, a.schedulable, a.priority,
       coalesce(a.credentials->>'base_url','') as base_url,
       coalesce(a.error_message,'') as error_message
from account_groups ag
join accounts a on a.id=ag.account_id
where ag.group_id=25 and a.deleted_at is null
order by a.priority, a.id;"
```

如果唯一 `schedulable=true` 的账号上游不可用，后端只能返回 502。2026-06-27 已做的热修：

```sql
update accounts
set schedulable = false,
    error_message = 'Disabled 2026-06-27: image upstream https://www.findcg.com returned TCP connection refused for /v1/images/generations'
where id = 4562;

update accounts
set schedulable = true,
    error_message = null
where id = 4565;
```

后端源码也已修正：Images API 的 transport/network 错误必须返回 `UpstreamFailoverError`，不能只 `fmt.Errorf("upstream request failed...")`，否则连接拒绝/超时时不会进入账号 failover。

命中文件：

```text
backend/internal/service/openai_images.go
backend/internal/service/openai_images_responses.go
```

如果页面显示：

```text
正在并行生成 1 张图片，请稍候...
图片 1/1: 正在通过备用接口生成...
```

并持续几十秒到两分钟，说明前端请求已经发到 `/v1/images/generations`，但上游长时间没有返回图片流数据。之前运行配置里：

```text
GATEWAY_IMAGE_STREAM_DATA_INTERVAL_TIMEOUT=900
```

这会让后端最多等 900 秒才判定图片流空闲超时。2026-06-27 已调整：

```text
GATEWAY_IMAGE_STREAM_DATA_INTERVAL_TIMEOUT=90
```

同时 `image-generator-main.js` 增加浏览器侧 90 秒单次请求超时：

```text
IMAGE_REQUEST_TIMEOUT_MS = 90000
```

如果 90 秒仍没有任何响应，前端会中断本次请求并重试，避免用户看到 136 秒以上才重试。

#### 4.3.4 必须验证真实公网文件

如果用户仍反馈旧错误，例如：

```text
生成失败
$(...).forEach is not a function
```

不要只看本地 `/home/aihub/Peter_ws/image-generator-main.js`。必须检查三层真实链路：

1. `/custom/<id>` 注入的菜单 URL 是否是新版。
2. `/image-generator/` 返回的 `main.js?v=...` 是否是新版。
3. 公网 `main.js` 内容是否是新版。

```bash
curl -k -sS https://api.peterai.cc.cd/custom/6768ebe29836ec72 -o /tmp/custom-page.html
grep -o 'image-generator/?v=[^"\\]*' /tmp/custom-page.html | head

curl -k -sS https://api.peterai.cc.cd/image-generator/ -o /tmp/image-generator-index.html
grep -n "main.js" /tmp/image-generator-index.html

curl -k -sS 'https://api.peterai.cc.cd/image-generator/main.js?v=<当前版本号>' -o /tmp/image-generator-main.js
node -c /tmp/image-generator-main.js
node - <<'NODE'
const fs = require('fs')
const s = fs.readFileSync('/tmp/image-generator-main.js', 'utf8')
let bad = 0
s.split(/\n/).forEach((line, i) => {
  if (!line.includes('.forEach')) return
  for (let p = line.indexOf('$('); p !== -1; p = line.indexOf('$(', p + 1)) {
    if (p === 0 || line[p - 1] !== '$') {
      bad++
      console.log((i + 1) + ':' + line)
    }
  }
})
console.log('single_dollar_forEach =', bad)
NODE
```

`single_dollar_forEach` 必须为 `0`。如果公网文件正确但用户仍看到旧错误，优先检查 iframe 入口 URL 是否仍在使用旧缓存或另一个域。

#### 4.3.5 前端样式注意事项

历史记录缩略图依赖 `styles.css` 中的 `.history-thumb`：

```css
height: 0;
padding-bottom: 75%;
```

如果把 `.history-thumb` 从 `div` 改成 `button`，动态 CSS 只能重置 `appearance/border/background`，不要写 `padding: 0`，否则会覆盖 `padding-bottom: 75%`，导致缩略图高度变成 0、看起来不显示。

当前价格展示：

```text
统一价每张 $0.1
预估费用按每张 $0.10 计算
```

线上验证：

```bash
curl -sS https://api.peterai.cc.cd/image-generator/ | grep -nE '统一价|\$0\.5|\$0\.50|\$0\.1|main\.js'
curl -sS 'https://api.peterai.cc.cd/image-generator/main.js?v=price-010-dollar-20260622' | grep -nE 'PRICE_PER_IMAGE|costEl\.textContent'
```

如果浏览器仍显示旧价格，通常是缓存。强制刷新：

- Windows/Linux：`Ctrl + F5`
- macOS：`Cmd + Shift + R`
- 或使用无痕窗口访问

### 4.4 自定义菜单 URL

当前 PeterAI 画图菜单 URL 来自数据库 `settings.custom_menu_items`，不是前端硬编码。当前应指向：

```text
https://api.peterai.cc.cd/image-generator/?v=<当前画图页版本>
```

检查命令：

```bash
sg docker -c 'docker exec peter-sub2api-postgres-1 psql -U sub2api -d sub2api -c "select value from settings where key='\''custom_menu_items'\'';"'
curl -sS http://127.0.0.1:18080/api/v1/settings/public | node -e "let s='';process.stdin.on('data',d=>s+=d).on('end',()=>console.log(JSON.stringify(JSON.parse(s).data.custom_menu_items,null,2)))"
```

更新菜单 URL 查询参数示例：

```bash
sg docker -c 'docker exec peter-sub2api-postgres-1 psql -U sub2api -d sub2api -t -A -c "select value from settings where key='\''custom_menu_items'\'';"' > /tmp/custom_menu_items.json

node - <<'NODE' > /tmp/update_custom_menu.sql
const fs = require('fs')
const items = JSON.parse(fs.readFileSync('/tmp/custom_menu_items.json', 'utf8').trim())
for (const item of items) {
  if (item.url && item.url.includes('/image-generator/')) {
    const u = new URL(item.url)
    u.searchParams.set('v', 'image-502-failover-20260627')
    item.url = u.toString()
  }
}
console.log(`update settings set value = $json$${JSON.stringify(items)}$json$, updated_at = now() where key = 'custom_menu_items';`)
NODE

sg docker -c 'docker exec -i peter-sub2api-postgres-1 psql -U sub2api -d sub2api < /tmp/update_custom_menu.sql'
```

如果要强制 iframe 入口刷新缓存，给菜单 URL 加查询参数，例如：

```text
https://api.peterai.cc.cd/image-generator/?v=image-502-failover-20260627
```

注意：直接改数据库 `settings.custom_menu_items` 后，`/custom/<id>` 的 HTML 注入配置可能仍被后端 `HTMLCache` 缓存。需要重启应用容器或通过管理端设置保存流程触发缓存失效：

```bash
sg docker -c 'docker restart peter-sub2api-sub2api-1'
curl -k -sS https://api.peterai.cc.cd/custom/6768ebe29836ec72 | grep -o 'image-generator[^"]*'
```

如果这里仍没有新的 `?v=...`，说明用户打开的自定义页还会嵌入旧 iframe URL。

之前保存易支付方式时遇到：

```text
Custom menu item URL must be an absolute http(s) URL or md:<slug>
```

已调整校验逻辑，允许：

- 绝对 HTTP/HTTPS URL，例如 `https://example.com/pay`
- 同站相对路径，例如 `/payment/result`
- Markdown 页面引用，例如 `md:<slug>`

相关改动：

```text
backend/internal/handler/admin/setting_handler.go
```

### 4.5 支付配置

易支付 V1 推荐使用接口：

```text
https://www.ezfpy.cn/
```

常见入口：

- 表单跳转：`https://www.ezfpy.cn/submit.php`
- API JSON：`https://www.ezfpy.cn/mapi.php`

当前重点：

- 支付结果页地址不要带空格。
- 示例正确地址：`https://api.peterai.cc.cd/payment/result`
- 如果支付宝桌面端二维码未生成，优先调整易支付通道配置、跳转模式和浏览器拦截设置。

## 五、计费与数据修复

### 5.1 图片生成价格

当前所有用户组图片价格已调整为每张 `0.1`：

```sql
update groups
set image_price_1k = 0.1,
    image_price_2k = 0.1,
    image_price_4k = 0.1;
```

线上成功请求已验证为：

```text
total_cost = 0.1000000000
actual_cost = 0.1000000000
```

### 5.2 失败请求不扣费

问题原因：

图片接口在非流式成功路径中曾把 `imageCount` 默认设为请求数量 `parsed.N`。当上游返回 HTTP 200，但响应体没有可用图片数据时，前端会报“Images API 未返回可用图片数据”，后端却仍按 1 张扣费。

修复方式：

- `imageCount` 默认改为 `0`
- 只有从响应中解析到 `b64_json`、`url` 或其他可用图片输出时才计费
- 增加回归测试：HTTP 200 但无图片数据时 `ImageCount = 0`

相关文件：

```text
backend/internal/service/openai_images.go
backend/internal/service/openai_images_responses.go
backend/internal/service/openai_images_test.go
```

当前代码注意点：

- `backend/internal/service/openai_images.go` 的主 Images API 路径已把 `imageCount` 初始化为 `0`，非流式响应只有解析到图片输出时才增加计费张数。
- `backend/internal/service/openai_images_responses.go` 的 OAuth/Responses 转换路径在无图时会优先返回 `UpstreamFailoverError` 或上游拒绝错误；正常成功返回前仍有 `if imageCount <= 0 { imageCount = parsed.N }` 兜底。按当前上下文，这个兜底通常不会处理“HTTP 200 但无图”的失败场景，但后续改动时应避免让无图响应正常落入成功计费路径。
- 如果继续维护图片计费，建议增加覆盖 OAuth/Responses 转换路径的无图回归测试，防止将来重构时重新引入失败扣费。

已处理历史失败扣费：

- 时间段：`2026-06-22 17:56:00` 到 `2026-06-22 18:00:06`
- usage log：`30294` 到 `30306`
- 用户：`admin@sub2api.local`
- 条数：`13`
- 原扣费：`13 * 0.5 = 6.5`
- 已将这 13 条 `total_cost` / `actual_cost` 置为 `0`
- 已退还余额 `6.5`
- 已清理 `billing:balance:3` 和 API key 鉴权缓存

### 5.3 管理员账号

已设为管理员：

```text
Daojie.Peng@qq.com
1312194755@qq.com
```

当前角色检查 SQL：

```sql
select id, email, role, balance
from users
where lower(email) in ('daojie.peng@qq.com', '1312194755@qq.com');
```

## 六、验证、发布与回滚

### 6.1 常用验证命令

容器状态：

```bash
sg docker -c 'docker compose -f /home/aihub/Peter_ws/sub2api/deploy/docker-compose.yml --env-file /home/aihub/Peter_ws/sub2api/deploy/.env ps'
```

健康检查：

```bash
curl -sS http://127.0.0.1:18080/health
curl -sS https://api.peterai.cc.cd/health
```

查看应用日志：

```bash
sg docker -c 'docker compose -f /home/aihub/Peter_ws/sub2api/deploy/docker-compose.yml --env-file /home/aihub/Peter_ws/sub2api/deploy/.env logs --tail=100 sub2api'
```

画图页发布验证：

```bash
/home/aihub/Peter_ws/sub2api/deploy/verify-production.sh
```

手工拆分验证：

```bash
# 1. 父级自定义页应注入新版 iframe URL
curl -k -sS https://api.peterai.cc.cd/custom/6768ebe29836ec72 -o /tmp/custom-page.html
grep -o 'image-generator/?v=[^"\\]*' /tmp/custom-page.html | head

# 2. iframe HTML 应引用新版 main.js
curl -k -sS 'https://api.peterai.cc.cd/image-generator/?v=<当前版本号>' -o /tmp/image-generator-index.html
grep -n 'main.js' /tmp/image-generator-index.html

# 3. 公网 main.js 应语法正确，且没有单 $().forEach
curl -k -sS 'https://api.peterai.cc.cd/image-generator/main.js?v=<当前版本号>' -o /tmp/image-generator-main.js
node -c /tmp/image-generator-main.js
node - <<'NODE'
const fs = require('fs')
const s = fs.readFileSync('/tmp/image-generator-main.js', 'utf8')
let bad = 0
s.split(/\n/).forEach((line, i) => {
  if (!line.includes('.forEach')) return
  for (let p = line.indexOf('$('); p !== -1; p = line.indexOf('$(', p + 1)) {
    if (p === 0 || line[p - 1] !== '$') {
      bad++
      console.log((i + 1) + ':' + line)
    }
  }
})
console.log('single_dollar_forEach =', bad)
NODE
```

检查图片价格：

```bash
sg docker -c 'docker compose -f /home/aihub/Peter_ws/sub2api/deploy/docker-compose.yml --env-file /home/aihub/Peter_ws/sub2api/deploy/.env exec -T postgres sh -lc '\''psql -U "$POSTGRES_USER" -d "$POSTGRES_DB" -P pager=off -c "select min(image_price_1k), max(image_price_1k), min(image_price_2k), max(image_price_2k), min(image_price_4k), max(image_price_4k) from groups;"'\'''
```

### 6.2 发布流程

标准发布：

```bash
cd /home/aihub/Peter_ws/sub2api
git checkout custom/gallery
sg docker -c 'docker build -t sub2api-custom:YYYYMMDD .'
# 构建成功后，把 /home/aihub/Peter_ws/sub2api/deploy/.env 里的 SUB2API_IMAGE 改成新 tag。
sg docker -c 'docker compose -f /home/aihub/Peter_ws/sub2api/deploy/docker-compose.yml --env-file /home/aihub/Peter_ws/sub2api/deploy/.env up -d --force-recreate sub2api'
/home/aihub/Peter_ws/sub2api/deploy/verify-production.sh
```

只发布 PeterAI 画图页静态文件：

```bash
/home/aihub/Peter_ws/sub2api/deploy/publish-image-generator.sh
/home/aihub/Peter_ws/sub2api/deploy/verify-production.sh
```

发布后检查：

- 后台登录正常
- API Key 请求正常
- PeterAI 画图页面正常
- 成功图片按 `$0.1` 计费
- 失败图片不扣图片费用
- 支付入口和回调页面正常

### 6.3 回滚流程

如果新镜像异常，先确认旧镜像 tag：

```bash
sg docker -c 'docker images | grep sub2api-custom'
```

把 `deploy/.env` 的 `SUB2API_IMAGE` 改回旧 tag，然后重启：

```bash
sg docker -c 'docker compose -f /home/aihub/Peter_ws/sub2api/deploy/docker-compose.yml --env-file /home/aihub/Peter_ws/sub2api/deploy/.env up -d --force-recreate sub2api'
```

不要删除数据库或 volume。回滚应用镜像通常不需要动 Postgres/Redis。

### 6.4 宿主机缺少构建工具时的替代验证

2026-06-30 检查时，宿主机直接运行：

```bash
go test ./internal/service -run 'OpenAIImages|Gallery'
pnpm vitest run src/utils/__tests__/embedded-url.spec.ts
```

返回：

```text
go: command not found
pnpm: command not found
```

这表示当前宿主机不能直接执行 Go/前端单测。此时不要把“无法运行单测”误判为“功能已验证充分”，应至少完成以下运行态替代验证：

当前已提供 Docker 测试脚本，不依赖宿主机安装 Go/pnpm：

```bash
# 后端 Images/Gallery 相关测试
/home/aihub/Peter_ws/sub2api/deploy/test-with-docker.sh backend

# 前端关键测试
/home/aihub/Peter_ws/sub2api/deploy/test-with-docker.sh frontend

# 全部默认测试
/home/aihub/Peter_ws/sub2api/deploy/test-with-docker.sh
```

2026-06-30 已验证：

```text
backend: ok github.com/Wei-Shaw/sub2api/internal/service
frontend: 3 files passed, 13 tests passed
```

运行态替代验证：

```bash
# 1. 容器和镜像
/home/aihub/Peter_ws/sub2api/deploy/verify-production.sh
```

脚本会自动检查：

- 容器状态和当前镜像 tag
- 本机和公网 `/health`
- `/custom/<id>` 注入的 `image-generator/?v=...`
- 公网 `/image-generator/` 引用的 `main.js?v=...`
- 公网 `main.js` 语法和 `single_dollar_forEach = 0`
- 图片价格是否均为 `0.10000000`
- 仓库静态文件与容器静态文件 hash

也可手工拆分执行：

```bash
# 1. 容器和镜像
sg docker -c 'docker compose -f /home/aihub/Peter_ws/sub2api/deploy/docker-compose.yml --env-file /home/aihub/Peter_ws/sub2api/deploy/.env ps'
sg docker -c 'docker inspect peter-sub2api-sub2api-1 --format "{{.Config.Image}}"'

# 2. 本机和公网健康检查
curl -sS --max-time 10 http://127.0.0.1:18080/health
curl -k -sS --max-time 15 https://api.peterai.cc.cd/health

# 3. 画图页真实公网链路
curl -k -sS --max-time 15 https://api.peterai.cc.cd/custom/6768ebe29836ec72 -o /tmp/custom-page.html
grep -o 'image-generator/?v=[^"\\]*' /tmp/custom-page.html | head
curl -k -sS --max-time 15 https://api.peterai.cc.cd/image-generator/ -o /tmp/image-generator-index.html
grep -n 'main.js' /tmp/image-generator-index.html

# 4. 公网 main.js 语法
curl -k -sS --max-time 15 'https://api.peterai.cc.cd/image-generator/main.js?v=<当前版本号>' -o /tmp/image-generator-main.js
node -c /tmp/image-generator-main.js

# 5. 图片价格
sg docker -c 'docker compose -f /home/aihub/Peter_ws/sub2api/deploy/docker-compose.yml --env-file /home/aihub/Peter_ws/sub2api/deploy/.env exec -T postgres sh -lc '\''psql -U "$POSTGRES_USER" -d "$POSTGRES_DB" -P pager=off -Atc "select min(image_price_1k), max(image_price_1k), min(image_price_2k), max(image_price_2k), min(image_price_4k), max(image_price_4k) from groups;"'\'''
```

如果要进一步增强源码级验证，可以在 CI 中跑更完整的 `go test ./...`、`pnpm vitest`、`pnpm build`。本机默认使用 Docker 测试脚本和运行态验收。

## 七、更新记录

### 2026-06-30

- 基于真实代码和运行态完成维护文档优化：
  - 记录当前 `custom/gallery` 与 `origin/custom/gallery` 的分叉状态：本地 `ahead 83, behind 7`，本地 HEAD 为 `135c1537`。
  - 明确 `origin/custom/gallery` 不是当前生产代码的可靠灾备来源，新增 Git bundle 灾备流程。
  - 明确生产运行态由 `deploy/.env`、Compose、Docker volume、数据库设置共同决定。
  - 标出 `GATEWAY_IMAGE_STREAM_DATA_INTERVAL_TIMEOUT` 生产依赖 `.env` 覆盖为 `90`，不要只看 Compose 默认 `900`。
  - 明确 PeterAI 画图页不是 Vue build 产物，真实源文件在仓库外 `/home/aihub/Peter_ws/image-generator-index.html` 和 `/home/aihub/Peter_ws/image-generator-main.js`。
  - 补充画图页源文件备份、容器覆盖、sha256 对比和公网验证流程。
  - 补充图片画廊、GPT-5.5 默认模型、自定义菜单、同源静态覆盖的真实代码路径。
  - 标注当前宿主机缺少 `go` 和 `pnpm`，并新增 Docker 测试脚本 `deploy/test-with-docker.sh`。
  - 新增画图页静态发布脚本 `deploy/publish-image-generator.sh` 和生产验证脚本 `deploy/verify-production.sh`。
- 实际落地：
  - 已创建 Git bundle 灾备：`/home/aihub/Peter_ws/migration/sub2api-git-20260630-123303.bundle`
  - 已把 PeterAI 画图页静态文件纳入仓库：`deploy/static/image-generator/`
  - 已运行 `deploy/publish-image-generator.sh`，仓库静态文件与容器 volume 文件 hash 一致。
  - 已运行 `deploy/test-with-docker.sh backend`：`ok github.com/Wei-Shaw/sub2api/internal/service`
  - 已运行 `deploy/test-with-docker.sh frontend`：3 个测试文件、13 个测试通过。
- 运行态检查结果：
  - 当前运行镜像：`sub2api-custom:20260628`
  - 本机健康检查：`http://127.0.0.1:18080/health -> {"status":"ok"}`
  - 公网健康检查：`https://api.peterai.cc.cd/health -> {"status":"ok"}`
  - 当前画图页版本：`image-timeout-motion-20260627`
  - 公网 `main.js` 语法检查通过，`single_dollar_forEach = 0`
  - 图片价格仍为每张 `0.1`：所有用户组 `1K / 2K / 4K` 最小值和最大值均为 `0.10000000`

### 2026-06-28

- 按本手册完成上游同步：
  - `upstream/main` 从 `85a3b122` 更新到 `c2754222`，上游版本同步到 `v0.1.139`。
  - 本地 `main` 已 reset 到 `upstream/main`。
  - `custom/gallery` 已 rebase 到最新 `main`，保留 GPT-5.5 默认模型、图片画廊、PeterAI 画图相关修复、失败图片不扣费、same-origin embedded static pages、使用教程页等自定义能力。
  - `gpt55-defaults` 已 rebase 到最新 `main`。
- 冲突处理：
  - `backend/cmd/server/wire_gen.go`：保留上游新增的 Grok OAuth / Compliance wiring，同时保留画廊 handler 和 `GalleryService` cleanup。
  - `frontend/src/components/keys/UseKeyModal.vue`：保留上游 Claude Code `CLAUDE_CODE_ATTRIBUTION_HEADER=0`，并继续复用本 fork 的 `frontend/src/utils/clientConfig.ts` 配置生成器。
- 备份：
  - Git 备份分支：`custom/gallery-backup-20260628-222516`
  - Git 备份分支：`gpt55-defaults-backup-20260628-222848`
  - 数据库备份：`/home/aihub/Peter_ws/migration/sub2api_pg_before_upstream_20260628_223316.dump`
- 验证：
  - 前端相关单测通过：`clientConfig.spec.ts`、`UsageTutorialView.spec.ts`、`AppSidebar.spec.ts`。
  - `npm run typecheck` 通过。
  - `npm run build` 通过，仅有既有 Vite chunk / dynamic import 警告。
  - Docker 镜像构建通过：`sub2api-custom:20260628`。
  - 本机健康检查通过：`http://127.0.0.1:18080/health -> {"status":"ok"}`。
  - 公网健康检查通过：`https://api.peterai.cc.cd/health -> {"status":"ok"}`。
  - 公网画图页 `main.js` 语法检查通过，`single_dollar_forEach = 0`。
  - 图片价格仍为每张 `0.1`：所有用户组 `1K / 2K / 4K` 最小值和最大值均为 `0.10000000`。
- 发布：
  - 已部署镜像：`sub2api-custom:20260628`
  - `deploy/.env` 已指向 `SUB2API_IMAGE=sub2api-custom:20260628`。
  - 仅重建应用容器 `sub2api`，Postgres / Redis 未重建。
- 远程状态：
  - 当前环境缺少 GitHub HTTPS 凭据，`git push origin main` 返回 `fatal: could not read Username for 'https://github.com': No such device or address`。
  - 本地分支已完成更新，但 `origin/main`、`origin/custom/gallery`、`origin/gpt55-defaults` 尚未推送。

### 2026-06-27

- 修复 PeterAI 画图页 `生成失败 HTTP 502`：
  - 真实日志显示请求 `/v1/images/generations` 路由到 `account_id=4562 FX_image2`，上游 `https://www.findcg.com/v1/images/generations` 返回 TCP `connect: connection refused`。
  - 生图组 `group_id=25` 原本只有 `4562 FX_image2` 可调度，已暂停该账号调度并启用同组 `4565 Krill生图`。
  - 前端静态版本更新为 `image-502-failover-20260627`，已覆盖容器 `/app/data/public/image-generator/index.html` 和 `main.js`，并同步数据库 `settings.custom_menu_items` 的 iframe URL 查询参数。
  - `image-generator-main.js` 为 Images API 和 Images Edit API 增加 retry/progress，HTTP 502/连接拒绝时不再 0.6 秒直接静默失败，会显示重试状态和更准确提示。
  - 后端源码修复 Images API transport/network 错误：`backend/internal/service/openai_images.go` 和 `backend/internal/service/openai_images_responses.go` 改为返回 `handleOpenAIUpstreamTransportError(...)`，让连接拒绝/超时进入账号 failover。
  - 已构建并部署镜像 `sub2api-custom:image-502-failover-20260627`，`deploy/.env` 已指向该镜像。
- 优化 PeterAI 画图页长时间无响应和动画问题：
  - 现象：页面显示“正在通过备用接口生成...”后 100 多秒才提示 `Images API 服务暂时不可用，15秒后重试`。
  - 真实原因：当前可调度图片上游长时间不返回图片流数据；后端默认 `GATEWAY_IMAGE_STREAM_DATA_INTERVAL_TIMEOUT=900`，等待过长。
  - 已把 `deploy/.env` 设置为 `GATEWAY_IMAGE_STREAM_DATA_INTERVAL_TIMEOUT=90` 并重启应用容器。
  - 前端版本更新为 `image-timeout-motion-20260627`：`IMAGE_REQUEST_TIMEOUT_MS=90000`，单次 Images API 请求超过 90 秒无响应会主动中断并重试。
  - 前端直接注入 `.gen-spinner` / `.gen-btn-spin` 动画样式，避免静态 CSS 缓存或 iframe 样式问题导致生成动画看起来不转。
- 排查 PeterAI 画图页“生成成功但主界面显示生成失败”的问题：
  - 用户看到的错误为 `$(...).forEach is not a function`。
  - 根因候选之一是历史记录渲染中误用单元素选择器 `$()` 后调用 `.forEach()`；该异常会从 `saveToHistory()` 冒泡到生成流程外层 `catch`，导致已生成图片的主界面被覆盖成“生成失败”。
  - 维护时必须确认实际公网加载的 `main.js`，不能只检查本地 `/home/aihub/Peter_ws/image-generator-main.js`。
- 已确认当前自定义菜单中的画图 iframe URL 来自数据库 `settings.custom_menu_items`：
  - `https://api.peterai.cc.cd/image-generator/`
  - 不是 `127.0.0.1:18080`，也不是 `api.peteraix.com`。
- 画图页静态覆盖路径：
  - 容器内：`/app/data/public/image-generator/`
  - Docker volume：`peter-sub2api_sub2api_data`
- 验证公网画图页时必须检查：
  - `curl -k -sS https://api.peterai.cc.cd/image-generator/`
  - `main.js` 查询参数是否已换版本号。
  - 公网 `main.js` 中 `single_dollar_forEach = 0`。
- 历史记录功能修复目标：
  - 生成成功后主画图界面保留结果，不被历史保存/渲染异常覆盖。
  - 历史记录缩略图可打开预览。
  - 新历史记录保存原图后可下载原图、发布画廊、填入文生图提示词。
  - 旧历史记录如果没有原图数据，只能提示原图不可用，不能用缩略图冒充原图。

### 2026-06-23

- 更新本文档结构：增加目录，大章节表示大更新域，子章节记录具体更新点。
- 当前本地分支：`custom/gallery`
- 当前本地提交领先远程 `origin/custom/gallery` 1 个提交：
  - `28483280 fix image billing for empty responses`
- `git push origin custom/gallery` 多次无输出挂起，疑似到 GitHub 的网络连接阻塞；本地提交和线上部署已完成，但远程尚未更新。
- 当前线上容器运行镜像：
  - `sub2api-custom:20260622`
- 当前公网入口：
  - `https://api.peterai.cc.cd`
- PeterAI 画图页面已确认显示：
  - `统一价每张 $0.1`
  - JS 常量 `PRICE_PER_IMAGE = 0.1`

### 2026-06-22

- 已把 `origin/main` 更新到最新 `upstream/main`。
- 已把 `custom/gallery` rebase 到最新 `main`。
- 已把 `gpt55-defaults` rebase 到最新 `main`。
- 最终分支状态：
  - `main`：`85a3b122`
  - `gpt55-defaults`：`16d636a1`
  - `custom/gallery`：`8b0e15bb`
- 当前 `custom/gallery` 包含：
  - `Default Codex model to GPT-5.5`
  - `Add image gallery feature`
- 本地已创建备份分支：
  - `custom/gallery-before-upstream-20260622`
  - `gpt55-defaults-before-upstream-20260622`
- 完成 Docker 迁移部署：
  - Compose 项目名：`peter-sub2api`
  - 本机端口：`127.0.0.1:18080`
  - 镜像：`sub2api-custom:20260622`
- 完成 Cloudflare Tunnel 配置：
  - `api.peterai.cc.cd -> http://localhost:18080`
- 完成管理员配置：
  - `Daojie.Peng@qq.com`
  - `1312194755@qq.com`
- 完成图片价格调整：
  - 所有组 `1K / 2K / 4K` 图片价格为 `0.1`
- 修复失败图片扣费问题并退款：
  - usage log `30294` 到 `30306`
  - 共 13 条
  - 共退还 `6.5`
