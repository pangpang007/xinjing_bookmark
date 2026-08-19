package middleware

import (
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"
	"github.com/soupcircle/bookjie-api/utils"
)

const UserIDKey = "user_id"

type Claims struct {
	UserID int64 `json:"uid"`
	jwt.RegisteredClaims
}

type JWTAuth struct {
	secret []byte
	expiry time.Duration
}

func NewJWTAuth(secret string, expiry time.Duration) *JWTAuth {
	return &JWTAuth{secret: []byte(secret), expiry: expiry}
}

func (j *JWTAuth) Generate(userID int64) (string, error) {
	now := time.Now()
	claims := Claims{
		UserID: userID,
		RegisteredClaims: jwt.RegisteredClaims{
			IssuedAt:  jwt.NewNumericDate(now),
			ExpiresAt: jwt.NewNumericDate(now.Add(j.expiry)),
		},
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString(j.secret)
}

func (j *JWTAuth) Parse(tokenString string) (*Claims, error) {
	token, err := jwt.ParseWithClaims(tokenString, &Claims{}, func(t *jwt.Token) (interface{}, error) {
		return j.secret, nil
	})
	if err != nil {
		return nil, err
	}
	claims, ok := token.Claims.(*Claims)
	if !ok || !token.Valid || claims.UserID <= 0 {
		return nil, jwt.ErrTokenInvalidClaims
	}
	return claims, nil
}

func (j *JWTAuth) Required() gin.HandlerFunc {
	return j.handle(true)
}

func (j *JWTAuth) Optional() gin.HandlerFunc {
	return j.handle(false)
}

func (j *JWTAuth) handle(required bool) gin.HandlerFunc {
	return func(c *gin.Context) {
		raw := extractBearer(c)
		if raw == "" {
			if required {
				utils.Fail(c, utils.ErrCodeJWTInvalid, "请先登录")
				c.Abort()
				return
			}
			c.Next()
			return
		}

		claims, err := j.Parse(raw)
		if err != nil {
			utils.Fail(c, utils.ErrCodeJWTInvalid, "登录已过期，请重新登录")
			c.Abort()
			return
		}
		c.Set(UserIDKey, claims.UserID)
		c.Next()
	}
}

func GetUserID(c *gin.Context) (int64, bool) {
	v, ok := c.Get(UserIDKey)
	if !ok {
		return 0, false
	}
	id, ok := v.(int64)
	return id, ok && id > 0
}

func extractBearer(c *gin.Context) string {
	h := c.GetHeader("Authorization")
	if h == "" {
		return ""
	}
	const prefix = "Bearer "
	if strings.HasPrefix(h, prefix) {
		return strings.TrimSpace(h[len(prefix):])
	}
	return strings.TrimSpace(h)
}
