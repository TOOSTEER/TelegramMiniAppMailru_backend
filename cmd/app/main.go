package main

import (
	"log"

	"github.com/TOOSTEER/TelegramMiniAppMailru_backend/internal/httpapi"
)

func main() {
	r := httpapi.NewRouter()

	if err := r.Run(":8080"); err != nil {
		log.Fatal(err)
	}
}
