# Homepage

Safari New Tab 风格的个人主页：纯色柔和背景随系统亮暗切换，收藏网站以大图标网格展示，打开过一次后可完全离线、秒开。

单个 Go 进程同时提供页面和 API，前端资源通过 `go:embed` 打进可执行文件，运行时不需要 Node.js。

## 功能

- 网站按分组以大图标网格展示，默认分组置顶且不显示标题
- 图标绑定域名，在独立的图标管理界面中维护
- 打开过一次后可离线浏览
- 通过 Authelia（OIDC）登录，多用户数据隔离，登录永久有效直到手动退出

完整的业务规则见 [docs/business-rules.md](docs/business-rules.md)。

## 开发

```sh
echo 'dev_user: "eric"' > config.yaml  # 本地开发跳过 OIDC，所有请求都以该用户登录
go run ./cmd/homepage                  # 后端 :8080

cd frontend
npm install
npm run dev                            # Vite 开发服务器，/api、/icons、/auth 代理到 :8080
```

Service Worker 只在生产构建中注册。

## 测试

```sh
go test ./...                               # 单元与接口测试
go test -tags authelia ./internal/auth      # Authelia 契约测试
```

契约测试用 testcontainers 启动真实的 Authelia，完整走一遍登录、多用户和退出流程。需要 Docker 或 Podman socket，并且本地已有以下镜像（测试不会拉取镜像）：

```sh
docker pull docker.io/authelia/authelia:4.39.20
docker pull docker.io/testcontainers/ryuk:0.14.0   # 版本须与 testcontainers-go 的 ReaperDefaultImage 一致
```

Ryuk 负责在测试进程被中断时清理残留容器。

## 构建

```sh
./scripts/build.sh                    # 产物：release/homepage
```

流程：`npm ci` → TypeScript 类型检查 → Vite 构建 `frontend/dist` → Go 测试 → `CGO_ENABLED=0` 编译并嵌入前端。版本号和 commit 通过 `-ldflags` 注入，`homepage -version` 可查看。

## 配置

```yaml
listen: ":8080"                          # 监听地址
data_dir: "./data"                       # SQLite 数据库与图标文件目录
public_url: "https://home.example.com"   # 浏览器访问的 origin
oidc:
  issuer: "https://auth.example.com"
  client_id: "homepage"
  client_secret: ""                      # 建议改用环境变量 HOMEPAGE_OIDC_CLIENT_SECRET
```

通过 `-config path/to/config.yaml` 指定配置文件，默认读取当前目录下的 `config.yaml`。除非设置 `dev_user`，否则 `public_url` 和 `oidc` 必填。

在 Authelia 中注册一个 confidential client：回调地址 `<public_url>/auth/callback`，scope 为 `openid profile email`，授权方式 `authorization_code`，要求 PKCE（S256），认证方式 `client_secret_basic`。

## 发布

推送 `v*` 标签后，GitHub Actions 构建多架构 Docker 镜像并推送到 GHCR。容器内数据目录为 `/app/data`，配置文件需挂载到 `/app/config.yaml`。

```sh
docker run -p 8080:8080 \
  -v homepage-data:/app/data \
  -v ./config.yaml:/app/config.yaml:ro \
  -e HOMEPAGE_OIDC_CLIENT_SECRET=... \
  ghcr.io/<owner>/homepage:latest
```
