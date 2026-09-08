package middleware

import (
	"api/utils/constants"
	"api/utils/errs"
	"api/utils/role"
	"crypto/subtle"
	"github.com/gin-gonic/gin"
	"os"
)

func (mw *Mw) RolesMiddleware(allowedRoles ...string) gin.HandlerFunc {
	return func(c *gin.Context) {
		bearerToken := c.GetHeader("Authorization")
		loaderToken := os.Getenv("BEARER_DATA_LOADER_TOKEN")
		for _, allowedRole := range allowedRoles {
			if allowedRole == role.DataLoaderService && loaderToken != "" && subtle.ConstantTimeCompare([]byte(bearerToken), []byte("Bearer "+loaderToken)) == 1 {
				c.Next()
				return
			}
		}

		userID, exists := c.Get(constants.ContextUserID)
		if !exists {
			errs.HandleError(c, errs.Unauthorized)
			c.Abort()
			return
		}

		var universityID uint
		if uniID, ok := c.Get(constants.ContextUniverID); ok {
			universityID = uniID.(uint)
		}

		if !mw.adminService.HasPermission(userID.(uint), allowedRoles, universityID) {
			errs.HandleError(c, errs.NotEnoughRights)
			c.Abort()
			return
		}

		c.Next()
	}
}

func (mw *Mw) NoMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Next()
	}
}
