package infrastructure

import (
	"bytes"
	"errors"
	"fmt"
	"github.com/gin-gonic/gin"
	"github.com/gokyle/filecache"
	"go_gw/infrastructure/jwt-token"
	"gopkg.in/yaml.v3"
	"io"
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
	Summary    string      `yaml:"summary"`
	Parameters []Parameter `yaml:"parameters"`
}

type Parameter struct {
	Name     string `yaml:"name"`
	In       string `yaml:"in"`
	Required bool   `yaml:"required"`
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

func (s Proxy) ReverseProxy(ch chan error) {
	serviceConfig, err := s.loadServiceConfig()
	if err != nil {
		ch <- err
		close(ch)
		return
	}
	httpMethod, _, err := s.loadServiceMethod(serviceConfig)
	if err != nil {
		ch <- err
		close(ch)
		return
	}

	targetUrl := s.buildUrl(serviceConfig.Servers[0].Url, s.context.Request.URL.Path) //todo Servers[0] ??
	parsedUrl, err := url.Parse(targetUrl)
	if err != nil {
		ch <- err
		close(ch)
		return
	}

	reverseProxy := httputil.NewSingleHostReverseProxy(parsedUrl)
	reverseProxy.Director = func(request *http.Request) {
		s.setDirector(request, httpMethod, parsedUrl)
		s.setQuery(request)
		s.setBody(request)
	}
	reverseProxy.ServeHTTP(s.context.Writer, s.context.Request)
	close(ch)
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
	bodyAsBytes, _ := io.ReadAll(s.context.Request.Body)
	if len(string(bodyAsBytes)) <= 0 {
		return
	}
	body := io.NopCloser(bytes.NewReader(bodyAsBytes))
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
	err := cache.Start()
	if err != nil {
		return nil, err
	}
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
		if ok {
			for method, methodData := range data {
				if len(methodData.Parameters) > 0 {
					for _, parameter := range methodData.Parameters {
						if parameter.In == "header" && parameter.Required == true {
							err := s.checkToken()
							if err != nil {
								return nil, nil, err
							}
							upMethod := strings.ToUpper(method)
							if upMethod == s.context.Request.Method {
								return &upMethod, &methodData, nil
							}
						}
					}
				} else {
					upMethod := strings.ToUpper(method)
					if upMethod == s.context.Request.Method {
						return &upMethod, &methodData, nil
					}
				}
			}
		}
	}
	return nil, nil, errors.New("not found")
}

func (s Proxy) checkToken() error {
	token := s.context.Request.Header.Get("X-ACCOUNT-TOKEN")
	if token == "" {
		return errors.New("token is empty")
	}
	jwtToken := jwt_token.NewToken()
	_, err := jwtToken.Validate(token)
	if err != nil {
		return errors.New("token is wrong")
	}
	return nil
}
