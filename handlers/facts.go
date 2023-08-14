package handlers

import (
	"context"
	"fmt"
	"log"
	"main/database"
	"os"

	"github.com/gofiber/fiber/v2"
	"github.com/jackc/pgx/v4"
	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"
)

func Home(c *fiber.Ctx) error {
	return c.SendString("Hello, Omar!")
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
	minioEndpoint := "minio:9000"
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

	//upload file to MinIO
	bucketName := "pdf"
	objectName := file.Filename
	contentType := "application/pdf"
	_, err = minioClient.PutObject(c.Context(), bucketName, objectName, inFile, -1, minio.PutObjectOptions{
		ContentType: contentType,
	})
	if err != nil {
		log.Fatalln(err)
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "Failed to upload the file to MinIO",
		})
	}

	//Insert record of upload into the database
	ctx := context.Background()
	urlExample := fmt.Sprintf("postgres://%s:%s@db:5432", os.Getenv("DB_USER"), os.Getenv("DB_NAME"))
	conn, err := pgx.Connect(context.Background(), urlExample)
	if err != nil {
		return err
	}
	defer conn.Close(context.Background())

	queries := database.New(conn)

	insertedRecord, err := queries.CreateRecord(ctx, file.Filename)

	if err != nil {
		return err
	}

	return c.SendString(insertedRecord.Name)
}
