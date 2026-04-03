# Release Checklist

用于 proxy-center 版本发布的完整检查清单。

## 发布前准备

### 代码审查
- [ ] 所有 Pull Request 已评审且合并
- [ ] 代码通过 CI/CD 测试
- [ ] 没有待解决的 TODO 注释
- [ ] 代码注释完整且准确
- [ ] 没有调试代码（console.log, print 等）

### 文档更新
- [ ] [README.md](README.md) 已更新
- [ ] [CHANGELOG.md](CHANGELOG.md) 已更新，包含新增功能和 Bug 修复
- [ ] [QUICK_REFERENCE.md](QUICK_REFERENCE.md) 已检查
- [ ] [ISTOREIOS_DEPLOYMENT.md](deploy/ISTOREIOS_DEPLOYMENT.md) 已检查
- [ ] API 文档已更新（如有变更）

### 版本标记
- [ ] 决定版本号（遵循 [Semantic Versioning](https://semver.org/)）
- [ ] 版本号已在以下文件中更新：
  - [ ] go.mod (go version)
  - [ ] README.md (Version badge)
  - [ ] CHANGELOG.md

### 代码质量检查
- [ ] 代码通过 `go fmt`
- [ ] 代码通过 `go vet`
- [ ] 代码通过 `golangci-lint` (本地)
- [ ] 单元测试覆盖率 > 70%
- [ ] 集成测试通过
- [ ] 没有编译警告

### 安全检查
- [ ] 依赖项已更新至最新稳定版本
- [ ] 没有已知的安全漏洞 (`go list -json -m all | nancy sleuth`)
- [ ] 密钥/凭证未提交到仓库
- [ ] 敏感信息已脱敏

## 发布流程

### 构建阶段
1. [ ] 清理工作目录：`git clean -fdx`
2. [ ] 确保在 main 分支：`git checkout main`
3. [ ] 获取最新更改：`git pull origin main`
4. [ ] 创建发布标签：`git tag -a v1.0.0 -m "Release v1.0.0"`
5. [ ] 推送标签：`git push origin v1.0.0`
6. [ ] 本地构建测试：
   - [ ] Linux: `CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o proxyd ./cmd/proxyd`
   - [ ] macOS: `CGO_ENABLED=0 GOOS=darwin GOARCH=amd64 go build -o proxyd ./cmd/proxyd`
   - [ ] armv8: `CGO_ENABLED=0 GOOS=linux GOARCH=arm64 go build -o proxyd ./cmd/proxyd`

### Docker 镜像构建
- [ ] 多平台镜像构建成功：
  ```bash
  docker buildx build --platform linux/amd64,linux/arm64 \
    -t your-registry/proxy-center:v1.0.0 \
    -t your-registry/proxy-center:latest \
    --push .
  ```
- [ ] Docker Hub 上镜像可拉取：`docker pull your-registry/proxy-center:v1.0.0`
- [ ] 镜像在 armv8 设备上运行正常

### 发布包生成
- [ ] 源代码发布包已生成：
  ```bash
  ./scripts/build-release.sh 1.0.0
  ```
- [ ] SHA256 校验和已生成
- [ ] 压缩包完整性已验证

### GitHub 发布
- [ ] GitHub Releases 页面已创建：
  - [ ] Tag 已推送
  - [ ] Release Notes 已编写（基于 CHANGELOG.md）
  - [ ] 发布物已上传：
    - [ ] `proxy-center-1.0.0-source.tar.gz`
    - [ ] `proxy-center-1.0.0-source.zip`
    - [ ] `SHA256SUMS`
  - [ ] Release 标记为 "Latest" (如适用)

### 测试验证（iStoreOS）
- [ ] iStoreOS 上 Docker 镜像运行正常
- [ ] Web UI 可访问且功能正常
- [ ] SOCKS5 代理可连接
- [ ] HTTP CONNECT 代理可连接
- [ ] 用户管理功能正常
- [ ] 流量统计准确

## 发布后通知

- [ ] 在 GitHub Discussions 发表发布说明
- [ ] 更新 OpenWrt/iStoreOS 社区论坛（如适用）
- [ ] 邮件或社交媒体通知用户（如有）
- [ ] 标记相关 Issue 为已解决

## 回滚计划

若发现严重问题，执行回滚：

1. 撤销 Git Tag:
   ```bash
   git tag -d v1.0.0
   git push origin --delete v1.0.0
   ```

2. 删除发布：在 GitHub Releases 页面删除

3. 撤销 Docker 镜像：
   ```bash
   docker image rm your-registry/proxy-center:v1.0.0
   ```

4. 通知用户已知问题，建议暂时使用前一个稳定版本

## 候选发布版本 (RC) 

对于重大版本，建议先发布 RC 版本：

- [ ] RC 准备：创建 `release/v1.0.0-rc1` 分支
- [ ] RC 测试：邀请社区 Beta 测试
- [ ] 收集反馈并修复关键 Bug
- [ ] 发布最终版本

---

**发布日期**: ________  
**发布人**: ________  
**检查人**: ________  

**发布完成时间**: ________  
**发布验证**: ✓ / ✗

**备注**:  
_________________________________
_________________________________
