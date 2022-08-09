package main

import (
	"github.com/gin-gonic/gin"
	"github.com/joho/godotenv"
	"go_gw/http"
	"log"
	"os"
)

func init() {
	err := godotenv.Load()
	if err != nil {
		log.Fatalln("no env gotten", err)
	}
}

func main() {
	router := gin.Default()
	router.Any("/*proxyPath", http.HandleProxy)
	log.Fatal(router.Run(":" + os.Getenv("API_PORT")))
}
