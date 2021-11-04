# **tcp代理服务端**

一款配置超级简单的tcp内网穿透代理程序，可以将内网tcp端口转发到公网服务器上。测试过内网HTTP服务、windows远程桌面、ssh访问等场景的代理。新手上路，请多指教！

注意：`需要配合客户端使用`

## 注册系统服务Linux

```
# Linux
./tcp-proxy-server.exe install
./tcp-proxy-server.exe remove
# 启动服务
systemctl start tcp-proxy-server.service
# 停止服务
systemctl stop tcp-proxy-server.service

```

## 注册系统服务Windows

```
tcp-proxy-server.exe install
tcp-proxy-server.exe remove
:: 启动服务
net start tcp-proxy-server
:: 停止服务
net stop tcp-proxy-server

```

## 配置文件config.json

`注意: 配置文件位于主程序相同目录下`

```json5
{
  //客户端连接的任务处理监听地址端口
  "Host": "0.0.0.0",
  "Port": 9000,
  //日志等级可选值，panic、fatal、error、warn、info、debug、trace
  "log_level": "debug",
  //on: 输出到屏幕及日志文件; stdout: 只输出到屏幕; file: 只输出到文件; off:关闭日志功能
  "log_output": "on",
  //代理信息配置列表，代理一个端口配置一个
  "proxy_list": [
    {
      //客户端连接服务器时配置
      "server_id": "proxy001",
      //客户端代理到服务端的地址
      "server_addr": "0.0.0.0:9001",
      //客户端认证密钥
      "client_secret": "d0592e31-e70b-4945-bcde-d886cc092eb0"
    },
    {
      "server_id": "proxy002",
      "server_addr": "0.0.0.0:9002",
      "client_secret": "d0592e31-e70b-4945-bcde-d886cc092eb0"
    }
  ]
}
```

