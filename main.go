package main

import (
	"go.uber.org/fx"
)

func main() {
	fx.New(
		fx.Provide(InitMeterProvider),
		fx.Provide(InitTracerProvider),
		fx.Provide(InitHTTPClient),
		fx.Provide(InitResty),
		fx.Provide(InitCloudEventsClient),
		fx.Provide(InitGinEngine),
		fx.Invoke(RegisterHandler),
	).Run()
}
