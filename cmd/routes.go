package main

import (
	"main/handlers"

	"github.com/gofiber/fiber/v2"
)

func start() {
	app := fiber.New()

	app.Get("/", handlers.Home)

	app.Post("/uploadPDF", handlers.SaveFile)

	app.Get("/listPDF", handlers.ListFiles)

	app.Listen(":3000")
}
