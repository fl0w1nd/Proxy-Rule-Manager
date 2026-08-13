# GitHub Pages 静态发布

该方案将上游源码与用户配置分开保存：`main` 跟随上游更新，`publish` 保存配置和本地资源，生成结果由 GitHub Pages Artifact 承载。

## 首次设置

1. Fork 仓库，并保留 `main` 为默认分支。
2. 在本地创建 `publish` orphan 分支，仅提交以下输入：

   ```text
   config.yaml
   data/
   ├── local/
   ├── templates/
   └── static/icons/
   ```

   ```bash
   git switch --orphan publish
   git rm -rf .
   mkdir -p data/local data/templates data/static/icons
   # 添加 config.yaml 和所需本地资源
   git add config.yaml data
   git commit -m "chore: initialize publish configuration"
   git push -u origin publish
   ```

3. 确认 `config.yaml` 使用 `data_dir: ./data`。本地文件源位于 `data/local/`，自定义模板位于 `data/templates/`，自定义图标位于 `data/static/icons/`。
4. 在仓库的 **Settings → Actions → General** 中启用 Actions，在 **Settings → Pages** 中选择 **GitHub Actions** 作为发布来源。
5. 新建 Repository Variable：`PRM_PAGES_ENABLED=true`。
6. 新建 Repository Variable：`PRM_VERSION=v0.0.1`。升级 PRM 时明确修改该版本号。
7. 建议为 `publish` 启用分支保护，并限制直接推送范围。

工作流支持手动运行，并在每天 `03:17 UTC` 自动运行。构建日志和错误详情保留在 Actions 运行记录中；更新错误会终止发布，现有 Pages 版本继续提供服务。

## 敏感配置

配置文件可使用 `${ENV_NAME}`。在 Repository Secret `PRM_ENV` 中按行保存对应值：

```dotenv
SOURCE_TOKEN=example-token
PRIVATE_URL=https://example.com/rules
```

变量名需符合 shell 环境变量格式，每个值占一行。工作流会在构建前将这些值写入任务环境。

## 同步上游

只在 `main` 同步上游提交：

```bash
git switch main
git fetch upstream
git merge --ff-only upstream/main
git push origin main
```

`publish` 保持独立提交历史；上游同步操作限定在 `main`，用户配置因此保持稳定。

## 本地预览构建

在包含 `config.yaml` 的目录执行：

```bash
prm build --output dist
```

成功输出固定包含 `index.html`、`rules/`、`static/icons/` 和 `.nojekyll`。可用任意静态文件服务器预览 `dist/`。
