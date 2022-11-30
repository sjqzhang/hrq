# http-request-queue=(hrq)

http-request-queue=(hrq) is a simple HTTP request queue for the web server.

## Install

```go
go get github.com/sjqzhang/hrq
```

## Basic Setup

```go
// hrq.Init(worker int, queueSize int)
hrq.Init(100, 1000) // 想获得更好性能时需手动设置worker和queueSize，否则默认为cpu*10和cpu*100
```

## Basic Usage

```go
package main

import (
	"github.com/sjqzhang/hrq"
	"net/http"
)

func main() {
	// define http request handler
	hrq.GET( "/", func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("hello world"))
	})
	// start http server
	hrq.ListenAndServe(":8080")

}
```

## integration with gin

```go
package main

import (
	"github.com/gin-gonic/gin"
	"github.com/sjqzhang/hrq"
	"io/ioutil"
	"net/http"
)
func main() {

	gin.SetMode(gin.ReleaseMode)
	gin.DefaultWriter = ioutil.Discard
	router := gin.Default()
	router.Use(hrq.MiddlewareForGin())// 使用中间件以激活队列功能
	hrq.GET("/hello", func(w http.ResponseWriter, req *http.Request) {
		w.Write([]byte("hello world"))
	})

	router.GET("/world", func(c *gin.Context) {
		c.String(200, "world, hello ")
	})
	hrq.ApplyToGin(router) // 将hrq的路由应用到gin中
	router.Run(":8080")
}

```
