# Homepage

Safari New Tab 风格的个人主页：纯色柔和背景随系统亮暗切换，收藏网站以大图标网格展示，打开过一次后可完全离线、秒开。

单个 Go 进程同时提供页面和 API，前端资源通过 `go:embed` 打进可执行文件，运行时不需要 Node.js。

## 功能

- 常驻搜索框：Enter 开始输入，输入时列出匹配的网站和子链接，Tab 选择建议；没有建议时 Tab 切换搜索引擎
- 网站可以带同域名的子链接，例如 GitHub 上常用的几个仓库
- 桌面端壁纸：壁纸库管理上传的壁纸，空闲一段时间或手动进入全屏壁纸界面，← → 切换
- 网站按分组以大图标网格展示，默认分组置顶且不显示标题
- 图标绑定域名，在独立的图标管理界面中维护
- 打开过一次后可离线浏览
- 通过 Authelia（OIDC）登录，多用户数据隔离，登录永久有效直到手动退出

完整的业务规则见 [docs/business-rules.md](docs/business-rules.md)。

## 开发

常用命令都在 [Taskfile.yml](Taskfile.yml) 里，需要先安装 [Task](https://taskfile.dev)。运行 `task --list` 查看全部任务。

```sh
task install:frontend                  # 安装前端依赖（npm ci）
echo 'dev_user: "eric"' > config.yaml  # 本地开发跳过 OIDC，所有请求都以该用户登录
task run:server                        # 后端 :36749
task run:web                           # Vite 开发服务器，/api、/icons、/wallpapers、/auth 代理到 :36749
```

`task run:server -- -config other.yaml` 可以把参数透传给 `homepage`。Service Worker 只在生产构建中注册。

## 测试

```sh
task format          # gofmt 格式化 Go 代码
task lint            # 前端类型检查、gofmt 检查、go vet
task test            # lint + Go 单元与接口测试
task test:contract   # Authelia 契约测试
```

契约测试用 testcontainers 启动真实的 Authelia，完整走一遍登录、多用户和退出流程，不包含在 `task test` 中。需要 Docker 或 Podman socket，并且本地已有以下镜像（测试不会拉取镜像）：

```sh
docker pull docker.io/authelia/authelia:4.39.20
docker pull docker.io/testcontainers/ryuk:0.14.0   # 版本须与 testcontainers-go 的 ReaperDefaultImage 一致
```

Ryuk 负责在测试进程被中断时清理残留容器。

## 构建

```sh
task build           # 产物：release/homepage
```

`task build` 调用 `scripts/build.sh`：`npm ci` → TypeScript 类型检查 → Vite 构建 `frontend/dist` → 嵌入检查 → Go 测试 → `CGO_ENABLED=0` 编译并嵌入前端。嵌入检查（`scripts/check-embed.sh`）用 `go list` 确认 `index.html`、`sw.js`、`favicon.svg` 以及 `index.html` 引用的全部打包资源都会被嵌入，缺失即构建失败，因此产物运行时不需要任何前端文件。版本号和 commit 通过 `-ldflags` 注入，`homepage -version` 可查看。`task clean` 删除 `release/` 和已构建的 `frontend/dist`。

## 配置

```yaml
listen: ":36749"                         # 监听地址
data_dir: "./data"                       # SQLite 数据库、图标与壁纸文件目录
idle_wait: "5m"                          # 桌面端无操作多久后进入壁纸界面，单位 h/m/s，默认 5m
public_url: "https://home.example.com"   # 浏览器访问的 origin
oidc:
  issuer: "https://auth.example.com"
  client_id: "homepage"
  client_secret: ""                      # 建议改用环境变量 HOMEPAGE_OIDC_CLIENT_SECRET
```

通过 `-config path/to/config.yaml` 指定配置文件，默认读取当前目录下的 `config.yaml`。除非设置 `dev_user`，否则 `public_url` 和 `oidc` 必填。

在 Authelia 中注册一个 confidential client：回调地址 `<public_url>/auth/callback`，scope 为 `openid profile email`，授权方式 `authorization_code`，要求 PKCE（S256），认证方式 `client_secret_basic`。

## 发布

推送 `v*` 标签（如 `v1.2.3`）后，GitHub Actions 构建 `linux/amd64`、`linux/arm64` 镜像并推送到 GHCR，标签为版本号（`1.2.3`）和 `latest`；带 `-` 的预发布版本（如 `v1.3.0-rc.1`）不更新 `latest`。也可以在 Actions 页面手动运行 release 工作流并填写版本号。镜像构建与 `task build` 走同样的类型检查、嵌入检查和 Go 测试。

容器内数据目录为 `/app/data`，配置文件需挂载到 `/app/config.yaml`。

```sh
docker run -p 36749:36749 \
  -v homepage-data:/app/data \
  -v ./config.yaml:/app/config.yaml:ro \
  -e HOMEPAGE_OIDC_CLIENT_SECRET=... \
  ghcr.io/<owner>/homepage:latest
```
