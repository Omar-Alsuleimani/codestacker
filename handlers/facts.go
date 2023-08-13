package handlers

import (
	"context"
	"database/sql"
	"fmt"
	"log"
	"main/database"
	"os"

	"github.com/gofiber/fiber/v2"
	"github.com/jackc/pgx/v4"
	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"
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

func SaveFile(c *fiber.Ctx) error {
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
	minioEndpoint := "127.0.0.1:9000"
	minioAccessKey := "minioadmin"
	minioSecretKey := "minioadmin"
	useSSL := false

	// Initialize MinIO client
	minioClient, err := minio.New(minioEndpoint, &minio.Options{
		Creds:  credentials.NewStaticV4(minioAccessKey, minioSecretKey, ""),
		Secure: useSSL,
	})
	if err != nil {
		log.Fatalln(err)
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "Failed to open the file",
		})
	}

	bucketName := "pdf"
	objectName := file.Filename
	contentType := "application/pdf"
	n, err := minioClient.PutObject(c.Context(), bucketName, objectName, inFile, -1, minio.PutObjectOptions{
		ContentType: contentType,
	})
	if err != nil {
		log.Fatalln(err)
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "Failed to upload the file to MinIO",
		})
	}
	return c.JSON(fiber.Map{
		"message": "File uploaded successfully",
		"size":    n,
	})
}
