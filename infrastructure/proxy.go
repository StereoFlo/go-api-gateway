package infrastructure

import (
	"bytes"
	"errors"
	"fmt"
	"github.com/gin-gonic/gin"
	"github.com/gokyle/filecache"
	"gopkg.in/yaml.v3"
	"io/ioutil"
	"net/http"
	"net/http/httputil"
	"net/url"
	"os"
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

type Proxy struct {
	context *gin.Context
}

func NewProxy(context *gin.Context) *Proxy {
	return &Proxy{context}
}

func (s Proxy) ReverseProxy() error {
	serviceConfig, err := s.loadServiceConfig()
	if err != nil {
		return err
	}
	httpMethod, _, err := s.loadServiceMethod(serviceConfig)
	if err != nil {
		return err
	}

	targetUrl := s.buildUrl(serviceConfig.Servers[0].Url, s.context.Request.URL.Path) //todo Servers[0] ??
	parsedUrl, err := url.Parse(targetUrl)
	if err != nil {
		return err
	}

	reverseProxy := httputil.NewSingleHostReverseProxy(parsedUrl)
	reverseProxy.Director = func(request *http.Request) {
		s.setDirector(request, httpMethod, parsedUrl)
		s.setQuery(request)
		s.setBody(request)
	}
	reverseProxy.ServeHTTP(s.context.Writer, s.context.Request)
	return nil
}

func (s Proxy) setDirector(request *http.Request, httpMethod *string, parsedUrl *url.URL) {
	request.Method = *httpMethod
	request.Host = parsedUrl.Host
	request.URL.Scheme = parsedUrl.Scheme
	request.URL.Host = parsedUrl.Host
	request.URL.Path = parsedUrl.Path
	request.Header.Set("x-account-token", s.context.Request.Header.Get("x-account-token"))
}

func (s Proxy) setQuery(request *http.Request) {
	if len(s.context.Request.URL.RawQuery) <= 0 {
		return
	}
	request.URL.RawQuery = s.context.Request.URL.RawQuery
}

func (s Proxy) setBody(request *http.Request) {
	method := s.context.Request.Method
	if method != http.MethodPost && method != http.MethodPut && method != http.MethodPatch {
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

func (s Proxy) buildUrl(serviceUrl string, path string) string {
	parts := strings.Split(strings.TrimPrefix(path, "/"), "/")
	targetUrl := serviceUrl + "/" + strings.Join(parts[1:], "/")
	return targetUrl
}

func (s Proxy) loadServiceConfig() (*ServiceConfig, error) {
	parts := strings.Split(strings.TrimPrefix(s.context.Request.URL.Path, "/"), "/")
	if len(parts) <= 1 {
		return nil, errors.New(fmt.Sprintf("failed to parse target host from path: %s", s.context.Request.URL.Path))
	}
	serviceName := parts[0]
	cache := filecache.NewDefaultCache()
	cache.Start()
	yamlFile, err := cache.ReadFile(os.Getenv("SERVICE_CONFIG") + "/" + serviceName + ".yaml")
	if err != nil {
		return nil, err
	}
	var sc *ServiceConfig
	err = yaml.Unmarshal(yamlFile, &sc)
	if err != nil {
		return nil, err
	}

	return sc, nil
}

func (s Proxy) loadServiceMethod(c *ServiceConfig) (*string, *Method, error) {
	for configPath, data := range c.Paths {
		regex := regexp.MustCompile(`:[a-z]+`)
		regexPath := regex.ReplaceAllString(configPath, `.*`)
		ok, err := regexp.MatchString(regexPath, s.context.Request.URL.Path)
		if err != nil {
			return nil, nil, err
		}
		for method, methodData := range data {
			upMethod := strings.ToUpper(method)
			if ok && upMethod == s.context.Request.Method {
				return &upMethod, &methodData, nil
			}
		}
	}
	return nil, nil, errors.New("something is wrong")
}
