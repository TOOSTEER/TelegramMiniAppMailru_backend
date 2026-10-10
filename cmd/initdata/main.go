package main

import (
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io/fs"
	"net/url"
	"os"
	"strconv"
	"time"

	"github.com/joho/godotenv"

	"github.com/TOOSTEER/TelegramMiniAppMailru_backend/internal/tgauth"
)

func main() {
	id := flag.Int64("id", 1, "telegram id пользователя")
	firstName := flag.String("first-name", "Dev", "имя пользователя")
	username := flag.String("username", "dev", "username пользователя")
	startParam := flag.String("start-param", "", "значение start_param")
	age := flag.Duration("age", 0, "насколько auth_date в прошлом, например 25h")
	flag.Parse()

	if err := godotenv.Load(); err != nil && !errors.Is(err, fs.ErrNotExist) {
		fail("чтение .env: %v", err)
	}
	token := os.Getenv("TG_BOT_TOKEN")
	if token == "" {
		fail("переменная окружения TG_BOT_TOKEN не задана")
	}

	user, err := json.Marshal(tgauth.User{ID: *id, FirstName: *firstName, Username: *username})
	if err != nil {
		fail("user: %v", err)
	}

	values := url.Values{
		"user":      {string(user)},
		"auth_date": {strconv.FormatInt(time.Now().Add(-*age).Unix(), 10)},
	}
	if *startParam != "" {
		values.Set("start_param", *startParam)
	}

	fmt.Println(tgauth.Sign(values, token))
}

func fail(format string, args ...any) {
	fmt.Fprintf(os.Stderr, format+"\n", args...)
	os.Exit(1)
}
