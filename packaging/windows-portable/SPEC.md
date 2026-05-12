# Windows Portable Packaging Spec

## 目标

生成 `tokenbridge.zip`，用户下载后解压即可直接使用。

## 目标目录结构

```text
TokenBridge/
├── tokenbridge.exe
├── config.yaml
├── data/
├── logs/
└── README.txt
```

## 开箱即用要求

1. 无需 Go / Node 环境。
2. Admin 前端静态资源内嵌到二进制。
3. 首次启动自动初始化 `config.yaml`、`data/` 和 `logs/`。
4. 默认监听 `127.0.0.1:18743`。
5. 首次打开浏览器即进入 `/admin`。

## 打包步骤

1. 构建 Admin 前端产物。
2. 同步前端构建结果到 `build/embed/admin`。
3. 生成 Windows 图标和版本资源。
4. 构建 `tokenbridge.exe`。
5. 复制 `config.yaml` 模板。
6. 组装 `build/portable/TokenBridge/`。
7. 压缩为 `tokenbridge.zip` 并生成校验信息。
