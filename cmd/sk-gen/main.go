package main

import (
	chatcrypto "GoChat/internal/crypto"
	"encoding/base64"
	"flag"
	"fmt"
	"os"
)

func main() {
	asEnv := flag.Bool("env", false, "以环境变量形式输出 export 语句")

	flag.Parse()

	key, err := chatcrypto.GenerateRandomKey(32)
	if err != nil {
		fmt.Fprintln(os.Stderr, "错误:", err)
		os.Exit(1)
	}
	printKey("GOCHAT_PSK", key, *asEnv)
}

func printKey(varName string, key []byte, asEnv bool) {
	printKV(varName, key, asEnv)
}

func printKV(varName string, value []byte, asEnv bool) {
	s := base64.StdEncoding.EncodeToString(value)
	if asEnv {
		fmt.Printf("export %s=%s\n", varName, s)
	} else {
		fmt.Println(s)
	}
}
