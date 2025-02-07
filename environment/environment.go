package environment

import (
	"fmt"
	"os"
	"strconv"
	"strings"

	// autoload env vars
	_ "github.com/joho/godotenv/autoload"
)

func GetInt64(varName string, defaultValue int64) int64 {
	val, ok := os.LookupEnv(varName)
	if !ok {
		return defaultValue
	}

	iVal, err := strconv.ParseInt(val, 10, 64)
	if err != nil {
		return defaultValue
	}

	return iVal
}

func MustGetString(varName string) string {
	val, _ := os.LookupEnv(varName)
	if val == "" {
		panic(fmt.Sprintf("environment error (string): required env var %s not found", varName))
	}

	return val
}

func GetString(varName string, defaultValue string) string {
	val, _ := os.LookupEnv(varName)
	if val == "" {
		return defaultValue
	}

	return val
}

func MustGetStringSlice(varName, separator string) []string {
	rawString := MustGetString(varName)
	return strings.Split(rawString, separator)
}
