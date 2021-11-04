# **超级简单的tcp内网穿透代理程序**

一款配置超级简单的tcp内网穿透代理程序，可以将内网tcp端口转发到公网服务器上。测试过内网HTTP服务、windows远程桌面、ssh访问等场景的代理。新手上路，请多指教！

## 当前版本： 1.0.2（发布日期：2021-11-04）

## 更新历史

### 1.0.2（2021-11-04）

- 增加日志输出控制功能，参考服务端及客户端log_output字段配置说明。
- 重构身份认证体系，老版本使用密钥明文提交认证，有安全风险，新版本使用JWT认证。

### 1.0.1（2021-10-31）

- 客户端及服务端文档格式调整。
- 增加客户端新增功能配置说明。
- 增加客户端对服务端部署在https站点的支持，参考客户端task_addr字段配置说明。

### 1.0.0（2021-10-30）

- 首次提交

### 使用说明

- [服务端使用说明](./tcp-proxy-server/README.md)

- [客户端使用说明](./tcp-proxy-client/README.md)

- [视频教程](https://b23.tv/6cPgUe)
