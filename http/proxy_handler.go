package http

import (
	"github.com/gin-gonic/gin"
	"go_gw/infrastructure"
	"log"
	"net/http"
)

func HandleProxy(ctx *gin.Context) {
	proxy := infrastructure.NewProxy(ctx)
	err := proxy.ReverseProxy()
	if err != nil {
		log.Println(err)
		ctx.AbortWithStatus(http.StatusServiceUnavailable)
		return
	}
	return
}
