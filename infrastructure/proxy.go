package infrastructure

import (
	"bytes"
	"errors"
	"fmt"
	"github.com/gin-gonic/gin"
	"github.com/sirupsen/logrus"
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
	Paths   map[string]Patch `yaml:"paths"`
	Servers []Server         `yaml:"servers"`
}

type Patch map[string]Method

type Method struct {
	Summary string `yaml:"summary"`
}

type Server struct {
	Url string `yaml:"url"`
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
			if ok && upMethod == httpMethod {
				targetUrl, _ := buildUrl(c.Servers[0].Url, path)
				return targetUrl, nil
			}
		}
	}
	return "", errors.New("something is wrong")
}

func buildUrl(serviceUrl string, path string) (string, error) {
	parts := strings.Split(strings.TrimPrefix(path, "/"), "/")
	targetUrl := serviceUrl + "/" + strings.Join(parts[1:], "/")
	return targetUrl, nil
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
		bodyAsByteArray, _ := ioutil.ReadAll(ctx.Request.Body)
		if len(string(bodyAsByteArray)) > 0 {
			request.Body = ctx.Request.Body
		}
		request.Host = parsedUrl.Host
		request.URL.Scheme = parsedUrl.Scheme
		request.URL.Host = parsedUrl.Host
		request.URL.Path = parsedUrl.Path
		request.Header.Set("x-account-token", ctx.Request.Header.Get("x-account-token"))
	}
	reverseProxy.ModifyResponse = func(response *http.Response) error {
		if response.StatusCode == http.StatusInternalServerError {
			s := readBody(response)
			logrus.Errorf("req %s ,with error %d, body:%s", parsedUrl, response.StatusCode, s)
			response.Body = ioutil.NopCloser(bytes.NewReader([]byte(fmt.Sprintf("error"))))
		} else if response.StatusCode > 300 {
			s := readBody(response)
			logrus.Errorf("req %s ,with error %d, body:%s", parsedUrl, response.StatusCode, s)
			response.Body = ioutil.NopCloser(bytes.NewReader([]byte(s)))
		}
		return nil
	}
	return reverseProxy, nil
}

func readBody(response *http.Response) string {
	defer response.Body.Close()
	all, _ := ioutil.ReadAll(response.Body)
	var bodyString string
	if len(all) > 0 {
		bodyString = string(all)
	}
	return bodyString
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
