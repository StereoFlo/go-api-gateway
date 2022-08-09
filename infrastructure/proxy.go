package infrastructure

import (
	"bytes"
	"errors"
	"fmt"
	"github.com/gin-gonic/gin"
	"gopkg.in/yaml.v3"
	"io/ioutil"
	"log"
	"net/http"
	"net/http/httputil"
	"net/url"
	"regexp"
	"strings"
)

type ServiceConfig struct {
	Paths   map[string]Path `yaml:"paths"`
	Servers []Server        `yaml:"servers"`
}

type Path map[string]Method

type Method struct {
	Summary string `yaml:"summary"`
}

type Server struct {
	Url string `yaml:"url"`
}

func Proxy(ctx *gin.Context) (*httputil.ReverseProxy, error) {
	path := ctx.Request.URL.Path
	c, e := loadServiceConfig(path)
	if e != nil {
		ctx.AbortWithStatus(http.StatusNotFound)
		return nil, e
	}
	urlTxt, err := loadServiceMethod(ctx.Request.Method, c, path)
	if err != nil {
		ctx.AbortWithStatus(http.StatusNotFound)
		return nil, err
	}
	parsedUrl, err := url.Parse(urlTxt)
	if err != nil {
		ctx.AbortWithStatus(http.StatusNotFound)
		return nil, err
	}

	reverseProxy := httputil.NewSingleHostReverseProxy(parsedUrl)
	reverseProxy.Director = func(request *http.Request) {
		if len(ctx.Request.URL.RawQuery) > 0 {
			request.URL.RawQuery = ctx.Request.URL.RawQuery
		}
		if ctx.Request.Method == "POST" || ctx.Request.Method == "PUT" || ctx.Request.Method == "PATCH" {
			bodyAsBytes, _ := ioutil.ReadAll(ctx.Request.Body)
			if len(string(bodyAsBytes)) > 0 {
				body := ioutil.NopCloser(bytes.NewReader(bodyAsBytes))
				if body != nil {
					request.Body = body
				}
			}
		}
		request.Host = parsedUrl.Host
		request.URL.Scheme = parsedUrl.Scheme
		request.URL.Host = parsedUrl.Host
		request.URL.Path = parsedUrl.Path
		request.Header.Set("x-account-token", ctx.Request.Header.Get("x-account-token"))
	}
	return reverseProxy, nil
}

func buildUrl(serviceUrl string, path string) (string, error) {
	parts := strings.Split(strings.TrimPrefix(path, "/"), "/")
	targetUrl := serviceUrl + "/" + strings.Join(parts[1:], "/")
	return targetUrl, nil
}

func loadServiceConfig(path string) (*ServiceConfig, error) {
	parts := strings.Split(strings.TrimPrefix(path, "/"), "/")
	if len(parts) <= 1 {
		return nil, errors.New(fmt.Sprintf("failed to parse target host from path: %s", path))
	}
	serviceName := fmt.Sprintf("%s", parts[0])
	yamlFile, err := ioutil.ReadFile("services/" + serviceName + ".yaml")
	if err != nil {
		log.Printf(serviceName+".Get err   #%v ", err)
		return nil, err
	}
	var sc *ServiceConfig
	err = yaml.Unmarshal(yamlFile, &sc)
	if err != nil {
		log.Fatalf("error: %v", err)
		return nil, err
	}

	return sc, nil
}

func loadServiceMethod(httpMethod string, c *ServiceConfig, path string) (string, error) {
	for configPath, data := range c.Paths {
		m := regexp.MustCompile(`:[a-z]+`)
		regexPath := m.ReplaceAllString(configPath, `.*`)
		ok, merr := regexp.MatchString(regexPath, path)
		if merr != nil {
			return "", merr
		}
		for method := range data {
			upMethod := strings.ToUpper(method)
			fmt.Println(ok, upMethod, httpMethod)
			if ok && upMethod == httpMethod {
				targetUrl, _ := buildUrl(c.Servers[0].Url, path) //todo Servers[0] ??
				return targetUrl, nil
			}
		}
	}
	return "", errors.New("something is wrong")
}
