package main

import (
	"fmt"
	"github.com/sjqzhang/hrq"
	"net/http"
	"runtime"
	"time"
)

func main() {

	//hrq.Conf.TimeoutProcess=1000

	//针对第三方库的支持
	hrq.Conf.SetWorkerOption(http.MethodGet,"/hello/:name",hrq.WithWorkerOption(10,10))


	// define http request handler
	hrq.GET("/hello/:name", func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(time.Millisecond*100)
		w.Write([]byte("hello"))
	})

	hrq.GET("/world", func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("world"))
	},hrq.WithWorkerOption(10,100))

	go func() {
		for {
			time.Sleep(time.Second)
			fmt.Println(runtime.NumGoroutine())
		}
	}()


	// start http server
	hrq.ListenAndServe(":8080")

}

//package main
//
//import (
//	"encoding/json"
//	"fmt"
//	"github.com/gin-gonic/gin"
//	"github.com/sjqzhang/hrq"
//	"io/ioutil"
//	"net/http"
//	"runtime"
//	"time"
//)
//
//func main() {
//
//
//
//	hrq.Conf.Workers=3
//	hrq.Conf.MaxQueueSize=10
//	gin.SetMode(gin.ReleaseMode)
//	gin.DefaultWriter = ioutil.Discard
//	router := gin.Default()
//	router.Use(hrq.MiddlewareForGin())// 使用中间件以激活队列功能
//	hrq.GET("/hello", func(w http.ResponseWriter, req *http.Request) {
//		time.Sleep(300*time.Millisecond)
//		w.Write([]byte("hello world"))
//	},hrq.WithWorkerOption(10,100))
//
//	hrq.POST("/upload", func(w http.ResponseWriter, req *http.Request) {
//		req.ParseMultipartForm(32 << 20)
//		_,b,_:=req.FormFile("file")
//		fmt.Println(b.Size)
//		w.Write([]byte("upload file"))
//	})
//
//	router.GET("/world", func(c *gin.Context) {
//		c.String(200, "world, hello ")
//	})
//	hrq.ApplyToGin(router) // 将hrq的路由应用到gin中
//
//
//	go func() {
//		for {
//			time.Sleep(time.Second)
//			m:=hrq.GetStat()
//			s,_:=json.Marshal(m)
//			fmt.Println(string(s))
//			fmt.Println(runtime.NumGoroutine())
//		}
//	}()
//
//
//	router.Run(":8080")
//
//
//}

//package main
//
//import (
//	"fmt"
//	"net/http"
//)
//
//func handler(writer http.ResponseWriter, request *http.Request) {
//	fmt.Fprintf(writer, "Hello World!")
//}
//
//func main() {
//	http.HandleFunc("/", handler)
//	http.ListenAndServe(":8080", nil)
//}

//package main
//
//import (
//	"github.com/beego/beego/v2/server/web"
//	"github.com/beego/beego/v2/server/web/context"
//	"github.com/sjqzhang/hrq"
//	"net/http"
//	"time"
//)
//
//
//func main() {
//	hrq.Conf.Workers = 1000
//	hrq.Conf.MaxQueueSize = 10000
//	hrq.Conf.MaxConnection=20000
//	hrq.Conf.EnableOverload=false
//	hrq.GET("/hello", func(w http.ResponseWriter, req *http.Request) {
//		//fmt.Println(req.Context())
//		//time.Sleep(time.Second*3)
//		time.Sleep(time.Millisecond*200)
//		w.Write([]byte("hello world, from hrq"))
//	})
//	web.BeeApp.Handlers.AddMethod("GET", "/hello2", func(ctx *context.Context) {
//		ctx.WriteString("hello2 world, from beego")
//	})
//	hrq.InstallFilterChanForBeego()
//	hrq.ApplyToBeego(web.BeeApp)
//	web.Run()
//
//}
//
//package main
//
//import (
//	"github.com/sjqzhang/hrq"
//	"net/http"
//	"time"
//
//	"github.com/labstack/echo/v4"
//	"github.com/labstack/echo/v4/middleware"
//)
//
//func main() {
//	// Echo instance
//	e := echo.New()
//
//	// Middleware
//	e.Use(middleware.Logger())
//	e.Use(middleware.Recover())
//	e.Use(hrq.MiddlewareForEcho())
//
//	hrq.Conf.Workers=2
//	hrq.Conf.MaxQueueSize=10
//
//	hrq.Conf.TimeoutProcess = 1000
//
//	hrq.GET("/hello", func(w http.ResponseWriter, req *http.Request) {
//
//		time.Sleep(time.Second * 3)
//		w.Write([]byte("hello world,hrq"))
//	})
//
//	// Route => handler
//	e.GET("/", func(c echo.Context) error {
//		return c.String(http.StatusOK, "Hello, World!\n")
//	})
//
//	hrq.ApplyToEcho(e)
//
//	go func() {
//		for {
//			time.Sleep(time.Second)
//			//print(hrq.GetStat())
//		}
//	}()
//
//	// Start server
//	e.Logger.Fatal(e.Start(":8080"))
//}
