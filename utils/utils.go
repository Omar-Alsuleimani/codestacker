package utils

import (
	"bytes"
	"context"
	"fmt"
	"image/jpeg"
	"io"
	"log"
	"main/database"
	"os"
	"regexp"
	"strconv"
	"strings"

	"github.com/bbalet/stopwords"
	"github.com/dslipak/pdf"
	"github.com/gen2brain/go-fitz"
	"github.com/gofiber/fiber/v2"
	"github.com/jackc/pgx/v4"
	"github.com/jdkato/prose/v2"
	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"
)

func GetWelcome() fiber.Map {
	return fiber.Map{
		"To upload a PDF":                                        "POST /uploadPDF",
		"To get a list of uploaded PDFs":                         "GET /listPDF",
		"To get a list of sentences in a PDF":                    "GET /listSentences/:id",
		"To search for the occurrences of a keyword in all PDFs": "GET /searchKeyword/:word",
		"To download a PDF":                                      "GET /getPDF/:id",
		"To get an image of a page in a PDF":                     "GET /getPDF/:id/:page",
		"To check the number of occurrences of a word in a PDF":  "GET /getOccurrences/:id/:word",
		"To get the top 5 occuring words in a PDF":               "GET /getMostOccurring/:id",
	}
}

func getMinioClient() (*minio.Client, error) {
	minioEndpoint := "minio:9000"
	minioAccessKey := "minioadmin"
	minioSecretKey := "minioadmin"
	useSSL := false

	// Initialize MinIO client
	return minio.New(minioEndpoint, &minio.Options{
		Creds:  credentials.NewStaticV4(minioAccessKey, minioSecretKey, ""),
		Secure: useSSL,
	})
}

func UploadPDF(ctx context.Context, bucketName string, objectName string, inFile io.Reader) error {
	minioClient, err := getMinioClient()
	if err != nil {
		return err
	}

	_, err = minioClient.PutObject(ctx, bucketName, objectName, inFile, -1, minio.PutObjectOptions{
		ContentType: "application/pdf",
	})
	if err != nil {
		return err
	}

	return nil
}

func getQueriesConnection(ctx context.Context) (*pgx.Conn, *database.Queries, error) {
	urlExample := fmt.Sprintf("postgres://%s:%s@db:5432", os.Getenv("DB_USER"), os.Getenv("DB_NAME"))
	conn, err := pgx.Connect(ctx, urlExample)
	if err != nil {
		return nil, nil, err
	}
	return conn, database.New(conn), nil
}

func CreateRecord(ctx context.Context, name string, pages int32, size int32) (database.Record, error) {
	conn, queries, err := getQueriesConnection(ctx)
	if err != nil {
		return database.Record{}, err
	}
	defer conn.Close(ctx)

	return queries.CreateRecord(ctx, database.CreateRecordParams{
		Name:       name,
		Numofpages: pages,
		Size:       size,
	})
}

func GetRecord(ctx context.Context, id int32) (database.Record, error) {
	conn, queries, err := getQueriesConnection(ctx)
	if err != nil {
		return database.Record{}, err
	}
	defer conn.Close(ctx)

	return queries.GetRecord(ctx, id)
}

func ListRecords(ctx context.Context) ([]database.Record, error) {
	conn, queries, err := getQueriesConnection(ctx)
	if err != nil {
		return []database.Record{}, err
	}
	defer conn.Close(ctx)

	return queries.ListRecords(ctx)
}

func ListRecordSentences(ctx context.Context, id int32) ([]database.Sentence, error) {
	conn, queries, err := getQueriesConnection(ctx)
	if err != nil {
		return []database.Sentence{}, err
	}
	defer conn.Close(ctx)

	return queries.ListRecordSentences(ctx, id)
}

func deleteRecord(ctx context.Context, name string) error {
	conn, queries, err := getQueriesConnection(ctx)
	if err != nil {
		return err
	}
	defer conn.Close(ctx)

	err = queries.DeleteRecord(ctx, name)
	if err != nil {
		return err
	}

	return nil
}

func SplitAndStore(ctx context.Context, text string, insertedRecord database.Record) error {
	//Sentence package initialization.
	doc, err := prose.NewDocument(text)
	if err != nil {
		return err
	}

	conn, queries, err := getQueriesConnection(ctx)
	if err != nil {
		return err
	}
	defer conn.Close(ctx)

	//Store sentences in database.
	for _, sentence := range doc.Sentences() {
		_, err := queries.CreateSentence(ctx, database.CreateSentenceParams{
			Sentence: sentence.Text,
			Pdfid:    insertedRecord.ID,
		})
		if err != nil {
			return err
		}
	}
	return nil
}

func SendErrorStatus(c *fiber.Ctx, err string) error {
	return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
		"error": err,
	})
}

func SendBadRequestStatus(c *fiber.Ctx, err string) error {
	return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
		"error": err,
	})
}

func ReadPdf(path string) (*pdf.Reader, string, error) {
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

func CountFilteredWords(text string) map[string]int {
	reg := regexp.MustCompile("[^a-zA-Z]+")
	pointless := reg.ReplaceAllString(text, " ")
	clean := stopwords.CleanString(pointless, "en", false)
	words := strings.Split(clean, " ")
	fmt.Println(clean)
	filteredWords := map[string]int{}

	for _, word := range words {
		if strings.Contains(word, " ") {
			for key, value := range CountFilteredWords(word) {
				val := filteredWords[key]
				filteredWords[key] = value + val
			}
			continue
		}
		if word == "" {
			continue
		}

		count := filteredWords[word]
		count++
		filteredWords[word] = count
	}

	return filteredWords
}

func ExtractFrequency(entry string) int {
	if entry == "" {
		return 0
	}

	freq, err := strconv.Atoi(strings.Split(entry, " ")[1])
	if err != nil {
		log.Fatalln(err)
	}
	return freq
}

func ShiftAndUpdateTopFive(index int, key string, value int, topFive *map[int]string) {
	for i := 5; i > index; i-- {
		(*topFive)[i] = (*topFive)[i-1]
	}
	(*topFive)[index] = fmt.Sprintf("%s: %d times", key, value)
}

func CopyPDF(id int) (string, error) {
	ctx := context.Background()
	record, err := GetRecord(ctx, int32(id))
	if err != nil {
		return "A file with the id provided does not exist", err
	}

	// Initialize MinIO client
	minioClient, err := getMinioClient()
	if err != nil {
		return "Failed to initialize minio client", err
	}

	//download file from MinIO
	bucketName := "pdf"
	objectName := record.Name

	file, err := minioClient.GetObject(ctx, bucketName, objectName, minio.GetObjectOptions{})
	if err != nil {
		return "Failed to get the file from MinIO", err
	}
	defer file.Close()

	localFile, err := os.Create(record.Name)
	if err != nil {
		return "Failed to create a temporary copy of the file", err
	}
	defer localFile.Close()

	buffer := make([]byte, 1024)
	for {
		n, err := file.Read(buffer)
		if err != nil {
			break
		}
		localFile.Write(buffer[:n])
	}

	return localFile.Name(), nil
}

func ConvertPDFPageToImage(pdfPath, imagePath string, pageNum int) error {

	doc, err := fitz.New(pdfPath)
	if err != nil {
		return err
	}
	defer doc.Close()

	img, err := doc.Image(pageNum - 1) // Pages are 0-indexed
	if err != nil {
		return err
	}

	file, err := os.Create(imagePath)
	if err != nil {
		return err
	}
	defer file.Close()

	err = jpeg.Encode(file, img, &jpeg.Options{Quality: jpeg.DefaultQuality})
	if err != nil {
		os.Remove(imagePath)
		return err
	}

	return nil
}

func DeletePDF(ctx context.Context, name string) error {
	minioClient, err := getMinioClient()
	if err != nil {
		return fmt.Errorf("minio")
	}

	file, err := minioClient.GetObject(ctx, "pdf", name, minio.GetObjectOptions{})
	if err != nil {
		return fmt.Errorf("minio")
	}
	defer file.Close()

	err = minioClient.RemoveObject(ctx, "pdf", name, minio.RemoveObjectOptions{})
	if err != nil {
		return fmt.Errorf("minio")
	}

	err = deleteRecord(ctx, name)
	if err != nil {
		UploadPDF(ctx, "pdf", name, file)
		return fmt.Errorf("db")
	}

	return nil
}
