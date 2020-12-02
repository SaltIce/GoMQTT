package main

import "github.com/kataras/iris/v12"

func main()  {
	app := iris.New()

	app.Get("/", func(ctx iris.Context) {
		ctx.WriteString("你好")
	})
	app.Run(iris.Addr(":80"))
}
