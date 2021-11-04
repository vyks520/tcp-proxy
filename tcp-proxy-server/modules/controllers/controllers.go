package controllers

import (
	"errors"
	"fmt"
	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
	"io"
	"net"
	"net/http"
	"tcp-proxy/modules/jwt"
	. "tcp-proxy/modules/log"
	. "tcp-proxy/modules/types"
	"tcp-proxy/modules/utils"
	. "tcp-proxy/modules/wsUpgrade"
	. "tcp-proxy/tcp-proxy-server/modules/config"
	. "tcp-proxy/tcp-proxy-server/modules/global"
	"time"
)

func Cors() gin.HandlerFunc {
	return func(c *gin.Context) {
		method := c.Request.Method
		c.Header("Access-Control-Allow-Origin", "*")
		c.Header("Access-Control-Allow-Headers", "Content-Type,Authorization")
		c.Header("Access-Control-Allow-Methods", "POST, GET, OPTIONS")
		c.Header("Access-Control-Expose-Headers", "Content-Length, Access-Control-Allow-Origin, Access-Control-Allow-Headers, Content-Type")
		c.Header("Access-Control-Allow-Credentials", "true")
		//放行所有OPTIONS方法
		if method == "OPTIONS" {
			c.AbortWithStatus(http.StatusNoContent)
		}
		// 处理请求
		c.Next()
	}
}

// 客户端注册，推送请求
func ClientRegister(c *gin.Context) {
	cConn, err := Upgrade(c.Writer, c.Request, 1024, 1024)
	if err != nil {
		Logger.Errorf("URL: %s,  连接错误: %s", c.Request.URL.String(), err.Error())
		return
	}
	defer func() {
		_ = cConn.Close()
	}()
	reqUrl := c.Request.URL.String()
	serverID := c.Param("serverID")
	clientSecret := c.Param("clientSecret")
	timestamp := utils.ToInt64(c.Param("timestamp"))

	token, err := TokenCreate(jwt.LoginInfo{ServerID: serverID, ClientSecret: clientSecret, Timestamp: timestamp}, reqUrl)
	if err != nil {
		_ = cConn.WriteJSON(TaskDataJson{
			Status: 1,
			Msg:    fmt.Sprintf("serverID: %s, %s", serverID, err.Error()),
			Method: "verify",
		})
	}

	proxyInfo, err := getProxyInfo(serverID)
	if err != nil {
		Logger.Errorf("URL: %s, %s", reqUrl, err.Error())
		return
	}
	if _, ok := ProxyTaskQueue[proxyInfo.ServerID]; ok {
		_ = cConn.WriteJSON(TaskDataJson{
			Status: 1,
			Msg:    fmt.Sprintf("server_id: %s, 已登录，不能重复登录，请退出其他已登录的代理客户端后重试", proxyInfo.ServerID),
			Method: "verify",
		})
		return
	}

	sListener, err := net.Listen("tcp", proxyInfo.ServerAddr)
	if err != nil {
		Logger.Errorf("server_id: %s,  监听'%s'时发生错误: %s", proxyInfo.ServerID, proxyInfo.ServerAddr, err.Error())
		_ = cConn.WriteJSON(TaskDataJson{
			Status: 1,
			Msg:    fmt.Sprintf("server_id: %s,  监听'%s'时发生错误，请联系管理员", proxyInfo.ServerID, proxyInfo.ServerAddr),
			Method: "verify",
		})
		return
	}
	defer func() {
		_ = sListener.Close()
	}()
	Logger.Infof("server_id: %s, 代理服务器地址: %s, 服务已启动！", proxyInfo.ServerID, proxyInfo.ServerAddr)
	//初始化当前server_id任务列表
	ProxyTaskQueue[proxyInfo.ServerID] = map[string]ProxyTaskQueueItem{}
	defer ProxyTaskQueueClose(proxyInfo.ServerID)

	//清理10秒未处理的超时任务队列
	go ProxyTaskQueueTimeoutClear(proxyInfo.ServerID, 10)

	err = cConn.WriteJSON(TaskDataJson{
		Status: 0,
		Msg:    "ok",
		Method: "verify",
		Token:  token,
	})
	if err != nil {
		Logger.Errorf("server_id: %s, 响应代理客户端验证结果错误: %s", proxyInfo.ServerID, err.Error())
		return
	}

	ch := make(chan bool)
	closed := false

	go func() {
		timestamp := time.Now().Unix()
		//token有效期60分钟，50分钟推送新token
		expireTime := int64(3000)
		for {
			time.Sleep(time.Second * 3)
			if closed {
				return
			}
			if time.Now().Unix()-timestamp < expireTime {
				continue
			}
			timestamp = time.Now().Unix()
			newToken, err := refreshToken(token)
			if err != nil {
				errMsg := fmt.Sprintf("serverID: %s, token刷新失败, %s", serverID, err.Error())
				Logger.Error(errMsg)
				err = cConn.WriteJSON(TaskDataJson{
					Status: 1,
					Msg:    errMsg,
					Method: "verify",
				})
				ch <- true
				return
			}
			_ = cConn.WriteJSON(TaskDataJson{
				Status: 0,
				Msg:    "ok",
				Method: "verify",
				Token:  newToken,
			})
			Logger.Debugf("serverID: %s, token刷新成功！", serverID)
		}
	}()

	go func() {
		for {
			reqConn, err := sListener.Accept()
			if closed {
				return
			}
			if err != nil {
				Logger.Errorf("server_id: %s, %s", proxyInfo.ServerID, err.Error())
				ch <- true
				return
			}
			taskID := utils.UUID()
			ProxyTaskQueue[proxyInfo.ServerID][taskID] = ProxyTaskQueueItem{
				ID:       taskID,
				Conn:     reqConn,
				TimeUnix: time.Now().Unix(),
				Status:   false,
			}

			err = cConn.WriteJSON(TaskDataJson{
				Status: 0,
				Msg:    "ok",
				Method: "request",
				TaskID: taskID,
			})
			if err != nil {
				Logger.Errorf("server_id: %s, WriteJSON error: %s", proxyInfo.ServerID, err.Error())
				ch <- true
				return
			}
			Logger.Debugf("server_id: %s, taskID: %s, 新请求推送成功！", proxyInfo.ServerID, taskID)
		}
	}()
	go func() {
		for {
			// 客户端关闭连接时服务器同步结束
			_, _, err = cConn.ReadMessage()
			if closed {
				return
			}
			if err != nil {
				ch <- true
				return
			}
		}
	}()
	<-ch
	closed = true
	Logger.Infof("server_id: %s, 代理服务器地址: %s, 服务已停止！", proxyInfo.ServerID, proxyInfo.ServerAddr)
}

//接收客户端任务请求处理
func TaskHandle(c *gin.Context) {
	cConn, err := Upgrade(c.Writer, c.Request, 4096, 4096)
	if err != nil {
		Logger.Errorf("URL: %s,  连接错误: %s", c.Request.URL.String(), err.Error())
		return
	}
	defer func() {
		_ = cConn.Close()
	}()

	reqToken := c.GetHeader("Authorization")
	proxyInfo, err := reqAuth(reqToken)
	if err != nil {
		errMsg := fmt.Sprintf("URL: %s, %s", c.Request.URL, err.Error())
		Logger.Errorf(errMsg)
		err = cConn.WriteJSON(TaskDataJson{
			Status: 1,
			Msg:    errMsg,
			Method: "verify",
		})
		if err != nil {
			Logger.Errorf("server_id: %s, 响应代理客户端验证结果错误: %s", proxyInfo.ServerID, err.Error())
		}
		return
	}

	taskID := c.Param("taskID")
	task, ok := ProxyTaskQueue[proxyInfo.ServerID][taskID]
	if !ok {
		errMsg := fmt.Sprintf("server_id: %s, 请求的任务'%s'不存在！", proxyInfo.ServerID, taskID)
		Logger.Error(errMsg)
		err := cConn.WriteJSON(TaskDataJson{
			Status: 1,
			Msg:    errMsg,
			Method: "notify",
		})
		if err != nil {
			Logger.Error()
		}
		return
	}
	defer func() {
		_ = task.Conn.Close()
		delete(ProxyTaskQueue[proxyInfo.ServerID], taskID)
		Logger.Debugf("server_id: %s, taskID: %s, 连接已关闭！", proxyInfo.ServerID, taskID)
	}()

	ch := make(chan bool)
	closed := false

	go func() {
		buffer := make([]byte, 4096)
		for {
			n, err := task.Conn.Read(buffer)
			if closed {
				return
			}
			if err != nil {
				ch <- true
				if err != io.EOF {
					Logger.Errorf("server_id: %s, 读取'%s'数据错误: %s", proxyInfo.ServerID, proxyInfo.ServerAddr, err.Error())
				}
				return
			}
			err = cConn.WriteMessage(websocket.BinaryMessage, buffer[:n])
			if err != nil {
				ch <- true
				Logger.Errorf("server_id: %s, 向代理客户端写数据出错: %s", proxyInfo.ServerID, err.Error())
				return
			}
		}
	}()

	go func() {
		for {
			msgType, data, err := cConn.ReadMessage()
			if closed {
				return
			}
			if err != nil {
				ch <- true
				return
			}
			if msgType != 2 {
				ch <- true
				Logger.Errorf("server_id: %s, 代理客户端websocket消息类型不正确: %s", proxyInfo.ServerID, err.Error())
				continue
			}
			_, err = task.Conn.Write(data)
			if err != nil {
				ch <- true
				Logger.Errorf("server_id: %s, 写数据出错: %s", proxyInfo.ServerID, err.Error())
				return
			}
		}
	}()
	<-ch
	closed = true
}

func TokenCreate(cInfo jwt.LoginInfo, reqUrl string) (token string, err error) {
	var ok bool
	var proxyInfo = ProxyItem{}
	proxyInfo, ok = ServerMap[cInfo.ServerID]
	if !ok {
		Logger.Errorf("URL: %s, serverID不存在，认证失败！", reqUrl)
		return "", errors.New("serverID或客户端密钥不正确，认证失败")
	}

	errMsg, err := jwt.ClientSecretVerifier(
		cInfo.ClientSecret,
		jwt.LoginInfo{
			ServerID:     proxyInfo.ServerID,
			ClientSecret: proxyInfo.ClientSecret,
			Timestamp:    cInfo.Timestamp,
		})
	if err != nil {
		Logger.Errorf("URL: %s, %s", reqUrl, err.Error())
		return "", errors.New(errMsg)
	}

	claims := jwt.Claims{ServerID: proxyInfo.ServerID}
	token, err = jwt.GetToken(&claims, 3600)
	if err != nil {
		Logger.Errorf("URL: %s, %s", reqUrl, err.Error())
		return "", err
	}
	return token, nil
}

func refreshToken(oToken string) (token string, err error) {
	claims, err := jwt.ParserToken(oToken)
	if err != nil {
		return "", err
	}
	token, err = jwt.GetToken(claims, 3600)
	if err != nil {
		return "", err
	}
	return token, nil
}

func reqAuth(token string) (proxyInfo ProxyItem, err error) {
	claims, err := jwt.ParserToken(token)
	if err != nil {
		return proxyInfo, errors.New(err.Error())
	}
	return getProxyInfo(claims.ServerID)
}

func getProxyInfo(serverID string) (proxyInfo ProxyItem, err error) {
	var ok bool
	proxyInfo, ok = ServerMap[serverID]
	if !ok {
		errMsg := fmt.Sprintf("serverID: '%s'不存在", proxyInfo.ServerID)
		return proxyInfo, errors.New(errMsg)
	}
	return proxyInfo, nil
}
