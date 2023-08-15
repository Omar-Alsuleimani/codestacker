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
	"github.com/jdkato/prose/v2"
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
	err = splitAndStore(ctx, text, queries, insertedRecord, c)
	if err != nil {
		return err
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

func SearchKeyword(c *fiber.Ctx) error {
	keyword := c.Params("key")
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

	result := fiber.Map{}
	for _, record := range records {
		sentences, err := queries.ListRecordSentences(ctx, record.ID)
		if err != nil {
			return sendErrorStatus(c, "Failed to get the list of sentences for record: "+fmt.Sprint(record.ID))
		}

		containers := fiber.Map{}
		i := 1
		for _, sentence := range sentences {

			isContained := false
			for _, word := range strings.Split(strings.TrimSpace(sentence.Sentence), " ") {
				if strings.ToLower(word) == strings.ToLower(keyword) {
					isContained = true
					break
				}
			}
			if isContained {
				containers[fmt.Sprint(i)] = sentence.Sentence
				i++
			}
		}

		if len(containers) > 0 {
			result[fmt.Sprintf("PDF ID %d", record.ID)] = containers
		}
	}

	if len(result) == 0 {
		return c.JSON("Not found")
	}
	return c.JSON(result)
}

func GetPDF(c *fiber.Ctx) error {
	ctx := context.Background()
	urlExample := fmt.Sprintf("postgres://%s:%s@db:5432", os.Getenv("DB_USER"), os.Getenv("DB_NAME"))
	conn, err := pgx.Connect(context.Background(), urlExample)
	if err != nil {
		return sendErrorStatus(c, "Failed to connect to the database")
	}
	defer conn.Close(context.Background())

	queries := database.New(conn)

	id, err := c.ParamsInt("id", -1)
	if err != nil || id == -1 {
		return sendErrorStatus(c, "Invalid id provided")
	}

	record, err := queries.GetRecord(ctx, int32(id))
	if err != nil {
		return sendErrorStatus(c, "A file with the id provided does not exist")
	}

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
		return sendErrorStatus(c, "Failed to initialize minio client")
	}

	//upload file to MinIO
	bucketName := "pdf"
	objectName := record.Name

	file, err := minioClient.GetObject(ctx, bucketName, objectName, minio.GetObjectOptions{})
	if err != nil {
		return sendErrorStatus(c, "Failed to get the file from MinIO")
	}
	defer file.Close()

	localFile, err := os.Create(record.Name)
	if err != nil {
		return sendErrorStatus(c, "Failed to create a temporary copy of the file")
	}
	defer os.Remove(localFile.Name())
	defer localFile.Close()

	buffer := make([]byte, 1024)
	for {
		n, err := file.Read(buffer)
		if err != nil {
			break
		}
		localFile.Write(buffer[:n])
	}

	return c.SendFile(localFile.Name())
}

func splitAndStore(ctx context.Context, text string, queries *database.Queries, insertedRecord database.Record, c *fiber.Ctx) error {
	//Sentence package initialization.
	doc, err := prose.NewDocument(text)
	if err != nil {
		return sendErrorStatus(c, "Failed to initialize tokenizer package")
	}

	//Store sentences in database.
	for _, sentence := range doc.Sentences() {
		_, err := queries.CreateSentence(ctx, database.CreateSentenceParams{
			Sentence: sentence.Text,
			Pdfid:    insertedRecord.ID,
		})
		if err != nil {
			return sendErrorStatus(c, "Failed to add a sentence to the database")
		}
	}
	return nil
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
