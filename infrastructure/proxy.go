package infrastructure

import (
	"bytes"
	"errors"
	"fmt"
	"github.com/gin-gonic/gin"
	"gopkg.in/yaml.v3"
	"io"
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

func BewProxy(context *gin.Context) *ProxyStr {
	return &ProxyStr{context}
}

func (s ProxyStr) ReverseProxy() (*httputil.ReverseProxy, error) {
	path := s.context.Request.URL.Path
	c, e := s.loadServiceConfig(path)
	if e != nil {
		return nil, e
	}
	urlTxt, err := s.loadServiceMethod(s.context.Request.Method, c, path)
	if err != nil {
		return nil, err
	}
	parsedUrl, err := url.Parse(urlTxt)
	if err != nil {
		return nil, err
	}

	reverseProxy := httputil.NewSingleHostReverseProxy(parsedUrl)
	reverseProxy.Director = func(request *http.Request) {
		s.setQuery(request, s.context.Request.URL.RawQuery)
		s.setBody(request, s.context.Request.Method, s.context.Request.Body)
		request.Host = parsedUrl.Host
		request.URL.Scheme = parsedUrl.Scheme
		request.URL.Host = parsedUrl.Host
		request.URL.Path = parsedUrl.Path
		request.Header.Set("x-account-token", s.context.Request.Header.Get("x-account-token"))
	}
	return reverseProxy, nil
}

func (s ProxyStr) setQuery(request *http.Request, query string) {
	if len(query) <= 0 {
		return
	}
	request.URL.RawQuery = query
}

func (s ProxyStr) setBody(request *http.Request, method string, reqBody io.ReadCloser) {
	if method != "POST" && method != "PUT" && method != "PATCH" {
		return
	}
	bodyAsBytes, _ := ioutil.ReadAll(reqBody)
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

func (s ProxyStr) loadServiceConfig(path string) (*ServiceConfig, error) {
	parts := strings.Split(strings.TrimPrefix(path, "/"), "/")
	if len(parts) <= 1 {
		return nil, errors.New(fmt.Sprintf("failed to parse target host from path: %s", path))
	}
	serviceName := fmt.Sprintf("%s", parts[0])
	yamlFile, err := ioutil.ReadFile("services-config/" + serviceName + ".yaml")
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

func (s ProxyStr) loadServiceMethod(httpMethod string, c *ServiceConfig, path string) (string, error) {
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
				targetUrl, _ := s.buildUrl(c.Servers[0].Url, path) //todo Servers[0] ??
				return targetUrl, nil
			}
		}
	}
	return "", errors.New("something is wrong")
}
