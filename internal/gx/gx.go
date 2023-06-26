package gx

import (
	"fmt"
	"net/http"

	"github.com/gin-gonic/gin"
)

func ServerError(c *gin.Context, e error) {
	c.AbortWithError(500, e)
}

func BadRequest(c *gin.Context, args ...any) {
	c.AbortWithStatusJSON(http.StatusBadRequest, gin.H{
		"code":    400,
		"message": fmt.Sprint(args...),
	})
}

func NotFound(c *gin.Context, args ...any) {
	c.AbortWithStatusJSON(http.StatusNotFound, gin.H{
		"code":    404,
		"message": fmt.Sprint(args...),
	})
}
