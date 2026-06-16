# Windows 打包说明

GitHub Actions 桌面端 workflow 会构建 Wails Windows 应用，并发布：

```text
TokenBridge-windows-amd64.exe
```

第一版发布流程不做签名。未签名构建适合内部测试，但 Windows SmartScreen 可能会在用户打开可执行文件时出现安全提示。

## 后续代码签名

公开分发 Windows 应用时，建议增加 Authenticode 签名。

需要配置的 GitHub Secrets：

```text
WINDOWS_CERTIFICATE_PFX
WINDOWS_CERTIFICATE_PASSWORD
```

预期签名流程：

1. 使用 Wails 构建 `TokenBridge.exe`。
2. 导入或引用签名证书。
3. 使用 `signtool` 签名可执行文件。
4. 验证签名。
5. 把已签名 EXE 上传到 GitHub Release。
