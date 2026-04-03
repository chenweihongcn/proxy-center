# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.0.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [1.0.2] - 2026-04-03

### Fixed
- 调整 GitHub Actions 的依赖准备流程，允许在 runner 中自动整理模块依赖并生成缺失的 `go.sum` 条目
- 修复缓存键仅依赖 `go.sum` 导致无 `go.sum` 仓库在 CI 中直接报错的问题
- 升级 lint action 版本，降低后续 GitHub Actions 运行时兼容风险

## [1.0.1] - 2026-04-03

### Fixed
- 修复 `.gitignore` 规则过宽导致 `cmd/proxyd/main.go` 未被纳入仓库的问题
- 修复 `main.version` 注入缺失，恢复 Dockerfile 和 ARM 构建脚本的版本注入兼容性
- 修复设备端构建脚本中管理员密码变量未展开的问题
- 调整 GitHub Actions 镜像发布到 GHCR，避免首发依赖额外 Docker Hub secrets

## [1.0.0] - 2026-04-03

### Added
- ✅ Complete SOCKS5 & HTTP CONNECT proxy support
- ✅ User authentication with BCrypt hashing
- ✅ User management (CRUD + PATCH updates)
- ✅ Flow control (per-day/month/total quotas)
- ✅ Session management with concurrent connection limits
- ✅ Automatic policy enforcement (expiry/disabled/quota checks)
- ✅ Web management console with responsive UI
- ✅ CSV batch user import
- ✅ Upstream proxy pool with health checks
- ✅ Access logging and audit trails
- ✅ Docker multi-platform support (arm64 + amd64)
- ✅ iStoreOS one-click deployment script
- ✅ Complete deployment documentation

### Infrastructure
- Go 1.23 pure implementation (no CGO)
- SQLite persistence with automatic migrations
- Alpine Linux container images
- Docker Compose orchestration
- GitHub Actions CI/CD pipeline

### Documentation
- Quick Reference guide
- Complete deployment manual
- API documentation
- Troubleshooting guide
- Contribution guidelines

## [Unreleased]

### Planned Features
- [ ] gRPC management interface
- [ ] User groups and advanced RBAC
- [ ] Traffic rate limiting (QoS)
- [ ] Geo-based routing
- [ ] Multi-node cluster support
- [ ] Prometheus metrics export
- [ ] Advanced logging (ELK integration)
- [ ] Web UI translations (i18n)

### Improvements
- [ ] Unit test coverage (target: >80%)
- [ ] Integration tests
- [ ] Performance benchmarks
- [ ] Security audit
- [ ] ARM32 support (armv7l)
- [ ] Build optimization for embedded systems

---

For older releases, see [GitHub Releases](https://github.com/chenweihongcn/proxy-center/releases)
