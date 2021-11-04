package jwt

import (
	"crypto/md5"
	"errors"
	"fmt"
	"github.com/dgrijalva/jwt-go"
	"math"
	"tcp-proxy/modules/utils"
	"time"
)

/*
iss: jwt签发者
sub: jwt所面向的用户
aud: 接收jwt的一方
exp: jwt的过期时间，这个过期时间必须要大于签发时间
nbf: 定义在什么时间之前，该jwt都是不可用的.
iat: jwt的签发时间
jti: jwt的唯一身份标识，主要用来作为一次性token,从而回避重放攻击
*/

//随机字符串加盐
var secret = ""

type Claims struct {
	jwt.StandardClaims
	ServerID string `json:"server_id"`
}

func init() {
	secret = utils.UUID()
}

//token字符串获取
func GetToken(claims *Claims, expireTime int64 /*token有效期*/) (string, error) {
	claims.IssuedAt = time.Now().Unix()
	claims.ExpiresAt = time.Now().Add(time.Second * time.Duration(expireTime)).Unix()
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	signedToken, err := token.SignedString([]byte(secret))
	if err != nil {
		return "", errors.New("登陆令牌获取失败，请重试或联系管理员")
	}
	return signedToken, nil
}

//token解析
func ParserToken(strToken string) (*Claims, error) {
	if len(strToken) == 0 {
		return nil, errors.New("用户未登陆授权，拒绝访问")
	}
	token, err := jwt.ParseWithClaims(strToken, &Claims{}, func(token *jwt.Token) (interface{}, error) {
		return []byte(secret), nil
	})
	if err != nil {
		return nil, errors.New("令牌不在有效期或令牌无效，请重新登陆")
	}
	claims, ok := token.Claims.(*Claims)
	if !ok {
		return nil, errors.New("JWTClaims解析失败,请尝试重新登陆")
	}
	if err := token.Claims.Valid(); err != nil {
		return nil, errors.New("令牌不在有效期，请重新登陆")
	}
	return claims, nil
}

type LoginInfo struct {
	ServerID     string
	ClientSecret string
	Timestamp    int64
}

// 加密密钥
func EncryptSecret(info LoginInfo) LoginInfo {
	//与服务器时间不能相差5分钟
	//timestamp := time.Now().Unix()
	secretString := fmt.Sprintf("%s%s%d", info.ServerID, info.ClientSecret, info.Timestamp)
	secret := md5.New()
	secret.Write([]byte(secretString))
	info.ClientSecret = fmt.Sprintf("%X", secret.Sum(nil))
	return info
}

// 客户端密钥验证
func ClientSecretVerifier(
	cSecret string  /*客户端提交的已加密密钥*/,
	sInfo LoginInfo /*服务端登录信息*/,
) (errMsg string, err error) {
	//与服务器时间不能相差5分钟
	expireTime := float64(5 * 60)
	timestamp := time.Now().Unix()
	if math.Abs(float64(timestamp-sInfo.Timestamp)) > expireTime {
		errMsg = "时间戳已过期，认证失败"
		return errMsg, errors.New(errMsg)
	}

	sInfo = EncryptSecret(sInfo)
	if sInfo.ClientSecret != cSecret {
		errMsg = "serverID或客户端密钥不正确，认证失败"
		return errMsg, errors.New("客户端密钥不正确，认证失败")
	}
	return errMsg, nil
}
