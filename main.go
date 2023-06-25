package main

import (
	"log"

	"github.com/StevenZack/tools/cmdToolkit"
	"github.com/gin-gonic/gin"
)

func init() {
	log.SetFlags(log.Lshortfile)
}

func main() {
	out, e := cmdToolkit.Run("ffmpeg", "--version")
	if e != nil {
		log.Println(e)
		return
	}
	println(out)

	r := gin.Default()
	api := r.Group("api")
	api.POST("tasks", postTasks)
}

func postTasks(c *gin.Context) {
	
}
