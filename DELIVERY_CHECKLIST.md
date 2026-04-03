# proxy-center v1.0 完整交付清单

## 📋 项目概述

**proxy-center** 是一个为 iStoreOS(NanoPi R2S Plus) 设计的完整代理管理系统，支持：
- ✅ SOCKS5 + HTTP CONNECT 入站协议
- ✅ 用户名/密码认证 + 按账号并发限制
- ✅ 按日/按月/累计流量额度控制
- ✅ 到期自动踢线 + 自动政策强制执行
- ✅ 上游代理池 + 健康检查 + 权重路由
- ✅ Web 管理控制台 + CSV 批量导入
- ✅ 域名日志记录 + 审计追踪
- ✅ Docker 容器部署 + OpenWrt procd 管理

**硬件目标**: ARMv8 (NanoPi R2S Plus, iStoreOS 24.10.5)  
**开发栈**: Go 1.23 + SQLite + Chi + Alpine  
**验证状态**: ✅ 代码完全编译无误，✅ armv8 纯 Go 兼容

---

## 📦 可交付物清单

### 1️⃣ 源代码

#### 核心服务
- [cmd/proxyd/main.go](/cmd/proxyd/main.go) — 主服务入口 (orchestrates all services)
- [internal/config/config.go](/internal/config/config.go) — 环境变量配置加载
- [internal/auth/service.go](/internal/auth/service.go) — 认证与授权引擎
- [internal/store/store.go](/internal/store/store.go) — SQLite 数据访问层 + CRUD
- [internal/session/manager.go](/internal/session/manager.go) — 会话管理 + 强制踢线

#### 协议与网络
- [internal/proxy/http_proxy.go](/internal/proxy/http_proxy.go) — HTTP CONNECT 入站 + Proxy-Authorization 认证
- [internal/proxy/socks5_proxy.go](/internal/proxy/socks5_proxy.go) — SOCKS5 入站 + 用户/密码认证
- [internal/proxy/relay.go](/internal/proxy/relay.go) — 双向流量中继 + 实时计量回调

#### 政策与运维
- [internal/policy/enforcer.go](/internal/policy/enforcer.go) — 后台政策强制执行 (过期/禁用/超额检查)
- [internal/upstream/router.go](/internal/upstream/router.go) — 出口路由 + 上游池选择 + 健康检查
- [internal/traffic/recorder.go](/internal/traffic/recorder.go) — 域名日志 + 流量统计

#### Web 管理
- [internal/web/server.go](/internal/web/server.go) — Web 管理 API + 内嵌 HTML 控制台
  - 用户 CRUD (GET/POST/PATCH/DELETE)
  - 用户批量导入 (CSV)
  - 使用统计查询
  - 实时会话管理 + 强制下线

#### 数据模型
- [internal/store/migrations/](/internal/store/migrations/) — SQLite 迁移脚本 (自动执行)
  - users: 账户基本信息 + 凭证 + 配额
  - usage: 流量计数 (day/month/total)
  - sessions: 活跃会话跟踪
  - domain_logs: 访问域名审计 (30天保留)
  - audit: 管理操作审计 (90天保留)

### 2️⃣ 容器化部署

#### 多架构 Dockerfile
- [Dockerfile](/Dockerfile) — Multi-platform 构建 (支持 linux/amd64 + linux/arm64)
  - 使用 docker buildx for cross-compilation
  - 静态编译 (CGO_ENABLED=0)
  - 优化的两阶段构建
  - 内置健康检查

#### Docker Compose
- [deploy/docker-compose.yml](/deploy/docker-compose.yml) — 完整编排配置
  - 暴露 8080/1080/8090 端口
  - 数据持久化到 ./data
  - 日志驱动配置 (JSON 格式，100MB 滚动)
  - 健康检查集成

### 3️⃣ 编译与部署脚本

#### 交叉编译工具
- [build-armv8.sh](/build-armv8.sh) — Linux bash 脚本 (docker buildx / 远程 SSH 编译)
- [build-armv8.ps1](/build-armv8.ps1) — Windows PowerShell 脚本 (buildx / 远程 iStoreOS ssh 编译)

#### iStoreOS 快速部署
- [deploy/istoreios-quick-start.sh](/deploy/istoreios-quick-start.sh) — 一键启动脚本
  - 自动克隆/更新源代码
  - Docker 镜像本地构建
  - 服务自动启动 + 健康检查
  - 输出快速参考信息

### 4️⃣ 部署与运维文档

- [deploy/ISTOREIOS_DEPLOYMENT.md](/deploy/ISTOREIOS_DEPLOYMENT.md) — 完整部署指南 (**强烈推荐阅读**)
  - 三种部署方案 (Docker/Direct/procd)
  - 环境变量完整清单
  - 上游代理池配置示例
  - 监控与维护手册
  - 常见问题排查
  - 升级与回滚流程
  
- [README.md](/README.md) — 项目主文档
  - API 端点列表
  - 配置说明
  - 快速开始

### 5️⃣ 验证与测试

#### 代码质量
- ✅ 静态分析: 零编译错误 (all packages)
- ✅ 架构兼容性: 纯 Go (无 CGO), 完全支持 linux/arm64
- ✅ 依赖检查: go.mod 仅使用稳定、轻量级库
  - github.com/go-chi/chi/v5 — HTTP 路由
  - golang.org/x/crypto — BCrypt 密钥哈希
  - modernc.org/sqlite — 纯 Go SQLite 驱动

---

## 🚀 快速部署步骤

### 方案 A: iStoreOS 上一键启动（最简单）

在 iStoreOS 上执行：

```bash
ssh root@192.168.50.94

# 一键部署
sh -c "$(curl -fsSL https://raw.githubusercontent.com/chenweihongcn/proxy-center/main/deploy/istoreios-quick-start.sh)"

# 或手动下载后运行
cd /opt/proxy-center
wget https://raw.githubusercontent.com/chenweihongcn/proxy-center/main/deploy/istoreios-quick-start.sh
sh istoreios-quick-start.sh
```

**预期结果**（约 3-5 分钟，取决于网络）:
```
✅ proxy-center started successfully!
📋 Quick reference:
   Web UI:       http://192.168.50.94:8090
   Username:     admin
  Password:     [自动生成]
   HTTP Proxy:   http://192.168.50.94:8080
   SOCKS5:       socks5://192.168.50.94:1080
```

### 方案 B: 从 Windows 交叉编译镜像

```powershell
# PowerShell 5.1

# 使用 docker buildx 构建 armv8 镜像
.\build-armv8.ps1 -Version "1.0" -Registry "your-registry.com"

# 或指定远程 iStoreOS 编译
.\build-armv8.ps1 -iStoreOSIP 192.168.50.94 -Version "1.0"

# 推送到镜像仓库（可选）
docker tag proxy-center:armv8-1.0 your-registry/proxy-center:armv8-1.0
docker push your-registry/proxy-center:armv8-1.0
```

### 方案 C: 手动在 iStoreOS 编译

```bash
ssh root@192.168.50.94

cd /tmp
git clone https://github.com/chenweihongcn/proxy-center.git
cd proxy-center

# 直接编译
CGO_ENABLED=0 go build -o proxyd ./cmd/proxyd

# 或用 Docker 构建镜像
docker build -t proxy-center:armv8-1.0 .
docker-compose up -d
```

---

## 🔧 使用示例

### 1. 修改管理员密码

```bash
# Web 界面：http://192.168.50.94:8090
# 用户名: admin
# 密码: [看 PROXY_ADMIN_PASS 环境变量]
# → 点击"用户管理" → 编辑 admin 用户 → 保存
```

### 2. 创建代理账户

```bash
# 通过 Web 界面添加
# 或 API 方式

curl -X POST http://192.168.50.94:8090/api/users \
  -H "Authorization: Basic $(echo -n 'admin:your-password' | base64)" \
  -H "Content-Type: application/json" \
  -d '{
    "username": "alice",
    "password": "secret123",
    "role": "user",
    "max_conns": 3,
    "quota_day_mb": 500,
    "quota_month_mb": 5000,
    "quota_total_mb": 50000
  }'
```

### 3. 批量导入用户 (CSV)

```bash
# 准备 users.csv
cat > /tmp/users.csv << 'EOF'
username,password,role,enabled,expires_at,max_conns,quota_day_mb,quota_month_mb,quota_total_mb
alice,pwd1,user,true,2026-12-31,3,500,5000,50000
bob,pwd2,user,true,2026-12-31,5,1000,10000,100000
charlie,pwd3,operator,true,,10,2000,20000,200000
EOF

# 导入
curl -X POST http://192.168.50.94:8090/api/users/import-csv \
  -H "Authorization: Basic $(echo -n 'admin:password' | base64)" \
  -F "file=@/tmp/users.csv"
```

### 4. 配置上游代理池

```bash
# 编辑 docker-compose.yml 中的环境变量
export PROXY_EGRESS_MODE=pool
export PROXY_EGRESS_POOL="http://upstream1:8080;10 http://upstream2:8080;20 socks5://upstream3:1080;5"

# 重启服务
docker-compose restart proxy-center
```

### 5. 查看日志

```bash
# Docker 方式
docker-compose logs -f proxy-center

# 或看 OpenWrt procd
/etc/init.d/proxy-center status
logread -f | grep proxy-center
```

---

## 📊 性能指标

### 测试环境
- **硬件**: NanoPi R2S Plus (ARMv8, 2GB RAM, eMMC storage)
- **操作系统**: iStoreOS 24.10.5 / Linux 6.6.119
- **Docker**: 预装，支持 arm64 镜像

### 基准数据（预期）
| 指标 | 值 |
|-----|-------|
| 启动时间 | < 5s |
| 内存占用 | ~50-100 MB (idle) |
| 单连接延迟 | < 1ms (LAN) |
| SOCKS5 吞吐 | ~10-50 Mbps (受限于网络) |
| 并发连接数 | 1000+ (test 无限制) |
| 数据库大小 | ~100MB (含 1 个月日志) |

---

## 🔐 安全建议

### 立即行动
- [ ] 修改默认管理员密码 (`PROXY_ADMIN_PASS`)
- [ ] 启用 HTTPS（配置反向代理，如 Nginx）
- [ ] 启用防火墙入站规则（仅允许 LAN）
- [ ] 备份 `/data/proxy-center.db` 定期备份

### 生产部署
- [ ] 使用强密码 (20+ 字符)
- [ ] 定期轮换管理员凭证
- [ ] 启用审计日志并定期审查
- [ ] 配置日志中心（ELK/Splunk）
- [ ] 设置告警规则（异常登陆、配额超限）

---

## 📞 故障排查

### 常见问题

| 问题 | 解决方案 |
|----|---------|
| Web 无法登陆 | 检查 `PROXY_ADMIN_PASS`，重启服务 |
| SOCKS5 连接超时 | 检查防火墙规则；`docker logs proxy-center` |
| 内存持续上升 | 清理过期日志；`sqlite3 /data/proxy-center.db "DELETE FROM domain_logs WHERE created_at < datetime('now', '-30 days');"` |
| 镜像拉取失败 | 配置 Docker 镜像加速器，或手动构建 |

### 调试命令

```bash
# 检查服务状态
docker ps | grep proxy-center

# 查看最近日志
docker logs --tail=50 proxy-center

# 进入容器
docker exec -it proxy-center sh

# 测试 SOCKS5
curl -x socks5://admin:password@192.168.50.94:1080 http://example.com

# 测试 HTTP CONNECT
curl -x http://admin:password@192.168.50.94:8080 https://example.com

# 查看活跃会话
docker exec proxy-center proxyd sessions list
```

---

## 📈 后续扩展方向

### Phase 2（可选）
- [ ] gRPC 管理接口
- [ ] 用户组 + 高级 RBAC
- [ ] VPN 隧道 (WireGuard)
- [ ] 带宽限流 + QoS
- [ ] 地域路由 + IP 黑名单

### Phase 3（远期）
- [ ] 多节点集群 + 负载均衡
- [ ] 流量回源 + CDN 集成
- [ ] ML 异常检测
- [ ] OpenAPI 文档 + 自动化 SDK

---

## 📚 文档导航

#### 用户文档
- [快速开始](#快速部署步骤) — 3 选 1 部署方案
- [iStoreOS 部署指南](deploy/ISTOREIOS_DEPLOYMENT.md) — 完整运维手册
- [API 参考](README.md) — 所有端点列表

#### 开发文档  
- [项目架构](README.md#architecture) — 模块设计
- [代码结构](#可交付物清单) — 文件组织
- [构建指南](#快速部署步骤) — 编译流程

#### 运维文档
- [监控告警](deploy/ISTOREIOS_DEPLOYMENT.md#监控与维护) — 性能监控
- [备份恢复](deploy/ISTOREIOS_DEPLOYMENT.md#数据备份) — 数据保护
- [升级回滚](deploy/ISTOREIOS_DEPLOYMENT.md#升级与回滚) — 版本管理

---

## ✅ 验收清单

### 代码质量
- [x] 代码通过静态分析 (zero errors)
- [x] armv8 架构兼容 (pure Go, no CGO)
- [x] 依赖版本稳定 (go 1.23)
- [x] 注释完整 (key functions documented)

### 功能完整性
- [x] SOCKS5 入站 + 认证
- [x] HTTP CONNECT 入站 + 认证
- [x] 用户+配额+过期管理
- [x] 会话踢线 + 政策强制
- [x] 上游路由 + 健康检查
- [x] Web 控制台 + CSV 导入
- [x] 日志审计 + 域名记录
- [x] Docker 容器化 + Compose

### 部署就绪
- [x] Multi-platform Dockerfile (amd64 + arm64)
- [x] 交叉编译脚本 (bash + PowerShell)
- [x] 一键启动脚本 (iStoreOS)
- [x] 完整部署文档
- [x] 运维手册

### 文档完整
- [x] 项目 README
- [x] iStoreOS 部署指南
- [x] API 参考
- [x] 故障排查指南

---

## 📝 版本信息

- **版本号**: v1.0.0
- **发布日期**: 2026-04-03
- **Go 版本**: 1.23+
- **目标平台**: iStoreOS 24.10.5 (ARMv8)
- **兼容架构**: linux/arm64, linux/amd64

---

## 📄 许可证

[Your License Here - e.g., MIT, GPL-3.0, etc.]

---

## 🙏 贡献与支持

- 问题反馈: https://github.com/chenweihongcn/proxy-center/issues
- 讨论区: https://github.com/chenweihongcn/proxy-center/discussions
- 贡献指南: [CONTRIBUTING.md](CONTRIBUTING.md)

---

**Last Updated**: 2026-04-03T10:33:37Z
