# Sub2API Fork 维护手册

这份文档记录当前 fork 的分支策略、部署方式、自定义功能、迁移状态和关键运维操作。原则是：`main` 保持干净同步上游，自定义能力集中维护在 `custom/gallery`，线上镜像从 `custom/gallery` 构建。

## 目录

- [一、分支与上游同步](#一分支与上游同步)
  - [1.1 当前分支模型](#11-当前分支模型)
  - [1.2 日常同步流程](#12-日常同步流程)
  - [1.3 冲突处理原则](#13-冲突处理原则)
- [二、Docker 迁移部署](#二docker-迁移部署)
  - [2.1 当前部署状态](#21-当前部署状态)
  - [2.2 镜像构建策略](#22-镜像构建策略)
  - [2.3 Compose 隔离约定](#23-compose-隔离约定)
  - [2.4 数据恢复与备份](#24-数据恢复与备份)
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
- [七、更新记录](#七更新记录)
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
- 应用镜像：`sub2api-custom:20260622`
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

### 4.2 图片画廊

画廊功能在 `custom/gallery` 中维护。当前要求：

- 生成后的图片可以发布到画廊。
- 最近生成和画廊展示应使用清晰图片。
- 下载应尽量使用原图，不使用压缩预览图。
- 静态页面位于 Docker volume 的 `/app/data/public/image-generator/`。

线上静态文件可通过 volume 检查：

```bash
sg docker -c 'docker run --rm -v peter-sub2api_sub2api_data:/data alpine ls -la /data/public/image-generator'
```

### 4.3 PeterAI 画图页面

当前页面入口：

```text
https://api.peterai.cc.cd/image-generator/
```

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
backend/internal/service/openai_images_test.go
```

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
sg docker -c 'docker compose -f /home/aihub/Peter_ws/sub2api/deploy/docker-compose.yml --env-file /home/aihub/Peter_ws/sub2api/deploy/.env up -d --force-recreate sub2api'
curl -sS https://api.peterai.cc.cd/health
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

## 七、更新记录

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
