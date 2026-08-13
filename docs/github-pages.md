# GitHub Pages 静态发布

把规则站点公开为静态页面，不需要常驻服务器。发布流程由 GitHub Actions 完成，构建与源代码分离，规则配置由你掌控。

## 工作原理

- 源仓库（`fl0w1nd/Proxy-Rule-Manager`）每次发版（打 `v*.*.*` 标签）时，自动发布最新二进制和 Docker 镜像。
- 你的 Fork 仓库里有一条 Pages 工作流：每天 `03:17 UTC` 自动运行一次，也可以手动触发。
- 运行时从源仓库下载最新正式 Release，校验 SHA-256，然后在 `publish` 分支上执行 `prm --data-dir data build --output dist`。
- 产物通过 Pages Artifact 发布。构建中断时，当前线上站点继续服务。

## 首次设置

1. **Fork 仓库**。Fork 时取消勾选 *Copy the DEFAULT branch only*，让 `main` 和 `publish` 分支一起复制，保持 `main` 是默认分支。
2. **检出 `publish` 分支**，放入你的输入：

   ```text
   config.yaml
   data/
   ├── local/
   ├── templates/
   └── static/icons/
   ```

   ```bash
   git checkout publish
   # 编辑 config.yaml；运行数据由工作流的 --data-dir data 指定
   # 把本地文件源放入 data/local/，自定义模板放入 data/templates/，图标放入 data/static/icons/
   git add config.yaml data
   git commit -m "chore: update publish configuration"
   git push origin publish
   ```

3. **启用 Actions**：仓库 **Settings → Actions → General**，允许 Actions 运行。
4. **设置 Pages 来源**：**Settings → Pages**，选择 *GitHub Actions* 作为发布来源。
5. **创建仓库变量**：**Settings → Variables → Actions**，新建 `PRM_PAGES_ENABLED=true`。没有这个变量时构建任务会被跳过。

完成后在 **Actions** 页面手动运行一次 *Publish Rules to GitHub Pages* 工作流，确认站点能构建和发布。

## 敏感配置

配置文件可用 `${ENV_NAME}` 引用环境变量。在仓库 Secret 里新建 `PRM_ENV`，每行一个变量：

```dotenv
SOURCE_TOKEN=example-token
PRIVATE_URL=https://example.com/rules
```

变量名需符合 shell 环境变量格式。工作流在构建前把这些值写入任务环境，`config.yaml` 里对应的 `${SOURCE_TOKEN}` 会被替换。

## 运行与维护

- **自动运行**：每天 `03:17 UTC`。
- **手动运行**：Actions 页面 → *Publish Rules to GitHub Pages* → Run workflow。
- **检查结果**：运行日志和构建错误保留在 Actions 运行记录里。更新错误会终止发布，线上站点不受影响。
- **数据缓存**：`data/.state`、`data/geosite`、`data/rules` 会缓存以加速后续构建。如配置变化后出现异常，可在 Actions 缓存管理中清除缓存再跑。

## 同步上游

只在 `main` 上同步上游提交，`publish` 保持独立历史，配置稳定：

```bash
git checkout main
git fetch upstream
git merge --ff-only upstream/main
git push origin main
```

prm 版本由源仓库的最新 Release 自动推进；同步 `main` 可以取得工作流与文档更新。

## 本地预览

构建前先在本地生成一次，确认配置与来源都正确：

```bash
prm --data-dir data build --output dist
```

产物固定包含 `index.html`、`rules/`、`static/icons/`、`.nojekyll`，用任意静态文件服务器即可预览 `dist/`。
