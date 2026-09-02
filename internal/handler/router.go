package handler

import (
	"github.com/gin-gonic/gin"

	"golang-gin/internal/pkg/password"
	"golang-gin/internal/repository/mysql"
	"golang-gin/internal/service"
)

// NewEngine wires the whole application and returns the router.
//
// Assembly lives here rather than in main so integration tests build the
// engine through the exact same path production does. db is a mysql.DB, so a
// test can pass a *sql.Tx it rolls back afterwards.
func NewEngine(db mysql.DB) *gin.Engine {
	txRunner := mysql.NewTxRunner(db)
	customerRepo := mysql.NewCustomerRepository(db)
	passwordRepo := mysql.NewCustomerPasswordRepository(db)

	hasher := password.NewHasher(password.DefaultParams())
	customerService := service.NewCustomerService(txRunner, customerRepo, passwordRepo, hasher)

	authHandler := NewAuthHandler(customerService)

	r := gin.New()
	r.Use(gin.Recovery())

	r.GET("/", Health)
	r.GET("/health", Health)

	auth := r.Group("/auth")
	auth.POST("/signup", authHandler.SignUp)

	return r
}
