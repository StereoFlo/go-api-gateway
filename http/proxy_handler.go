package http

import (
	"errors"
	"github.com/gin-gonic/gin"
	"go_gw/infrastructure"
	"go_gw/infrastructure/jwt-token"
	"log"
	"net/http"
)

func HandleProxy(ctx *gin.Context) {
	responder := infrastructure.NewResponder()
	err := checkToken(ctx)
	if err != nil {
		ctx.AbortWithStatusJSON(http.StatusUnauthorized, responder.Fail(err))
		return
	}

	proxy(ctx, responder)
}

func proxy(ctx *gin.Context, responder *infrastructure.Responder) {
	proxy := infrastructure.NewProxy(ctx)
	errCh := make(chan error)
	go proxy.ReverseProxy(errCh)
	res := <-errCh
	if res != nil {
		log.Println(res)
		ctx.AbortWithStatusJSON(http.StatusNotFound, responder.Fail(res.Error()))
		return
	}
	return
}

func checkToken(ctx *gin.Context) error {
	token := ctx.Request.Header.Get("X-ACCOUNT-TOKEN")
	if token == "" {
		return errors.New("unauthorized")
	}
	jwt := jwt_token.NewToken()
	_, err := jwt.Validate(token)
	if err != nil {
		return errors.New(err.Error())
	}
	return nil
}
