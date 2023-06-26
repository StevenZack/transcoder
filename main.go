package main

import (
	"flag"
	"log"
	"mime"
	"net/http"
	"path/filepath"
	"strconv"

	"github.com/StevenZack/tools/cmdToolkit"
	"github.com/StevenZack/tools/strToolkit"
	"github.com/StevenZack/transcoder/internal/core"
	"github.com/StevenZack/transcoder/internal/gx"
	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
)

var (
	jwtSecret = flag.String("jwt-secret", "", "Use JWT authentication")
	port      = flag.Int("p", 80, "port")
	upgrader  = websocket.Upgrader{}
)

func init() {
	log.SetFlags(log.Lshortfile)
}

func main() {
	flag.Parse()

	out, e := cmdToolkit.Run("ffmpeg", "-version")
	if e != nil {
		log.Println(e)
		return
	}
	println(out)

	r := gin.Default()
	r.LoadHTMLGlob("internal/web/*")
	r.GET("/", func(ctx *gin.Context) {
		ctx.Redirect(http.StatusFound, "/web/index.html")
	})

	//web
	web := r.Group("web")
	web.GET("index.html", webHome)
	web.GET("add.html", func(c *gin.Context) { c.HTML(200, "add.html", nil) })

	// api
	api := r.Group("api")
	api.POST("tasks", postTasks)
	api.GET("tasks/:id", getTask)
	api.GET("tasks", getAllTasks)
	api.DELETE("tasks/:id", deleteTask)
	api.GET("tasks/:id/ws", ws)

	r.Static("files", core.AppDir)

	println("started on http://localhost:" + strconv.Itoa(*port))
	e = r.Run(":" + strconv.Itoa(*port))
	if e != nil {
		log.Fatal(e)
	}
}

func ws(c *gin.Context) {
	id := c.Param("id")
	_, ok := core.TaskMap.Load(id)
	if !ok {
		gx.NotFound(c, id)
		return
	}

	conn, e := upgrader.Upgrade(c.Writer, c.Request, nil)
	if e != nil {
		log.Println(e)
		gx.ServerError(c, e)
		return
	}
	defer conn.Close()

}

func webHome(c *gin.Context) {
	l := []core.Task{}
	core.TaskMap.Range(func(key string, value core.Task) bool {
		e := value.LoadProgress()
		if e != nil {
			log.Println(e)
		}

		l = append(l, value)
		return true
	})
	c.HTML(200, "index.html", gin.H{
		"Tasks": l,
	})
}
func deleteTask(c *gin.Context) {
	id := c.Param("id")
	core.TaskMap.Delete(id)
}
func getAllTasks(c *gin.Context) {
	l := []core.Task{}
	core.TaskMap.Range(func(key string, value core.Task) bool {
		l = append(l, value)
		return true
	})
	c.JSON(200, l)
}

func postTasks(c *gin.Context) {
	form, e := c.MultipartForm()
	if e != nil {
		log.Println(e)
		gx.ServerError(c, e)
		return
	}

	tasks := []core.Task{}
	fhs := form.File["file"]
	for _, fh := range fhs {
		mime := mime.TypeByExtension(filepath.Ext(fh.Filename))
		mime = strToolkit.SubBefore(mime, "/", mime)
		switch mime {
		case "image", "video":
			task, e := core.CreateTask(fh, "")
			if e != nil {
				log.Println(e)
				gx.ServerError(c, e)
				return
			}
			core.TaskMap.Store(task.Id, *task)
			tasks = append(tasks, *task)
		default:
			gx.BadRequest(c, "Unsupported file type :", fh.Filename)
			return
		}
	}

	c.JSON(200, gin.H{
		"tasks": tasks,
	})
}

func getTask(c *gin.Context) {
	id := c.Param("id")
	v, ok := core.TaskMap.Load(id)
	if !ok {
		gx.NotFound(c, id)
		return
	}
	c.JSON(200, v)
}
