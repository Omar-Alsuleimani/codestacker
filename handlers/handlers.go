package handlers

import (
	"encoding/base64"
	"fmt"
	"main/utils"
	"os"
	"regexp"
	"strconv"
	"strings"

	"github.com/gofiber/fiber/v2"
)

func Home(c *fiber.Ctx) error {
	return c.JSON(utils.GetWelcome())
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
		return utils.SendErrorStatus(c, "Failed to open the pdf file")
	}
	defer inFile.Close()

	ctx := c.Context()
	err = utils.UploadPDF(ctx, "pdf", file.Filename, inFile)
	if err != nil {
		return utils.SendErrorStatus(c, "Failed to upload the pdf file to MinIO")
	}

	//Insert record of upload into the database
	err = c.SaveFile(file, file.Filename)
	defer os.Remove(file.Filename)
	if err != nil {
		return utils.SendErrorStatus(c, "Failed to save pdf file")
	}

	pdfFileName := file.Filename
	reader, text, err := utils.ReadPdf(pdfFileName)
	if err != nil {
		fmt.Printf("err: %v\n", err)
		return utils.SendErrorStatus(c, "Failed to read pdf as text")
	}

	insertedRecord, err := utils.CreateRecord(ctx, file.Filename, int32(reader.NumPage()), int32(file.Size))
	if err != nil {
		return utils.SendErrorStatus(c, "Failed to create a record for the file")
	}

	//Split sentences and upload them to the db
	err = utils.SplitAndStore(ctx, text, insertedRecord)
	if err != nil {
		return utils.SendErrorStatus(c, err.Error())
	}

	return c.JSON(fiber.Map{"Id": insertedRecord.ID, "File": insertedRecord.Name})
}

func ListFiles(c *fiber.Ctx) error {
	records, err := utils.ListRecords(c.Context())
	if err != nil {
		return utils.SendErrorStatus(c, "Failed to get the list of records")
	}

	return c.JSON(records)
}

func SearchKeyword(c *fiber.Ctx) error {
	keyword := c.Params("key")
	if keyword == "" {
		return utils.SendBadRequestStatus(c, "Invalid keyword")
	}

	ctx := c.Context()
	records, err := utils.ListRecords(ctx)
	if err != nil {
		return utils.SendErrorStatus(c, "Failed to get the list of records")
	}

	result := fiber.Map{}
	for _, record := range records {
		sentences, err := utils.ListRecordSentences(ctx, record.ID)
		if err != nil {
			return utils.SendErrorStatus(c, "Failed to get the list of sentences for record: "+fmt.Sprint(record.ID))
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
	id, err := c.ParamsInt("id", -1)
	if err != nil || id == -1 {
		return utils.SendBadRequestStatus(c, "Invalid id provided")
	}

	localFile, err := utils.CopyPDF(id)
	if err != nil {
		return utils.SendErrorStatus(c, localFile)
	}
	defer os.Remove(localFile)

	return c.SendFile(localFile)
}

func ListSentences(c *fiber.Ctx) error {
	id, err := c.ParamsInt("id", -1)
	if err != nil || id == -1 {
		return utils.SendBadRequestStatus(c, "Id invalid or not provided")
	}

	ctx := c.Context()
	sentences, err := utils.ListRecordSentences(ctx, int32(id))
	if err != nil {
		return utils.SendErrorStatus(c, "Failed to retrieve the list of sentences for the selected file")
	}

	return c.JSON(sentences)
}

func GetOccurrence(c *fiber.Ctx) error {
	id, err := c.ParamsInt("id", -1)
	if id == -1 || err != nil {
		return utils.SendBadRequestStatus(c, "Id invalid or not provided")
	}

	keyword := c.Params("key")
	if keyword == "" {
		return utils.SendBadRequestStatus(c, "Keyword invalid or not provided")
	}

	ctx := c.Context()
	sentences, err := utils.ListRecordSentences(ctx, int32(id))
	if err != nil {
		return utils.SendErrorStatus(c, "Failed to retrieve the list of sentences for the selected file")
	}

	count := 0
	result := fiber.Map{}
	foundIn := map[string]string{}
	for index, sentence := range sentences {
		reg := regexp.MustCompile("[^a-zA-Z\\s-_/]+")
		rep := reg.ReplaceAllString(sentence.Sentence, " ")
		words := strings.Split(rep, " ")
		found := false
		for _, word := range words {

			if strings.ToLower(word) == strings.ToLower(keyword) {
				count++
				found = true
			}
		}
		if found {
			foundIn[fmt.Sprintf("Sentence %d", index)] = sentence.Sentence
		}
	}

	result["Count"] = count
	result["Found in"] = foundIn

	return c.JSON(result)
}

func GetMostOccurring(c *fiber.Ctx) error {
	id, err := c.ParamsInt("id", -1)
	if id == -1 || err != nil {
		return utils.SendBadRequestStatus(c, "Id invalid or not provided")
	}

	ctx := c.Context()
	sentences, err := utils.ListRecordSentences(ctx, int32(id))
	if err != nil {
		return utils.SendErrorStatus(c, "Failed to retrieve the list of sentences for the selected fil")
	}

	text := ""
	for _, sentence := range sentences {
		trim := strings.TrimSpace(sentence.Sentence)
		if trim != "" {
			text += trim + " "
		}
	}

	trim := strings.TrimSpace(text)
	filteredWords := utils.CountFilteredWords(trim)
	topFive := map[int]string{}

	for key, value := range filteredWords {
		if topFive[1] == "" || value > utils.ExtractFrequency(topFive[1]) {
			utils.ShiftAndUpdateTopFive(1, key, value, &topFive)
			continue
		}

		if value > utils.ExtractFrequency(topFive[2]) {
			utils.ShiftAndUpdateTopFive(2, key, value, &topFive)
			continue
		}

		if value > utils.ExtractFrequency(topFive[3]) {
			utils.ShiftAndUpdateTopFive(3, key, value, &topFive)
			continue
		}

		if value > utils.ExtractFrequency(topFive[4]) {
			utils.ShiftAndUpdateTopFive(4, key, value, &topFive)
			continue
		}

		if value > utils.ExtractFrequency(topFive[5]) {
			utils.ShiftAndUpdateTopFive(5, key, value, &topFive)
		}
	}

	return c.JSON(topFive)
}

func GetPdfPage(c *fiber.Ctx) error {
	id, err := c.ParamsInt("id", -1)
	if err != nil || id == -1 {
		return utils.SendBadRequestStatus(c, "Invalid id provided")
	}

	page, err := c.ParamsInt("page", -1)
	if err != nil || page <= -1 {
		return utils.SendBadRequestStatus(c, "Page invalid")
	}

	localFile, err := utils.CopyPDF(id)
	if err != nil {
		return utils.SendErrorStatus(c, localFile)
	}
	defer os.Remove(localFile)

	imageFile := "output.png"

	err = utils.ConvertPDFPageToImage(localFile, imageFile, page)
	if err != nil {
		return utils.SendErrorStatus(c, "Failed to convert pdf page to an image")
	}
	defer os.Remove(imageFile)

	return c.SendFile(imageFile)
}

func DeleteFile(c *fiber.Ctx) error {
	in := c.FormValue("id")
	id, err := strconv.Atoi(in)
	if err != nil {
		return utils.SendBadRequestStatus(c, "Invalid input")
	}

	if id <= -1 {
		return utils.SendBadRequestStatus(c, "Invalid id")
	}

	ctx := c.Context()
	record, err := utils.GetRecord(ctx, int32(id))
	if err != nil {
		return utils.SendErrorStatus(c, "Failed to get the pdf details from the database")
	}

	pdfName := record.Name
	err = utils.DeletePDF(ctx, pdfName)
	if err != nil {
		switch err.Error() {
		case "minio":
			return utils.SendErrorStatus(c, "Failed to delete the file from MinIO")
		case "db":
			return utils.SendErrorStatus(c, "Failed to delete the file details from the database")
		}
	}

	return c.JSON(fiber.Map{"success": "pdf deleted without errors"})
}

func BasicAuthMiddleware(username, password string) fiber.Handler {
	return func(c *fiber.Ctx) error {
		auth := c.Get(fiber.HeaderAuthorization)
		if auth == "" {
			c.Status(fiber.StatusUnauthorized)
			return c.SendString("Unauthorized")
		}

		encodedCredentials := strings.TrimPrefix(auth, "Basic ")
		credentials, err := base64.StdEncoding.DecodeString(encodedCredentials)
		if err != nil {
			c.Status(fiber.StatusUnauthorized)
			return c.SendString("Unauthorized\n")
		}

		credentialsParts := strings.SplitN(string(credentials), ":", 2)
		if len(credentialsParts) != 2 || credentialsParts[0] != username || credentialsParts[1] != password {
			c.Status(fiber.StatusUnauthorized)
			return c.SendString("Unauthorized\n")
		}

		return c.Next()
	}
}
