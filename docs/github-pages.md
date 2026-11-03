# GitHub Pages 静态发布指南

`prm` 支持通过 GitHub Actions 将编译后的规则与公开索引页构建为纯静态站点并发布至 GitHub Pages，无需常驻服务器。

## 架构与工作流机制

仓库采用**双分支隔离**模式管理代码与用户配置：

- **`main` 分支（默认分支）**：承载项目源码与 GitHub Actions 工作流定义（`.github/workflows/pages.yml`）。
- **`publish` 分支**：仅存放用户的 `config.yaml` 及本地静态输入（`data/local/`、`data/templates/`、`data/static/icons/`），与上游源码提交历史完全解耦。

工作流执行流程：

1. 每日 `03:17 UTC` 定时触发（或通过 `workflow_dispatch` 手动触发）。
2. 从上游源仓库拉取最新正式 Release 二进制包并校验 SHA-256 摘要。
3. 检出 `publish` 分支并在其工作目录下执行：
   ```bash
   prm --config config.yaml --data-dir data build --output dist
   ```
4. 将生成的 `dist/` 目录通过 GitHub Pages Artifact 部署上线。若任一规则或上游拉取失败，构建将直接中止，已发布的线上站点保持不变。

## 首次配置步骤

1. **Fork 仓库（包含全部分支）**
   在 GitHub 上 Fork 本仓库时，**取消勾选** `Copy the DEFAULT branch only`，使 `main` 与 `publish` 分支同时复制到你的仓库，并保持 `main` 为默认分支。
2. **在 `publish` 分支提交配置**
   检出 `publish` 分支，按以下目录结构维护配置与自定义文件：
   ```text
   config.yaml
   data/
   ├── local/          # 本地规则文件（可选）
   ├── templates/      # 自定义模板（可选）
   └── static/icons/   # 自定义图标（可选）
   ```
   提交并推送到远端 `publish` 分支：
   ```bash
   git checkout publish
   git add config.yaml data/
   git commit -m "chore: update publish configuration"
   git push origin publish
   ```
3. **启用 GitHub Actions**
   进入 Fork 仓库的 **Settings → Actions → General**，启用 Actions 执行权限。
4. **配置 GitHub Pages 构建源**
   进入 **Settings → Pages**，在 **Build and deployment → Source** 中选择 **GitHub Actions**。
5. **开启发布开关变量**
   进入 **Settings → Secrets and variables → Actions → Variables**，新建 Repository variable：
   - Name：`PRM_PAGES_ENABLED`
   - Value：`true`
6. **触发首次构建**
   进入 **Actions → Publish Rules to GitHub Pages**，点击 **Run workflow** 手动触发首次构建与部署。

## 本地验证与日常维护

### 本地构建测试

在推送 `publish` 分支前，可在本地执行静态构建以验证配置与产物：

```bash
prm --config config.yaml --data-dir data build --output dist
```

`dist/` 目录包含完整的独立静态站点（`index.html`、`rules/`、`geo/`、`static/assets/`、`static/icons/`、`.nojekyll`）。页面资源均采用相对路径引用，兼容 GitHub Pages 的仓库子路径（`https://<user>.github.io/<repo>/`）及自定义域名。

### 同步上游工作流更新

`prm` 编译器的版本在工作流运行时自动跟随上游最新 Release，日常无需手动升级。若需同步上游工作流或文档更新，仅需更新 `main` 分支：

```bash
git checkout main
git fetch upstream
git merge --ff-only upstream/main
git push origin main
```

### 缓存说明

工作流通过 `actions/cache` 缓存 `data/.state`、`data/geosite`、`data/geoip` 与 `data/rules` 目录以保留更新历史并加速后续构建。如需重置状态，可在仓库 **Actions → Caches** 页面清理已有缓存。
