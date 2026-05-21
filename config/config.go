package config

import (
	"errors"
	"fmt"
	"os"
	"strings"

	"github.com/Maxxxxxx-x/rotector-slop/utils"
	"github.com/joho/godotenv"
)

type Config struct {
	RotectorKey string
	Groups      map[string]string
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

	config.Groups, err = splitGroup(os.Getenv("GROUPS"))
	if err != nil {
		return Config{}, err
	}

	return config, nil
}
