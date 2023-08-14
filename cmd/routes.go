package main

import (
	"main/handlers"

	"github.com/gofiber/fiber/v2"
)

func start() {
	app := fiber.New()

	app.Get("/", handlers.Home)

	app.Post("/pdf", handlers.SaveFile)

	app.Listen(":3000")
}
