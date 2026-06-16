# macOS 打包说明

GitHub Actions 桌面端 workflow 会构建 Wails universal macOS app，并打包为：

```text
TokenBridge-macos-universal.dmg
```

第一版发布流程不做签名。未签名构建适合内部测试，但 macOS Gatekeeper 可能会在用户打开应用时提示无法验证开发者。

## 后续签名与公证

公开分发 macOS 应用时，建议增加 Developer ID 签名和 Apple notarization 公证。

需要配置的 GitHub Secrets：

```text
APPLE_CERTIFICATE_P12
APPLE_CERTIFICATE_PASSWORD
APPLE_TEAM_ID
APPLE_API_KEY_ID
APPLE_API_ISSUER_ID
APPLE_API_KEY_P8
```

预期签名和公证流程：

1. 把 Developer ID Application 证书导入临时 keychain。
2. 使用 Wails 构建 `TokenBridge.app`。
3. 使用 `codesign --deep --force --options runtime --timestamp` 签名 `.app`。
4. 创建 DMG。
5. 使用 `xcrun notarytool submit --wait` 提交公证。
6. 使用 `xcrun stapler staple` 装订公证票据。
7. 把已公证 DMG 上传到 GitHub Release。
