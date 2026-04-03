# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.0.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

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
