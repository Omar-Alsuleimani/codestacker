package utils

import (
	"bytes"
	"context"
	"fmt"
	"image/jpeg"
	"log"
	"main/database"
	"os"
	"regexp"
	"strconv"
	"strings"

	"github.com/dslipak/pdf"
	"github.com/gen2brain/go-fitz"
	"github.com/gofiber/fiber/v2"
	"github.com/jackc/pgx/v4"
	"github.com/jdkato/prose/v2"
	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"
)

func SplitAndStore(ctx context.Context, text string, queries *database.Queries, insertedRecord database.Record) error {
	//Sentence package initialization.
	doc, err := prose.NewDocument(text)
	if err != nil {
		return err
	}

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

	trim := strings.TrimSpace(text)
	words := strings.Split(trim, " ")
	stopWords := []string{
		"a", "b", "c", "d", "e", "f", "g", "h", "i", "j",
		"k", "l", "m", "n", "o", "p", "q", "r", "s", "t",
		"u", "v", "w", "x", "y", "z", "about", "above", "after", "again", "against", "all", "am", "an", "and",
		"any", "are", "aren't", "as", "at", "be", "because", "been", "before", "being",
		"below", "between", "both", "but", "by", "can't", "cannot", "could", "couldn't",
		"did", "didn't", "do", "does", "doesn't", "doing", "don't", "down", "during",
		"each", "few", "for", "from", "further", "had", "hadn't", "has", "hasn't", "have",
		"haven't", "having", "he", "he'd", "he'll", "he's", "her", "here", "here's", "hers",
		"herself", "him", "himself", "his", "how", "how's", "i", "i'd", "i'll", "i'm",
		"i've", "if", "in", "into", "is", "isn't", "it", "it's", "its", "itself", "let's",
		"me", "more", "most", "mustn't", "my", "myself", "no", "nor", "not", "of", "off",
		"on", "once", "only", "or", "other", "ought", "our", "ours", "ourselves", "out",
		"over", "own", "same", "shan't", "she", "she'd", "she'll", "she's", "should",
		"shouldn't", "so", "some", "such", "than", "that", "that's", "the", "their", "theirs",
		"them", "themselves", "then", "there", "there's", "these", "they", "they'd", "they'll",
		"they're", "they've", "this", "those", "through", "to", "too", "under", "until", "up",
		"very", "was", "wasn't", "we", "we'd", "we'll", "we're", "we've", "were", "weren't",
		"what", "what's", "when", "when's", "where", "where's", "which", "while", "who",
		"who's", "whom", "why", "why's", "with", "won't", "would", "wouldn't", "you", "you'd",
		"you'll", "you're", "you've", "your", "yours", "yourself", "yourselves", "can",
	}

	filteredWords := map[string]int{}
	reg := regexp.MustCompile("[^a-zA-Z]+")
	for _, word := range words {
		word = reg.ReplaceAllString(word, " ")
		word = strings.TrimSpace(word)
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

		isStopWord := false
		for _, stopWord := range stopWords {
			if strings.ToLower(word) == stopWord {
				isStopWord = true
				break
			}
		}

		if !isStopWord {
			count := filteredWords[word]
			count++
			filteredWords[word] = count
		}
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

func CopyPDF(c *fiber.Ctx) (string, error) {
	id, err := c.ParamsInt("id", -1)
	if err != nil || id == -1 {
		return "", SendErrorStatus(c, "Invalid id provided")
	}

	ctx := c.Context()
	urlExample := fmt.Sprintf("postgres://%s:%s@db:5432", os.Getenv("DB_USER"), os.Getenv("DB_NAME"))
	conn, err := pgx.Connect(ctx, urlExample)
	if err != nil {
		return "", SendErrorStatus(c, "Failed to connect to the database")
	}
	defer conn.Close(ctx)

	queries := database.New(conn)

	record, err := queries.GetRecord(ctx, int32(id))
	if err != nil {
		return "", SendErrorStatus(c, "A file with the id provided does not exist")
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
		return "", SendErrorStatus(c, "Failed to initialize minio client")
	}

	//download file from MinIO
	bucketName := "pdf"
	objectName := record.Name

	file, err := minioClient.GetObject(ctx, bucketName, objectName, minio.GetObjectOptions{})
	if err != nil {
		return "", SendErrorStatus(c, "Failed to get the file from MinIO")
	}
	defer file.Close()

	localFile, err := os.Create(record.Name)
	if err != nil {
		return "", SendErrorStatus(c, "Failed to create a temporary copy of the file")
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
