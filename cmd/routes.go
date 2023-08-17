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

	app.Get("/searchKeyword/:key", handlers.SearchKeyword)

	app.Get("/getPDF/:id", handlers.GetPDF)

	app.Get("/listSentences/:id", handlers.ListSentences)

	app.Get("/getOccurrence/:id/:key", handlers.GetOccurrence)

	app.Get("/getMostOccurring/:id", handlers.GetMostOccurring)

	app.Get("/getPDF/:id/:page", handlers.GetPdfPage)

	app.Listen(":3000")
}
