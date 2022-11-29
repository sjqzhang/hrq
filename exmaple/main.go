package main

import (
	"github.com/gin-gonic/gin"
	"github.com/sjqzhang/hrq"
	"io/ioutil"
	"net/http"
)

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

	//
	//hrq.GET("/x", func(w http.ResponseWriter, req *http.Request) {
	//	w.Write([]byte("hello world  sdfasfasdfa"))
	//})


	hrq.GET("/xx",func(w http.ResponseWriter, req *http.Request)  {
		w.Write([]byte("hello world  hrq"))
	})



	router.GET("/x", func(c *gin.Context) {
		c.String(200, "hello world  xxxx")
	})

	hrq.ApplyForGin(router)

	router.Use(hrq.MiddlewareForGin())
	//hrq.ApplyForGin(router)

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
}
