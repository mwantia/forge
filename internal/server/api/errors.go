package api

import "github.com/gin-gonic/gin"

type errorDetail struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

type errorResponse struct {
	Error errorDetail `json:"error"`
}

func respondError(c *gin.Context, status int, code, message string) {
	c.JSON(status, errorResponse{Error: errorDetail{Code: code, Message: message}})
}
