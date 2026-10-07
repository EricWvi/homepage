# Homepage

Safari New Tab 风格的个人主页：纯色柔和背景随系统亮暗切换，收藏网站以大图标网格展示，打开过一次后可完全离线、秒开。

单个 Go 进程同时提供页面和 API，前端资源通过 `go:embed` 打进可执行文件，运行时不需要 Node.js。

## 功能

- **分组**：默认分组在最上方且不显示标题，自定义分组依次单列排布；支持新建、重命名、排序、删除（删除后网站移入默认分组）。
- **网站**：添加/编辑时填写标题、链接、分组（默认选中默认分组）；右键或长按图标可编辑、删除。
- **图标**：图标绑定域名而非网站，在“图标管理”中上传、更换或移除。添加新域名的网站时自动登记该域名；删除网站不会删除域名，域名需单独删除（仍有网站使用时不可删）。图标文件以内容 hash 命名，HTTP 缓存一年。
- **离线**：快照存于 localStorage，首帧即可渲染；Service Worker 缓存页面、静态资源和图标，后台静默更新。

## 开发

```sh
cp config.example.yaml config.yaml   # 可选，缺省时使用默认值
go run ./cmd/homepage                 # 后端 :8080

cd frontend
npm install
npm run dev                           # Vite 开发服务器，/api 与 /icons 代理到 :8080
```

Service Worker 只在生产构建中注册。

## 构建

```sh
./scripts/build.sh                    # 产物：release/homepage
```

流程：`npm ci` → TypeScript 类型检查 → Vite 构建 `frontend/dist` → Go 测试 → `CGO_ENABLED=0` 编译并嵌入前端。版本号和 commit 通过 `-ldflags` 注入，`homepage -version` 可查看。

## 配置

```yaml
listen: ":8080"      # 监听地址
data_dir: "./data"   # SQLite 数据库与图标文件目录
```

通过 `-config path/to/config.yaml` 指定配置文件，默认读取当前目录下的 `config.yaml`。

## 发布

推送 `v*` 标签后，GitHub Actions 构建多架构 Docker 镜像并推送到 GHCR。容器内数据目录为 `/app/data`，配置文件可挂载到 `/app/config.yaml`。

```sh
docker run -p 8080:8080 -v homepage-data:/app/data ghcr.io/<owner>/homepage:latest
```
