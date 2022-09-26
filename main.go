package main

import (
	"go.uber.org/fx"
)

func main() {
	fx.New(
		fx.Provide(InitTracerProvider),
		fx.Provide(InitResty),
		fx.Provide(InitGinEngine),
		fx.Invoke(RegisterHandler),
	).Run()
}
