# proxy-center v1.0

基于 Go 的 iStoreOS/OpenWrt 代理中心完整实现，为 NanoPi R2S Plus 特别优化。

## 🎯 核心特性

### 代理协议
- ✅ **SOCKS5** — RFC1928 完整实现，支持用户名/密码认证
- ✅ **HTTP CONNECT** — RFC2817 代理支持，Proxy-Authorization Basic 认证

### 用户管理
- ✅ 账户启停、过期时间控制
- ✅ 按账号并发连接限制 (1-255)
- ✅ 流量配额：按日 / 按月 / 累计
- ✅ Web 管理控制台 + CSV 批量导入
- ✅ 进阶 CRUD API (GET/POST/PATCH/DELETE)

### 政策执行
- ✅ 自动政策检查 (~2s 周期)
- ✅ 过期/禁用/超额用户自动踢线
- ✅ 拒绝新连接请求
- ✅ 实时流量计量 (每包更新)

### 出口路由
- ✅ 直连模式
- ✅ 单上游代理
- ✅ 上游代理池（权重轮询）
- ✅ 自动健康检查 (TCP 探针)
- ✅ 故障自动切换

### 运维功能
- ✅ 访问域名日志（30 天保留）
- ✅ 审计日志（管理操作跟踪）
- ✅ SQLite 数据库（内置备份）
- ✅ Docker 容器化部署
- ✅ OpenWrt procd/init 服务管理

## 🚀 快速启动

### 方案 A: iStoreOS 一键部署（推荐）

```bash
# SSH 到 NanoPi
ssh root@192.168.50.94

# 一键启动
sh -c "$(curl -fsSL https://raw.githubusercontent.com/chenweihongcn/proxy-center/main/deploy/istoreios-quick-start.sh)"
```

**预期输出**：
```
✅ proxy-center started successfully!
Web UI:       http://192.168.50.94:8090
SOCKS5:       socks5://192.168.50.94:1080
HTTP Proxy:   http://192.168.50.94:8080
Username:     admin
Password:     [自动生成]
```

### 方案 B: Docker Compose

```bash
# 本地开发/测试
docker compose -f deploy/docker-compose.yml up -d

# iStoreOS 上运行
cd /opt/proxy-center
docker-compose up -d
```

### 方案 C: 直接编译运行

```bash
CGO_ENABLED=0 go build -o proxyd ./cmd/proxyd
./proxyd
```

## 📝 默认配置

首次启动会自动创建管理员账号：

- 用户名：`admin`（可通过 `PROXY_ADMIN_USER` 覆盖）
- 密码：`change-me-now`（可通过 `PROXY_ADMIN_PASS` 覆盖）
  
**⚠️ 强烈建议立即修改默认密码**

## 主要环境变量

- `PROXY_DB_PATH`：SQLite 路径，默认 `./data/proxy-center.db`
- `PROXY_HTTP_LISTEN`：HTTP 代理监听，默认 `:8080`
- `PROXY_SOCKS_LISTEN`：SOCKS5 监听，默认 `:1080`
- `PROXY_WEB_LISTEN`：Web 管理监听，默认 `:8090`
- `PROXY_EGRESS_MODE`：`direct` / `http-upstream` / `socks5-upstream`
- `PROXY_EGRESS_ADDR`：上游代理地址（上游模式必填）
- `PROXY_EGRESS_USER` / `PROXY_EGRESS_PASS`：上游认证
- `PROXY_EGRESS_MODE=pool`：启用代理池
- `PROXY_EGRESS_POOL`：代理池配置，逗号分隔，例如 `http://u:p@10.0.0.2:3128?weight=2,socks5://10.0.0.3:1080?weight=1`
- `PROXY_HEALTH_TICK`：上游池健康检查周期，默认 `10s`
- `PROXY_LOG_DOMAINS`：是否记录域名日志（默认 `true`）

## 管理 API（内网）

所有 `/api/*` 需要 HTTP Basic（管理员账号）。

- `GET /healthz`
- `GET /`（Web 控制台）
- `GET /api/dashboard`
- `GET /api/users`
- `POST /api/users`
- `POST /api/users/import-csv`
- `PATCH /api/users/{username}`
- `POST /api/users/{username}/kick`
- `POST /api/users/{username}/reset-usage`
- `DELETE /api/users/{username}`
- `GET /api/active-connections`
- `GET /api/upstreams`

`POST /api/users` 示例：

```json
{
  "username": "u001",
  "password": "p001",
  "role": "user",
  "enabled": true,
  "expires_at": 1775404800,
  "max_conns": 2,
  "quota_day_mb": 1024,
  "quota_month_mb": 20480,
  "quota_total_mb": 102400
}
```

`PATCH /api/users/{username}` 示例：

```json
{
  "enabled": true,
  "expires_at": 0,
  "max_conns": 3,
  "quota_day_mb": 2048,
  "quota_month_mb": 40960,
  "quota_total_mb": 0
}
```

`POST /api/users/import-csv` 示例（text/csv）：

```csv
username,password,role,enabled,expires_at,max_conns,quota_day_mb,quota_month_mb,quota_total_mb
u101,p101,user,true,0,2,1024,20480,0
u102,p102,user,true,1775404800,1,512,10240,51200
```

- 已存在用户：按提供字段增量更新（留空字段不改）。
- 新用户：必须提供 `password`。

## 下一步建议

- 管理端增加 RBAC、审计日志查询和密码轮换
- 增加按用户/域名的上游选路规则
- 增加管理员修改用户策略（启停、到期、限额）接口
