package middleware

import (
	"net/http"

	"github.com/gin-gonic/gin"
	customErr "github.com/saurabhkr78/sudowallet/monolith/internal/errors"
	"github.com/saurabhkr78/sudowallet/monolith/internal/logger"
)

func ErrorHandler() gin.HandlerFunc {
	return func(c *gin.Context) {
		//let the request travel through rest of the application
		c.Next()
		//when it comes back check whether anything went wrong or not?
		if len(c.Errors) > 0 {
			//Give me the last Gin error object(c.Errors.Last()) then From that Gin error object, give me its underlying error(c.Errors.Last().Err)
			//the static type of err is error
			err := c.Errors.Last().Err

			// now check if is it our application custom error or not?
			/*
				here err.(*customErr.AppErr) is a type assertion mean
							"What is actually inside err?"
					                 │
					                 ▼
					       Is it *customErr.AppErr?
					                 │
					          ┌──────┴──────┐
					          │             │
					         YES            NO
					          │             │
					          ▼             ▼
					       return        assertion
					      *AppErr        fails
				customErr is alias for package and AppErr The AppErr type defined in that package. so *customErr.AppErr a pointer to an AppErr.
			*/
			if appErr, ok := err.(*customErr.AppError); ok {
				//log it

				currContext := c.Request.Context()
				logger.Warn(currContext,
					"Client error occured",
					"code", appErr.Code,
					"message", appErr.Message,
					"statuscode", appErr.StatusCode)
				//convert it into json
				//gin.H is just a convenient type for constructing a JSON object.
				//the underlying type is type H map[string]any
				//gin.H isn't required. You can pass a struct, slice, map, etc. directly to c.JSON()
				c.JSON(appErr.StatusCode, gin.H{
					"success": false,
					"error":   appErr,
				})
				return
			}
			//then it's an unexpected error other from the custom ErrorHandler
			currContext := c.Request.Context()

			logger.Error(currContext, "unhandeled error occured", "error", err.Error())
			//send as json but dont expose to the client
			c.JSON(http.StatusInternalServerError, gin.H{
				"success": false,
				"error":   customErr.ErrInternalServer,
			})
		}
	}
}
