package main

import (
	"flag"
	"fmt"
	"log"
	"mime"
	"net/http"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/StevenZack/tools/cmdToolkit"
	"github.com/StevenZack/tools/strToolkit"
	"github.com/StevenZack/transcoder/internal/core"
	"github.com/StevenZack/transcoder/internal/gx"
	"github.com/StevenZack/transcoder/internal/vars"
	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"
	"github.com/gorilla/websocket"
)

var (
	jwtSecret = flag.String("jwt-secret", "", "Use JWT authentication")
	origins   = flag.String("origins", "*", "Allowed origins, split by [,]")
	port      = flag.Int("p", 80, "port")
	upgrader  websocket.Upgrader
)

func init() {
	log.SetFlags(log.Lshortfile)
	gin.SetMode(vars.Mode)
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
	corsConfig := cors.Config{
		AllowOrigins:     strings.Split(*origins, ","),
		AllowCredentials: true,
		AllowHeaders:     []string{"authorization", "content-type"},
	}
	r.Use(cors.New(corsConfig))
	upgrader = websocket.Upgrader{
		CheckOrigin: func(r *http.Request) bool {
			origin := r.Header.Get("Origin")
			for _, v := range corsConfig.AllowOrigins {
				if v == "*" || v == origin {
					return true
				}
			}
			return false
		},
	}

	//web
	if gin.Mode() == gin.DebugMode {
		r.LoadHTMLGlob("internal/web/*")
		r.GET("/", func(ctx *gin.Context) {
			ctx.Redirect(http.StatusFound, "/web/index.html")
		})
		web := r.Group("web")
		web.GET("index.html", webHome)
		web.GET("add.html", func(c *gin.Context) { c.HTML(200, "add.html", nil) })
	}

	// api
	api := r.Group("api")
	api.POST("tasks", authMiddleware, postTasks)
	api.GET("tasks/:id", authMiddleware, getTask)
	if gin.Mode() == gin.DebugMode {
		api.GET("tasks", authMiddleware, getAllTasks)
	}
	api.DELETE("tasks/:id", authMiddleware, deleteTask)
	api.GET("tasks/:id/ws", authMiddleware, ws)

	r.Static(core.PUBLIC_PREFIX, core.AppDir)

	println("started on http://localhost:" + strconv.Itoa(*port))
	e = r.Run(":" + strconv.Itoa(*port))
	if e != nil {
		log.Fatal(e)
	}
}

func ws(c *gin.Context) {
	id := c.Param("id")
	task, ok := core.TaskMap.Load(id)
	if !ok {
		gx.NotFound(c, id)
		return
	}
	sub := getSub(c)
	if task.User != sub {
		c.AbortWithStatus(http.StatusForbidden)
		return
	}

	conn, e := upgrader.Upgrade(c.Writer, c.Request, nil)
	if e != nil {
		log.Println(e)
		gx.ServerError(c, e)
		return
	}
	defer conn.Close()

	active := true
	go func() {
		for {
			_, _, e := conn.ReadMessage()
			if e != nil {
				break
			}
		}
		active = false
		task.Clean()
		core.TaskMap.Delete(id)
	}()
	task.LoadProgress()
	for active {
		if task.IsEnded {
			time.Sleep(time.Second * 30)
		} else {
			time.Sleep(time.Second * 2)
		}
		task, ok = core.TaskMap.Load(id)
		if !ok {
			gx.NotFound(c, id)
			return
		}
		task.LoadProgress()
		e = conn.WriteJSON(task)
		if e != nil {
			break
		}
	}
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
	task, ok := core.TaskMap.Load(id)
	if !ok {
		return
	}
	sub := getSub(c)
	if task.User != sub {
		c.AbortWithStatus(http.StatusForbidden)
		return
	}

	task.Clean()
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
			task, e := core.CreateTask(fh, getSub(c))
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
	if v.User != getSub(c) {
		c.AbortWithStatus(http.StatusForbidden)
		return
	}

	c.JSON(200, v)
}
func getSub(c *gin.Context) string {
	return c.Value("sub").(string)
}
func authMiddleware(c *gin.Context) {
	if jwtSecret == nil {
		return
	}

	accessToken := c.GetHeader("Authorization")
	if accessToken == "" {
		accessToken = c.GetHeader("authorization")
		if accessToken == "" {
			accessToken = c.Query("token")
			if accessToken == "" {
				c.AbortWithError(http.StatusUnauthorized, nil)
				return
			}
		}
	}

	sub, e := parseAccessToken(accessToken)
	if e != nil {
		c.AbortWithError(http.StatusUnauthorized, e)
		return
	}
	c.Set("sub", sub)
	c.Next()
}

func parseAccessToken(accessToken string) (string, error) {
	t, e := jwt.Parse(accessToken, func(t *jwt.Token) (interface{}, error) {
		return []byte(*jwtSecret), nil
	})
	if e != nil {
		if strings.Contains(e.Error(), jwt.ErrTokenExpired.Error()) {
			return "", jwt.ErrTokenExpired
		}
		return "", e
	}
	exp, e := t.Claims.GetExpirationTime()
	if e != nil {
		return "", e
	}
	if exp.Before(time.Now()) {
		return "", fmt.Errorf("token expired")
	}
	uid, e := t.Claims.GetSubject()
	if e != nil {
		return "", e
	}
	if !t.Valid {
		return "", e
	}

	return uid, nil
}
