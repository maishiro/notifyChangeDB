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
	PathDB           string `toml:"path"`

	Items []configItem `toml:"item"`
}

type configItem struct {
	ID                   string            `toml:"id"`
	SqlTemplate          string            `toml:"sql_template"`
	IndicatorColumnName  string            `toml:"indicator_column_name"`
	IndicatorColumnValue string            `toml:"indicator_column_value"`
	Tags                 []string          `toml:"tag_columns"`
	ExcludeColumns       []string          `toml:"exclude_columns"`
	ColumnTypes          map[string]string `toml:"column_types"`
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
