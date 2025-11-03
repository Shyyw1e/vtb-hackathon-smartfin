package config

import (
	"encoding/hex"
	"errors"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/go-playground/validator/v10"
	"github.com/joho/godotenv"
	"github.com/spf13/viper"
)


type AppConfig struct {
	Name     string         `mapstructure:"name"`
	Env      string         `mapstructure:"env"`
	HTTP     HTTPConfig     `mapstructure:"http"`
	Features FeaturesConfig `mapstructure:"features"`
}

type HTTPConfig struct {
	Addr         string        `mapstructure:"addr"`
	ReadTimeout  time.Duration `mapstructure:"-"`
	WriteTimeout time.Duration `mapstructure:"-"`
	ReadTimeoutStr  string `mapstructure:"read_timeout"`
	WriteTimeoutStr string `mapstructure:"write_timeout"`
}

type FeaturesConfig struct {
	Paywall     bool `mapstructure:"paywall"`
	GostEnabled bool `mapstructure:"gost_enabled"`
	VrpEnabled  bool `mapstructure:"vrp_enabled"`
}

type PostgresConfig struct {
	DSN         string `mapstructure:"dsn" validate:"required"`
	MaxOpenConn int    `mapstructure:"max_open_conns"`
	MaxIdleConn int    `mapstructure:"max_idle_conns"`
}

type BankConfig struct {
	BaseURL      string `mapstructure:"base_url" validate:"required"`
	UseGost      bool   `mapstructure:"use_gost"`
	ClientID     string `mapstructure:"-"`
	ClientSecret string `mapstructure:"-"` 
}

type RedisConfig struct {
	Addr     string `mapstructure:"addr"`
	Password string `mapstructure:"-"`
	DB       int    `mapstructure:"db"`
}

type NatsConfig struct {
	URL string `mapstructure:"url"`
}

type MLConfig struct {
	BaseURL    string        `mapstructure:"base_url"`
	TimeoutStr string        `mapstructure:"timeout"`
	Timeout    time.Duration `mapstructure:"-"`
	Gzip       bool          `mapstructure:"gzip"`
	HmacEnabled bool         `mapstructure:"hmac_enabled"`
}

type SecurityConfig struct {
	JWTPublicKeyPEM string `mapstructure:"jwt_public_key_pem"`
	MLSharedSecret  string `mapstructure:"-"`
	EncryptKeyHex   string `mapstructure:"-"`
}

type LimitsConfig struct {
	TxBatchMax     int `mapstructure:"tx_batch_max"`
	MLTimeoutSec   int `mapstructure:"ml_timeout_sec"`
	BankTimeoutSec int `mapstructure:"bank_timeout_sec"`
}

type Config struct {
	App      AppConfig             `mapstructure:"app"`
	Postgres PostgresConfig        `mapstructure:"postgres"`
	Redis    RedisConfig           `mapstructure:"redis"`
	Nats     NatsConfig            `mapstructure:"nats"`
	ML       MLConfig              `mapstructure:"ml"`
	Banks    map[string]BankConfig `mapstructure:"banks"`
	Security SecurityConfig        `mapstructure:"security"`
	Limits   LimitsConfig          `mapstructure:"limits"`
}


func Load(filePath string) (*Config, error) {
	_ = godotenv.Load()

	v := viper.New()
	v.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))
	v.AutomaticEnv()

	v.SetDefault("app.name", "smartfin-api")
	v.SetDefault("app.env", "dev")
	v.SetDefault("app.http.addr", ":8080")
	v.SetDefault("postgres.max_open_conns", 20)
	v.SetDefault("postgres.max_idle_conns", 10)
	v.SetDefault("redis.addr", "redis:6379")
	v.SetDefault("ml.timeout", "8s")
	v.SetDefault("limits.tx_batch_max", 5000)
	v.SetDefault("limits.ml_timeout_sec", 8)
	v.SetDefault("limits.bank_timeout_sec", 4)

	if filePath != "" {
		v.SetConfigFile(filePath)
		if err := v.ReadInConfig(); err != nil {
			return nil, fmt.Errorf("read config file: %w", err)
		}
	}

	var cfg Config
	if err := v.Unmarshal(&cfg); err != nil {
		return nil, fmt.Errorf("unmarshal config: %w", err)
	}

	if err := parseDurations(v, &cfg); err != nil {
		return nil, err
	}

	fillSecretsFromEnv(&cfg)

	if err := validateConfig(&cfg); err != nil {
		return nil, err
	}

	return &cfg, nil
}

func MustLoad(filePath string) *Config {
	cfg, err := Load(filePath)
	if err != nil {
		panic(fmt.Errorf("config load failed: %w", err))
	}
	return cfg
}

func (c *Config) Redacted() *Config {
	cc := *c
	cc.Security.MLSharedSecret = "REDACTED"
	cc.Security.EncryptKeyHex = "REDACTED"
	cc.Redis.Password = "REDACTED"
	for k, b := range cc.Banks {
		b.ClientID = "REDACTED"
		b.ClientSecret = "REDACTED"
		cc.Banks[k] = b
	}
	return &cc
}


func parseDurations(v *viper.Viper, cfg *Config) error {
	readStr := cfg.App.HTTP.ReadTimeoutStr
	if readStr == "" {
		readStr = v.GetString("app.http.read_timeout")
	}
	if readStr != "" {
		d, err := time.ParseDuration(readStr)
		if err != nil {
			return fmt.Errorf("invalid app.http.read_timeout: %w", err)
		}
		cfg.App.HTTP.ReadTimeout = d
	}

	writeStr := cfg.App.HTTP.WriteTimeoutStr
	if writeStr == "" {
		writeStr = v.GetString("app.http.write_timeout")
	}
	if writeStr != "" {
		d, err := time.ParseDuration(writeStr)
		if err != nil {
			return fmt.Errorf("invalid app.http.write_timeout: %w", err)
		}
		cfg.App.HTTP.WriteTimeout = d
	}

	mlt := cfg.ML.TimeoutStr
	if mlt == "" {
		mlt = v.GetString("ml.timeout")
	}
	if mlt != "" {
		d, err := time.ParseDuration(mlt)
		if err != nil {
			return fmt.Errorf("invalid ml.timeout: %w", err)
		}
		cfg.ML.Timeout = d
	}

	return nil
}

func fillSecretsFromEnv(cfg *Config) {
	if v, ok := os.LookupEnv("SECURITY_ML_SHARED_SECRET"); ok {
		cfg.Security.MLSharedSecret = v
	}
	if v, ok := os.LookupEnv("SECURITY_ENCRYPT_KEY_HEX"); ok {
		cfg.Security.EncryptKeyHex = v
	}
	if v, ok := os.LookupEnv("REDIS_PASSWORD"); ok {
		cfg.Redis.Password = v
	}
	if v, ok := os.LookupEnv("POSTGRES_DSN"); ok {
		cfg.Postgres.DSN = v
	}
	if v, ok := os.LookupEnv("NATS_URL"); ok {
		cfg.Nats.URL = v
	}

	for k := range cfg.Banks {
		up := strings.ToUpper(k)
		idKey := fmt.Sprintf("BANKS_%s_CLIENT_ID", strings.ReplaceAll(up, "-", "_"))
		secKey := fmt.Sprintf("BANKS_%s_CLIENT_SECRET", strings.ReplaceAll(up, "-", "_"))
		if v, ok := os.LookupEnv(idKey); ok {
			b := cfg.Banks[k]
			b.ClientID = v
			cfg.Banks[k] = b
		}
		if v, ok := os.LookupEnv(secKey); ok {
			b := cfg.Banks[k]
			b.ClientSecret = v
			cfg.Banks[k] = b
		}
	}
}

func validateConfig(cfg *Config) error {
	validate := validator.New()

	if err := validate.StructPartial(cfg.Postgres, "DSN"); err != nil {
		return fmt.Errorf("postgres config invalid: %w", err)
	}

	if cfg.App.HTTP.Addr == "" {
		return errors.New("app.http.addr is required")
	}

	if cfg.Postgres.DSN == "" {
		return errors.New("POSTGRES_DSN or postgres.dsn in config required")
	}
	if !strings.HasPrefix(cfg.Postgres.DSN, "postgres://") && !strings.HasPrefix(cfg.Postgres.DSN, "postgresql://") {
		return errors.New("postgres.dsn must start with postgres:// or postgresql://")
	}

	if cfg.ML.BaseURL == "" {
		// ML service might be optional in dev; warn by not failing. If ML is mandatory for your flow, return error instead.
		return errors.New("ml.base_url is required")
	}

	if cfg.Security.EncryptKeyHex != "" {
		if err := validateEncryptKey(cfg.Security.EncryptKeyHex); err != nil {
			return err
		}
	}

	if cfg.Security.MLSharedSecret != "" && len(cfg.Security.MLSharedSecret) < 8 {
		return errors.New("SECURITY_ML_SHARED_SECRET must be at least 8 characters")
	}

	if cfg.Limits.TxBatchMax <= 0 {
		return errors.New("limits.tx_batch_max must be > 0")
	}
	if cfg.Limits.TxBatchMax > 100000 {
		return errors.New("limits.tx_batch_max too large")
	}

	for name, b := range cfg.Banks {
		if strings.TrimSpace(b.BaseURL) == "" {
			return fmt.Errorf("banks.%s.base_url is required", name)
		}
		// if client id/secret are required for this bank in your setup, validate:
		if b.ClientID == "" || b.ClientSecret == "" { return fmt.Errorf("BANKS_%s_CLIENT_ID/SECRET required", strings.ToUpper(name)) }
	}

	return nil
}

func validateEncryptKey(s string) error {
	if len(s) != 64 {
		return fmt.Errorf("encrypt_key_hex must be 64 hex chars (32 bytes), got len=%d", len(s))
	}
	if _, err := hex.DecodeString(s); err != nil {
		return fmt.Errorf("encrypt_key_hex invalid hex: %w", err)
	}
	return nil
}
