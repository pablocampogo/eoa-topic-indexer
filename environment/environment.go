package environment

import (
	"fmt"
	"os"
	"strconv"
	"strings"

	// autoload env vars
	_ "github.com/joho/godotenv/autoload"
)

// GetInt64 retrieves an environment variable as an int64. If the variable is not found, it returns the provided default value.
// If the environment variable exists but cannot be parsed as an int64, it also returns the default value.
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

// MustGetString retrieves a string environment variable. If the variable is not found or is empty, it panics with an error message.
func MustGetString(varName string) string {
	val, _ := os.LookupEnv(varName)
	if val == "" {
		panic(fmt.Sprintf("environment error (string): required env var %s not found", varName))
	}

	return val
}

// GetString retrieves an environment variable as a string. If the variable is not found, it returns the provided default value.
func GetString(varName string, defaultValue string) string {
	val, _ := os.LookupEnv(varName)
	if val == "" {
		return defaultValue
	}

	return val
}

// MustGetStringSlice retrieves a string environment variable and splits it by the given separator. If the variable is not found, it panics.
// It returns the resulting slice of strings.
func MustGetStringSlice(varName, separator string) []string {
	rawString := MustGetString(varName)
	return strings.Split(rawString, separator)
}
