package handlers

import (
	"context"
	"database/sql"
	"fmt"
	"main/database"
	"os"

	"github.com/gofiber/fiber/v2"
	"github.com/jackc/pgx/v4"
)

type Author struct {
	Name string `json:"name"`
	Bio  string `json:"bio"`
}

func Home(c *fiber.Ctx) error {
	return c.SendString("Hello, Omar!")
}

func CreateAuthor(c *fiber.Ctx) error {
	author := new(Author)
	if err := c.BodyParser(&author); err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"message": err.Error(),
		})
	}
	ctx := context.Background()
	urlExample := fmt.Sprintf("postgres://%s:%s@db:5432", os.Getenv("DB_USER"), os.Getenv("DB_NAME"))
	conn, err := pgx.Connect(context.Background(), urlExample)
	if err != nil {
		return err
	}
	defer conn.Close(context.Background())

	queries := database.New(conn)

	inserted, err := queries.CreateAuthor(ctx, database.CreateAuthorParams{
		Name: author.Name,
		Bio:  sql.NullString{String: author.Bio, Valid: author.Bio != ""},
	})

	if err != nil {
		return err
	}

	return c.SendString(inserted.Name + " " + inserted.Bio.String)
}

func saveFile(c *fiber.Ctx) error {
	file, err := c.FormFile("pdf")
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "File upload failed",
		})
	}
	inFile, err := file.Open()
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "Failed to open the file",
		})

	}
	defer inFile.Close()

}
