package config

import (
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/joho/godotenv"
)

type Config struct {
	Port string

	DatabaseURL string
	RedisURL    string

	DeepSeekAPIKey  string
	DeepSeekBaseURL string

	WeChatAppID      string
	WeChatAppSecret  string
	WeChatQRPage     string
	WeChatEnvVersion string

	JWTSecret string
	JWTExpiry time.Duration

	R2AccountID   string
	R2AccessKey   string
	R2SecretKey   string
	R2BucketName  string
	R2PublicURL   string
	R2KeyPrefix   string
	PublicBaseURL string

	FontPath string
	Timezone *time.Location

	// InterpretDailyLimit is the max successful POST /interpret calls per user per day.
	// 0 means unlimited (for develop/trial builds).
	InterpretDailyLimit int
}

func Load() (*Config, error) {
	_ = godotenv.Load()

	expiryHours, err := strconv.Atoi(getEnv("JWT_EXPIRY_HOURS", "720"))
	if err != nil || expiryHours <= 0 {
		return nil, fmt.Errorf("invalid JWT_EXPIRY_HOURS")
	}

	tzName := getEnv("TZ", "Asia/Shanghai")
	loc, err := time.LoadLocation(tzName)
	if err != nil {
		loc = time.FixedZone("CST", 8*3600)
	}

	cfg := &Config{
		Port:                getEnv("PORT", "8080"),
		DatabaseURL:         getEnv("DATABASE_URL", ""),
		RedisURL:            getEnv("REDIS_URL", ""),
		DeepSeekAPIKey:      getEnv("DEEPSEEK_API_KEY", ""),
		DeepSeekBaseURL:     getEnv("DEEPSEEK_BASE_URL", "https://api.deepseek.com"),
		WeChatAppID:         getEnv("WECHAT_APP_ID", "wx40efcaf0ff532927"),
		WeChatAppSecret:     getEnv("WECHAT_APP_SECRET", ""),
		WeChatQRPage:        getEnv("WECHAT_QR_PAGE", ""),
		WeChatEnvVersion:    getEnv("WECHAT_ENV_VERSION", "release"),
		JWTSecret:           getEnv("JWT_SECRET", ""),
		JWTExpiry:           time.Duration(expiryHours) * time.Hour,
		R2AccountID:         getEnv("R2_ACCOUNT_ID", ""),
		R2AccessKey:         getEnv("R2_ACCESS_KEY_ID", ""),
		R2SecretKey:         getEnv("R2_SECRET_KEY", ""),
		R2BucketName:        getEnv("R2_BUCKET_NAME", ""),
		R2PublicURL:         strings.TrimRight(getEnv("R2_PUBLIC_URL", "https://r2.soupcircle.xyz"), "/"),
		R2KeyPrefix:         strings.Trim(getEnv("R2_KEY_PREFIX", "bookmark"), "/"),
		PublicBaseURL:       strings.TrimRight(getEnv("PUBLIC_BASE_URL", "https://api.soupcircle.xyz/bookmark"), "/"),
		FontPath:            getEnv("FONT_PATH", "./assets/fonts/SourceHanSansCN-Regular.otf"),
		Timezone:            loc,
		InterpretDailyLimit: getEnvInt("INTERPRET_DAILY_LIMIT", 3),
	}

	if cfg.JWTSecret == "" {
		return nil, fmt.Errorf("JWT_SECRET is required")
	}
	if cfg.DatabaseURL == "" {
		return nil, fmt.Errorf("DATABASE_URL is required")
	}

	return cfg, nil
}

func getEnv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

func getEnvInt(key string, fallback int) int {
	raw := os.Getenv(key)
	if raw == "" {
		return fallback
	}
	n, err := strconv.Atoi(raw)
	if err != nil {
		return fallback
	}
	if n < 0 {
		return 0
	}
	return n
}
