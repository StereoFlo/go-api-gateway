package infrastructure

import (
	"bytes"
	"errors"
	"fmt"
	"github.com/gin-gonic/gin"
	"github.com/gokyle/filecache"
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

type ProxyStr struct {
	context *gin.Context
}

func NewProxy(context *gin.Context) *ProxyStr {
	return &ProxyStr{context}
}

func (s ProxyStr) ReverseProxy() (*httputil.ReverseProxy, error) {
	serviceConfig, err := s.loadServiceConfig()
	if err != nil {
		return nil, err
	}
	urlTxt, err := s.loadServiceMethod(serviceConfig)
	if err != nil {
		return nil, err
	}
	parsedUrl, err := url.Parse(urlTxt)
	if err != nil {
		return nil, err
	}

	reverseProxy := httputil.NewSingleHostReverseProxy(parsedUrl)
	reverseProxy.Director = func(request *http.Request) {
		s.setQuery(request)
		s.setBody(request)
		request.Host = parsedUrl.Host
		request.URL.Scheme = parsedUrl.Scheme
		request.URL.Host = parsedUrl.Host
		request.URL.Path = parsedUrl.Path
		request.Header.Set("x-account-token", s.context.Request.Header.Get("x-account-token"))
	}
	return reverseProxy, nil
}

func (s ProxyStr) setQuery(request *http.Request) {
	if len(s.context.Request.URL.RawQuery) <= 0 {
		return
	}
	request.URL.RawQuery = s.context.Request.URL.RawQuery
}

func (s ProxyStr) setBody(request *http.Request) {
	method := s.context.Request.Method
	if method != "POST" && method != "PUT" && method != "PATCH" {
		return
	}
	bodyAsBytes, _ := ioutil.ReadAll(s.context.Request.Body)
	if len(string(bodyAsBytes)) <= 0 {
		return
	}
	body := ioutil.NopCloser(bytes.NewReader(bodyAsBytes))
	if body == nil {
		return
	}
	request.Body = body
}

func (s ProxyStr) buildUrl(serviceUrl string, path string) (string, error) {
	parts := strings.Split(strings.TrimPrefix(path, "/"), "/")
	targetUrl := serviceUrl + "/" + strings.Join(parts[1:], "/")
	return targetUrl, nil
}

func (s ProxyStr) loadServiceConfig() (*ServiceConfig, error) {
	parts := strings.Split(strings.TrimPrefix(s.context.Request.URL.Path, "/"), "/")
	if len(parts) <= 1 {
		return nil, errors.New(fmt.Sprintf("failed to parse target host from path: %s", s.context.Request.URL.Path))
	}
	serviceName := fmt.Sprintf("%s", parts[0])
	cache := filecache.NewDefaultCache()
	cache.Start()
	yamlFile, err := cache.ReadFile("services-config/" + serviceName + ".yaml")
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

func (s ProxyStr) loadServiceMethod(c *ServiceConfig) (string, error) {
	for configPath, data := range c.Paths {
		m := regexp.MustCompile(`:[a-z]+`)
		regexPath := m.ReplaceAllString(configPath, `.*`)
		ok, merr := regexp.MatchString(regexPath, s.context.Request.URL.Path)
		if merr != nil {
			return "", merr
		}
		for method := range data {
			upMethod := strings.ToUpper(method)
			if ok && upMethod == s.context.Request.Method {
				targetUrl, _ := s.buildUrl(c.Servers[0].Url, s.context.Request.URL.Path) //todo Servers[0] ??
				return targetUrl, nil
			}
		}
	}
	return "", errors.New("something is wrong")
}
