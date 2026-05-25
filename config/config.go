package config

import (
	"errors"
	"fmt"
	"os"
	"strconv"
	"strings"

	"github.com/Maxxxxxx-x/rotector-slop/utils"
	"github.com/joho/godotenv"
)

type Database struct {
	Name        string
	MaxOpenConn int
	MaxIdleConn int
}

type Config struct {
	RotectorKey  string
	Groups       map[string]string
	RobloxCookie string
	Database     Database
}

func splitGroup(groupStr string) (map[string]string, error) {
	if groupStr == "" {
		return nil, errors.New("GROUPS not set!")
	}
	list := strings.Split(groupStr, ",")
	if len(list)%2 != 0 {
		return nil, errors.New("Invalid GROUPS string. GROUP string example: id,name,id,name,id,name")
	}

	groups := make(map[string]string, len(list)/2)

	for idx, val := range list {
		if idx%2 != 0 {
			continue
		}
		val = strings.Trim(val, " ")
		if !utils.IsNumeric(val) {
			return groups, fmt.Errorf("Expected group id at index %d to be numeric. Got %s", idx, val)
		}

		groups[val] = strings.Trim(list[idx+1], " ")
	}

	return groups, nil
}

func GetFromEnv(path string) (Config, error) {
	err := godotenv.Load(path)
	if err != nil {
		return Config{}, errors.New("Failed to load .env file")
	}

	var config Config

	config.RotectorKey = os.Getenv("ROTECTOR_API_KEY")
	config.RobloxCookie = os.Getenv("ROBLOX_COOKIE")

	maxIdleConnStr := os.Getenv("MAX_IDLE_CONN")
	maxOpenConnStr := os.Getenv("MAX_OPEN_CONN")

	maxIdleConn := 0
	maxOpenConn := 0

	if maxIdleConnStr != "" {
		if maxIdleConn, err = strconv.Atoi(maxIdleConnStr); err != nil {
			maxIdleConn = 0
		}
	}

	if maxOpenConnStr != "" {
		if maxOpenConn, err = strconv.Atoi(maxOpenConnStr); err != nil {
			maxOpenConn = 0
		}
	}

	config.Database = Database{
		Name:        os.Getenv("DB_NAME"),
		MaxIdleConn: maxIdleConn,
		MaxOpenConn: maxOpenConn,
	}

	config.Groups, err = splitGroup(os.Getenv("GROUPS"))
	if err != nil {
		return Config{}, err
	}

	return config, nil
}
