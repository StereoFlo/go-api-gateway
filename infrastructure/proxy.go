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

// NewProxy Constructor
func NewProxy(context *gin.Context) *Proxy {
	return &Proxy{context}
}

// ReverseProxy make request to service
func (p Proxy) ReverseProxy(ch chan error) {
	serviceConfig, err := p.loadServiceConfig()
	if err != nil {
		ch <- err
		close(ch)
		return
	}
	httpMethod, _, err := p.loadServiceMethod(serviceConfig)
	if err != nil {
		ch <- err
		close(ch)
		return
	}

	targetUrl := p.buildUrl(serviceConfig.Servers[0].Url, p.context.Request.URL.Path) //todo Servers[0] ??
	parsedUrl, err := url.Parse(targetUrl)
	if err != nil {
		ch <- err
		close(ch)
		return
	}

	reverseProxy := httputil.NewSingleHostReverseProxy(parsedUrl)
	reverseProxy.Director = func(request *http.Request) {
		p.setDirector(request, httpMethod, parsedUrl)
		p.setQuery(request)
		p.setBody(request)
	}
	reverseProxy.ServeHTTP(p.context.Writer, p.context.Request)
	close(ch)
}

// prepare request to service
func (p Proxy) setDirector(request *http.Request, httpMethod *string, parsedUrl *url.URL) {
	request.Method = *httpMethod
	request.Host = parsedUrl.Host
	request.URL.Scheme = parsedUrl.Scheme
	request.URL.Host = parsedUrl.Host
	request.URL.Path = parsedUrl.Path
	request.Header.Set("x-account-token", p.context.Request.Header.Get("x-account-token"))
}

// prepare query to service
func (p Proxy) setQuery(request *http.Request) {
	if len(p.context.Request.URL.RawQuery) <= 0 {
		return
	}
	request.URL.RawQuery = p.context.Request.URL.RawQuery
}

// prepare body to send to service
func (p Proxy) setBody(request *http.Request) {
	method := p.context.Request.Method
	if method != http.MethodPost && method != http.MethodPut && method != http.MethodPatch {
		return
	}
	bodyAsBytes, _ := io.ReadAll(p.context.Request.Body)
	if len(string(bodyAsBytes)) <= 0 {
		return
	}
	body := io.NopCloser(bytes.NewReader(bodyAsBytes))
	if body == nil {
		return
	}
	request.Body = body
}

func (p Proxy) buildUrl(serviceUrl string, path string) string {
	parts := strings.Split(strings.TrimPrefix(path, "/"), "/")
	targetUrl := serviceUrl + "/" + strings.Join(parts[1:], "/")
	return targetUrl
}

// load service config
func (p Proxy) loadServiceConfig() (*ServiceConfig, error) {
	parts := strings.Split(strings.TrimPrefix(p.context.Request.URL.Path, "/"), "/")
	if len(parts) <= 1 {
		return nil, errors.New(fmt.Sprintf("failed to parse target host from path: %p", p.context.Request.URL.Path))
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

// load service method && checks token if it need
func (p Proxy) loadServiceMethod(c *ServiceConfig) (*string, *Method, error) {
	for configPath, data := range c.Paths {
		regex := regexp.MustCompile(`:[a-z]+`)
		regexPath := regex.ReplaceAllString(configPath, `.*`)
		ok, err := regexp.MatchString(regexPath, p.context.Request.URL.Path)
		if err != nil {
			return nil, nil, err
		}
		if ok {
			for method, methodData := range data {
				if len(methodData.Parameters) > 0 {
					for _, parameter := range methodData.Parameters {
						if parameter.In == "header" && parameter.Required == true {
							token := p.context.Request.Header.Get("X-ACCOUNT-TOKEN")
							err := p.checkToken(token)
							if err != nil {
								return nil, nil, err
							}
							upMethod := strings.ToUpper(method)
							if upMethod == p.context.Request.Method {
								return &upMethod, &methodData, nil
							}
						}
					}
				} else {
					upMethod := strings.ToUpper(method)
					if upMethod == p.context.Request.Method {
						return &upMethod, &methodData, nil
					}
				}
			}
		}
	}
	return nil, nil, errors.New("not found")
}

// checks jwt token
func (p Proxy) checkToken(token string) error {
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
