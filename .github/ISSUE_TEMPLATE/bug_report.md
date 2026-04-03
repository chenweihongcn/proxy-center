name: Bug Report
description: 提交 Bug 或问题
title: "[BUG] "
labels: ["bug"]
assignees: []

body:
  - type: markdown
    attributes:
      value: |
        感谢您提交 Bug 报告！请提供尽可能多的细节，这将帮助我们快速定位问题。

  - type: textarea
    attributes:
      label: 问题描述
      description: 清楚地描述您遇到的问题
      placeholder: "我尝试执行 X，但发生了 Y..."
    validations:
      required: true

  - type: textarea
    attributes:
      label: 复现步骤
      description: 重现问题的具体步骤
      placeholder: |
        1. 第一步
        2. 第二步
        3. 看到问题...
    validations:
      required: true

  - type: textarea
    attributes:
      label: 预期行为
      description: 应该发生什么
      placeholder: "应该正确显示/执行..."
    validations:
      required: true

  - type: textarea
    attributes:
      label: 实际行为
      description: 实际发生了什么
      placeholder: "出现错误/崩溃/卡顿..."
    validations:
      required: true

  - type: dropdown
    attributes:
      label: 环境信息
      description: 您使用的环境
      options:
        - "Docker on iStoreOS"
        - "Docker on Linux (other)"
        - "Docker on Windows"
        - "Direct build on iStoreOS"
        - "Direct build on Linux"
        - "其他"
    validations:
      required: true

  - type: textarea
    attributes:
      label: 系统信息
      description: 输出以下命令的结果抄贴到此
      placeholder: |
        uname -a
        docker version
        go version (如适用)
    validations:
      required: false

  - type: textarea
    attributes:
      label: 错误日志
      description: 粘贴相关的错误信息或日志
      placeholder: "docker logs proxy-center 的输出..."
      render: text
    validations:
      required: false

  - type: textarea
    attributes:
      label: 其他信息
      description: 任何其他相关信息
      placeholder: "环境特殊配置、已尝试的解决方案等..."
    validations:
      required: false

  - type: checkboxes
    attributes:
      label: 确认
      options:
        - label: "我已阅读 [文档](../deploy/ISTOREIOS_DEPLOYMENT.md)"
          required: false
        - label: "我检查了是否已有类似的 Issue"
          required: false
        - label: "我愿意提供更多信息以帮助调查"
          required: false
