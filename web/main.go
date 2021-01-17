package main

import (
	"fmt"
	"github.com/kataras/iris/v12"
)

func main() {
	app := iris.New()

	app.Get("/", func(ctx iris.Context) {
		fmt.Println(ctx.Request().Header.Get("Accept"))
		ctx.WriteString("你好")
	})
	app.Run(iris.Addr(":80"))
}
