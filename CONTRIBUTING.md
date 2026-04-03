# Proxy Center

## 关于

proxy-center 是一个为 iStoreOS/OpenWrt 路由器设计的完整代理管理系统，支持 SOCKS5 + HTTP CONNECT 协议，内置用户管理、流量控制、Web 管理控制台。

**特别针对 NanoPi R2S Plus + iStoreOS 24.10.5 优化**

## ⚡ 快速开始

### Docker 方式（推荐）

```bash
docker compose -f deploy/docker-compose.yml up -d

# 访问 Web UI
curl http://localhost:8090
```

### iStoreOS 一键启动

```bash
ssh root@192.168.50.94
sh -c "$(curl -fsSL https://raw.githubusercontent.com/chenweihongcn/proxy-center/main/deploy/istoreios-quick-start.sh)"
```

## 📦 核心特性

- ✅ **SOCKS5 + HTTP CONNECT** — 完整代理协议支持
- ✅ **用户认证** — 用户名/密码 + BCrypt 哈希
- ✅ **流量控制** — 按日/按月/累计配额限制
- ✅ **会话管理** — 并发连接限制 + 强制踢线
- ✅ **Web 控制台** — 内嵌响应式 UI
- ✅ **上游路由** — 代理池 + 健康检查 + 权重轮询
- ✅ **审计日志** — 操作追踪 + 访问记录
- ✅ **Docker 容器化** — Multi-arch 支持 (arm64 + amd64)

## 🚀 使用场景

### 家庭网络代理

在 iStoreOS 路由器上运行，为家庭设备提供统一代理出口

### VPN 补充

配合 VPN 使用，按用户/流量进行精细化控制

### 团队代理

企业内网代理管理，支持员工账户隔离和流量审计

## 📚 文档

- [快速参考](QUICK_REFERENCE.md) — 5 分钟快速了解
- [完整部署指南](deploy/ISTOREIOS_DEPLOYMENT.md) — 详细部署手册
- [交付清单](DELIVERY_CHECKLIST.md) — 功能验收列表

## 🔧 开发

### 项目结构

```
cmd/proxyd/           # 主服务入口
internal/
  ├── proxy/          # SOCKS5 + HTTP CONNECT 实现
  ├── auth/           # 认证与授权
  ├── session/        # 会话管理
  ├── store/          # SQLite 数据层
  ├── upstream/       # 上游路由
  ├── policy/         # 政策执行
  ├── traffic/        # 流量计量
  └── web/            # Web API + UI
deploy/               # 容器 + 脚本
```

### 编译

```bash
# 本地编译
CGO_ENABLED=0 go build -o proxyd ./cmd/proxyd

# Docker 构建 (支持多架构)
docker buildx build --platform linux/arm64 -t proxy-center:armv8 .
```

### 测试

```bash
# 运行单元测试 (未来补充)
go test ./...

# 集成测试 (未来补充)
./scripts/integration-test.sh
```

## 🔐 安全

- 密码 BCrypt 哈希存储 (cost=12)
- HTTP Basic Auth 保护 API
- 敏感字段脱敏
- 操作审计日志
- 数据库文件权限限制

## 📊 性能

| 指标 | 值 |
|------|-----|
| 启动时间 | < 5 秒 |
| 内存占用 | ~50-100 MB |
| 并发连接 | 1000+ |
| SOCKS5 吞吐 | 10-50 Mbps |

## 🤝 贡献

欢迎提交 Issue 和 Pull Request！

## 📄 许可证

MIT License — 详见 [LICENSE](LICENSE)

## 📞 支持

- 🐛 [Bug Report](https://github.com/chenweihongcn/proxy-center/issues)
- 💡 [Feature Request](https://github.com/chenweihongcn/proxy-center/issues)
- 💬 [Discussion](https://github.com/chenweihongcn/proxy-center/discussions)

---

Made with ❤️ for iStoreOS & OpenWrt Community
