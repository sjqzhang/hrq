package main

import (
	"github.com/gin-gonic/gin"
	"github.com/sjqzhang/hrq"
	"io/ioutil"
	"net/http"
)
//
//func main() {
//
//
//	// define http request handler
//	hrq.GET( "/", func(w http.ResponseWriter, r *http.Request) {
//		w.Write([]byte("hello world"))
//	})
//
//	// start http server
//	hrq.ListenAndServe(":8080")
//
//
//
//}

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



	//hrq.ApplyFromGin(router)

	//
	//router.Any("/", func(c *gin.Context) {
	//	c.String(http.StatusOK, "Hello World")
	//})
	//
	//router.POST("/form_post", func(c *gin.Context) {
	//	message := c.PostForm("message")
	//	nick := c.DefaultPostForm("nick", "anonymous")
	//
	//	c.JSON(http.StatusOK, gin.H{
	//		"status":  "posted",
	//		"message": message,
	//		"nick":    nick,
	//	})
	//})
	router.Run(":8080")

	//fmt.Println(hrq.ListenAndServe(":8080"))
}


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