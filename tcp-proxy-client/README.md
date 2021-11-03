# **tcp代理客户端**

一款配置超级简单的tcp内网穿透代理程序，可以将内网tcp端口转发到公网服务器上。测试过内网HTTP服务、windows远程桌面、ssh访问等场景的代理。新手上路，请多指教！

注意：`需要配合服务端使用`

## 注册系统服务Linux
```
# Linux
./tcp-proxy-client.exe install
./tcp-proxy-client.exe remove
# 启动服务
systemctl start tcp-proxy-client.service
# 停止服务
systemctl stop tcp-proxy-client.service

```

## 注册系统服务Windows

```
tcp-proxy-client.exe install
tcp-proxy-client.exe remove
:: 启动服务
net start tcp-proxy-client
:: 停止服务
net stop tcp-proxy-client

```


## 配置文件config.json
`注意: 配置文件位于主程序相同目录下`

```json5
{
  //服务端代理任务处理地址，对应代理服务端[外网IP:port]
  //如果代理服务端使用nginx等代理软件部署在https站点下，配置为[wss://域名/路径]
  "task_addr": "ip:9000",
  //日志等级可选值，panic、fatal、error、warn、info、debug、trace
  "log_level": "debug",
  //on: 输出到屏幕及日志文件; stdout: 只输出到屏幕; file: 只输出到文件; off:关闭日志功能
  "log_output": "on",
  //代理信息配置列表，代理一个端口配置一个
  "proxy_list": [
    {
      //对应服务器配置
      "server_id": "proxy001",
      "client_secret": "d0592e31-e70b-4945-bcde-d886cc092eb0",
      //代理的目标地址, 如果需要将本地80端口转发到服务器就如以下配置即可
      "proxy_target_addr": "127.0.0.1:80"
    },
    {
      "server_id": "proxy002",
      "client_secret": "d0592e31-e70b-4945-bcde-d886cc092eb0",
      "proxy_target_addr": "127.0.0.1:90"
    }
  ]
}
```

