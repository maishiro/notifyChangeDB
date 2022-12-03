package config

import (
	"bytes"
	"os"
	"strings"

	"github.com/BurntSushi/toml"
)

var (
	envVarEscaper = strings.NewReplacer(
		`"`, `\"`,
		`\`, `\\`,
	)
)

type Config struct {
	Cfg config `toml:"config"`
}

type config struct {
	Driver           string `toml:"driver"`
	ConnectionString string `toml:"connection_string"`

	Items []configItem `toml:"item"`
}

type configItem struct {
	SqlTemplate          string `toml:"sql_template"`
	IndicatorColunmName  string `toml:"indicator_colunm_name"`
	IndicatorColunmValue string `toml:"indicator_colunm_value"`
}

func NewConfig() *Config {
	c := &Config{}
	return c
}

func (c *Config) LoadConfig(path string) error {
	var err error
	b, err := os.ReadFile(path)
	if err != nil {
		return err
	}

	s := expandEnvVars(b)

	_, err = toml.Decode(s, c)
	if err != nil {
		return err
	}

	return nil
}

func trimBOM(f []byte) []byte {
	return bytes.TrimPrefix(f, []byte("\xef\xbb\xbf"))
}

func expandEnvVars(contents []byte) string {
	return os.Expand(string(contents), getEnv)
}

func getEnv(key string) string {
	v := os.Getenv(key)

	return envVarEscaper.Replace(v)
}
