# Fork 维护说明

这个仓库按“干净同步上游 + 独立自定义功能分支”的方式维护。

## 当前分支模型

```text
upstream/main
    |
origin/main
    |
custom/gallery
```

各分支职责：

- `main`：干净同步 `upstream/main`。不要在这里放自己的功能改动。
- `gpt55-defaults`：最新 `main` + GPT-5.5 默认模型改动。
- `custom/gallery`：最新 `main` + GPT-5.5 默认模型改动 + 图片画廊功能。

当前远端：

- `upstream`：`https://github.com/Wei-Shaw/sub2api.git`
- `origin`：`https://github.com/Bingtao-Wang/sub2api.git`

## 为什么这样维护

`main` 保持干净，只负责跟源项目同步。这样以后源项目更新时，可以直接把
`main` 更新到上游最新版，不会和自己的功能混在一起。

自己的功能以 Git 提交的形式放在 `custom/gallery` 上。上游更新后，把
`custom/gallery` 重新 rebase 到新的 `main` 上，就能做到：

- 获得源项目最新代码
- 保留自己的功能改动
- 清楚区分哪些是官方代码，哪些是自己的代码

可以理解为：

```text
旧官方源码 -> 自己的功能提交
```

更新后变成：

```text
新官方源码 -> 自己的功能提交
```

## 日常从上游更新

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

如果还要继续保留单独的 `gpt55-defaults` 分支，也同步更新它：

```bash
git checkout gpt55-defaults
git rebase main
git push --force-with-lease origin gpt55-defaults
```

`rebase` 会改写功能分支历史，所以推送时需要用 `--force-with-lease`。
它比普通 `--force` 安全，因为如果远端分支在你上次拉取之后被别人改过，
它会拒绝覆盖。

## 遇到冲突怎么办

如果执行：

```bash
git rebase main
```

时出现冲突，先看冲突文件：

```bash
git status
```

打开冲突文件，原则是：

- 保留上游新增的代码
- 保留自己的功能代码
- 如果两边改到同一段逻辑，需要人工判断怎么合并

解决后继续：

```bash
git add <已经解决的文件>
GIT_EDITOR=true git rebase --continue
```

如果发现 rebase 方向不对，可以中止：

```bash
git rebase --abort
```

重要更新前建议先建备份分支：

```bash
git branch custom/gallery-backup-$(date +%Y%m%d) custom/gallery
```

## Docker 构建策略

如果要运行自己的定制版本，Docker 镜像应该从 `custom/gallery` 构建，
不要从 `main` 构建。

推荐：

```bash
git checkout custom/gallery
docker build -t sub2api-custom:YYYYMMDD .
```

然后在 `deploy/docker-compose.yml` 中使用自己的镜像，不再使用：

```yaml
image: weishaw/sub2api:latest
```

示例：

```yaml
image: sub2api-custom:YYYYMMDD
```

不建议只使用 `latest` 标签。带日期或版本号的镜像更容易回滚。

## 每次更新后的检查

更新完成后，检查 `custom/gallery` 相比 `main` 多了哪些提交：

```bash
git log --oneline origin/main..origin/custom/gallery
git diff --stat origin/main..origin/custom/gallery
```

正常情况下，`custom/gallery` 应该只多出自己的功能提交，例如：

- `Default Codex model to GPT-5.5`
- `Add image gallery feature`

然后验证构建和运行：

```bash
docker build -t sub2api-custom:YYYYMMDD .
cd deploy
docker compose up -d
docker compose logs -f sub2api
```

## 更新记录

以后重要更新、冲突处理、部署信息、回滚点都记录在这里。

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
