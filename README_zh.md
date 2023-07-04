# MirageDebug

[English](./README.md) | 简体中文

MirageDebug：为 Kubernetes 应用提供本地远程调试，实现完全真实的环境调试。

![MirageDebug](./docs/draw/flow.d2.svg)

## 安装

### 安装 MirageDebug

目前，MirageDebug 需要 GO 运行时环境进行安装，并可使用以下方法安装：

```bash
go install github.com/miragedebug/miragedebug/cmd/mirage-debug@latest
```

## 使用

### MirageDebug 服务器 - 后台服务

MirageDebug 服务器是一个后台服务，用于管理调试会话并提供与调试会话相关的信息。

#### 启动 MirageDebug 服务器

```bash
mirage-debug server
```

### 初始化调试应用

在项目根目录中执行以下命令以初始化调试应用，并根据提示填写相关信息。

```bash
mirage-debug init
```

### 编写 IDE 配置文件

MirageDebug 可以自动生成不同 IDE 的调试配置文件，方便在本地启动调试。

```bash
mirage-debug config <APPNAME>
```

### 开始调试

一旦配置了 IDE，您可以直接在 IDE 中开始调试。

## 演示

### 在 Kubernetes 集群中使用 VSCode 调试 Rust 应用

[![在 K8s 中使用 MirageDebug 调试 Rust 应用：使用 VSCode 从本地调试 ztunnel](https://img.youtube.com/vi/RpggulEd48M/0.jpg)](https://www.youtube.com/watch?v=RpggulEd48M)

### 在 Kubernetes 集群中使用 Goland 调试 istiod

[![MirageDebug：从本地集群中使用 Goland 调试 Istio（引导发现）](https://img.youtube.com/vi/ZwG0uaG72_8/0.jpg)](https://www.youtube.com/watch?v=ZwG0uaG72_8)
