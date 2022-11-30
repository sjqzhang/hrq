# http-request-queue=(hrq)

http-request-queue=(hrq) is a simple HTTP request queue for the web server.

## Install

```go
go get github.com/sjqzhang/hrq
```

## Basic Setup

```go
hrq.Conf.Workers = 100
hrq.Conf.MaxQueueSize = 1000
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

## integration with beego

```go

package main

import (
	"fmt"
	"github.com/beego/beego/v2/server/web"
	"github.com/beego/beego/v2/server/web/context"
	"github.com/sjqzhang/hrq"
	"net/http"
)


func main() {
	hrq.Conf.Workers = 100
	hrq.Conf.MaxQueueSize = 1000
	hrq.GET("/hello", func(w http.ResponseWriter, req *http.Request) {
		fmt.Println(req.Context())
		w.Write([]byte("hello world, from hrq"))
	})
	web.BeeApp.Handlers.AddMethod("GET", "/hello2", func(ctx *context.Context) {
		ctx.WriteString("hello2 world, from beego")
	})
	hrq.InstallFilterChanForBeego()
	hrq.ApplyToBeego(web.BeeApp)
	web.Run()

}
```
