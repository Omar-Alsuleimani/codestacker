package handlers

import (
	"bytes"
	"context"
	"fmt"
	"log"
	"main/database"
	"os"
	"strings"

	"github.com/dslipak/pdf"
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
		return sendErrorStatus(c, "Failed to open the pdf file")
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
		return sendErrorStatus(c, "Failed to initialize minio client")
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
		return sendErrorStatus(c, "Failed to upload the file to MinIO")
	}

	//Insert record of upload into the database
	ctx := context.Background()
	urlExample := fmt.Sprintf("postgres://%s:%s@db:5432", os.Getenv("DB_USER"), os.Getenv("DB_NAME"))
	conn, err := pgx.Connect(context.Background(), urlExample)
	if err != nil {
		return sendErrorStatus(c, "Failed to connect to the database")
	}
	defer conn.Close(context.Background())

	err = c.SaveFile(file, file.Filename)
	defer os.Remove(file.Filename)
	if err != nil {
		return sendErrorStatus(c, "Failed to save pdf file")
	}

	pdfFileName := file.Filename
	reader, text, err := readPdf(pdfFileName)
	if err != nil {
		return sendErrorStatus(c, "Failed to read pdf as text")
	}

	queries := database.New(conn)

	insertedRecord, err := queries.CreateRecord(ctx, database.CreateRecordParams{
		Name:       file.Filename,
		Numofpages: int32(reader.NumPage()),
		Size:       int32(file.Size),
	})
	if err != nil {
		return sendErrorStatus(c, "Failed to create a record for the file")
	}

	//Split sentences and upload them to the db
	sentences := strings.Split(text, ".")
	for _, sentence := range sentences {
		if sentence != "" {
			_, err = queries.CreateSentence(ctx, database.CreateSentenceParams{
				Sentence: sentence,
				Pdfid:    insertedRecord.ID,
			})
			if err != nil {
				return sendErrorStatus(c, "Failed to add sentence to the database")
			}
		}
	}
	return c.SendString(insertedRecord.Name)
}

func ListFiles(c *fiber.Ctx) error {
	ctx := context.Background()
	urlExample := fmt.Sprintf("postgres://%s:%s@db:5432", os.Getenv("DB_USER"), os.Getenv("DB_NAME"))
	conn, err := pgx.Connect(context.Background(), urlExample)
	if err != nil {
		return sendErrorStatus(c, "Failed to connect to the database")
	}
	defer conn.Close(context.Background())

	queries := database.New(conn)

	records, err := queries.ListRecords(ctx)
	if err != nil {
		return sendErrorStatus(c, "Failed to get the list of records")
	}

	return c.JSON(records)
}

func sendErrorStatus(c *fiber.Ctx, err string) error {
	return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
		"error": err,
	})
}

func readPdf(path string) (*pdf.Reader, string, error) {
	r, err := pdf.Open(path)
	if err != nil {
		return nil, "", err
	}
	var buf bytes.Buffer
	b, err := r.GetPlainText()
	if err != nil {
		return nil, "", err
	}
	buf.ReadFrom(b)
	return r, buf.String(), nil
}
