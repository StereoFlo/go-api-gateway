package main

import (
	"github.com/gin-gonic/gin"
	"github.com/joho/godotenv"
	"go_gw/http"
	"log"
	"os"
)

func init() {
	if err := godotenv.Load(); err != nil {
		log.Panicln("no env gotten")
	}
}

func main() {
	router := gin.Default()
	router.Any("/*proxyPath", http.HandleProxy)
	log.Fatal(router.Run(":" + os.Getenv("API_PORT")))
}
